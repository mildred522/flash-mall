package ordermysql

import (
	"context"

	"flash-mall/app/common/orderstatus"
	"flash-mall/app/common/paymentstatus"
)

func (r *ReconciliationRepository) Scan(ctx context.Context) (int64, error) {
	statements := []struct {
		query string
		args  []any
	}{
		{query: `INSERT INTO reconciliation_issue
  (issue_key, issue_type, order_id, payment_order_id, expected_amount_fen, actual_amount_fen, severity, status, detail)
SELECT CONCAT('payment_amount_mismatch:', o.id, ':', p.id), 'payment_amount_mismatch', o.id, p.id,
       COALESCE(s.payable_amount_fen,0), COALESCE(p.payable_amount_fen,0), 3, 0,
       'payment amount differs from order snapshot'
FROM orders o LEFT JOIN order_price_snapshot s ON s.order_id = o.id
JOIN payment_order p ON p.order_id = o.id
WHERE p.status = ? AND COALESCE(s.payable_amount_fen,0) <> COALESCE(p.payable_amount_fen,0)
ON DUPLICATE KEY UPDATE expected_amount_fen=VALUES(expected_amount_fen),
  actual_amount_fen=VALUES(actual_amount_fen), severity=VALUES(severity), status=0, detail=VALUES(detail)`,
			args: []any{paymentstatus.Success}},
		{query: `INSERT INTO reconciliation_issue
  (issue_key, issue_type, order_id, payment_order_id, severity, status, detail)
SELECT CONCAT('payment_success_order_pending:', o.id, ':', p.id), 'payment_success_order_pending', o.id, p.id,
       3, 0, 'payment succeeded while order remains pending payment'
FROM orders o JOIN payment_order p ON p.order_id = o.id
WHERE p.status = ? AND o.status = ?
ON DUPLICATE KEY UPDATE severity=VALUES(severity), status=0, detail=VALUES(detail)`,
			args: []any{paymentstatus.Success, orderstatus.PendingPayment}},
		{query: `INSERT INTO reconciliation_issue
  (issue_key, issue_type, order_id, payment_order_id, severity, status, detail)
SELECT CONCAT('order_active_payment_not_success:', o.id, ':', p.id), 'order_active_payment_not_success', o.id, p.id,
       3, 0, 'active order does not have a successful payment'
FROM orders o JOIN payment_order p ON p.order_id = o.id
WHERE o.status IN (?, ?, ?, ?, ?) AND p.status <> ?
ON DUPLICATE KEY UPDATE severity=VALUES(severity), status=0, detail=VALUES(detail)`,
			args: []any{orderstatus.Paid, orderstatus.Shipped, orderstatus.Completed,
				orderstatus.RefundRequested, orderstatus.Refunded, paymentstatus.Success}},
	}
	var affected int64
	for _, statement := range statements {
		result, err := r.db.ExecContext(ctx, statement.query, statement.args...)
		if err != nil {
			return 0, err
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return 0, err
		}
		affected += rows
	}
	return affected, nil
}
