package productmysql

import (
	"context"
	"database/sql"
	"testing"

	"flash-mall/app/gateway/hertz/internal/application/productcommand"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestProductCommandCreateRejectsMissingActiveSupplier(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer func() { _ = db.Close() }()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM mall_product.supplier WHERE id = \\? AND status = 1 FOR UPDATE").
		WithArgs(int64(3)).WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	result, err := NewProductCommandRepository(db).Create(context.Background(), productcommand.CreateRecord{
		Name: "商品", MerchantID: 7, OriginPriceFen: 2000, SalePriceFen: 1600, SupplierID: 3, Status: 1,
	})
	if err != nil || result.Rejection != productcommand.RejectSupplierNotFound {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}

func TestProductCommandCreateChecksAuthoritativeMerchantSchema(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer func() { _ = db.Close() }()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM mall_product.supplier").WithArgs(int64(3)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(3))
	mock.ExpectQuery("SELECT id FROM mall_order.merchant").WithArgs(int64(7)).WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	result, err := NewProductCommandRepository(db).Create(context.Background(), productcommand.CreateRecord{
		Name: "商品", MerchantID: 7, OriginPriceFen: 2000, SalePriceFen: 1600, SupplierID: 3, Status: 1,
	})
	if err != nil || result.Rejection != productcommand.RejectMerchantNotFound {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}

func TestProductCommandUpdateRejectsInvalidFinalPriceInsideTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer func() { _ = db.Close() }()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT origin_price_fen, sale_price_fen FROM mall_product.product WHERE id = \\? FOR UPDATE").
		WithArgs(int64(100)).WillReturnRows(sqlmock.NewRows([]string{"origin", "sale"}).AddRow(2000, 1600))
	mock.ExpectRollback()
	salePrice := int64(2100)

	result, err := NewProductCommandRepository(db).Update(context.Background(), productcommand.UpdateCommand{
		ProductID: 100, SalePriceFen: &salePrice,
	})
	if err != nil || !result.Found || result.Rejection != productcommand.RejectInvalidPrice {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}
