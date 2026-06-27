package handler

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"flash-mall/app/entry/api/internal/config"
	"flash-mall/app/entry/api/internal/svc"
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

func TestPaymentCallbackHandler_DelegatesToOrderRPC(t *testing.T) {
	orderRPC := &stubOrderRPC{
		resp: &orderclient.MarkOrderPaidResp{
			Updated:     true,
			OrderStatus: "PAID",
		},
	}
	svcCtx := &svc.ServiceContext{
		Config:   config.Config{PaymentCallbackSecret: "test-secret"},
		OrderRpc: orderRPC,
	}

	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	signature := mockPaymentSignature("test-secret", timestamp, "n-1", "o-1", "pay:o-1", "mock-o-1", 9900)
	body := fmt.Sprintf(`{"order_id":"o-1","payment_order_id":"pay:o-1","out_trade_no":"mock-o-1","paid_amount_fen":9900,"timestamp":"%s","nonce":"n-1","signature":"%s"}`, timestamp, signature)
	req := httptest.NewRequest(http.MethodPost, "/api/payment/callback", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	PaymentCallbackHandler(svcCtx).ServeHTTP(rec, req)

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

func TestPaymentCallbackHandler_RejectsInvalidCallbackSignature(t *testing.T) {
	orderRPC := &stubOrderRPC{
		resp: &orderclient.MarkOrderPaidResp{
			Updated:     true,
			OrderStatus: "PAID",
		},
	}
	svcCtx := &svc.ServiceContext{
		Config:   config.Config{PaymentCallbackSecret: "test-secret"},
		OrderRpc: orderRPC,
	}

	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	body := fmt.Sprintf(`{"order_id":"o-1","payment_order_id":"pay:o-1","out_trade_no":"mock-o-1","paid_amount_fen":9900,"timestamp":"%s","nonce":"n-1","signature":"bad"}`, timestamp)
	req := httptest.NewRequest(http.MethodPost, "/api/payment/callback", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	PaymentCallbackHandler(svcCtx).ServeHTTP(rec, req)

	if rec.Code == http.StatusOK {
		t.Fatalf("expected invalid signature to be rejected, body=%s", rec.Body.String())
	}
	if orderRPC.lastReq != nil {
		t.Fatalf("order rpc should not be called for invalid signature: %#v", orderRPC.lastReq)
	}
}

func TestPaymentCallbackHandler_AcceptsSignedCallback(t *testing.T) {
	orderRPC := &stubOrderRPC{
		resp: &orderclient.MarkOrderPaidResp{
			Updated:     true,
			OrderStatus: "PAID",
		},
	}
	svcCtx := &svc.ServiceContext{
		Config:   config.Config{PaymentCallbackSecret: "test-secret"},
		OrderRpc: orderRPC,
	}
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	signature := mockPaymentSignature("test-secret", timestamp, "n-1", "o-1", "pay:o-1", "mock-o-1", 9900)
	body := fmt.Sprintf(`{"order_id":"o-1","payment_order_id":"pay:o-1","out_trade_no":"mock-o-1","paid_amount_fen":9900,"provider":"mock","event_id":"evt-1","timestamp":"%s","nonce":"n-1","signature":"%s"}`, timestamp, signature)
	req := httptest.NewRequest(http.MethodPost, "/api/payment/callback", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	PaymentCallbackHandler(svcCtx).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	if orderRPC.lastReq == nil {
		t.Fatal("expected order rpc request")
	}
	if !strings.Contains(orderRPC.lastReq.CallbackBody, `"paid_amount_fen":9900`) || !strings.Contains(orderRPC.lastReq.CallbackBody, `"event_id":"evt-1"`) {
		t.Fatalf("unexpected callback body: %s", orderRPC.lastReq.CallbackBody)
	}
}

func TestPayOrderHandler_DoesNotProcessPaymentCallback(t *testing.T) {
	orderRPC := &stubOrderRPC{
		resp: &orderclient.MarkOrderPaidResp{
			Updated:     true,
			OrderStatus: "PAID",
		},
	}
	svcCtx := &svc.ServiceContext{
		OrderRpc: orderRPC,
	}

	req := httptest.NewRequest(http.MethodPost, "/api/order/pay", bytes.NewBufferString(`{"order_id":"o-1","payment_order_id":"pay:o-1","out_trade_no":"mock-o-1","paid_amount_fen":9900}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	PayOrderHandler(svcCtx).ServeHTTP(rec, req)

	if rec.Code == http.StatusOK {
		t.Fatalf("expected callback-shaped request to be rejected by user pay endpoint, body=%s", rec.Body.String())
	}
	if orderRPC.lastReq != nil {
		t.Fatalf("order rpc should not be called by user pay endpoint callback payload: %#v", orderRPC.lastReq)
	}
}

func mockPaymentSignature(secret, timestamp, nonce, orderID, paymentOrderID, outTradeNo string, paidAmountFen int64) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = fmt.Fprintf(mac, "%s.%s.%s.%s.%s.%d", timestamp, nonce, orderID, paymentOrderID, outTradeNo, paidAmountFen)
	return hex.EncodeToString(mac.Sum(nil))
}
