package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"flash-mall/app/order/api/internal/svc"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const paymentReconcileHandlerTestDSN = "root:6494kj06@tcp(127.0.0.1:3306)/mall_order?charset=utf8mb4&parseTime=true&loc=Local"

func TestAdminPaymentReconcileHandler_RecordsPaymentOrderMismatch(t *testing.T) {
	sqlConn := sqlx.NewMysql(paymentReconcileHandlerTestDSN)
	orderID := "o-reconcile-pending"
	ensurePaymentReconcileSchema(t, sqlConn)
	cleanupPaymentReconcileRows(t, sqlConn, orderID)
	seedPaymentSuccessOrderPending(t, sqlConn, orderID)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/payments/reconcile", nil)
	rec := httptest.NewRecorder()

	AdminPaymentReconcileHandler(&svc.ServiceContext{SqlConn: sqlConn}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"issue_type":"PAYMENT_SUCCESS_ORDER_PENDING"`) {
		t.Fatalf("expected mismatch issue in response, body=%s", body)
	}
	assertPaymentReconcileIssue(t, sqlConn, orderID, "PAYMENT_SUCCESS_ORDER_PENDING")
}

func ensurePaymentReconcileSchema(t *testing.T, sqlConn sqlx.SqlConn) {
	t.Helper()

	statements := []string{
		`CREATE TABLE IF NOT EXISTS orders (
			id varchar(64) NOT NULL,
			request_id varchar(64) DEFAULT NULL,
			user_id bigint NOT NULL DEFAULT 0,
			product_id bigint NOT NULL DEFAULT 0,
			amount int NOT NULL DEFAULT 0,
			status tinyint NOT NULL DEFAULT 0,
			create_time timestamp NULL DEFAULT CURRENT_TIMESTAMP,
			update_time timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			PRIMARY KEY (id),
			UNIQUE KEY uniq_request_id (request_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS payment_order (
			id varchar(64) NOT NULL,
			order_id varchar(64) NOT NULL,
			user_id bigint NOT NULL DEFAULT 0,
			payable_amount_fen bigint NOT NULL DEFAULT 0,
			status tinyint NOT NULL DEFAULT 0,
			out_trade_no varchar(64) NOT NULL DEFAULT '',
			create_time timestamp NULL DEFAULT CURRENT_TIMESTAMP,
			update_time timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			PRIMARY KEY (id),
			UNIQUE KEY uniq_order_id (order_id),
			UNIQUE KEY uniq_out_trade_no (out_trade_no)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS payment_reconciliation_issue (
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
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}
	for _, statement := range statements {
		if _, err := sqlConn.ExecCtx(context.Background(), statement); err != nil {
			t.Fatalf("schema ensure failed: %v", err)
		}
	}
}

func cleanupPaymentReconcileRows(t *testing.T, sqlConn sqlx.SqlConn, orderID string) {
	t.Helper()

	statements := []string{
		"DELETE FROM payment_reconciliation_issue WHERE order_id = ?",
		"DELETE FROM payment_order WHERE order_id = ?",
		"DELETE FROM orders WHERE id = ?",
	}
	for _, statement := range statements {
		if _, err := sqlConn.ExecCtx(context.Background(), statement, orderID); err != nil {
			t.Fatalf("cleanup failed for %q: %v", statement, err)
		}
	}
}

func seedPaymentSuccessOrderPending(t *testing.T, sqlConn sqlx.SqlConn, orderID string) {
	t.Helper()

	if _, err := sqlConn.ExecCtx(context.Background(),
		"INSERT INTO orders (id, request_id, user_id, product_id, amount, status) VALUES (?, ?, 10001, 100, 2, 0)",
		orderID, "req-"+orderID); err != nil {
		t.Fatalf("seed order failed: %v", err)
	}
	if _, err := sqlConn.ExecCtx(context.Background(),
		"INSERT INTO payment_order (id, order_id, user_id, payable_amount_fen, status, out_trade_no) VALUES (?, ?, 10001, 19800, 1, ?)",
		"pay:"+orderID, orderID, "mock-"+orderID); err != nil {
		t.Fatalf("seed payment failed: %v", err)
	}
}

func assertPaymentReconcileIssue(t *testing.T, sqlConn sqlx.SqlConn, orderID, issueType string) {
	t.Helper()

	var got int64
	if err := sqlConn.QueryRowCtx(context.Background(), &got,
		"SELECT COUNT(*) FROM payment_reconciliation_issue WHERE order_id = ? AND issue_type = ? AND status = 'open'",
		orderID, issueType); err != nil {
		t.Fatalf("query issue failed: %v", err)
	}
	if got != 1 {
		t.Fatalf("issue count = %d, want 1", got)
	}
}
