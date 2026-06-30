# Kitex InventoryService P1 方案

## 目标

P1 在长期架构分支上引入第一个 Kitex 服务：`InventoryService`。这一阶段只建立库存服务骨架和领域边界，不替换当前 go-zero 下单链路。

## 为什么先做库存服务

当前项目里库存问题最集中：

- 后台新增商品后 Redis 库存种子可能缺失。
- 下单链路涉及 Redis 分片预扣和 MySQL 分桶扣减。
- 退款、关单、回滚都需要归还库存。
- 库存对账和自愈逻辑分散在多个服务里。

因此 P1 先把库存能力收敛成独立服务边界，为后续订单服务和商品服务迁移做准备。

## 当前实现范围

新增目录：

```text
app/inventory/
  config/
  domain/
  repository/
  service/
  kitex/
```

核心接口来自 `idl/inventory.thrift`：

- `GetStock`
- `SeedStock`
- `ReserveStock`
- `ConfirmDeduct`
- `ReleaseStock`
- `ReconcileStock`

当前 Kitex handler 已经接入 `service.Service`，不再是空 TODO。

## 当前 repository 策略

P1 默认使用 `MemoryStockRepository`，原因是：

- 先验证库存服务语义，不急着替换现有 Redis/MySQL 链路。
- 避免在同一阶段同时引入 Kitex、Redis Lua、MySQL 分桶和旧链路迁移。
- 单测可以稳定覆盖预扣、确认扣减、释放库存和幂等行为。

后续 P2 再增加 Redis/MySQL repository，并逐步接入现有链路。

## 已保留的现有模型

P1 抽出了当前项目已存在的库存策略：

- Redis 库存分片 key：`stock:{product_id}:{shard}`
- 根据 `order_id` 计算分片起始位置。
- 将总库存按分片拆分。
- 预扣、确认扣减、释放库存保持幂等语义。

## 部署占位

新增：

- `build/docker/inventory-kitex.Dockerfile`
- `deploy/config/inventory.yaml`

当前还没有把 `inventory-kitex` 接入 compose/k8s 主链路。这样可以保证旧服务仍然稳定运行。

## 后续 P2

P2 建议做 Redis/MySQL repository：

- Redis repository 接管 `SeedStock`、`ReserveStock`、`ReleaseStock`。
- MySQL repository 接管库存分桶扣减、对账读取。
- 后台新增商品后调用 `InventoryService.SeedStock`。
- 订单创建链路调用 `InventoryService.ReserveStock`。
- 退款/关单调用 `InventoryService.ReleaseStock`。

## 面试叙述

这一步可以概括为：在正式迁移订单链路前，我先选择库存服务作为 Kitex 试点，把库存种子、预扣、确认扣减、释放和对账定义成明确 RPC 能力。为了降低风险，P1 只建立服务骨架和领域层，旧 go-zero 交易链路保持不变，后续再逐步把 Redis/MySQL 库存实现接入。
