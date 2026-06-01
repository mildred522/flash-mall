package logic

import (
	"context"
	"fmt"
	"testing"

	"flash-mall/app/order/rpc/internal/config"
	"flash-mall/app/order/rpc/internal/svc"
	orderpb "flash-mall/app/order/rpc/order"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const orderDetailTestDSN = "root:6494kj06@tcp(127.0.0.1:3306)/mall_order?charset=utf8mb4&parseTime=true&loc=Local"

func TestGetOrderDetailLogic_GetOrderDetail_ReturnsSnapshotBackedFields(t *testing.T) {
	svcCtx := newOrderDetailServiceContext()
	orderID := "o-detail-1"

	ensureOrderDetailSchema(t, svcCtx)
	cleanupOrderDetailRows(t, svcCtx, orderID)
	seedOrderDetailRows(t, svcCtx, orderID)

	resp, err := NewGetOrderDetailLogic(context.Background(), svcCtx).GetOrderDetail(&orderpb.GetOrderDetailReq{
		OrderId: orderID,
	})
	if err != nil {
		t.Fatalf("GetOrderDetail returned error: %v", err)
	}
	if resp.OrderId != orderID || resp.UserId != 7 {
		t.Fatalf("unexpected basic detail: %#v", resp)
	}
	if resp.OrderStatus != "PENDING_PAYMENT" {
		t.Fatalf("unexpected order status: %#v", resp)
	}
	if resp.PaymentStatus != "INIT" {
		t.Fatalf("unexpected payment status: %#v", resp)
	}
	if resp.PayableAmountFen != 29700 || resp.OriginPriceFen != 38700 || resp.DiscountAmountFen != 9000 {
		t.Fatalf("unexpected price detail: %#v", resp)
	}
}

func newOrderDetailServiceContext() *svc.ServiceContext {
	return &svc.ServiceContext{
		Config:  config.Config{DataSource: orderDetailTestDSN},
		SqlConn: sqlx.NewMysql(orderDetailTestDSN),
	}
}

func ensureOrderDetailSchema(t *testing.T, svcCtx *svc.ServiceContext) {
	t.Helper()

	statements := []string{
		`CREATE TABLE IF NOT EXISTS orders (
			id varchar(64) NOT NULL,
			request_id varchar(64) DEFAULT NULL,
			user_id bigint NOT NULL DEFAULT 0,
			product_id bigint NOT NULL DEFAULT 0,
			amount int NOT NULL DEFAULT 0,
			status tinyint NOT NULL DEFAULT 0,
			create_time timestamp NULL DEFAULT CURRENT_TIMESTAMP,
			update_time timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			PRIMARY KEY (id),
			UNIQUE KEY uniq_request_id (request_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS order_price_snapshot (
			order_id varchar(64) NOT NULL,
			product_id bigint NOT NULL DEFAULT 0,
			supplier_id bigint NOT NULL DEFAULT 0,
			product_name varchar(128) NOT NULL DEFAULT '',
			amount int NOT NULL DEFAULT 0,
			origin_unit_price_fen bigint NOT NULL DEFAULT 0,
			sale_unit_price_fen bigint NOT NULL DEFAULT 0,
			payable_amount_fen bigint NOT NULL DEFAULT 0,
			discount_amount_fen bigint NOT NULL DEFAULT 0,
			promotion_type varchar(32) NOT NULL DEFAULT '',
			promotion_tag varchar(64) NOT NULL DEFAULT '',
			create_time timestamp NULL DEFAULT CURRENT_TIMESTAMP,
			update_time timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			PRIMARY KEY (order_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS payment_order (
			id varchar(64) NOT NULL,
			order_id varchar(64) NOT NULL,
			user_id bigint NOT NULL DEFAULT 0,
			payable_amount_fen bigint NOT NULL DEFAULT 0,
			status tinyint NOT NULL DEFAULT 0,
			out_trade_no varchar(64) NOT NULL DEFAULT '',
			paid_at timestamp NULL DEFAULT NULL,
			callback_payload json DEFAULT NULL,
			create_time timestamp NULL DEFAULT CURRENT_TIMESTAMP,
			update_time timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			PRIMARY KEY (id),
			UNIQUE KEY uniq_order_id (order_id),
			UNIQUE KEY uniq_out_trade_no (out_trade_no)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}

	for _, statement := range statements {
		if _, err := svcCtx.SqlConn.ExecCtx(context.Background(), statement); err != nil {
			t.Fatalf("schema ensure failed for %q: %v", statement, err)
		}
	}
}

func cleanupOrderDetailRows(t *testing.T, svcCtx *svc.ServiceContext, orderID string) {
	t.Helper()

	statements := []string{
		fmt.Sprintf("DELETE FROM payment_order WHERE order_id = '%s'", orderID),
		fmt.Sprintf("DELETE FROM order_price_snapshot WHERE order_id = '%s'", orderID),
		fmt.Sprintf("DELETE FROM orders WHERE id = '%s'", orderID),
	}

	for _, statement := range statements {
		if _, err := svcCtx.SqlConn.ExecCtx(context.Background(), statement); err != nil {
			t.Fatalf("cleanup failed for %q: %v", statement, err)
		}
	}
}

func seedOrderDetailRows(t *testing.T, svcCtx *svc.ServiceContext, orderID string) {
	t.Helper()

	statements := []string{
		fmt.Sprintf("INSERT INTO orders (id, request_id, user_id, product_id, amount, status) VALUES ('%s', 'req-%s', 7, 9100, 3, 0)", orderID, orderID),
		fmt.Sprintf("INSERT INTO order_price_snapshot (order_id, product_id, supplier_id, product_name, amount, origin_unit_price_fen, sale_unit_price_fen, payable_amount_fen, discount_amount_fen, promotion_type, promotion_tag) VALUES ('%s', 9100, 200, 'Flash Coat', 3, 12900, 9900, 29700, 9000, 'LIMITED_PRICE', '限时价')", orderID),
		fmt.Sprintf("INSERT INTO payment_order (id, order_id, user_id, payable_amount_fen, status, out_trade_no) VALUES ('pay:%s', '%s', 7, 29700, 0, 'mock-%s')", orderID, orderID, orderID),
	}

	for _, statement := range statements {
		if _, err := svcCtx.SqlConn.ExecCtx(context.Background(), statement); err != nil {
			t.Fatalf("seed failed for %q: %v", statement, err)
		}
	}
}
