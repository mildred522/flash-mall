# Flash-Mall 项目优化发现报告（2026-03-07）

## 1. 当前状态理解（基于代码证据）

1. 下单主链路使用 DTM SAGA，订单号与事务 gid 已解耦，订单号由 Snowflake 生成。
2. request_id 幂等路径是 DB 查询 -> Redis 查询 -> Redis SetNX 抢占。
3. 已引入 RabbitMQ Outbox 增量架构，发布器支持单活 leader lock。
4. 已有 RabbitMQ consumer，采用手动 ack + Redis 去重。
5. 延迟关单仍使用 Redis ZSET + processing 队列 + retry + DLQ。

证据文件：
- `app/order/api/internal/logic/createorderlogic.go`
- `app/order/rpc/internal/job/outbox_publisher.go`
- `app/order/api/job/order_event_consumer.go`
- `app/order/api/job/closeorder.go`
- `app/order/api/internal/idgen/snowflake.go`
- `docs/PERF_EXECUTION_20260228.md`
- `docs/RABBITMQ_INCREMENTAL_EXECUTION_20260301.md`

## 2. 关键瓶颈与根因

1. 幂等 key 同时承担“锁”和“结果”语义，重复请求可能在首单处理中拿到占位值并提前返回。
2. 新请求先查 DB，再查 Redis，导致高并发时 DB 读放大。
3. Outbox claim 采用“先查再逐行更新”，在批量场景有 N+1 SQL 开销和竞争窗口。
4. 消费端当前只做去重与日志，没有“业务落地 sink”来展示强工程价值。
5. Snowflake 节点号用 hostname hash 自动推导，规模扩展时治理与碰撞风险上升。
6. 延迟任务仍以 Redis 实现，跨服务事件时间语义分散在两套系统（Rabbit + Redis）。

## 3. 发散优化选项（含收益/风险/验证/简历表达）

### 选项 A：拆分幂等锁键与结果键（P0 / S / 低风险）

Change summary：
- `order:req:{request_id}:lock` 仅表示处理中。
- `order:req:{request_id}:result` 仅保存最终 order_id。
- 重复请求命中 lock 时返回“处理中”，命中 result 时返回历史结果。

Mechanism：
- 避免“处理中占位值”被当成最终结果返回，幂等语义从二值提升为状态机。

Advantages：
- 正确性更清晰，减少并发边界争议。
- 面试时可讲清“processing/done”状态转换。

Risks：
- 需要处理异常退出导致 lock 泄漏。

Mitigation：
- lock TTL + watchdog 续约上限 + submit 失败主动释放。

Validation：
- 双并发同 request_id 压测，验证无重复下单且第二请求返回语义正确。

Resume value：
- 设计并落地 request_id 三态幂等机制（processing/done/expired），消除并发重复请求下的结果歧义。

### 选项 B：幂等查询改为 Redis-first + DB 回源（P0 / S / 低风险）

Change summary：
- 查询顺序从 DB-first 改为 Redis-first。
- 未命中 Redis 再查 DB，查到后回填 Redis result key。

Mechanism：
- 热路径优先走缓存，减少 DB 读放大。

Advantages：
- 在请求重放或网络抖动场景下降低 DB 压力。
- 对尾延迟改善更直接。

Risks：
- Redis 短时不稳定会带来回源尖峰。

Mitigation：
- 对 DB 查询失败做退避重试并加短负缓存。

Validation：
- 对比优化前后 DB QPS、P95/P99 与 hit ratio。

Resume value：
- 将幂等查重路径改造为 Redis-first，显著降低高并发下 DB 读放大并优化尾延迟。

### 选项 C：Outbox 批量认领优化（P1 / M / 中风险）

Change summary：
- 将逐行 claim 改为批量 claim（如单 SQL 批量更新 + RETURNING/临时表）。
- 增加自适应轮询退避，空轮询时扩大间隔。

Mechanism：
- 减少 N+1 更新和空扫描。

Advantages：
- 提升发布吞吐，降低 DB 压力。
- 多副本下竞争窗口更小。

Risks：
- SQL 方言兼容与迁移复杂度上升。

Mitigation：
- 保留旧逻辑开关，灰度发布并可快速回滚。

Validation：
- 监控 outbox claim 耗时、每秒发布量、DB CPU、pending backlog。

Resume value：
- 重构 Outbox claim 机制（批量认领 + 自适应轮询），提升消息发布吞吐并降低数据库开销。

### 选项 D：消费端增加 Inbox 业务落地（P1 / M / 中风险）

Change summary：
- 新增 inbox 表，message_id 唯一约束。
- 先写 inbox 再执行业务 side effect，成功后 ack。

Mechanism：
- 把“消息去重”升级为“业务幂等落地”，更接近 exactly-once 效果。

Advantages：
- 面试可讲“at-least-once + 业务幂等 = 工程可接受 exactly-once”。
- 便于审计与重放。

Risks：
- 需要明确 side effect 的事务边界。

Mitigation：
- 用本地事务封装 inbox+业务状态变更，失败走重试补偿。

Validation：
- 人为重复投递同 message_id，验证只产生一次业务效果。

Resume value：
- 在 RabbitMQ 消费链路引入 Inbox 幂等落地，将“消息去重”升级为“业务结果级幂等”。

### 选项 E：Snowflake 节点号治理（P1 / S / 低风险）

Change summary：
- 生产环境禁用 hostname hash 自动分配。
- 改为配置中心/StatefulSet ordinal/启动注册服务分配 node_id。

Mechanism：
- 从“概率唯一”升级为“治理唯一”。

Advantages：
- 降低节点扩容和迁移时的 ID 冲突风险。
- 便于面试解释分布式 ID 运维治理。

Risks：
- 增加部署流程约束。

Mitigation：
- 启动时校验 node_id 冲突并阻断进程。

Validation：
- 多实例重启演练，确认 node_id 稳定且无重复 ID。

Resume value：
- 完成 Snowflake 节点号治理改造，将 ID 唯一性从“算法保证”扩展到“运维流程保证”。

### 选项 F：延迟关单从 Redis 队列迁移到 MQ 延迟机制（P2 / L / 高风险，架构替换）

Change summary：
- 使用 RabbitMQ 延迟能力（插件或 TTL+DLX）替代 Redis ZSET 延迟队列。
- Redis 仅保留短期兜底或逐步下线。

Mechanism：
- 统一异步通道，降低双栈维护成本。

Advantages：
- 异步架构更统一，事件链路观测一致。
- 简化延迟任务的多套重试与迁移逻辑。

Risks：
- 迁移期存在双写/重复消费复杂度。

Mitigation：
- 三阶段迁移：双写验证 -> shadow 消费 -> 切流；保留 Redis 回退开关。

Validation：
- 对比迁移前后 close-order 成功率、延迟分布、DLQ 规模与恢复时间。

Resume value：
- 主导延迟关单架构从 Redis ZSET 演进到 MQ 延迟消息，完成双写灰度与回滚预案设计。

### 选项 G：SLO + 混沌演练体系（P2 / M / 中风险）

Change summary：
- 定义下单/补偿/投递 SLO 与 error budget。
- 固化故障注入脚本（DB 抖动、Rabbit 断连、Redis 超时、DTM 不可用）。

Mechanism：
- 从“功能可用”升级为“可证明可靠”。

Advantages：
- 面试问“数据可靠性怎么保证”时可给流程证据。
- 形成持续优化抓手。

Risks：
- 初期脚本维护成本较高。

Mitigation：
- 优先覆盖 P0 关键路径，按告警事件持续扩展。

Validation：
- 每次演练输出固定报告模板，沉淀 MTTR、成功率、p95/p99。

Resume value：
- 建立 SLO 与混沌演练闭环，实现可靠性指标可观测、可回归、可答辩。

## 4. 优先级路线图（P0 -> P2）

1. P0（本周）：选项 A + 选项 B。
2. P1（两周）：选项 C + 选项 D + 选项 E。
3. P2（迭代项）：选项 F + 选项 G。

执行顺序建议：
1. 幂等语义修正（A）
2. 幂等读路径优化（B）
3. Outbox 发布效率（C）
4. 消费落地增强（D）
5. ID 治理补强（E）
6. 架构替换试点（F）
7. 可靠性体系化（G）

## 5. 建议的首个执行步骤

首选执行选项 A（幂等锁键/结果键拆分）。

理由：
1. 改动面小，收益直接，能立即提升并发语义正确性。
2. 面试可讲清楚“为何不能把锁和值复用在一个 key”。
3. 可在现有压测脚本上快速验证并形成新一轮数据证据。
