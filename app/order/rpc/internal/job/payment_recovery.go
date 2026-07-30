package job

import (
	"context"
	"errors"
	"strings"
	"time"

	"flash-mall/app/order/rpc/internal/paymentprovider"
	"flash-mall/app/order/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type paymentRecoveryItem struct {
	OrderID        string
	PaymentOrderID string
	OutTradeNo     string
	Provider       string
}

type PaymentRecovery struct {
	svcCtx         *svc.ServiceContext
	onProviderPaid func(context.Context, ProviderPaidPayment) error
}

type ProviderPaidPayment struct {
	OrderID        string
	PaymentOrderID string
	OutTradeNo     string
	TradeNo        string
	AmountFen      int64
}

func NewPaymentRecovery(svcCtx *svc.ServiceContext) *PaymentRecovery {
	return &PaymentRecovery{svcCtx: svcCtx}
}

func (r *PaymentRecovery) OnProviderPaid(handler func(context.Context, ProviderPaidPayment) error) *PaymentRecovery {
	r.onProviderPaid = handler
	return r
}

func (r *PaymentRecovery) Start() {
	r.startLoop("payment expiry sweep", r.svcCtx.Config.PaymentSweepIntervalSec, r.closeExpiredBatch)
	r.startLoop("payment inventory recovery", r.svcCtx.Config.PaymentFinalizeIntervalSec, func(ctx context.Context) error {
		if err := r.retryInventoryFinalizations(ctx); err != nil {
			return err
		}
		return r.retryInventoryReleases(ctx)
	})
}

func (r *PaymentRecovery) startLoop(name string, intervalSec int, run func(context.Context) error) {
	interval := time.Duration(intervalSec) * time.Second
	if interval <= 0 {
		interval = 10 * time.Second
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			ctx, cancel := context.WithTimeout(context.Background(), interval)
			if err := run(ctx); err != nil {
				logx.Errorf("%s failed: %v", name, err)
			}
			cancel()
			<-ticker.C
		}
	}()
}

func (r *PaymentRecovery) RunOnce(ctx context.Context) error {
	if err := r.closeExpiredBatch(ctx); err != nil {
		return err
	}
	if err := r.retryInventoryFinalizations(ctx); err != nil {
		return err
	}
	return r.retryInventoryReleases(ctx)
}

func (r *PaymentRecovery) closeExpiredBatch(ctx context.Context) error {
	db, err := r.svcCtx.SqlConn.RawDB()
	if err != nil {
		return err
	}
	rows, err := db.QueryContext(ctx, `SELECT p.order_id, p.id, p.out_trade_no, p.provider
FROM payment_order p JOIN orders o ON o.id=p.order_id
WHERE p.status=0 AND o.status=0 AND p.expires_at IS NOT NULL AND p.expires_at<=NOW()
ORDER BY p.expires_at LIMIT 20`)
	if err != nil {
		return err
	}
	defer rows.Close()
	items := make([]paymentRecoveryItem, 0, 20)
	for rows.Next() {
		var item paymentRecoveryItem
		if err = rows.Scan(&item.OrderID, &item.PaymentOrderID, &item.OutTradeNo, &item.Provider); err != nil {
			return err
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	for _, item := range items {
		if err = r.closeExpired(ctx, item); err != nil {
			logx.Errorf("close expired payment failed: order_id=%s err=%v", item.OrderID, err)
		}
	}
	return nil
}

func (r *PaymentRecovery) closeExpired(ctx context.Context, item paymentRecoveryItem) error {
	if item.Provider == paymentprovider.NameAlipaySandbox && r.svcCtx.PaymentProvider != nil {
		query, err := r.svcCtx.PaymentProvider.Query(ctx, item.OutTradeNo)
		if err == nil && (query.Status == "TRADE_SUCCESS" || query.Status == "TRADE_FINISHED") {
			if r.onProviderPaid == nil {
				return errors.New("provider payment paid handler is not configured")
			}
			return r.onProviderPaid(ctx, ProviderPaidPayment{
				OrderID: item.OrderID, PaymentOrderID: item.PaymentOrderID,
				OutTradeNo: item.OutTradeNo, TradeNo: query.TradeNo, AmountFen: query.AmountFen,
			})
		}
		if err := r.svcCtx.PaymentProvider.Close(ctx, item.OutTradeNo); err != nil {
			_, _ = r.svcCtx.SqlConn.ExecCtx(ctx,
				"UPDATE payment_order SET provider_status='CLOSE_FAILED' WHERE order_id=?", item.OrderID)
			return err
		}
	}
	db, err := r.svcCtx.SqlConn.RawDB()
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, "UPDATE orders SET status=2, update_time=NOW() WHERE id=? AND status=0", item.OrderID)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed == 0 {
		return tx.Commit()
	}
	if _, err = tx.ExecContext(ctx, `UPDATE payment_order SET status=3,
provider_status=CASE WHEN provider='alipay_sandbox' THEN 'TRADE_CLOSED' ELSE 'CLOSED' END,
update_time=NOW() WHERE order_id=? AND status=0`, item.OrderID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO order_status_log
(order_id, from_status, to_status, operator_id, remark) VALUES (?, 0, 2, 0, 'payment expired')`, item.OrderID); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	return r.releaseInventory(ctx, item.OrderID, "payment expired")
}

func (r *PaymentRecovery) releaseInventory(ctx context.Context, orderID, reason string) error {
	if r.svcCtx.InventoryClient == nil {
		return errors.New("inventory client is not configured")
	}
	if err := r.svcCtx.InventoryClient.ReleaseStock(ctx, orderID, reason); err != nil {
		_, _ = r.svcCtx.SqlConn.ExecCtx(ctx, `UPDATE payment_order SET inventory_release_status=2,
inventory_release_attempts=inventory_release_attempts+1, inventory_release_error=? WHERE order_id=?`,
			shortRecoveryError(err), orderID)
		return err
	}
	_, err := r.svcCtx.SqlConn.ExecCtx(ctx, `UPDATE payment_order SET inventory_release_status=1,
inventory_release_attempts=inventory_release_attempts+1, inventory_release_error='',
inventory_released_at=NOW() WHERE order_id=?`, orderID)
	return err
}

func shortRecoveryError(err error) string {
	value := strings.TrimSpace(err.Error())
	if len(value) > 255 {
		return value[:255]
	}
	return value
}
