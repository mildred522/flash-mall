package handler

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"

	"flash-mall/app/order/api/internal/svc"
	"flash-mall/app/order/api/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

func AdminPaymentReconcileHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if err := ensurePaymentReconciliationIssueTable(r.Context(), db); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		resp, err := reconcilePaymentIssues(r.Context(), db)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}

func ensurePaymentReconciliationIssueTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS payment_reconciliation_issue (
  id bigint NOT NULL AUTO_INCREMENT,
  issue_key varchar(160) NOT NULL,
  issue_type varchar(64) NOT NULL,
  order_id varchar(64) NOT NULL DEFAULT '',
  payment_order_id varchar(64) NOT NULL DEFAULT '',
  severity varchar(16) NOT NULL DEFAULT 'warning',
  status varchar(16) NOT NULL DEFAULT 'open',
  detail varchar(255) NOT NULL DEFAULT '',
  create_time timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  update_time timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  resolved_at timestamp NULL DEFAULT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uniq_issue_key (issue_key),
  KEY ix_order_id (order_id),
  KEY ix_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`)
	return err
}

func reconcilePaymentIssues(ctx context.Context, db *sql.DB) (types.PaymentReconcileResp, error) {
	resp := types.PaymentReconcileResp{Issues: make([]types.PaymentReconcileIssue, 0)}

	pendingRows, err := db.QueryContext(ctx, `
SELECT o.id, p.id, o.status, p.status
FROM payment_order p
JOIN orders o ON o.id = p.order_id
WHERE p.status = 1 AND o.status = 0`)
	if err != nil {
		return resp, err
	}
	defer pendingRows.Close()
	for pendingRows.Next() {
		var orderID, paymentOrderID string
		var orderStatus, paymentStatus int64
		if err := pendingRows.Scan(&orderID, &paymentOrderID, &orderStatus, &paymentStatus); err != nil {
			return resp, err
		}
		resp.Scanned++
		issue := types.PaymentReconcileIssue{
			IssueType:      "PAYMENT_SUCCESS_ORDER_PENDING",
			OrderId:        orderID,
			PaymentOrderId: paymentOrderID,
			Severity:       "critical",
			Detail:         fmt.Sprintf("payment status=%d but order status=%d", paymentStatus, orderStatus),
		}
		if err := upsertPaymentReconcileIssue(ctx, db, issue); err != nil {
			return resp, err
		}
		resp.Issues = append(resp.Issues, issue)
	}
	if err := pendingRows.Err(); err != nil {
		return resp, err
	}

	activeRows, err := db.QueryContext(ctx, `
SELECT o.id, p.id, o.status, p.status
FROM orders o
JOIN payment_order p ON p.order_id = o.id
WHERE o.status IN (1,3,4,5,6,7) AND p.status <> 1`)
	if err != nil {
		return resp, err
	}
	defer activeRows.Close()
	for activeRows.Next() {
		var orderID, paymentOrderID string
		var orderStatus, paymentStatus int64
		if err := activeRows.Scan(&orderID, &paymentOrderID, &orderStatus, &paymentStatus); err != nil {
			return resp, err
		}
		resp.Scanned++
		issue := types.PaymentReconcileIssue{
			IssueType:      "ORDER_ACTIVE_PAYMENT_NOT_SUCCESS",
			OrderId:        orderID,
			PaymentOrderId: paymentOrderID,
			Severity:       "critical",
			Detail:         fmt.Sprintf("order status=%d but payment status=%d", orderStatus, paymentStatus),
		}
		if err := upsertPaymentReconcileIssue(ctx, db, issue); err != nil {
			return resp, err
		}
		resp.Issues = append(resp.Issues, issue)
	}
	if err := activeRows.Err(); err != nil {
		return resp, err
	}

	return resp, nil
}

func upsertPaymentReconcileIssue(ctx context.Context, db *sql.DB, issue types.PaymentReconcileIssue) error {
	key := fmt.Sprintf("%s:%s:%s", issue.IssueType, issue.OrderId, issue.PaymentOrderId)
	_, err := db.ExecContext(ctx, `
INSERT INTO payment_reconciliation_issue (
  issue_key, issue_type, order_id, payment_order_id, severity, status, detail
) VALUES (?, ?, ?, ?, ?, 'open', ?)
ON DUPLICATE KEY UPDATE
  severity = VALUES(severity),
  status = 'open',
  detail = VALUES(detail),
  update_time = NOW(),
  resolved_at = NULL`,
		key, issue.IssueType, issue.OrderId, issue.PaymentOrderId, issue.Severity, issue.Detail,
	)
	return err
}
