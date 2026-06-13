package logic

import (
	"context"
	"errors"
	"testing"

	"flash-mall/app/order/rpc/internal/config"
	"flash-mall/app/order/rpc/internal/svc"
	orderpb "flash-mall/app/order/rpc/order"
	productpb "flash-mall/app/product/rpc/productclient"

	"github.com/alicebob/miniredis/v2"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc"
)

const refundTestDSN = "root:6494kj06@tcp(127.0.0.1:3306)/mall_order?charset=utf8mb4&parseTime=true&loc=Local"

type refundProductRPC struct {
	revertErr   error
	revertCalls int
}

func (p *refundProductRPC) GetProductCard(context.Context, *productpb.GetProductCardReq, ...grpc.CallOption) (*productpb.GetProductCardResp, error) {
	panic("unexpected GetProductCard call")
}

func (p *refundProductRPC) ListProducts(context.Context, *productpb.ListProductsReq, ...grpc.CallOption) (*productpb.ListProductsResp, error) {
	panic("unexpected ListProducts call")
}

func (p *refundProductRPC) Deduct(context.Context, *productpb.DeductReq, ...grpc.CallOption) (*productpb.Empty, error) {
	panic("unexpected Deduct call")
}

func (p *refundProductRPC) DeductRollback(context.Context, *productpb.DeductReq, ...grpc.CallOption) (*productpb.Empty, error) {
	panic("unexpected DeductRollback call")
}

func (p *refundProductRPC) RevertStock(context.Context, *productpb.RevertStockReq, ...grpc.CallOption) (*productpb.RevertStockResp, error) {
	p.revertCalls++
	if p.revertErr != nil {
		return nil, p.revertErr
	}
	return &productpb.RevertStockResp{}, nil
}

func TestRequestRefund_MovesPaidOrderToRefundRequested(t *testing.T) {
	svcCtx, cleanup := newRefundServiceContext(t, &refundProductRPC{})
	defer cleanup()

	orderID := "o-refund-request"
	ensureRefundSchema(t, svcCtx)
	cleanupRefundRows(t, svcCtx, orderID)
	seedRefundOrder(t, svcCtx, orderID, 1)

	resp, err := NewRequestRefundLogic(context.Background(), svcCtx).RequestRefund(&orderpb.LifecycleOrderReq{
		OrderId:      orderID,
		OperatorId:   10001,
		OperatorRole: "user",
		Reason:       "user requested refund",
		RequestId:    "refund-req-1",
	})
	if err != nil {
		t.Fatalf("RequestRefund returned error: %v", err)
	}
	if resp.Status != 5 || resp.StatusText != "refund_requested" {
		t.Fatalf("unexpected response: %#v", resp)
	}
	if got := queryRefundOrderStatus(t, svcCtx, orderID); got != 5 {
		t.Fatalf("order status = %d, want 5", got)
	}
	assertRefundLogCount(t, svcCtx, orderID, 1, 5, 1)
	assertRefundOutboxCount(t, svcCtx, orderID, "order.refund.requested", 1)
}

func TestApproveRefund_MarksRefundFailedWhenStockRestoreFails(t *testing.T) {
	productRPC := &refundProductRPC{revertErr: errors.New("stock rpc down")}
	svcCtx, cleanup := newRefundServiceContext(t, productRPC)
	defer cleanup()

	orderID := "o-refund-failed"
	ensureRefundSchema(t, svcCtx)
	cleanupRefundRows(t, svcCtx, orderID)
	seedRefundOrder(t, svcCtx, orderID, 5)

	resp, err := NewApproveRefundLogic(context.Background(), svcCtx).ApproveRefund(&orderpb.LifecycleOrderReq{
		OrderId:      orderID,
		OperatorId:   90001,
		OperatorRole: "admin",
		Reason:       "approve refund",
		RequestId:    "refund-approve-1",
	})
	if err != nil {
		t.Fatalf("ApproveRefund should persist refund_failed instead of returning error, got %v", err)
	}
	if productRPC.revertCalls != 1 {
		t.Fatalf("RevertStock calls = %d, want 1", productRPC.revertCalls)
	}
	if resp.Status != 7 || resp.StatusText != "refund_failed" {
		t.Fatalf("unexpected response: %#v", resp)
	}
	if got := queryRefundOrderStatus(t, svcCtx, orderID); got != 7 {
		t.Fatalf("order status = %d, want 7", got)
	}
	assertRefundLogCount(t, svcCtx, orderID, 5, 7, 1)
	assertRefundOutboxCount(t, svcCtx, orderID, "order.refund.failed", 1)
}

func newRefundServiceContext(t *testing.T, productRPC productpb.Product) (*svc.ServiceContext, func()) {
	t.Helper()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis failed: %v", err)
	}
	rds := redis.MustNewRedis(redis.RedisConf{
		Host: mr.Addr(),
		Type: redis.NodeType,
	})

	return &svc.ServiceContext{
		Config: config.Config{
			DataSource:      refundTestDSN,
			StockShardCount: 4,
		},
		SqlConn:    sqlx.NewMysql(refundTestDSN),
		Redis:      rds,
		ProductRpc: productRPC,
	}, mr.Close
}

func ensureRefundSchema(t *testing.T, svcCtx *svc.ServiceContext) {
	t.Helper()

	statements := []string{
		`CREATE TABLE IF NOT EXISTS orders (
			id varchar(64) NOT NULL,
			request_id varchar(64) DEFAULT NULL,
			user_id bigint NOT NULL DEFAULT 0,
			product_id bigint NOT NULL DEFAULT 0,
			amount int NOT NULL DEFAULT 0,
			status tinyint NOT NULL DEFAULT 0,
			refund_requested_at timestamp NULL DEFAULT NULL,
			refunded_at timestamp NULL DEFAULT NULL,
			create_time timestamp NULL DEFAULT CURRENT_TIMESTAMP,
			update_time timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			PRIMARY KEY (id),
			UNIQUE KEY uniq_request_id (request_id)
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
		`CREATE TABLE IF NOT EXISTS order_status_log (
			id bigint NOT NULL AUTO_INCREMENT,
			order_id varchar(64) NOT NULL,
			from_status tinyint NOT NULL,
			to_status tinyint NOT NULL,
			operator_id bigint NOT NULL DEFAULT 0,
			remark varchar(255) NOT NULL DEFAULT '',
			create_time timestamp NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (id),
			KEY ix_order_id (order_id)
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
			UNIQUE KEY uniq_event_id (event_id),
			KEY ix_status_retry (status, next_retry_at),
			KEY ix_aggregate_id (aggregate_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}

	for _, statement := range statements {
		if _, err := svcCtx.SqlConn.ExecCtx(context.Background(), statement); err != nil {
			t.Fatalf("schema ensure failed: %v", err)
		}
	}
}

func cleanupRefundRows(t *testing.T, svcCtx *svc.ServiceContext, orderID string) {
	t.Helper()

	statements := []string{
		"DELETE FROM order_outbox WHERE aggregate_id = ?",
		"DELETE FROM order_status_log WHERE order_id = ?",
		"DELETE FROM payment_order WHERE order_id = ?",
		"DELETE FROM orders WHERE id = ?",
	}
	for _, statement := range statements {
		if _, err := svcCtx.SqlConn.ExecCtx(context.Background(), statement, orderID); err != nil {
			t.Fatalf("cleanup failed for %q: %v", statement, err)
		}
	}
}

func seedRefundOrder(t *testing.T, svcCtx *svc.ServiceContext, orderID string, status int64) {
	t.Helper()

	if _, err := svcCtx.SqlConn.ExecCtx(context.Background(),
		"INSERT INTO orders (id, request_id, user_id, product_id, amount, status) VALUES (?, ?, 10001, 100, 2, ?)",
		orderID, "req-"+orderID, status); err != nil {
		t.Fatalf("seed order failed: %v", err)
	}
	if _, err := svcCtx.SqlConn.ExecCtx(context.Background(),
		"INSERT INTO payment_order (id, order_id, user_id, payable_amount_fen, status, out_trade_no) VALUES (?, ?, 10001, 19800, 1, ?)",
		"pay:"+orderID, orderID, "mock-"+orderID); err != nil {
		t.Fatalf("seed payment failed: %v", err)
	}
}

func queryRefundOrderStatus(t *testing.T, svcCtx *svc.ServiceContext, orderID string) int64 {
	t.Helper()

	var got int64
	if err := svcCtx.SqlConn.QueryRowCtx(context.Background(), &got, "SELECT status FROM orders WHERE id = ?", orderID); err != nil {
		t.Fatalf("query order status failed: %v", err)
	}
	return got
}

func assertRefundLogCount(t *testing.T, svcCtx *svc.ServiceContext, orderID string, fromStatus, toStatus, want int64) {
	t.Helper()

	var got int64
	if err := svcCtx.SqlConn.QueryRowCtx(context.Background(), &got,
		"SELECT COUNT(*) FROM order_status_log WHERE order_id = ? AND from_status = ? AND to_status = ?",
		orderID, fromStatus, toStatus); err != nil {
		t.Fatalf("query status log failed: %v", err)
	}
	if got != want {
		t.Fatalf("status log count for %s %d->%d = %d, want %d", orderID, fromStatus, toStatus, got, want)
	}
}

func assertRefundOutboxCount(t *testing.T, svcCtx *svc.ServiceContext, orderID, eventType string, want int64) {
	t.Helper()

	var got int64
	if err := svcCtx.SqlConn.QueryRowCtx(context.Background(), &got,
		"SELECT COUNT(*) FROM order_outbox WHERE aggregate_id = ? AND event_type = ?",
		orderID, eventType); err != nil {
		t.Fatalf("query outbox failed: %v", err)
	}
	if got != want {
		t.Fatalf("outbox count for %s %s = %d, want %d", orderID, eventType, got, want)
	}
}
