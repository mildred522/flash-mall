package handler

import (
	"context"
	"strings"
	"testing"

	"flash-mall/app/gateway/hertz/internal/svc"
	"flash-mall/app/product/rpc/productclient"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc"
)

type showcaseProductClient struct {
	productclient.Product
	list func(context.Context, *productclient.ListProductsReq) (*productclient.ListProductsResp, error)
}

func (client showcaseProductClient) ListProducts(ctx context.Context, req *productclient.ListProductsReq, _ ...grpc.CallOption) (*productclient.ListProductsResp, error) {
	return client.list(ctx, req)
}

func TestDeriveShowcaseInvalidReason(t *testing.T) {
	cases := []struct {
		name  string
		state showcaseSlotState
		want  string
	}{
		{"active", showcaseSlotState{ProductExists: true, ProductStatus: 1, MerchantExists: true, MerchantStatus: 1, StockAvailable: 1}, ""},
		{"product missing", showcaseSlotState{}, "product_not_found"},
		{"product inactive", showcaseSlotState{ProductExists: true, ProductStatus: 2}, "product_inactive"},
		{"merchant missing", showcaseSlotState{ProductExists: true, ProductStatus: 1}, "merchant_not_found"},
		{"merchant inactive", showcaseSlotState{ProductExists: true, ProductStatus: 1, MerchantExists: true, MerchantStatus: 2}, "merchant_inactive"},
		{"out of stock", showcaseSlotState{ProductExists: true, ProductStatus: 1, MerchantExists: true, MerchantStatus: 1}, "out_of_stock"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := deriveShowcaseInvalidReason(tc.state); got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestLoadShowcaseLayoutPreservesInvalidAndEmptySlots(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT showcase.version, showcase.operator_id").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{
			"version", "operator_id", "publish_time", "slot_no", "product_id", "product_exists",
			"product_status", "merchant_id", "merchant_exists", "merchant_status", "stock_available",
		}).
			AddRow(7, 1002, "2026-07-13 19:00:00", 1, 100, 1, 1, 1000, 1, 1, 9).
			AddRow(7, 1002, "2026-07-13 19:00:00", 2, 101, 1, 2, 1000, 1, 1, 9).
			AddRow(7, 1002, "2026-07-13 19:00:00", 3, 999, 0, 0, 0, 0, 0, 0).
			AddRow(7, 1002, "2026-07-13 19:00:00", 4, 102, 1, 1, 2000, 0, 0, 9).
			AddRow(7, 1002, "2026-07-13 19:00:00", 5, 103, 1, 1, 3000, 1, 2, 9).
			AddRow(7, 1002, "2026-07-13 19:00:00", 6, 104, 1, 1, 1000, 1, 1, 0))

	got, err := loadShowcaseLayout(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != 7 || got.OperatorID != 1002 || len(got.Items) != 12 {
		t.Fatalf("unexpected layout: %#v", got)
	}
	wantReasons := []string{"", "product_inactive", "product_not_found", "merchant_not_found", "merchant_inactive", "out_of_stock"}
	for index, want := range wantReasons {
		if got.Items[index].InvalidReason != want {
			t.Fatalf("slot %d reason=%q want=%q", index+1, got.Items[index].InvalidReason, want)
		}
	}
	if !got.Items[6].Empty || got.Items[6].SlotNo != 7 {
		t.Fatalf("slot 7 should be an explicit empty slot: %#v", got.Items[6])
	}
}

func TestBuildPublicShowcaseCatalogFiltersAndPreservesOrder(t *testing.T) {
	layout := ShowcaseResp{Items: []ShowcaseSlot{
		{SlotNo: 1, ProductID: 100, Valid: true},
		{SlotNo: 2, ProductID: 101, Valid: false, InvalidReason: "out_of_stock"},
		{SlotNo: 3, ProductID: 102, Valid: true},
	}}
	cards := map[int64]ProductCard{
		100: {ProductID: 100, Name: "first"},
		102: {ProductID: 102, Name: "third"},
	}
	got := buildPublicShowcaseCatalog(layout, cards)
	if got.Total != 2 || len(got.Items) != 2 || got.Items[0].ProductID != 100 || got.Items[0].SlotNo != 1 || got.Items[1].ProductID != 102 || got.Items[1].SlotNo != 3 {
		t.Fatalf("unexpected catalog: %#v", got)
	}
}

func TestCatalogHandlerReadsPublishedShowcase(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	storefrontSchemaStates.Store(db, &storefrontSchemaState{ready: true})
	merchantStoreProfileSchemaStates.Store(db, &storefrontSchemaState{ready: true})
	t.Cleanup(func() {
		storefrontSchemaStates.Delete(db)
		merchantStoreProfileSchemaStates.Delete(db)
	})

	mock.ExpectQuery("SELECT showcase.version, showcase.operator_id").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{
			"version", "operator_id", "publish_time", "slot_no", "product_id", "product_exists",
			"product_status", "merchant_id", "merchant_exists", "merchant_status", "stock_available",
		}).AddRow(2, 1002, "2026-07-13 19:00:00", 3, 105, 1, 1, 1001, 1, 1, 5))
	mock.ExpectQuery("SELECT p.id, COALESCE\\(p.image_url").
		WithArgs(int64(105)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "image_url", "supplier_name", "merchant_id", "merchant_name", "merchant_logo", "store_status", "product_status",
		}).AddRow(105, "/uploads/products/105.png", "Supplier", 1001, "新店", "", 1, 1))

	client := showcaseProductClient{list: func(_ context.Context, req *productclient.ListProductsReq) (*productclient.ListProductsResp, error) {
		if len(req.ProductIds) != 1 || req.ProductIds[0] != 105 {
			t.Fatalf("unexpected product ids: %v", req.ProductIds)
		}
		return &productclient.ListProductsResp{Items: []*productclient.GetProductCardResp{{
			ProductId: 105, Name: "新商品", FinalPriceFen: 9900, StockAvailable: 5,
		}}}, nil
	}}
	svcCtx := &svc.ServiceContext{SqlConn: sqlx.NewSqlConnFromDB(db), ProductRpc: client}
	c := app.NewContext(0)
	CatalogHandler(svcCtx)(context.Background(), c)
	if c.Response.StatusCode() != 200 || !strings.Contains(string(c.Response.Body()), `"product_id":105`) || !strings.Contains(string(c.Response.Body()), `"slot_no":3`) {
		t.Fatalf("status=%d body=%s", c.Response.StatusCode(), c.Response.Body())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
