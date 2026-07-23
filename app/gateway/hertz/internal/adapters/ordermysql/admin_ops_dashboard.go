package ordermysql

import (
	"context"

	"flash-mall/app/common/orderstatus"
	"flash-mall/app/gateway/hertz/internal/application/adminops"
)

func (r *AdminOpsRepository) Dashboard(ctx context.Context) (adminops.DashboardStats, error) {
	var stats adminops.DashboardStats
	err := r.db.QueryRowContext(ctx, `SELECT
  (SELECT COUNT(*) FROM orders),
  (SELECT COALESCE(SUM(s.payable_amount_fen), 0) FROM orders o
     JOIN order_price_snapshot s ON s.order_id = o.id WHERE o.status IN (?, ?, ?)),
  (SELECT COUNT(*) FROM mall_product.product),
  (SELECT COUNT(*) FROM mall_product.supplier),
  (SELECT COUNT(*) FROM mall_product.promotion_rule),
  (SELECT COUNT(*) FROM mall_product.promotion_rule
     WHERE status = 1 AND (starts_at IS NULL OR starts_at <= NOW()) AND (ends_at IS NULL OR ends_at >= NOW())),
  (SELECT COUNT(*) FROM (SELECT p.id, COALESCE(snap.available, p.stock, 0) AS available
     FROM mall_product.product p LEFT JOIN mall_product.product_stock_snapshot snap ON snap.product_id = p.id
     WHERE p.status = 1 HAVING available > 0 AND available <= 100) low_stock),
  (SELECT COUNT(*) FROM (SELECT p.id, COALESCE(snap.available, p.stock, 0) AS available
     FROM mall_product.product p LEFT JOIN mall_product.product_stock_snapshot snap ON snap.product_id = p.id
     WHERE p.status = 1 HAVING available = 0) out_of_stock),
  (SELECT COUNT(*) FROM orders WHERE status = ?),
  (SELECT COUNT(*) FROM orders WHERE status = ?),
  (SELECT COUNT(*) FROM orders WHERE status = ?),
  (SELECT COUNT(*) FROM orders WHERE status = ?),
  (SELECT COUNT(*) FROM orders WHERE status = ?),
  (SELECT COUNT(*) FROM orders WHERE status = ?),
  (SELECT COUNT(*) FROM reconciliation_issue WHERE status = 0),
  (SELECT COUNT(*) FROM order_outbox WHERE status IN (0, 2)),
  (SELECT COUNT(*) FROM order_outbox WHERE status = 3)`,
		orderstatus.Paid, orderstatus.Shipped, orderstatus.Completed,
		orderstatus.PendingPayment, orderstatus.Paid, orderstatus.Shipped,
		orderstatus.Completed, orderstatus.RefundRequested, orderstatus.Refunded,
	).Scan(
		&stats.TotalOrders, &stats.TotalRevenueFen, &stats.TotalProducts, &stats.TotalSuppliers,
		&stats.TotalPromotions, &stats.ActivePromotions, &stats.LowStockProducts, &stats.OutOfStockProducts,
		&stats.PendingOrders, &stats.PaidOrders, &stats.ShippedOrders, &stats.CompletedOrders,
		&stats.RefundRequested, &stats.RefundedOrders, &stats.OpenReconIssues, &stats.PendingEvents, &stats.DeadEvents,
	)
	return stats, err
}
