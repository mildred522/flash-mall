# kind 多节点方案与部署（Flash-Mall）

## 目标
- 在单机上模拟多节点集群，用于验证调度/容错/扩缩容策略。
- 保留现有业务链路与压测能力，补齐“跨节点调度”面试点。

## 原理简述
- kind 用多个 Docker 容器模拟多个 K8s 节点。
- 每个节点容器运行 kubelet 等组件，构成一个完整集群。
- 节点间通信走 Docker 网络，适合演示调度与高可用，但不等价于真实多机性能。

## 架构方案
- 1 个 control-plane + 2 个 worker（共 3 节点）
- 业务 Pod 通过调度分散到不同节点
- 通过反亲和/拓扑分布（可选）强化“跨节点”体验

## 关键文件
- kind 集群配置：`k8s/kind/cluster-multi.yaml`
- WSL 一键部署脚本：`scripts/k8s/dev-up.sh`
- WSL 底层 kind 脚本：`scripts/k8s/kind-deploy.sh`
- Windows 兼容脚本：`scripts/k8s/kind-multi-deploy.ps1`

## 部署步骤
> 注意：该操作会删除现有 kind 集群，数据将清空。

WSL 推荐路径：

```bash
cd /home/mildred/code/flash-mall

# 直接创建多节点集群并部署
scripts/k8s/dev-up.sh

# 如果刚改过代码，需要重建镜像
scripts/k8s/dev-up.sh --rebuild-images

# 保留 2 副本和演示型 HPA/PDB 语义
scripts/k8s/dev-up.sh --profile demo

# 转发入口服务
scripts/k8s/restore-port-forward.sh
```

Windows 兼容路径：

```powershell
./scripts/k8s/kind-multi-deploy.ps1
```

## 验证方式
```bash
kubectl get nodes -o wide
kubectl -n flash-mall get pods -o wide
```
- 期望看到 3 个节点；Pod 分布到不同节点。

## 可选增强（面试加分）
已落地的增强项（可直接演示）：
- entry-api / order-rpc / product-rpc 已加入 `podAntiAffinity` + `topologySpreadConstraints`，保证跨节点均衡。
- 为核心服务增加 PDB（PodDisruptionBudget），确保节点维护时最少保留 1 个副本。

验证：
```powershell
kubectl -n flash-mall get pdb
kubectl -n flash-mall get pods -o wide
```

## 演练脚本（可直接用来背诵/演示）
- 节点维护/故障演练：`./scripts/k8s/ha-node-drain.ps1`
- HPA 扩缩容演练：`./scripts/k8s/hpa-e2e.ps1 -PrepareData`
  - 如果 HPA TARGETS 显示 `<unknown>`，说明未安装 metrics-server（参见 `docs/K8S_DEPLOY.md`）。

## 回滚
```bash
scripts/k8s/dev-down.sh
```
