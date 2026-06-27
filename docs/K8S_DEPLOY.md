# K8s 部署指南（最小可用 + 高含金量）

## 目标
- 用 K8s 标准化部署 auth-api / entry-api / order-rpc / product-rpc。
- 对齐大厂面试热点：发布、弹性、稳定性、可观测、配置治理。

## WSL 优先路径
当前本地开发主链路是 WSL 原生工作区：

```bash
cd /home/mildred/code/flash-mall
```

K8s 也建议部署在 WSL 内的 Docker Engine/kind 上，不走 Windows
Docker Desktop 主链路。Windows 只作为浏览器访问端，访问
`http://127.0.0.1:8888`。

常用入口：

```bash
scripts/k8s/dev-up.sh
scripts/k8s/restore-port-forward.sh
scripts/k8s/health.sh
scripts/k8s/dev-restart.sh
scripts/k8s/dev-down.sh
```

`dev-up.sh` 默认使用 `local` profile：业务服务压到 1 副本，适合 WSL
单节点 kind 日常开发。需要保留 2 副本、HPA、PDB 等演示语义时使用：

```bash
scripts/k8s/dev-up.sh --profile demo
```

如果刚改过 Go 服务代码：

```bash
scripts/k8s/dev-up.sh --rebuild-images
```

WSL 或 Docker 重启后，如果 Pod 出现 `Unknown` 或 port-forward 丢失：

```bash
scripts/k8s/dev-restart.sh
```

如果本地没有 `k8s/examples/runtime-secrets.yaml`，`scripts/k8s/apply.sh`
会创建一套开发用 Secret。生产或正式演示环境仍应基于
`runtime-secrets.example.yaml` 填写真实 Secret 文件，并且不要提交到 Git。

## 目录结构
```
k8s/
  00-namespace.yaml
  apps/
    01-configmaps.yaml
    02-entry-api.yaml
    03-order-rpc.yaml
    04-product-rpc.yaml
    05-ingress.yaml
    06-pdb.yaml
    07-auth-api.yaml
  deps/
    redis.yaml
    mysql.yaml
    etcd.yaml
    dtm.yaml
  jobs/
    01-mysql-init.yaml
    02-redis-seed.yaml
  examples/
    runtime-secrets.example.yaml
```

## 依赖说明（我这边也可以帮，但你更快）
- **Redis/MySQL/etcd/DTM** 这些组件你用 Helm 装更快、更稳：
  - `bitnami/redis`
  - `bitnami/mysql`
  - `bitnami/etcd`
  - `dtm-labs/dtm`
- 如果你不想装 Helm，我也提供了 `k8s/deps/*.yaml` 的最小演示版（非生产）。
- **HPA 依赖 metrics-server**（kind/minikube 没装的话 HPA 会显示 Unknown）。

## 镜像准备
你需要先构建 4 个镜像（与 K8s 配置保持一致）：
- `flash-mall/auth-api:dev`
- `flash-mall/entry-api:dev`
- `flash-mall/order-rpc:dev`
- `flash-mall/product-rpc:dev`

WSL 内构建：

```bash
scripts/k8s/build-images.sh dev
```

PowerShell 脚本仍保留给 Windows 兼容路径：

```powershell
./scripts/k8s/build-images.ps1 -Tag dev
```

## 一键部署（最小可用）
```bash
kubectl apply -f k8s/00-namespace.yaml
# 基于 k8s/examples/runtime-secrets.example.yaml 创建真实 Secret
kubectl apply -f k8s/examples/runtime-secrets.yaml
kubectl apply -f k8s/deps/   # 如果不用 Helm
kubectl -n flash-mall create configmap mysql-init-sql --from-file=init-db.sql=scripts/k8s/init-db.sql --dry-run=client -o yaml | kubectl apply -f -
kubectl apply -f k8s/jobs/   # 初始化 MySQL/Redis
kubectl apply -f k8s/apps/
```

## 多节点（kind）方案
如果你需要展示“跨节点调度/容错/反亲和”等面试点，使用多节点方案：
- 方案文档：`docs/K8S_KIND_MULTI_NODE.md`
- WSL 一键脚本：`scripts/k8s/dev-up.sh`
- WSL 底层脚本：`scripts/k8s/kind-deploy.sh`
- Windows 兼容脚本：`./scripts/k8s/kind-multi-deploy.ps1`

## 访问方式
- WSL/kind 本地推荐：`scripts/k8s/dev-up.sh` 默认会恢复端口转发；如需手动恢复，运行 `scripts/k8s/restore-port-forward.sh`。然后从 Windows
  浏览器访问 `http://127.0.0.1:8888`
- Ingress 入口：`http://flash-mall.local`
- Metrics：
  - entry-api：`/metrics` (9090)
  - order-rpc：`/metrics` (9091)
  - product-rpc：`/metrics` (9092)

## 面试可背诵版
“我把 Flash-Mall 做成 K8s 标准化部署：
- 用 Deployment/Service/Ingress 完成发布与流量入口；
- 加上 Requests/Limits 与 Readiness/Liveness 提升稳定性；
- HPA 实现自动扩缩容；
- ConfigMap/Secret 做配置治理；
- Prometheus/pprof 输出指标，形成性能与稳定性证据链。”

## 注意事项
- MySQL/Redis 初始化已由 `k8s/jobs/*.yaml` 自动化；如需重置可删除 Job 后重建：
  - `kubectl -n flash-mall delete job/mysql-init job/redis-seed`
  - `kubectl -n flash-mall apply -f k8s/jobs/`
- `k8s/examples/runtime-secrets.example.yaml` 只放占位值；真实 Secret 文件不要提交到 Git。
