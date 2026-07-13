package handler

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestEnsureMerchantStoreProfileTable(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectExec("CREATE TABLE IF NOT EXISTS mall_order.merchant_store_profile").
		WillReturnResult(sqlmock.NewResult(0, 0))
	if err := ensureMerchantStoreProfileTable(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureMerchantStoreProfileTableCachesSuccessfulDDL(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS mall_order.merchant_store_profile").
		WillReturnResult(sqlmock.NewResult(0, 0))
	if err := ensureMerchantStoreProfileTable(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	if err := ensureMerchantStoreProfileTable(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureHomepageShowcaseTables(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectExec("CREATE TABLE IF NOT EXISTS mall_product.homepage_showcase ").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS mall_product.homepage_showcase_item").
		WillReturnResult(sqlmock.NewResult(0, 0))
	if err := ensureHomepageShowcaseTables(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureProductCreateTimeColumnMigratesLegacyRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(`
SELECT COUNT(1)
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? AND COLUMN_NAME = ?`)).
		WithArgs("mall_product", "product", "create_time").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectExec(regexp.QuoteMeta("ALTER TABLE mall_product.product ADD COLUMN create_time datetime NULL")).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE mall_product.product SET create_time = NOW() - INTERVAL 31 DAY WHERE create_time IS NULL")).
		WillReturnResult(sqlmock.NewResult(0, 5))
	mock.ExpectExec(regexp.QuoteMeta("ALTER TABLE mall_product.product MODIFY COLUMN create_time datetime NOT NULL DEFAULT CURRENT_TIMESTAMP")).
		WillReturnResult(sqlmock.NewResult(0, 0))

	if err := ensureProductCreateTimeColumn(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureProductCreateTimeColumnSkipsExistingColumn(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT COUNT\\(1\\)").
		WithArgs("mall_product", "product", "create_time").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	if err := ensureProductCreateTimeColumn(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSeedDefaultShowcaseDoesNotOverwriteExistingLayout(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectExec("INSERT INTO mall_product.homepage_showcase").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT COUNT\\(1\\) FROM mall_product.homepage_showcase_item").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	if err := seedDefaultShowcase(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSeedDefaultShowcaseSeedsOnlyEmptyLayout(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectExec("INSERT INTO mall_product.homepage_showcase").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT COUNT\\(1\\) FROM mall_product.homepage_showcase_item").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectExec("INSERT INTO mall_product.homepage_showcase_item").
		WillReturnResult(sqlmock.NewResult(0, 5))

	if err := seedDefaultShowcase(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
