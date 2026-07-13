package handler

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"flash-mall/app/common/authctx"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func TestMerchantStoreProfileHandlerRequiresIdentity(t *testing.T) {
	c := app.NewContext(0)
	MerchantStoreProfileHandler(&svc.ServiceContext{})(context.Background(), c)
	if c.Response.StatusCode() != consts.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", c.Response.StatusCode(), c.Response.Body())
	}
}

func TestMerchantStoreUpdateHandlerRejectsInvalidRequestBeforeDatabase(t *testing.T) {
	c := app.NewContext(0)
	c.Request.SetBodyString(`{"logo_url":"javascript:alert(1)","expected_version":0}`)
	ctx := authctx.WithIdentity(context.Background(), authctx.Identity{UserID: 1001, Role: authctx.RoleMerchant})
	MerchantStoreUpdateHandler(&svc.ServiceContext{})(ctx, c)
	if c.Response.StatusCode() != consts.StatusBadRequest {
		t.Fatalf("status=%d body=%s", c.Response.StatusCode(), c.Response.Body())
	}
}

func TestLoadMerchantStoreProfileReturnsDefaults(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT m.id, m.name").
		WithArgs(int64(1000)).
		WillReturnRows(sqlmock.NewRows([]string{
			"merchant_id", "merchant_name", "logo_url", "banner_url", "description", "version",
		}).AddRow(1000, "Flash Mall 自营店", "", "", "", 0))

	got, err := loadMerchantStoreProfile(context.Background(), db, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if got.MerchantID != 1000 || got.MerchantName != "Flash Mall 自营店" || got.Version != 0 {
		t.Fatalf("unexpected profile: %#v", got)
	}
}

func TestValidateMerchantStoreUpdate(t *testing.T) {
	valid := merchantStoreUpdateReq{
		LogoURL:         "/uploads/stores/1000/logo.png",
		BannerURL:       "https://cdn.example.com/banner.webp",
		Description:     "店铺简介",
		ExpectedVersion: 2,
	}
	if err := validateMerchantStoreUpdate(valid); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}

	cases := []merchantStoreUpdateReq{
		{LogoURL: "javascript:alert(1)"},
		{BannerURL: "data:image/svg+xml,bad"},
		{LogoURL: "https://user:pass@example.com/logo.png"},
		{Description: string(make([]rune, 1001))},
		{ExpectedVersion: -1},
	}
	for _, req := range cases {
		if err := validateMerchantStoreUpdate(req); err == nil {
			t.Fatalf("expected invalid request to fail: %#v", req)
		}
	}
}

func TestSaveMerchantStoreProfileCreatesFirstVersion(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status FROM mall_order.merchant").
		WithArgs(int64(1000)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow(1))
	mock.ExpectQuery("SELECT version FROM mall_order.merchant_store_profile").
		WithArgs(int64(1000)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO mall_order.merchant_store_profile").
		WithArgs(int64(1000), "/logo.png", "/banner.png", "简介").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	version, err := saveMerchantStoreProfile(context.Background(), db, 1000, merchantStoreUpdateReq{
		LogoURL: "/logo.png", BannerURL: "/banner.png", Description: "简介", ExpectedVersion: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if version != 1 {
		t.Fatalf("expected version 1, got %d", version)
	}
}

func TestSaveMerchantStoreProfileRejectsStaleVersion(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status FROM mall_order.merchant").
		WithArgs(int64(1000)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow(1))
	mock.ExpectQuery("SELECT version FROM mall_order.merchant_store_profile").
		WithArgs(int64(1000)).
		WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow(3))
	mock.ExpectRollback()

	_, err = saveMerchantStoreProfile(context.Background(), db, 1000, merchantStoreUpdateReq{ExpectedVersion: 2})
	if !errors.Is(err, errMerchantStoreVersionConflict) {
		t.Fatalf("expected version conflict, got %v", err)
	}
}

func TestSaveMerchantStoreProfileUpdatesMatchingVersion(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status FROM mall_order.merchant").
		WithArgs(int64(1000)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow(1))
	mock.ExpectQuery("SELECT version FROM mall_order.merchant_store_profile").
		WithArgs(int64(1000)).
		WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow(3))
	mock.ExpectExec("UPDATE mall_order.merchant_store_profile").
		WithArgs("/logo.png", "/banner.png", "更新简介", int64(1000), int64(3)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	version, err := saveMerchantStoreProfile(context.Background(), db, 1000, merchantStoreUpdateReq{
		LogoURL: "/logo.png", BannerURL: "/banner.png", Description: "更新简介", ExpectedVersion: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if version != 4 {
		t.Fatalf("expected version 4, got %d", version)
	}
}
