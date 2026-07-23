package repository

import (
	"context"
	"database/sql"
	"fmt"

	"flash-mall/app/common/apperror"
	"flash-mall/app/inventory/domain"
)

func (r *RedisMySQLRepository) redisAvailable(ctx context.Context, productID int64) (int64, bool, error) {
	keys := StockShardKeys(productID, r.shardCount)
	val, err := r.redis.EvalCtx(ctx, sumStockLuaScript, keys)
	if err != nil {
		return 0, false, err
	}
	parts, err := toInt64Slice(val)
	if err != nil {
		return 0, false, err
	}
	if len(parts) < 2 {
		return 0, false, fmt.Errorf("unexpected redis stock sum result %v", val)
	}
	return parts[1], parts[0] == 1, nil
}

func (r *RedisMySQLRepository) redisReserved(ctx context.Context, productID int64) (int64, error) {
	val, err := r.redis.EvalCtx(ctx, getReservedStockLuaScript, []string{reservedStockKey(productID)})
	if err != nil {
		return 0, err
	}
	return toInt64(val)
}

func (r *RedisMySQLRepository) redisBatchStocks(ctx context.Context, productIDs []int64) ([]domain.Stock, map[int64]struct{}, error) {
	keys := make([]string, 0, len(productIDs)*(r.shardCount+1))
	for _, productID := range productIDs {
		keys = append(keys, StockShardKeys(productID, r.shardCount)...)
		keys = append(keys, reservedStockKey(productID))
	}
	args := make([]any, 0, len(productIDs)+1)
	args = append(args, r.shardCount)
	for _, productID := range productIDs {
		args = append(args, productID)
	}
	val, err := r.redis.EvalCtx(ctx, batchStockLuaScript, keys, args...)
	if err != nil {
		return nil, nil, err
	}
	parts, err := toInt64Slice(val)
	if err != nil {
		return nil, nil, err
	}
	if len(parts)%4 != 0 {
		return nil, nil, fmt.Errorf("unexpected batch stock result %v", val)
	}
	stocks := make([]domain.Stock, 0, len(productIDs))
	found := make(map[int64]struct{}, len(productIDs))
	for idx := 0; idx < len(parts); idx += 4 {
		productID := parts[idx]
		exists := parts[idx+1] == 1
		available := parts[idx+2]
		reserved := parts[idx+3]
		if !exists {
			continue
		}
		found[productID] = struct{}{}
		stocks = append(stocks, domain.Stock{ProductID: productID, Available: available, Reserved: reserved, Total: available + reserved})
	}
	return stocks, found, nil
}

func (r *RedisMySQLRepository) seedRedis(ctx context.Context, productID int64, total int64, shardCount int) error {
	return r.seedRedisState(ctx, productID, total, 0, shardCount)
}

func (r *RedisMySQLRepository) seedRedisState(ctx context.Context, productID int64, available int64, reserved int64, shardCount int) error {
	keys := StockShardKeys(productID, shardCount)
	values := SplitStockAcrossShards(available, shardCount)
	args := make([]any, 0, len(values))
	for _, value := range values {
		args = append(args, value)
	}
	if _, err := r.redis.EvalCtx(ctx, seedStockLuaScript, keys, args...); err != nil {
		return apperror.Wrap(apperror.CodeInternal, "seed redis stock failed", err)
	}
	if _, err := r.redis.EvalCtx(ctx, resetReservedStockLuaScript, []string{reservedStockKey(productID)}, reserved); err != nil {
		return apperror.Wrap(apperror.CodeInternal, "reset redis reserved stock failed", err)
	}
	return nil
}

func (r *RedisMySQLRepository) mysqlBucketTotal(ctx context.Context, productID int64) (int64, bool, error) {
	if r.db == nil {
		return 0, false, nil
	}
	var total int64
	var count int64
	if err := r.db.QueryRowContext(ctx, "SELECT COALESCE(SUM(stock), 0), COUNT(*) FROM product_stock_bucket WHERE product_id = ?", productID).Scan(&total, &count); err != nil {
		return 0, false, apperror.Wrap(apperror.CodeInternal, "read mysql stock bucket failed", err)
	}
	return total, count > 0, nil
}

func (r *RedisMySQLRepository) seedMySQLBuckets(ctx context.Context, productID int64, total int64, shardCount int) error {
	values := SplitStockAcrossShards(total, shardCount)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return apperror.Wrap(apperror.CodeInternal, "begin stock seed transaction failed", err)
	}
	defer tx.Rollback()
	for idx, value := range values {
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO product_stock_bucket (product_id, bucket_idx, stock, version) VALUES (?, ?, ?, 0) ON DUPLICATE KEY UPDATE stock = VALUES(stock), version = version + 1",
			productID, idx, value,
		); err != nil {
			return apperror.Wrap(apperror.CodeInternal, "seed mysql stock bucket failed", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return apperror.Wrap(apperror.CodeInternal, "commit stock seed transaction failed", err)
	}
	return nil
}

func (r *RedisMySQLRepository) adjustMySQLStock(ctx context.Context, productID int64, delta int64, bucketIdx int) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return apperror.Wrap(apperror.CodeInternal, "begin stock adjust transaction failed", err)
	}
	defer tx.Rollback()

	var result sql.Result
	if delta > 0 {
		result, err = tx.ExecContext(ctx,
			"INSERT INTO product_stock_bucket (product_id, bucket_idx, stock, version) VALUES (?, ?, ?, 0) ON DUPLICATE KEY UPDATE stock = stock + VALUES(stock), version = version + 1",
			productID, bucketIdx, delta,
		)
	} else {
		result, err = tx.ExecContext(ctx,
			"UPDATE product_stock_bucket SET stock = stock + ?, version = version + 1 WHERE product_id = ? AND bucket_idx = ? AND stock + ? >= 0",
			delta, productID, bucketIdx, delta,
		)
	}
	if err != nil {
		return apperror.Wrap(apperror.CodeInternal, "adjust mysql stock bucket failed", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return apperror.Wrap(apperror.CodeInternal, "read stock adjust result failed", err)
	}
	if rows == 0 {
		return domain.ErrStockInsufficient
	}

	total, ok, err := mysqlBucketTotalTx(ctx, tx, productID)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrStockNotFound
	}
	result, err = tx.ExecContext(ctx, "UPDATE product SET stock = ?, version = version + 1 WHERE id = ?", total, productID)
	if err != nil {
		return apperror.Wrap(apperror.CodeInternal, "update product stock total failed", err)
	}
	rows, err = result.RowsAffected()
	if err != nil {
		return apperror.Wrap(apperror.CodeInternal, "read product stock update result failed", err)
	}
	if rows == 0 {
		return domain.ErrStockNotFound
	}
	if err := tx.Commit(); err != nil {
		return apperror.Wrap(apperror.CodeInternal, "commit stock adjust transaction failed", err)
	}
	return nil
}

func mysqlBucketTotalTx(ctx context.Context, tx *sql.Tx, productID int64) (int64, bool, error) {
	var total int64
	var count int64
	if err := tx.QueryRowContext(ctx, "SELECT COALESCE(SUM(stock), 0), COUNT(*) FROM product_stock_bucket WHERE product_id = ?", productID).Scan(&total, &count); err != nil {
		return 0, false, apperror.Wrap(apperror.CodeInternal, "read mysql stock bucket failed", err)
	}
	return total, count > 0, nil
}

func (r *RedisMySQLRepository) refreshStockSnapshot(ctx context.Context, productID int64) error {
	if r.db == nil {
		return nil
	}
	stock, err := r.GetStock(ctx, productID)
	if err != nil {
		return err
	}
	return r.upsertStockSnapshot(ctx, stock)
}

func (r *RedisMySQLRepository) upsertStockSnapshot(ctx context.Context, stock domain.Stock) error {
	if r.db == nil || stock.ProductID <= 0 {
		return nil
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO product_stock_snapshot (product_id, available, reserved, total, source, version)
VALUES (?, ?, ?, ?, 'inventory-kitex', 0)
ON DUPLICATE KEY UPDATE
  available = VALUES(available),
  reserved = VALUES(reserved),
  total = VALUES(total),
  source = VALUES(source),
  version = version + 1`,
		stock.ProductID,
		stock.Available,
		stock.Reserved,
		stock.Total,
	)
	if err != nil {
		return apperror.Wrap(apperror.CodeInternal, "upsert stock snapshot failed", err)
	}
	return nil
}

func (r *RedisMySQLRepository) adjustRedisAvailable(ctx context.Context, productID int64, delta int64, bucketIdx int) error {
	startIndex := bucketIdx + 1
	if startIndex <= 0 || startIndex > r.shardCount {
		startIndex = 1
	}
	ret, err := evalInt64(ctx, r.redis, adjustAvailableStockLuaScript, StockShardKeys(productID, r.shardCount), delta, r.shardCount, startIndex)
	if err != nil {
		return apperror.Wrap(apperror.CodeInternal, "adjust redis stock failed", err)
	}
	if ret == -2 {
		return domain.ErrStockInsufficient
	}
	if ret != 1 {
		return apperror.New(apperror.CodeInternal, fmt.Sprintf("unexpected adjust stock result %d", ret))
	}
	return nil
}

func (r *RedisMySQLRepository) insertStockChangeLog(ctx context.Context, changeType string, productID int64, orderID string, delta int64, bucketIdx int, before domain.Stock, after domain.Stock, meta domain.StockChangeMeta) error {
	if r.db == nil {
		return nil
	}
	if orderID == "" {
		orderID = meta.OrderID
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO inventory_stock_change_log
  (product_id, order_id, change_type, delta, bucket_idx, before_available, before_reserved, before_total, after_available, after_reserved, after_total, reason, request_id, trace_id, operator_user_id, operator_merchant_id, operator_role)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		productID,
		orderID,
		changeType,
		delta,
		bucketIdx,
		before.Available,
		before.Reserved,
		before.Total,
		after.Available,
		after.Reserved,
		after.Total,
		meta.Reason,
		meta.RequestID,
		meta.TraceID,
		meta.UserID,
		meta.MerchantID,
		meta.Role,
	)
	if err != nil {
		return apperror.Wrap(apperror.CodeInternal, "insert stock change log failed", err)
	}
	return nil
}

func (r *RedisMySQLRepository) confirmMySQLDeduct(ctx context.Context, orderID string, productID int64, quantity int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return apperror.Wrap(apperror.CodeInternal, "begin stock deduct transaction failed", err)
	}
	defer tx.Rollback()

	if exists, err := stockDeductLogExists(ctx, tx, orderID); err != nil {
		return err
	} else if exists {
		return tx.Commit()
	}

	preferredBucketIdx := StockShardStartIndex(orderID, r.shardCount)
	bucketIdx, err := lockDeductibleStockBucket(ctx, tx, productID, quantity, preferredBucketIdx)
	if err != nil {
		return err
	}
	logType := stockDeductBucketLogType(bucketIdx)
	result, err := tx.ExecContext(ctx, "INSERT IGNORE INTO stock_log (order_id, type) VALUES (?, ?)", orderID, logType)
	if err != nil {
		return apperror.Wrap(apperror.CodeInternal, "insert stock deduct log failed", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return apperror.Wrap(apperror.CodeInternal, "read stock deduct log result failed", err)
	}
	if affected == 0 {
		return tx.Commit()
	}
	if _, err = tx.ExecContext(ctx,
		"UPDATE product_stock_bucket SET stock = stock - ?, version = version + 1 WHERE product_id = ? AND bucket_idx = ? AND stock >= ?",
		quantity, productID, bucketIdx, quantity,
	); err != nil {
		return apperror.Wrap(apperror.CodeInternal, "deduct mysql stock bucket failed", err)
	}
	if err := tx.Commit(); err != nil {
		return apperror.Wrap(apperror.CodeInternal, "commit stock deduct transaction failed", err)
	}
	return nil
}

func (r *RedisMySQLRepository) releaseMySQLDeduct(ctx context.Context, orderID string, productID int64, quantity int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return apperror.Wrap(apperror.CodeInternal, "begin stock release transaction failed", err)
	}
	defer tx.Rollback()

	bucketIdx, ok, err := stockDeductBucketFromLog(ctx, tx, orderID)
	if err != nil {
		return err
	}
	if !ok {
		return tx.Commit()
	}
	result, err := tx.ExecContext(ctx, "INSERT IGNORE INTO stock_log (order_id, type) VALUES (?, 'REVERT')", orderID)
	if err != nil {
		return apperror.Wrap(apperror.CodeInternal, "insert stock release log failed", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return apperror.Wrap(apperror.CodeInternal, "read stock release log result failed", err)
	}
	if affected == 0 {
		return tx.Commit()
	}
	if _, err = tx.ExecContext(ctx,
		"INSERT INTO product_stock_bucket (product_id, bucket_idx, stock, version) VALUES (?, ?, ?, 0) ON DUPLICATE KEY UPDATE stock = stock + VALUES(stock), version = version + 1",
		productID, bucketIdx, quantity,
	); err != nil {
		return apperror.Wrap(apperror.CodeInternal, "release mysql stock bucket failed", err)
	}
	if err := tx.Commit(); err != nil {
		return apperror.Wrap(apperror.CodeInternal, "commit stock release transaction failed", err)
	}
	return nil
}

func lockDeductibleStockBucket(ctx context.Context, tx *sql.Tx, productID int64, quantity int64, preferredBucketIdx int) (int, error) {
	var bucketIdx int
	err := tx.QueryRowContext(ctx,
		`SELECT bucket_idx
		 FROM product_stock_bucket
		 WHERE product_id = ? AND stock >= ?
		 ORDER BY CASE WHEN bucket_idx = ? THEN 0 ELSE 1 END, bucket_idx
		 LIMIT 1
		 FOR UPDATE`,
		productID, quantity, preferredBucketIdx,
	).Scan(&bucketIdx)
	if err == nil {
		return bucketIdx, nil
	}
	if err != sql.ErrNoRows {
		return 0, apperror.Wrap(apperror.CodeInternal, "lock mysql stock bucket failed", err)
	}

	var exists int
	err = tx.QueryRowContext(ctx, "SELECT 1 FROM product_stock_bucket WHERE product_id = ? LIMIT 1", productID).Scan(&exists)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, domain.ErrStockNotFound
		}
		return 0, apperror.Wrap(apperror.CodeInternal, "read mysql stock bucket failed", err)
	}
	return 0, domain.ErrStockInsufficient
}

func stockDeductLogExists(ctx context.Context, tx *sql.Tx, orderID string) (bool, error) {
	var exists int
	err := tx.QueryRowContext(ctx, "SELECT 1 FROM stock_log WHERE order_id = ? AND type LIKE 'DEDUCT_BUCKET_%' LIMIT 1", orderID).Scan(&exists)
	if err == nil {
		return true, nil
	}
	if err == sql.ErrNoRows {
		return false, nil
	}
	return false, apperror.Wrap(apperror.CodeInternal, "read stock deduct log failed", err)
}

func stockDeductBucketFromLog(ctx context.Context, tx *sql.Tx, orderID string) (int, bool, error) {
	var logType string
	err := tx.QueryRowContext(ctx, "SELECT type FROM stock_log WHERE order_id = ? AND type LIKE 'DEDUCT_BUCKET_%' ORDER BY id DESC LIMIT 1", orderID).Scan(&logType)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, false, nil
		}
		return 0, false, apperror.Wrap(apperror.CodeInternal, "read stock deduct log failed", err)
	}
	var bucketIdx int
	if _, err := fmt.Sscanf(logType, "DEDUCT_BUCKET_%d", &bucketIdx); err != nil {
		return 0, false, apperror.Wrap(apperror.CodeInternal, "parse stock deduct bucket failed", err)
	}
	return bucketIdx, true, nil
}

func stockDeductBucketLogType(bucketIdx int) string {
	return fmt.Sprintf("DEDUCT_BUCKET_%d", bucketIdx)
}
