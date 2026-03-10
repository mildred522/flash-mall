# 性能测试实跑记录（2026-02-28）

## 本次目标
- 按“可答辩流程”实际跑通性能测试。
- 产出可复核证据（原始数据 + 汇总结论）。
- 基于结果找下一步优化点。

## 执行命令

```powershell
./scripts/k8s/perf-reliable.ps1 `
  -Namespace flash-mall `
  -Runs 2 `
  -Concurrency 40 `
  -WarmupSeconds 5 `
  -DurationSeconds 20 `
  -CooldownSeconds 5 `
  -Scenario incluster-baseline `
  -Shards 4 `
  -ProductId 100 `
  -TotalStock 10000 `
  -LoadMode in-cluster
```

## 原始结果位置
- 汇总报告：`docs/PERF_RELIABLE_20260228-223701.md`
- 汇总 JSON：`docs/perf/reliable-20260228-223701/summary.json`
- 单次证据目录：
  - `docs/perf/reliable-20260228-223701/run-01`
  - `docs/perf/reliable-20260228-223701/run-02`

## 关键数据（本次实跑）
- Run-01：QPS 26.60，P95 1882.03ms，P99 1969.37ms，成功率 100%
- Run-02：QPS 25.85，P95 1960.66ms，P99 2002.83ms，成功率 100%
- 中位数：QPS 25.85，P95 1882.03ms，P99 1969.37ms，429/503 均为 0
- 波动：QPS CV=0.0202，P95 CV=0.0289（稳定）

## 流程状态
- ✅ 多次重复、预热窗口、稳态采样跑通。
- ✅ 自动归档日志、k8s 快照、heap/cpu profile（order-api/order-rpc）。
- ⚠️ product-rpc pprof 端口未就绪（已自动降级跳过，不阻断流程）。

## 本次定位出的“下一个优化点”（优先级 P0）

### 优化点：`request_id` 幂等查重改为 Redis-first（减少每请求 DB 查询）

证据：
1. 压测入口已升级为 `in-cluster`（`k6` Job），排除了 `port-forward` 单点偏差后，吞吐仍主要受业务链路耗时影响。
2. `CreateOrder` 当前路径里会先查 DB `FindOneByRequestId`，再查 Redis（`app/order/api/internal/logic/createorderlogic.go`），高并发下会放大 MySQL 读压。
3. 该查询对“新 request_id”命中率通常很低，属于可优化热路径。

预期收益：
- 降低 order-api 每请求 DB 读次数，减轻连接池与 SQL 往返开销。
- 降低高并发下尾延迟，改善 P95/P99 稳定性。
- 为后续提升并发上限留出数据库余量。

实施建议：
1. 幂等查询顺序改为 Redis -> DB（仅 Redis miss 再查 DB）。
2. 为 `request_id` 映射增加短 TTL 与负缓存策略，避免重复 miss 反复打 DB。
3. 压测对比：同参数下观测 SQL 查询次数、P95/P99 与 QPS 变化。

## 次级优化点（P1）
- 为 `product-rpc` 增加 pprof 暴露端口，补齐三段链路（order-api/order-rpc/product-rpc）CPU/heap 证据。
- 安装 metrics-server，补齐 `kubectl top` 与 HPA 观测闭环。

## 面试可复述结论（30 秒）
- 我把发压链路升级成集群内 k6 Job，并按固定参数做了 2 轮稳态压测，结果稳定（QPS/P95 的 CV 都很低）。
- 数据证据已归档到单次 run 目录（bench、k6、pprof、日志、k8s 快照），可复核可回放。
- 下一步我会优化 `request_id` 的幂等查重顺序（Redis-first），目标是减少 DB 读放大并继续压低 P95/P99。
