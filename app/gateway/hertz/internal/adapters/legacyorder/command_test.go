package legacyorder

import (
	"context"
	"testing"

	"flash-mall/app/common/orderstatus"
	"flash-mall/app/gateway/hertz/internal/ports"

	"github.com/DATA-DOG/go-sqlmock"
)

type stockReleaserStub struct {
	orderID string
	reason  string
	meta    ports.RequestMeta
}

func (s *stockReleaserStub) ReleaseStock(_ context.Context, orderID string, reason string, meta ports.RequestMeta) error {
	s.orderID = orderID
	s.reason = reason
	s.meta = meta
	return nil
}

func TestCancelUserClosesPendingOrderAndReleasesReservation(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status FROM orders").WithArgs("order-1", int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow(orderstatus.PendingPayment))
	mock.ExpectExec("UPDATE orders SET status").WithArgs(orderstatus.Closed, "order-1", orderstatus.PendingPayment).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO order_status_log").
		WithArgs("order-1", orderstatus.PendingPayment, orderstatus.Closed, int64(9), "user cancelled: changed mind").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	releaser := &stockReleaserStub{}
	service := New(db, releaser)
	meta := ports.RequestMeta{RequestID: "req-1", UserID: 9}
	err = service.CancelUser(context.Background(), ports.CancelUserOrderCommand{
		OrderID: "order-1", Reason: "changed mind", UserID: 9, Meta: meta,
	})
	if err != nil {
		t.Fatalf("CancelUser() error = %v", err)
	}
	if releaser.orderID != "order-1" || releaser.reason != "changed mind" || releaser.meta != meta {
		t.Fatalf("unexpected release call: %+v", releaser)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
