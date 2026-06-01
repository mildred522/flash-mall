package logic

import (
	"context"
	"fmt"
	"testing"
	"time"

	"flash-mall/app/order/rpc/internal/config"
	"flash-mall/app/order/rpc/internal/svc"
	orderpb "flash-mall/app/order/rpc/order"
	productclient "flash-mall/app/product/rpc/productclient"

	"github.com/dtm-labs/dtm/client/dtmcli/dtmimp"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const orderCreateTestDSN = "root:6494kj06@tcp(127.0.0.1:3306)/mall_order?charset=utf8mb4&parseTime=true&loc=Local"

type stubProductRPC struct {
	card *productclient.GetProductCardResp
	err  error
}

func (s *stubProductRPC) GetProductCard(context.Context, *productclient.GetProductCardReq, ...grpc.CallOption) (*productclient.GetProductCardResp, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.card, nil
}

func (s *stubProductRPC) Deduct(context.Context, *productclient.DeductReq, ...grpc.CallOption) (*productclient.Empty, error) {
	panic("unexpected Deduct call")
}

func (s *stubProductRPC) DeductRollback(context.Context, *productclient.DeductReq, ...grpc.CallOption) (*productclient.Empty, error) {
	panic("unexpected DeductRollback call")
}

func (s *stubProductRPC) RevertStock(context.Context, *productclient.RevertStockReq, ...grpc.CallOption) (*productclient.RevertStockResp, error) {
	panic("unexpected RevertStock call")
}

func (s *stubProductRPC) ListProducts(context.Context, *productclient.ListProductsReq, ...grpc.CallOption) (*productclient.ListProductsResp, error) {
	panic("unexpected ListProducts call")
}

func TestCreateOrderLogic_CreateOrder_RejectsExpectedPriceMismatch(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		ProductRpc: &stubProductRPC{
			card: &productclient.GetProductCardResp{
				ProductId:      100,
				Name:           "Flash Coat",
				OriginPriceFen: 12900,
				FinalPriceFen:  9900,
				PromotionType:  "LIMITED_PRICE",
				PromotionTag:   "限时价",
				SupplierId:     200,
			},
		},
	}

	logic := NewCreateOrderLogic(withDTMBarrierContext(), svcCtx)
	_, err := logic.CreateOrder(&orderpb.CreateOrderReq{
		OrderId:          "order-mismatch",
		RequestId:        "req-mismatch",
		UserId:           1,
		ProductId:        100,
		Amount:           3,
		ExpectedPriceFen: 30000,
	})
	if err == nil {
		t.Fatal("expected price mismatch error")
	}
}

func TestCreateOrderLogic_CreateOrder_PersistsSnapshotAndPaymentOrder(t *testing.T) {
	dtmimp.BarrierTableName = "barrier"

	svcCtx := newOrderCreateServiceContext()
	orderID := "order-create-9001"
	requestID := "req-create-9001"

	ensureOrderCreateSchema(t, svcCtx)
	cleanupOrderCreateRows(t, svcCtx, orderID)

	logic := NewCreateOrderLogic(withDTMBarrierContext(), svcCtx)
	resp, err := logic.CreateOrder(&orderpb.CreateOrderReq{
		OrderId:          orderID,
		RequestId:        requestID,
		UserId:           42,
		ProductId:        9100,
		Amount:           3,
		ExpectedPriceFen: 29700,
	})
	if err != nil {
		t.Fatalf("CreateOrder returned error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}
	if resp.OrderId != orderID || resp.Status != orderStatusPendingPayment || resp.PaymentOrderId != paymentOrderIDFor(orderID) {
		t.Fatalf("unexpected response: %#v", resp)
	}

	var orderRow struct {
		ID        string `db:"id"`
		RequestID string `db:"request_id"`
		Status    int64  `db:"status"`
	}
	if err := svcCtx.SqlConn.QueryRowCtx(context.Background(), &orderRow, "SELECT id, request_id, status FROM orders WHERE id = ?", orderID); err != nil {
		t.Fatalf("query order row failed: %v", err)
	}
	if orderRow.RequestID != requestID || orderRow.Status != 0 {
		t.Fatalf("unexpected order row: %#v", orderRow)
	}

	var snapshotRow struct {
		OrderID          string `db:"order_id"`
		ProductID        int64  `db:"product_id"`
		Amount           int64  `db:"amount"`
		PayableAmountFen int64  `db:"payable_amount_fen"`
		PromotionType    string `db:"promotion_type"`
	}
	if err := svcCtx.SqlConn.QueryRowCtx(context.Background(), &snapshotRow, "SELECT order_id, product_id, amount, payable_amount_fen, promotion_type FROM order_price_snapshot WHERE order_id = ?", orderID); err != nil {
		t.Fatalf("query snapshot row failed: %v", err)
	}
	if snapshotRow.ProductID != 9100 || snapshotRow.Amount != 3 || snapshotRow.PayableAmountFen != 29700 {
		t.Fatalf("unexpected snapshot row: %#v", snapshotRow)
	}
	if snapshotRow.PromotionType != "LIMITED_PRICE" {
		t.Fatalf("unexpected promotion type: %#v", snapshotRow)
	}

	var paymentRow struct {
		ID               string `db:"id"`
		OrderID          string `db:"order_id"`
		Status           int64  `db:"status"`
		PayableAmountFen int64  `db:"payable_amount_fen"`
	}
	if err := svcCtx.SqlConn.QueryRowCtx(context.Background(), &paymentRow, "SELECT id, order_id, status, payable_amount_fen FROM payment_order WHERE order_id = ?", orderID); err != nil {
		t.Fatalf("query payment row failed: %v", err)
	}
	if paymentRow.ID != paymentOrderIDFor(orderID) || paymentRow.Status != 0 || paymentRow.PayableAmountFen != 29700 {
		t.Fatalf("unexpected payment row: %#v", paymentRow)
	}
}

func newOrderCreateServiceContext() *svc.ServiceContext {
	return &svc.ServiceContext{
		Config:  config.Config{DataSource: orderCreateTestDSN},
		SqlConn: sqlx.NewMysql(orderCreateTestDSN),
		ProductRpc: &stubProductRPC{
			card: &productclient.GetProductCardResp{
				ProductId:      9100,
				Name:           "Flash Coat",
				OriginPriceFen: 12900,
				FinalPriceFen:  9900,
				PromotionType:  "LIMITED_PRICE",
				PromotionTag:   "限时价",
				SupplierId:     200,
			},
		},
	}
}

func ensureOrderCreateSchema(t *testing.T, svcCtx *svc.ServiceContext) {
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
			create_time timestamp NULL DEFAULT CURRENT_TIMESTAMP,
			update_time timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			PRIMARY KEY (id),
			UNIQUE KEY uniq_order_id (order_id),
			UNIQUE KEY uniq_out_trade_no (out_trade_no)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS order_outbox (
			id bigint NOT NULL AUTO_INCREMENT,
			event_id varchar(128) NOT NULL,
			event_type varchar(64) NOT NULL,
			aggregate_id varchar(64) NOT NULL,
			payload json NOT NULL,
			status tinyint NOT NULL DEFAULT 0,
			attempt_count int NOT NULL DEFAULT 0,
			next_retry_at timestamp NULL DEFAULT CURRENT_TIMESTAMP,
			published_at timestamp NULL DEFAULT NULL,
			last_error varchar(255) NOT NULL DEFAULT '',
			create_time timestamp NULL DEFAULT CURRENT_TIMESTAMP,
			update_time timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			PRIMARY KEY (id),
			UNIQUE KEY uniq_event_id (event_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS barrier (
			id bigint NOT NULL AUTO_INCREMENT,
			trans_type varchar(45) NOT NULL,
			gid varchar(128) NOT NULL,
			branch_id varchar(128) NOT NULL,
			op varchar(45) NOT NULL,
			barrier_id varchar(45) NOT NULL,
			reason varchar(45) DEFAULT '',
			create_time datetime DEFAULT CURRENT_TIMESTAMP,
			update_time datetime DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			PRIMARY KEY (id),
			UNIQUE KEY uniq_barrier (gid, branch_id, op, barrier_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}

	for _, statement := range statements {
		if _, err := svcCtx.SqlConn.ExecCtx(context.Background(), statement); err != nil {
			t.Fatalf("schema ensure failed for %q: %v", statement, err)
		}
	}
}

func cleanupOrderCreateRows(t *testing.T, svcCtx *svc.ServiceContext, orderID string) {
	t.Helper()

	statements := []string{
		fmt.Sprintf("DELETE FROM payment_order WHERE order_id = '%s'", orderID),
		fmt.Sprintf("DELETE FROM order_price_snapshot WHERE order_id = '%s'", orderID),
		fmt.Sprintf("DELETE FROM orders WHERE id = '%s'", orderID),
		fmt.Sprintf("DELETE FROM order_outbox WHERE aggregate_id = '%s'", orderID),
	}
	for _, statement := range statements {
		if _, err := svcCtx.SqlConn.ExecCtx(context.Background(), statement); err != nil {
			t.Fatalf("cleanup failed for %q: %v", statement, err)
		}
	}
}

func withDTMBarrierContext() context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		"dtm-gid", fmt.Sprintf("gid-test-%d", time.Now().UnixNano()),
		"dtm-trans_type", "saga",
		"dtm-branch_id", "01",
		"dtm-op", "action",
		"dtm-dtm", "127.0.0.1:36790",
	))
}
