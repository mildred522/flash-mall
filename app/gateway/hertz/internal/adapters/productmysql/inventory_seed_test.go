package productmysql

import (
	"context"
	"testing"

	"flash-mall/app/gateway/hertz/internal/ports"

	"github.com/DATA-DOG/go-sqlmock"
)

type inventorySeederStub struct {
	productID  int64
	total      int64
	shardCount int32
}

func (s *inventorySeederStub) SeedStock(_ context.Context, productID int64, total int64, shardCount int32, _ ports.RequestMeta) error {
	s.productID, s.total, s.shardCount = productID, total, shardCount
	return nil
}

func TestInitializeInventorySeedsKitexThenActivatesProduct(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT desired_total, shard_count, desired_product_status").WithArgs(int64(101)).
		WillReturnRows(sqlmock.NewRows([]string{"desired_total", "shard_count", "desired_product_status", "status", "attempt_count"}).AddRow(12, 4, 1, 0, 0))
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE mall_product.product_inventory_seed").WithArgs(int64(101)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE mall_product.product SET status").WithArgs(int64(1), int64(101)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	seeder := &inventorySeederStub{}
	initializer := NewInventoryInitializer(db, seeder)
	state, err := initializer.Initialize(context.Background(), int64(101), ports.RequestMeta{RequestID: "seed-101"})
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	if state.Status != ports.InventorySeedSucceeded || seeder.productID != 101 || seeder.total != 12 || seeder.shardCount != 4 {
		t.Fatalf("unexpected state=%+v seeder=%+v", state, seeder)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestInitializeInventoryIsNoOpAfterSuccessfulSeed(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT desired_total, shard_count, desired_product_status").WithArgs(int64(101)).
		WillReturnRows(sqlmock.NewRows([]string{"desired_total", "shard_count", "desired_product_status", "status", "attempt_count"}).AddRow(12, 4, 1, 1, 3))
	seeder := &inventorySeederStub{}
	state, err := NewInventoryInitializer(db, seeder).Initialize(context.Background(), 101, ports.RequestMeta{})
	if err != nil {
		t.Fatal(err)
	}
	if state.Status != ports.InventorySeedSucceeded || state.Attempts != 3 {
		t.Fatalf("state=%+v", state)
	}
	if seeder.productID != 0 {
		t.Fatalf("successful task must not seed again: %+v", seeder)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
