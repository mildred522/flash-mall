package productmysql

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestCatalogRepositoryChecksProductOwnership(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM mall_product.product").WithArgs(int64(8), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	owned, err := NewCatalogRepository(db).OwnsProduct(context.Background(), 7, 8)
	if err != nil || !owned {
		t.Fatalf("owned=%v err=%v", owned, err)
	}
}
