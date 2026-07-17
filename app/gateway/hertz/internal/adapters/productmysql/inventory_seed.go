package productmysql

import (
	"context"
	"database/sql"
	"errors"

	"flash-mall/app/common/apperror"
	"flash-mall/app/gateway/hertz/internal/ports"
)

type InventoryInitializer struct {
	db     *sql.DB
	seeder ports.InventoryStockSeeder
}

var _ ports.ProductInventoryInitializer = (*InventoryInitializer)(nil)

func NewInventoryInitializer(db *sql.DB, seeder ports.InventoryStockSeeder) *InventoryInitializer {
	return &InventoryInitializer{db: db, seeder: seeder}
}

func (i *InventoryInitializer) Initialize(ctx context.Context, productID int64, meta ports.RequestMeta) (ports.InventorySeedState, error) {
	state := ports.InventorySeedState{ProductID: productID, Status: ports.InventorySeedPending}
	if i.db == nil {
		return state, apperror.New(apperror.CodeInternal, "product datasource is not configured")
	}
	if i.seeder == nil {
		return state, apperror.New(apperror.CodeInternal, "inventory kitex client is not configured")
	}
	var desiredTotal int64
	var shardCount int32
	var desiredStatus int64
	var seedStatus int8
	var attemptCount int64
	err := i.db.QueryRowContext(ctx, `SELECT desired_total, shard_count, desired_product_status
       , status, attempt_count
FROM mall_product.product_inventory_seed
WHERE product_id = ?`, productID).Scan(&desiredTotal, &shardCount, &desiredStatus, &seedStatus, &attemptCount)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return state, apperror.New(apperror.CodeProductNotFound, "product inventory seed task not found")
		}
		return state, err
	}
	state.Attempts = attemptCount
	if ports.InventorySeedStatus(seedStatus) == ports.InventorySeedSucceeded {
		state.Status = ports.InventorySeedSucceeded
		return state, nil
	}
	if err = i.seeder.SeedStock(ctx, productID, desiredTotal, shardCount, meta); err != nil {
		state.Status = ports.InventorySeedFailed
		state.Attempts++
		state.LastError = err.Error()
		_, updateErr := i.db.ExecContext(ctx, `UPDATE mall_product.product_inventory_seed
SET status = 2, attempt_count = attempt_count + 1, last_error = ?, next_retry_time = DATE_ADD(NOW(), INTERVAL 30 SECOND)
WHERE product_id = ?`, state.LastError, productID)
		if updateErr != nil {
			return state, apperror.Wrap(apperror.CodeInternal, "record inventory seed failure", updateErr)
		}
		return state, apperror.Wrap(apperror.CodeStockReconcileFailed, "initialize product inventory", err)
	}

	tx, err := i.db.BeginTx(ctx, nil)
	if err != nil {
		return state, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.ExecContext(ctx, `UPDATE mall_product.product_inventory_seed
SET status = 1, attempt_count = attempt_count + 1, last_error = '', next_retry_time = NULL, seeded_at = NOW()
WHERE product_id = ?`, productID); err != nil {
		return state, err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE mall_product.product SET status = ? WHERE id = ?", desiredStatus, productID); err != nil {
		return state, err
	}
	if err = tx.Commit(); err != nil {
		return state, err
	}
	state.Status = ports.InventorySeedSucceeded
	state.Attempts++
	return state, nil
}
