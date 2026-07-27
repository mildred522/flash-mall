# Flash Mall 当前项目状态

> 本文件是仓库内唯一有效的项目状态文档。内容以当前代码、配置和部署清单为准；历史方案、执行计划与聊天日志都不是开发依据。

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
- 支付回调使用业务订单号、支付单号和幂等状态机防止重复入账。
- 支付成功状态与 Outbox 事件在同一 MySQL 事务提交，消息发布失败由后台发布器重试。
- 取消、关闭、发货、确认收货等写命令由 Hertz 经 Go-zero Order RPC 执行，Hertz 不直接更新订单状态。

### 分层缓存

- 商品、店铺和首页橱窗使用统一缓存协调器。
- L1 是进程内短 TTL 缓存；L2 是 Redis，支持软/硬 TTL、抖动、singleflight、过期回源和原点失败时的陈旧值兜底。
- 商品、店铺、快照和橱窗变更执行精确键与前缀失效，并通过 Redis Pub/Sub 清理其他 Hertz 实例的 L1。
- 商品详情只在 L1/L2 未命中并进入 singleflight loader 后检查短期负缓存和 Redis Bitmap 布隆过滤器；ready generation 明确判定不存在时不访问 MySQL 或 Product RPC，Redis 异常、元数据异常和未就绪状态一律 fail-open。
- 商品过滤器保存所有历史商品 ID，不承担上下架、店铺状态、库存或权限判断。默认按 100 万商品、1% 假阳性率配置为 9,585,059 bit、7 次双哈希，负缓存基础 TTL 为 30 秒并带稳定抖动。
- 多 Hertz 副本共享版本化 active generation；新 generation 完整构建后才通过 Lua 切换，重建失败保留旧版本。新商品创建、库存种子重试和既有商品上架均先幂等写入过滤器，再允许公开；商品变更同时精确清除负缓存。

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
./scripts/local/start-compose-all.sh

# 查看健康状态
./scripts/local/health-compose.sh

# 只重建一个服务
./scripts/local/rebuild-compose-service.sh hertz-gateway

# 首次升级到命名卷时，无损迁移一个或多个旧上传目录；重复执行安全
./scripts/local/migrate-uploads-to-volume.sh /path/to/old/.runtime/uploads

# 用真实管理员/用户链路验证哈希上传、中文/emoji 订单快照和图片读取
node scripts/local/verify-durable-assets.mjs --allow-mutation /path/to/image.png 106
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
- 数据库初始化源码已按 bootstrap、订单、商品 schema、商品种子、历史数据修复、Auth schema、Auth 种子拆成 `scripts/k8s/sql` 七个模块；`scripts/k8s/init-db.sql` 由生成器聚合，现有 Docker/K8s 入口保持不变，CI 校验聚合物一致性。
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

本轮代码清理已收口。后续不再围绕已经完成的分层重复重构，优先转入以下产品与工程验证：

1. 用 Grafana 固化库存命令、支付、Outbox、缓存和 RPC 的延迟、成功率与一致性面板。
2. 对库存 Kitex、订单 RPC、Redis、RabbitMQ 和 MySQL 做故障注入，验证超时、补偿、重试、幂等及恢复任务。
3. 在固定数据集和运行拓扑下补充 Go-zero Entry API 与 Hertz 网关的性能对比，形成可复现的面试叙事。
4. 继续完善支付、退款、商家经营和首页推荐等业务能力；只有发现明确边界泄漏时才安排新的重构。

## 验证基线

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
