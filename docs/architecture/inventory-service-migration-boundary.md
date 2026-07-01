# InventoryService Migration Boundary

## Current Ownership

The long-term target is to make `inventory-kitex` the single write owner of product stock. The current branch is still in a transition phase and intentionally keeps final MySQL deduction in `product-rpc`.

Current stock write ownership:

| Flow | Redis available stock | MySQL stock bucket |
| --- | --- | --- |
| Admin create product | `entry-api` best-effort seeds `InventoryService` | `entry-api` writes product buckets |
| Admin adjust stock | `entry-api` best-effort reconciles `InventoryService` | `entry-api` writes product buckets |
| Create order reserve | `order-rpc` calls `InventoryService.ReserveStock` when configured | unchanged |
| SAGA final deduct | unchanged | `product-rpc.Deduct` |
| SAGA rollback / close / refund | `order-rpc.PreDeductRollback` calls `InventoryService.ReleaseStock` when configured | `product-rpc.RevertStock` / `DeductRollback` |
| Payment success | `InventoryService.ConfirmDeduct` confirms reservation | unchanged |

## Why Product Deduct Is Still Active

`entry-api/internal/logic/createorderlogic.go` still adds this SAGA branch:

```go
saga.Add(productRoute+"/Deduct", productRoute+"/DeductRollback", deductReq)
```

As long as that branch exists, `InventoryService.ConfirmDeduct` must not update MySQL stock buckets. Otherwise one order would decrement stock twice:

1. `InventoryService.ReserveStock` decrements Redis available stock.
2. `product-rpc.Deduct` decrements MySQL stock buckets.
3. If `InventoryService.ConfirmDeduct` also decremented MySQL, the same order would double deduct.

## Safe Next Migration Step

To make `InventoryService` the final stock write owner, apply the migration in this order:

1. Keep `INVENTORY_FINAL_DEDUCT_ENABLED=false` in deployed `inventory-kitex`.
2. Add a matching config switch in `entry-api`, for example `InventoryOwnsFinalDeduct`.
3. When both switches are enabled, skip `product-rpc.Deduct` in the create-order SAGA.
4. When both switches are enabled, skip `product-rpc` rollback/revert in close and refund compensation paths.
5. Keep `product-rpc.Deduct` as the default path until the inventory-owned path is verified in compose and k8s.
6. Remove product stock write APIs only after the inventory-owned path is stable.

## Compatibility Rules

- `InventoryKitexEndpoint` controls only Redis reservation and reservation confirmation today.
- `INVENTORY_FINAL_DEDUCT_ENABLED` exists in `inventory-kitex`, but deployed configs keep it disabled today.
- `product-rpc` remains the source of final MySQL stock bucket deduction today.
- Refund and close flows must continue to call both:
  - `product-rpc` rollback/revert for MySQL buckets.
  - `order-rpc.PreDeductRollback`, which delegates to `InventoryService.ReleaseStock` when enabled.

## Known Risk

The transition still has two write owners for stock:

- Redis available stock: `inventory-kitex`
- MySQL stock bucket: `product-rpc`

This is acceptable only as a migration step. The next architecture milestone should remove that split by moving final MySQL deduction into `inventory-kitex` behind a feature switch.
