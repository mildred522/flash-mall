package inventorykitex

import (
	"context"
	"errors"
	"strings"

	"flash-mall/app/common/apperror"
	"flash-mall/app/gateway/hertz/internal/ports"
	common "flash-mall/app/inventory/kitex/kitex_gen/flashmall/common"
	inventory "flash-mall/app/inventory/kitex/kitex_gen/flashmall/inventory"
	"flash-mall/app/inventory/kitex/kitex_gen/flashmall/inventory/inventoryservice"

	kitexclient "github.com/cloudwego/kitex/client"
)

type Client struct {
	client inventoryservice.Client
}

var _ ports.InventoryService = (*Client)(nil)

func New(endpoint string) (*Client, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return nil, apperror.New(apperror.CodeInvalidArgument, "inventory kitex endpoint required")
	}
	client, err := inventoryservice.NewClient("InventoryService", kitexclient.WithHostPorts(endpoint))
	if err != nil {
		return nil, err
	}
	return &Client{client: client}, nil
}

func (c *Client) GetStock(ctx context.Context, productID int64, meta ports.RequestMeta) (ports.Stock, error) {
	resp, err := c.client.GetStock(ctx, &inventory.GetStockRequest{Meta: toRequestMeta(meta), ProductId: productID})
	if err != nil {
		return ports.Stock{}, toAppError(err)
	}
	if resp == nil || resp.GetStock() == nil {
		return ports.Stock{}, apperror.New(apperror.CodeStockNotFound, "stock not found")
	}
	return toStock(resp.GetStock()), nil
}

func (c *Client) BatchGetStock(ctx context.Context, productIDs []int64, meta ports.RequestMeta) (map[int64]ports.Stock, error) {
	resp, err := c.client.BatchGetStock(ctx, &inventory.BatchGetStockRequest{Meta: toRequestMeta(meta), ProductIds: productIDs})
	if err != nil {
		return nil, toAppError(err)
	}
	result := make(map[int64]ports.Stock, len(productIDs))
	if resp == nil {
		return result, nil
	}
	for _, dto := range resp.GetStocks() {
		if dto == nil || dto.GetProductId() <= 0 {
			continue
		}
		result[dto.GetProductId()] = toStock(dto)
	}
	return result, nil
}

func (c *Client) SeedStock(ctx context.Context, productID int64, total int64, shardCount int32, meta ports.RequestMeta) error {
	var shardCountPtr *int32
	if shardCount > 0 {
		shardCountPtr = &shardCount
	}
	_, err := c.client.SeedStock(ctx, &inventory.SeedStockRequest{Meta: toRequestMeta(meta), ProductId: productID, Total: total, ShardCount: shardCountPtr})
	return toAppError(err)
}

func (c *Client) AdjustStock(ctx context.Context, productID int64, delta int64, bucketIdx int, reason string, meta ports.RequestMeta) (ports.Stock, error) {
	bucketIdxValue := int32(bucketIdx)
	var reasonPtr *string
	reason = strings.TrimSpace(reason)
	if reason != "" {
		reasonPtr = &reason
	}
	resp, err := c.client.AdjustStock(ctx, &inventory.AdjustStockRequest{Meta: toRequestMeta(meta), ProductId: productID, Delta: delta, BucketIdx: &bucketIdxValue, Reason: reasonPtr})
	if err != nil {
		return ports.Stock{}, toAppError(err)
	}
	if resp == nil || resp.GetAfter() == nil {
		return ports.Stock{}, nil
	}
	return toStock(resp.GetAfter()), nil
}

func (c *Client) ReserveStock(ctx context.Context, orderID string, productID int64, quantity int64, meta ports.RequestMeta) error {
	_, err := c.client.ReserveStock(ctx, &inventory.ReserveStockRequest{Meta: toRequestMeta(meta), OrderId: orderID, ProductId: productID, Quantity: quantity})
	return toAppError(err)
}

func (c *Client) ConfirmDeduct(ctx context.Context, orderID string, meta ports.RequestMeta) error {
	_, err := c.client.ConfirmDeduct(ctx, &inventory.ConfirmDeductRequest{Meta: toRequestMeta(meta), OrderId: orderID})
	return toAppError(err)
}

func (c *Client) ReleaseStock(ctx context.Context, orderID string, reason string, meta ports.RequestMeta) error {
	req := &inventory.ReleaseStockRequest{Meta: toRequestMeta(meta), OrderId: orderID}
	reason = strings.TrimSpace(reason)
	if reason != "" {
		req.Reason = &reason
	}
	_, err := c.client.ReleaseStock(ctx, req)
	return toAppError(err)
}

func (c *Client) GetRuntimeState(ctx context.Context, meta ports.RequestMeta) (ports.InventoryRuntimeState, error) {
	resp, err := c.client.GetRuntimeState(ctx, &inventory.GetRuntimeStateRequest{Meta: toRequestMeta(meta)})
	if err != nil {
		return ports.InventoryRuntimeState{}, toAppError(err)
	}
	if resp == nil || resp.GetState() == nil {
		return ports.InventoryRuntimeState{}, apperror.New(apperror.CodeInternal, "inventory runtime state unavailable")
	}
	state := resp.GetState()
	return ports.InventoryRuntimeState{
		FinalDeductEnabled: state.GetFinalDeductEnabled(), RedisConfigured: state.GetRedisConfigured(),
		MySQLConfigured: state.GetMysqlConfigured(), ShardCount: state.GetShardCount(),
		ReservationLedgerMode: state.GetReservationLedgerMode(), ReservationLedgerEnabled: state.GetReservationLedgerEnabled(),
	}, nil
}

func toRequestMeta(meta ports.RequestMeta) *common.RequestMeta {
	result := &common.RequestMeta{}
	if meta.RequestID != "" {
		result.RequestId = &meta.RequestID
	}
	if meta.TraceID != "" {
		result.TraceId = &meta.TraceID
	}
	if meta.UserID > 0 {
		result.UserId = &meta.UserID
	}
	if meta.MerchantID > 0 {
		result.MerchantId = &meta.MerchantID
	}
	if meta.Role != "" {
		result.Role = &meta.Role
	}
	return result
}

func toStock(dto *inventory.StockDTO) ports.Stock {
	return ports.Stock{ProductID: dto.GetProductId(), Available: dto.GetAvailable(), Reserved: dto.GetReserved(), Total: dto.GetTotal()}
}

func toAppError(err error) error {
	if err == nil {
		return nil
	}
	var biz *common.BizException
	if !errors.As(err, &biz) {
		return err
	}
	return apperror.New(toAppCode(biz.GetCode()), biz.GetMessage())
}

func toAppCode(code common.ErrorCode) apperror.Code {
	switch code {
	case common.ErrorCode_INVALID_ARGUMENT:
		return apperror.CodeInvalidArgument
	case common.ErrorCode_UNAUTHORIZED:
		return apperror.CodeUnauthorized
	case common.ErrorCode_FORBIDDEN:
		return apperror.CodeForbidden
	case common.ErrorCode_NOT_FOUND:
		return apperror.CodeNotFound
	case common.ErrorCode_CONFLICT:
		return apperror.CodeConflict
	case common.ErrorCode_STOCK_NOT_FOUND:
		return apperror.CodeStockNotFound
	case common.ErrorCode_STOCK_INSUFFICIENT:
		return apperror.CodeStockInsufficient
	case common.ErrorCode_STOCK_RESERVE_FAILED:
		return apperror.CodeStockReserveFailed
	case common.ErrorCode_STOCK_RECONCILE_FAILED:
		return apperror.CodeStockReconcileFailed
	default:
		return apperror.CodeInternal
	}
}
