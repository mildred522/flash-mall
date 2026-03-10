# RabbitMQ 增量架构清单（面试导向）

> 目标：在不推翻现有 Redis 延迟关单链路的前提下，引入 RabbitMQ，展示事件驱动架构能力与工程化可靠性。

## 1) 范围界定（增量，不替换）
- [x] 保留现有下单主链路与 Redis 延迟关单逻辑。
- [x] 新增 RabbitMQ 事件链路：`order-rpc -> RabbitMQ -> order-api consumer`。
- [x] 采用 Outbox Pattern，避免“DB 已提交但消息丢失”。

## 2) 设计与实现项
- [x] 新增 `order_outbox` 表（订单库）并提供迁移脚本。
- [x] `CreateOrder` 分支在同一事务内写入 outbox 事件（`order.created`）。
- [x] order-rpc 新增 outbox publisher 后台任务：批量拉取、发布、重试、状态回写。
- [x] 使用 RabbitMQ durable exchange + persistent message。
- [x] order-api 新增 RabbitMQ consumer：手动 ack、幂等去重（Redis SETNX+TTL）、消费指标。
- [x] 新增配置项（本地 YAML + K8s ConfigMap）：RabbitMQ 连接、队列、路由键、批次/间隔等。
- [x] 新增 K8s RabbitMQ 依赖清单并接入现有 apply 流程。

## 3) 验证项（可答辩）
- [x] 编译/测试通过（至少关键包 `go test` 通过）。
- [x] K8s 中 RabbitMQ/服务启动成功。
- [x] 下单后 outbox 从 pending 变 sent。
- [x] consumer 有消费日志与幂等行为（重复消息不重复处理）。
- [x] 文档记录“为什么这样设计 + 证据路径 + 下一步优化”。

## 4) 面试可讲点（验收口径）
- [x] 为什么增量引入而不是全量替换：降低改造风险、保留稳定主链路。
- [x] 为什么要 outbox：解决本地事务与消息发布的一致性缺口。
- [x] 为什么要幂等：至少一次投递语义下，消费侧必须可重复。
- [x] 为什么保留 Redis DLQ：下单关单是核心链路，先稳再扩展。
