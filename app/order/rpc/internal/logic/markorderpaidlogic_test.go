package logic

import (
	"context"
	"fmt"
	"testing"

	"flash-mall/app/order/rpc/internal/config"
	"flash-mall/app/order/rpc/internal/svc"
	orderpb "flash-mall/app/order/rpc/order"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
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
		CallbackBody:   `{"trade_status":"SUCCESS"}`,
	})
	if err != nil || !first.Updated {
		t.Fatalf("first callback should update, resp=%#v err=%v", first, err)
	}

	second, err := l.MarkPaid(&orderpb.MarkOrderPaidReq{
		OrderId:        orderID,
		PaymentOrderId: paymentOrderID,
		OutTradeNo:     "mock-o-paid-1",
		CallbackBody:   `{"trade_status":"SUCCESS"}`,
	})
	if err != nil || second.Updated {
		t.Fatalf("second callback should be idempotent, resp=%#v err=%v", second, err)
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
		fmt.Sprintf("DELETE FROM payment_order WHERE id = '%s'", paymentOrderID),
		fmt.Sprintf("DELETE FROM orders WHERE id = '%s'", orderID),
	}

	for _, statement := range statements {
		if _, err := svcCtx.SqlConn.ExecCtx(context.Background(), statement); err != nil {
			t.Fatalf("cleanup failed for %q: %v", statement, err)
		}
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
