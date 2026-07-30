package logic

import (
	"context"
	"errors"
	"testing"

	"flash-mall/app/common/orderstatus"
	"flash-mall/app/common/tracectx"
	"flash-mall/app/order/rpc/internal/paymentprovider"
	"flash-mall/app/order/rpc/internal/svc"
	orderpb "flash-mall/app/order/rpc/order"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type lifecycleInventoryStub struct {
	releaseCalls int
	releaseErr   error
	orderID      string
	reason       string
	trace        tracectx.Trace
}

func (*lifecycleInventoryStub) ReserveStock(context.Context, string, int64, int64) error { return nil }
func (*lifecycleInventoryStub) ConfirmDeduct(context.Context, string) error              { return nil }
func (s *lifecycleInventoryStub) ReleaseStock(ctx context.Context, orderID string, reason string) error {
	s.releaseCalls++
	s.orderID = orderID
	s.reason = reason
	s.trace, _ = tracectx.FromContext(ctx)
	return s.releaseErr
}

func newLifecycleTestContext(t *testing.T) (*svc.ServiceContext, sqlmock.Sqlmock, *lifecycleInventoryStub) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	inventory := &lifecycleInventoryStub{}
	return &svc.ServiceContext{SqlConn: sqlx.NewSqlConnFromDB(db), InventoryClient: inventory}, mock, inventory
}

func commandMeta() *orderpb.OrderCommandMeta {
	return &orderpb.OrderCommandMeta{RequestId: "req-1", TraceId: "trace-1", UserId: 9, Role: "user"}
}

func TestCancelUserOrder_CommitsThenReleasesAndPropagatesTrace(t *testing.T) {
	svcCtx, mock, inventory := newLifecycleTestContext(t)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status FROM orders WHERE id = \\? AND user_id = \\? FOR UPDATE").
		WithArgs("order-1", int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow(orderstatus.PendingPayment))
	mock.ExpectExec("UPDATE orders SET status = \\?, update_time = NOW\\(\\) WHERE id = \\? AND status = \\?").
		WithArgs(orderstatus.Closed, "order-1", orderstatus.PendingPayment).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE payment_order SET status").
		WithArgs(int64(3), "order-1", int64(0)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO order_status_log").
		WithArgs("order-1", orderstatus.PendingPayment, orderstatus.Closed, int64(9), "user cancelled: changed mind").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectExec("UPDATE payment_order SET inventory_release_status=1").
		WithArgs("order-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	resp, err := NewCancelUserOrderLogic(context.Background(), svcCtx).CancelUserOrder(&orderpb.CancelUserOrderReq{
		OrderId: "order-1", Reason: "changed mind", UserId: 9, Meta: commandMeta(),
	})
	if err != nil {
		t.Fatalf("CancelUserOrder() error = %v", err)
	}
	if resp.GetRepeated() || resp.GetOrderStatus() != orderstatus.Closed {
		t.Fatalf("unexpected response: %#v", resp)
	}
	if inventory.releaseCalls != 1 || inventory.orderID != "order-1" || inventory.reason != "changed mind" {
		t.Fatalf("unexpected inventory release: %#v", inventory)
	}
	if inventory.trace.RequestID != "req-1" || inventory.trace.TraceID != "trace-1" {
		t.Fatalf("trace not propagated: %#v", inventory.trace)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCancelUserOrderClosesAlipayBeforeLocalOrder(t *testing.T) {
	svcCtx, mock, _ := newLifecycleTestContext(t)
	provider := &paymentProviderStub{}
	svcCtx.PaymentProvider = provider
	mock.ExpectQuery("SELECT provider, out_trade_no, provider_status FROM payment_order").
		WithArgs("order-alipay").
		WillReturnRows(sqlmock.NewRows([]string{"provider", "out_trade_no", "provider_status"}).
			AddRow(paymentprovider.NameAlipaySandbox, "FM-ALIPAY", "WAIT_BUYER_PAY"))
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status FROM orders WHERE id = \\? AND user_id = \\? FOR UPDATE").
		WithArgs("order-alipay", int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow(orderstatus.PendingPayment))
	mock.ExpectExec("UPDATE orders SET status").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE payment_order SET status").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO order_status_log").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectExec("UPDATE payment_order SET inventory_release_status=1").
		WithArgs("order-alipay").
		WillReturnResult(sqlmock.NewResult(0, 1))
	_, err := NewCancelUserOrderLogic(context.Background(), svcCtx).CancelUserOrder(&orderpb.CancelUserOrderReq{
		OrderId: "order-alipay", UserId: 9, Reason: "cancel", Meta: commandMeta(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if provider.closeOutTradeNo != "FM-ALIPAY" {
		t.Fatalf("provider close trade = %q", provider.closeOutTradeNo)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCancelUserOrder_ReleaseFailureCanRetryAfterOrderClosed(t *testing.T) {
	svcCtx, mock, inventory := newLifecycleTestContext(t)
	inventory.releaseErr = errors.New("inventory unavailable")

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status FROM orders WHERE id = \\? AND user_id = \\? FOR UPDATE").
		WithArgs("order-2", int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow(orderstatus.PendingPayment))
	mock.ExpectExec("UPDATE orders SET status").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE payment_order SET status").
		WithArgs(int64(3), "order-2", int64(0)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO order_status_log").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectExec("UPDATE payment_order SET inventory_release_status=2").
		WithArgs(sqlmock.AnyArg(), "order-2").
		WillReturnResult(sqlmock.NewResult(0, 1))

	logic := NewCancelUserOrderLogic(context.Background(), svcCtx)
	_, err := logic.CancelUserOrder(&orderpb.CancelUserOrderReq{OrderId: "order-2", Reason: "retry", UserId: 9, Meta: commandMeta()})
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("release error = %v, want unavailable", err)
	}

	inventory.releaseErr = nil
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status FROM orders WHERE id = \\? AND user_id = \\? FOR UPDATE").
		WithArgs("order-2", int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow(orderstatus.Closed))
	mock.ExpectCommit()
	mock.ExpectExec("UPDATE payment_order SET inventory_release_status=1").
		WithArgs("order-2").
		WillReturnResult(sqlmock.NewResult(0, 1))
	resp, err := logic.CancelUserOrder(&orderpb.CancelUserOrderReq{OrderId: "order-2", Reason: "retry", UserId: 9, Meta: commandMeta()})
	if err != nil || !resp.GetRepeated() {
		t.Fatalf("retry response=%#v error=%v", resp, err)
	}
	if inventory.releaseCalls != 2 {
		t.Fatalf("release calls=%d, want 2", inventory.releaseCalls)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestShipMerchantOrder_IsOwnerScopedAndIdempotent(t *testing.T) {
	svcCtx, mock, _ := newLifecycleTestContext(t)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status FROM orders WHERE id = \\? AND merchant_id = \\? FOR UPDATE").
		WithArgs("order-3", int64(88)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow(orderstatus.Paid))
	mock.ExpectExec("UPDATE orders SET status = \\?, shipped_at = NOW\\(\\), update_time = NOW\\(\\)").
		WithArgs(orderstatus.Shipped, "order-3", int64(88), orderstatus.Paid).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO order_status_log").
		WithArgs("order-3", orderstatus.Paid, orderstatus.Shipped, int64(88), "merchant ship order").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	logic := NewShipMerchantOrderLogic(context.Background(), svcCtx)
	resp, err := logic.ShipMerchantOrder(&orderpb.ShipMerchantOrderReq{OrderId: "order-3", MerchantId: 88, Meta: commandMeta()})
	if err != nil || resp.GetRepeated() {
		t.Fatalf("ship response=%#v error=%v", resp, err)
	}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status FROM orders WHERE id = \\? AND merchant_id = \\? FOR UPDATE").
		WithArgs("order-3", int64(88)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow(orderstatus.Shipped))
	mock.ExpectCommit()
	resp, err = logic.ShipMerchantOrder(&orderpb.ShipMerchantOrderReq{OrderId: "order-3", MerchantId: 88, Meta: commandMeta()})
	if err != nil || !resp.GetRepeated() {
		t.Fatalf("repeat ship response=%#v error=%v", resp, err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestConfirmReceipt_RejectsNonShippedOrder(t *testing.T) {
	svcCtx, mock, _ := newLifecycleTestContext(t)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status FROM orders WHERE id = \\? AND user_id = \\? FOR UPDATE").
		WithArgs("order-4", int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow(orderstatus.Paid))
	mock.ExpectRollback()

	_, err := NewConfirmReceiptLogic(context.Background(), svcCtx).ConfirmReceipt(&orderpb.ConfirmReceiptReq{
		OrderId: "order-4", UserId: 9, Meta: commandMeta(),
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("error=%v, want failed precondition", err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
