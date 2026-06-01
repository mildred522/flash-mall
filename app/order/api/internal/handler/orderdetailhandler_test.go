package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"flash-mall/app/order/api/internal/svc"
	orderclient "flash-mall/app/order/rpc/orderclient"

	"google.golang.org/grpc"
)

func (s *stubOrderRPC) GetOrderDetail(_ context.Context, in *orderclient.GetOrderDetailReq, _ ...grpc.CallOption) (*orderclient.GetOrderDetailResp, error) {
	return &orderclient.GetOrderDetailResp{
		OrderId:           in.OrderId,
		UserId:            7,
		OrderStatus:       "PENDING_PAYMENT",
		PayableAmountFen:  29700,
		PaymentStatus:     "INIT",
		OriginPriceFen:    38700,
		DiscountAmountFen: 9000,
	}, nil
}

func TestOrderDetailHandler_ReturnsOrderDetail(t *testing.T) {
	orderRPC := &stubOrderRPC{}
	svcCtx := &svc.ServiceContext{
		OrderRpc: orderRPC,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/order/detail?order_id=o-detail-1", nil)
	rec := httptest.NewRecorder()

	OrderDetailHandler(svcCtx).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, needle := range []string{
		`"order_id":"o-detail-1"`,
		`"order_status":"PENDING_PAYMENT"`,
		`"payment_status":"INIT"`,
		`"payable_amount_fen":29700`,
		`"discount_amount_fen":9000`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("expected response to contain %q, body=%s", needle, body)
		}
	}
}
