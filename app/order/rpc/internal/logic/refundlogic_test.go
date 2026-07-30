package logic

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"flash-mall/app/order/rpc/internal/config"
	"flash-mall/app/order/rpc/internal/paymentprovider"
	"flash-mall/app/order/rpc/internal/svc"
	orderpb "flash-mall/app/order/rpc/order"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const refundRPCLogicTestDSN = "root:6494kj06@tcp(127.0.0.1:3306)/mall_order?charset=utf8mb4&parseTime=true&loc=Local"

type refundInventoryClient struct {
	releaseCalls int
	releaseErr   error
}

func (c *refundInventoryClient) ReserveStock(context.Context, string, int64, int64) error {
	panic("unexpected ReserveStock call")
}

func (c *refundInventoryClient) ConfirmDeduct(context.Context, string) error {
	panic("unexpected ConfirmDeduct call")
}

func (c *refundInventoryClient) ReleaseStock(context.Context, string, string) error {
	c.releaseCalls++
	return c.releaseErr
}

func TestRequestRefund_IsIdempotentAndChecksOwner(t *testing.T) {
	svcCtx, inventory := newRefundRPCServiceContext(t)
	orderID := refundTestOrderID("request")
	cleanupRefundRPCRows(t, svcCtx, orderID)
	seedRefundRPCOrder(t, svcCtx, orderID, 7001, 1)
	t.Cleanup(func() { cleanupRefundRPCRows(t, svcCtx, orderID) })

	logic := NewRequestRefundLogic(context.Background(), svcCtx)
	req := &orderpb.RequestRefundReq{OrderId: orderID, RequesterId: 7001, RequesterRole: "user", Reason: "changed mind", RequestId: "req:" + orderID}
	first, err := logic.RequestRefund(req)
	if err != nil || first.Repeated || first.RefundId != "rf:"+orderID {
		t.Fatalf("first request: resp=%#v err=%v", first, err)
	}
	second, err := logic.RequestRefund(req)
	if err != nil || !second.Repeated || second.RefundId != first.RefundId {
		t.Fatalf("repeated request: resp=%#v err=%v", second, err)
	}
	if inventory.releaseCalls != 0 {
		t.Fatalf("request must not release stock, calls=%d", inventory.releaseCalls)
	}
	assertRefundRPCCount(t, svcCtx, "refund_order", "order_id", orderID, 1)
	assertRefundRPCCount(t, svcCtx, "order_outbox", "aggregate_id", orderID, 1)

	otherOrderID := refundTestOrderID("owner")
	cleanupRefundRPCRows(t, svcCtx, otherOrderID)
	seedRefundRPCOrder(t, svcCtx, otherOrderID, 7002, 1)
	t.Cleanup(func() { cleanupRefundRPCRows(t, svcCtx, otherOrderID) })
	_, err = logic.RequestRefund(&orderpb.RequestRefundReq{OrderId: otherOrderID, RequesterId: 7001, RequesterRole: "user"})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("wrong owner error=%v, want permission denied", err)
	}
}

func TestAuditRefund_ApproveIsIdempotent(t *testing.T) {
	svcCtx, inventory := newRefundRPCServiceContext(t)
	orderID := refundTestOrderID("approve")
	cleanupRefundRPCRows(t, svcCtx, orderID)
	seedRefundRPCOrder(t, svcCtx, orderID, 7001, 1)
	t.Cleanup(func() { cleanupRefundRPCRows(t, svcCtx, orderID) })
	requested, err := NewRequestRefundLogic(context.Background(), svcCtx).RequestRefund(&orderpb.RequestRefundReq{OrderId: orderID, RequesterId: 7001, RequesterRole: "user"})
	if err != nil {
		t.Fatalf("request refund: %v", err)
	}

	logic := NewAuditRefundLogic(context.Background(), svcCtx)
	req := &orderpb.AuditRefundReq{RefundId: requested.RefundId, OperatorId: 9001, Approve: true, Remark: "approved", RequestId: "audit:" + orderID}
	first, err := logic.AuditRefund(req)
	if err != nil || first.Repeated || first.RefundStatus != 2 || first.OrderStatus != 6 {
		t.Fatalf("first audit: resp=%#v err=%v", first, err)
	}
	second, err := logic.AuditRefund(req)
	if err != nil || !second.Repeated || second.RefundStatus != 2 {
		t.Fatalf("repeated audit: resp=%#v err=%v", second, err)
	}
	if inventory.releaseCalls != 2 {
		t.Fatalf("idempotent stock release calls=%d, want 2", inventory.releaseCalls)
	}
	var providerRecord struct {
		ProviderStatus   string `db:"provider_status"`
		ProviderRefundID string `db:"provider_refund_id"`
	}
	if err = svcCtx.SqlConn.QueryRowCtx(context.Background(), &providerRecord,
		"SELECT provider_status, provider_refund_id FROM refund_order WHERE id=?", requested.RefundId); err != nil {
		t.Fatalf("query provider refund status: %v", err)
	}
	if providerRecord.ProviderStatus != "SUCCESS" || providerRecord.ProviderRefundID != providerRefundIDFor(requested.RefundId) {
		t.Fatalf("provider status=%q refund_id=%q", providerRecord.ProviderStatus, providerRecord.ProviderRefundID)
	}
}

func TestAuditRefund_AlipayUsesStableProviderRefundAndCompletes(t *testing.T) {
	svcCtx, inventory := newRefundRPCServiceContext(t)
	provider := &paymentProviderStub{refundResult: paymentprovider.RefundResult{Status: "SUCCESS"}}
	svcCtx.PaymentProvider = provider
	orderID := refundTestOrderID("alipay")
	cleanupRefundRPCRows(t, svcCtx, orderID)
	seedRefundRPCOrder(t, svcCtx, orderID, 7001, 1)
	if _, err := svcCtx.SqlConn.ExecCtx(context.Background(),
		"UPDATE payment_order SET provider=? WHERE order_id=?", paymentprovider.NameAlipaySandbox, orderID); err != nil {
		t.Fatalf("set payment provider: %v", err)
	}
	t.Cleanup(func() { cleanupRefundRPCRows(t, svcCtx, orderID) })
	requested, err := NewRequestRefundLogic(context.Background(), svcCtx).RequestRefund(&orderpb.RequestRefundReq{
		OrderId: orderID, RequesterId: 7001, RequesterRole: "user", Reason: "sandbox refund",
	})
	if err != nil {
		t.Fatalf("request refund: %v", err)
	}

	resp, err := NewAuditRefundLogic(context.Background(), svcCtx).AuditRefund(&orderpb.AuditRefundReq{
		RefundId: requested.RefundId, OperatorId: 9001, Approve: true, Remark: "approved",
	})
	if err != nil || resp.GetRefundStatus() != refundStatusSuccess {
		t.Fatalf("audit refund: resp=%#v err=%v", resp, err)
	}
	if provider.refundRequest.OutTradeNo != "trade:"+orderID ||
		provider.refundRequest.RefundID != providerRefundIDFor(requested.RefundId) ||
		provider.refundRequest.AmountFen != 9900 {
		t.Fatalf("provider refund request=%#v", provider.refundRequest)
	}
	if inventory.releaseCalls != 1 {
		t.Fatalf("release calls=%d, want 1", inventory.releaseCalls)
	}
}

func TestAuditRefund_ReleaseFailureCanRetry(t *testing.T) {
	svcCtx, inventory := newRefundRPCServiceContext(t)
	orderID := refundTestOrderID("retry")
	cleanupRefundRPCRows(t, svcCtx, orderID)
	seedRefundRPCOrder(t, svcCtx, orderID, 7001, 1)
	t.Cleanup(func() { cleanupRefundRPCRows(t, svcCtx, orderID) })
	requested, err := NewRequestRefundLogic(context.Background(), svcCtx).RequestRefund(&orderpb.RequestRefundReq{OrderId: orderID, RequesterId: 7001, RequesterRole: "user"})
	if err != nil {
		t.Fatalf("request refund: %v", err)
	}

	inventory.releaseErr = errors.New("inventory unavailable")
	logic := NewAuditRefundLogic(context.Background(), svcCtx)
	req := &orderpb.AuditRefundReq{RefundId: requested.RefundId, OperatorId: 9001, Approve: true, Remark: "approved"}
	if _, err = logic.AuditRefund(req); status.Code(err) != codes.Unavailable {
		t.Fatalf("release failure error=%v, want unavailable", err)
	}
	if got := queryRefundRPCStatus(t, svcCtx, requested.RefundId); got != 4 {
		t.Fatalf("failed refund status=%d, want 4", got)
	}
	if got := queryRefundRPCOrderStatus(t, svcCtx, orderID); got != 5 {
		t.Fatalf("order status after release failure=%d, want 5", got)
	}

	inventory.releaseErr = nil
	resp, err := logic.AuditRefund(req)
	if err != nil || resp.RefundStatus != 2 || resp.OrderStatus != 6 {
		t.Fatalf("retry audit: resp=%#v err=%v", resp, err)
	}
}

func TestAuditRefund_ReleaseFailureCannotOverwriteCompletedRefund(t *testing.T) {
	svcCtx, _ := newRefundRPCServiceContext(t)
	orderID := refundTestOrderID("late-failure")
	cleanupRefundRPCRows(t, svcCtx, orderID)
	seedRefundRPCOrder(t, svcCtx, orderID, 7001, 1)
	t.Cleanup(func() { cleanupRefundRPCRows(t, svcCtx, orderID) })
	requested, err := NewRequestRefundLogic(context.Background(), svcCtx).RequestRefund(&orderpb.RequestRefundReq{
		OrderId: orderID, RequesterId: 7001, RequesterRole: "user",
	})
	if err != nil {
		t.Fatalf("request refund: %v", err)
	}

	logic := NewAuditRefundLogic(context.Background(), svcCtx)
	req := &orderpb.AuditRefundReq{RefundId: requested.RefundId, OperatorId: 9001, Approve: true, Remark: "approved"}
	if _, err = logic.AuditRefund(req); err != nil {
		t.Fatalf("approve refund: %v", err)
	}
	if err = logic.markRefundReleaseFailed(req, requested.RefundId, orderID, errors.New("late inventory failure")); err != nil {
		t.Fatalf("record late failure: %v", err)
	}
	if got := queryRefundRPCStatus(t, svcCtx, requested.RefundId); got != refundStatusSuccess {
		t.Fatalf("refund status=%d, want completed status %d", got, refundStatusSuccess)
	}
	assertRefundRPCEventCount(t, svcCtx, orderID, "refund.failed", 0)
}

func TestAuditRefund_RejectRestoresShippedStatus(t *testing.T) {
	svcCtx, inventory := newRefundRPCServiceContext(t)
	orderID := refundTestOrderID("reject")
	cleanupRefundRPCRows(t, svcCtx, orderID)
	seedRefundRPCOrder(t, svcCtx, orderID, 7001, 3)
	t.Cleanup(func() { cleanupRefundRPCRows(t, svcCtx, orderID) })
	requested, err := NewRequestRefundLogic(context.Background(), svcCtx).RequestRefund(&orderpb.RequestRefundReq{OrderId: orderID, RequesterId: 7001, RequesterRole: "user"})
	if err != nil {
		t.Fatalf("request refund: %v", err)
	}

	logic := NewAuditRefundLogic(context.Background(), svcCtx)
	req := &orderpb.AuditRefundReq{RefundId: requested.RefundId, OperatorId: 9001, Approve: false, Remark: "rejected"}
	first, err := logic.AuditRefund(req)
	if err != nil || first.Repeated || first.RefundStatus != 3 || first.OrderStatus != 3 {
		t.Fatalf("reject audit: resp=%#v err=%v", first, err)
	}
	second, err := logic.AuditRefund(req)
	if err != nil || !second.Repeated || second.OrderStatus != 3 {
		t.Fatalf("repeated reject: resp=%#v err=%v", second, err)
	}
	if inventory.releaseCalls != 0 {
		t.Fatalf("reject must not release stock, calls=%d", inventory.releaseCalls)
	}
}

func newRefundRPCServiceContext(t *testing.T) (*svc.ServiceContext, *refundInventoryClient) {
	t.Helper()
	inventory := &refundInventoryClient{}
	svcCtx := &svc.ServiceContext{
		Config:          config.Config{DataSource: refundRPCLogicTestDSN},
		SqlConn:         sqlx.NewMysql(refundRPCLogicTestDSN),
		InventoryClient: inventory,
	}
	ensureRefundRPCSchema(t, svcCtx)
	return svcCtx, inventory
}

func ensureRefundRPCSchema(t *testing.T, svcCtx *svc.ServiceContext) {
	t.Helper()
	statements := []string{
		`CREATE TABLE IF NOT EXISTS refund_order (
			id varchar(64) NOT NULL, order_id varchar(64) NOT NULL, payment_order_id varchar(64) NOT NULL DEFAULT '',
			user_id bigint NOT NULL DEFAULT 0, merchant_id bigint NOT NULL DEFAULT 1000, product_id bigint NOT NULL DEFAULT 0,
			refund_amount_fen bigint NOT NULL DEFAULT 0, status tinyint NOT NULL DEFAULT 0, reason varchar(255) NOT NULL DEFAULT '',
			audit_remark varchar(255) NOT NULL DEFAULT '', operator_id bigint NOT NULL DEFAULT 0,
			request_time timestamp NULL DEFAULT CURRENT_TIMESTAMP, audit_time timestamp NULL DEFAULT NULL,
			finish_time timestamp NULL DEFAULT NULL, create_time timestamp NULL DEFAULT CURRENT_TIMESTAMP,
			update_time timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			PRIMARY KEY (id), UNIQUE KEY uniq_order_id (order_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS order_status_log (
			id bigint NOT NULL AUTO_INCREMENT, order_id varchar(64) NOT NULL, from_status tinyint NOT NULL,
			to_status tinyint NOT NULL, operator_id bigint NOT NULL DEFAULT 0, remark varchar(255) NOT NULL DEFAULT '',
			create_time timestamp NULL DEFAULT CURRENT_TIMESTAMP, PRIMARY KEY (id), KEY ix_order_id (order_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS order_outbox (
			id bigint NOT NULL AUTO_INCREMENT, event_id varchar(128) NOT NULL, event_type varchar(64) NOT NULL,
			aggregate_id varchar(64) NOT NULL, payload json NOT NULL, status tinyint NOT NULL DEFAULT 0,
			attempt_count int NOT NULL DEFAULT 0, next_retry_at timestamp NULL DEFAULT CURRENT_TIMESTAMP,
			published_at timestamp NULL DEFAULT NULL, last_error varchar(255) NOT NULL DEFAULT '',
			create_time timestamp NULL DEFAULT CURRENT_TIMESTAMP, update_time timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			PRIMARY KEY (id), UNIQUE KEY uniq_event_id (event_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}
	for _, statement := range statements {
		if _, err := svcCtx.SqlConn.ExecCtx(context.Background(), statement); err != nil {
			t.Fatalf("ensure refund schema: %v", err)
		}
	}
	ensureRefundRPCColumn(t, svcCtx, "merchant_id", "ALTER TABLE orders ADD COLUMN merchant_id bigint NOT NULL DEFAULT 1000 AFTER user_id")
	ensureRefundRPCColumn(t, svcCtx, "refund_requested_at", "ALTER TABLE orders ADD COLUMN refund_requested_at timestamp NULL DEFAULT NULL AFTER status")
	ensureRefundRPCColumn(t, svcCtx, "refunded_at", "ALTER TABLE orders ADD COLUMN refunded_at timestamp NULL DEFAULT NULL AFTER refund_requested_at")
	ensureRefundRPCTableColumn(t, svcCtx, "payment_order", "provider",
		"ALTER TABLE payment_order ADD COLUMN provider varchar(32) NOT NULL DEFAULT 'local_sandbox'")
	ensureRefundRPCTableColumn(t, svcCtx, "refund_order", "provider",
		"ALTER TABLE refund_order ADD COLUMN provider varchar(32) NOT NULL DEFAULT 'local_sandbox'")
	ensureRefundRPCTableColumn(t, svcCtx, "refund_order", "provider_refund_id",
		"ALTER TABLE refund_order ADD COLUMN provider_refund_id varchar(64) NOT NULL DEFAULT ''")
	ensureRefundRPCTableColumn(t, svcCtx, "refund_order", "provider_status",
		"ALTER TABLE refund_order ADD COLUMN provider_status varchar(32) NOT NULL DEFAULT ''")
	ensureRefundRPCTableColumn(t, svcCtx, "refund_order", "provider_error",
		"ALTER TABLE refund_order ADD COLUMN provider_error varchar(255) NOT NULL DEFAULT ''")
}

func ensureRefundRPCColumn(t *testing.T, svcCtx *svc.ServiceContext, column, alterSQL string) {
	ensureRefundRPCTableColumn(t, svcCtx, "orders", column, alterSQL)
}

func ensureRefundRPCTableColumn(t *testing.T, svcCtx *svc.ServiceContext, table, column, alterSQL string) {
	t.Helper()
	var count int64
	err := svcCtx.SqlConn.QueryRowCtx(context.Background(), &count,
		"SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=? AND COLUMN_NAME=?",
		table, column)
	if err != nil {
		t.Fatalf("inspect %s.%s: %v", table, column, err)
	}
	if count == 0 {
		if _, err = svcCtx.SqlConn.ExecCtx(context.Background(), alterSQL); err != nil {
			t.Fatalf("add %s.%s: %v", table, column, err)
		}
	}
}

func refundTestOrderID(suffix string) string {
	return fmt.Sprintf("refund-rpc-%s-%d", suffix, time.Now().UnixNano())
}

func seedRefundRPCOrder(t *testing.T, svcCtx *svc.ServiceContext, orderID string, userID, orderStatus int64) {
	t.Helper()
	ctx := context.Background()
	if _, err := svcCtx.SqlConn.ExecCtx(ctx,
		"INSERT INTO orders (id, request_id, user_id, merchant_id, product_id, amount, status) VALUES (?, ?, ?, 1000, 100, 1, ?)",
		orderID, "req-"+orderID, userID, orderStatus); err != nil {
		t.Fatalf("seed refund order: %v", err)
	}
	if _, err := svcCtx.SqlConn.ExecCtx(ctx,
		"INSERT INTO payment_order (id, order_id, user_id, payable_amount_fen, status, out_trade_no) VALUES (?, ?, ?, 9900, 1, ?)",
		"pay:"+orderID, orderID, userID, "trade:"+orderID); err != nil {
		t.Fatalf("seed refund payment: %v", err)
	}
}

func cleanupRefundRPCRows(t *testing.T, svcCtx *svc.ServiceContext, orderID string) {
	t.Helper()
	ctx := context.Background()
	for _, statement := range []string{
		"DELETE FROM order_outbox WHERE aggregate_id = ?",
		"DELETE FROM order_status_log WHERE order_id = ?",
		"DELETE FROM refund_order WHERE order_id = ?",
		"DELETE FROM payment_order WHERE order_id = ?",
		"DELETE FROM orders WHERE id = ?",
	} {
		if _, err := svcCtx.SqlConn.ExecCtx(ctx, statement, orderID); err != nil {
			t.Fatalf("cleanup %q: %v", statement, err)
		}
	}
}

func assertRefundRPCCount(t *testing.T, svcCtx *svc.ServiceContext, table, column, value string, want int64) {
	t.Helper()
	var got int64
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s = ?", table, column)
	if err := svcCtx.SqlConn.QueryRowCtx(context.Background(), &got, query, value); err != nil {
		t.Fatalf("query %s count: %v", table, err)
	}
	if got != want {
		t.Fatalf("%s count=%d, want %d", table, got, want)
	}
}

func assertRefundRPCEventCount(t *testing.T, svcCtx *svc.ServiceContext, orderID, eventType string, want int64) {
	t.Helper()
	var got int64
	if err := svcCtx.SqlConn.QueryRowCtx(context.Background(), &got,
		"SELECT COUNT(*) FROM order_outbox WHERE aggregate_id = ? AND event_type = ?", orderID, eventType); err != nil {
		t.Fatalf("query outbox event count: %v", err)
	}
	if got != want {
		t.Fatalf("outbox event %s count=%d, want %d", eventType, got, want)
	}
}

func queryRefundRPCStatus(t *testing.T, svcCtx *svc.ServiceContext, refundID string) int64 {
	t.Helper()
	var got int64
	if err := svcCtx.SqlConn.QueryRowCtx(context.Background(), &got, "SELECT status FROM refund_order WHERE id = ?", refundID); err != nil {
		t.Fatalf("query refund status: %v", err)
	}
	return got
}

func queryRefundRPCOrderStatus(t *testing.T, svcCtx *svc.ServiceContext, orderID string) int64 {
	t.Helper()
	var got int64
	if err := svcCtx.SqlConn.QueryRowCtx(context.Background(), &got, "SELECT status FROM orders WHERE id = ?", orderID); err != nil {
		t.Fatalf("query order status: %v", err)
	}
	return got
}
