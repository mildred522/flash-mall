# Flash-Mall 性能优化报告（2026-02-24）

## 1. 现状基线（优化前）
- 采集脚本：`./scripts/k8s/perf-collect.ps1 -Concurrency 20 -DurationSeconds 15`
- 输出目录：`docs/perf/20260224-175608`
- 关键指标（bench_report.json）：
  - QPS: 21.71
  - Avg: 906.50 ms
  - P50 / P95 / P99: 868.12 / 1619.93 / 3028.62 ms
  - 成功率: 62.94%（Success 214 / Failed 126）

## 2. 根因定位
- order-api 日志出现大量：
  - `submit SAGA failed: ... product-rpc:8080/product.Product/Deduct return failed: 库存扣减冲突，请重试`
- DTM 日志出现大量：
  - `Deduct return failed: 库存扣减冲突，请重试`
- pprof 采样：
  - order-api CPU samples 很少（10ms），非 CPU 计算瓶颈
  - order-rpc CPU samples 为 0（请求未形成 CPU 压力）
- 结论：
  - 503 的主要原因是 **product-rpc 的库存扣减冲突**（乐观锁版本号冲突），触发 DTM 中止事务并返回 503。

统计（优化前）：
- `order-api` 中 “submit SAGA failed” 次数：81
- `dtm` 中 “Deduct return failed” 次数：261

## 3. 优化措施
- 目标：降低扣减冲突导致的 503
- 改动：将“SELECT + version 乐观锁重试”改为“单条原子 UPDATE 扣减库存”
- 位置：`app/product/rpc/internal/logic/deductLogic.go`
- 变更点：
  - 由 `SELECT stock, version` + `UPDATE ... AND version = ?` 改为：
    - `UPDATE product SET stock = stock - ?, version = version + 1 WHERE id = ? AND stock >= ?`
  - 若影响行数为 0，再判断是否为“商品不存在”或“库存不足”。

## 4. 优化后验证
- 采集脚本：同上
- 输出目录：`docs/perf/20260224-180956`
- 关键指标（bench_report.json）：
  - QPS: 22.37
  - Avg: 885.17 ms
  - P50 / P95 / P99: 878.16 / 1043.77 / 1129.49 ms
  - 成功率: 100%（Success 345 / Failed 0）

## 5. 指标对比（优化前 → 优化后）
- 成功率：62.94% → 100%
- P95：1619.93 ms → 1043.77 ms（约 -35.6%）
- P99：3028.62 ms → 1129.49 ms（约 -62.7%）
- QPS：21.71 → 22.37（约 +3.1%）

## 6. 结论
- 主要瓶颈不是 CPU，而是 **库存扣减冲突导致的 DTM 事务中止**。
- 通过原子更新减少冲突后，503 消失，延迟显著下降，吞吐小幅提升。

## 7. 下一步优化建议
1) 增加商品库存扣减的热点监控（Redis/DB 指标）。
2) 引入更精细的库存分桶或分库策略，降低单行热度。
3) 对 Deduct 增加限流/熔断兜底策略，避免高峰期异常放大。
4) 扩展压测场景，形成“并发-延迟曲线”与容量上限。
