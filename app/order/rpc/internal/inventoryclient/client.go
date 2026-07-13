package inventoryclient

import (
	"context"
	"errors"
	"time"

	"flash-mall/app/common/authctx"
	"flash-mall/app/common/tracectx"
	common "flash-mall/app/inventory/kitex/kitex_gen/flashmall/common"
	inventory "flash-mall/app/inventory/kitex/kitex_gen/flashmall/inventory"
	"flash-mall/app/inventory/kitex/kitex_gen/flashmall/inventory/inventoryservice"

	kitexclient "github.com/cloudwego/kitex/client"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const requestTimeout = 800 * time.Millisecond

type Client interface {
	ReserveStock(ctx context.Context, orderID string, productID int64, quantity int64) error
	ConfirmDeduct(ctx context.Context, orderID string) error
	ReleaseStock(ctx context.Context, orderID string, reason string) error
}

type KitexClient struct {
	client inventoryservice.Client
}

func NewKitexClient(endpoint string) (*KitexClient, error) {
	client, err := inventoryservice.NewClient("InventoryService", kitexclient.WithHostPorts(endpoint))
	if err != nil {
		return nil, err
	}
	return &KitexClient{client: client}, nil
}

func (c *KitexClient) ReserveStock(ctx context.Context, orderID string, productID int64, quantity int64) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	_, err := c.client.ReserveStock(ctx, &inventory.ReserveStockRequest{Meta: requestMeta(ctx), OrderId: orderID, ProductId: productID, Quantity: quantity})
	return toOrderStatusError(err)
}

func (c *KitexClient) ConfirmDeduct(ctx context.Context, orderID string) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	_, err := c.client.ConfirmDeduct(ctx, &inventory.ConfirmDeductRequest{Meta: requestMeta(ctx), OrderId: orderID})
	return toOrderStatusError(err)
}

func (c *KitexClient) ReleaseStock(ctx context.Context, orderID string, reason string) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	req := &inventory.ReleaseStockRequest{Meta: requestMeta(ctx), OrderId: orderID}
	if reason != "" {
		req.Reason = &reason
	}
	_, err := c.client.ReleaseStock(ctx, req)
	return toOrderStatusError(err)
}

func requestMeta(ctx context.Context) *common.RequestMeta {
	requestID := tracectx.RequestIDFrom(ctx)
	traceID := requestID
	if trace, ok := tracectx.FromContext(ctx); ok {
		if trace.RequestID != "" {
			requestID = trace.RequestID
		}
		if trace.TraceID != "" {
			traceID = trace.TraceID
		}
	}
	if requestID == "" {
		requestID = tracectx.NewRequestID()
	}
	if traceID == "" {
		traceID = requestID
	}
	meta := &common.RequestMeta{RequestId: &requestID, TraceId: &traceID}
	if identity, ok := authctx.IdentityFrom(ctx); ok {
		userID, merchantID, role := identity.UserID, identity.MerchantID, string(identity.Role)
		meta.UserId = &userID
		meta.MerchantId = &merchantID
		meta.Role = &role
	}
	return meta
}

func toOrderStatusError(err error) error {
	if err == nil {
		return nil
	}
	var biz *common.BizException
	if !errors.As(err, &biz) {
		return err
	}
	switch biz.GetCode() {
	case common.ErrorCode_INVALID_ARGUMENT:
		return status.Error(codes.InvalidArgument, biz.GetMessage())
	case common.ErrorCode_STOCK_NOT_FOUND:
		return status.Error(codes.FailedPrecondition, biz.GetMessage())
	case common.ErrorCode_STOCK_INSUFFICIENT:
		return status.Error(codes.Aborted, biz.GetMessage())
	default:
		return status.Error(codes.Internal, biz.GetMessage())
	}
}
