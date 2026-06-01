package job

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"flash-mall/app/order/api/internal/model"
	"flash-mall/app/order/api/internal/svc"
	orderclient "flash-mall/app/order/rpc/orderclient"
	productclient "flash-mall/app/product/rpc/productclient"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc"
)

const closeOrderTestDSN = "root:6494kj06@tcp(127.0.0.1:3306)/mall_order?charset=utf8mb4&parseTime=true&loc=Local"

type closeOrderModel struct {
	order     *model.Orders
	updateHit bool
}

func (m *closeOrderModel) Insert(context.Context, *model.Orders) (sql.Result, error) {
	panic("unexpected Insert call")
}

func (m *closeOrderModel) FindOne(context.Context, string) (*model.Orders, error) {
	return m.order, nil
}

func (m *closeOrderModel) Update(context.Context, *model.Orders) error {
	m.updateHit = true
	return nil
}

func (m *closeOrderModel) Delete(context.Context, string) error {
	panic("unexpected Delete call")
}

func (m *closeOrderModel) FindOneByRequestId(context.Context, string) (*model.Orders, error) {
	panic("unexpected FindOneByRequestId call")
}

type closeOrderProductRPC struct {
	revertCalls int
}

func (p *closeOrderProductRPC) GetProductCard(context.Context, *productclient.GetProductCardReq, ...grpc.CallOption) (*productclient.GetProductCardResp, error) {
	panic("unexpected GetProductCard call")
}

func (p *closeOrderProductRPC) Deduct(context.Context, *productclient.DeductReq, ...grpc.CallOption) (*productclient.Empty, error) {
	panic("unexpected Deduct call")
}

func (p *closeOrderProductRPC) DeductRollback(context.Context, *productclient.DeductReq, ...grpc.CallOption) (*productclient.Empty, error) {
	panic("unexpected DeductRollback call")
}

func (p *closeOrderProductRPC) RevertStock(context.Context, *productclient.RevertStockReq, ...grpc.CallOption) (*productclient.RevertStockResp, error) {
	p.revertCalls++
	return &productclient.RevertStockResp{}, nil
}

type closeOrderOrderRPC struct {
	rollbackCalls int
}

func (o *closeOrderOrderRPC) PreDeduct(context.Context, *orderclient.PreDeductReq, ...grpc.CallOption) (*orderclient.Empty, error) {
	panic("unexpected PreDeduct call")
}

func (o *closeOrderOrderRPC) PreDeductRollback(context.Context, *orderclient.PreDeductReq, ...grpc.CallOption) (*orderclient.Empty, error) {
	o.rollbackCalls++
	return &orderclient.Empty{}, nil
}

func (o *closeOrderOrderRPC) CreateOrder(context.Context, *orderclient.CreateOrderReq, ...grpc.CallOption) (*orderclient.CreateOrderResp, error) {
	panic("unexpected CreateOrder call")
}

func (o *closeOrderOrderRPC) CreateOrderRollback(context.Context, *orderclient.CreateOrderReq, ...grpc.CallOption) (*orderclient.Empty, error) {
	panic("unexpected CreateOrderRollback call")
}

func (o *closeOrderOrderRPC) MarkOrderPaid(context.Context, *orderclient.MarkOrderPaidReq, ...grpc.CallOption) (*orderclient.MarkOrderPaidResp, error) {
	panic("unexpected MarkOrderPaid call")
}

func (o *closeOrderOrderRPC) GetOrderDetail(context.Context, *orderclient.GetOrderDetailReq, ...grpc.CallOption) (*orderclient.GetOrderDetailResp, error) {
	panic("unexpected GetOrderDetail call")
}

func TestCloseOrderJob_HandleCloseOrder_SkipsWhenPaymentWinsRace(t *testing.T) {
	sqlConn := sqlx.NewMysql(closeOrderTestDSN)
	orderID := "o-close-race"
	ensureCloseOrderSchema(t, sqlConn)
	cleanupCloseOrderRows(t, sqlConn, orderID)
	seedCloseOrderStatus(t, sqlConn, orderID, 1)

	orderModel := &closeOrderModel{
		order: &model.Orders{
			Id:        orderID,
			RequestId: "req-" + orderID,
			UserId:    1,
			ProductId: 100,
			Amount:    2,
			Status:    0,
		},
	}
	productRPC := &closeOrderProductRPC{}
	orderRPC := &closeOrderOrderRPC{}
	job := NewCloseOrderJob(&svc.ServiceContext{
		SqlConn:    sqlConn,
		OrderModel: orderModel,
		ProductRpc: productRPC,
		OrderRpc:   orderRPC,
	})

	if err := job.handleCloseOrder(orderID); err != nil {
		t.Fatalf("paid race should be skipped, got %v", err)
	}
	if productRPC.revertCalls != 0 || orderRPC.rollbackCalls != 0 || orderModel.updateHit {
		t.Fatalf("expected no release/update after payment race, product=%d rollback=%d update=%v", productRPC.revertCalls, orderRPC.rollbackCalls, orderModel.updateHit)
	}
}

func ensureCloseOrderSchema(t *testing.T, sqlConn sqlx.SqlConn) {
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

func cleanupCloseOrderRows(t *testing.T, sqlConn sqlx.SqlConn, orderID string) {
	t.Helper()

	if _, err := sqlConn.ExecCtx(context.Background(), fmt.Sprintf("DELETE FROM orders WHERE id = '%s'", orderID)); err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}
}

func seedCloseOrderStatus(t *testing.T, sqlConn sqlx.SqlConn, orderID string, status int64) {
	t.Helper()

	statement := fmt.Sprintf("INSERT INTO orders (id, request_id, user_id, product_id, amount, status) VALUES ('%s', 'req-%s', 1, 100, 2, %d)", orderID, orderID, status)
	if _, err := sqlConn.ExecCtx(context.Background(), statement); err != nil {
		t.Fatalf("seed failed: %v", err)
	}
}
