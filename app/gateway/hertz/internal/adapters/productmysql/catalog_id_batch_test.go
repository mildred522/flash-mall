package productmysql

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestCatalogRepositoryProductIDBatchIsOrderedAndBounded(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT id FROM mall_product.product WHERE id > .* ORDER BY id LIMIT").
		WithArgs(int64(100), 2).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(101).AddRow(205))

	ids, err := NewCatalogRepository(db).ProductIDBatch(context.Background(), 100, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids[0] != 101 || ids[1] != 205 {
		t.Fatalf("ids=%v", ids)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
