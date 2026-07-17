package handler

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestRequireSchemaTableRejectsMissingMigration(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT COUNT\\(1\\)").WithArgs("mall_product", "homepage_showcase").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	if err := requireSchemaTable(context.Background(), db, "mall_product", "homepage_showcase"); err == nil {
		t.Fatal("missing table must fail readiness instead of being created at runtime")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRequireMerchantStoreProfileSchemaCachesSuccessfulCheck(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT COUNT\\(1\\)").WithArgs("mall_order", "merchant_store_profile").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	if err := requireMerchantStoreProfileSchema(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	if err := requireMerchantStoreProfileSchema(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRequireStorefrontSchemaChecksMigrationOwnedObjects(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT COUNT\\(1\\)").WithArgs("mall_order", "merchant_store_profile").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("SELECT COUNT\\(1\\)").WithArgs("mall_product", "product", "create_time").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("SELECT COUNT\\(1\\)").WithArgs("mall_product", "homepage_showcase").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("SELECT COUNT\\(1\\)").WithArgs("mall_product", "homepage_showcase_item").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	if err := requireStorefrontSchema(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
