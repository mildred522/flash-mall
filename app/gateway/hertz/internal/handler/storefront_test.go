package handler

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"flash-mall/app/gateway/hertz/internal/svc"
	"flash-mall/app/product/rpc/productclient"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func TestLoadPublicStoreDetailReturnsActiveStore(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT m.id, m.name").
		WithArgs(int64(1000)).
		WillReturnRows(sqlmock.NewRows([]string{
			"merchant_id", "merchant_name", "logo_url", "banner_url", "description", "status", "product_count",
		}).AddRow(1000, "Flash Mall 自营店", "/logo.png", "/banner.png", "简介", 1, 5))

	got, err := loadPublicStoreDetail(context.Background(), db, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if got.MerchantID != 1000 || got.ProductCount != 5 || got.Status != 1 {
		t.Fatalf("unexpected store: %#v", got)
	}
}

func TestLoadPublicStoreDetailHidesInactiveStore(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT m.id, m.name").WithArgs(int64(2000)).WillReturnError(sql.ErrNoRows)
	if _, err := loadPublicStoreDetail(context.Background(), db, 2000); err != sql.ErrNoRows {
		t.Fatalf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestLoadStoreProductIDsFiltersAndPaginates(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM mall_product.product p").
		WithArgs(int64(1000), int64(1), "%风衣%").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
	mock.ExpectQuery("SELECT p.id FROM mall_product.product p").
		WithArgs(int64(1000), int64(1), "%风衣%", int64(20), int64(20)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(103).AddRow(100))

	ids, total, err := loadStoreProductIDs(context.Background(), db, 1000, "风衣", 2, 20)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(ids) != 2 || ids[0] != 103 || ids[1] != 100 {
		t.Fatalf("ids=%v total=%d", ids, total)
	}
}

func TestBuildProductCardsIncludesStoreMetadata(t *testing.T) {
	cards := buildProductCards([]*productclient.GetProductCardResp{{
		ProductId: 100, Name: "风衣", OriginPriceFen: 12900, FinalPriceFen: 11900, StockAvailable: 8,
	}}, map[int64]productMeta{
		100: {
			ImageURL: "/products/100.svg", MerchantID: 1000, MerchantName: "Flash Mall 自营店",
			MerchantLogo: "/logo.png", StoreStatus: 1, ProductStatus: 1,
		},
	}, nil)
	got := cards[100]
	if got.MerchantID != 1000 || got.MerchantName != "Flash Mall 自营店" || got.StoreURL != "/store/1000" || got.StoreStatus != 1 {
		t.Fatalf("unexpected card: %#v", got)
	}
}

func TestLoadProductMetaEnsuresStoreProfileTable(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT COUNT\\(1\\)").WithArgs("mall_order", "merchant_store_profile").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("SELECT p.id, COALESCE\\(p.image_url").
		WithArgs(int64(100)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "image_url", "supplier_name", "merchant_id", "merchant_name", "merchant_logo", "store_status", "product_status",
		}).AddRow(100, "/products/100.svg", "Supplier", 1000, "Store", "", 1, 1))

	svcCtx := &svc.ServiceContext{SqlConn: sqlx.NewSqlConnFromDB(db)}
	got := loadProductMeta(context.Background(), svcCtx, []int64{100})
	if got[100].MerchantID != 1000 {
		t.Fatalf("unexpected meta: %#v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestProductMetaPubliclyVisible(t *testing.T) {
	if !productMetaPubliclyVisible(productMeta{ProductStatus: 1, StoreStatus: 1}) {
		t.Fatal("active product in active store should be visible")
	}
	if productMetaPubliclyVisible(productMeta{ProductStatus: 2, StoreStatus: 1}) {
		t.Fatal("inactive product should be hidden")
	}
	if productMetaPubliclyVisible(productMeta{ProductStatus: 1, StoreStatus: 2}) {
		t.Fatal("inactive store should hide product")
	}
}

func TestBuildProductDetailIncludesOnlyRequestedRelatedProducts(t *testing.T) {
	cards := map[int64]ProductCard{
		100: {ProductID: 100, Name: "主商品"},
		101: {ProductID: 101, Name: "同店一"},
		102: {ProductID: 102, Name: "同店二"},
	}
	got, ok := buildProductDetailResp(100, []int64{102, 999, 101}, cards)
	if !ok {
		t.Fatal("main product should exist")
	}
	if got.Item.ProductID != 100 || len(got.StoreProducts) != 2 || got.StoreProducts[0].ProductID != 102 || got.StoreProducts[1].ProductID != 101 {
		t.Fatalf("unexpected detail: %#v", got)
	}
}

func TestExcludeProductIDPreservesOrderAndLimit(t *testing.T) {
	got := excludeProductID([]int64{104, 100, 103, 102, 101}, 100, 3)
	want := []int64{104, 103, 102}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("got=%v want=%v", got, want)
		}
	}
}

func TestStorefrontRoutesAreRegistered(t *testing.T) {
	h := server.Default()
	registerSystemRoutes(h, &svc.ServiceContext{}, time.Now())
	registerShopRoutes(h, &svc.ServiceContext{})
	want := map[string]bool{
		"GET /product/*any":             false,
		"GET /store/*any":               false,
		"GET /api/shop/stores/detail":   false,
		"GET /api/shop/stores/products": false,
		"GET /api/shop/products/detail": false,
	}
	for _, route := range h.Routes() {
		if _, exists := want[route.Method+" "+route.Path]; exists {
			want[route.Method+" "+route.Path] = true
		}
	}
	for route, found := range want {
		if !found {
			t.Errorf("route missing: %s", route)
		}
	}
}
