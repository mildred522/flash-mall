package inventoryclient

import (
	"context"
	"strconv"
	"strings"

	"flash-mall/app/common/authctx"
	"flash-mall/app/common/tracectx"
	common "flash-mall/app/inventory/kitex/kitex_gen/flashmall/common"
	inventory "flash-mall/app/inventory/kitex/kitex_gen/flashmall/inventory"
	"flash-mall/app/inventory/kitex/kitex_gen/flashmall/inventory/inventoryservice"

	kitexclient "github.com/cloudwego/kitex/client"
)

type Client interface {
	SeedStock(ctx context.Context, productID int64, total int64, shardCount int) error
	AdjustStock(ctx context.Context, productID int64, delta int64, bucketIdx int, reason string) (*inventory.AdjustStockResponse, error)
	ConfirmDeduct(ctx context.Context, orderID string) error
	ReconcileStock(ctx context.Context, productID int64) (*inventory.ReconcileStockResponse, error)
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

func (c *KitexClient) SeedStock(ctx context.Context, productID int64, total int64, shardCount int) error {
	var shardCountPtr *int32
	if shardCount > 0 {
		value := int32(shardCount)
		shardCountPtr = &value
	}
	_, err := c.client.SeedStock(ctx, &inventory.SeedStockRequest{Meta: requestMeta(ctx), ProductId: productID, Total: total, ShardCount: shardCountPtr})
	return err
}

func (c *KitexClient) AdjustStock(ctx context.Context, productID int64, delta int64, bucketIdx int, reason string) (*inventory.AdjustStockResponse, error) {
	bucketIdxValue := int32(bucketIdx)
	var reasonPtr *string
	if reason != "" {
		reasonPtr = &reason
	}
	return c.client.AdjustStock(ctx, &inventory.AdjustStockRequest{Meta: requestMeta(ctx), ProductId: productID, Delta: delta, BucketIdx: &bucketIdxValue, Reason: reasonPtr})
}

func (c *KitexClient) ConfirmDeduct(ctx context.Context, orderID string) error {
	_, err := c.client.ConfirmDeduct(ctx, &inventory.ConfirmDeductRequest{Meta: requestMeta(ctx), OrderId: orderID})
	return err
}

func (c *KitexClient) ReconcileStock(ctx context.Context, productID int64) (*inventory.ReconcileStockResponse, error) {
	return c.client.ReconcileStock(ctx, &inventory.ReconcileStockRequest{Meta: requestMeta(ctx), ProductId: productID})
}

func requestMeta(ctx context.Context) *common.RequestMeta {
	meta := &common.RequestMeta{}
	if ctx == nil {
		return meta
	}
	if trace, ok := tracectx.FromContext(ctx); ok {
		if trace.RequestID != "" {
			meta.RequestId = &trace.RequestID
		}
		if trace.TraceID != "" {
			meta.TraceId = &trace.TraceID
		}
		if userID, err := strconv.ParseInt(strings.TrimSpace(trace.UserID), 10, 64); err == nil && userID > 0 {
			meta.UserId = &userID
		}
		if merchantID, err := strconv.ParseInt(strings.TrimSpace(trace.MerchantID), 10, 64); err == nil && merchantID > 0 {
			meta.MerchantId = &merchantID
		}
	}
	if identity, ok := authctx.IdentityFrom(ctx); ok {
		if identity.RequestID != "" {
			meta.RequestId = &identity.RequestID
		}
		if identity.UserID > 0 {
			meta.UserId = &identity.UserID
		}
		if identity.MerchantID > 0 {
			meta.MerchantId = &identity.MerchantID
		}
		if identity.Role != "" {
			meta.Role = &identity.Role
		}
	}
	if userID, ok := int64ContextValue(ctx, "user_id"); ok && userID > 0 {
		meta.UserId = &userID
	}
	if merchantID, ok := int64ContextValue(ctx, "merchant_id"); ok && merchantID > 0 {
		meta.MerchantId = &merchantID
	}
	if role, ok := ctx.Value("role").(string); ok && role != "" {
		meta.Role = &role
	}
	return meta
}

func int64ContextValue(ctx context.Context, key string) (int64, bool) {
	switch value := ctx.Value(key).(type) {
	case int64:
		return value, true
	case int:
		return int64(value), true
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}
