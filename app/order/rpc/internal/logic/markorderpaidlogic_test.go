package logic

import (
	"context"
	"fmt"
	"testing"

	"flash-mall/app/order/rpc/internal/config"
	"flash-mall/app/order/rpc/internal/svc"
	orderpb "flash-mall/app/order/rpc/order"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const markPaidTestDSN = "root:6494kj06@tcp(127.0.0.1:3306)/mall_order?charset=utf8mb4&parseTime=true&loc=Local"

func TestMarkOrderPaidLogic_MarkPaid_IsIdempotent(t *testing.T) {
	svcCtx := newMarkPaidServiceContext()
	orderID := "o-paid-1"
	paymentOrderID := "pay:o-paid-1"

	ensureMarkPaidSchema(t, svcCtx)
	cleanupMarkPaidRows(t, svcCtx, orderID, paymentOrderID)
	seedPendingPaymentOrder(t, svcCtx, orderID, paymentOrderID)

	l := NewMarkOrderPaidLogic(context.Background(), svcCtx)

	first, err := l.MarkPaid(&orderpb.MarkOrderPaidReq{
		OrderId:        orderID,
		PaymentOrderId: paymentOrderID,
		OutTradeNo:     "mock-o-paid-1",
		CallbackBody:   paymentCallbackBody(9900, "evt-o-paid-1"),
	})
	if err != nil || !first.Updated {
		t.Fatalf("first callback should update, resp=%#v err=%v", first, err)
	}
	assertMarkPaidLogCount(t, svcCtx, orderID, 0, 1, 1)
	assertMarkPaidOutboxCount(t, svcCtx, orderID, "order.paid", 1)

	second, err := l.MarkPaid(&orderpb.MarkOrderPaidReq{
		OrderId:        orderID,
		PaymentOrderId: paymentOrderID,
		OutTradeNo:     "mock-o-paid-1",
		CallbackBody:   paymentCallbackBody(9900, "evt-o-paid-1"),
	})
	if err != nil || second.Updated {
		t.Fatalf("second callback should be idempotent, resp=%#v err=%v", second, err)
	}
}

func TestMarkOrderPaidLogic_MarkPaid_RejectsPaymentOrderForDifferentOrder(t *testing.T) {
	svcCtx := newMarkPaidServiceContext()
	orderA := "o-bind-a"
	paymentA := "pay:o-bind-a"
	orderB := "o-bind-b"
	paymentB := "pay:o-bind-b"

	ensureMarkPaidSchema(t, svcCtx)
	cleanupMarkPaidRows(t, svcCtx, orderA, paymentA)
	cleanupMarkPaidRows(t, svcCtx, orderB, paymentB)
	seedPendingPaymentOrder(t, svcCtx, orderA, paymentA)
	seedPendingPaymentOrder(t, svcCtx, orderB, paymentB)

	l := NewMarkOrderPaidLogic(context.Background(), svcCtx)

	resp, err := l.MarkPaid(&orderpb.MarkOrderPaidReq{
		OrderId:        orderA,
		PaymentOrderId: paymentB,
		OutTradeNo:     "mock-o-bind-b",
		CallbackBody:   paymentCallbackBody(9900, "evt-bind-mismatch"),
	})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected not found for mismatched order/payment binding, resp=%#v err=%v", resp, err)
	}
	if got := queryOrderStatus(t, svcCtx, orderA); got != 0 {
		t.Fatalf("order %s should remain pending, got status=%d", orderA, got)
	}
	if got := queryPaymentStatus(t, svcCtx, paymentB); got != 0 {
		t.Fatalf("payment %s should remain pending, got status=%d", paymentB, got)
	}
}

func TestMarkOrderPaidLogic_MarkPaid_RejectsAmountMismatch(t *testing.T) {
	svcCtx := newMarkPaidServiceContext()
	orderID := "o-amount-mismatch"
	paymentOrderID := "pay:o-amount-mismatch"

	ensureMarkPaidSchema(t, svcCtx)
	cleanupMarkPaidRows(t, svcCtx, orderID, paymentOrderID)
	seedPendingPaymentOrder(t, svcCtx, orderID, paymentOrderID)

	l := NewMarkOrderPaidLogic(context.Background(), svcCtx)

	resp, err := l.MarkPaid(&orderpb.MarkOrderPaidReq{
		OrderId:        orderID,
		PaymentOrderId: paymentOrderID,
		OutTradeNo:     "mock-o-amount-mismatch",
		CallbackBody:   paymentCallbackBody(1, "evt-amount-mismatch"),
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected failed precondition for amount mismatch, resp=%#v err=%v", resp, err)
	}
	if got := queryOrderStatus(t, svcCtx, orderID); got != 0 {
		t.Fatalf("order should remain pending, got status=%d", got)
	}
	if got := queryPaymentStatus(t, svcCtx, paymentOrderID); got != 0 {
		t.Fatalf("payment should remain pending, got status=%d", got)
	}
}

func newMarkPaidServiceContext() *svc.ServiceContext {
	return &svc.ServiceContext{
		Config:  config.Config{DataSource: markPaidTestDSN},
		SqlConn: sqlx.NewMysql(markPaidTestDSN),
	}
}

func ensureMarkPaidSchema(t *testing.T, svcCtx *svc.ServiceContext) {
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
			paid_at timestamp NULL DEFAULT NULL,
			callback_payload json DEFAULT NULL,
			create_time timestamp NULL DEFAULT CURRENT_TIMESTAMP,
			update_time timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			PRIMARY KEY (id),
			UNIQUE KEY uniq_order_id (order_id),
			UNIQUE KEY uniq_out_trade_no (out_trade_no)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS payment_callback_event (
			id bigint NOT NULL AUTO_INCREMENT,
			provider varchar(32) NOT NULL DEFAULT 'mock',
			event_id varchar(128) NOT NULL DEFAULT '',
			payment_order_id varchar(64) NOT NULL,
			order_id varchar(64) NOT NULL,
			out_trade_no varchar(64) NOT NULL,
			paid_amount_fen bigint NOT NULL DEFAULT 0,
			signature_valid tinyint NOT NULL DEFAULT 1,
			process_status varchar(32) NOT NULL DEFAULT 'SUCCESS',
			error_message varchar(255) NOT NULL DEFAULT '',
			raw_payload json DEFAULT NULL,
			create_time timestamp NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (id),
			UNIQUE KEY uniq_provider_event (provider, event_id),
			KEY ix_payment_order_id (payment_order_id),
			KEY ix_order_id (order_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS order_status_log (
			id bigint NOT NULL AUTO_INCREMENT,
			order_id varchar(64) NOT NULL,
			from_status tinyint NOT NULL,
			to_status tinyint NOT NULL,
			operator_id bigint NOT NULL DEFAULT 0,
			remark varchar(255) NOT NULL DEFAULT '',
			create_time timestamp NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (id),
			KEY ix_order_id (order_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS order_outbox (
			id bigint NOT NULL AUTO_INCREMENT,
			event_id varchar(128) NOT NULL,
			event_type varchar(64) NOT NULL,
			aggregate_id varchar(64) NOT NULL,
			payload json NOT NULL,
			status tinyint NOT NULL DEFAULT 0,
			attempt_count int NOT NULL DEFAULT 0,
			next_retry_at timestamp NULL DEFAULT CURRENT_TIMESTAMP,
			published_at timestamp NULL DEFAULT NULL,
			last_error varchar(255) NOT NULL DEFAULT '',
			create_time timestamp NULL DEFAULT CURRENT_TIMESTAMP,
			update_time timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			PRIMARY KEY (id),
			UNIQUE KEY uniq_event_id (event_id),
			KEY ix_status_retry (status, next_retry_at),
			KEY ix_aggregate_id (aggregate_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}

	for _, statement := range statements {
		if _, err := svcCtx.SqlConn.ExecCtx(context.Background(), statement); err != nil {
			t.Fatalf("schema ensure failed for %q: %v", statement, err)
		}
	}

	ensurePaymentOrderColumn(t, svcCtx, "paid_at", "ALTER TABLE payment_order ADD COLUMN paid_at timestamp NULL DEFAULT NULL")
	ensurePaymentOrderColumn(t, svcCtx, "callback_payload", "ALTER TABLE payment_order ADD COLUMN callback_payload json DEFAULT NULL")
}

func cleanupMarkPaidRows(t *testing.T, svcCtx *svc.ServiceContext, orderID, paymentOrderID string) {
	t.Helper()

	statements := []string{
		fmt.Sprintf("DELETE FROM order_outbox WHERE aggregate_id = '%s'", orderID),
		fmt.Sprintf("DELETE FROM order_status_log WHERE order_id = '%s'", orderID),
		fmt.Sprintf("DELETE FROM payment_callback_event WHERE order_id = '%s' OR payment_order_id = '%s'", orderID, paymentOrderID),
		fmt.Sprintf("DELETE FROM payment_order WHERE id = '%s'", paymentOrderID),
		fmt.Sprintf("DELETE FROM orders WHERE id = '%s'", orderID),
	}

	for _, statement := range statements {
		if _, err := svcCtx.SqlConn.ExecCtx(context.Background(), statement); err != nil {
			t.Fatalf("cleanup failed for %q: %v", statement, err)
		}
	}
}

func assertMarkPaidLogCount(t *testing.T, svcCtx *svc.ServiceContext, orderID string, fromStatus, toStatus, want int64) {
	t.Helper()

	var got int64
	if err := svcCtx.SqlConn.QueryRowCtx(context.Background(), &got,
		"SELECT COUNT(*) FROM order_status_log WHERE order_id = ? AND from_status = ? AND to_status = ?",
		orderID, fromStatus, toStatus); err != nil {
		t.Fatalf("query status log failed: %v", err)
	}
	if got != want {
		t.Fatalf("status log count = %d, want %d", got, want)
	}
}

func assertMarkPaidOutboxCount(t *testing.T, svcCtx *svc.ServiceContext, orderID, eventType string, want int64) {
	t.Helper()

	var got int64
	if err := svcCtx.SqlConn.QueryRowCtx(context.Background(), &got,
		"SELECT COUNT(*) FROM order_outbox WHERE aggregate_id = ? AND event_type = ?",
		orderID, eventType); err != nil {
		t.Fatalf("query outbox failed: %v", err)
	}
	if got != want {
		t.Fatalf("outbox count = %d, want %d", got, want)
	}
}

func seedPendingPaymentOrder(t *testing.T, svcCtx *svc.ServiceContext, orderID, paymentOrderID string) {
	t.Helper()

	statements := []string{
		fmt.Sprintf("INSERT INTO orders (id, request_id, user_id, product_id, amount, status) VALUES ('%s', 'req-%s', 1, 100, 1, 0)", orderID, orderID),
		fmt.Sprintf("INSERT INTO payment_order (id, order_id, user_id, payable_amount_fen, status, out_trade_no) VALUES ('%s', '%s', 1, 9900, 0, 'mock-%s')", paymentOrderID, orderID, orderID),
	}

	for _, statement := range statements {
		if _, err := svcCtx.SqlConn.ExecCtx(context.Background(), statement); err != nil {
			t.Fatalf("seed failed for %q: %v", statement, err)
		}
	}
}

func paymentCallbackBody(paidAmountFen int64, eventID string) string {
	return fmt.Sprintf(`{"trade_status":"SUCCESS","provider":"mock","event_id":"%s","paid_amount_fen":%d}`, eventID, paidAmountFen)
}

func queryOrderStatus(t *testing.T, svcCtx *svc.ServiceContext, orderID string) int64 {
	t.Helper()

	var got int64
	if err := svcCtx.SqlConn.QueryRowCtx(context.Background(), &got, "SELECT status FROM orders WHERE id = ?", orderID); err != nil {
		t.Fatalf("query order status failed: %v", err)
	}
	return got
}

func queryPaymentStatus(t *testing.T, svcCtx *svc.ServiceContext, paymentOrderID string) int64 {
	t.Helper()

	var got int64
	if err := svcCtx.SqlConn.QueryRowCtx(context.Background(), &got, "SELECT status FROM payment_order WHERE id = ?", paymentOrderID); err != nil {
		t.Fatalf("query payment status failed: %v", err)
	}
	return got
}

func ensurePaymentOrderColumn(t *testing.T, svcCtx *svc.ServiceContext, columnName string, alterSQL string) {
	t.Helper()

	var count int64
	query := `SELECT COUNT(*) AS count
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = DATABASE()
  AND TABLE_NAME = 'payment_order'
  AND COLUMN_NAME = ?`
	if err := svcCtx.SqlConn.QueryRowCtx(context.Background(), &count, query, columnName); err != nil {
		t.Fatalf("column check failed for %s: %v", columnName, err)
	}
	if count > 0 {
		return
	}

	if _, err := svcCtx.SqlConn.ExecCtx(context.Background(), alterSQL); err != nil {
		t.Fatalf("column add failed for %s using %q: %v", columnName, alterSQL, err)
	}
}
