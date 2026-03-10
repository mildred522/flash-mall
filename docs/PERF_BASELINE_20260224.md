# Flash-Mall 性能与一致性基线报告（2026-02-24）

## 1. 测试环境
- 运行方式：Kubernetes（kind）
- 入口方式：`kubectl port-forward svc/order-api 8888:8888`
- 依赖组件：MySQL / Redis / etcd / DTM 均已就绪

## 2. 验证项与结果

### 2.1 幂等性验证（同 request_id 只生成一单）
- 脚本：`./scripts/k8s/verify-idempotency.ps1 -PrepareData`
- 结果：同一 request_id 重放只生成一条订单
- 证据：
  - `order_id = hYB4cRDSiTEzyhfFsGjiVJ`
  - DB 查询结果：
    - `id = hYB4cRDSiTEzyhfFsGjiVJ`
    - `status = 0`
    - `request_id = idem-7db0565b4ba5445180e65b61f32fcf10`

### 2.2 库存一致性校验（Redis 分片 ≈ DB 库存）
- 脚本：`./scripts/k8s/check-consistency.ps1 -ProductId 100 -Shards 4`
- 结果：一致
- 证据：`db_stock=9999 redis_sum=9999 missing_keys=0`

### 2.3 SAGA 失败补偿演练（预期失败 + 结果回滚）
- 脚本：`./scripts/k8s/saga-failure-demo.ps1 -ProductId 100 -Shards 4 -RedisStock 10 -DbStock 0 -Amount 1`
- 结果：接口返回 503（预期失败），订单状态落为已关闭，Redis 预扣库存回滚成功
- 证据：
  - `status = 2`
  - `redis_sum=10 (expect=10)`
  - `db_stock=0`

## 3. 性能基线（本次压测）
- 脚本：`./scripts/k8s/run-benchmark.ps1 -Concurrency 20 -DurationSeconds 15 -PrepareData`
- 请求量：307
- 成功 / 失败：221 / 86
- QPS：19.68
- Avg：1003.87 ms
- P50 / P95 / P99：829.83 ms / 1509.43 ms / 1736.51 ms
- 报告文件：`bench_report.json`

## 4. 观察与结论
- 幂等链路已闭环（同 request_id 只生成一单）。
- Redis 分片库存与 DB 库存保持一致（无漂移）。
- SAGA 失败补偿可复现且正确回滚（订单关闭 + 预扣回补）。
- 压测存在 503，表明当前链路在该并发下触发限流/熔断或上游不可用，需要进一步定位。

## 5. 下一步优化方向（待执行）
1) **定位 503 根因**：区分是 API 自身限流、DTM 超时、RPC 失败还是 Redis/DB 资源瓶颈。
2) **采集 pprof 与 metrics**：在压测期间抓 CPU/heap/profile，锁定热点。
3) **压测参数分层**：逐步提高并发，形成 P50/P95/QPS 曲线与拐点。
4) **针对热点优化**：减少慢路径、降低锁竞争、优化 Redis/DB 访问。
5) **复测验证收益**：形成“优化前/优化后”对比数据。
