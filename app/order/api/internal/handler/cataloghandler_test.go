package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"flash-mall/app/order/api/internal/svc"
	"flash-mall/app/product/rpc/productclient"

	"google.golang.org/grpc"
)

type stubProductRPC struct{}

func (s *stubProductRPC) GetProductCard(_ context.Context, in *productclient.GetProductCardReq, _ ...grpc.CallOption) (*productclient.GetProductCardResp, error) {
	return &productclient.GetProductCardResp{
		ProductId:      in.ProductId,
		Name:           "首发风衣",
		OriginPriceFen: 12900,
		FinalPriceFen:  9900,
		PromotionTag:   "限时价",
		StockAvailable: 10,
	}, nil
}

func (s *stubProductRPC) Deduct(context.Context, *productclient.DeductReq, ...grpc.CallOption) (*productclient.Empty, error) {
	return nil, nil
}

func (s *stubProductRPC) DeductRollback(context.Context, *productclient.DeductReq, ...grpc.CallOption) (*productclient.Empty, error) {
	return nil, nil
}

func (s *stubProductRPC) RevertStock(context.Context, *productclient.RevertStockReq, ...grpc.CallOption) (*productclient.RevertStockResp, error) {
	return nil, nil
}

func (s *stubProductRPC) ListProducts(context.Context, *productclient.ListProductsReq, ...grpc.CallOption) (*productclient.ListProductsResp, error) {
	return &productclient.ListProductsResp{
		Items: []*productclient.GetProductCardResp{{
			ProductId:      100,
			Name:           "首发风衣",
			OriginPriceFen: 12900,
			FinalPriceFen:  9900,
			PromotionTag:   "限时价",
			StockAvailable: 10,
		}},
		Total: 1,
	}, nil
}

func TestCatalogHandler_ReturnsProductCards(t *testing.T) {
	productRPC := &stubProductRPC{}
	svcCtx := &svc.ServiceContext{
		ProductRpc: productRPC,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/shop/catalog", nil)
	rec := httptest.NewRecorder()

	CatalogHandler(svcCtx).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	for _, needle := range []string{
		`"items"`,
		`"product_id":100`,
		`"name":"首发风衣"`,
		`"origin_price_fen":12900`,
		`"final_price_fen":9900`,
		`"promotion_tag":"限时价"`,
		`"stock_available":10`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("expected response to contain %q, body=%s", needle, body)
		}
	}
}
