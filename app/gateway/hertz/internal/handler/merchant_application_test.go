package handler

import (
	"context"
	"database/sql"
	"testing"

	"flash-mall/app/gateway/hertz/internal/adapters/ordermysql"
	"flash-mall/app/gateway/hertz/internal/application/merchantquery"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/cloudwego/hertz/pkg/app"
)

func TestLoadLatestMerchantApplication(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	mock.ExpectQuery("SELECT id, merchant_name, contact_phone, status").WithArgs(int64(1001)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "merchant_name", "contact_phone", "status", "merchant_id",
			"audit_remark", "create_time", "audit_time",
		}).AddRow(12, "测试商店", "13800000003", 2, 0, "名称不清晰", "2026-07-13 15:00:00", "2026-07-13 16:00:00"))

	service := merchantquery.NewService(ordermysql.NewMerchantProfileRepository(db))
	got, err := service.LatestApplication(context.Background(), 1001)
	if err != nil {
		t.Fatal(err)
	}
	if got.Application == nil || got.Application.ApplyID != 12 || got.Application.StatusText != "rejected" {
		t.Fatalf("unexpected application: %#v", got.Application)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLoadLatestMerchantApplicationReturnsNullWhenMissing(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	mock.ExpectQuery("SELECT id, merchant_name, contact_phone, status").WithArgs(int64(1001)).
		WillReturnError(sql.ErrNoRows)

	service := merchantquery.NewService(ordermysql.NewMerchantProfileRepository(db))
	got, err := service.LatestApplication(context.Background(), 1001)
	if err != nil {
		t.Fatal(err)
	}
	if got.Application != nil {
		t.Fatalf("expected null application, got %#v", got.Application)
	}
}

func TestAdminMerchantApplicationQueryParsesPagination(t *testing.T) {
	c := app.NewContext(0)
	c.Request.SetRequestURI("/?status=0&page=2&page_size=500")
	req, err := adminMerchantApplicationQueryFromRequest(c)
	if err != nil {
		t.Fatal(err)
	}
	if req.Status != 0 || req.Page != 2 || req.PageSize != 500 {
		t.Fatalf("unexpected parsed query: %#v", req)
	}
}
