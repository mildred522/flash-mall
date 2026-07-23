package ordermysql

import (
	"context"

	"flash-mall/app/gateway/hertz/internal/application/reconciliation"
)

func (r *ReconciliationRepository) List(
	ctx context.Context,
	query reconciliation.Query,
) (reconciliation.List, error) {
	where := "1=1"
	args := make([]any, 0, 3)
	if query.Status >= 0 {
		where += " AND status = ?"
		args = append(args, query.Status)
	}
	if query.IssueType != "" {
		where += " AND issue_type = ?"
		args = append(args, query.IssueType)
	}
	if query.OrderID != "" {
		where += " AND order_id = ?"
		args = append(args, query.OrderID)
	}
	var total int64
	if err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM reconciliation_issue WHERE "+where, args...).Scan(&total); err != nil {
		return reconciliation.List{}, err
	}
	queryArgs := append(append([]any{}, args...), query.PageSize, (query.Page-1)*query.PageSize)
	rows, err := r.db.QueryContext(ctx, `SELECT id, issue_type, order_id, payment_order_id, refund_order_id,
       expected_amount_fen, actual_amount_fen, severity, status, detail,
       COALESCE(DATE_FORMAT(create_time, '%Y-%m-%d %H:%i:%s'), '')
FROM reconciliation_issue WHERE `+where+` ORDER BY id DESC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return reconciliation.List{}, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]reconciliation.Issue, 0)
	for rows.Next() {
		var item reconciliation.Issue
		if err := rows.Scan(&item.ID, &item.IssueType, &item.OrderID, &item.PaymentOrderID,
			&item.RefundOrderID, &item.ExpectedAmountFen, &item.ActualAmountFen, &item.Severity,
			&item.Status, &item.Detail, &item.CreateTime); err != nil {
			return reconciliation.List{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return reconciliation.List{}, err
	}
	return reconciliation.List{Items: items, Total: total}, nil
}
