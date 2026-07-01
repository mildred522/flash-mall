package repository

import (
	"context"
	"regexp"
	"testing"

	"flash-mall/app/common/apperror"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/alicebob/miniredis/v2"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

func TestRedisMySQLRepositoryRedisLifecycle(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	defer mr.Close()

	repo := NewRedisMySQLRepository(redis.MustNewRedis(redis.RedisConf{Host: mr.Addr(), Type: redis.NodeType}), nil, 2)
	ctx := context.Background()
	if err := repo.SeedStock(ctx, 100, 10, 0); err != nil {
		t.Fatalf("SeedStock error: %v", err)
	}
	stock, err := repo.GetStock(ctx, 100)
	if err != nil {
		t.Fatalf("GetStock error: %v", err)
	}
	if stock.Available != 10 {
		t.Fatalf("available = %d, want 10", stock.Available)
	}
	if err := repo.ReserveStock(ctx, "order-1", 100, 3); err != nil {
		t.Fatalf("ReserveStock error: %v", err)
	}
	stock, _ = repo.GetStock(ctx, 100)
	if stock.Available != 7 {
		t.Fatalf("available after reserve = %d, want 7", stock.Available)
	}
	if err := repo.ReserveStock(ctx, "order-1", 100, 3); err != nil {
		t.Fatalf("ReserveStock retry error: %v", err)
	}
	stock, _ = repo.GetStock(ctx, 100)
	if stock.Available != 7 {
		t.Fatalf("available after retry = %d, want 7", stock.Available)
	}
	if err := repo.ConfirmDeduct(ctx, "order-1"); err != nil {
		t.Fatalf("ConfirmDeduct error: %v", err)
	}
	if err := repo.ReleaseStock(ctx, "order-1", "refund"); err != nil {
		t.Fatalf("ReleaseStock error: %v", err)
	}
	stock, _ = repo.GetStock(ctx, 100)
	if stock.Available != 10 {
		t.Fatalf("available after release = %d, want 10", stock.Available)
	}
}

func TestRedisMySQLRepositoryReserveInsufficient(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	defer mr.Close()

	repo := NewRedisMySQLRepository(redis.MustNewRedis(redis.RedisConf{Host: mr.Addr(), Type: redis.NodeType}), nil, 2)
	ctx := context.Background()
	if err := repo.SeedStock(ctx, 100, 2, 0); err != nil {
		t.Fatalf("SeedStock error: %v", err)
	}
	err = repo.ReserveStock(ctx, "order-1", 100, 3)
	if apperror.CodeOf(err) != apperror.CodeStockInsufficient {
		t.Fatalf("CodeOf(err) = %s, want %s", apperror.CodeOf(err), apperror.CodeStockInsufficient)
	}
}

func TestRedisMySQLRepositoryFallsBackToMySQLAndReconciles(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	defer mr.Close()

	repo := NewRedisMySQLRepository(redis.MustNewRedis(redis.RedisConf{Host: mr.Addr(), Type: redis.NodeType}), db, 2)
	ctx := context.Background()

	stockQuery := regexp.QuoteMeta("SELECT COALESCE(SUM(stock), 0), COUNT(*) FROM product_stock_bucket WHERE product_id = ?")
	mock.ExpectQuery(stockQuery).WithArgs(int64(100)).WillReturnRows(sqlmock.NewRows([]string{"total", "count"}).AddRow(int64(8), int64(2)))
	stock, err := repo.GetStock(ctx, 100)
	if err != nil {
		t.Fatalf("GetStock fallback error: %v", err)
	}
	if stock.Available != 8 {
		t.Fatalf("fallback available = %d, want 8", stock.Available)
	}

	if err := mr.Set("stock:100:0", "0"); err != nil {
		t.Fatalf("set stale redis stock: %v", err)
	}
	if err := mr.Set("stock:100:1", "0"); err != nil {
		t.Fatalf("set stale redis stock: %v", err)
	}
	mock.ExpectQuery(stockQuery).WithArgs(int64(100)).WillReturnRows(sqlmock.NewRows([]string{"total", "count"}).AddRow(int64(8), int64(2)))
	before, after, changed, err := repo.ReconcileStock(ctx, 100)
	if err != nil {
		t.Fatalf("ReconcileStock error: %v", err)
	}
	if before.Available != 0 || after.Available != 8 || !changed {
		t.Fatalf("unexpected reconcile result before=%+v after=%+v changed=%v", before, after, changed)
	}
	stock, err = repo.GetStock(ctx, 100)
	if err != nil {
		t.Fatalf("GetStock after reconcile error: %v", err)
	}
	if stock.Available != 8 {
		t.Fatalf("redis available after reconcile = %d, want 8", stock.Available)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestRedisMySQLRepositorySeedsMySQLBuckets(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	defer mr.Close()

	repo := NewRedisMySQLRepository(redis.MustNewRedis(redis.RedisConf{Host: mr.Addr(), Type: redis.NodeType}), db, 2)
	mock.ExpectBegin()
	upsert := regexp.QuoteMeta("INSERT INTO product_stock_bucket (product_id, bucket_idx, stock, version) VALUES (?, ?, ?, 0) ON DUPLICATE KEY UPDATE stock = VALUES(stock), version = version + 1")
	mock.ExpectExec(upsert).WithArgs(int64(100), 0, int64(3)).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(upsert).WithArgs(int64(100), 1, int64(2)).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	if err := repo.SeedStock(context.Background(), 100, 5, 2); err != nil {
		t.Fatalf("SeedStock error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}
