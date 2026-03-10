# K8s 部署指南（最小可用 + 高含金量）

## 目标
- 用 K8s 标准化部署 order-api / order-rpc / product-rpc。
- 对齐大厂面试热点：发布、弹性、稳定性、可观测、配置治理。

## 目录结构
```
k8s/
  00-namespace.yaml
  apps/
    01-configmaps.yaml
    02-order-api.yaml
    03-order-rpc.yaml
    04-product-rpc.yaml
    05-ingress.yaml
    06-pdb.yaml
  deps/
    redis.yaml
    mysql.yaml
    etcd.yaml
    dtm.yaml
  jobs/
    01-mysql-init.yaml
    02-redis-seed.yaml
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
你需要先构建 3 个镜像（与 K8s 配置保持一致）：
- `flash-mall/order-api:dev`
- `flash-mall/order-rpc:dev`
- `flash-mall/product-rpc:dev`

建议：后续加 `Makefile` 或 CI 构建，面试时更加分。

## 一键部署（最小可用）
```bash
kubectl apply -f k8s/00-namespace.yaml
kubectl apply -f k8s/deps/   # 如果不用 Helm
kubectl apply -f k8s/jobs/   # 初始化 MySQL/Redis
kubectl apply -f k8s/apps/
```

## 多节点（kind）方案
如果你需要展示“跨节点调度/容错/反亲和”等面试点，使用多节点方案：
- 方案文档：`docs/K8S_KIND_MULTI_NODE.md`
- 一键脚本：`./scripts/k8s/kind-multi-deploy.ps1`

## 访问方式
- Ingress 入口：`http://flash-mall.local`
- Metrics：
  - order-api：`/metrics` (9090)
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
- 本配置为演示用途，生产需要把敏感信息移到 Secret。
