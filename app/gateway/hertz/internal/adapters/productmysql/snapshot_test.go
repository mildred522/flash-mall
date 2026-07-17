package productmysql

import (
	"context"
	"testing"

	"flash-mall/app/gateway/hertz/internal/ports"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestSnapshotStoreRebuildCardsScopesProduct(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectExec("INSERT INTO mall_product.product_card_snapshot").WithArgs(int64(101), int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 2))
	store := NewSnapshotStore(db)
	affected, err := store.RebuildCards(context.Background(), ports.SnapshotRebuildRequest{ProductID: 101, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if affected != 2 {
		t.Fatalf("affected=%d", affected)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
