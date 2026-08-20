package job

import (
	"context"
	"sync"

	"github.com/zeromicro/go-zero/core/logx"
)

func (r *PaymentRecovery) retryInventoryFinalizations(ctx context.Context) error {
	if r.svcCtx.InventoryClient == nil {
		return nil
	}
	batchSize := r.inventoryFinalizeBatchSize()
	orderIDs, err := r.recoveryOrderIDs(ctx, `SELECT order_id FROM payment_order
WHERE status=1 AND inventory_finalize_status<>1 ORDER BY update_time LIMIT ?`, batchSize)
	if err != nil {
		return err
	}
	jobs := make(chan string)
	errors := make(chan error, 1)
	var workers sync.WaitGroup
	workerCount := r.inventoryFinalizeConcurrency()
	if workerCount > len(orderIDs) {
		workerCount = len(orderIDs)
	}
	for range workerCount {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for orderID := range jobs {
				r.finalizeInventory(ctx, orderID, errors)
			}
		}()
	}
	for _, orderID := range orderIDs {
		jobs <- orderID
	}
	close(jobs)
	workers.Wait()
	select {
	case err = <-errors:
		return err
	default:
		return nil
	}
}

func (r *PaymentRecovery) finalizeInventory(ctx context.Context, orderID string, errors chan<- error) {
	if err := r.svcCtx.InventoryClient.ConfirmDeduct(ctx, orderID); err != nil {
		if _, updateErr := r.svcCtx.SqlConn.ExecCtx(ctx, `UPDATE payment_order SET inventory_finalize_status=2,
inventory_finalize_attempts=inventory_finalize_attempts+1, inventory_finalize_error=? WHERE order_id=?`,
			shortRecoveryError(err), orderID); updateErr != nil {
			recordRecoveryError(errors, updateErr)
		}
		logx.Errorf("retry inventory finalize failed: order_id=%s err=%v", orderID, err)
		return
	}
	if _, err := r.svcCtx.SqlConn.ExecCtx(ctx, `UPDATE payment_order SET inventory_finalize_status=1,
inventory_finalize_attempts=inventory_finalize_attempts+1, inventory_finalize_error='',
inventory_finalized_at=NOW() WHERE order_id=?`, orderID); err != nil {
		recordRecoveryError(errors, err)
	}
}

func recordRecoveryError(errors chan<- error, err error) {
	select {
	case errors <- err:
	default:
	}
}

func (r *PaymentRecovery) inventoryFinalizeBatchSize() int {
	batchSize := r.svcCtx.Config.PaymentFinalizeBatchSize
	if batchSize <= 0 {
		return 200
	}
	return batchSize
}

func (r *PaymentRecovery) inventoryFinalizeConcurrency() int {
	concurrency := r.svcCtx.Config.PaymentFinalizeConcurrency
	if concurrency <= 0 {
		return 8
	}
	if concurrency > 64 {
		return 64
	}
	return concurrency
}

func (r *PaymentRecovery) retryInventoryReleases(ctx context.Context) error {
	if r.svcCtx.InventoryClient == nil {
		return nil
	}
	orderIDs, err := r.recoveryOrderIDs(ctx, `SELECT order_id FROM payment_order
WHERE status=3 AND inventory_release_status<>1 ORDER BY update_time LIMIT ?`, r.inventoryFinalizeBatchSize())
	if err != nil {
		return err
	}
	for _, orderID := range orderIDs {
		if err = r.releaseInventory(ctx, orderID, "closed payment recovery"); err != nil {
			logx.Errorf("retry inventory release failed: order_id=%s err=%v", orderID, err)
		}
	}
	return nil
}

func (r *PaymentRecovery) recoveryOrderIDs(ctx context.Context, query string, args ...any) ([]string, error) {
	db, err := r.svcCtx.SqlConn.RawDB()
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	orderIDs := make([]string, 0, r.inventoryFinalizeBatchSize())
	for rows.Next() {
		var orderID string
		if err = rows.Scan(&orderID); err != nil {
			return nil, err
		}
		orderIDs = append(orderIDs, orderID)
	}
	return orderIDs, rows.Err()
}
