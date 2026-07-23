package handler

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"flash-mall/app/gateway/hertz/internal/adapters/ordermysql"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const reconciliationTestDSN = "root:6494kj06@tcp(127.0.0.1:3307)/mall_order?charset=utf8mb4&parseTime=true&loc=Local"

func TestScanGatewayReconciliationIssues_CoversStatusMismatchesWithoutDuplicates(t *testing.T) {
	conn := sqlx.NewMysql(reconciliationTestDSN)
	db, err := conn.RawDB()
	if err != nil {
		t.Fatalf("open order db: %v", err)
	}
	prefix := fmt.Sprintf("reconcile-%d", time.Now().UnixNano())
	cases := []struct {
		orderID, issueType string
		orderStatus        int64
		paymentStatus      int64
		expected, actual   int64
	}{
		{prefix + "-amount", "payment_amount_mismatch", 1, 1, 9900, 9800},
		{prefix + "-pending", "payment_success_order_pending", 0, 1, 9900, 9900},
		{prefix + "-active", "order_active_payment_not_success", 3, 0, 9900, 9900},
	}
	for _, tc := range cases {
		seedReconciliationCase(t, db, tc.orderID, tc.orderStatus, tc.paymentStatus, tc.expected, tc.actual)
	}
	t.Cleanup(func() {
		for _, tc := range cases {
			cleanupReconciliationCase(db, tc.orderID)
		}
	})

	repository := ordermysql.NewReconciliationRepository(db)
	if _, err = repository.Scan(context.Background()); err != nil {
		t.Fatalf("first reconciliation scan: %v", err)
	}
	if _, err = repository.Scan(context.Background()); err != nil {
		t.Fatalf("second reconciliation scan: %v", err)
	}
	for _, tc := range cases {
		var count int64
		if err = db.QueryRowContext(context.Background(),
			"SELECT COUNT(*) FROM reconciliation_issue WHERE order_id = ? AND issue_type = ? AND status = 0",
			tc.orderID, tc.issueType).Scan(&count); err != nil {
			t.Fatalf("query issue %s: %v", tc.issueType, err)
		}
		if count != 1 {
			t.Errorf("issue %s count = %d, want 1", tc.issueType, count)
		}
	}
}

func seedReconciliationCase(t *testing.T, db *sql.DB, orderID string, orderStatus, paymentStatus, expected, actual int64) {
	t.Helper()
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, "INSERT INTO orders (id, request_id, user_id, product_id, amount, status) VALUES (?, ?, 7001, 100, 1, ?)", orderID, "req-"+orderID, orderStatus); err != nil {
		t.Fatalf("seed order: %v", err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO order_price_snapshot (order_id, product_id, amount, payable_amount_fen) VALUES (?, 100, 1, ?)", orderID, expected); err != nil {
		t.Fatalf("seed snapshot: %v", err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO payment_order (id, order_id, user_id, payable_amount_fen, status, out_trade_no) VALUES (?, ?, 7001, ?, ?, ?)", "pay:"+orderID, orderID, actual, paymentStatus, "trade:"+orderID); err != nil {
		t.Fatalf("seed payment: %v", err)
	}
}

func cleanupReconciliationCase(db *sql.DB, orderID string) {
	ctx := context.Background()
	_, _ = db.ExecContext(ctx, "DELETE FROM reconciliation_issue WHERE order_id = ?", orderID)
	_, _ = db.ExecContext(ctx, "DELETE FROM payment_order WHERE order_id = ?", orderID)
	_, _ = db.ExecContext(ctx, "DELETE FROM order_price_snapshot WHERE order_id = ?", orderID)
	_, _ = db.ExecContext(ctx, "DELETE FROM orders WHERE id = ?", orderID)
}
