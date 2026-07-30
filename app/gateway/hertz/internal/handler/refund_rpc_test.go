package handler

import (
	"context"
	"testing"

	"flash-mall/app/gateway/hertz/internal/svc"
	orderclient "flash-mall/app/order/rpc/orderclient"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type refundOrderRPCStub struct {
	requestReq      *orderclient.RequestRefundReq
	auditReq        *orderclient.AuditRefundReq
	notificationReq *orderclient.HandlePaymentNotificationReq
}

func (*refundOrderRPCStub) PreDeduct(context.Context, *orderclient.PreDeductReq, ...grpc.CallOption) (*orderclient.Empty, error) {
	panic("unexpected PreDeduct call")
}

func (*refundOrderRPCStub) PreDeductRollback(context.Context, *orderclient.PreDeductReq, ...grpc.CallOption) (*orderclient.Empty, error) {
	panic("unexpected PreDeductRollback call")
}

func (*refundOrderRPCStub) CreateOrder(context.Context, *orderclient.CreateOrderReq, ...grpc.CallOption) (*orderclient.CreateOrderResp, error) {
	panic("unexpected CreateOrder call")
}

func (*refundOrderRPCStub) CreateOrderRollback(context.Context, *orderclient.CreateOrderReq, ...grpc.CallOption) (*orderclient.Empty, error) {
	panic("unexpected CreateOrderRollback call")
}

func (*refundOrderRPCStub) CreatePayment(context.Context, *orderclient.CreatePaymentReq, ...grpc.CallOption) (*orderclient.CreatePaymentResp, error) {
	panic("unexpected CreatePayment call")
}

func (s *refundOrderRPCStub) HandlePaymentNotification(_ context.Context, in *orderclient.HandlePaymentNotificationReq, _ ...grpc.CallOption) (*orderclient.HandlePaymentNotificationResp, error) {
	s.notificationReq = in
	return &orderclient.HandlePaymentNotificationResp{Accepted: true, OrderStatus: "PAID"}, nil
}

func (*refundOrderRPCStub) MarkOrderPaid(context.Context, *orderclient.MarkOrderPaidReq, ...grpc.CallOption) (*orderclient.MarkOrderPaidResp, error) {
	panic("unexpected MarkOrderPaid call")
}

func (*refundOrderRPCStub) GetOrderDetail(context.Context, *orderclient.GetOrderDetailReq, ...grpc.CallOption) (*orderclient.GetOrderDetailResp, error) {
	panic("unexpected GetOrderDetail call")
}

func (s *refundOrderRPCStub) RequestRefund(_ context.Context, in *orderclient.RequestRefundReq, _ ...grpc.CallOption) (*orderclient.RequestRefundResp, error) {
	s.requestReq = in
	return &orderclient.RequestRefundResp{RefundId: "rf:" + in.OrderId, OrderId: in.OrderId, OrderStatus: 5}, nil
}

func (s *refundOrderRPCStub) AuditRefund(_ context.Context, in *orderclient.AuditRefundReq, _ ...grpc.CallOption) (*orderclient.AuditRefundResp, error) {
	s.auditReq = in
	return &orderclient.AuditRefundResp{RefundId: in.RefundId, OrderId: "order-1", OrderStatus: 6, RefundStatus: 2}, nil
}

func (*refundOrderRPCStub) CancelUserOrder(context.Context, *orderclient.CancelUserOrderReq, ...grpc.CallOption) (*orderclient.OrderCommandResp, error) {
	panic("unexpected CancelUserOrder call")
}
func (*refundOrderRPCStub) CloseAdminOrder(context.Context, *orderclient.CloseAdminOrderReq, ...grpc.CallOption) (*orderclient.OrderCommandResp, error) {
	panic("unexpected CloseAdminOrder call")
}
func (*refundOrderRPCStub) ShipAdminOrder(context.Context, *orderclient.ShipAdminOrderReq, ...grpc.CallOption) (*orderclient.OrderCommandResp, error) {
	panic("unexpected ShipAdminOrder call")
}
func (*refundOrderRPCStub) ShipMerchantOrder(context.Context, *orderclient.ShipMerchantOrderReq, ...grpc.CallOption) (*orderclient.OrderCommandResp, error) {
	panic("unexpected ShipMerchantOrder call")
}
func (*refundOrderRPCStub) ConfirmReceipt(context.Context, *orderclient.ConfirmReceiptReq, ...grpc.CallOption) (*orderclient.OrderCommandResp, error) {
	panic("unexpected ConfirmReceipt call")
}

func TestRequestUserRefundDelegatesToOrderRPC(t *testing.T) {
	stub := &refundOrderRPCStub{}
	svcCtx := &svc.ServiceContext{OrderRpc: stub}
	err := requestUserRefund(context.Background(), svcCtx, RefundOrderReq{OrderID: "order-1", Reason: "changed mind"}, 7001)
	if err != nil {
		t.Fatalf("request user refund: %v", err)
	}
	if stub.requestReq == nil || stub.requestReq.OrderId != "order-1" || stub.requestReq.RequesterId != 7001 || stub.requestReq.RequesterRole != "user" {
		t.Fatalf("unexpected request refund RPC input: %#v", stub.requestReq)
	}
}

func TestAuditAdminRefundDelegatesToOrderRPC(t *testing.T) {
	stub := &refundOrderRPCStub{}
	svcCtx := &svc.ServiceContext{OrderRpc: stub}
	statusText, err := auditAdminRefund(context.Background(), svcCtx, AdminRefundAuditReq{
		RefundID: "rf:order-1", Approve: true, Remark: "approved",
	}, 9001)
	if err != nil || statusText != "success" {
		t.Fatalf("audit admin refund: status=%q err=%v", statusText, err)
	}
	if stub.auditReq == nil || stub.auditReq.RefundId != "rf:order-1" || stub.auditReq.OperatorId != 9001 || !stub.auditReq.Approve {
		t.Fatalf("unexpected audit refund RPC input: %#v", stub.auditReq)
	}
}

func TestRefundAdminOrderUsesRequestThenAuditRPC(t *testing.T) {
	stub := &refundOrderRPCStub{}
	svcCtx := &svc.ServiceContext{OrderRpc: stub}
	err := refundAdminOrder(context.Background(), svcCtx, RefundOrderReq{OrderID: "order-1", Reason: "admin refund"}, 9001)
	if err != nil {
		t.Fatalf("refund admin order: %v", err)
	}
	if stub.requestReq == nil || stub.requestReq.RequesterRole != "admin" || stub.requestReq.RequesterId != 9001 {
		t.Fatalf("unexpected admin request refund input: %#v", stub.requestReq)
	}
	if stub.auditReq == nil || stub.auditReq.RefundId != "rf:order-1" || !stub.auditReq.Approve {
		t.Fatalf("unexpected admin audit refund input: %#v", stub.auditReq)
	}
}

func TestCreateOrderStatusCodeMapsGRPCPermissionDenied(t *testing.T) {
	if got := createOrderStatusCode(status.Error(codes.PermissionDenied, "wrong owner")); got != 403 {
		t.Fatalf("permission denied status=%d, want 403", got)
	}
}
