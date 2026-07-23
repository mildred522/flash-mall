package ordermysql

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestPaymentByClaimsUsesAllTokenIdentifiers(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT o.id, o.user_id, o.status, p.id").
		WithArgs("pay-1", "order-1", "trade-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"order_id", "user_id", "order_status", "payment_id", "payment_status", "out_trade_no", "payable",
		}).AddRow("order-1", 7, 0, "pay-1", 0, "trade-1", 100))
	payment, found, err := NewQueryRepository(db).PaymentByClaims(context.Background(), "pay-1", "order-1", "trade-1")
	if err != nil || !found || payment.PayableAmountFen != 100 {
		t.Fatalf("payment=%+v found=%v err=%v", payment, found, err)
	}
}
