package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"flash-mall/app/order/api/internal/svc"
	orderclient "flash-mall/app/order/rpc/orderclient"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc"
)

const adminOrderHandlerTestDSN = "root:6494kj06@tcp(127.0.0.1:3306)/mall_order?charset=utf8mb4&parseTime=true&loc=Local"

func (s *stubOrderRPC) RequestRefund(context.Context, *orderclient.LifecycleOrderReq, ...grpc.CallOption) (*orderclient.LifecycleOrderResp, error) {
	panic("unexpected RequestRefund call")
}

func (s *stubOrderRPC) ApproveRefund(_ context.Context, in *orderclient.LifecycleOrderReq, _ ...grpc.CallOption) (*orderclient.LifecycleOrderResp, error) {
	s.approveRefundReq = in
	if s.approveRefundResp != nil {
		return s.approveRefundResp, nil
	}
	return &orderclient.LifecycleOrderResp{OrderId: in.OrderId, Status: 6, StatusText: "refunded"}, nil
}

func TestAdminRefundOrderHandler_DelegatesToApproveRefund(t *testing.T) {
	sqlConn := sqlx.NewMysql(adminOrderHandlerTestDSN)
	orderID := "o-admin-refund"
	ensureAdminOrderHandlerSchema(t, sqlConn)
	cleanupAdminOrderHandlerRows(t, sqlConn, orderID)
	seedAdminOrderHandlerOrder(t, sqlConn, orderID, 5)

	orderRPC := &stubOrderRPC{
		approveRefundResp: &orderclient.LifecycleOrderResp{
			OrderId:    orderID,
			Status:     6,
			StatusText: "refunded",
		},
	}
	svcCtx := &svc.ServiceContext{SqlConn: sqlConn, OrderRpc: orderRPC}

	req := httptest.NewRequest(http.MethodPost, "/api/admin/orders/refund", bytes.NewBufferString(`{"order_id":"o-admin-refund","reason":"approved"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	AdminRefundOrderHandler(svcCtx).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	if orderRPC.approveRefundReq == nil {
		t.Fatal("expected AdminRefundOrderHandler to delegate to OrderRpc.ApproveRefund")
	}
	if orderRPC.approveRefundReq.OrderId != orderID || orderRPC.approveRefundReq.OperatorRole != "admin" {
		t.Fatalf("unexpected approve request: %#v", orderRPC.approveRefundReq)
	}
	if !strings.Contains(rec.Body.String(), `"status":"refunded"`) {
		t.Fatalf("unexpected response body: %s", rec.Body.String())
	}
}

func ensureAdminOrderHandlerSchema(t *testing.T, sqlConn sqlx.SqlConn) {
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

func cleanupAdminOrderHandlerRows(t *testing.T, sqlConn sqlx.SqlConn, orderID string) {
	t.Helper()

	if _, err := sqlConn.ExecCtx(context.Background(), "DELETE FROM orders WHERE id = ?", orderID); err != nil {
		t.Fatalf("order cleanup failed: %v", err)
	}
}

func seedAdminOrderHandlerOrder(t *testing.T, sqlConn sqlx.SqlConn, orderID string, status int64) {
	t.Helper()

	if _, err := sqlConn.ExecCtx(context.Background(),
		"INSERT INTO orders (id, request_id, user_id, product_id, amount, status) VALUES (?, ?, 10001, 100, 2, ?)",
		orderID, "req-"+orderID, status); err != nil {
		t.Fatalf("seed order failed: %v", err)
	}
}
