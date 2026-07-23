package productmysql

import (
	"context"
	"testing"

	"flash-mall/app/gateway/hertz/internal/application/supplier"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestSupplierUpdateRejectsDeactivationInsideTransactionWhenActiveProductExists(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM mall_product.supplier WHERE id = \\? FOR UPDATE").
		WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(7))
	mock.ExpectQuery("SELECT id FROM mall_product.product WHERE supplier_id = \\? AND status = 1 LIMIT 1 FOR UPDATE").
		WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(100))
	mock.ExpectRollback()

	status := int64(supplier.StatusInactive)
	result, err := NewSupplierRepository(db).Update(context.Background(), 7, supplier.Changes{Status: &status})
	if err != nil {
		t.Fatalf("update supplier: %v", err)
	}
	if !result.Found || !result.HasActiveProducts {
		t.Fatalf("result = %+v, want found active-product conflict", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}
