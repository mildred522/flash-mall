package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"flash-mall/app/order/api/internal/svc"
	orderclient "flash-mall/app/order/rpc/orderclient"

	"google.golang.org/grpc"
)

type stubOrderRPC struct {
	lastReq *orderclient.MarkOrderPaidReq
	resp    *orderclient.MarkOrderPaidResp
}

func (s *stubOrderRPC) PreDeduct(context.Context, *orderclient.PreDeductReq, ...grpc.CallOption) (*orderclient.Empty, error) {
	panic("unexpected PreDeduct call")
}

func (s *stubOrderRPC) PreDeductRollback(context.Context, *orderclient.PreDeductReq, ...grpc.CallOption) (*orderclient.Empty, error) {
	panic("unexpected PreDeductRollback call")
}

func (s *stubOrderRPC) CreateOrder(context.Context, *orderclient.CreateOrderReq, ...grpc.CallOption) (*orderclient.CreateOrderResp, error) {
	panic("unexpected CreateOrder call")
}

func (s *stubOrderRPC) CreateOrderRollback(context.Context, *orderclient.CreateOrderReq, ...grpc.CallOption) (*orderclient.Empty, error) {
	panic("unexpected CreateOrderRollback call")
}

func (s *stubOrderRPC) MarkOrderPaid(_ context.Context, in *orderclient.MarkOrderPaidReq, _ ...grpc.CallOption) (*orderclient.MarkOrderPaidResp, error) {
	s.lastReq = in
	return s.resp, nil
}

func TestPayOrderHandler_DelegatesToOrderRPC(t *testing.T) {
	orderRPC := &stubOrderRPC{
		resp: &orderclient.MarkOrderPaidResp{
			Updated:     true,
			OrderStatus: "PAID",
		},
	}
	svcCtx := &svc.ServiceContext{
		OrderRpc: orderRPC,
	}

	req := httptest.NewRequest(http.MethodPost, "/api/order/pay", bytes.NewBufferString(`{"order_id":"o-1","payment_order_id":"pay:o-1","out_trade_no":"mock-o-1"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	PayOrderHandler(svcCtx).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	if orderRPC.lastReq == nil {
		t.Fatal("expected order rpc request")
	}
	if orderRPC.lastReq.OrderId != "o-1" || orderRPC.lastReq.PaymentOrderId != "pay:o-1" || orderRPC.lastReq.OutTradeNo != "mock-o-1" {
		t.Fatalf("unexpected rpc request: %#v", orderRPC.lastReq)
	}
	if !strings.Contains(rec.Body.String(), `"updated":true`) || !strings.Contains(rec.Body.String(), `"order_status":"PAID"`) {
		t.Fatalf("unexpected response body: %s", rec.Body.String())
	}
}
