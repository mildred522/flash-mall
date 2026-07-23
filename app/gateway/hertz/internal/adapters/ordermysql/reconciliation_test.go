package ordermysql

import (
	"context"
	"testing"

	"flash-mall/app/common/orderstatus"
	"flash-mall/app/common/paymentstatus"
	"flash-mall/app/gateway/hertz/internal/application/reconciliation"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestReconciliationListFiltersAndPaginates(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM reconciliation_issue WHERE 1=1 AND status = \\? AND order_id = \\?").
		WithArgs(int64(0), "order-1").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("SELECT id, issue_type, order_id").WithArgs(int64(0), "order-1", int64(20), int64(20)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "issue_type", "order_id", "payment_order_id", "refund_order_id", "expected", "actual",
			"severity", "status", "detail", "create_time",
		}).AddRow(1, "payment_amount_mismatch", "order-1", "pay-1", "", 100, 90, 3, 0, "mismatch", "2026-07-22"))

	result, err := NewReconciliationRepository(db).List(context.Background(), reconciliation.Query{
		Page: 2, PageSize: 20, Status: 0, OrderID: "order-1",
	})
	if err != nil || result.Total != 1 || len(result.Items) != 1 || result.Items[0].OrderID != "order-1" {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestReconciliationScanUpsertsAllDetectors(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	mock.ExpectExec("INSERT INTO reconciliation_issue").WithArgs(paymentstatus.Success).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("INSERT INTO reconciliation_issue").WithArgs(paymentstatus.Success, orderstatus.PendingPayment).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO reconciliation_issue").WithArgs(
		orderstatus.Paid, orderstatus.Shipped, orderstatus.Completed,
		orderstatus.RefundRequested, orderstatus.Refunded, paymentstatus.Success,
	).WillReturnResult(sqlmock.NewResult(0, 1))

	inserted, err := NewReconciliationRepository(db).Scan(context.Background())
	if err != nil || inserted != 4 {
		t.Fatalf("inserted = %d, err = %v", inserted, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
