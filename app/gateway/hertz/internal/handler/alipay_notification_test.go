package handler

import (
	"context"
	"testing"

	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/cloudwego/hertz/pkg/app"
)

func TestAlipayPaymentNotificationReturnsProtocolSuccess(t *testing.T) {
	stub := &refundOrderRPCStub{}
	svcCtx := &svc.ServiceContext{OrderRpc: stub}
	c := &app.RequestContext{}
	c.Request.SetBodyString("out_trade_no=FM-100&trade_status=TRADE_SUCCESS&sign=signed")

	AlipayPaymentNotificationHandler(svcCtx)(context.Background(), c)

	if got := string(c.Response.Body()); got != "success" {
		t.Fatalf("response body = %q", got)
	}
	if stub.notificationReq == nil ||
		stub.notificationReq.GetProvider() != "alipay_sandbox" ||
		stub.notificationReq.GetRawBody() != "out_trade_no=FM-100&trade_status=TRADE_SUCCESS&sign=signed" {
		t.Fatalf("unexpected notification RPC request: %#v", stub.notificationReq)
	}
}
