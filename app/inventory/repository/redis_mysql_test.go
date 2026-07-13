package repository

import (
	"context"
	"regexp"
	"testing"

	"flash-mall/app/common/apperror"
	"flash-mall/app/inventory/domain"

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
	if err := repo.ReserveStock(ctx, "order-1", 100, 3, domain.StockChangeMeta{}); err != nil {
		t.Fatalf("ReserveStock error: %v", err)
	}
	stock, _ = repo.GetStock(ctx, 100)
	if stock.Available != 7 || stock.Reserved != 3 || stock.Total != 10 {
		t.Fatalf("unexpected stock after reserve: %+v", stock)
	}
	if err := repo.ReserveStock(ctx, "order-1", 100, 3, domain.StockChangeMeta{}); err != nil {
		t.Fatalf("ReserveStock retry error: %v", err)
	}
	stock, _ = repo.GetStock(ctx, 100)
	if stock.Available != 7 || stock.Reserved != 3 || stock.Total != 10 {
		t.Fatalf("unexpected stock after retry: %+v", stock)
	}
	if err := repo.ConfirmDeduct(ctx, "order-1", domain.StockChangeMeta{}); err != nil {
		t.Fatalf("ConfirmDeduct error: %v", err)
	}
	stock, _ = repo.GetStock(ctx, 100)
	if stock.Available != 7 || stock.Reserved != 0 || stock.Total != 7 {
		t.Fatalf("unexpected stock after confirm: %+v", stock)
	}
	if err := repo.ReleaseStock(ctx, "order-1", domain.StockChangeMeta{Reason: "refund"}); err != nil {
		t.Fatalf("ReleaseStock error: %v", err)
	}
	stock, _ = repo.GetStock(ctx, 100)
	if stock.Available != 10 {
		t.Fatalf("available after release = %d, want 10", stock.Available)
	}
}

func TestConfirmDeductScriptDistinguishesFirstApplyFromReplay(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	defer mr.Close()

	client := redis.MustNewRedis(redis.RedisConf{Host: mr.Addr(), Type: redis.NodeType})
	repo := NewRedisMySQLRepository(client, nil, 2)
	ctx := context.Background()
	if err := repo.SeedStock(ctx, 100, 10, 0); err != nil {
		t.Fatal(err)
	}
	if err := repo.ReserveStock(ctx, "order-confirm-replay", 100, 2, domain.StockChangeMeta{}); err != nil {
		t.Fatal(err)
	}

	first, err := evalInt64(ctx, client, confirmDeductLuaScript, []string{reservationKey("order-confirm-replay")}, confirmedReservationTTLSeconds, 0)
	if err != nil {
		t.Fatal(err)
	}
	second, err := evalInt64(ctx, client, confirmDeductLuaScript, []string{reservationKey("order-confirm-replay")}, confirmedReservationTTLSeconds, 0)
	if err != nil {
		t.Fatal(err)
	}
	if first != 3 || second != 1 {
		t.Fatalf("confirm results first=%d second=%d, want first=3 applied and second=1 replay", first, second)
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
	err = repo.ReserveStock(ctx, "order-1", 100, 3, domain.StockChangeMeta{})
	if apperror.CodeOf(err) != apperror.CodeStockInsufficient {
		t.Fatalf("CodeOf(err) = %s, want %s", apperror.CodeOf(err), apperror.CodeStockInsufficient)
	}
}

func TestRedisMySQLRepositoryReleaseIsIdempotent(t *testing.T) {
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
	if err := repo.ReserveStock(ctx, "order-release", 100, 3, domain.StockChangeMeta{}); err != nil {
		t.Fatalf("ReserveStock error: %v", err)
	}
	if err := repo.ReleaseStock(ctx, "order-release", domain.StockChangeMeta{Reason: "rollback"}); err != nil {
		t.Fatalf("first ReleaseStock error: %v", err)
	}
	if err := repo.ReleaseStock(ctx, "order-release", domain.StockChangeMeta{Reason: "rollback"}); err != nil {
		t.Fatalf("repeat ReleaseStock error: %v", err)
	}
	stock, err := repo.GetStock(ctx, 100)
	if err != nil {
		t.Fatalf("GetStock error: %v", err)
	}
	if stock.Available != 10 || stock.Reserved != 0 || stock.Total != 10 {
		t.Fatalf("repeat release changed stock: %+v", stock)
	}
}

func TestRedisMySQLRepositoryReleaseReportsRedisFailure(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	repo := NewRedisMySQLRepository(redis.MustNewRedis(redis.RedisConf{Host: mr.Addr(), Type: redis.NodeType}), nil, 2)
	ctx := context.Background()
	if err := repo.SeedStock(ctx, 100, 10, 0); err != nil {
		t.Fatalf("SeedStock error: %v", err)
	}
	if err := repo.ReserveStock(ctx, "order-redis-down", 100, 1, domain.StockChangeMeta{}); err != nil {
		t.Fatalf("ReserveStock error: %v", err)
	}
	mr.Close()

	if err := repo.ReleaseStock(ctx, "order-redis-down", domain.StockChangeMeta{Reason: "rollback"}); err == nil {
		t.Fatal("ReleaseStock must report Redis unavailability to the SAGA caller")
	}
}

func TestRedisMySQLRepositoryBatchGetStock(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	defer mr.Close()

	repo := NewRedisMySQLRepository(redis.MustNewRedis(redis.RedisConf{Host: mr.Addr(), Type: redis.NodeType}), nil, 2)
	ctx := context.Background()
	if err := repo.SeedStock(ctx, 100, 10, 0); err != nil {
		t.Fatalf("SeedStock 100 error: %v", err)
	}
	if err := repo.SeedStock(ctx, 101, 6, 0); err != nil {
		t.Fatalf("SeedStock 101 error: %v", err)
	}
	if err := repo.ReserveStock(ctx, "order-1", 100, 3, domain.StockChangeMeta{}); err != nil {
		t.Fatalf("ReserveStock error: %v", err)
	}
	stocks, err := repo.BatchGetStock(ctx, []int64{100, 101, 100})
	if err != nil {
		t.Fatalf("BatchGetStock error: %v", err)
	}
	if len(stocks) != 2 {
		t.Fatalf("len(stocks) = %d, want 2: %+v", len(stocks), stocks)
	}
	if stocks[0].ProductID != 100 || stocks[0].Available != 7 || stocks[0].Reserved != 3 || stocks[0].Total != 10 {
		t.Fatalf("unexpected stock 100: %+v", stocks[0])
	}
	if stocks[1].ProductID != 101 || stocks[1].Available != 6 || stocks[1].Reserved != 0 || stocks[1].Total != 6 {
		t.Fatalf("unexpected stock 101: %+v", stocks[1])
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
	before, after, changed, err := repo.ReconcileStock(ctx, 100, domain.StockChangeMeta{})
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
	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS product_stock_snapshot")).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO product_stock_snapshot")).WithArgs(int64(100), int64(5), int64(0), int64(5)).WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.SeedStock(context.Background(), 100, 5, 2); err != nil {
		t.Fatalf("SeedStock error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}
