package handler

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	gatewaycache "flash-mall/app/gateway/hertz/internal/cache"
	"flash-mall/app/gateway/hertz/internal/config"
	"flash-mall/app/gateway/hertz/internal/svc"
	"flash-mall/app/product/rpc/productclient"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
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
	svcCtx := &svc.ServiceContext{
		SqlConn: sqlx.NewSqlConnFromDB(db), ProductRpc: client,
		Cache: gatewaycache.New(gatewaycache.Config{EnableL1: true, L1TTL: time.Minute}, nil),
	}
	for attempt := 0; attempt < 2; attempt++ {
		c := app.NewContext(0)
		CatalogHandler(svcCtx)(context.Background(), c)
		if c.Response.StatusCode() != 200 || !strings.Contains(string(c.Response.Body()), `"product_id":105`) || !strings.Contains(string(c.Response.Body()), `"slot_no":3`) {
			t.Fatalf("attempt=%d status=%d body=%s", attempt, c.Response.StatusCode(), c.Response.Body())
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestValidateShowcaseDraft(t *testing.T) {
	active := func(productID, merchantID int64) showcaseProductState {
		return showcaseProductState{
			ProductID: productID, ProductExists: true, ProductStatus: 1,
			MerchantID: merchantID, MerchantExists: true, MerchantStatus: 1, StockAvailable: 5,
		}
	}
	states := map[int64]showcaseProductState{100: active(100, 10), 101: active(101, 10), 102: active(102, 10)}
	if err := validateShowcaseDraft([]showcasePublishItem{{SlotNo: 1, ProductID: 100}, {SlotNo: 12, ProductID: 101}}, states); err != nil {
		t.Fatalf("valid draft rejected: %v", err)
	}

	invalidDrafts := [][]showcasePublishItem{
		{{SlotNo: 0, ProductID: 100}},
		{{SlotNo: 1, ProductID: 100}, {SlotNo: 1, ProductID: 101}},
		{{SlotNo: 1, ProductID: 100}, {SlotNo: 2, ProductID: 100}},
		{{SlotNo: 1, ProductID: 100}, {SlotNo: 2, ProductID: 101}, {SlotNo: 3, ProductID: 102}},
	}
	for _, items := range invalidDrafts {
		if err := validateShowcaseDraft(items, states); !errors.Is(err, errShowcaseInvalidDraft) {
			t.Fatalf("items=%v expected invalid draft, got %v", items, err)
		}
	}

	stateConflict := map[int64]showcaseProductState{100: active(100, 10)}
	stateConflict[100] = showcaseProductState{ProductID: 100, ProductExists: true, ProductStatus: 1, MerchantID: 10, MerchantExists: true, MerchantStatus: 1}
	if err := validateShowcaseDraft([]showcasePublishItem{{SlotNo: 4, ProductID: 100}}, stateConflict); !errors.Is(err, errShowcaseStateConflict) {
		t.Fatalf("expected stock conflict, got %v", err)
	}
}

func TestPublishShowcaseRejectsStaleVersion(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT version FROM mall_product.homepage_showcase").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow(8))
	mock.ExpectRollback()

	_, err = publishShowcase(context.Background(), db, 1002, showcasePublishReq{ExpectedVersion: 7})
	if !errors.Is(err, errShowcaseVersionConflict) {
		t.Fatalf("expected version conflict, got %v", err)
	}
}

func TestPublishShowcaseReplacesLayoutAtomically(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT version FROM mall_product.homepage_showcase").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow(7))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT product.id, product.status, product.merchant_id,`)).
		WithArgs(int64(100), int64(105)).
		WillReturnRows(sqlmock.NewRows([]string{
			"product_id", "product_status", "merchant_id", "merchant_exists", "merchant_status", "stock_available",
		}).AddRow(100, 1, 1000, 1, 1, 8).AddRow(105, 1, 1001, 1, 1, 3))
	mock.ExpectExec("DELETE FROM mall_product.homepage_showcase_item").
		WithArgs(int64(1)).WillReturnResult(sqlmock.NewResult(0, 5))
	mock.ExpectExec("INSERT INTO mall_product.homepage_showcase_item").
		WithArgs(int64(1), int64(1), int64(100), int64(1), int64(4), int64(105)).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("UPDATE mall_product.homepage_showcase").
		WithArgs(int64(1002), int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	version, err := publishShowcase(context.Background(), db, 1002, showcasePublishReq{
		ExpectedVersion: 7,
		Items:           []showcasePublishItem{{SlotNo: 1, ProductID: 100}, {SlotNo: 4, ProductID: 105}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if version != 8 {
		t.Fatalf("version=%d want 8", version)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestBuildAdminShowcaseAttachesKnownProducts(t *testing.T) {
	layout := ShowcaseResp{Items: []ShowcaseSlot{{SlotNo: 1, ProductID: 100, Valid: true}, {SlotNo: 2, ProductID: 999, InvalidReason: "product_not_found"}}}
	got := buildAdminShowcase(layout, map[int64]ProductCard{100: {ProductID: 100, Name: "商品"}})
	if got.Items[0].Product == nil || got.Items[0].Product.Name != "商品" || got.Items[1].Product != nil {
		t.Fatalf("unexpected layout: %#v", got)
	}
}

func TestAdminShowcaseRoutesRequireAuthentication(t *testing.T) {
	h := server.Default()
	registerAdminRoutes(h, &svc.ServiceContext{Config: config.Config{JwtAuthSecret: "jwt-secret"}})
	for _, tc := range []struct{ method, path string }{
		{"GET", "/api/admin/showcase"},
		{"POST", "/api/admin/showcase/publish"},
	} {
		resp := ut.PerformRequest(h.Engine, tc.method, tc.path, nil).Result()
		if resp.StatusCode() != consts.StatusUnauthorized {
			t.Errorf("%s %s status=%d body=%s", tc.method, tc.path, resp.StatusCode(), resp.Body())
		}
	}
}

func TestAdminShowcasePublishHandlerRejectsInvalidShapeBeforeDatabase(t *testing.T) {
	c := app.NewContext(0)
	c.Request.SetBodyString(`{"expected_version":1,"items":[{"slot_no":0,"product_id":100}]}`)
	AdminShowcasePublishHandler(&svc.ServiceContext{})(context.Background(), c)
	if c.Response.StatusCode() != consts.StatusBadRequest {
		t.Fatalf("status=%d body=%s", c.Response.StatusCode(), c.Response.Body())
	}
}
