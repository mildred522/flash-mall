# K8s 手动部署步骤（一步一步）

## 0. 你更快的部分（我不代替）
- 集群创建（kind/minikube/云上）
- 安装 Ingress Controller、metrics-server
- 依赖组件用 Helm 装（比我写 YAML 更快）

## 1. 构建镜像
```powershell
./scripts/k8s/build-images.ps1 -Tag dev
```
> 如果你在 kind/minikube，需要把镜像加载进集群：
- kind: `kind load docker-image flash-mall/order-api:dev`（其他两个同理）
- minikube: `minikube image load flash-mall/order-api:dev`（其他两个同理）

## 2. 部署 K8s 资源（包含初始化 Job）
```powershell
./scripts/k8s/apply.ps1
```

## 3. 等待初始化 Job 完成（MySQL + Redis）
```powershell
kubectl -n flash-mall get jobs
kubectl -n flash-mall logs job/mysql-init
kubectl -n flash-mall logs job/redis-seed
```

## 3.1 验证 HPA（需要 metrics-server）
```powershell
kubectl -n flash-mall get hpa
```

## 3.2 HPA 触发演示（压测）
> 先确保已 `port-forward` 或有 Ingress 可访问。

终端 A（观察扩缩容）：
```powershell
kubectl -n flash-mall get hpa -w
```

终端 B（发压测流量）：
```powershell
./scripts/k8s/hpa-demo.ps1 -Concurrency 12 -DurationSeconds 120 -PrepareData
```

## 4. 访问服务
- 如果你没配 Ingress：
```powershell
./scripts/k8s/port-forward.ps1
```

## 5. 验证
```powershell
$body = @{user_id=1; product_id=100; amount=1; request_id="req-001"} | ConvertTo-Json -Compress
Invoke-RestMethod -Uri http://localhost:8888/api/order/create -Method Post -ContentType "application/json" -Body $body
```

## 5.1 一键重置库存（可选）
```powershell
./scripts/k8s/seed-stock.ps1 -ProductId 100 -TotalStock 10000 -Shards 4
# 强制覆盖已有库存：
./scripts/k8s/seed-stock.ps1 -ProductId 100 -TotalStock 10000 -Shards 4 -Force
```

## 5.2 幂等性验证（同 request_id 只生成一单）
```powershell
./scripts/k8s/verify-idempotency.ps1 -PrepareData
```

## 5.3 库存一致性校验（Redis 分片 ≈ DB 库存）
```powershell
./scripts/k8s/check-consistency.ps1 -ProductId 100 -Shards 4
```

## 5.4 SAGA 失败演练（触发补偿）
```powershell
./scripts/k8s/saga-failure-demo.ps1 -ProductId 100 -Shards 4 -RedisStock 10 -DbStock 0 -Amount 1
```

## 6. 可观测
- order-api metrics: `http://<pod-ip>:9090/metrics`
- order-rpc metrics: `http://<pod-ip>:9091/metrics`
- product-rpc metrics: `http://<pod-ip>:9092/metrics`

## 7. 性能基线（p50/p95/p99 + 报告）
```powershell
./scripts/k8s/run-benchmark.ps1 -Concurrency 20 -DurationSeconds 15 -PrepareData
```

## 7.1 一键采集（压测 + pprof + 日志）
```powershell
./scripts/k8s/perf-collect.ps1 -Concurrency 20 -DurationSeconds 15
```

---

## 附：如果你更高效的地方
- Helm 安装依赖组件（Redis/MySQL/etcd/DTM）比我手写 YAML 快
- 镜像构建/推送你本地或 CI 更顺
- 如果要重置初始化：删除 Job 并重建
  - `kubectl -n flash-mall delete job/mysql-init job/redis-seed`
  - `kubectl -n flash-mall apply -f k8s/jobs/`
