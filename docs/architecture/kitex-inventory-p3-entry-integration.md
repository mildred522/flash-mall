# InventoryService P3 低风险接入方案

## 目标

P3 将 InventoryService 以可选方式接入 `entry-api` 的低风险入口，先不替换完整下单链路。

当前接入点：

- 后台新增商品后调用 `InventoryService.SeedStock`。
- 后台调整商品库存后调用 `InventoryService.ReconcileStock`。

两个调用都是 best-effort：失败只记录日志，不影响原有接口返回。

## 配置

`entry-api` 新增配置：

```yaml
InventoryKitexEndpoint: ''
```

默认空值表示不启用 InventoryService 客户端，旧逻辑完全保持原样。

如果后续部署 inventory-kitex，可以配置成：

```yaml
InventoryKitexEndpoint: inventory-kitex:8093
```

## 新增客户端包装

新增：

```text
app/entry/api/internal/inventoryclient/client.go
```

它包装 Kitex 生成 client，只暴露当前 P3 需要的能力：

- `SeedStock`
- `ReconcileStock`

这样 `entry-api` 的 handler 不直接依赖 Kitex 生成代码细节。

## 接入点

### 后台新增商品

旧逻辑仍保留：

1. 写商品表。
2. 写 MySQL 库存桶。
3. 写 Redis 库存分片。
4. 可选调用 `InventoryService.SeedStock`。

### 后台调整库存

旧逻辑仍保留：

1. 更新 MySQL 库存桶。
2. 可选调用 `InventoryService.ReconcileStock`。
3. 刷新商品缓存。

## 为什么不直接替换下单链路

下单链路涉及订单创建、Redis 预扣、DTM 补偿、支付确认、退款归还。直接迁移风险较高。

P3 只接商品新增和库存调整，是因为：

- 调用频率低。
- 失败可降级。
- 不影响用户下单主链路。
- 可以验证 Kitex 客户端、配置和库存服务运行状态。

## 后续 P4

建议下一步接入：

1. 增加 inventory-kitex 的 compose/k8s 部署。
2. 让 `entry-api` 配置 `InventoryKitexEndpoint`。
3. 在管理后台增加库存对账入口，调用 `ReconcileStock`。
4. 再考虑订单预扣迁移。
