package handler

import (
	"testing"
	"time"

	"flash-mall/app/gateway/hertz/internal/svc"
	orderpb "flash-mall/app/order/rpc/order"

	"github.com/cloudwego/hertz/pkg/app"
)

func TestBuildPaymentIntentUsesProviderQRCodeWithoutRewriting(t *testing.T) {
	rpcResp := &orderpb.CreatePaymentResp{
		OrderId: "order-100", PaymentOrderId: "pay:order-100", OutTradeNo: "FM-100",
		PayableAmountFen: 1234, PaymentStatus: "init", Provider: "alipay_sandbox",
		QrUrl: "https://qr.alipay.test/100", ExpiresAt: 1_800_000_900,
	}
	resp, err := buildPaymentIntentFromRPC(&app.RequestContext{}, &svc.ServiceContext{}, rpcResp, time.Unix(1_800_000_000, 0))
	if err != nil {
		t.Fatal(err)
	}
	if resp.QRURL != rpcResp.QrUrl || resp.ExpiresAt != rpcResp.ExpiresAt || resp.Status != "pending" {
		t.Fatalf("unexpected payment intent: %#v", resp)
	}
}
