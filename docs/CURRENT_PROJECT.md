# Flash Mall 当前项目状态

> 本文件是仓库内唯一有效的项目状态文档。内容以当前代码、配置和部署清单为准；历史方案、执行计划与聊天日志都不是开发依据。

## 面试资料入口

- `docs/INTERVIEW_GUIDE.md`：30秒、3分钟和15分钟讲解主线，核心链路深挖、高频追问、表达边界与现场兜底。
- `docs/RESUME_PROJECT.md`：仅包含可替换旧简历中 Flash Mall 项目经历的新文本，不修改或重制整份简历。
- `docs/ARCHITECTURE_HIGHLIGHTS.md`：系统结构、关键链路、架构亮点、代码证据、验证结果及已知不足。

三份文件是基于本现状文档和当前代码生成的面试材料，不替代本文件作为开发事实来源；代码或验证基线变化后应同步更新对应数字和表述。

## 当前目标

Flash Mall 用同一套商城业务展示从 Go-zero Entry API 向 Hertz + Kitex 演进的过程：

- `codex/arch-hertz-kitex` 是默认开发分支，Hertz 是默认外部 HTTP 网关。
- `main` 保留 Go-zero Entry API 基线，用于功能、性能和架构对比，不继续承载新功能。
- Kitex 只承担库存领域的高频、稳定、强边界同步命令；商品读取继续使用 Product RPC/快照。
- 支付后的卡片刷新、库存审计、运营统计和搜索更新通过 Outbox + RabbitMQ 异步执行。

## 运行拓扑

默认本地入口是 `http://127.0.0.1:8889`。

| 组件 | 职责 | 默认端口 |
| --- | --- | --- |
| `hertz-gateway` | 商城、商家、管理员 HTTP API 与静态页面 | 8889 |
| `entry-api` | Go-zero 对比基线；默认本地启动流程不使用 | 8888 |
| `auth-api` | 凭证、会话、验证码与安全审计 | 8890 |
| `product-rpc` | 商品元数据和商品读模型 | 8080 |
| `order-rpc` | 订单状态机、支付、退款、Outbox 与 SAGA | 8090 |
| `inventory-kitex` | 预占、释放、确认、调库存与一致性修复 | 8093 |
| MySQL / Redis | 持久化、缓存、库存热数据 | 3307 / 6379 |
| RabbitMQ / Etcd / DTM | 事件、服务发现和分布式事务辅助 | 5672 / 2379 / 36789 |

Compose 同时保留两个 HTTP 服务定义；桌面控制中心和默认快速启动链路只启动 Hertz 拓扑。旧 Entry API 通过显式对比流程启动。

## 领域所有权

- Hertz 负责外部路由、请求身份、角色校验、响应格式和静态页面服务，不应直接拥有订单写状态或库存命令。
- Auth API 负责用户、管理员与商家登录态；调用方不得通过本地伪造身份绕开它。
- Order RPC 负责订单行锁、状态迁移、状态日志、支付/退款幂等与 Outbox 事务。
- Inventory Kitex 是库存写入唯一入口；预占、释放、确认和调库存必须携带请求 ID、追踪 ID、订单 ID 与幂等键。
- Product RPC、商品表和商品卡片快照负责商品读路径。列表与详情不为统一技术栈额外增加 Kitex 跳数。
- RabbitMQ 消费者只处理支付后的非关键派生工作，不进入同步支付成功条件。

## 已实现业务范围

- 用户：注册登录、地址、商品和店铺浏览、下单、沙箱支付、取消、退款申请、确认收货。
- 商家：申请入驻、店铺资料和素材、商品、库存、订单发货、退款处理与运营看板。
- 管理员：商家审核、商品、供应商、促销活动、首页橱窗、订单、退款、对账、事件和系统看板。
- 店铺：首页商品进入详情或商家店面；商家维护店内上架商品，管理员发布首页 12 槽橱窗。
- 首页推荐：销量、库存、促销、新鲜度和商家多样性共同产生候选分数；最终发布仍由管理员决定。

## 可靠性设计

### 库存

- Redis 负责库存热路径，MySQL 保存库存事实、预占账本、变更日志与快照。
- `order_id` 是预占幂等身份；相同订单不同商品或数量必须拒绝。
- Release 支持空补偿和重复调用；Redis 数据缺失时可依据 MySQL 账本恢复。
- 恢复任务处理过期预占、重试和死信；对账不能用数据库总量覆盖仍被预占的可用量。
- `/live` 只表示 Hertz 进程存活；`/ready`、`/health` 和 `/api/system/health` 校验 Kitex、Redis、MySQL、最终扣减开关、账本模式及上传存储就绪状态。

### 订单和支付

- 下单写入订单并通过 Kitex 预占库存；失败按既定 SAGA/补偿规则恢复。
- 默认支付渠道是 `local_sandbox`，保证离线演示和 CI 可重复；设置 `FLASH_MALL_PAYMENT_PROVIDER=alipay_sandbox` 并注入支付宝开放平台沙箱凭据后，Order RPC 通过 `alipay.trade.precreate` 创建原生付款码。
- 支付宝请求使用 RSA2 签名并验证同步响应签名；异步通知 `/api/payment/alipay/notify` 校验 RSA2、应用 ID、支付渠道、外部交易号和实付金额，再进入统一支付状态机。
- 支付回调使用业务订单号、支付单号、渠道事件号和幂等状态机防止重复入账；支付成功事务提交后即向渠道确认成功，库存最终扣减失败进入持久化重试，不让已经收款的订单因瞬时 Kitex 故障反复回调。
- 支付单保存付款渠道、渠道交易号、二维码内容、渠道状态和截止时间；恢复任务在截止前后查询支付宝交易状态，先补偿漏通知的已付款交易，再关闭未付款交易并释放库存。
- 取消和超时关单在本地状态迁移前调用渠道关单；退款审核使用稳定的渠道退款请求号调用原路退款，渠道成功、库存释放和本地退款完成可以分阶段重试。
- 支付成功状态与 Outbox 事件在同一 MySQL 事务提交，消息发布失败由后台发布器重试。
- 取消、关闭、发货、确认收货等写命令由 Hertz 经 Go-zero Order RPC 执行，Hertz 不直接更新订单状态。

### 分层缓存

- 商品、店铺和首页橱窗使用统一缓存协调器。
- L1 是进程内短 TTL 缓存；L2 是 Redis，支持软/硬 TTL、抖动、singleflight、过期回源和原点失败时的陈旧值兜底。
- 商品、店铺、快照和橱窗变更执行精确键与前缀失效，并通过 Redis Pub/Sub 清理其他 Hertz 实例的 L1。
- 商品详情只在 L1/L2 未命中并进入 singleflight loader 后检查短期负缓存和 Redis Bitmap 布隆过滤器；ready generation 明确判定不存在时不访问 MySQL 或 Product RPC，Redis 异常、元数据异常和未就绪状态一律 fail-open。
- 商品过滤器保存所有历史商品 ID，不承担上下架、店铺状态、库存或权限判断。默认按 100 万商品、1% 假阳性率配置为 9,585,059 bit、7 次双哈希，负缓存基础 TTL 为 30 秒并带稳定抖动。
- 多 Hertz 副本共享版本化 active generation；新 generation 完整构建后才通过 Lua 切换，重建失败保留旧版本。新商品创建、库存种子重试和既有商品上架均先幂等写入过滤器，再允许公开；商品变更同时精确清除负缓存。

### 容量与 SLO

- Hertz 全局中间件记录固定低基数 RED 指标：按 HTTP 方法、匹配后的路由模板和状态码类别统计请求数、耗时直方图与在途请求；未匹配 API 统一归为 `unmatched_api`，静态资源统一归为 `static`。
- Prometheus 为 Hertz 5xx 比例和 p95 尾延迟提供告警，Grafana 的“容量与 SLO”面板展示 API 吞吐、成功率、p95/p99、在途请求与服务端错误率。
- `tools/capacitybench` 直接使用真实登录、公开读、Kitex 库存预占驱动的订单生命周期、本地沙箱支付和幂等重放，不使用伪 Handler 或内存替身。
- 容量实验只允许写入回环地址，必须显式授权修改与演示数据重置；运行前后备份并恢复固定数据，保留上传素材卷。结果同时校验重复订单、重复支付回调、负库存、悬挂预占、未清空 Outbox、支付库存未最终确认及 Redis/MySQL 库存桶差异。
- “安全目标 RPS”表示当前固定 Compose 资源和开发机上最高已测且通过的档位；未出现失败档位时只能陈述“至少达到”，不能解释为系统极限或公网生产容量。
- 完整套件区分三轮基准、预期负载、压力阶梯、五分钟混合稳定性和故障恢复；同时采集业务步骤延迟、Go CPU profile、容器 CPU/内存、MySQL、Redis 与主机资源。稳定性阶段若单服务内存增长超过 128 MiB、Redis 出现阻塞或主机可用内存异常下降会直接失败。

### 指标、日志与追踪

- Prometheus 持续抓取 Hertz、Order RPC、Product RPC 和 Inventory Kitex 四个业务目标；Grafana 自动装载总览、库存一致性、支付与 Outbox、缓存与 RPC、容量与 SLO 五个面板。
- Hertz 的全局 Trace 中间件为每个业务 API 请求创建 OTel 服务端 Span，并记录路由模板、状态码、请求 ID 和业务追踪 ID；探针、Prometheus 抓取和静态资源不写入 Jaeger，避免观测流量淹没业务链路。
- Order RPC 为订单创建、支付意图和支付确认创建业务 Span；Inventory Kitex 为预占、确认和释放创建库存命令 Span，并记录订单、商品、数量和关联 ID。OTLP/HTTP 统一写入 Jaeger。
- 商品列表和详情仍以低跳数读路径、缓存指标和 Product RPC 指标为主，不为了形式上的全链路统一给高频读取增加 Kitex 调用。

## 前端与静态资源

- 唯一前端源码是 `frontend/packages` 下的 `shop`、`admin`、`merchant` 和 `shared`。
- 根目录旧 `web` 工程不再继续开发，清理后不得重新作为 Entry API 的构建来源。
- 构建产物统一放在中立目录 `artifacts/web`，由 Hertz 和 Go-zero Entry API 分别复制或嵌入。
- 商品和店铺上传素材存储在显式命名卷 `flash-mall-uploads`，不属于前端编译产物，也不随 worktree、源码清理或镜像重建消失。
- 新素材以内容 SHA-256 命名，通过同目录临时文件、文件和父目录 `fsync`、原子重命名写入；已存在的同名文件会先校验内容哈希，损坏文件不会被错误复用。
- 素材上传 Handler 只依赖可注入的 `assetstore.Store` 写边界，本地文件系统是当前适配器，后续接入 MinIO 不需要修改上传业务 Handler。
- 启动阶段只执行廉价的上传目录可写探测，数据库引用、缺失文件、非法路径和内容哈希由后台审计并缓存；历史素材缺失使就绪响应进入 `degraded`，不再导致网关 503。审计结果同时暴露为 `flash_mall_upload_integrity_*` Prometheus 指标。
- 管理员、商家、商城图片组件均提供加载失败降级；管理员缩略图在 URL 变化后会清除旧失败状态并重新加载。
- 订单价格快照保存 `product_image_url`，订单列表和详情不再依赖商品当前图片或前端硬编码映射。

## 开发边界

- 新增外部 API 只实现到 Hertz；除修复对比基线自身缺陷外，不向 Entry API 同步新业务。
- Hertz Handler 应只保留 HTTP 适配、鉴权调用和结果映射；数据库查询与业务编排逐域下沉到应用服务和适配器。
- DDL 只允许出现在数据库迁移/初始化脚本，运行时代码只能做只读 schema readiness 检查。
- 生成的 Protobuf/Kitex 文件不手工编辑；接口变化从 IDL 重新生成。
- 不因框架统一而把商品高频读取迁往 Kitex。

## 本地操作

```powershell
# 安装或更新 Windows 桌面控制中心
pwsh -NoProfile -File scripts/local/install-desktop-launcher.ps1
```

```bash
# 在 Ubuntu WSL 中启动默认 Compose 拓扑
./scripts/local/flash-mall-control.sh start --profile interview --observability

# 对当前运行环境执行只读发布就绪检查：三套页面、图片、四个角色、
# 商家隔离、Prometheus、Grafana、Jaeger 和 RabbitMQ
node scripts/local/verify-release-readiness.mjs

# 检查固定演示账号、商家、商品和 MySQL/Redis 库存一致性
./scripts/local/flash-mall-control.sh verify-demo --profile interview

# 明确确认后备份数据库、重置 MySQL/Redis 并恢复固定演示环境
./scripts/local/flash-mall-control.sh reset-demo --confirm-reset --profile interview

# 查看健康状态
./scripts/local/health-compose.sh

# 启动 Prometheus 与 Grafana；Grafana 默认监听 3000
docker compose -f deploy/docker-compose.yml --profile observability up -d prometheus grafana

# 主机 3000 被占用时只改宿主端口，容器内配置保持不变
FLASH_MALL_GRAFANA_PORT=3001 docker compose -f deploy/docker-compose.yml --profile observability up -d prometheus grafana

# 启用支付宝沙箱前复制 deploy/.env.example，填入沙箱 AppID、应用私钥、
# 支付宝公钥和公网可访问的通知地址；密钥只放本地 .env 或 K8s Secret。
# 未配置这些值时保持 local_sandbox，不影响商城演示。

# 显式授权后执行可自动恢复的本地故障演练
./scripts/local/verify-failure-recovery.sh --allow-disruption --scenario all

# 只重建一个服务
./scripts/local/rebuild-compose-service.sh hertz-gateway

# 首次升级到命名卷时，无损迁移一个或多个旧上传目录；重复执行安全
./scripts/local/migrate-uploads-to-volume.sh /path/to/old/.runtime/uploads

# 用真实管理员/用户链路验证哈希上传、中文/emoji 订单快照和图片读取
node scripts/local/verify-durable-assets.mjs --allow-mutation /path/to/image.png 106

# 对 origin/main 的 Go-zero Entry API 与当前 Hertz 网关执行成对性能对比
# baseline worktree 必须干净且内容树与 origin/main 完全一致
./scripts/perf/compare-entry-hertz.sh --runs 7 --requests 60000 --concurrency 30 --warmup 2000

# 快速验证性能链路，或执行完整基准、负载、压力、稳定性与恢复套件
./scripts/perf/run-performance-suite.sh --suite quick --allow-mutation --confirm-reset
./scripts/perf/run-performance-suite.sh --suite full --allow-mutation --confirm-reset
```

素材持久化验收脚本只允许连接本机回环地址，必须显式传入 `--allow-mutation`。脚本结束时会取消验收订单以释放库存，并恢复商品原始名称、图片、价格、供应商和状态；账号密码可用 `FLASH_MALL_VERIFY_*` 环境变量覆盖。

Docker 构建使用服务级源码复制和共享 BuildKit 缓存。日常迭代按服务重建，缓存超过预算后保留近期热缓存；任何自动清理都不得删除 MySQL、Redis 或 RabbitMQ 数据卷。

## 当前清理重点

已完成的清理：

- Inventory Repository 不再执行 DDL，运行检查只读验证必需表；Redis/MySQL 实现已按命令、持久化、编解码和 Lua 脚本拆分。
- `frontend/packages` 是唯一前端源码，产物统一写入 `artifacts/web`；根目录旧 `web` 工程和 Entry Handler 内产物副本已删除。
- Hertz 与 Go-zero Entry API 均消费中立产物；Entry API 已标记为冻结的比较基线，Hertz 架构测试禁止依赖它。
- Auth 部署显式使用 `StorageMode: mysql`；具名服务缺失模式或 MySQL 模式缺失数据源会拒绝启动。
- 管理员促销已拆成 `application/promotion` 与 `adapters/productmysql`：折扣、时间窗、冲突和更新规则不再由 HTTP Handler 编排。
- 首页橱窗已拆成 `application/showcase` 与 `adapters/productmysql`：版本锁、槽位/商家约束和发布事务不再位于 Handler。
- 用户订单列表、详情、支付单和下单幂等回读已拆成 `application/orderquery` 与 `adapters/ordermysql`；订单写命令仍保持走 Order RPC。
- 管理员/商家订单列表、详情、状态日志和退款列表已统一进入 Backoffice 订单查询服务；`admin_order.go` 与 `merchant_order.go` 不再执行 SQL。
- 首页橱窗候选 SQL 已进入 `productmysql.ShowcaseRepository`，Handler 只负责参数、缓存、Product RPC 卡片补全和响应。
- `catalog.go` 已按参数解析、商品卡片、查询服务分拆；商品图片、商家/供应商元数据由 `application/catalogquery` 和 `adapters/productmysql` 提供，不再在主 Handler 中拼接 SQL。
- 商品列表筛选、关联商品、公开店铺详情/商品列表以及管理员商品列表/详情已统一进入 `catalogquery` 与 `productmysql`；`catalog.go`、`storefront.go`、`admin_product_query.go` 均有禁止直接持久化的架构约束。
- 管理员供应商查询与写入已进入 `application/supplier` 和 `productmysql.SupplierRepository`；供应商停用时的启用商品检查与状态更新在同一数据库事务内完成，Handler 只保留 HTTP 映射与审计。
- 商家身份列表、最新入驻申请和仪表盘统计已进入 `application/merchantquery` 与 `adapters/ordermysql`。
- 商家申请提交、管理员申请列表和审核状态机已统一进入 `application/merchantonboarding` 与 `ordermysql.MerchantOnboardingRepository`；审批通过创建商家、写入 owner 成员关系和更新申请在同一事务内完成，同方向重复审核幂等返回已有结果，相反方向重复审核才返回冲突。商家权限继续以 `mall_order.merchant_user` 为事实来源，不跨库篡改 Auth 用户角色。
- 管理员与商家的商品创建、元数据更新已统一进入 `application/productcommand` 与 `productmysql.ProductCommandRepository`；有效供应商/商家校验、商品行锁、最终价格约束、离线商品与库存初始化任务写入由事务保证，商家更新通过事务内 `merchant_id` 条件隔离所有权。商品初始库存仍在事务提交后通过 Inventory Kitex 写入，失败时商品保持离线并保留可重试种子任务。
- 管理员商品、促销、订单和供应商页面已拆出列定义、编辑/详情/日志弹窗及页面模型；四个页面只保留状态、导航和 API 编排，并由架构测试限制体积与组件边界。
- 管理员首页橱窗页已拆出草稿模型、12 槽编辑器和推荐候选面板；安全事件页已拆出事件语义、筛选条和列定义；用户页已拆出列定义、详情弹窗和角色/状态展示。页面仍保留各自的请求状态与业务动作，现有橱窗拖拽、商家多样性和版本冲突行为保持不变。
- 数据库结构迁移与演示数据已经彻底分离：七个结构模块生成 `scripts/k8s/schema.sql`，三个演示模块生成 `scripts/k8s/demo-seed.sql`，旧 `init-db.sql` 已删除。Compose 只在全新环境首次写入演示数据，已有完整演示环境只登记版本而不覆盖业务数据；K8s 默认只执行结构迁移。
- Auth MySQL Store 不再在登录、会话或验证码请求中懒加载演示账号；四个演示账号及管理员一键登录凭据只由版本化演示种子维护。
- Windows 桌面控制中心支持日常开发/面试演示运行配置、可观测组件开关、演示数据检查和确认后重置；重置前自动备份 MySQL，只删除当前 Compose 项目的 MySQL/Redis 卷并保留 `flash-mall-uploads`。
- 旧版 8888 原生进程启动链 `launcher.ps1`、`start-all.ps1`、`stop-all.ps1`、`prepare-local-exes.ps1` 和 `test-launcher.ps1` 已删除；本地操作只保留 WSL Docker 与 Windows 桌面控制中心两层入口。
- 管理员看板、Outbox 事件列表/重试已进入 `application/adminops` 与 `ordermysql.AdminOpsRepository`；看板统计由原先 17 次串行查询收敛为一次聚合查询。
- 支付/退款/订单对账已进入 `application/reconciliation` 与 `ordermysql.ReconciliationRepository`；扫描、幂等问题键和列表查询不再位于 Handler，集成测试也不再运行时修改表结构。
- 用户地址已进入 `application/useraddress` 与 `adapters/authmysql`；默认地址切换、地址保存及用户所有权检查在同一事务内完成。
- 商家店铺资料已进入 `application/merchantstore` 与 `ordermysql.MerchantStoreRepository`；素材 URL、描述长度和乐观版本校验位于应用层，更新事务负责商家状态、行锁和版本冲突。
- 秒杀活动管理已进入 `application/campaign` 与 `productmysql.CampaignRepository`；库存、限购数默认值、输入规范化及新增/更新不再由 Handler 持有。
- 管理员退款查询复用 Backoffice 订单查询服务，库存变更日志进入 `application/stockaudit` 与 `productmysql.StockAuditRepository`；管理员可选商家筛选和商家强制隔离仍分别保留。
- 商家操作范围统一由 `merchantquery.ResolveScope` 根据 `mall_order.merchant_user` 解析，商品所有权统一由 `catalogquery.OwnsProduct` 在商品库判断；库存调整、库存种子重试和店铺素材接口不再获取裸数据库句柄。
- 支付二维码/状态查询和下单状态轮询已进入 `orderquery`；令牌查询同时约束支付单号、订单号和外部交易号，支付成功写入仍只走 Order RPC。
- Hertz 生产 Handler 已清除全部 `database/sql`、`RawDB`、SQL 查询和事务调用，并新增全目录架构守卫；SQL 只允许存在于 `adapters`，依赖只允许由 `ServiceContext` 装配。历史 schema readiness Handler 及其自测死代码已删除。
- 原 396 行 `order.go` 已按订单创建/支付意图、用户订单读取、取消/退款/确认收货写命令拆分；路由和 RPC 边界保持不变，后续修改不再集中到单一文件。
- 支付回调已拆成 HTTP 适配和独立签名协议模块；HMAC-SHA256、规范化签名载荷和 300 秒时钟偏差可在固定时间下确定性验证。支付意图、支付状态和沙箱确认也已拆成独立文件，均继续通过 Order RPC/查询服务访问支付状态。
- 迁移状态接口的 HTTP 契约与静态路由目录已经分离；认证、支付、订单、库存、管理员和商家路由各有唯一归属，已上线的活动、库存审计与支付路由不再被误报为规划中。
- 商家商品页已拆成页面编排、列定义、商品编辑弹窗、库存弹窗和表单模型；店铺设置页已拆成资料表单、公开预览和页面请求状态，图片上传、库存 Kitex 命令、乐观锁冲突等既有行为保持不变。
- `shared/src/types.ts` 已从 543 行混合声明改为公共 barrel，认证、商城、订单、商家及管理员领域类型分别维护；现有 `@flash-mall/shared` 导入方式保持兼容。
- 管理员商品页已进一步拆成薄页面、`useProductManagement` 页面控制器和独立商品 API；筛选、详情、创建、编辑、上下架、图片上传与库存调整不再堆积在页面组件中。
- 仓库内已确认没有前端、脚本或服务继续消费 `/api/gateway/*` 与 `/api/catalog` 旧别名，因此兼容路由已删除；公开 API 只保留当前规范路由，路由回归测试会阻止旧别名重新注册。
- MySQL Compose 默认字符集和排序规则显式固定为 `utf8mb4/utf8mb4_unicode_ci`；Auth、Product、Order 和 Hertz 拒绝未显式携带 `charset=utf8mb4` 的 DSN，Hertz 还会在真实连接建立后验证 product、order、auth 三个会话的 client/connection/results 字符集。
- 历史订单乱码修复只处理名称含连续三个问号、且规范商品名有效的快照；修复前原值和十六进制字节进入 `order_snapshot_repair_audit`，迁移版本 `20260723_order_snapshot_utf8_asset_repair` 成功记录后不再重复扫描历史订单。
- 商品 RPC 卡片同时返回图片 URL 和商家 ID，Order RPC 下单直接写入名称/图片/商家快照；Hertz 用户、管理员和商家订单查询统一返回 `image_url`。
- 商品缓存穿透防护已收口到独立 `existencefilter` 组件：普通 Redis Bitmap、版本化重建、分布式重建锁、短期负缓存、写链路可见性屏障、健康详情、固定低基数指标和 Grafana 面板均已接入；Go-zero Entry API 基线未复制该能力。

本轮代码清理、容量证据链、可观测闭环、故障恢复和真实浏览器验收均已收口。仓库不再围绕已经完成的分层重复重构；功能层面的剩余工作仅是实际部署时注入支付宝沙箱凭据和公网通知地址，或根据目标环境配置域名、TLS、Secret 与资源限额。当前仍有一项明确工程债：`app/order/rpc/internal/job/order_paid_projection.go` 为兼容旧环境保留了消费者运行时建表，生产化前应迁入版本化 schema 脚本并改为只读 readiness 校验。收尾阶段不再扩展 Stripe 或微信支付，避免扩大维护面。

## 15 分钟面试演示顺序

1. **0–2 分钟：启动与拓扑。** 从 Windows 桌面控制中心选择“面试演示 + 可观测”，说明外部入口是 Hertz 8889，Go-zero Entry API 8888 只保留为 `main` 基线；展示 `verify-release-readiness.mjs` 的 34 项只读检查。
2. **2–4 分钟：商城与店铺。** 浏览首页六槽商品、商品详情和两个独立店铺；强调商家负责店内上架，管理员通过推荐候选和 12 槽橱窗决定首页曝光。
3. **4–7 分钟：支付与幂等。** 使用固定用户下单，展示 Kitex 库存预占、二维码和本地沙箱付款页；连续两次确认同一令牌，说明支付回调事件唯一、Outbox 的 `order.created/order.paid` 各唯一、库存只最终扣减一次。
4. **7–9 分钟：商家与管理员隔离。** 登录商家 1101/1102，分别只能看到自己的店铺、商品和订单；用“一键管理员登录”展示橱窗、订单、对账和事件入口。
5. **9–12 分钟：架构取舍。** 说明商品读继续走 Product RPC、卡片快照和 L1/L2 缓存；库存写走 Kitex；支付后派生工作走 MySQL Outbox + RabbitMQ，安全、鉴权、幂等不与传输框架绑定。
6. **12–14 分钟：稳定性与观测。** 在 Grafana 展示五个面板，在 Jaeger 检索 Hertz/Order/Inventory 业务 Span；说明布隆过滤、负缓存、singleflight、Redis 分布式锁、SAGA 补偿和库存恢复任务。
7. **14–15 分钟：迁移证据。** 展示冻结的 Entry/Hertz 成对压测结果：真实响应更大的 Hertz 链路未出现性能回退；结论限定为本项目真实链路结果，不包装成框架微基准。

演示结束若产生订单，执行 `reset-demo --confirm-reset --profile interview --observability`；该命令先备份 MySQL，仅重置本项目 MySQL/Redis 卷并保留上传素材卷。随后运行 `verify-demo` 和发布就绪检查恢复固定现场。

## 验证基线

2026-07-30 最终发布就绪与浏览器验收：

- 从当前源码重新构建并运行 Auth、Product RPC、Order RPC、Inventory Kitex 和 Hertz 五个业务镜像；固定演示夹具为 `20260730_demo_fixture_v1`。
- 发布就绪脚本 34/34 通过：`/live`、`/ready`、系统健康、商城/管理员/商家页面、六个橱窗商品、十个商品/店铺素材、四个固定账号、两个商家隔离、Prometheus 四目标、Grafana 五面板、Jaeger 业务服务和 RabbitMQ 均正常。报告不会保存或打印登录令牌。
- Windows Chrome 实际完成普通用户登录、商家商品下单、二维码展示、本机沙箱付款、同一令牌重复确认和订单已支付回读；商品 201 库存只从 36 降到 35，预占状态为 `CONFIRMED`，支付回调事件只有 1 条，`order.created` 与 `order.paid` 各只有 1 条且均已发布。
- 商家 1101 只看到山岚烘焙两件商品，商家 1102 只看到北纬三十六两件商品和对应店铺资料；管理员一键登录、六槽首页橱窗、全部图片自然尺寸和中文文本均正常。
- Grafana 总览实际显示 Hertz 读取和 Inventory reserve/confirm 成功数据；Jaeger UI 可检索 Hertz 业务 API、Order 支付和 Inventory 库存命令 Span，探针、指标抓取和静态资源已从追踪噪声中排除。
- 验收发现并修复商城登录/注册标签未关联输入框的问题；组件回归测试和 Chrome 控制台复验均通过，无商城错误、警告或可访问性问题。
- 真实付款产生的数据已在备份后通过受控重置清除；重置后固定商品 201 库存恢复为 36，`verify-demo` 与 34/34 发布就绪检查再次通过。
- Go 全仓 `vet`、测试和六个服务构建通过；前端 38 个测试文件、61 个用例和三套单文件生产构建通过，提交产物解析检查通过。

2026-07-30 Hertz 容量与 SLO 证据链验证：

- 正式结果绑定提交 `f1be145d22fa3a6aa459c2c1902302f61ca2523c`，环境为 4 vCPU、约 7.76 GiB 内存的 Ubuntu WSL Docker Engine 和固定演示数据 `20260730_demo_fixture_v1`；冻结结果位于 `benchmarks/results/capacity-20260730.json`。
- 公开读混合链路按商品目录 70%、商品详情 20%、店铺详情 10% 执行 100/300/600 RPS 阶梯；最高已测档位实际 594.68 QPS，8,921/8,921 成功，p95 0.734 ms、p99 1.566 ms，未观察到容量边界。
- 订单完整生命周期按 2/5/10 RPS 执行真实登录、下单、Kitex 库存预占和取消补偿；最高已测档位 149/149 成功，p95 123.464 ms、p99 130.589 ms，未观察到容量边界。
- 本地沙箱支付正常样本与 RabbitMQ 暂停样本合计 12/12 成功；RabbitMQ 恢复后 Outbox 在 90 秒窗口内清空。40 次相同幂等键重放全部返回成功，数据库只保留一个业务订单。
- 实验后重复订单、重复支付回调、负库存、悬挂预占、未完成库存确认、Outbox 残留和 Redis/MySQL 库存桶差异均为 0；RabbitMQ 已恢复为 running/non-paused，演示数据重置校验通过，`flash-mall-uploads` 卷身份未改变。
- Hertz 新 RED 指标已由 Prometheus 实际抓取，四个服务目标均为 `up`；`promtool` 校验 8 条规则通过，Grafana API 可见五个自动配置面板，其中容量面板 UID 为 `flashmall-capacity-slo`。

2026-08-15 完整性能套件收口：

- 正式结果绑定提交 `ab99259bb847db10bbb00c45fef7407cd52657ff`，冻结在 `benchmarks/results/performance-20260815.json`；完整方法和解释边界见 `docs/PERFORMANCE_TESTING.md`。
- 600 RPS 预期公开读实际 599.99 QPS、p95 0.376 ms；公开读压力升至 3000 RPS 仍 100% 成功、实际 2999.93 QPS、p95 0.476 ms，当前只可陈述“至少达到 3000 RPS”。
- 订单 10 RPS 预期负载 100% 成功、p95 139.773 ms；25 RPS 压力档通过，40 RPS 首次失败。失败档主要延迟集中在 `create_order`，MySQL 活跃线程峰值 8，而 Hertz、Order RPC、Inventory Kitex 均未出现 CPU 饱和。
- 五分钟 500 RPS 读与 5 RPS 订单混合稳定性全部成功；Trace 默认采样率从 100% 降为 10% 后，Jaeger 内存增长由旧轮次的 406.5 MiB 降至 37.4 MiB，资源门禁通过。
- 正常支付、40 次同幂等键重放、RabbitMQ 暂停支付及恢复排空全部通过，最终全部业务不变量通过，演示数据和上传卷已恢复。

2026-07-30 可复现演示环境与桌面控制中心收尾验证：

- 数据库结构与演示数据已分别生成 `scripts/k8s/schema.sql` 和 `scripts/k8s/demo-seed.sql`；演示版本标记固定在全部订单、商品和认证数据写入之后，任一中间语句失败都不能把半成品标记为就绪。CI 同时校验九个固定商品均具备库存快照。
- Compose 默认启用版本化演示数据，旧环境只有在账号凭据、商家成员/店铺资料和 MySQL 库存事实均完整时才无损登记版本，否则要求显式重置；实测接管和正常停机再启动前后的商品、四个账号密码哈希和首页橱窗校验值完全一致。K8s 默认仅执行结构迁移，演示数据必须通过部署工作流的 `seed_demo_data` 显式开启。
- `reset-demo --confirm-reset --profile interview` 实测先生成权限为 `0600` 的非空 MySQL gzip 备份，只删除当前 Compose 项目的 MySQL/Redis 数据卷并恢复固定数据；写入 `flash-mall-uploads` 的验收哨兵跨完整重置保留，验收结束后已清除。
- 固定用户 `13800000001`、管理员 `13800000002`、商家 `13800001101` 和 `13800001102` 均通过最新 Auth 镜像登录；管理员看板可访问，两个商家账号分别只解析到商家 1101 和 1102。首页返回 6 个橱窗商品，商品图、商家 Logo 和店铺横幅全部返回 200。
- Windows Chrome 实际完成首页、商品详情、商家店面、管理员一键登录和商家密码登录；页面无白屏、坏图或控制台错误。三套入口内联 favicon，登录/注册字段显式声明浏览器自动填充语义，静态产物守卫会阻止这两项回退。
- Go 全仓 `vet`、测试和构建通过；前端 37 个测试文件、60 个用例和三套生产构建通过；桌面控制中心 38 个 Release 测试及构建通过；SQL 聚合、控制协议、持久化、可观测、Docker 上下文、Actionlint 和静态产物守卫全部通过。

2026-07-30 支付链路收尾验证：

- 支付渠道边界已从 Hertz 下沉到 Order RPC，Hertz 只创建支付意图、转发支付宝通知和展示二维码；本地沙箱与支付宝沙箱共用支付单、幂等入账、Outbox、库存确认、关单和退款状态机。
- RSA2 请求签名、同步响应验签、通知验签、分金额精确转换、上海时区请求时间、二维码预创建、查询、关单和退款均由带签名的模拟支付宝网关测试覆盖。
- Go 全仓 `vet`、测试和六个服务构建通过；前端 37 个测试文件、60 个用例和三套生产构建通过；数据库聚合、持久化、可观测、故障恢复、性能对比、Docker 构建上下文、工具链、Action 版本、冒烟配置和静态产物检查全部通过。
- 最终 Docker 链路订单 `final-payment-1785377746847` 完成登录、Kitex 预占、二维码创建、同一令牌重复付款、支付状态查询、用户退款申请和管理员审核；最终订单为 refunded，支付回调事件只有 1 条，Outbox 的 created/paid/refund requested/refund succeeded 均发布成功，库存日志各有 1 条 RESERVE/CONFIRM/RELEASE。
- 最终超时订单 `final-expiry-1785377873015` 人工推进到过期后，由恢复任务自动关闭订单和支付单，数据库状态为 `order=closed/payment=closed/provider_status=CLOSED`，库存释放成功且仅尝试一次。
- MySQL Compose 和 K8s 部署显式使用 `Asia/Shanghai` 与 `+08:00`，截止时间由 MySQL `NOW()` 计算；订单直达页 `/orders` 已补齐 Hertz SPA 路由，Chrome 刷新后显示本地时间 `2026-07-30 10:15:48`，当前视口没有坏图或控制台错误。
- 当前支付配置仍使用 `local_sandbox`；伪造支付宝通知实测返回 `failure`，RSA2 正向链路由带签名的模拟网关覆盖。
- 当前机器未保存支付宝沙箱商户密钥，因此没有向支付宝公网发起真实交易；注入 `deploy/.env.example` 中五项沙箱参数即可切换，真实密钥不得进入 Git。

2026-07-29 Go-zero Entry API 与 Hertz 对比基线：

- 对比脚本要求基线 worktree 内容树与 `origin/main` 完全一致，并在同一 Docker 网络、Product RPC、Redis、MySQL 和 Etcd 上交替执行两条 `/api/shop/catalog` 链路；临时 Entry 容器在退出时自动删除。
- 正式扩样为每端 7 轮、每轮 60,000 请求、并发 30、预热 2,000 请求；Entry 与 Hertz 最低成功率均为 100%。Hertz 响应包含 6 个商品、2,660 字节，Entry 包含 5 个商品、893 字节。
- Entry 的 QPS/p95 中位数为 10,262.11/6.33 ms；Hertz 为 11,863.90/5.26 ms。Hertz 在响应体约为 2.98 倍的情况下，QPS 中位数高 15.61%，p95 中位数低 16.90%。另一组每端 5 轮、每轮 30,000 请求的重复实验同样保持 Hertz 更快的方向。
- 扩样中的 QPS CV 已低于 10%，但两端 p95 CV 为 12.33%/10.82%，超过严格稳定门槛。因此结论限定为“迁移后的真实公开读链路未出现性能回退且方向可重复”，不把百分比包装成 Hertz 与 Go-zero 框架本身的隔离微基准。
- 可复现工具、固定元数据与汇总规则进入 CI；冻结结果保存在 `benchmarks/results/entry-hertz-20260729.json`。历史 Entry 源码没有为测试改写，专用 Dockerfile 只增加 BuildKit 依赖/编译缓存以缩短重复构建时间。

2026-07-27 故障注入与恢复演练完成以下验证：

- `verify-failure-recovery.sh` 必须显式传入 `--allow-disruption`，并校验容器属于当前 Compose 项目；退出 trap 会恢复所有暂停容器并删除唯一 `chaos.probe` Outbox 记录，CI 固化九项安全契约。
- 暂停 Inventory Kitex 和 Order RPC 后，对应 Prometheus `up` 变为 0；解除暂停后重新变为 1，Hertz 最终恢复就绪。
- 暂停 Redis、MySQL 后，Hertz 就绪接口均返回 503；解除暂停后在等待窗口内恢复 200，没有重启业务服务。
- RabbitMQ 暂停期间，隔离 Outbox 探针保持 pending 且 `attempt_count` 增加；RabbitMQ 恢复后同一事件进入 published，随后探针数据被删除。
- 演练结束后 Hertz、Inventory Kitex、Order RPC、Redis、MySQL、RabbitMQ 均为 running 且非 paused，残留 `chaos.probe` 记录为 0。

2026-07-27 Grafana 观测闭环完成以下验证：

- Prometheus 2.53 `promtool` 校验主配置和六条告警规则通过；Compose 配置与仓库观测契约检查通过，CI 会阻止面板、告警挂载、关键 PromQL 或可配置 Grafana 端口回退。
- 使用现有业务拓扑启动 observability profile，Prometheus 对 `hertz-gateway`、`order-rpc`、`product-rpc`、`inventory-kitex` 四个目标均报告 `up`，六条告警规则健康状态均为 `ok`。
- Grafana 11.1 自动装载“总览”“库存一致性”“支付与 Outbox”“缓存与 RPC”四个面板；通过 Prometheus HTTP API 实际编译执行全部 32 条面板 PromQL，均返回成功。
- Windows 上已有独立 Next.js 服务占用 3000 时，以 `FLASH_MALL_GRAFANA_PORT=3001` 启动成功；Prometheus 仍使用 9099，业务容器未重启。

2026-07-27 CI 修复与双入口集成基线：

- Fast CI [30236181296](https://github.com/mildred522/flash-mall/actions/runs/30236181296) 全绿：Go 全仓 `vet`、测试与六个服务二进制构建通过，桌面启动器测试/发布通过，数据库聚合、持久化与字符集、Docker 构建上下文、Go 工具链、GitHub Actions 运行时、集成冒烟配置及 Actionlint 守卫全部通过。
- Full Integration CI [30236336893](https://github.com/mildred522/flash-mall/actions/runs/30236336893) 全绿：旧 Go-zero Entry API 与新 Hertz + Kitex 两条端到端冒烟均从空白依赖环境完成，验证注册/登录、商品读取、下单、库存预占与最终扣减等核心链路。
- `entry-api`、`hertz-gateway`、`auth-api`、`product-rpc`、`order-rpc`、`inventory-kitex` 六个服务镜像均由 GitHub Buildx 成功构建；服务 Dockerfile 必须显式复制共享 `app/common`，该契约已进入 Fast CI。
- 工作流统一使用支持 Node 24 的官方 Action 大版本，Go 安装以 `go.mod` 的 `toolchain go1.24.11` 为唯一版本来源并以 `go.sum` 为缓存键；日常提交只跑受影响检查，完整双入口与镜像矩阵保留为手动/定时验证。

2026-07-27 商品存在性过滤与负缓存完成以下验证：

- `go test ./app/gateway/hertz/... -count=1` 与 `go vet ./app/gateway/hertz/...` 通过；过滤器测试覆盖固定哈希位置、幂等 Add、重建锁、失败不切 active、多实例共享、Redis 不可用 fail-open、负缓存生命周期和副本指标初始化。
- 在 Ubuntu WSL Docker Engine 中从当前源码重建并启动 `hertz-gateway`；`/api/system/health` 返回过滤器 `ready=true`、active generation 和 11 个历史商品 ID，普通商品 100 详情返回 200。
- 随机不存在商品 `9999999999` 由 Bitmap 在 0 ms 内返回既有 404，指标分别记录 `possible=1`、`absent=1`；已下架商品 106 第一次权威回源后写入约 30 秒负缓存，第二次请求记录 `hit=1`。
- 临时启动第二个 Hertz 容器连接同一 Redis 后，商品 100 仍返回 200、随机缺失商品 `9999999998` 返回 404，副本指标同时观测到 `possible`、`absent` 和 `ready=1`；验收后已删除临时容器。
- Redis 中只发布 active 指针、ready metadata 和对应 Bitmap；metadata 记录算法 `xxhash64-double-v1`、9,585,059 bit、7 次哈希与 11 个条目，未创建新数据库表或新网络服务。

2026-07-23 当前清理与素材持久化修复已完成以下验证：

- Hertz 全部包测试和 `go vet ./app/gateway/hertz/...` 通过；Docker MySQL 可用后，支付绑定与对账扫描两个集成用例也已纳入完整 Handler 测试并通过。
- Inventory Repository、Auth ServiceContext、Entry 静态入口、Handler 持久化架构守卫、支付签名与迁移目录回归通过。
- Go 全仓 `go test ./... -count=1` 通过；前端全部 37 个测试文件、59 个用例通过，shop、admin、merchant 三套生产构建通过。
- 数据库初始化七个模块与聚合 SQL 一致，持久化卷/utf8mb4 配置守卫和 `artifacts/web` 三套静态产物检查通过。
- 在 Ubuntu WSL Docker Engine 中从当前源码重新构建 `auth-api`、`product-rpc`、`order-rpc`、`inventory-kitex`、`entry-api` 和 `hertz-gateway` 六个镜像，默认 Hertz Compose 拓扑健康。
- 真实链路订单 `docker-e2e-1784777507` 使用同一请求 ID 重复下单只生成同一订单；库存从可用 `9999` 经预占变为 `9998`，支付确认后预占归零、总库存变为 `9998`。
- 同一支付令牌重复确认仍只产生 1 条支付回调事件、2 条 Outbox 事件和 2 条库存变更日志；两条 Outbox 均已发布，RabbitMQ 消费者已写入支付投影。
- 管理员和商家真实登录、管理看板、商家成员关系与店铺资料、商品目录及图片、商城/管理端/商家端/商品详情/店铺页面均返回正常；已删除的 `/api/gateway/health` 返回 404。
- 旧 worktree 的 4 个上传文件已无损迁入 `flash-mall-uploads`；Hertz 重建和重启后，内容寻址图片 `f7e96c…c856.png` 的 HTTP SHA-256 仍为 `f7e96c…c856`。
- 历史 40 条问号商品名快照全部先写入审计表再修复，修复后连续问号残留为 0；包含验收订单在内已有 55 条订单保存商品图片快照。
- 真实订单 `durable-assets-1784792738791` 保存了 `持久化验收商品🧥-1784792738713` 的完整 utf8mb4 字节和内容寻址图片 URL。
- Windows 本机 Google Chrome 对管理员商品、管理员订单和商城首页做真实渲染：商品与订单中文名可见，所有图片 `naturalWidth > 0`，无 4xx 响应或失败请求。
- 素材可靠性优化后再次完成 Go 全仓测试、Hertz `go vet`、前端 37 个测试文件/60 个用例和三套生产构建；版本化数据库修复已在现有 MySQL 卷登记且保留 40 条历史审计记录。
- 新 Hertz 镜像已在 Ubuntu WSL Docker Engine 重建并运行：`/live` 与 `/ready` 分离，后台审计报告 3 个引用、0 缺失、0 损坏、0 非法路径，六项 `flash_mall_upload_integrity_*` 指标可采集。
- 验收订单 `durable-assets-1784795832540` 验证了中文/emoji 快照和 WebP 内容哈希，随后通过用户取消链路进入 `closed`；商品 106 的原名称、图片和总库存均已恢复。
