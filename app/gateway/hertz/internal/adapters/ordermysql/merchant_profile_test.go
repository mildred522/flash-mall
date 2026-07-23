package ordermysql

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestMerchantProfileRepositoryReadsMerchantMemberships(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer func() { _ = db.Close() }()
	mock.ExpectQuery("SELECT m.id, m.name, mu.role, m.status").WithArgs(int64(1001)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "role", "status"}).AddRow(7, "山岚商店", "owner", 1))

	items, err := NewMerchantProfileRepository(db).MerchantsByUser(context.Background(), 1001)
	if err != nil || len(items) != 1 || items[0].MerchantID != 7 {
		t.Fatalf("items = %+v, err = %v", items, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}

func TestMerchantProfileRepositoryReturnsMissingApplication(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer func() { _ = db.Close() }()
	mock.ExpectQuery("SELECT id, merchant_name, contact_phone, status").WithArgs(int64(1001)).WillReturnError(sql.ErrNoRows)

	_, found, err := NewMerchantProfileRepository(db).LatestApplication(context.Background(), 1001)
	if err != nil || found {
		t.Fatalf("found = %v, err = %v, want missing without error", found, err)
	}
}

func TestMerchantProfileRepositoryReadsDashboardAggregates(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer func() { _ = db.Close() }()
	mock.ExpectQuery("SELECT COUNT\\(\\*\\)").WithArgs(int64(1), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"orders", "paid", "ship_pending"}).AddRow(9, 6, 2))
	mock.ExpectQuery("SELECT COALESCE\\(SUM\\(s.payable_amount_fen\\), 0\\)").WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"sales"}).AddRow(8800))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM refund_order").WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"refunds"}).AddRow(1))

	stats, err := NewMerchantProfileRepository(db).Dashboard(context.Background(), 7)
	if err != nil || stats.OrderCount != 9 || stats.SalesAmountFen != 8800 || stats.RefundPending != 1 {
		t.Fatalf("stats = %+v, err = %v", stats, err)
	}
}

func TestMerchantScopeQueriesMembership(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	repository := NewMerchantProfileRepository(db)
	mock.ExpectQuery("SELECT COUNT\\(\\*\\).*FROM merchant_user").WithArgs(int64(1001), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	allowed, err := repository.UserCanAccessMerchant(context.Background(), 1001, 7)
	if err != nil || !allowed {
		t.Fatalf("allowed=%v err=%v", allowed, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
