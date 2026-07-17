package handler

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestActiveMerchantExistsUsesAuthoritativeOrderSchema(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("FROM mall_order.merchant").WithArgs(int64(1101)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	exists, err := activeMerchantExists(context.Background(), db, 1101)
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatal("active merchant should exist")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
