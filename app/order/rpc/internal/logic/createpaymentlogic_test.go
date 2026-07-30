package logic

import (
	"context"
	"net/url"
	"testing"
	"time"

	"flash-mall/app/order/rpc/internal/config"
	"flash-mall/app/order/rpc/internal/paymentprovider"
	"flash-mall/app/order/rpc/internal/svc"
	orderpb "flash-mall/app/order/rpc/order"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type paymentProviderStub struct {
	precreateRequest paymentprovider.PrecreateRequest
	precreateResult  paymentprovider.PrecreateResult
	closeOutTradeNo  string
	refundRequest    paymentprovider.RefundRequest
	refundResult     paymentprovider.RefundResult
	refundErr        error
}

func (*paymentProviderStub) Name() string { return paymentprovider.NameAlipaySandbox }
func (s *paymentProviderStub) Precreate(_ context.Context, in paymentprovider.PrecreateRequest) (paymentprovider.PrecreateResult, error) {
	s.precreateRequest = in
	return s.precreateResult, nil
}
func (*paymentProviderStub) Query(context.Context, string) (paymentprovider.QueryResult, error) {
	panic("unexpected Query call")
}
func (s *paymentProviderStub) Close(_ context.Context, outTradeNo string) error {
	s.closeOutTradeNo = outTradeNo
	return nil
}
func (s *paymentProviderStub) Refund(_ context.Context, in paymentprovider.RefundRequest) (paymentprovider.RefundResult, error) {
	s.refundRequest = in
	return s.refundResult, s.refundErr
}
func (*paymentProviderStub) VerifyNotification(url.Values) (paymentprovider.Notification, error) {
	panic("unexpected VerifyNotification call")
}

func TestCreatePaymentCreatesAlipayQRCode(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	provider := &paymentProviderStub{precreateResult: paymentprovider.PrecreateResult{
		OutTradeNo: "FM-100", QRCode: "https://qr.alipay.test/100",
	}}
	svcCtx := &svc.ServiceContext{
		Config:  config.Config{PaymentProvider: paymentprovider.NameAlipaySandbox, PaymentExpireMinutes: 15},
		SqlConn: sqlx.NewSqlConnFromDB(db), PaymentProvider: provider,
	}
	mock.ExpectQuery("SELECT o.status AS order_status, p.id AS payment_id").
		WithArgs("order-100", int64(7001)).
		WillReturnRows(sqlmock.NewRows([]string{
			"order_status", "payment_id", "out_trade_no", "amount", "payment_status",
			"provider", "provider_qr_url", "expires_at_unix",
		}).AddRow(0, "pay:order-100", "FM-100", 1234, 0, "", "", 0))
	mock.ExpectExec("UPDATE payment_order SET provider=").
		WithArgs(paymentprovider.NameAlipaySandbox, "https://qr.alipay.test/100", 15, "FM-100", "pay:order-100", 0).
		WillReturnResult(sqlmock.NewResult(0, 1))

	resp, err := NewCreatePaymentLogic(context.Background(), svcCtx).CreatePayment(&orderpb.CreatePaymentReq{
		OrderId: "order-100", UserId: 7001,
	})
	if err != nil {
		t.Fatalf("CreatePayment() error = %v", err)
	}
	if resp.GetQrUrl() != "https://qr.alipay.test/100" || resp.GetProvider() != paymentprovider.NameAlipaySandbox {
		t.Fatalf("unexpected response: %#v", resp)
	}
	if provider.precreateRequest.AmountFen != 1234 || provider.precreateRequest.ExpireMinutes != 15 {
		t.Fatalf("unexpected precreate request: %#v", provider.precreateRequest)
	}
	if resp.GetExpiresAt() <= time.Now().Unix() {
		t.Fatalf("ExpiresAt = %d", resp.GetExpiresAt())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCreatePaymentPersistsLocalSandboxExpiry(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	svcCtx := &svc.ServiceContext{
		Config: config.Config{PaymentExpireMinutes: 15}, SqlConn: sqlx.NewSqlConnFromDB(db),
	}
	mock.ExpectQuery("SELECT o.status AS order_status, p.id AS payment_id").
		WithArgs("order-local", int64(7001)).
		WillReturnRows(sqlmock.NewRows([]string{
			"order_status", "payment_id", "out_trade_no", "amount", "payment_status",
			"provider", "provider_qr_url", "expires_at_unix",
		}).AddRow(0, "pay:order-local", "FMLOCAL", 2300, 0, "", "", 0))
	mock.ExpectExec("UPDATE payment_order SET provider=").
		WithArgs(paymentprovider.NameLocalSandbox, 15, "pay:order-local", int64(0)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	resp, err := NewCreatePaymentLogic(context.Background(), svcCtx).CreatePayment(&orderpb.CreatePaymentReq{
		OrderId: "order-local", UserId: 7001,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetProvider() != paymentprovider.NameLocalSandbox || resp.GetExpiresAt() <= time.Now().Unix() {
		t.Fatalf("unexpected response: %#v", resp)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
