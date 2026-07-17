package orderrpc

import (
	"context"
	"testing"

	"flash-mall/app/gateway/hertz/internal/ports"
	orderpb "flash-mall/app/order/rpc/order"

	"google.golang.org/grpc"
)

type orderCommandRPCStub struct {
	cancel   *orderpb.CancelUserOrderReq
	close    *orderpb.CloseAdminOrderReq
	ship     *orderpb.ShipAdminOrderReq
	merchant *orderpb.ShipMerchantOrderReq
	confirm  *orderpb.ConfirmReceiptReq
}

func (s *orderCommandRPCStub) CancelUserOrder(_ context.Context, in *orderpb.CancelUserOrderReq, _ ...grpc.CallOption) (*orderpb.OrderCommandResp, error) {
	s.cancel = in
	return &orderpb.OrderCommandResp{}, nil
}
func (s *orderCommandRPCStub) CloseAdminOrder(_ context.Context, in *orderpb.CloseAdminOrderReq, _ ...grpc.CallOption) (*orderpb.OrderCommandResp, error) {
	s.close = in
	return &orderpb.OrderCommandResp{}, nil
}
func (s *orderCommandRPCStub) ShipAdminOrder(_ context.Context, in *orderpb.ShipAdminOrderReq, _ ...grpc.CallOption) (*orderpb.OrderCommandResp, error) {
	s.ship = in
	return &orderpb.OrderCommandResp{}, nil
}
func (s *orderCommandRPCStub) ShipMerchantOrder(_ context.Context, in *orderpb.ShipMerchantOrderReq, _ ...grpc.CallOption) (*orderpb.OrderCommandResp, error) {
	s.merchant = in
	return &orderpb.OrderCommandResp{}, nil
}
func (s *orderCommandRPCStub) ConfirmReceipt(_ context.Context, in *orderpb.ConfirmReceiptReq, _ ...grpc.CallOption) (*orderpb.OrderCommandResp, error) {
	s.confirm = in
	return &orderpb.OrderCommandResp{}, nil
}

func TestAdapterMapsAllOrderCommandsAndRequestMetadata(t *testing.T) {
	stub := &orderCommandRPCStub{}
	adapter := New(stub)
	meta := ports.RequestMeta{RequestID: "req-1", TraceID: "trace-1", UserID: 9, Role: "user"}
	ctx := context.Background()

	if err := adapter.CancelUser(ctx, ports.CancelUserOrderCommand{OrderID: "o1", Reason: "cancel", UserID: 9, Meta: meta}); err != nil {
		t.Fatal(err)
	}
	if err := adapter.CloseAdmin(ctx, ports.CloseAdminOrderCommand{OrderID: "o2", Reason: "close", OperatorID: 7, Meta: meta}); err != nil {
		t.Fatal(err)
	}
	if err := adapter.ShipAdmin(ctx, ports.ShipAdminOrderCommand{OrderID: "o3", OperatorID: 7, Meta: meta}); err != nil {
		t.Fatal(err)
	}
	if err := adapter.ShipMerchant(ctx, ports.ShipMerchantOrderCommand{OrderID: "o4", MerchantID: 88, Meta: meta}); err != nil {
		t.Fatal(err)
	}
	if err := adapter.ConfirmReceipt(ctx, ports.ConfirmReceiptCommand{OrderID: "o5", UserID: 9, Meta: meta}); err != nil {
		t.Fatal(err)
	}

	if stub.cancel.GetOrderId() != "o1" || stub.cancel.GetUserId() != 9 || stub.cancel.GetMeta().GetRequestId() != "req-1" {
		t.Fatalf("cancel mapping: %#v", stub.cancel)
	}
	if stub.close.GetOperatorId() != 7 || stub.ship.GetOperatorId() != 7 {
		t.Fatalf("admin mapping: close=%#v ship=%#v", stub.close, stub.ship)
	}
	if stub.merchant.GetMerchantId() != 88 || stub.merchant.GetMeta().GetMerchantId() != 88 || stub.merchant.GetMeta().GetRole() != "merchant" {
		t.Fatalf("merchant mapping: %#v", stub.merchant)
	}
	if stub.confirm.GetUserId() != 9 || stub.confirm.GetMeta().GetTraceId() != "trace-1" {
		t.Fatalf("confirm mapping: %#v", stub.confirm)
	}
}
