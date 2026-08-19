package repository

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"flash-mall/app/common/apperror"
	"flash-mall/app/inventory/domain"
)

type recoveryReservation struct {
	OrderID    string
	ProductID  int64
	Quantity   int64
	ShardIndex int
	Status     domain.ReservationStatus
	ExpiresAt  time.Time
}

func (r *RedisMySQLRepository) loadRecoveryBuckets(ctx context.Context) (map[int64][]int64, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT product_id, bucket_idx, stock
FROM product_stock_bucket
ORDER BY product_id, bucket_idx`)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "load inventory recovery buckets failed", err)
	}
	defer rows.Close()

	buckets := make(map[int64][]int64)
	for rows.Next() {
		var productID, stock int64
		var bucketIdx int
		if err := rows.Scan(&productID, &bucketIdx, &stock); err != nil {
			return nil, apperror.Wrap(apperror.CodeInternal, "scan inventory recovery bucket failed", err)
		}
		if bucketIdx < 0 || bucketIdx >= r.shardCount || stock < 0 {
			return nil, apperror.New(apperror.CodeInternal, fmt.Sprintf(
				"invalid inventory bucket: product_id=%d bucket_idx=%d stock=%d shard_count=%d",
				productID, bucketIdx, stock, r.shardCount,
			))
		}
		if buckets[productID] == nil {
			buckets[productID] = make([]int64, r.shardCount)
		}
		buckets[productID][bucketIdx] = stock
	}
	if err := rows.Err(); err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "read inventory recovery buckets failed", err)
	}
	return buckets, nil
}

func (r *RedisMySQLRepository) loadRecoveryReservations(ctx context.Context) ([]recoveryReservation, error) {
	if normalizeReservationLedgerMode(r.reservationLedgerMode) == reservationLedgerModeOff {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT r.order_id, r.product_id, r.quantity, r.shard_index, r.status, r.expires_at,
       COALESCE((SELECT d.type FROM stock_log d WHERE d.order_id = r.order_id AND d.type LIKE 'DEDUCT_BUCKET_%' ORDER BY d.id DESC LIMIT 1), ''),
       EXISTS(SELECT 1 FROM stock_log v WHERE v.order_id = r.order_id AND v.type = 'REVERT')
FROM inventory_reservation r
WHERE r.status IN ('RESERVED', 'CONFIRMED')
ORDER BY r.product_id, r.order_id`)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "load inventory recovery reservations failed", err)
	}
	defer rows.Close()

	reservations := make([]recoveryReservation, 0)
	for rows.Next() {
		var item recoveryReservation
		var status string
		var deductType string
		var reverted bool
		if err := rows.Scan(&item.OrderID, &item.ProductID, &item.Quantity, &item.ShardIndex,
			&status, &item.ExpiresAt, &deductType, &reverted); err != nil {
			return nil, apperror.Wrap(apperror.CodeInternal, "scan inventory recovery reservation failed", err)
		}
		if reverted {
			continue
		}
		item.Status = domain.ReservationStatus(strings.ToLower(status))
		if deductType != "" {
			bucketIdx, parseErr := strconv.Atoi(strings.TrimPrefix(deductType, "DEDUCT_BUCKET_"))
			if parseErr != nil || bucketIdx < 0 || bucketIdx >= r.shardCount {
				return nil, apperror.New(apperror.CodeInternal,
					fmt.Sprintf("invalid inventory deduct log: order_id=%s type=%s", item.OrderID, deductType))
			}
			item.ShardIndex = bucketIdx + 1
			item.Status = domain.ReservationConfirmed
		} else if item.Status == domain.ReservationConfirmed && r.finalDeductEnabled {
			return nil, apperror.New(apperror.CodeInternal,
				fmt.Sprintf("confirmed inventory reservation is missing deduct log: order_id=%s", item.OrderID))
		}
		reservations = append(reservations, item)
	}
	if err := rows.Err(); err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "read inventory recovery reservations failed", err)
	}
	return reservations, nil
}

func (r *RedisMySQLRepository) RecoverRuntime(ctx context.Context) (domain.RuntimeRecoveryReport, error) {
	if r.db == nil {
		return domain.RuntimeRecoveryReport{}, nil
	}
	if err := r.checkSchema(ctx); err != nil {
		return domain.RuntimeRecoveryReport{}, err
	}
	buckets, err := r.loadRecoveryBuckets(ctx)
	if err != nil {
		return domain.RuntimeRecoveryReport{}, err
	}
	reservations, err := r.loadRecoveryReservations(ctx)
	if err != nil {
		return domain.RuntimeRecoveryReport{}, err
	}

	reservedByProduct := make(map[int64]int64)
	for _, item := range reservations {
		productBuckets, ok := buckets[item.ProductID]
		if !ok {
			return domain.RuntimeRecoveryReport{}, apperror.New(apperror.CodeInternal,
				fmt.Sprintf("inventory reservation references missing stock: order_id=%s product_id=%d", item.OrderID, item.ProductID))
		}
		if item.Status != domain.ReservationReserved {
			continue
		}
		bucketIdx := item.ShardIndex - 1
		if item.Quantity <= 0 || bucketIdx < 0 || bucketIdx >= len(productBuckets) {
			return domain.RuntimeRecoveryReport{}, apperror.New(apperror.CodeInternal,
				fmt.Sprintf("invalid inventory reservation: order_id=%s quantity=%d shard_index=%d", item.OrderID, item.Quantity, item.ShardIndex))
		}
		if productBuckets[bucketIdx] < item.Quantity {
			return domain.RuntimeRecoveryReport{}, apperror.New(apperror.CodeInternal,
				fmt.Sprintf("inventory recovery underflow: order_id=%s product_id=%d bucket_idx=%d stock=%d quantity=%d",
					item.OrderID, item.ProductID, bucketIdx, productBuckets[bucketIdx], item.Quantity))
		}
		productBuckets[bucketIdx] -= item.Quantity
		reservedByProduct[item.ProductID] += item.Quantity
	}

	if _, err := evalInt64(ctx, r.redis, resetReservationRecoveryIndexesLuaScript, []string{
		reservationExpiryIndexKey,
		reservationProcessingIndexKey,
	}); err != nil {
		return domain.RuntimeRecoveryReport{}, apperror.Wrap(apperror.CodeInternal, "reset inventory recovery indexes failed", err)
	}

	report := domain.RuntimeRecoveryReport{Products: len(buckets)}
	for productID, productBuckets := range buckets {
		reserved := reservedByProduct[productID]
		if err := r.restoreProductStock(ctx, productID, productBuckets, reserved); err != nil {
			return report, err
		}
		available := sumInt64(productBuckets)
		if err := r.upsertStockSnapshot(ctx, domain.Stock{
			ProductID: productID,
			Available: available,
			Reserved:  reserved,
			Total:     available + reserved,
		}); err != nil {
			return report, err
		}
	}
	for _, item := range reservations {
		if err := r.restoreReservation(ctx, item); err != nil {
			return report, err
		}
		if item.Status == domain.ReservationConfirmed {
			report.ConfirmedReservations++
		} else {
			report.ReservedReservations++
		}
	}
	return report, nil
}

func sumInt64(values []int64) int64 {
	var total int64
	for _, value := range values {
		total += value
	}
	return total
}

func (r *RedisMySQLRepository) restoreProductStock(ctx context.Context, productID int64, buckets []int64, reserved int64) error {
	keys := append(StockShardKeys(productID, r.shardCount), reservedStockKey(productID))
	args := make([]any, 0, len(buckets)+2)
	args = append(args, r.shardCount)
	for _, value := range buckets {
		args = append(args, value)
	}
	args = append(args, reserved)
	if _, err := evalInt64(ctx, r.redis, restoreProductStockLuaScript, keys, args...); err != nil {
		return apperror.Wrap(apperror.CodeInternal, "restore redis product stock failed", err)
	}
	return nil
}

func (r *RedisMySQLRepository) restoreReservation(ctx context.Context, item recoveryReservation) error {
	ttl := int64(confirmedReservationTTLSeconds)
	expiresAt := item.ExpiresAt.Unix()
	if item.Status == domain.ReservationReserved {
		remaining := time.Until(item.ExpiresAt)
		if remaining < 0 {
			remaining = 0
		}
		ttl = int64((remaining + time.Duration(reservationExpirySeconds)*time.Second) / time.Second)
	}
	if _, err := evalInt64(ctx, r.redis, restoreReservationLuaScript, []string{
		reservationKey(item.OrderID),
		reservationExpiryIndexKey,
		reservationProcessingIndexKey,
		reservationRetryCountKey,
		reservationDeadLetterIndexKey,
	},
		string(item.Status),
		item.ProductID,
		item.Quantity,
		item.ShardIndex,
		item.OrderID,
		ttl,
		confirmedReservationTTLSeconds,
		expiresAt,
	); err != nil {
		return apperror.Wrap(apperror.CodeInternal, "restore redis reservation failed", err)
	}
	return nil
}
