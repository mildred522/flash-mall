# Step2 报告：库存分桶扣减（2026-02-24）

## 变更内容
- 将库存扣减从单行 `product` 表改为 `product_stock_bucket` 分桶表，按 `order_id` 哈希选择桶。
- 回滚/归还库存走同样分桶路径，保证补偿一致性。
- 新增分桶初始化脚本，压测/演练可一键准备数据。

## 变更位置
- `app/product/rpc/product.proto`
- `app/product/rpc/internal/logic/deductLogic.go`
- `app/product/rpc/internal/logic/deductRollbackLogic.go`
- `app/product/rpc/internal/logic/revertstocklogic.go`
- `scripts/k8s/init-db.sql`
- `scripts/k8s/seed-stock-bucket.ps1`
- `scripts/k8s/run-benchmark.ps1`
- `scripts/k8s/check-consistency.ps1`

## 压测与采集
- 脚本：`./scripts/k8s/perf-collect.ps1 -Concurrency 20 -DurationSeconds 15 -Scenario step2-bucket`
- 输出目录：`docs/perf/20260224-185716`

## 指标结果（本次）
- QPS：20.11
- Avg：958.46 ms
- P50 / P95 / P99：1091.95 / 1316.66 / 1340.79 ms
- 成功率：100%（Success 326 / Failed 0）

## 与上一步（Step1 异步提交）对比
- 上一步：`docs/perf/20260224-182150`
- 指标变化：
  - QPS：21.49 → 20.11（下降）
  - Avg：899.91 → 958.46（上升）
  - P95：1159.82 → 1316.66（上升）
  - 成功率：维持 100%

## 结论
- 分桶的核心价值是减少热点行冲突，在更高并发/更大库存冲突场景下收益更明显。
- 当前 20 并发下，分桶引入了额外路由与表操作开销，指标略有上升；后续可提高并发或桶数验证收益。
