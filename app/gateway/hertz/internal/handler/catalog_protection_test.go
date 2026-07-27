package handler

import (
	"context"
	"errors"
	"testing"

	"flash-mall/app/gateway/hertz/internal/adapters/productmysql"
	"flash-mall/app/gateway/hertz/internal/application/catalogquery"
	"flash-mall/app/gateway/hertz/internal/existencefilter"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

type stubProductFilter struct {
	result existencefilter.Result
	err    error
	checks int
	addErr error
	adds   int
	status existencefilter.Status
}

func (f *stubProductFilter) Check(context.Context, int64) (existencefilter.Result, error) {
	f.checks++
	return f.result, f.err
}
func (f *stubProductFilter) Add(context.Context, int64) error {
	f.adds++
	return f.addErr
}
func (f *stubProductFilter) Rebuild(context.Context) error { return nil }
func (f *stubProductFilter) Status(context.Context) existencefilter.Status {
	return f.status
}

type stubNegativeCache struct {
	hit         bool
	containsErr error
	marks       int
	invalidates int
}

func (c *stubNegativeCache) Contains(context.Context, int64) (bool, error) {
	return c.hit, c.containsErr
}
func (c *stubNegativeCache) Mark(context.Context, int64) error {
	c.marks++
	return nil
}
func (c *stubNegativeCache) Invalidate(context.Context, int64) error {
	c.invalidates++
	return nil
}

func TestProductDetailRejectsBloomAbsentWithoutOriginQuery(t *testing.T) {
	filter := &stubProductFilter{result: existencefilter.ResultAbsent}
	c := app.NewContext(0)
	c.Request.SetRequestURI("/?product_id=999999")
	ProductDetailHandler(&svc.ServiceContext{ProductExistenceFilter: filter})(context.Background(), c)
	if c.Response.StatusCode() != consts.StatusNotFound || filter.checks != 1 {
		t.Fatalf("status=%d checks=%d body=%s", c.Response.StatusCode(), filter.checks, c.Response.Body())
	}
}

func TestProductDetailFilterErrorFailsOpenAndMarksConfirmedMissing(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT p.id, COALESCE").WithArgs(int64(404)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "image_url", "supplier_name", "merchant_id", "merchant_name", "merchant_logo", "store_status", "product_status",
		}))
	filter := &stubProductFilter{err: errors.New("redis unavailable")}
	negative := &stubNegativeCache{}
	c := app.NewContext(0)
	c.Request.SetRequestURI("/?product_id=404")
	ProductDetailHandler(&svc.ServiceContext{
		ProductExistenceFilter: filter,
		ProductNegativeCache:   negative,
		CatalogQueries:         catalogquery.NewService(productmysql.NewCatalogRepository(db)),
	})(context.Background(), c)
	if c.Response.StatusCode() != consts.StatusNotFound || negative.marks != 1 {
		t.Fatalf("status=%d marks=%d body=%s", c.Response.StatusCode(), negative.marks, c.Response.Body())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestProductDetailDoesNotNegativeCacheMetadataFailure(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT p.id, COALESCE").WithArgs(int64(405)).
		WillReturnError(errors.New("mysql unavailable"))
	negative := &stubNegativeCache{}
	c := app.NewContext(0)
	c.Request.SetRequestURI("/?product_id=405")
	ProductDetailHandler(&svc.ServiceContext{
		ProductExistenceFilter: &stubProductFilter{result: existencefilter.ResultPossible},
		ProductNegativeCache:   negative,
		CatalogQueries:         catalogquery.NewService(productmysql.NewCatalogRepository(db)),
	})(context.Background(), c)
	if c.Response.StatusCode() != consts.StatusBadGateway || negative.marks != 0 {
		t.Fatalf("status=%d marks=%d body=%s", c.Response.StatusCode(), negative.marks, c.Response.Body())
	}
}

func TestProductDetailNegativeCacheHitSkipsFilter(t *testing.T) {
	filter := &stubProductFilter{result: existencefilter.ResultPossible}
	c := app.NewContext(0)
	c.Request.SetRequestURI("/?product_id=406")
	ProductDetailHandler(&svc.ServiceContext{
		ProductExistenceFilter: filter,
		ProductNegativeCache:   &stubNegativeCache{hit: true},
	})(context.Background(), c)
	if c.Response.StatusCode() != consts.StatusNotFound || filter.checks != 0 {
		t.Fatalf("status=%d checks=%d body=%s", c.Response.StatusCode(), filter.checks, c.Response.Body())
	}
}

func TestPrepareProductVisibilityRequiresFilterAdd(t *testing.T) {
	filter := &stubProductFilter{addErr: errors.New("redis unavailable")}
	err := prepareProductVisibility(context.Background(), &svc.ServiceContext{
		ProductExistenceFilter: filter,
	}, 500)
	if err == nil || filter.adds != 1 {
		t.Fatalf("err=%v adds=%d", err, filter.adds)
	}
}

func TestInvalidateProductProtectionClearsNegativeCache(t *testing.T) {
	negative := &stubNegativeCache{}
	invalidateProductProtection(context.Background(), &svc.ServiceContext{
		ProductNegativeCache: negative,
	}, 501)
	if negative.invalidates != 1 {
		t.Fatalf("invalidates=%d", negative.invalidates)
	}
}
