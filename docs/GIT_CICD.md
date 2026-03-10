# Git 与 CI/CD 现状说明

## 当前结论

- `2026-03-10` 检查时，项目目录原本**没有** `.git`，不是一个已初始化的 Git 仓库。
- 也没有远端仓库、分支策略或现成的 CI/CD 配置。
- 本次已在本地执行 `git init -b main`，补齐了最小 Git 骨架，后续只差推送到 GitHub 即可启用 Actions。

## 之前的 Git 流程问题

1. 没有版本控制入口，代码、脚本、性能报告和部署清单无法形成可追溯变更历史。
2. 没有统一分支模型，后续多人协作时容易直接改主线。
3. 没有自动验证，`go test ./...`、`go vet ./...`、Dockerfile 构建有效性全靠手动执行。
4. 没有制品发布和集群部署入口，镜像发布与 K8s 发布过程不可复现。

## 本次补齐的工作流

### 1. CI

文件：`.github/workflows/ci.yml`

- 触发：`push(main/develop)`、`pull_request`
- 执行内容：
  - `go mod download`
  - `go vet ./...`
  - `go test ./...`
  - `go build ./app/order/api ./app/order/rpc ./app/product/rpc`
  - 校验 3 个 Dockerfile 可成功构建
  - 运行 `scripts/ci/smoke-e2e.sh` 做端到端冒烟：
    - 拉起 `etcd/mysql/redis/dtm/rabbitmq`
    - 初始化数据库与 Redis 库存
    - 启动 `product-rpc/order-rpc/order-api`
    - 校验健康检查、JWT 登录、下单成功与订单落库

### 2. 镜像发布

文件：`.github/workflows/release-images.yml`

- 触发：
  - 推送 `main` / `develop`
  - 推送 tag：`v*`
  - 手动触发 `workflow_dispatch`
- 发布目标：`ghcr.io/<owner>/flash-mall-order-api`
  - `ghcr.io/<owner>/flash-mall-order-rpc`
  - `ghcr.io/<owner>/flash-mall-product-rpc`
- 自动标签策略：
  - `sha-<commit_sha>`
  - 分支标签（如 `main` / `develop`）
  - `main-latest` / `develop-latest`
  - tag 发布时额外生成 `latest`

### 3. Demo K8s 部署

文件：`.github/workflows/deploy-k8s.yml`

- 触发：手动触发 `workflow_dispatch`
- 自动触发：`Release Images` 在 `main` 成功完成后自动部署
- 能力：
  - 可选应用 `k8s/deps/`
  - 应用 `k8s/apps/`
  - 可选执行 `k8s/jobs/`
  - 用 `kubectl set image` 切换到指定镜像 tag
  - 等待 3 个 Deployment rollout 完成
  - 自动部署时默认使用 `sha-<workflow_run.head_sha>` 镜像 tag

## 推荐 Git 流程

建议采用轻量 `trunk-based + PR` 流程：

1. `main` 只接收可发布代码。
2. 日常开发从 `feature/<topic>` 分支切出。
3. 所有改动通过 PR 合并到 `main`。
4. PR 必须通过 `CI`。
5. 合并到 `main` 后自动发布 GHCR 镜像。
6. 如果已配置 `KUBE_CONFIG` 且未关闭自动部署，则 `main` 会自动部署到 demo 集群。
7. 生产或回滚场景仍使用手动 `Deploy Demo Cluster`，输入指定镜像 tag。

## 推到 GitHub 前还需要的动作

1. 在 GitHub 新建远端仓库。
2. 设置远端：

```bash
git remote add origin <your-github-repo-url>
git push -u origin main
```

3. 在仓库 `Settings -> Secrets and variables -> Actions` 中新增：
  - `KUBE_CONFIG`
    - 内容为目标集群的 kubeconfig 文本
4. 可选新增仓库变量：
  - `AUTO_DEPLOY_DEMO=false`
    - 设置后可关闭 `main` 的自动 demo 部署

## 建议的仓库保护策略

- 开启 `main` 分支保护
- 要求 PR 合并前必须通过 `CI`
- 禁止直接 push 到 `main`
- 发布动作仅允许维护者手动触发

## 本地已验证内容

- `go test ./...` 通过
- `go vet ./...` 通过

## 仍然需要你在 GitHub 上完成的事

- 关联远端仓库
- 首次 push
- 配置 `KUBE_CONFIG` Secret
- 按需配置 `AUTO_DEPLOY_DEMO`
- 按需启用 `demo` environment 的审批规则
