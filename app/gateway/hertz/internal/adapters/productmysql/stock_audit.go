package productmysql

import (
	"context"
	"database/sql"

	"flash-mall/app/gateway/hertz/internal/application/stockaudit"
)

type StockAuditRepository struct{ db *sql.DB }

var _ stockaudit.Repository = (*StockAuditRepository)(nil)

func NewStockAuditRepository(db *sql.DB) *StockAuditRepository { return &StockAuditRepository{db: db} }

func (r *StockAuditRepository) List(ctx context.Context, query stockaudit.Query) ([]stockaudit.Item, int64, error) {
	where := "1=1"
	args := make([]any, 0, 4)
	if query.ProductID > 0 {
		where += " AND l.product_id = ?"
		args = append(args, query.ProductID)
	}
	if query.OrderID != "" {
		where += " AND l.order_id = ?"
		args = append(args, query.OrderID)
	}
	if query.ChangeType != "" {
		where += " AND l.change_type = ?"
		args = append(args, query.ChangeType)
	}
	if query.MerchantID > 0 {
		where += " AND EXISTS (SELECT 1 FROM mall_product.product p WHERE p.id = l.product_id AND p.merchant_id = ?)"
		args = append(args, query.MerchantID)
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mall_product.inventory_stock_change_log l WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	queryArgs := append(append([]any{}, args...), query.PageSize, (query.Page-1)*query.PageSize)
	rows, err := r.db.QueryContext(ctx, `SELECT id, product_id, order_id, change_type, delta,
       before_available, after_available, reason, request_id, trace_id,
       operator_user_id, operator_merchant_id, operator_role,
       COALESCE(DATE_FORMAT(create_time, '%Y-%m-%d %H:%i:%s'), '')
FROM mall_product.inventory_stock_change_log l
WHERE `+where+`
ORDER BY id DESC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]stockaudit.Item, 0)
	for rows.Next() {
		var item stockaudit.Item
		if err := rows.Scan(&item.ID, &item.ProductID, &item.OrderID, &item.ChangeType, &item.Delta,
			&item.BeforeAvailable, &item.AfterAvailable, &item.Reason, &item.RequestID, &item.TraceID,
			&item.OperatorUserID, &item.OperatorMerchantID, &item.OperatorRole, &item.CreateTime); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}
