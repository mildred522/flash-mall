# RabbitMQ 增量架构执行记录（2026-03-01）

## 1. 目标
- 在不替换现有 Redis 延迟关单链路的前提下，引入 RabbitMQ 事件链路（Outbox + Consumer）。
- 要求可答辩：有命令、有运行证据、有问题修复记录。

## 2. 执行清单（已完成）
1) 编译与测试
- `go test ./...` 通过（本地 Go 环境）。

2) 集群增量部署
- `kubectl apply -f k8s/deps/rabbitmq.yaml`
- `kubectl apply -f k8s/apps/01-configmaps.yaml`
- `kubectl -n flash-mall rollout restart deploy/order-rpc deploy/order-api`

3) 数据库迁移
- 执行 `./scripts/k8s/alter-orders.ps1 -Namespace flash-mall -RootPassword flashmall`
- `mall_order.order_outbox` 表创建成功。

4) 触发链路验证
- 使用 `./scripts/k8s/run-benchmark-incluster.ps1` 触发下单流量。
- 结果文件：`docs/rabbitmq_event_smoke_bench.json`。

## 3. 过程问题与修复

### 问题 A：`alter-orders.sql` 在 MySQL 5.7 报语法错误
- 原因：`ADD COLUMN IF NOT EXISTS` 与 `CREATE INDEX IF NOT EXISTS` 在 5.7 不可用。
- 修复：`scripts/k8s/alter-orders.sql` 改为 `information_schema + PREPARE` 条件执行。

### 问题 B：`alter-orders.ps1` 执行时密码参数未正确传递
- 现象：`Access denied for user 'root'@'localhost'`。
- 修复：`scripts/k8s/alter-orders.ps1` 改为显式构造 `$mysqlPasswordArg`，并增加失败退出检查。

### 问题 C：Outbox 发布器出现 `publish confirm timeout` 与状态卡在 `publishing`
- 原因 1：`NotifyPublish` 每次发布都新建监听，导致确认通道使用不当。
- 原因 2：发布超时后复用同一个已超时 context 回写状态，造成 `mark retry` 失败。
- 修复：
  - `app/order/rpc/internal/job/rabbit_publisher.go`：confirm channel 改为连接级复用。
  - `app/order/rpc/internal/job/outbox_publisher.go`：处理超时拉长，状态回写改独立短 context。

## 4. 运行证据

1) RabbitMQ 组件在线
- `kubectl -n flash-mall get pods -l app=rabbitmq`：`1/1 Running`
- `kubectl -n flash-mall get svc rabbitmq`：暴露 `5672/15672`

2) 压测请求成功（触发下单）
- `docs/rabbitmq_event_smoke_bench.json`：
  - `success=44`
  - `failed=0`
  - `p95=60.27ms`

3) Outbox 成功发布
- SQL：`SELECT status, COUNT(*) FROM mall_order.order_outbox GROUP BY status;`
- 结果：仅 `status=1`，数量 `44`（全部 published）。

4) Consumer 成功消费 + 幂等生效
- order-api 日志可见多条 `order event consumed`。
- 指标（样例）：
  - `flashmall_order_event_consume_total{result="success"} 50`
  - `flashmall_order_event_consume_total{result="duplicate"} 4`

5) RabbitMQ 队列无积压
- `rabbitmqctl list_queues name messages_ready messages_unacknowledged`
- `order.events.created.q  0  0`

## 5. 下一优化点（P0）
- P0 已执行：Outbox 单活发布（leader lock）
  - 改造点：
    - `app/order/rpc/internal/job/outbox_publisher.go`：增加 Redis leader lock（`SETNX EX` 抢锁 + Lua 续约）。
    - `app/order/rpc/internal/config/config.go`：新增 `OutboxSingleActive/OutboxLeaderLockKey/OutboxLeaderLockTTL`。
    - `k8s/apps/01-configmaps.yaml`：开启单活发布并下发锁配置。
  - 运行证据：
    - 仅一个 order-rpc 实例日志出现 `outbox publisher became leader`。
    - 压测后 `order_outbox` 仅 `status=1`（`44` 条），无 `publishing` 残留。
    - 两个 order-rpc 副本近 3 分钟无 outbox publish error。

## 6. 下一优化点（P1）
- 自适应轮询退避：当 outbox 连续空轮询时自动放大 `poll interval`，出现待发布事件时快速回落，减少无效 DB 扫描。
