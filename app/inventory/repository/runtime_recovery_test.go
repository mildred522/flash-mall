package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/alicebob/miniredis/v2"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

func TestRecoverRuntimeRestoresBucketsAndActiveReservations(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.MatchExpectationsInOrder(false)
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	expiresAt := time.Now().Add(time.Hour).Truncate(time.Second)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT TABLE_NAME FROM information_schema.TABLES")).
		WillReturnRows(allInventorySchemaRows())
	mock.ExpectQuery("SELECT product_id, bucket_idx, stock").
		WillReturnRows(sqlmock.NewRows([]string{"product_id", "bucket_idx", "stock"}).
			AddRow(int64(100), 0, int64(6)).
			AddRow(int64(100), 1, int64(4)).
			AddRow(int64(101), 0, int64(5)).
			AddRow(int64(101), 1, int64(3)))
	mock.ExpectQuery("SELECT r.order_id, r.product_id").
		WillReturnRows(sqlmock.NewRows([]string{
			"order_id", "product_id", "quantity", "shard_index", "status", "expires_at", "deducted", "reverted",
		}).
			AddRow("order-reserved", int64(100), int64(3), 1, "RESERVED", expiresAt, "", false).
			AddRow("order-confirmed", int64(101), int64(2), 1, "RESERVED", expiresAt, "DEDUCT_BUCKET_1", false))
	expectRecoverySnapshot(mock, 100, 7, 3, 10)
	expectRecoverySnapshot(mock, 101, 8, 0, 8)

	client := redis.MustNewRedis(redis.RedisConf{Host: mr.Addr(), Type: redis.NodeType})
	repo := NewRedisMySQLRepository(client, db, 2).WithReservationLedgerMode("enforce")
	if _, err := mr.ZAdd(reservationDeadLetterIndexKey, float64(time.Now().Unix()), "order-reserved"); err != nil {
		t.Fatal(err)
	}
	mr.HSet(reservationRetryCountKey, "order-reserved", "3")
	report, err := repo.RecoverRuntime(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if report.Products != 2 || report.ReservedReservations != 1 || report.ConfirmedReservations != 1 {
		t.Fatalf("unexpected recovery report: %+v", report)
	}
	assertRedisValue(t, mr, "stock:100:0", "3")
	assertRedisValue(t, mr, "stock:100:1", "4")
	assertRedisValue(t, mr, reservedStockKey(100), "3")
	assertRedisValue(t, mr, "stock:101:0", "5")
	assertRedisValue(t, mr, "stock:101:1", "3")
	if got := mr.HGet(reservationKey("order-reserved"), "status"); got != "reserved" {
		t.Fatalf("reserved status=%q", got)
	}
	if got := mr.HGet(reservationKey("order-confirmed"), "mysql_deducted"); got != "1" {
		t.Fatalf("confirmed mysql_deducted=%q", got)
	}
	if members, _ := mr.ZMembers(reservationExpiryIndexKey); len(members) != 0 {
		t.Fatalf("dead-letter reservation must not be requeued: %v", members)
	}
	deadLetters, err := mr.ZMembers(reservationDeadLetterIndexKey)
	if err != nil || len(deadLetters) != 1 || deadLetters[0] != "order-reserved" {
		t.Fatalf("dead letters=%v err=%v", deadLetters, err)
	}
	if got := mr.HGet(reservationRetryCountKey, "order-reserved"); got != "3" {
		t.Fatalf("retry count=%q, want 3", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func allInventorySchemaRows() *sqlmock.Rows {
	rows := sqlmock.NewRows([]string{"TABLE_NAME"})
	for _, table := range inventoryRequiredTables {
		rows.AddRow(table)
	}
	return rows
}

func expectRecoverySnapshot(mock sqlmock.Sqlmock, productID, available, reserved, total int64) {
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO product_stock_snapshot")).
		WithArgs(productID, available, reserved, total).
		WillReturnResult(sqlmock.NewResult(1, 1))
}

func assertRedisValue(t *testing.T, mr *miniredis.Miniredis, key, want string) {
	t.Helper()
	got, err := mr.Get(key)
	if err != nil || got != want {
		t.Fatalf("redis %s=%q err=%v, want %q", key, got, err, want)
	}
}
