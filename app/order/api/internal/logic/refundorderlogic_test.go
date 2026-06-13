package logic

import (
	"context"
	"testing"

	"flash-mall/app/order/api/internal/svc"
	"flash-mall/app/order/api/internal/types"
	orderclient "flash-mall/app/order/rpc/orderclient"
	productclient "flash-mall/app/product/rpc/productclient"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc"
)

const refundOrderLogicTestDSN = "root:6494kj06@tcp(127.0.0.1:3306)/mall_order?charset=utf8mb4&parseTime=true&loc=Local"

type refundOrderRPCStub struct {
	requestRefundReq *orderclient.LifecycleOrderReq
	rollbackCalls    int
}

func (s *refundOrderRPCStub) PreDeduct(context.Context, *orderclient.PreDeductReq, ...grpc.CallOption) (*orderclient.Empty, error) {
	panic("unexpected PreDeduct call")
}

func (s *refundOrderRPCStub) PreDeductRollback(context.Context, *orderclient.PreDeductReq, ...grpc.CallOption) (*orderclient.Empty, error) {
	s.rollbackCalls++
	return &orderclient.Empty{}, nil
}

func (s *refundOrderRPCStub) CreateOrder(context.Context, *orderclient.CreateOrderReq, ...grpc.CallOption) (*orderclient.CreateOrderResp, error) {
	panic("unexpected CreateOrder call")
}

func (s *refundOrderRPCStub) CreateOrderRollback(context.Context, *orderclient.CreateOrderReq, ...grpc.CallOption) (*orderclient.Empty, error) {
	panic("unexpected CreateOrderRollback call")
}

func (s *refundOrderRPCStub) MarkOrderPaid(context.Context, *orderclient.MarkOrderPaidReq, ...grpc.CallOption) (*orderclient.MarkOrderPaidResp, error) {
	panic("unexpected MarkOrderPaid call")
}

func (s *refundOrderRPCStub) GetOrderDetail(context.Context, *orderclient.GetOrderDetailReq, ...grpc.CallOption) (*orderclient.GetOrderDetailResp, error) {
	panic("unexpected GetOrderDetail call")
}

func (s *refundOrderRPCStub) RequestRefund(_ context.Context, in *orderclient.LifecycleOrderReq, _ ...grpc.CallOption) (*orderclient.LifecycleOrderResp, error) {
	s.requestRefundReq = in
	return &orderclient.LifecycleOrderResp{OrderId: in.OrderId, Status: 5, StatusText: "refund_requested"}, nil
}

func (s *refundOrderRPCStub) ApproveRefund(context.Context, *orderclient.LifecycleOrderReq, ...grpc.CallOption) (*orderclient.LifecycleOrderResp, error) {
	panic("unexpected ApproveRefund call")
}

type refundProductRPCStub struct{}

func (s *refundProductRPCStub) GetProductCard(context.Context, *productclient.GetProductCardReq, ...grpc.CallOption) (*productclient.GetProductCardResp, error) {
	panic("unexpected GetProductCard call")
}

func (s *refundProductRPCStub) ListProducts(context.Context, *productclient.ListProductsReq, ...grpc.CallOption) (*productclient.ListProductsResp, error) {
	panic("unexpected ListProducts call")
}

func (s *refundProductRPCStub) Deduct(context.Context, *productclient.DeductReq, ...grpc.CallOption) (*productclient.Empty, error) {
	panic("unexpected Deduct call")
}

func (s *refundProductRPCStub) DeductRollback(context.Context, *productclient.DeductReq, ...grpc.CallOption) (*productclient.Empty, error) {
	panic("unexpected DeductRollback call")
}

func (s *refundProductRPCStub) RevertStock(context.Context, *productclient.RevertStockReq, ...grpc.CallOption) (*productclient.RevertStockResp, error) {
	return &productclient.RevertStockResp{}, nil
}

func TestRefundOrderLogic_DelegatesToRequestRefund(t *testing.T) {
	sqlConn := sqlx.NewMysql(refundOrderLogicTestDSN)
	orderID := "o-refund-api"
	ensureRefundOrderLogicSchema(t, sqlConn)
	cleanupRefundOrderLogicRows(t, sqlConn, orderID)
	seedRefundOrderLogicOrder(t, sqlConn, orderID)

	orderRPC := &refundOrderRPCStub{}
	l := NewRefundOrderLogic(context.Background(), &svc.ServiceContext{
		SqlConn:    sqlConn,
		OrderRpc:   orderRPC,
		ProductRpc: &refundProductRPCStub{},
	})

	resp, err := l.RefundOrder(&types.RefundOrderReq{OrderId: orderID, Reason: "changed mind"}, 10001)
	if err != nil {
		t.Fatalf("RefundOrder returned error: %v", err)
	}
	if resp.Status != "refund_requested" {
		t.Fatalf("unexpected response: %#v", resp)
	}
	if orderRPC.requestRefundReq == nil {
		t.Fatal("expected RefundOrder to delegate to OrderRpc.RequestRefund")
	}
	if orderRPC.requestRefundReq.OperatorId != 10001 || orderRPC.requestRefundReq.OperatorRole != "user" {
		t.Fatalf("unexpected RequestRefund request: %#v", orderRPC.requestRefundReq)
	}
	if orderRPC.rollbackCalls != 0 {
		t.Fatalf("unexpected direct PreDeductRollback calls: %d", orderRPC.rollbackCalls)
	}
}

func ensureRefundOrderLogicSchema(t *testing.T, sqlConn sqlx.SqlConn) {
	t.Helper()

	statement := `CREATE TABLE IF NOT EXISTS orders (
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
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`
	if _, err := sqlConn.ExecCtx(context.Background(), statement); err != nil {
		t.Fatalf("schema ensure failed: %v", err)
	}
}

func cleanupRefundOrderLogicRows(t *testing.T, sqlConn sqlx.SqlConn, orderID string) {
	t.Helper()

	if _, err := sqlConn.ExecCtx(context.Background(), "DELETE FROM orders WHERE id = ?", orderID); err != nil {
		t.Fatalf("order cleanup failed: %v", err)
	}
}

func seedRefundOrderLogicOrder(t *testing.T, sqlConn sqlx.SqlConn, orderID string) {
	t.Helper()

	if _, err := sqlConn.ExecCtx(context.Background(),
		"INSERT INTO orders (id, request_id, user_id, product_id, amount, status) VALUES (?, ?, 10001, 100, 2, 1)",
		orderID, "req-"+orderID); err != nil {
		t.Fatalf("seed order failed: %v", err)
	}
}
