package ordermysql

import (
	"context"
	"database/sql"
	"testing"

	"flash-mall/app/gateway/hertz/internal/application/merchantstore"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestMerchantStoreProfileReturnsDefaults(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	mock.ExpectQuery("SELECT m.id, m.name").WithArgs(int64(1000)).
		WillReturnRows(sqlmock.NewRows([]string{
			"merchant_id", "merchant_name", "logo_url", "banner_url", "description", "version",
		}).AddRow(1000, "Flash Mall 自营店", "", "", "", 0))

	profile, found, err := NewMerchantStoreRepository(db).Profile(context.Background(), 1000)
	if err != nil || !found || profile.MerchantName != "Flash Mall 自营店" || profile.Version != 0 {
		t.Fatalf("profile = %+v, found = %v, err = %v", profile, found, err)
	}
}

func TestMerchantStoreUpdateCreatesFirstVersion(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status FROM mall_order.merchant").WithArgs(int64(1000)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow(1))
	mock.ExpectQuery("SELECT version FROM mall_order.merchant_store_profile").WithArgs(int64(1000)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO mall_order.merchant_store_profile").
		WithArgs(int64(1000), "/logo.png", "/banner.png", "简介").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	record, err := NewMerchantStoreRepository(db).Update(context.Background(), merchantstore.UpdateInput{
		MerchantID: 1000, LogoURL: "/logo.png", BannerURL: "/banner.png", Description: "简介", ExpectedVersion: 0,
	})
	if err != nil || !record.Found || record.Version != 1 {
		t.Fatalf("record = %+v, err = %v", record, err)
	}
}

func TestMerchantStoreUpdateRejectsStaleVersion(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status FROM mall_order.merchant").WithArgs(int64(1000)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow(1))
	mock.ExpectQuery("SELECT version FROM mall_order.merchant_store_profile").WithArgs(int64(1000)).
		WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow(3))
	mock.ExpectRollback()

	record, err := NewMerchantStoreRepository(db).Update(context.Background(), merchantstore.UpdateInput{
		MerchantID: 1000, ExpectedVersion: 2,
	})
	if err != nil || record.Rejection != merchantstore.RejectVersionConflict {
		t.Fatalf("record = %+v, err = %v", record, err)
	}
}

func TestMerchantStoreUpdateUsesMatchingVersion(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status FROM mall_order.merchant").WithArgs(int64(1000)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow(1))
	mock.ExpectQuery("SELECT version FROM mall_order.merchant_store_profile").WithArgs(int64(1000)).
		WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow(3))
	mock.ExpectExec("UPDATE mall_order.merchant_store_profile").
		WithArgs("/logo.png", "/banner.png", "更新简介", int64(1000), int64(3)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	record, err := NewMerchantStoreRepository(db).Update(context.Background(), merchantstore.UpdateInput{
		MerchantID: 1000, LogoURL: "/logo.png", BannerURL: "/banner.png", Description: "更新简介", ExpectedVersion: 3,
	})
	if err != nil || record.Version != 4 || !record.Found {
		t.Fatalf("record = %+v, err = %v", record, err)
	}
}
