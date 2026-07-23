package ordermysql

import (
	"context"
	"testing"

	"flash-mall/app/gateway/hertz/internal/application/adminops"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestAdminOpsDashboardUsesOneAggregateRoundTrip(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	columns := []string{
		"total_orders", "total_revenue", "total_products", "total_suppliers", "total_promotions",
		"active_promotions", "low_stock", "out_of_stock", "pending_orders", "paid_orders", "shipped_orders",
		"completed_orders", "refund_requested", "refunded_orders", "open_recon", "pending_events", "dead_events",
	}
	mock.ExpectQuery("SELECT[[:space:]]+\\(SELECT COUNT\\(\\*\\) FROM orders\\)").
		WillReturnRows(sqlmock.NewRows(columns).AddRow(9, 8800, 7, 2, 3, 1, 2, 1, 4, 2, 1, 1, 1, 1, 2, 3, 1))

	result, err := NewAdminOpsRepository(db).Dashboard(context.Background())
	if err != nil || result.TotalOrders != 9 || result.TotalRevenueFen != 8800 || result.PendingEvents != 3 {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAdminOpsEventsFiltersAndPaginates(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM order_outbox WHERE 1=1 AND status = \\? AND event_type = \\?").
		WithArgs(int64(2), "payment.succeeded").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("SELECT id, event_id, event_type, aggregate_id").
		WithArgs(int64(2), "payment.succeeded", int64(20), int64(20)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "event_id", "event_type", "aggregate_id", "status", "attempt_count", "last_error", "create_time", "update_time",
		}).AddRow(1, "evt-1", "payment.succeeded", "order-1", 2, 3, "timeout", "2026-07-22", "2026-07-22"))

	result, err := NewAdminOpsRepository(db).Events(context.Background(), adminops.EventQuery{
		Page: 2, PageSize: 20, Status: 2, EventType: "payment.succeeded",
	})
	if err != nil || result.Total != 1 || len(result.Items) != 1 || result.Items[0].EventID != "evt-1" {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAdminOpsRetryEventResetsDeliveryState(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	mock.ExpectExec("UPDATE order_outbox SET status = 0").WithArgs("evt-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := NewAdminOpsRepository(db).RetryEvent(context.Background(), "evt-1"); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
