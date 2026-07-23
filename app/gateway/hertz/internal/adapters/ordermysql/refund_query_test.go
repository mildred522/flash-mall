package ordermysql

import (
	"context"
	"testing"

	"flash-mall/app/gateway/hertz/internal/application/orderquery"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestListAdminRefundsSupportsOptionalMerchantFilter(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repository := NewQueryRepository(db)
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM refund_order r WHERE 1=1 AND r.status = \\? AND r.merchant_id = \\?").
		WithArgs(int64(1), int64(7)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("SELECT r.id").WithArgs(int64(1), int64(7), int64(20), int64(0)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "order_id", "payment_order_id", "user_id", "merchant_id", "merchant_name", "product_id",
			"refund_amount_fen", "status", "reason", "audit_remark", "operator_id", "request_time", "audit_time", "finish_time",
		}).AddRow("r-1", "o-1", "p-1", 2, 7, "商家", 8, 100, 1, "原因", "", 0, "now", "", ""))

	items, total, err := repository.ListAdminRefunds(context.Background(), orderquery.RefundListQuery{
		Page: 1, PageSize: 20, Status: 1, MerchantID: 7,
	})
	if err != nil || total != 1 || len(items) != 1 || items[0].RefundID != "r-1" {
		t.Fatalf("total=%d items=%+v err=%v", total, items, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
