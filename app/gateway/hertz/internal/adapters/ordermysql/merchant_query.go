package ordermysql

import (
	"context"

	"flash-mall/app/gateway/hertz/internal/application/orderquery"
)

func (r *QueryRepository) ListMerchantOrders(ctx context.Context, query orderquery.MerchantListQuery) ([]orderquery.BackofficeOrderItem, int64, error) {
	where := "o.merchant_id = ?"
	args := []any{query.MerchantID}
	if query.Status >= 0 {
		where += " AND o.status = ?"
		args = append(args, query.Status)
	}
	if query.UserID > 0 {
		where += " AND o.user_id = ?"
		args = append(args, query.UserID)
	}
	if query.ProductID > 0 {
		where += " AND o.product_id = ?"
		args = append(args, query.ProductID)
	}
	if query.OrderID != "" {
		where += " AND o.id = ?"
		args = append(args, query.OrderID)
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM orders o WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	queryArgs := append(append([]any{}, args...), query.PageSize, (query.Page-1)*query.PageSize)
	rows, err := r.db.QueryContext(ctx, `SELECT o.id, o.user_id, o.merchant_id, COALESCE(m.name, ''),
       o.product_id, COALESCE(s.product_name, ''), COALESCE(s.product_image_url, ''), o.amount, o.status,
       COALESCE(s.payable_amount_fen, 0), DATE_FORMAT(o.create_time, '%Y-%m-%d %H:%i:%s')
FROM orders o
LEFT JOIN order_price_snapshot s ON s.order_id = o.id
LEFT JOIN merchant m ON m.id = o.merchant_id
WHERE `+where+` ORDER BY o.create_time DESC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	items, err := scanBackofficeOrders(rows)
	return items, total, err
}

func (r *QueryRepository) ListMerchantRefunds(ctx context.Context, query orderquery.RefundListQuery) ([]orderquery.RefundItem, int64, error) {
	return r.listRefunds(ctx, query, true)
}

func (r *QueryRepository) ListAdminRefunds(ctx context.Context, query orderquery.RefundListQuery) ([]orderquery.RefundItem, int64, error) {
	return r.listRefunds(ctx, query, false)
}

func (r *QueryRepository) listRefunds(ctx context.Context, query orderquery.RefundListQuery, merchantScoped bool) ([]orderquery.RefundItem, int64, error) {
	where := "1=1"
	args := make([]any, 0, 4)
	if merchantScoped {
		where += " AND r.merchant_id = ?"
		args = append(args, query.MerchantID)
	}
	if query.Status >= 0 {
		where += " AND r.status = ?"
		args = append(args, query.Status)
	}
	if query.UserID > 0 {
		where += " AND r.user_id = ?"
		args = append(args, query.UserID)
	}
	if !merchantScoped && query.MerchantID > 0 {
		where += " AND r.merchant_id = ?"
		args = append(args, query.MerchantID)
	}
	if query.OrderID != "" {
		where += " AND r.order_id = ?"
		args = append(args, query.OrderID)
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM refund_order r WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	queryArgs := append(append([]any{}, args...), query.PageSize, (query.Page-1)*query.PageSize)
	rows, err := r.db.QueryContext(ctx, `SELECT r.id, r.order_id, r.payment_order_id, r.user_id, r.merchant_id,
       COALESCE(m.name, ''), r.product_id, r.refund_amount_fen, r.status, r.reason,
       r.audit_remark, r.operator_id, DATE_FORMAT(r.request_time, '%Y-%m-%d %H:%i:%s'),
       COALESCE(DATE_FORMAT(r.audit_time, '%Y-%m-%d %H:%i:%s'), ''),
       COALESCE(DATE_FORMAT(r.finish_time, '%Y-%m-%d %H:%i:%s'), '')
FROM refund_order r LEFT JOIN merchant m ON m.id = r.merchant_id
WHERE `+where+` ORDER BY r.create_time DESC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]orderquery.RefundItem, 0)
	for rows.Next() {
		var item orderquery.RefundItem
		if err := rows.Scan(&item.RefundID, &item.OrderID, &item.PaymentOrderID, &item.UserID, &item.MerchantID,
			&item.MerchantName, &item.ProductID, &item.RefundAmountFen, &item.Status, &item.Reason,
			&item.AuditRemark, &item.OperatorID, &item.RequestTime, &item.AuditTime, &item.FinishTime); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}
