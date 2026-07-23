package ordermysql

import (
	"context"
	"database/sql"
	"errors"

	"flash-mall/app/gateway/hertz/internal/application/orderquery"
)

type QueryRepository struct{ db *sql.DB }

var _ orderquery.Repository = (*QueryRepository)(nil)

func NewQueryRepository(db *sql.DB) *QueryRepository { return &QueryRepository{db: db} }

func (r *QueryRepository) ListByUser(ctx context.Context, userID, limit int64) ([]orderquery.ListItem, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT o.id, o.product_id, o.amount, o.status,
       DATE_FORMAT(o.create_time, '%Y-%m-%d %H:%i:%s'), s.product_name, s.product_image_url, s.payable_amount_fen
FROM orders o JOIN order_price_snapshot s ON s.order_id = o.id
WHERE o.user_id = ? ORDER BY o.create_time DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]orderquery.ListItem, 0)
	for rows.Next() {
		var item orderquery.ListItem
		if err := rows.Scan(&item.OrderID, &item.ProductID, &item.Amount, &item.Status, &item.CreateTime, &item.ProductName, &item.ImageURL, &item.PayableAmountFen); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *QueryRepository) DetailByUser(ctx context.Context, orderID string, userID int64) (orderquery.Detail, bool, error) {
	var detail orderquery.Detail
	err := r.db.QueryRowContext(ctx, `SELECT o.id, o.user_id, COALESCE(o.merchant_id, 0), COALESCE(m.name, ''),
       o.product_id, o.amount, o.status, DATE_FORMAT(o.create_time, '%Y-%m-%d %H:%i:%s'),
       s.product_name, s.product_image_url, s.origin_unit_price_fen, s.sale_unit_price_fen, s.payable_amount_fen,
       s.discount_amount_fen, s.promotion_type, s.promotion_tag, p.id, p.status
FROM orders o JOIN order_price_snapshot s ON s.order_id = o.id JOIN payment_order p ON p.order_id = o.id
LEFT JOIN merchant m ON m.id = o.merchant_id WHERE o.id = ? AND o.user_id = ? LIMIT 1`, orderID, userID).Scan(
		&detail.OrderID, &detail.UserID, &detail.MerchantID, &detail.MerchantName, &detail.ProductID,
		&detail.Amount, &detail.Status, &detail.CreateTime, &detail.ProductName, &detail.ImageURL, &detail.OriginUnitPriceFen,
		&detail.SaleUnitPriceFen, &detail.PayableAmountFen, &detail.DiscountAmountFen, &detail.PromotionType,
		&detail.PromotionTag, &detail.PaymentOrderID, &detail.PaymentStatus)
	return detail, !errors.Is(err, sql.ErrNoRows), normalizeNotFound(err)
}

func (r *QueryRepository) PaymentByUser(ctx context.Context, orderID string, userID int64) (orderquery.PaymentOrder, bool, error) {
	var payment orderquery.PaymentOrder
	err := r.db.QueryRowContext(ctx, `SELECT o.id, o.user_id, o.status, p.id, p.status, p.out_trade_no, p.payable_amount_fen
FROM orders o JOIN payment_order p ON p.order_id = o.id WHERE o.id = ? AND o.user_id = ? LIMIT 1`, orderID, userID).Scan(
		&payment.OrderID, &payment.UserID, &payment.OrderStatus, &payment.PaymentOrderID,
		&payment.PaymentStatus, &payment.OutTradeNo, &payment.PayableAmountFen)
	return payment, !errors.Is(err, sql.ErrNoRows), normalizeNotFound(err)
}

func (r *QueryRepository) PaymentByID(ctx context.Context, paymentOrderID string, userID int64) (orderquery.PaymentOrder, bool, error) {
	var payment orderquery.PaymentOrder
	err := r.db.QueryRowContext(ctx, `SELECT o.id, o.user_id, o.status, p.id, p.status, p.out_trade_no, p.payable_amount_fen
FROM orders o JOIN payment_order p ON p.order_id = o.id
WHERE p.id = ? AND o.user_id = ? LIMIT 1`, paymentOrderID, userID).Scan(
		&payment.OrderID, &payment.UserID, &payment.OrderStatus, &payment.PaymentOrderID,
		&payment.PaymentStatus, &payment.OutTradeNo, &payment.PayableAmountFen)
	return payment, !errors.Is(err, sql.ErrNoRows), normalizeNotFound(err)
}

func (r *QueryRepository) PaymentByClaims(ctx context.Context, paymentOrderID, orderID, outTradeNo string) (orderquery.PaymentOrder, bool, error) {
	var payment orderquery.PaymentOrder
	err := r.db.QueryRowContext(ctx, `SELECT o.id, o.user_id, o.status, p.id, p.status, p.out_trade_no, p.payable_amount_fen
FROM orders o JOIN payment_order p ON p.order_id = o.id
WHERE p.id = ? AND p.order_id = ? AND p.out_trade_no = ? LIMIT 1`, paymentOrderID, orderID, outTradeNo).Scan(
		&payment.OrderID, &payment.UserID, &payment.OrderStatus, &payment.PaymentOrderID,
		&payment.PaymentStatus, &payment.OutTradeNo, &payment.PayableAmountFen)
	return payment, !errors.Is(err, sql.ErrNoRows), normalizeNotFound(err)
}

func (r *QueryRepository) OrderStatusByRequest(ctx context.Context, requestID string, userID int64) (string, int64, bool, error) {
	var orderID string
	var status int64
	err := r.db.QueryRowContext(ctx,
		"SELECT id, status FROM orders WHERE request_id = ? AND user_id = ? LIMIT 1",
		requestID, userID,
	).Scan(&orderID, &status)
	return orderID, status, !errors.Is(err, sql.ErrNoRows), normalizeNotFound(err)
}

func (r *QueryRepository) OrderIDByRequest(ctx context.Context, requestID string, userID int64) (string, bool, error) {
	var orderID string
	err := r.db.QueryRowContext(ctx, "SELECT id FROM orders WHERE request_id = ? AND user_id = ? LIMIT 1", requestID, userID).Scan(&orderID)
	return orderID, !errors.Is(err, sql.ErrNoRows), normalizeNotFound(err)
}

func (r *QueryRepository) CreateResultByOrder(ctx context.Context, orderID string, userID int64) (orderquery.CreateOrderResult, int64, bool, error) {
	var result orderquery.CreateOrderResult
	var status int64
	err := r.db.QueryRowContext(ctx, `SELECT o.id, o.status, COALESCE(s.payable_amount_fen, 0), COALESCE(p.id, '')
FROM orders o LEFT JOIN order_price_snapshot s ON s.order_id = o.id LEFT JOIN payment_order p ON p.order_id = o.id
WHERE o.id = ? AND o.user_id = ? LIMIT 1`, orderID, userID).Scan(&result.OrderID, &status, &result.PayableAmountFen, &result.PaymentOrderID)
	return result, status, !errors.Is(err, sql.ErrNoRows), normalizeNotFound(err)
}

func normalizeNotFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	return err
}
