# Flash-Mall 可靠性能测试流程（面试可答辩版）

## 目标

当面试官追问“你的数据怎么来的、为什么可信”时，你可以给出一条可复现实验链路：

1. 固定环境与数据集。
2. 预热 + 稳态采样，避免冷启动噪声。
3. 重复多次（默认 5 次）并给出中位数与波动。
4. 同时采集压测报告、pprof、服务日志、K8s 快照。
5. 输出可审计证据目录与汇总报告。

---

## 一、实验前准备（固定变量）

### 1) 固定集群状态

```powershell
kubectl -n flash-mall get deploy
kubectl -n flash-mall get hpa
kubectl -n flash-mall get pods -o wide
```

建议：评估时先固定副本数（不做弹性扩容）再跑对比，避免 HPA 干扰。

### 2) 固定镜像和配置

- 使用同一份 `k8s/apps/01-configmaps.yaml`。
- 记录部署和 ConfigMap 快照（脚本会自动保存）。
- 每次测试前都重新灌入相同库存（脚本已自动做）。

---

## 二、单次采集（带证据链）

> 脚本：`scripts/k8s/perf-collect.ps1`

```powershell
./scripts/k8s/perf-collect.ps1 `
  -Namespace flash-mall `
  -Concurrency 100 `
  -WarmupSeconds 30 `
  -DurationSeconds 180 `
  -Scenario baseline `
  -TargetRps 0 `
  -LoadMode in-cluster
```

### 这个脚本会自动做什么

- 端口转发到 `order-api` 与 3 个 RPC pprof 端口。
- **先启动 CPU pprof，再开始压测**（采样覆盖真实负载窗口）。
- 跑压测（带 warmup/timeout/错误分类）。
- 采集 heap profile、应用日志、events、pods/nodes/deployments/configmap 快照。
- 输出目录：`docs/perf/<timestamp>/`。

---

## 三、可靠性跑法（推荐面试用）

> 脚本：`scripts/k8s/perf-reliable.ps1`

```powershell
./scripts/k8s/perf-reliable.ps1 `
  -Namespace flash-mall `
  -Runs 5 `
  -Concurrency 100 `
  -WarmupSeconds 30 `
  -DurationSeconds 180 `
  -Scenario interview-baseline `
  -CooldownSeconds 20 `
  -LoadMode in-cluster
```

### 输出内容

- 原始证据：`docs/perf/reliable-<timestamp>/run-xx/*`
- 汇总 JSON：`docs/perf/reliable-<timestamp>/summary.json`
- 面试版报告：`docs/PERF_RELIABLE_<timestamp>.md`

### 汇总指标（自动计算）

- QPS 中位数 + IQR + CV
- P95 中位数 + IQR + CV
- P99 中位数
- 成功率中位数
- 429/503 总量
- 稳定性建议（`stable` / `unstable_retest`）

---

## 四、对比实验（优化前 vs 优化后）

1. 在相同参数下跑一组 baseline（5 次）。
2. 部署改动后再跑一组 candidate（5 次）。
3. 对比两份 `summary.json` 的中位数，不用单次最好值。

最小可辩护结论模板：

- “我们使用 5 次重复实验，报告中位数与 IQR，避免单次偶然值。”
- “采样窗口是 warmup 30s 后的稳态 180s。”
- “压测、pprof、日志、K8s 快照在同一轮目录可交叉验证。”
- “若 CV > 10%，判定不稳定并重测，不直接下结论。”

---

## 五、面试官常见追问与回答要点

### Q1: 为什么你说数据可信？

答：
- 同场景重复 5 次，不用单次峰值。
- 报告提供中位数 + 波动（IQR/CV）。
- 固定了配置、库存、并发、时长、版本。
- 有原始证据可回放：bench JSON、pprof、日志、events、pods 快照。

### Q2: 会不会只是限流把失败挡掉了？

答：
- 报告里有状态码拆分（200/429/503）。
- 结合业务成功率与日志验证：如果 429 增加而 503 降低，需要区分“保护生效”与“能力提升”。
- 结论必须同时看成功率、延迟、状态码结构。

### Q3: 为什么不是一次就够？

答：
- 单次受调度、GC、网络抖动影响大。
- 多次重复并看 CV 才能判断是否稳定。

---

## 六、建议的“通过门槛”

用于决定“可以写进简历”的最小标准：

- 成功率中位数 >= 99.9%
- P95 改善幅度 >= 20%（且方向一致）
- QPS 不明显回退（<5% 回退可接受，需说明权衡）
- QPS 与 P95 的 CV <= 10%
- 429/503 变化可解释（与限流配置或故障演练一致）

如果不满足，结论写“趋势观察”，不要写“已优化完成”。

---

## 七、关键脚本说明

- `scripts/k8s/run-benchmark.ps1`
  - 新增 warmup、targetRps、timeout 等参数透传。
- `scripts/k8s/perf-collect.ps1`
  - 支持 `port-forward` / `in-cluster` 两种发压模式，并发采集 pprof，增加环境快照与证据归档。
- `scripts/k8s/perf-reliable.ps1`
  - 自动重复运行并输出中位数/IQR/CV 级别的统计报告。
- `scripts/k8s/run-benchmark-incluster.ps1`
  - 使用 K8s Job + k6 在集群内发压，规避 host 侧 port-forward 单点瓶颈。
- `app/order/api/scripts/benchmark/benchmark_tool.go`
  - 支持 warmup 窗口、可选 open-loop RPS、状态码/错误类型拆分与结构化报告。

---

## 八、实跑注意事项（2026-02-28 补充）

- 推荐优先使用 `-LoadMode in-cluster`；`port-forward` 仅用于本地快速调试。
- `product-rpc` 当前未暴露 pprof（可先跳过其 profile 采集，不影响主流程）。
- 若出现 `Metrics API not available`，说明集群未安装 metrics-server，仅影响 `kubectl top`，不影响压测主结果。
