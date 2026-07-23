package productmysql

import (
	"context"
	"testing"

	"flash-mall/app/gateway/hertz/internal/application/stockaudit"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestStockAuditListScopesMerchantThroughProductOwnership(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM mall_product.inventory_stock_change_log").
		WithArgs(int64(8), int64(7)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("SELECT id, product_id").WithArgs(int64(8), int64(7), int64(20), int64(0)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "product_id", "order_id", "change_type", "delta", "before_available", "after_available", "reason",
			"request_id", "trace_id", "operator_user_id", "operator_merchant_id", "operator_role", "create_time",
		}).AddRow(1, 8, "o-1", "RESERVE", -1, 10, 9, "checkout", "req-1", "trace-1", 2, 7, "merchant", "now"))

	items, total, err := NewStockAuditRepository(db).List(context.Background(), stockaudit.Query{
		Page: 1, PageSize: 20, ProductID: 8, MerchantID: 7,
	})
	if err != nil || total != 1 || len(items) != 1 || items[0].AfterAvailable != 9 {
		t.Fatalf("total=%d items=%+v err=%v", total, items, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
