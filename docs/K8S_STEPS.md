# K8s 手动部署步骤（一步一步）

## 0. 运行位置
当前推荐在 WSL 原生工作区执行 K8s 部署：

```bash
cd /home/mildred/code/flash-mall
```

Windows 只作为访问端，通过 `http://127.0.0.1:8888` 访问 port-forward 后的
entry-api。不要把 WSL `172.x.x.x` 地址写死。

## 0. 你更快的部分（我不代替）
- 集群创建（kind/minikube/云上）。本地演示优先使用 WSL 内的 kind。
- 安装 Ingress Controller、metrics-server
- 依赖组件用 Helm 装（比我写 YAML 更快）

## 1. 构建镜像
WSL：

```bash
scripts/k8s/build-images.sh dev
```

Windows 兼容入口：

```powershell
./scripts/k8s/build-images.ps1 -Tag dev
```
> 如果你在 kind/minikube，需要把镜像加载进集群：
- kind: `kind load docker-image flash-mall/entry-api:dev`（其他两个同理）
- minikube: `minikube image load flash-mall/entry-api:dev`（其他两个同理）

## 2. 部署 K8s 资源（包含初始化 Job）
日常 WSL 一键路径：

```bash
scripts/k8s/dev-up.sh
```

默认是本地单节点 profile。演示多副本调度时：

```bash
scripts/k8s/dev-up.sh --profile demo
```

如果刚改过 Go 服务代码：

```bash
scripts/k8s/dev-up.sh --rebuild-images
```

只应用 manifests 到当前集群：

WSL：

```bash
scripts/k8s/apply.sh
```

Windows 兼容入口：

```powershell
./scripts/k8s/apply.ps1
```

## 3. 等待初始化 Job 完成（MySQL + Redis）
WSL：

```bash
scripts/k8s/wait-ready.sh
```

WSL/Docker 重启后的恢复入口：

```bash
scripts/k8s/dev-restart.sh
```

手工查看：

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
WSL：

```bash
scripts/k8s/restore-port-forward.sh
```

Windows 兼容入口：

```powershell
./scripts/k8s/port-forward.ps1
```

## 5. 验证
快速查看 Pod、Service、健康接口和 Redis 库存种子：

```bash
scripts/k8s/health.sh
```

下单接口手工验证仍可按需执行：

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
- entry-api metrics: `http://<pod-ip>:9090/metrics`
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
- 本地 kind 环境停止：
  - `scripts/k8s/dev-down.sh`
- 只删除当前集群里的项目 namespace：
  - `scripts/k8s/dev-down.sh --namespace-only`
- 如果要重置初始化：删除 Job 并重建
  - `kubectl -n flash-mall delete job/mysql-init job/redis-seed`
  - `kubectl -n flash-mall apply -f k8s/jobs/`
