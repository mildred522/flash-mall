# Flash Mall 性能测试指南

## 测试目标

性能套件同时回答四个不同问题，不能用一次高并发请求代替：

| 类型 | 要回答的问题 | 当前全量配置 |
| --- | --- | --- |
| 基准 | 低负载延迟是否可重复 | 公开读 100 RPS、订单 2 RPS，各 3 轮 30 秒 |
| 负载 | 预期工作负载是否满足 SLO | 公开读 600 RPS、订单 10 RPS，各 60 秒 |
| 压力 | 容量拐点出现在哪里 | 公开读 800/1200/2000/3000 RPS；订单 15/25/40/60 RPS |
| 稳定性 | 持续流量下是否退化或增长资源 | 500 RPS 公开读与 5 RPS 订单并行 5 分钟 |

恢复阶段另行验证正常支付、同幂等键重放、RabbitMQ 暂停期间支付、Outbox 恢复排空和最终业务不变量。压力档失败用于发现边界，不单独判整套失败；基准、负载、稳定性、恢复、资源门禁或业务不变量失败都会阻断结果。

## 安全边界

- 只能向 `127.0.0.1`、`localhost` 或 `::1` 发起有副作用的测试。
- 必须同时传入 `--allow-mutation` 和 `--confirm-reset`。
- 套件开始和结束都会恢复固定演示夹具，上传素材卷不参与重置。
- 负载生成器运行在 Compose 网络内，避免 Windows 端口转发和 WSL 时钟调度污染吞吐结论。
- 每个负载容器带独立运行标签，退出时只清理本次测试产生的容器。

## 执行方法

在 Ubuntu WSL 的项目目录执行：

```bash
# 约几分钟，用于检查测试链路本身
scripts/perf/run-performance-suite.sh \
  --suite quick --allow-mutation --confirm-reset

# 约 20 分钟，生成可作为项目证据的完整结果
scripts/perf/run-performance-suite.sh \
  --suite full --allow-mutation --confirm-reset
```

结果保存在 `.runtime/performance/<运行时间>/`：

- `report.md`：适合人工阅读的阶段结果与瓶颈结论；
- `summary.json`：完整机器可读结果；
- `stages/*.json`：每一档的操作和业务步骤延迟；
- `resources-*.tsv/jsonl`：容器、MySQL、Redis 和主机资源时间序列；
- `profiles/<压力档>/`：Hertz、Order RPC、Product RPC、Inventory Kitex 的 CPU profile 与 top；
- `invariants.json`：重复订单、重复支付回调、负库存、悬挂预占、Outbox 和 Redis/MySQL 库存一致性结果。

只有通过的完整结果才能用 `freeze-performance-result.mjs` 固化到 `benchmarks/results`。

## 当前冻结结果

正式结果绑定提交 `ab99259bb847db10bbb00c45fef7407cd52657ff`，环境为 4 vCPU、约 7.76 GiB 的 Ubuntu WSL Docker Engine，夹具为 `20260730_demo_fixture_v1`。

- 公开读：600 RPS 预期负载实际 599.99 QPS，p95 0.376 ms；压力最高测到 3000 RPS，实际 2999.93 QPS、全部成功、p95 0.476 ms，尚未观察到失败边界。
- 订单生命周期：10 RPS 预期负载实际 9.99 QPS、全部成功、p95 139.773 ms；25 RPS 压力档通过，40 RPS 首次失败，边界位于 25–40 RPS。
- 40 RPS 订单档的 `create_order` 是主导步骤，整体 p95 2943.442 ms，MySQL `Threads_running` 峰值为 8；Hertz、Order RPC、Inventory Kitex 都没有 CPU 饱和，因此优先排查事务编排、数据库连接等待和 DTM 分支开销，而不是先扩网关。
- 5 分钟稳定性中，500 RPS 读与 5 RPS 订单均 100% 成功；Redis 无阻塞客户端，MySQL 无当前行锁等待，主机可用内存没有持续下降。
- 将 Trace 默认采样率从 100% 调整为 10% 后，Jaeger 同一稳定性阶段内存增长由旧轮次的 406.5 MiB 降至 37.4 MiB，低于 128 MiB 稳定性门禁。
- 支付、40 次同幂等键重放和 RabbitMQ 暂停恢复均通过；最终重复订单、重复支付回调、负库存、悬挂预占、未完成库存确认、Outbox 残留和 Redis/MySQL 库存差异全部为 0。

冻结证据为 `benchmarks/results/performance-20260815.json`。这些数字只描述当前提交、固定数据和本机 Compose 资源，不代表公网生产容量。

本轮订单容量拐点、统计口径修正、根因假设和优化路线记录在 `docs/PERFORMANCE_ANALYSIS_20260815.md`。

## 如何定位瓶颈

1. 先看该档是否达到目标 RPS、成功率、丢弃数和 p95/p99，区分负载生成器跟不上与服务退化。
2. 再看 `steps`，把完整请求拆为读取、创建订单、取消、创建支付和确认支付，找到尾延迟的业务阶段。
3. 对照同一阶段的容器 CPU/内存、MySQL 活跃线程与行锁、Redis 阻塞和主机内存。
4. 使用该压力档自动采集的 pprof 判断 CPU、锁竞争、网络等待或数据库调用占比。
5. 最后用 Jaeger 定位单请求跨度；Trace 用于解释瓶颈，不用 100% 采样制造新的瓶颈。
