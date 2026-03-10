# Flash-Mall 项目细节清单（可背诵）

## 01. 配置与基线
- 我做了什么：把 DTM 地址、Product/Order RPC 目标地址、订单超时从硬编码改为配置。
- 为什么：方便本地/容器/线上环境切换，避免改代码才能部署。
- 关键位置：
  - `app/order/api/internal/config/config.go`
  - `app/order/api/etc/order-api.yaml`
  - `app/order/api/internal/logic/createorderlogic.go`
- 一句话：我先消除环境耦合，为后续分布式事务与压测打稳定基线。

## 02. 订单闭环与 SAGA
- 我做了什么：新增 order-rpc（gRPC）服务，把 Redis 预扣、订单创建、库存扣减拆成三个 SAGA 分支；补了幂等验证脚本。
- 为什么：确保“预扣-落单-扣库”可补偿、可幂等，避免库存不一致。
- 关键位置：
  - `app/order/rpc/order.proto`
  - `app/order/rpc/order.go`
  - `app/order/rpc/internal/logic/*`
  - `app/order/api/internal/logic/createorderlogic.go`
  - `scripts/k8s/verify-idempotency.ps1`
- 一句话：我把核心下单链路改成 DTM SAGA，失败自动回滚，保证一致性。

## 03. 延迟关单可靠性
- 我做了什么：延迟队列从简单 ZRANGEBYSCORE+ZREM 改为 Lua 原子领取；增加 processing 队列、可见性超时、重试计数与 DLQ；关单时同步回滚 Redis 预扣库存；补充一致性校验与失败演练脚本。
- 为什么：避免并发重复处理与消费者崩溃丢单，保障至少一次语义。
- 关键位置：
  - `app/order/api/job/closeorder.go`
  - `app/order/api/order.go`
  - `app/order/api/internal/svc/servicecontext.go`
  - `scripts/k8s/check-consistency.ps1`
  - `scripts/k8s/saga-failure-demo.ps1`
- 一句话：我让延迟关单具备“原子领取 + 可恢复 + 可追踪”的工程级可靠性。

## 04. 库存分桶扣减
- 我做了什么：新增 `product_stock_bucket` 分桶表，按 `order_id` 哈希选择桶，扣减/回滚/归还统一走分桶；新增分桶初始化脚本。
- 为什么：降低热点行冲突，提升高并发下库存扣减成功率。
- 关键位置：
  - `app/product/rpc/internal/logic/deductLogic.go`
  - `app/product/rpc/internal/logic/deductRollbackLogic.go`
  - `app/product/rpc/internal/logic/revertstocklogic.go`
  - `scripts/k8s/seed-stock-bucket.ps1`
  - `scripts/k8s/init-db.sql`
- 一句话：我把库存从单行升级为分桶路由，降低热点冲突。

## 05. 库存对账与自动修复
- 我做了什么：新增库存对账脚本，比较分桶表/单行库存/Redis 预扣，并支持一键修复。
- 为什么：形成一致性闭环，异常可验证、可恢复。
- 关键位置：
  - `scripts/k8s/reconcile-stock.ps1`
- 一句话：我把一致性从“口述”升级为“可执行脚本”。

## 06. 下单限流兜底
- 我做了什么：在 order-api 增加令牌桶限流中间件，超限直接 429；限流参数配置化。
- 为什么：峰值流量下快速失败保护后端，避免雪崩。
- 关键位置：
  - `app/order/api/internal/middleware/ratelimitmiddleware.go`
  - `app/order/api/internal/handler/routes.go`
  - `app/order/api/internal/config/config.go`
  - `k8s/apps/01-configmaps.yaml`
- 一句话：我为下单入口加兜底限流，保障系统高峰期稳定性。

## 07. 可观测性与压测基线
- 我做了什么：新增 pprof 与 Prometheus metrics 端口；压测脚本输出 p50/p95/p99；提供一键性能基线脚本；优化库存扣减冲突后 503 降为 0。
- 为什么：性能问题有证据链，优化前后可量化对比。
- 关键位置：
  - `app/order/api/order.go`
  - `app/order/rpc/order.go`
  - `app/order/api/scripts/benchmark/benchmark_tool.go`
  - `scripts/k8s/run-benchmark.ps1`
- 一句话：我补齐可观测闭环，让性能优化有数据支撑。

## 08. 性能矩阵与容量模型
- 我做了什么：新增性能矩阵脚本，按“分桶数 × 并发”跑全量压测并生成报告。
- 为什么：形成容量模型，能解释瓶颈与收益区间。
- 关键位置：
  - `scripts/k8s/perf-matrix.ps1`
  - `scripts/k8s/perf-collect.ps1`
- 一句话：我让性能优化从“单点结论”变为“成体系证据”。

## 09. K8s 工程化落地（含多节点）
- 我做了什么：补齐 Deployment/Service/Ingress、HPA、ConfigMap，并提供依赖组件最小化 YAML；新增 MySQL/Redis 初始化 Job。
- 多节点升级：用 kind 建 1 控制面 + 2 工作节点；为核心服务加入反亲和、拓扑分布与 PDB，确保跨节点均衡与维护可用性。
- 演练脚本：节点维护/故障演练 + HPA 端到端演练。
- 为什么：展示调度/高可用/扩缩容能力，符合大厂工程化面试重点。
- 关键位置：
  - `k8s/kind/cluster-multi.yaml`
  - `scripts/k8s/kind-multi-deploy.ps1`
  - `k8s/apps/02-order-api.yaml`
  - `k8s/apps/03-order-rpc.yaml`
  - `k8s/apps/04-product-rpc.yaml`
  - `k8s/apps/06-pdb.yaml`
  - `scripts/k8s/ha-node-drain.ps1`
  - `scripts/k8s/hpa-e2e.ps1`
  - `docs/K8S_KIND_MULTI_NODE.md`
- 一句话：我把项目升级为多节点 K8s 部署，具备跨节点调度与可用性保障。

## 10. 可背诵总结（60 秒）
- 我把订单系统从“单机逻辑”升级为“可运维的分布式事务链路”：
  1) 配置外置，消除环境耦合；
  2) 下单改成 DTM SAGA（预扣/落单/扣库）保证一致性；
  3) 延迟关单改为原子领取 + 可见性超时 + 重试/死信，避免丢单；
  4) 库存分桶路由降低热点行冲突；
  5) 一致性对账 + 自动修复闭环；
  6) 入口限流兜底保护后端；
  7) 性能矩阵 + 容量模型 + p95/p99 证据链；
  8) 多节点 K8s + 反亲和 + PDB + 演练脚本，保证跨节点高可用。

## 11. 可靠压测流程（新增）
- 我做了什么：补齐“可答辩”压测链路，支持 warmup 稳态采样、多次重复、状态码拆分、环境快照留痕。
- 关键脚本：
  - `scripts/k8s/run-benchmark.ps1`
  - `scripts/k8s/run-benchmark-incluster.ps1`
  - `scripts/k8s/perf-collect.ps1`
  - `scripts/k8s/perf-reliable.ps1`
  - `app/order/api/scripts/benchmark/benchmark_tool.go`
- 实跑证据：`docs/PERF_EXECUTION_20260228.md`、`docs/PERF_RELIABLE_20260228-223701.md`
- 当前结论：已支持 `-LoadMode in-cluster`，发压从 host 转发升级为集群内 Job，结果更贴近真实流量路径。
- 下一优化点（P0）：`CreateOrder` 幂等查重路径改为 Redis-first，减少每请求 DB 读放大并继续优化 P95/P99。

## 12. RabbitMQ 增量架构（新增）
- 我做了什么：在不替换现有 Redis 延迟关单链路的前提下，引入 RabbitMQ 事件总线；在 order-rpc 落地 Outbox Pattern（同事务写订单+事件），后台发布器异步投递；在 order-api 增加 RabbitMQ consumer（手动 ack + Redis 去重 + Prometheus 指标）。
- 为什么：补齐“本地事务与消息投递一致性”能力，展示事件驱动架构与可靠消费的工程实践。
- 关键位置：
  - `app/order/rpc/internal/logic/createOrderLogic.go`
  - `app/order/rpc/internal/job/outbox_publisher.go`
  - `app/order/rpc/internal/job/rabbit_publisher.go`
  - `app/order/api/job/order_event_consumer.go`
  - `app/order/api/internal/metrics/metrics.go`
  - `scripts/k8s/alter-orders.sql`
  - `k8s/deps/rabbitmq.yaml`
  - `docs/RABBITMQ_INCREMENTAL_EXECUTION_20260301.md`
- 一句话：我把“下单主链路”升级为“事务一致 + 事件可扩展”的增量架构，能给面试官完整讲清设计、落地与证据链。
- P0 已完成：Outbox 单活发布（Redis leader lock），避免多副本重复轮询竞争；压测后 outbox 全量 `published`，无 `publishing` 卡死。
- 下一优化点（P1）：Outbox 自适应轮询退避（空轮询升间隔、来流量降间隔），继续降低 DB 空扫描成本。

## 13. request_id 幂等语义修正（新增）
- 我做了什么：将 `request_id` Redis key 从“单 key 复用锁+结果”改为双 key（`lock` / `result`）；并将 `gid/orderID` 生成延后到抢占锁成功后，避免重复请求消耗事务号与订单号。
- 为什么：修复并发场景下“处理中占位值被误当最终结果”的语义风险，明确区分“处理中”与“已完成”状态。
- 关键位置：
  - `app/order/api/internal/logic/createorderlogic.go`
  - `docs/IDEMPOTENCY_KEY_SPLIT_EXECUTION_20260307.md`
- 一句话：我把 request_id 幂等从“单键复用”升级为“锁键/结果键分离”，在并发重试下保证返回语义正确且可解释。

## 14. JWT 鉴权接入（新增）
- 我做了什么：新增登录签发 JWT 接口；将下单接口改为 JWT 保护路由；下单时从 JWT claim 读取 `user_id` 作为身份来源，忽略 body 里的伪造用户字段。
- 为什么：补齐认证授权基础能力，避免匿名请求与越权下单，提升项目在面试中的完整性。
- 关键位置：
  - `app/order/api/internal/handler/routes.go`
  - `app/order/api/internal/handler/loginhandler.go`
  - `app/order/api/internal/logic/loginlogic.go`
  - `app/order/api/internal/handler/createorderhandler.go`
  - `app/order/api/internal/config/config.go`
  - `app/order/api/etc/order-api.yaml`
  - `docs/JWT_EXECUTION_20260307.md`
- 一句话：我为订单服务接入 JWT 认证链路（签发+鉴权+身份注入），将下单身份从“前端传参”升级为“服务端可信声明”。 

## 15. 前端控制台适配（新增）
- 我做了什么：在 `order-api` 内嵌双页面前端（无需独立前端工程）：`/shop` 用于正常购物演示，`/debug` 用于链路调试与性能观测。
- 为什么：让项目从“接口可调”升级为“可演示、可观测、可答辩”，面试时可现场展示完整链路。
- 关键位置：
  - `app/order/api/internal/handler/webuihandler.go`
  - `app/order/api/internal/handler/web/home.html`
  - `app/order/api/internal/handler/web/shop.html`
  - `app/order/api/internal/handler/web/debug.html`
  - `app/order/api/internal/handler/systemhealthhandler.go`
  - `app/order/api/internal/logic/systemhealthlogic.go`
  - `app/order/api/internal/handler/routes.go`
  - `docs/WEB_UI_EXECUTION_20260307.md`
- 一句话：我把订单服务前端拆分为“商城页 + 调试页”双视图，分别覆盖业务演示与工程观测，支持面试现场完整走查。

## 16. 一键启动脚本（新增）
- 我做了什么：新增本地一键启动脚本，自动拉起 Docker 依赖（etcd/mysql/redis/dtm/rabbitmq）、初始化数据库、预置 Redis 分片库存、后台启动 `product-rpc/order-rpc/order-api` 并打开前端控制台。
- 为什么：降低项目演示门槛，避免面试现场手动启动多进程导致的失败风险。
- 关键位置：
  - `scripts/local/start-all.ps1`
  - `scripts/local/stop-all.ps1`
  - `deploy/docker-compose.yml`
  - `docs/LOCAL_ONE_CLICK_START_20260307.md`
- 一句话：我将分布式项目的本地启动流程产品化为一键脚本，支持“依赖拉起 + 数据初始化 + 服务编排 + 可视化入口”全自动执行。
