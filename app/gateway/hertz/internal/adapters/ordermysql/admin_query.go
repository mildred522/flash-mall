package ordermysql

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"flash-mall/app/gateway/hertz/internal/application/orderquery"
)

var _ orderquery.BackofficeRepository = (*QueryRepository)(nil)

func (r *QueryRepository) ListOrders(ctx context.Context, query orderquery.AdminListQuery) ([]orderquery.BackofficeOrderItem, int64, error) {
	where, args := adminOrderWhere(query)
	var total int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM orders o LEFT JOIN order_price_snapshot s ON s.order_id = o.id WHERE "+where, args...).Scan(&total); err != nil {
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

func (r *QueryRepository) DetailByID(ctx context.Context, orderID string) (orderquery.Detail, bool, error) {
	var detail orderquery.Detail
	err := r.db.QueryRowContext(ctx, `SELECT o.id, o.user_id, COALESCE(o.merchant_id, 0), COALESCE(m.name, ''),
       o.product_id, o.amount, o.status, DATE_FORMAT(o.create_time, '%Y-%m-%d %H:%i:%s'),
       s.product_name, s.product_image_url, s.origin_unit_price_fen, s.sale_unit_price_fen, s.payable_amount_fen,
       s.discount_amount_fen, s.promotion_type, s.promotion_tag, p.id, p.status
FROM orders o
JOIN order_price_snapshot s ON s.order_id = o.id
JOIN payment_order p ON p.order_id = o.id
LEFT JOIN merchant m ON m.id = o.merchant_id
WHERE o.id = ? LIMIT 1`, orderID).Scan(
		&detail.OrderID, &detail.UserID, &detail.MerchantID, &detail.MerchantName, &detail.ProductID,
		&detail.Amount, &detail.Status, &detail.CreateTime, &detail.ProductName, &detail.ImageURL, &detail.OriginUnitPriceFen,
		&detail.SaleUnitPriceFen, &detail.PayableAmountFen, &detail.DiscountAmountFen, &detail.PromotionType,
		&detail.PromotionTag, &detail.PaymentOrderID, &detail.PaymentStatus)
	return detail, !errors.Is(err, sql.ErrNoRows), normalizeNotFound(err)
}

func (r *QueryRepository) StatusLogs(ctx context.Context, orderID string) ([]orderquery.StatusLogItem, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, order_id, from_status, to_status, operator_id, remark,
       DATE_FORMAT(create_time, '%Y-%m-%d %H:%i:%s')
FROM order_status_log WHERE order_id = ? ORDER BY id ASC`, orderID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]orderquery.StatusLogItem, 0)
	for rows.Next() {
		var item orderquery.StatusLogItem
		if err := rows.Scan(&item.ID, &item.OrderID, &item.FromStatus, &item.ToStatus, &item.OperatorID, &item.Remark, &item.CreateTime); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func adminOrderWhere(query orderquery.AdminListQuery) (string, []any) {
	where := "1=1"
	args := make([]any, 0, 8)
	filters := []struct {
		active bool
		clause string
		value  any
	}{
		{query.Status >= 0, "o.status = ?", query.Status},
		{query.UserID > 0, "o.user_id = ?", query.UserID},
		{query.MerchantID > 0, "o.merchant_id = ?", query.MerchantID},
		{query.ProductID > 0, "o.product_id = ?", query.ProductID},
		{query.ProductName != "", "s.product_name LIKE ?", "%" + query.ProductName + "%"},
		{query.CreatedFrom != "", "o.create_time >= ?", normalizeDateLower(query.CreatedFrom)},
		{query.CreatedTo != "", "o.create_time <= ?", normalizeDateUpper(query.CreatedTo)},
		{query.OrderID != "", "o.id = ?", query.OrderID},
	}
	for _, filter := range filters {
		if filter.active {
			where += " AND " + filter.clause
			args = append(args, filter.value)
		}
	}
	return where, args
}

func normalizeDateLower(value string) string {
	value = strings.TrimSpace(value)
	if len(value) == len("2006-01-02") {
		return value + " 00:00:00"
	}
	return value
}

func normalizeDateUpper(value string) string {
	value = strings.TrimSpace(value)
	if len(value) == len("2006-01-02") {
		return value + " 23:59:59"
	}
	return value
}

type orderRows interface {
	Next() bool
	Scan(...any) error
	Err() error
}

func scanBackofficeOrders(rows orderRows) ([]orderquery.BackofficeOrderItem, error) {
	items := make([]orderquery.BackofficeOrderItem, 0)
	for rows.Next() {
		var item orderquery.BackofficeOrderItem
		if err := rows.Scan(&item.OrderID, &item.UserID, &item.MerchantID, &item.MerchantName, &item.ProductID,
			&item.ProductName, &item.ImageURL, &item.Amount, &item.Status, &item.PayableAmountFen, &item.CreateTime); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
