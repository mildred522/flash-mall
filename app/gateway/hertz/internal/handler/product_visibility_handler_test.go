package handler

import (
	"context"
	"errors"
	"testing"

	"flash-mall/app/gateway/hertz/internal/application/productcommand"
	"flash-mall/app/gateway/hertz/internal/ports"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

type productCreateRepositoryStub struct {
	productID int64
}

func (r productCreateRepositoryStub) Create(context.Context, productcommand.CreateRecord) (productcommand.CreateResult, error) {
	return productcommand.CreateResult{ProductID: r.productID}, nil
}

func (productCreateRepositoryStub) Update(context.Context, productcommand.UpdateCommand) (productcommand.UpdateResult, error) {
	return productcommand.UpdateResult{Found: true}, nil
}

type inventoryInitializerStub struct {
	calls int
}

func (s *inventoryInitializerStub) Initialize(
	context.Context,
	int64,
	ports.RequestMeta,
) (ports.InventorySeedState, error) {
	s.calls++
	return ports.InventorySeedState{Status: ports.InventorySeedSucceeded}, nil
}

func TestAdminProductCreateDoesNotPublishWhenFilterAddFails(t *testing.T) {
	filter := &stubProductFilter{addErr: errors.New("redis unavailable")}
	initializer := &inventoryInitializerStub{}
	svcCtx := &svc.ServiceContext{
		ProductCommands:        productcommand.NewService(productCreateRepositoryStub{productID: 600}),
		ProductInventory:       initializer,
		ProductExistenceFilter: filter,
	}
	c := app.NewContext(0)
	c.Request.SetBodyString(`{"name":"coat","merchant_id":1000,"origin_price_fen":100,"sale_price_fen":90,"stock_available":1,"supplier_id":1,"status":1}`)
	AdminProductCreateHandler(svcCtx)(context.Background(), c)
	if c.Response.StatusCode() != consts.StatusBadGateway || initializer.calls != 0 || filter.adds != 1 {
		t.Fatalf("status=%d initializer_calls=%d filter_adds=%d body=%s",
			c.Response.StatusCode(), initializer.calls, filter.adds, c.Response.Body())
	}
}
