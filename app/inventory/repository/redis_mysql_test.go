package repository

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

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

func TestRedisMySQLRepositoryRejectsConflictingReservationReplay(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()
	repo := NewRedisMySQLRepository(redis.MustNewRedis(redis.RedisConf{Host: mr.Addr(), Type: redis.NodeType}), nil, 2)
	ctx := context.Background()
	if err := repo.SeedStock(ctx, 100, 10, 0); err != nil {
		t.Fatal(err)
	}
	if err := repo.SeedStock(ctx, 101, 10, 0); err != nil {
		t.Fatal(err)
	}
	if err := repo.ReserveStock(ctx, "order-conflict", 100, 2, domain.StockChangeMeta{}); err != nil {
		t.Fatal(err)
	}
	err = repo.ReserveStock(ctx, "order-conflict", 101, 3, domain.StockChangeMeta{})
	if apperror.CodeOf(err) != apperror.CodeConflict {
		t.Fatalf("CodeOf(err)=%s, want %s", apperror.CodeOf(err), apperror.CodeConflict)
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

func TestReservationExpiryIndexTracksOnlyActiveReservations(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()
	repo := NewRedisMySQLRepository(redis.MustNewRedis(redis.RedisConf{Host: mr.Addr(), Type: redis.NodeType}), nil, 2)
	ctx := context.Background()
	if err := repo.SeedStock(ctx, 100, 10, 0); err != nil {
		t.Fatal(err)
	}
	if err := repo.ReserveStock(ctx, "order-expiry", 100, 2, domain.StockChangeMeta{}); err != nil {
		t.Fatal(err)
	}
	members, err := mr.ZMembers(reservationExpiryIndexKey)
	if err != nil || len(members) != 1 || members[0] != "order-expiry" {
		t.Fatalf("active expiry members=%v err=%v", members, err)
	}
	score, err := mr.ZScore(reservationExpiryIndexKey, "order-expiry")
	if err != nil {
		t.Fatal(err)
	}
	logicalTTL := time.Until(time.Unix(int64(score), 0))
	keyTTL := mr.TTL(reservationKey("order-expiry"))
	if keyTTL < logicalTTL+time.Hour {
		t.Fatalf("reservation key TTL %s must outlive logical expiry %s for recovery", keyTTL, logicalTTL)
	}
	if err := repo.ConfirmDeduct(ctx, "order-expiry", domain.StockChangeMeta{}); err != nil {
		t.Fatal(err)
	}
	if mr.Exists(reservationExpiryIndexKey) {
		members, _ = mr.ZMembers(reservationExpiryIndexKey)
		if len(members) != 0 {
			t.Fatalf("confirmed reservation remained in expiry index: %v", members)
		}
	}
}

func TestReservationStatsUseSharedRedisState(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()
	repo := NewRedisMySQLRepository(redis.MustNewRedis(redis.RedisConf{Host: mr.Addr(), Type: redis.NodeType}), nil, 2)
	ctx := context.Background()
	if err := repo.SeedStock(ctx, 100, 10, 0); err != nil {
		t.Fatal(err)
	}
	if err := repo.ReserveStock(ctx, "order-stats", 100, 2, domain.StockChangeMeta{}); err != nil {
		t.Fatal(err)
	}
	stats, err := repo.ReservationStats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Active != 1 || stats.Processing != 0 || stats.DeadLetter != 0 {
		t.Fatalf("stats after reserve: %+v", stats)
	}
	if err := repo.ConfirmDeduct(ctx, "order-stats", domain.StockChangeMeta{}); err != nil {
		t.Fatal(err)
	}
	stats, err = repo.ReservationStats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Active != 0 {
		t.Fatalf("stats after confirm: %+v", stats)
	}
}

func TestReleaseExpiredReservationsRestoresStock(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()
	repo := NewRedisMySQLRepository(redis.MustNewRedis(redis.RedisConf{Host: mr.Addr(), Type: redis.NodeType}), nil, 2)
	ctx := context.Background()
	if err := repo.SeedStock(ctx, 100, 10, 0); err != nil {
		t.Fatal(err)
	}
	if err := repo.ReserveStock(ctx, "order-expired", 100, 3, domain.StockChangeMeta{}); err != nil {
		t.Fatal(err)
	}
	if _, err := mr.ZAdd(reservationExpiryIndexKey, float64(time.Now().Add(-time.Minute).Unix()), "order-expired"); err != nil {
		t.Fatal(err)
	}
	processed, err := repo.ReleaseExpiredReservations(ctx, 10, domain.StockChangeMeta{Reason: "reservation timeout"})
	if err != nil {
		t.Fatal(err)
	}
	if processed != 1 {
		t.Fatalf("processed=%d, want 1", processed)
	}
	stock, err := repo.GetStock(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	if stock.Available != 10 || stock.Reserved != 0 {
		t.Fatalf("stock after timeout release: %+v", stock)
	}
}

func TestReleaseStockRecoversMissingRedisReservationFromLedger(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()
	repo := NewRedisMySQLRepository(redis.MustNewRedis(redis.RedisConf{Host: mr.Addr(), Type: redis.NodeType}), db, 2).
		WithReservationLedgerMode("enforce")
	ctx := context.Background()
	if err := repo.seedRedisState(ctx, 100, 7, 3, 2); err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT product_id, quantity, shard_index, status, expires_at, request_id, trace_id FROM inventory_reservation WHERE order_id = ?")).
		WithArgs("order-ledger-recover").
		WillReturnRows(sqlmock.NewRows([]string{"product_id", "quantity", "shard_index", "status", "expires_at", "request_id", "trace_id"}).
			AddRow(int64(100), int64(3), 1, "RESERVED", time.Now().Add(-time.Minute), "req", "trace"))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE inventory_reservation SET status = ?, version = version + 1")).
		WithArgs("RELEASED", "order-ledger-recover", "RESERVED", "CONFIRMED").
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.ReleaseStock(ctx, "order-ledger-recover", domain.StockChangeMeta{Reason: "timeout"}); err != nil {
		t.Fatal(err)
	}
	stock, err := repo.GetStock(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	if stock.Available != 10 || stock.Reserved != 0 {
		t.Fatalf("stock after ledger recovery: %+v", stock)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
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

func TestRedisMySQLRepositoryRuntimeCheckReportsRedisFailure(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	repo := NewRedisMySQLRepository(redis.MustNewRedis(redis.RedisConf{Host: mr.Addr(), Type: redis.NodeType}), nil, 2)
	if err := repo.CheckRuntime(context.Background()); err != nil {
		t.Fatalf("healthy runtime check: %v", err)
	}
	mr.Close()
	if err := repo.CheckRuntime(context.Background()); err == nil {
		t.Fatal("runtime check must fail when redis is unavailable")
	}
}

func TestRedisMySQLRepositoryRuntimeCheckRejectsMissingSchema(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectPing()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT TABLE_NAME FROM information_schema.TABLES")).
		WillReturnRows(sqlmock.NewRows([]string{"TABLE_NAME"}).
			AddRow("product").
			AddRow("stock_log").
			AddRow("product_stock_bucket").
			AddRow("product_stock_snapshot").
			AddRow("inventory_reservation"))

	repo := NewRedisMySQLRepository(redis.MustNewRedis(redis.RedisConf{Host: mr.Addr(), Type: redis.NodeType}), db, 2)
	err = repo.CheckRuntime(context.Background())
	if err == nil || !strings.Contains(err.Error(), "inventory_stock_change_log") {
		t.Fatalf("CheckRuntime error=%v, want missing inventory_stock_change_log", err)
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

func TestReconcilePreservesActiveReservedStock(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()
	repo := NewRedisMySQLRepository(redis.MustNewRedis(redis.RedisConf{Host: mr.Addr(), Type: redis.NodeType}), db, 2).
		WithReservationLedgerMode("enforce")
	ctx := context.Background()
	if err := repo.seedRedis(ctx, 100, 7, 2); err != nil {
		t.Fatal(err)
	}
	if err := mr.Set(reservedStockKey(100), "3"); err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(SUM(stock), 0), COUNT(*) FROM product_stock_bucket WHERE product_id = ?")).
		WithArgs(int64(100)).
		WillReturnRows(sqlmock.NewRows([]string{"total", "count"}).AddRow(int64(10), int64(2)))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(SUM(quantity), 0) FROM inventory_reservation WHERE product_id = ? AND status = 'RESERVED'")).
		WithArgs(int64(100)).
		WillReturnRows(sqlmock.NewRows([]string{"reserved"}).AddRow(int64(3)))

	before, after, changed, err := repo.ReconcileStock(ctx, 100, domain.StockChangeMeta{})
	if err != nil {
		t.Fatal(err)
	}
	if changed || before != after || after.Available != 7 || after.Reserved != 3 || after.Total != 10 {
		t.Fatalf("unexpected reconcile before=%+v after=%+v changed=%v", before, after, changed)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
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
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO product_stock_snapshot")).WithArgs(int64(100), int64(5), int64(0), int64(5)).WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.SeedStock(context.Background(), 100, 5, 2); err != nil {
		t.Fatalf("SeedStock error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestNormalizeReservationLedgerMode(t *testing.T) {
	for input, want := range map[string]string{
		"":         "off",
		"OFF":      "off",
		" shadow ": "shadow",
		"ENFORCE":  "enforce",
		"unknown":  "off",
	} {
		if got := normalizeReservationLedgerMode(input); got != want {
			t.Fatalf("normalizeReservationLedgerMode(%q)=%q, want %q", input, got, want)
		}
	}
}

func TestExpectedAvailableAccountsForActiveReservations(t *testing.T) {
	tests := []struct {
		name           string
		mysqlTotal     int64
		redisReserved  int64
		ledgerReserved int64
		mode           string
		want           int64
	}{
		{name: "off uses redis", mysqlTotal: 10, redisReserved: 3, mode: "off", want: 7},
		{name: "shadow uses safer larger reservation", mysqlTotal: 10, redisReserved: 3, ledgerReserved: 2, mode: "shadow", want: 7},
		{name: "enforce uses ledger", mysqlTotal: 10, redisReserved: 3, ledgerReserved: 2, mode: "enforce", want: 8},
		{name: "never negative", mysqlTotal: 1, ledgerReserved: 3, mode: "enforce", want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := expectedAvailable(tt.mysqlTotal, tt.redisReserved, tt.ledgerReserved, tt.mode); got != tt.want {
				t.Fatalf("expectedAvailable()=%d, want %d", got, tt.want)
			}
		})
	}
}

func TestInsertReservationLedgerPersistsReservationIdentity(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := NewRedisMySQLRepository(nil, db, 4).WithReservationLedgerMode("shadow")
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO inventory_reservation")).
		WithArgs("order-ledger", int64(100), int64(2), 3, "RESERVED", sqlmock.AnyArg(), "req-1", "trace-1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = repo.insertReservationLedger(context.Background(), reservationLedgerRecord{
		OrderID:    "order-ledger",
		ProductID:  100,
		Quantity:   2,
		ShardIndex: 3,
		Status:     domain.ReservationReserved,
		ExpiresAt:  time.Now().Add(time.Hour),
		RequestID:  "req-1",
		TraceID:    "trace-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestInsertReservationLedgerRejectsIdentityConflict(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := NewRedisMySQLRepository(nil, db, 4).WithReservationLedgerMode("enforce")
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO inventory_reservation")).
		WithArgs("order-conflict", int64(100), int64(2), 1, "RESERVED", sqlmock.AnyArg(), "req", "trace").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT product_id, quantity FROM inventory_reservation WHERE order_id = ?")).
		WithArgs("order-conflict").
		WillReturnRows(sqlmock.NewRows([]string{"product_id", "quantity"}).AddRow(int64(101), int64(3)))

	err = repo.insertReservationLedger(context.Background(), reservationLedgerRecord{
		OrderID: "order-conflict", ProductID: 100, Quantity: 2, ShardIndex: 1,
		Status: domain.ReservationReserved, ExpiresAt: time.Now(), RequestID: "req", TraceID: "trace",
	})
	if apperror.CodeOf(err) != apperror.CodeConflict {
		t.Fatalf("CodeOf(err)=%s, want conflict: %v", apperror.CodeOf(err), err)
	}
}

func TestReservationLedgerModeControlsWriteFailure(t *testing.T) {
	for _, tt := range []struct {
		mode    string
		wantErr bool
	}{
		{mode: "shadow", wantErr: false},
		{mode: "enforce", wantErr: true},
	} {
		t.Run(tt.mode, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			mr, err := miniredis.Run()
			if err != nil {
				t.Fatal(err)
			}
			defer mr.Close()
			repo := NewRedisMySQLRepository(redis.MustNewRedis(redis.RedisConf{Host: mr.Addr(), Type: redis.NodeType}), db, 2).
				WithReservationLedgerMode(tt.mode)
			if err := repo.seedRedis(context.Background(), 100, 10, 2); err != nil {
				t.Fatal(err)
			}
			mock.ExpectExec(regexp.QuoteMeta("INSERT INTO inventory_reservation")).
				WillReturnError(errors.New("ledger unavailable"))

			err = repo.ReserveStock(context.Background(), "order-ledger-failure", 100, 2, domain.StockChangeMeta{})
			if (err != nil) != tt.wantErr {
				t.Fatalf("ReserveStock error=%v, wantErr=%v", err, tt.wantErr)
			}
		})
	}
}

func TestTransitionReservationLedgerAdvancesVersion(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := NewRedisMySQLRepository(nil, db, 4).WithReservationLedgerMode("enforce")
	mock.ExpectExec(regexp.QuoteMeta("UPDATE inventory_reservation SET status = ?, version = version + 1")).
		WithArgs("CONFIRMED", "order-transition", "RESERVED").
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.transitionReservationLedger(context.Background(), "order-transition", domain.ReservationConfirmed, domain.ReservationReserved); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
