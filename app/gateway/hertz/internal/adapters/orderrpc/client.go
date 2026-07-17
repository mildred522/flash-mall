package orderrpc

import (
	"context"

	"flash-mall/app/gateway/hertz/internal/ports"
	orderpb "flash-mall/app/order/rpc/order"

	"google.golang.org/grpc"
)

type commandClient interface {
	CancelUserOrder(context.Context, *orderpb.CancelUserOrderReq, ...grpc.CallOption) (*orderpb.OrderCommandResp, error)
	CloseAdminOrder(context.Context, *orderpb.CloseAdminOrderReq, ...grpc.CallOption) (*orderpb.OrderCommandResp, error)
	ShipAdminOrder(context.Context, *orderpb.ShipAdminOrderReq, ...grpc.CallOption) (*orderpb.OrderCommandResp, error)
	ShipMerchantOrder(context.Context, *orderpb.ShipMerchantOrderReq, ...grpc.CallOption) (*orderpb.OrderCommandResp, error)
	ConfirmReceipt(context.Context, *orderpb.ConfirmReceiptReq, ...grpc.CallOption) (*orderpb.OrderCommandResp, error)
}

type Adapter struct {
	client commandClient
}

var _ ports.OrderCommands = (*Adapter)(nil)

func New(client commandClient) *Adapter {
	return &Adapter{client: client}
}

func (a *Adapter) CancelUser(ctx context.Context, command ports.CancelUserOrderCommand) error {
	_, err := a.client.CancelUserOrder(ctx, &orderpb.CancelUserOrderReq{
		OrderId: command.OrderID, Reason: command.Reason, UserId: command.UserID,
		Meta: requestMetaFor(command.Meta, command.UserID, 0, "user"),
	})
	return err
}

func (a *Adapter) CloseAdmin(ctx context.Context, command ports.CloseAdminOrderCommand) error {
	_, err := a.client.CloseAdminOrder(ctx, &orderpb.CloseAdminOrderReq{
		OrderId: command.OrderID, Reason: command.Reason, OperatorId: command.OperatorID,
		Meta: requestMetaFor(command.Meta, command.OperatorID, 0, "admin"),
	})
	return err
}

func (a *Adapter) ShipAdmin(ctx context.Context, command ports.ShipAdminOrderCommand) error {
	_, err := a.client.ShipAdminOrder(ctx, &orderpb.ShipAdminOrderReq{
		OrderId: command.OrderID, OperatorId: command.OperatorID,
		Meta: requestMetaFor(command.Meta, command.OperatorID, 0, "admin"),
	})
	return err
}

func (a *Adapter) ShipMerchant(ctx context.Context, command ports.ShipMerchantOrderCommand) error {
	_, err := a.client.ShipMerchantOrder(ctx, &orderpb.ShipMerchantOrderReq{
		OrderId: command.OrderID, MerchantId: command.MerchantID,
		Meta: requestMetaFor(command.Meta, 0, command.MerchantID, "merchant"),
	})
	return err
}

func (a *Adapter) ConfirmReceipt(ctx context.Context, command ports.ConfirmReceiptCommand) error {
	_, err := a.client.ConfirmReceipt(ctx, &orderpb.ConfirmReceiptReq{
		OrderId: command.OrderID, UserId: command.UserID,
		Meta: requestMetaFor(command.Meta, command.UserID, 0, "user"),
	})
	return err
}

func requestMetaFor(meta ports.RequestMeta, userID, merchantID int64, role string) *orderpb.OrderCommandMeta {
	result := &orderpb.OrderCommandMeta{
		RequestId: meta.RequestID, TraceId: meta.TraceID, UserId: meta.UserID,
		MerchantId: meta.MerchantID, Role: meta.Role,
	}
	if userID > 0 {
		result.UserId = userID
	}
	if merchantID > 0 {
		result.MerchantId = merchantID
	}
	if role != "" {
		result.Role = role
	}
	return result
}
