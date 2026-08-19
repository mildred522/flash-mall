package repository

import (
	"context"
	"fmt"
	"time"

	"flash-mall/app/common/apperror"
	"flash-mall/app/inventory/domain"
)

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
	if r.db != nil {
		if err := r.seedMySQLBuckets(ctx, productID, total, shardCount); err != nil {
			return err
		}
	}
	if err := r.seedRedis(ctx, productID, total, shardCount); err != nil {
		return err
	}
	return r.refreshStockSnapshot(ctx, productID)
}

func (r *RedisMySQLRepository) AdjustStock(ctx context.Context, productID int64, delta int64, bucketIdx int, meta domain.StockChangeMeta) (domain.Stock, domain.Stock, error) {
	before, err := r.GetStock(ctx, productID)
	if err != nil {
		return domain.Stock{}, domain.Stock{}, err
	}
	if r.db != nil {
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
	keys := append(StockShardKeys(productID, r.shardCount), reservationKey(orderID), reservedStockKey(productID), reservationExpiryIndexKey)
	ret, err := evalInt64(ctx, r.redis, reserveStockLuaScript, keys, quantity, reservationKeyTTLSeconds, r.shardCount, StockShardStartIndex(orderID, r.shardCount), productID, orderID, reservationExpirySeconds)
	if err != nil {
		return apperror.Wrap(apperror.CodeStockReserveFailed, "reserve stock failed", err)
	}
	switch ret {
	case 1:
		reservation, readErr := evalReservation(ctx, r.redis, getReservationLuaScript, []string{reservationKey(orderID)})
		if readErr != nil {
			return apperror.Wrap(apperror.CodeStockReserveFailed, "read reserved stock identity failed", readErr)
		}
		if err := r.recordReservation(ctx, reservationLedgerRecord{
			OrderID:    orderID,
			ProductID:  productID,
			Quantity:   quantity,
			ShardIndex: reservation.ShardIndex,
			Status:     domain.ReservationReserved,
			ExpiresAt:  time.Now().Add(time.Duration(reservationExpirySeconds) * time.Second),
			RequestID:  meta.RequestID,
			TraceID:    meta.TraceID,
		}); err != nil {
			return err
		}
		after, _ := r.GetStock(ctx, productID)
		_ = r.insertStockChangeLog(ctx, "RESERVE", productID, orderID, -quantity, -1, before, after, meta)
		_ = r.upsertStockSnapshot(ctx, after)
		return nil
	case -1:
		return domain.ErrStockNotFound
	case -2:
		return domain.ErrStockInsufficient
	case -3:
		return domain.ErrReservationConflict
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
	ret, err := evalInt64(ctx, r.redis, confirmDeductLuaScript, []string{reservationKey(orderID), reservationExpiryIndexKey}, confirmedReservationTTLSeconds, finalDeductFlag, orderID)
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
	if reservation.ProductID > 0 {
		if err := r.transitionReservationLedger(ctx, orderID, domain.ReservationConfirmed, domain.ReservationReserved); err != nil {
			return err
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
		ledgerRecord, ok, err := r.loadReservationLedger(ctx, orderID)
		if err != nil {
			return err
		}
		if !ok || ledgerRecord.ProductID <= 0 || ledgerRecord.Quantity <= 0 || ledgerRecord.Status == domain.ReservationReleased {
			return nil
		}
		if _, err := evalInt64(ctx, r.redis, hydrateReservationLuaScript, []string{reservationKey(orderID)},
			string(ledgerRecord.Status), ledgerRecord.ProductID, ledgerRecord.Quantity, ledgerRecord.ShardIndex, orderID, reservationKeyTTLSeconds,
		); err != nil {
			return apperror.Wrap(apperror.CodeInternal, "restore stock reservation from ledger failed", err)
		}
		reservation = redisReservation{ProductID: ledgerRecord.ProductID, Quantity: ledgerRecord.Quantity, ShardIndex: ledgerRecord.ShardIndex}
	}
	before, _ := r.GetStock(ctx, reservation.ProductID)
	if r.finalDeductEnabled && r.db != nil {
		if err := r.releaseMySQLDeduct(ctx, orderID, reservation.ProductID, reservation.Quantity); err != nil {
			return err
		}
	}
	keys := append(StockShardKeys(reservation.ProductID, r.shardCount), reservationKey(orderID), reservedStockKey(reservation.ProductID), reservationExpiryIndexKey)
	_, err = evalInt64(ctx, r.redis, releaseStockLuaScript, keys, confirmedReservationTTLSeconds, r.shardCount)
	if err != nil {
		return apperror.Wrap(apperror.CodeInternal, "release stock failed", err)
	}
	if err := r.transitionReservationLedger(ctx, orderID, domain.ReservationReleased, domain.ReservationReserved, domain.ReservationConfirmed); err != nil {
		return err
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
	ledgerReserved, err := r.activeReservationTotal(ctx, productID)
	if err != nil {
		return before, before, false, err
	}
	reserved := expectedReserved(before.Reserved, ledgerReserved, r.reservationLedgerMode)
	after := domain.Stock{
		ProductID: productID,
		Available: expectedAvailable(total, before.Reserved, ledgerReserved, r.reservationLedgerMode),
		Reserved:  reserved,
		Total:     total,
	}
	changed := before.Available != after.Available || before.Reserved != after.Reserved
	if changed {
		if err := r.seedRedisState(ctx, productID, after.Available, after.Reserved, r.shardCount); err != nil {
			return before, after, false, err
		}
		_ = r.insertStockChangeLog(ctx, "RECONCILE", productID, "", after.Available-before.Available, -1, before, after, meta)
		_ = r.upsertStockSnapshot(ctx, after)
	}
	return before, after, changed, nil
}
