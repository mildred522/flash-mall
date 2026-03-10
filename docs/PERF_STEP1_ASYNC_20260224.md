# Step1 报告：DTM 异步提交（2026-02-24）

## 变更内容
- 将 `DtmWaitResult` 从 `true` 改为 `false`，API 不再同步等待 DTM 事务结果。
- 目的：降低请求阻塞与尾延迟（代价是改为最终一致性）。

## 变更位置
- `app/order/api/etc/order-api.yaml`
- `k8s/apps/01-configmaps.yaml`

## 压测与采集
- 脚本：`./scripts/k8s/perf-collect.ps1 -Concurrency 20 -DurationSeconds 15 -Scenario step1-async`
- 输出目录：`docs/perf/20260224-182150`

## 指标结果（本次）
- QPS：21.49
- Avg：899.91 ms
- P50 / P95 / P99：1021.93 / 1159.82 / 1168.60 ms
- 成功率：100%（Success 346 / Failed 0）

## 与上一步（优化后基线）对比
- 上一步：`docs/perf/20260224-180956`
- 指标变化：
  - QPS：22.37 → 21.49（略降）
  - Avg：885.17 → 899.91（略升）
  - P95：1043.77 → 1159.82（略升）
  - 成功率：维持 100%

## 结论
- 异步提交没有显著提升吞吐或降低延迟，但消除了同步等待的耦合，适合“最终一致性 + 异步回查”的业务模型。
- 后续若要进一步提升，需要从库存热点与扣减冲突处继续优化（进入 Step2）。
