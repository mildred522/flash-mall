package logic

import (
	"context"
	"strings"
	"testing"

	"flash-mall/app/order/api/internal/svc"
	"flash-mall/app/order/api/internal/types"
	orderclient "flash-mall/app/order/rpc/orderclient"

	"github.com/alicebob/miniredis/v2"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc"
)

const payOrderLogicTestDSN = "root:6494kj06@tcp(127.0.0.1:3306)/mall_order?charset=utf8mb4&parseTime=true&loc=Local"

type payOrderRPCStub struct {
	lastReq *orderclient.MarkOrderPaidReq
}

func (s *payOrderRPCStub) PreDeduct(context.Context, *orderclient.PreDeductReq, ...grpc.CallOption) (*orderclient.Empty, error) {
	panic("unexpected PreDeduct call")
}

func (s *payOrderRPCStub) PreDeductRollback(context.Context, *orderclient.PreDeductReq, ...grpc.CallOption) (*orderclient.Empty, error) {
	panic("unexpected PreDeductRollback call")
}

func (s *payOrderRPCStub) CreateOrder(context.Context, *orderclient.CreateOrderReq, ...grpc.CallOption) (*orderclient.CreateOrderResp, error) {
	panic("unexpected CreateOrder call")
}

func (s *payOrderRPCStub) CreateOrderRollback(context.Context, *orderclient.CreateOrderReq, ...grpc.CallOption) (*orderclient.Empty, error) {
	panic("unexpected CreateOrderRollback call")
}

func (s *payOrderRPCStub) MarkOrderPaid(_ context.Context, in *orderclient.MarkOrderPaidReq, _ ...grpc.CallOption) (*orderclient.MarkOrderPaidResp, error) {
	s.lastReq = in
	return &orderclient.MarkOrderPaidResp{Updated: true, OrderStatus: "PAID"}, nil
}

func (s *payOrderRPCStub) GetOrderDetail(context.Context, *orderclient.GetOrderDetailReq, ...grpc.CallOption) (*orderclient.GetOrderDetailResp, error) {
	panic("unexpected GetOrderDetail call")
}

func (s *payOrderRPCStub) RequestRefund(context.Context, *orderclient.LifecycleOrderReq, ...grpc.CallOption) (*orderclient.LifecycleOrderResp, error) {
	panic("unexpected RequestRefund call")
}

func (s *payOrderRPCStub) ApproveRefund(context.Context, *orderclient.LifecycleOrderReq, ...grpc.CallOption) (*orderclient.LifecycleOrderResp, error) {
	panic("unexpected ApproveRefund call")
}

func TestPayOrderLogic_UsesMarkOrderPaidForMockPayment(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis failed: %v", err)
	}
	defer mr.Close()

	sqlConn := sqlx.NewMysql(payOrderLogicTestDSN)
	orderID := "o-pay-rpc"
	ensurePayOrderLogicSchema(t, sqlConn)
	cleanupPayOrderLogicRows(t, sqlConn, orderID)
	seedPayOrderLogicOrder(t, sqlConn, orderID)

	orderRPC := &payOrderRPCStub{}
	l := NewPayOrderLogic(context.Background(), &svc.ServiceContext{
		SqlConn:  sqlConn,
		OrderRpc: orderRPC,
		Redis: redis.MustNewRedis(redis.RedisConf{
			Host: mr.Addr(),
			Type: redis.NodeType,
		}),
	})

	resp, err := l.PayOrder(&types.PayOrderReq{OrderId: orderID}, 10001)
	if err != nil {
		t.Fatalf("PayOrder returned error: %v", err)
	}
	if resp.Status != "paid" {
		t.Fatalf("unexpected response: %#v", resp)
	}
	if orderRPC.lastReq == nil {
		t.Fatal("expected PayOrder to delegate to OrderRpc.MarkOrderPaid")
	}
	if orderRPC.lastReq.PaymentOrderId != "pay:"+orderID || orderRPC.lastReq.OutTradeNo != "mock-"+orderID {
		t.Fatalf("unexpected MarkOrderPaid request: %#v", orderRPC.lastReq)
	}
	if !strings.Contains(orderRPC.lastReq.CallbackBody, `"paid_amount_fen":19800`) {
		t.Fatalf("callback body missing amount: %s", orderRPC.lastReq.CallbackBody)
	}
}

func ensurePayOrderLogicSchema(t *testing.T, sqlConn sqlx.SqlConn) {
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
		if _, err := sqlConn.ExecCtx(context.Background(), statement); err != nil {
			t.Fatalf("schema ensure failed: %v", err)
		}
	}
}

func cleanupPayOrderLogicRows(t *testing.T, sqlConn sqlx.SqlConn, orderID string) {
	t.Helper()

	if _, err := sqlConn.ExecCtx(context.Background(), "DELETE FROM payment_order WHERE order_id = ?", orderID); err != nil {
		t.Fatalf("payment cleanup failed: %v", err)
	}
	if _, err := sqlConn.ExecCtx(context.Background(), "DELETE FROM orders WHERE id = ?", orderID); err != nil {
		t.Fatalf("order cleanup failed: %v", err)
	}
}

func seedPayOrderLogicOrder(t *testing.T, sqlConn sqlx.SqlConn, orderID string) {
	t.Helper()

	if _, err := sqlConn.ExecCtx(context.Background(),
		"INSERT INTO orders (id, request_id, user_id, product_id, amount, status) VALUES (?, ?, 10001, 100, 2, 0)",
		orderID, "req-"+orderID); err != nil {
		t.Fatalf("seed order failed: %v", err)
	}
	if _, err := sqlConn.ExecCtx(context.Background(),
		"INSERT INTO payment_order (id, order_id, user_id, payable_amount_fen, status, out_trade_no) VALUES (?, ?, 10001, 19800, 0, ?)",
		"pay:"+orderID, orderID, "mock-"+orderID); err != nil {
		t.Fatalf("seed payment failed: %v", err)
	}
}
