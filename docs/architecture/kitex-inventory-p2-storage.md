# InventoryService P2 Redis/MySQL 仓储方案

## 目标

P2 在 P1 的 Kitex InventoryService 骨架上补充真实存储仓储：Redis 管理运行时库存和订单级 reservation，MySQL 分桶表作为对账和重建来源。

这一阶段仍不替换当前 go-zero 下单链路，只让 inventory 服务具备可接入真实存储的能力。

## 新增仓储

新增 `RedisMySQLRepository`：

```text
app/inventory/repository/redis_mysql.go
```

职责：

- `SeedStock`：按分片写入 Redis，并可同步写入 MySQL `product_stock_bucket`。
- `ReserveStock`：使用 Redis Lua 在分片库存中预扣，并写入 `inventory:reservation:{order_id}`。
- `ConfirmDeduct`：将 reservation 从 `reserved` 标记为 `confirmed`。
- `ReleaseStock`：根据 reservation 记录的商品、数量和分片归还 Redis 库存。
- `GetStock`：优先汇总 Redis 分片；Redis 未初始化时可 fallback 到 MySQL 分桶表。
- `ReconcileStock`：比较 Redis 库存和 MySQL 分桶库存，发现差异时用 MySQL 重建 Redis 分片。

## Redis Key 约定

沿用现有库存 key：

```text
stock:{product_id}:{shard_index}
```

新增 reservation key：

```text
inventory:reservation:{order_id}
```

字段：

```text
status      reserved | confirmed | released
product_id
quantity
shard_index
order_id
```

## MySQL 表约定

继续使用现有表：

```text
product_stock_bucket(product_id, bucket_idx, stock, version)
```

P2 没有新增数据库表，避免扩大迁移面。

## Kitex 入口选择仓储

`app/inventory/kitex/main.go` 支持环境变量：

```text
INVENTORY_LISTEN_ON
INVENTORY_REDIS_HOST
INVENTORY_DATASOURCE
INVENTORY_STOCK_SHARD_COUNT
```

如果未设置 `INVENTORY_REDIS_HOST`，服务继续使用内存仓储，便于本地骨架启动。设置 Redis 后，使用 `RedisMySQLRepository`。

## 当前边界

P2 已完成真实仓储能力，但还没有改动旧业务链路：

- 后台新增商品仍走旧逻辑。
- 下单预扣仍走旧 order-rpc 逻辑。
- 退款/关单归还仍走旧逻辑。

下一阶段再逐步接入这些调用点。

## 后续 P3

建议从低风险入口开始接入：

1. 后台新增商品后调用 `InventoryService.SeedStock`。
2. 库存对账脚本/后台接口调用 `InventoryService.ReconcileStock`。
3. 下单预扣后续再迁移到 `ReserveStock`。
4. 退款和关单最后迁移到 `ReleaseStock`。
