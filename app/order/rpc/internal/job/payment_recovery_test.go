package job

import (
	"context"
	"net/url"
	"testing"

	"flash-mall/app/order/rpc/internal/config"
	"flash-mall/app/order/rpc/internal/paymentprovider"
	"flash-mall/app/order/rpc/internal/svc"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type recoveryProviderStub struct {
	closed      string
	queryResult paymentprovider.QueryResult
}

func (*recoveryProviderStub) Name() string { return paymentprovider.NameAlipaySandbox }
func (*recoveryProviderStub) Precreate(context.Context, paymentprovider.PrecreateRequest) (paymentprovider.PrecreateResult, error) {
	panic("unexpected Precreate")
}
func (s *recoveryProviderStub) Query(context.Context, string) (paymentprovider.QueryResult, error) {
	return s.queryResult, nil
}
func (s *recoveryProviderStub) Close(_ context.Context, outTradeNo string) error {
	s.closed = outTradeNo
	return nil
}
func (*recoveryProviderStub) Refund(context.Context, paymentprovider.RefundRequest) (paymentprovider.RefundResult, error) {
	panic("unexpected Refund")
}
func (*recoveryProviderStub) VerifyNotification(url.Values) (paymentprovider.Notification, error) {
	panic("unexpected VerifyNotification")
}

type recoveryInventoryStub struct{ released string }

func (*recoveryInventoryStub) ReserveStock(context.Context, string, int64, int64) error { return nil }
func (*recoveryInventoryStub) ConfirmDeduct(context.Context, string) error              { return nil }
func (s *recoveryInventoryStub) ReleaseStock(_ context.Context, orderID, _ string) error {
	s.released = orderID
	return nil
}

func TestInventoryFinalizeBatchSize(t *testing.T) {
	recovery := NewPaymentRecovery(&svc.ServiceContext{})
	if got := recovery.inventoryFinalizeBatchSize(); got != 200 {
		t.Fatalf("default batch size=%d, want 200", got)
	}
	recovery.svcCtx.Config = config.Config{PaymentFinalizeBatchSize: 1000}
	if got := recovery.inventoryFinalizeBatchSize(); got != 1000 {
		t.Fatalf("configured batch size=%d, want 1000", got)
	}
	if got := recovery.inventoryFinalizeConcurrency(); got != 8 {
		t.Fatalf("default concurrency=%d, want 8", got)
	}
	recovery.svcCtx.Config.PaymentFinalizeConcurrency = 16
	if got := recovery.inventoryFinalizeConcurrency(); got != 16 {
		t.Fatalf("configured concurrency=%d, want 16", got)
	}
	recovery.svcCtx.Config.PaymentFinalizeConcurrency = 100
	if got := recovery.inventoryFinalizeConcurrency(); got != 64 {
		t.Fatalf("bounded concurrency=%d, want 64", got)
	}
}

func TestCloseExpiredPaymentClosesProviderAndReleasesInventory(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	provider := &recoveryProviderStub{}
	inventory := &recoveryInventoryStub{}
	svcCtx := &svc.ServiceContext{
		SqlConn: sqlx.NewSqlConnFromDB(db), PaymentProvider: provider, InventoryClient: inventory,
	}
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE orders SET status=2").WithArgs("order-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE payment_order SET status=3").WithArgs("order-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO order_status_log").WithArgs("order-1").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectExec("UPDATE payment_order SET inventory_release_status=1").WithArgs("order-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	recovery := NewPaymentRecovery(svcCtx)
	err = recovery.closeExpired(context.Background(), paymentRecoveryItem{
		OrderID: "order-1", PaymentOrderID: "pay-1", OutTradeNo: "FM-1", Provider: paymentprovider.NameAlipaySandbox,
	})
	if err != nil {
		t.Fatal(err)
	}
	if provider.closed != "FM-1" || inventory.released != "order-1" {
		t.Fatalf("provider.closed=%q inventory.released=%q", provider.closed, inventory.released)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCloseExpiredPaymentReconcilesPaidProviderBeforeClosing(t *testing.T) {
	provider := &recoveryProviderStub{queryResult: paymentprovider.QueryResult{
		OutTradeNo: "FM-2", TradeNo: "ALI-2", Status: "TRADE_SUCCESS", AmountFen: 8800,
	}}
	svcCtx := &svc.ServiceContext{PaymentProvider: provider}
	var confirmed ProviderPaidPayment
	recovery := NewPaymentRecovery(svcCtx).OnProviderPaid(func(_ context.Context, payment ProviderPaidPayment) error {
		confirmed = payment
		return nil
	})
	err := recovery.closeExpired(context.Background(), paymentRecoveryItem{
		OrderID: "order-2", PaymentOrderID: "pay-2", OutTradeNo: "FM-2",
		Provider: paymentprovider.NameAlipaySandbox,
	})
	if err != nil {
		t.Fatal(err)
	}
	if confirmed.PaymentOrderID != "pay-2" || confirmed.TradeNo != "ALI-2" || confirmed.AmountFen != 8800 {
		t.Fatalf("confirmed=%#v", confirmed)
	}
	if provider.closed != "" {
		t.Fatalf("paid provider trade must not be closed: %q", provider.closed)
	}
}
