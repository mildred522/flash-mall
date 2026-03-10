# Step3 报告：下单限流兜底（2026-02-24）

## 变更内容
- 在 order-api 增加基于令牌桶的限流中间件，超过速率直接返回 429。
- 新增限流配置项（QPS + Burst），可通过 ConfigMap 动态调整。

## 变更位置
- `app/order/api/internal/config/config.go`
- `app/order/api/internal/svc/serviceContext.go`
- `app/order/api/internal/middleware/ratelimitmiddleware.go`
- `app/order/api/internal/handler/routes.go`
- `app/order/api/etc/order-api.yaml`
- `k8s/apps/01-configmaps.yaml`

## 压测与采集
- 脚本：`./scripts/k8s/perf-collect.ps1 -Concurrency 20 -DurationSeconds 15 -Scenario step3-limit`
- 输出目录：`docs/perf/20260224-185945`

## 指标结果（本次）
- QPS：21.27
- Avg：908.96 ms
- P50 / P95 / P99：1040.06 / 1181.89 / 1187.47 ms
- 成功率：100%（Success 342 / Failed 0）

## 与上一步（Step2 分桶扣减）对比
- 上一步：`docs/perf/20260224-185716`
- 指标变化：
  - QPS：20.11 → 21.27（上升）
  - Avg：958.46 → 908.96（下降）
  - P95：1316.66 → 1181.89（下降）
  - 成功率：维持 100%

## 结论
- 当前 20 并发未触发限流，但限流中间件作为兜底，能在突发流量下快速失败并保护后端。
- 若需要体现“限流效果”，可将并发提升或将 QPS 降低进行对比验证。
