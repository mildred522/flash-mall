package job

import (
	"context"

	"github.com/zeromicro/go-zero/core/logx"
)

func (r *PaymentRecovery) retryInventoryFinalizations(ctx context.Context) error {
	if r.svcCtx.InventoryClient == nil {
		return nil
	}
	orderIDs, err := r.recoveryOrderIDs(ctx, `SELECT order_id FROM payment_order
WHERE status=1 AND inventory_finalize_status<>1 ORDER BY update_time LIMIT 20`)
	if err != nil {
		return err
	}
	for _, orderID := range orderIDs {
		if err = r.svcCtx.InventoryClient.ConfirmDeduct(ctx, orderID); err != nil {
			_, _ = r.svcCtx.SqlConn.ExecCtx(ctx, `UPDATE payment_order SET inventory_finalize_status=2,
inventory_finalize_attempts=inventory_finalize_attempts+1, inventory_finalize_error=? WHERE order_id=?`,
				shortRecoveryError(err), orderID)
			logx.Errorf("retry inventory finalize failed: order_id=%s err=%v", orderID, err)
			continue
		}
		_, err = r.svcCtx.SqlConn.ExecCtx(ctx, `UPDATE payment_order SET inventory_finalize_status=1,
inventory_finalize_attempts=inventory_finalize_attempts+1, inventory_finalize_error='',
inventory_finalized_at=NOW() WHERE order_id=?`, orderID)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *PaymentRecovery) retryInventoryReleases(ctx context.Context) error {
	if r.svcCtx.InventoryClient == nil {
		return nil
	}
	orderIDs, err := r.recoveryOrderIDs(ctx, `SELECT order_id FROM payment_order
WHERE status=3 AND inventory_release_status<>1 ORDER BY update_time LIMIT 20`)
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

func (r *PaymentRecovery) recoveryOrderIDs(ctx context.Context, query string) ([]string, error) {
	db, err := r.svcCtx.SqlConn.RawDB()
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	orderIDs := make([]string, 0, 20)
	for rows.Next() {
		var orderID string
		if err = rows.Scan(&orderID); err != nil {
			return nil, err
		}
		orderIDs = append(orderIDs, orderID)
	}
	return orderIDs, rows.Err()
}
