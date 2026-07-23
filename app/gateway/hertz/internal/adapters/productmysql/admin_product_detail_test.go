package productmysql

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestAdminProductDetailReturnsNotFoundWithoutSQLLeak(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT p.id").WithArgs(int64(999)).WillReturnError(sql.ErrNoRows)
	_, found, err := NewCatalogRepository(db).AdminProductDetail(context.Background(), 999)
	if err != nil || found {
		t.Fatalf("found=%v err=%v", found, err)
	}
}
