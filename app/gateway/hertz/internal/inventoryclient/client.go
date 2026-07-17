package inventoryclient

import (
	"context"
	"errors"
	"strings"

	"flash-mall/app/common/apperror"
	common "flash-mall/app/inventory/kitex/kitex_gen/flashmall/common"
	inventory "flash-mall/app/inventory/kitex/kitex_gen/flashmall/inventory"
	"flash-mall/app/inventory/kitex/kitex_gen/flashmall/inventory/inventoryservice"

	kitexclient "github.com/cloudwego/kitex/client"
)

type Stock struct {
	ProductID int64
	Available int64
	Reserved  int64
	Total     int64
}

type RuntimeState struct {
	FinalDeductEnabled       bool
	RedisConfigured          bool
	MySQLConfigured          bool
	ShardCount               int32
	ReservationLedgerMode    string
	ReservationLedgerEnabled bool
}

type Client interface {
	GetStock(ctx context.Context, productID int64, meta *common.RequestMeta) (Stock, error)
	BatchGetStock(ctx context.Context, productIDs []int64, meta *common.RequestMeta) (map[int64]Stock, error)
	SeedStock(ctx context.Context, productID int64, total int64, shardCount int32, meta *common.RequestMeta) error
	AdjustStock(ctx context.Context, productID int64, delta int64, bucketIdx int, reason string, meta *common.RequestMeta) (Stock, error)
	ReserveStock(ctx context.Context, orderID string, productID int64, quantity int64, meta *common.RequestMeta) error
	ConfirmDeduct(ctx context.Context, orderID string, meta *common.RequestMeta) error
	ReleaseStock(ctx context.Context, orderID string, reason string, meta *common.RequestMeta) error
	GetRuntimeState(ctx context.Context, meta *common.RequestMeta) (RuntimeState, error)
}

type KitexClient struct {
	client inventoryservice.Client
}

func NewKitexClient(endpoint string) (*KitexClient, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return nil, apperror.New(apperror.CodeInvalidArgument, "inventory kitex endpoint required")
	}
	client, err := inventoryservice.NewClient("InventoryService", kitexclient.WithHostPorts(endpoint))
	if err != nil {
		return nil, err
	}
	return &KitexClient{client: client}, nil
}

func (c *KitexClient) GetStock(ctx context.Context, productID int64, meta *common.RequestMeta) (Stock, error) {
	resp, err := c.client.GetStock(ctx, &inventory.GetStockRequest{Meta: meta, ProductId: productID})
	if err != nil {
		return Stock{}, toAppError(err)
	}
	if resp == nil {
		return Stock{}, apperror.New(apperror.CodeStockNotFound, "stock not found")
	}
	dto := resp.GetStock()
	if dto == nil {
		return Stock{}, apperror.New(apperror.CodeStockNotFound, "stock not found")
	}
	return Stock{
		ProductID: dto.GetProductId(),
		Available: dto.GetAvailable(),
		Reserved:  dto.GetReserved(),
		Total:     dto.GetTotal(),
	}, nil
}

func (c *KitexClient) BatchGetStock(ctx context.Context, productIDs []int64, meta *common.RequestMeta) (map[int64]Stock, error) {
	resp, err := c.client.BatchGetStock(ctx, &inventory.BatchGetStockRequest{Meta: meta, ProductIds: productIDs})
	if err != nil {
		return nil, toAppError(err)
	}
	result := make(map[int64]Stock, len(productIDs))
	if resp == nil {
		return result, nil
	}
	for _, dto := range resp.GetStocks() {
		if dto == nil || dto.GetProductId() <= 0 {
			continue
		}
		result[dto.GetProductId()] = Stock{
			ProductID: dto.GetProductId(),
			Available: dto.GetAvailable(),
			Reserved:  dto.GetReserved(),
			Total:     dto.GetTotal(),
		}
	}
	return result, nil
}

func (c *KitexClient) SeedStock(ctx context.Context, productID int64, total int64, shardCount int32, meta *common.RequestMeta) error {
	var shardCountPtr *int32
	if shardCount > 0 {
		shardCountPtr = &shardCount
	}
	_, err := c.client.SeedStock(ctx, &inventory.SeedStockRequest{
		Meta:       meta,
		ProductId:  productID,
		Total:      total,
		ShardCount: shardCountPtr,
	})
	return toAppError(err)
}

func (c *KitexClient) AdjustStock(ctx context.Context, productID int64, delta int64, bucketIdx int, reason string, meta *common.RequestMeta) (Stock, error) {
	bucketIdxValue := int32(bucketIdx)
	var reasonPtr *string
	reason = strings.TrimSpace(reason)
	if reason != "" {
		reasonPtr = &reason
	}
	resp, err := c.client.AdjustStock(ctx, &inventory.AdjustStockRequest{
		Meta:      meta,
		ProductId: productID,
		Delta:     delta,
		BucketIdx: &bucketIdxValue,
		Reason:    reasonPtr,
	})
	if err != nil {
		return Stock{}, toAppError(err)
	}
	if resp == nil || resp.GetAfter() == nil {
		return Stock{}, nil
	}
	after := resp.GetAfter()
	return Stock{
		ProductID: after.GetProductId(),
		Available: after.GetAvailable(),
		Reserved:  after.GetReserved(),
		Total:     after.GetTotal(),
	}, nil
}

func (c *KitexClient) ReserveStock(ctx context.Context, orderID string, productID int64, quantity int64, meta *common.RequestMeta) error {
	_, err := c.client.ReserveStock(ctx, &inventory.ReserveStockRequest{
		Meta:      meta,
		OrderId:   orderID,
		ProductId: productID,
		Quantity:  quantity,
	})
	return toAppError(err)
}

func (c *KitexClient) ConfirmDeduct(ctx context.Context, orderID string, meta *common.RequestMeta) error {
	_, err := c.client.ConfirmDeduct(ctx, &inventory.ConfirmDeductRequest{
		Meta:    meta,
		OrderId: orderID,
	})
	return toAppError(err)
}

func (c *KitexClient) ReleaseStock(ctx context.Context, orderID string, reason string, meta *common.RequestMeta) error {
	req := &inventory.ReleaseStockRequest{
		Meta:    meta,
		OrderId: orderID,
	}
	reason = strings.TrimSpace(reason)
	if reason != "" {
		req.Reason = &reason
	}
	_, err := c.client.ReleaseStock(ctx, req)
	return toAppError(err)
}

func (c *KitexClient) GetRuntimeState(ctx context.Context, meta *common.RequestMeta) (RuntimeState, error) {
	resp, err := c.client.GetRuntimeState(ctx, &inventory.GetRuntimeStateRequest{Meta: meta})
	if err != nil {
		return RuntimeState{}, toAppError(err)
	}
	if resp == nil || resp.GetState() == nil {
		return RuntimeState{}, apperror.New(apperror.CodeInternal, "inventory runtime state unavailable")
	}
	state := resp.GetState()
	return RuntimeState{
		FinalDeductEnabled:       state.GetFinalDeductEnabled(),
		RedisConfigured:          state.GetRedisConfigured(),
		MySQLConfigured:          state.GetMysqlConfigured(),
		ShardCount:               state.GetShardCount(),
		ReservationLedgerMode:    state.GetReservationLedgerMode(),
		ReservationLedgerEnabled: state.GetReservationLedgerEnabled(),
	}, nil
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
