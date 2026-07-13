package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"sync"

	"flash-mall/app/common/apperror"
	"flash-mall/app/inventory/domain"
)

const (
	reservationTTLSeconds          = 24 * 60 * 60
	confirmedReservationTTLSeconds = 180 * 24 * 60 * 60
)

type RedisClient interface {
	EvalCtx(ctx context.Context, script string, keys []string, args ...any) (any, error)
}

type RedisMySQLRepository struct {
	redis              RedisClient
	db                 *sql.DB
	shardCount         int
	finalDeductEnabled bool
	changeLogOnce      sync.Once
	changeLogErr       error
	snapshotOnce       sync.Once
	snapshotErr        error
}

func NewRedisMySQLRepository(redis RedisClient, db *sql.DB, shardCount int) *RedisMySQLRepository {
	return &RedisMySQLRepository{redis: redis, db: db, shardCount: NormalizeShardCount(shardCount)}
}

func (r *RedisMySQLRepository) WithFinalDeductEnabled(enabled bool) *RedisMySQLRepository {
	r.finalDeductEnabled = enabled
	return r
}

func (r *RedisMySQLRepository) GetStock(ctx context.Context, productID int64) (domain.Stock, error) {
	available, exists, err := r.redisAvailable(ctx, productID)
	if err != nil {
		return domain.Stock{}, apperror.Wrap(apperror.CodeInternal, "read redis stock failed", err)
	}
	if exists {
		reserved, err := r.redisReserved(ctx, productID)
		if err != nil {
			return domain.Stock{}, apperror.Wrap(apperror.CodeInternal, "read redis reserved stock failed", err)
		}
		return domain.Stock{ProductID: productID, Available: available, Reserved: reserved, Total: available + reserved}, nil
	}
	if r.db == nil {
		return domain.Stock{}, domain.ErrStockNotFound
	}
	total, ok, err := r.mysqlBucketTotal(ctx, productID)
	if err != nil {
		return domain.Stock{}, err
	}
	if !ok {
		return domain.Stock{}, domain.ErrStockNotFound
	}
	return domain.Stock{ProductID: productID, Available: total, Total: total}, nil
}

func (r *RedisMySQLRepository) BatchGetStock(ctx context.Context, productIDs []int64) ([]domain.Stock, error) {
	if len(productIDs) == 0 {
		return []domain.Stock{}, nil
	}
	uniqueIDs := uniquePositiveProductIDs(productIDs)
	if len(uniqueIDs) == 0 {
		return []domain.Stock{}, nil
	}
	stocks, found, err := r.redisBatchStocks(ctx, uniqueIDs)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "batch read redis stock failed", err)
	}
	if len(found) < len(uniqueIDs) && r.db != nil {
		for _, productID := range uniqueIDs {
			if _, ok := found[productID]; ok {
				continue
			}
			total, ok, err := r.mysqlBucketTotal(ctx, productID)
			if err != nil {
				return nil, err
			}
			if ok {
				stocks = append(stocks, domain.Stock{ProductID: productID, Available: total, Total: total})
			}
		}
	}
	return orderStocksByProductIDs(uniqueIDs, stocks), nil
}

func (r *RedisMySQLRepository) SeedStock(ctx context.Context, productID int64, total int64, shardCount int) error {
	if shardCount <= 0 {
		shardCount = r.shardCount
	}
	if err := r.seedRedis(ctx, productID, total, shardCount); err != nil {
		return err
	}
	if r.db != nil {
		if err := r.seedMySQLBuckets(ctx, productID, total, shardCount); err != nil {
			return err
		}
		return r.refreshStockSnapshot(ctx, productID)
	}
	return nil
}

func (r *RedisMySQLRepository) AdjustStock(ctx context.Context, productID int64, delta int64, bucketIdx int, meta domain.StockChangeMeta) (domain.Stock, domain.Stock, error) {
	before, err := r.GetStock(ctx, productID)
	if err != nil {
		return domain.Stock{}, domain.Stock{}, err
	}
	if r.db != nil {
		if err := r.ensureStockChangeLogTable(ctx); err != nil {
			return before, before, err
		}
		if err := r.adjustMySQLStock(ctx, productID, delta, bucketIdx); err != nil {
			return before, before, err
		}
	}
	if err := r.adjustRedisAvailable(ctx, productID, delta, bucketIdx); err != nil {
		return before, before, err
	}
	after, err := r.GetStock(ctx, productID)
	if err != nil {
		return before, domain.Stock{}, err
	}
	if r.db != nil {
		if err := r.insertStockChangeLog(ctx, "ADJUST", productID, meta.OrderID, delta, bucketIdx, before, after, meta); err != nil {
			return before, after, err
		}
		if err := r.upsertStockSnapshot(ctx, after); err != nil {
			return before, after, err
		}
	}
	return before, after, nil
}

func (r *RedisMySQLRepository) ReserveStock(ctx context.Context, orderID string, productID int64, quantity int64, meta domain.StockChangeMeta) error {
	before, _ := r.GetStock(ctx, productID)
	keys := append(StockShardKeys(productID, r.shardCount), reservationKey(orderID), reservedStockKey(productID))
	ret, err := evalInt64(ctx, r.redis, reserveStockLuaScript, keys, quantity, reservationTTLSeconds, r.shardCount, StockShardStartIndex(orderID, r.shardCount), productID, orderID)
	if err != nil {
		return apperror.Wrap(apperror.CodeStockReserveFailed, "reserve stock failed", err)
	}
	switch ret {
	case 1:
		after, _ := r.GetStock(ctx, productID)
		_ = r.insertStockChangeLog(ctx, "RESERVE", productID, orderID, -quantity, -1, before, after, meta)
		_ = r.upsertStockSnapshot(ctx, after)
		return nil
	case -1:
		return domain.ErrStockNotFound
	case -2:
		return domain.ErrStockInsufficient
	default:
		return apperror.New(apperror.CodeStockReserveFailed, fmt.Sprintf("unexpected reserve result %d", ret))
	}
}

func (r *RedisMySQLRepository) ConfirmDeduct(ctx context.Context, orderID string, meta domain.StockChangeMeta) error {
	reservation, _ := evalReservation(ctx, r.redis, getReservationLuaScript, []string{reservationKey(orderID)})
	var before domain.Stock
	if reservation.ProductID > 0 {
		before, _ = r.GetStock(ctx, reservation.ProductID)
	}
	finalDeductFlag := 0
	if r.finalDeductEnabled && r.db != nil {
		finalDeductFlag = 1
	}
	ret, err := evalInt64(ctx, r.redis, confirmDeductLuaScript, []string{reservationKey(orderID)}, confirmedReservationTTLSeconds, finalDeductFlag)
	if err != nil {
		return apperror.Wrap(apperror.CodeInternal, "confirm stock deduction failed", err)
	}
	if ret == 2 {
		if reservation.ProductID <= 0 || reservation.Quantity <= 0 {
			return nil
		}
		if err := r.confirmMySQLDeduct(ctx, orderID, reservation.ProductID, reservation.Quantity); err != nil {
			return err
		}
		if _, err := evalInt64(ctx, r.redis, markMySQLDeductedLuaScript, []string{reservationKey(orderID)}, confirmedReservationTTLSeconds); err != nil {
			return apperror.Wrap(apperror.CodeInternal, "mark mysql stock deduction failed", err)
		}
	}
	if (ret == 2 || ret == 3) && reservation.ProductID > 0 {
		after, _ := r.GetStock(ctx, reservation.ProductID)
		_ = r.insertStockChangeLog(ctx, "CONFIRM", reservation.ProductID, orderID, -reservation.Quantity, -1, before, after, meta)
		_ = r.upsertStockSnapshot(ctx, after)
	}
	return nil
}

func (r *RedisMySQLRepository) ReleaseStock(ctx context.Context, orderID string, meta domain.StockChangeMeta) error {
	// The reservation stores a 1-based shard index, so all stock shard keys are needed for restore.
	reservation, err := evalReservation(ctx, r.redis, getReservationLuaScript, []string{reservationKey(orderID)})
	if err != nil {
		return apperror.Wrap(apperror.CodeInternal, "read stock reservation failed", err)
	}
	if reservation.ProductID <= 0 || reservation.Quantity <= 0 {
		return nil
	}
	before, _ := r.GetStock(ctx, reservation.ProductID)
	if r.finalDeductEnabled && r.db != nil {
		if err := r.releaseMySQLDeduct(ctx, orderID, reservation.ProductID, reservation.Quantity); err != nil {
			return err
		}
	}
	keys := append(StockShardKeys(reservation.ProductID, r.shardCount), reservationKey(orderID), reservedStockKey(reservation.ProductID))
	_, err = evalInt64(ctx, r.redis, releaseStockLuaScript, keys, reservationTTLSeconds, r.shardCount)
	if err != nil {
		return apperror.Wrap(apperror.CodeInternal, "release stock failed", err)
	}
	after, _ := r.GetStock(ctx, reservation.ProductID)
	_ = r.insertStockChangeLog(ctx, "RELEASE", reservation.ProductID, orderID, reservation.Quantity, -1, before, after, meta)
	_ = r.upsertStockSnapshot(ctx, after)
	return nil
}

func (r *RedisMySQLRepository) ReconcileStock(ctx context.Context, productID int64, meta domain.StockChangeMeta) (domain.Stock, domain.Stock, bool, error) {
	before, err := r.GetStock(ctx, productID)
	if err != nil && apperror.CodeOf(err) != apperror.CodeStockNotFound {
		return domain.Stock{}, domain.Stock{}, false, err
	}
	if r.db == nil {
		return before, before, false, nil
	}
	total, ok, err := r.mysqlBucketTotal(ctx, productID)
	if err != nil {
		return domain.Stock{}, domain.Stock{}, false, err
	}
	if !ok {
		return before, before, false, domain.ErrStockNotFound
	}
	after := domain.Stock{ProductID: productID, Available: total, Total: total}
	changed := before.Available != after.Available
	if changed {
		if err := r.seedRedis(ctx, productID, total, r.shardCount); err != nil {
			return before, after, false, err
		}
		_ = r.insertStockChangeLog(ctx, "RECONCILE", productID, "", after.Available-before.Available, -1, before, after, meta)
		_ = r.upsertStockSnapshot(ctx, after)
	}
	return before, after, changed, nil
}

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
	keys := StockShardKeys(productID, shardCount)
	values := SplitStockAcrossShards(total, shardCount)
	args := make([]any, 0, len(values))
	for _, value := range values {
		args = append(args, value)
	}
	if _, err := r.redis.EvalCtx(ctx, seedStockLuaScript, keys, args...); err != nil {
		return apperror.Wrap(apperror.CodeInternal, "seed redis stock failed", err)
	}
	if _, err := r.redis.EvalCtx(ctx, resetReservedStockLuaScript, []string{reservedStockKey(productID)}); err != nil {
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
	if err := r.ensureStockSnapshotTable(ctx); err != nil {
		return err
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

func (r *RedisMySQLRepository) ensureStockSnapshotTable(ctx context.Context) error {
	r.snapshotOnce.Do(func() {
		if r.db == nil {
			return
		}
		_, r.snapshotErr = r.db.ExecContext(ctx, stockSnapshotDDL)
		if r.snapshotErr != nil {
			r.snapshotErr = apperror.Wrap(apperror.CodeInternal, "ensure stock snapshot table failed", r.snapshotErr)
		}
	})
	return r.snapshotErr
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
	if err := r.ensureStockChangeLogTable(ctx); err != nil {
		return err
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

func (r *RedisMySQLRepository) ensureStockChangeLogTable(ctx context.Context) error {
	r.changeLogOnce.Do(func() {
		if r.db == nil {
			return
		}
		_, r.changeLogErr = r.db.ExecContext(ctx, stockChangeLogDDL)
		if r.changeLogErr != nil {
			r.changeLogErr = apperror.Wrap(apperror.CodeInternal, "ensure stock change log table failed", r.changeLogErr)
		}
	})
	return r.changeLogErr
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

type redisReservation struct {
	ProductID int64
	Quantity  int64
}

func reservationKey(orderID string) string {
	return "inventory:reservation:" + orderID
}

func reservedStockKey(productID int64) string {
	return fmt.Sprintf("stock_reserved:%d", productID)
}

func uniquePositiveProductIDs(productIDs []int64) []int64 {
	seen := make(map[int64]struct{}, len(productIDs))
	result := make([]int64, 0, len(productIDs))
	for _, productID := range productIDs {
		if productID <= 0 {
			continue
		}
		if _, ok := seen[productID]; ok {
			continue
		}
		seen[productID] = struct{}{}
		result = append(result, productID)
	}
	return result
}

func orderStocksByProductIDs(productIDs []int64, stocks []domain.Stock) []domain.Stock {
	byID := make(map[int64]domain.Stock, len(stocks))
	for _, stock := range stocks {
		byID[stock.ProductID] = stock
	}
	ordered := make([]domain.Stock, 0, len(stocks))
	for _, productID := range productIDs {
		if stock, ok := byID[productID]; ok {
			ordered = append(ordered, stock)
		}
	}
	return ordered
}

func evalInt64(ctx context.Context, redis RedisClient, script string, keys []string, args ...any) (int64, error) {
	val, err := redis.EvalCtx(ctx, script, keys, args...)
	if err != nil {
		return 0, err
	}
	return toInt64(val)
}

func evalReservation(ctx context.Context, redis RedisClient, script string, keys []string, args ...any) (redisReservation, error) {
	val, err := redis.EvalCtx(ctx, script, keys, args...)
	if err != nil {
		return redisReservation{}, err
	}
	parts, err := toInt64Slice(val)
	if err != nil {
		return redisReservation{}, err
	}
	if len(parts) < 2 {
		return redisReservation{}, fmt.Errorf("unexpected reservation result %v", val)
	}
	return redisReservation{ProductID: parts[0], Quantity: parts[1]}, nil
}

func toInt64(val any) (int64, error) {
	switch typed := val.(type) {
	case int64:
		return typed, nil
	case int:
		return int64(typed), nil
	case string:
		return strconv.ParseInt(typed, 10, 64)
	case []byte:
		return strconv.ParseInt(string(typed), 10, 64)
	default:
		return 0, fmt.Errorf("unexpected redis eval result type %T", val)
	}
}

func toInt64Slice(val any) ([]int64, error) {
	switch typed := val.(type) {
	case []any:
		out := make([]int64, 0, len(typed))
		for _, item := range typed {
			v, err := toInt64(item)
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
		return out, nil

	default:
		return nil, fmt.Errorf("unexpected redis eval array type %T", val)
	}
}

const seedStockLuaScript = `
for i = 1, #KEYS do
  redis.call("set", KEYS[i], ARGV[i])
end
return 1
`

const resetReservedStockLuaScript = `
redis.call("set", KEYS[1], 0)
return 1
`

const sumStockLuaScript = `
local exists = 0
local total = 0
for i = 1, #KEYS do
  local stock = redis.call("get", KEYS[i])
  if stock then
    exists = 1
    total = total + tonumber(stock)
  end
end
return {exists, total}
`

const getReservedStockLuaScript = `
local reserved = redis.call("get", KEYS[1])
if not reserved then
  return 0
end
return tonumber(reserved)
`

const batchStockLuaScript = `
local shardCount = tonumber(ARGV[1])
local result = {}
for productOffset = 1, #ARGV - 1 do
  local productID = tonumber(ARGV[productOffset + 1])
  local keyOffset = (productOffset - 1) * (shardCount + 1)
  local exists = 0
  local available = 0
  for shardOffset = 1, shardCount do
    local stock = redis.call("get", KEYS[keyOffset + shardOffset])
    if stock then
      exists = 1
      available = available + tonumber(stock)
    end
  end
  local reserved = tonumber(redis.call("get", KEYS[keyOffset + shardCount + 1]) or "0")
  table.insert(result, productID)
  table.insert(result, exists)
  table.insert(result, available)
  table.insert(result, reserved)
end
return result
`

const stockChangeLogDDL = `
CREATE TABLE IF NOT EXISTS inventory_stock_change_log (
  id bigint NOT NULL AUTO_INCREMENT,
  product_id bigint NOT NULL,
  order_id varchar(64) NOT NULL DEFAULT '',
  change_type varchar(32) NOT NULL,
  delta bigint NOT NULL DEFAULT 0,
  bucket_idx int NOT NULL DEFAULT 0,
  before_available bigint NOT NULL DEFAULT 0,
  before_reserved bigint NOT NULL DEFAULT 0,
  before_total bigint NOT NULL DEFAULT 0,
  after_available bigint NOT NULL DEFAULT 0,
  after_reserved bigint NOT NULL DEFAULT 0,
  after_total bigint NOT NULL DEFAULT 0,
  reason varchar(255) NOT NULL DEFAULT '',
  request_id varchar(64) NOT NULL DEFAULT '',
  trace_id varchar(64) NOT NULL DEFAULT '',
  operator_user_id bigint NOT NULL DEFAULT 0,
  operator_merchant_id bigint NOT NULL DEFAULT 0,
  operator_role varchar(32) NOT NULL DEFAULT '',
  create_time timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY ix_product_time (product_id, create_time),
  KEY ix_order_id (order_id),
  KEY ix_request_id (request_id),
  KEY ix_operator (operator_user_id, operator_merchant_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
`

const stockSnapshotDDL = `
CREATE TABLE IF NOT EXISTS product_stock_snapshot (
  product_id bigint NOT NULL,
  available bigint NOT NULL DEFAULT 0,
  reserved bigint NOT NULL DEFAULT 0,
  total bigint NOT NULL DEFAULT 0,
  source varchar(32) NOT NULL DEFAULT 'inventory-kitex',
  version bigint NOT NULL DEFAULT 0,
  update_time timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (product_id),
  KEY ix_update_time (update_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
`

const adjustAvailableStockLuaScript = `
local delta = tonumber(ARGV[1])
local shardCount = tonumber(ARGV[2])
local start = tonumber(ARGV[3])
if delta > 0 then
  redis.call("incrby", KEYS[start], delta)
  return 1
end
local amount = -delta
local total = 0
for i = 1, shardCount do
  total = total + tonumber(redis.call("get", KEYS[i]) or "0")
end
if total < amount then
  return -2
end
local remain = amount
for i = 0, shardCount - 1 do
  local idx = ((start - 1 + i) % shardCount) + 1
  local stock = tonumber(redis.call("get", KEYS[idx]) or "0")
  if stock > 0 then
    local take = stock
    if take > remain then
      take = remain
    end
    redis.call("decrby", KEYS[idx], take)
    remain = remain - take
    if remain == 0 then
      return 1
    end
  end
end
return -2
`

const reserveStockLuaScript = `
local shardCount = tonumber(ARGV[3])
local reservationKey = KEYS[shardCount + 1]
local reservedKey = KEYS[shardCount + 2]
local status = redis.call("hget", reservationKey, "status")
if status and status ~= "released" then
  return 1
end
local amount = tonumber(ARGV[1])
local ttl = tonumber(ARGV[2])
local start = tonumber(ARGV[4])
local productID = ARGV[5]
local orderID = ARGV[6]
local hasStockKey = false
for i = 0, shardCount - 1 do
  local idx = ((start + i) % shardCount) + 1
  local stockKey = KEYS[idx]
  local stock = redis.call("get", stockKey)
  if stock then
    hasStockKey = true
    stock = tonumber(stock)
    if stock >= amount then
      redis.call("decrby", stockKey, amount)
      redis.call("incrby", reservedKey, amount)
      redis.call("hset", reservationKey, "status", "reserved", "product_id", productID, "quantity", amount, "shard_index", idx, "order_id", orderID)
      if ttl and ttl > 0 then
        redis.call("expire", reservationKey, ttl)
      end
      return 1
    end
  end
end
if hasStockKey == false then
  return -1
end
return -2
`

const confirmDeductLuaScript = `
local reservationKey = KEYS[1]
local status = redis.call("hget", reservationKey, "status")
local changed = 0
if not status then
  return 1
end
if status == "released" then
  return 1
end
if status == "reserved" then
  changed = 1
  redis.call("hset", reservationKey, "status", "confirmed")
  local productID = redis.call("hget", reservationKey, "product_id")
  local quantity = tonumber(redis.call("hget", reservationKey, "quantity") or "0")
  if productID and quantity > 0 then
    local reservedKey = "stock_reserved:" .. productID
    local current = tonumber(redis.call("get", reservedKey) or "0")
    local nextValue = current - quantity
    if nextValue < 0 then
      nextValue = 0
    end
    redis.call("set", reservedKey, nextValue)
  end
end
local ttl = tonumber(ARGV[1])
if ttl and ttl > 0 then
  redis.call("expire", reservationKey, ttl)
end
local finalDeductEnabled = tonumber(ARGV[2])
local mysqlDeducted = redis.call("hget", reservationKey, "mysql_deducted")
if finalDeductEnabled == 1 and mysqlDeducted ~= "1" then
  return 2
end
if changed == 1 then
  return 3
end
return 1
`

const markMySQLDeductedLuaScript = `
local reservationKey = KEYS[1]
redis.call("hset", reservationKey, "mysql_deducted", "1")
local ttl = tonumber(ARGV[1])
if ttl and ttl > 0 then
  redis.call("expire", reservationKey, ttl)
end
return 1
`

const getReservationLuaScript = `
local reservationKey = KEYS[1]
local productID = redis.call("hget", reservationKey, "product_id")
local quantity = redis.call("hget", reservationKey, "quantity")
if not productID or not quantity then
  return {0, 0}
end
return {tonumber(productID), tonumber(quantity)}
`

const releaseStockLuaScript = `
local shardCount = tonumber(ARGV[2])
local reservationKey = KEYS[shardCount + 1]
local reservedKey = KEYS[shardCount + 2]
local status = redis.call("hget", reservationKey, "status")
if not status or status == "released" then
  return 1
end
local quantity = tonumber(redis.call("hget", reservationKey, "quantity"))
local shardIndex = tonumber(redis.call("hget", reservationKey, "shard_index"))
local stockKey = KEYS[shardIndex]
if stockKey and quantity and quantity > 0 then
  redis.call("incrby", stockKey, quantity)
end
if status == "reserved" and quantity and quantity > 0 then
  local current = tonumber(redis.call("get", reservedKey) or "0")
  local nextValue = current - quantity
  if nextValue < 0 then
    nextValue = 0
  end
  redis.call("set", reservedKey, nextValue)
end
redis.call("hset", reservationKey, "status", "released")
local ttl = tonumber(ARGV[1])
if ttl and ttl > 0 then
  redis.call("expire", reservationKey, ttl)
end
return 1
`
