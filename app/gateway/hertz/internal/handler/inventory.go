package handler

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"flash-mall/app/common/apperror"
	"flash-mall/app/common/authctx"
	"flash-mall/app/common/tracectx"
	"flash-mall/app/gateway/hertz/internal/inventoryclient"
	"flash-mall/app/gateway/hertz/internal/svc"
	common "flash-mall/app/inventory/kitex/kitex_gen/flashmall/common"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

const stockSourceInventoryKitex = "inventory-kitex"
const inventoryKitexClientNotConfigured = "inventory kitex client is not configured"

func InventorySummaryHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		productID, err := parsePositiveInt64(c.Query("product_id"))
		if err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "product_id required"))
			return
		}
		stock, err := getInventoryStock(ctx, svcCtx, productID)
		if err != nil {
			fail(ctx, c, inventoryStatusCode(err), err)
			return
		}
		ok(ctx, c, map[string]any{"item": toInventorySummary(stock)})
	}
}

func InventoryReserveHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var req InventoryReserveReq
		if err := decodeJSONBody(c, &req); err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid reserve request"))
			return
		}
		req.OrderID = strings.TrimSpace(req.OrderID)
		if req.OrderID == "" || req.ProductID <= 0 || req.Quantity <= 0 {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "order_id, product_id and positive quantity are required"))
			return
		}
		if err := reserveInventoryStock(ctx, svcCtx, req); err != nil {
			fail(ctx, c, inventoryStatusCode(err), err)
			return
		}
		ok(ctx, c, InventoryOperationResp{
			OrderID:   req.OrderID,
			ProductID: req.ProductID,
			Quantity:  req.Quantity,
			Status:    "reserved",
			Source:    stockSourceInventoryKitex,
		})
	}
}

func InventoryReleaseHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var req InventoryReleaseReq
		if err := decodeJSONBody(c, &req); err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid release request"))
			return
		}
		req.OrderID = strings.TrimSpace(req.OrderID)
		req.Reason = strings.TrimSpace(req.Reason)
		if req.OrderID == "" {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "order_id required"))
			return
		}
		if err := releaseInventoryStock(ctx, svcCtx, req); err != nil {
			fail(ctx, c, inventoryStatusCode(err), err)
			return
		}
		ok(ctx, c, InventoryOperationResp{
			OrderID: req.OrderID,
			Status:  "released",
			Source:  stockSourceInventoryKitex,
		})
	}
}

func InventoryConfirmDeductHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var req InventoryConfirmDeductReq
		if err := decodeJSONBody(c, &req); err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid confirm deduct request"))
			return
		}
		req.OrderID = strings.TrimSpace(req.OrderID)
		if req.OrderID == "" {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "order_id required"))
			return
		}
		if err := confirmInventoryDeduct(ctx, svcCtx, req); err != nil {
			fail(ctx, c, inventoryStatusCode(err), err)
			return
		}
		ok(ctx, c, InventoryOperationResp{
			OrderID: req.OrderID,
			Status:  "deducted",
			Source:  stockSourceInventoryKitex,
		})
	}
}

func getInventoryStock(ctx context.Context, svcCtx *svc.ServiceContext, productID int64) (inventoryclient.Stock, error) {
	client, err := requireInventoryClient(svcCtx)
	if err != nil {
		return inventoryclient.Stock{}, err
	}
	return client.GetStock(ctx, productID, inventoryRequestMeta(ctx))
}

func reserveInventoryStock(ctx context.Context, svcCtx *svc.ServiceContext, req InventoryReserveReq) error {
	client, err := requireInventoryClient(svcCtx)
	if err != nil {
		return err
	}
	return client.ReserveStock(ctx, req.OrderID, req.ProductID, req.Quantity, inventoryRequestMeta(ctx))
}

func releaseInventoryStock(ctx context.Context, svcCtx *svc.ServiceContext, req InventoryReleaseReq) error {
	client, err := requireInventoryClient(svcCtx)
	if err != nil {
		return err
	}
	return client.ReleaseStock(ctx, req.OrderID, req.Reason, inventoryRequestMeta(ctx))
}

func confirmInventoryDeduct(ctx context.Context, svcCtx *svc.ServiceContext, req InventoryConfirmDeductReq) error {
	client, err := requireInventoryClient(svcCtx)
	if err != nil {
		return err
	}
	return client.ConfirmDeduct(ctx, req.OrderID, inventoryRequestMeta(ctx))
}

func requireInventoryClient(svcCtx *svc.ServiceContext) (inventoryclient.Client, error) {
	if svcCtx.InventoryRpc == nil {
		return nil, apperror.New(apperror.CodeInternal, inventoryKitexClientNotConfigured)
	}
	return svcCtx.InventoryRpc, nil
}

func inventoryRequestMeta(ctx context.Context) *common.RequestMeta {
	meta := &common.RequestMeta{}
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
	return meta
}

func decodeJSONBody(c *app.RequestContext, v any) error {
	body, err := c.Body()
	if err != nil {
		return err
	}
	return json.Unmarshal(body, v)
}

func toInventorySummary(stock inventoryclient.Stock) InventorySummary {
	return InventorySummary{
		ProductID: stock.ProductID,
		Available: stock.Available,
		Reserved:  stock.Reserved,
		Total:     stock.Total,
		Source:    stockSourceInventoryKitex,
	}
}

func inventoryStatusCode(err error) int {
	switch apperror.CodeOf(err) {
	case apperror.CodeInvalidArgument:
		return consts.StatusBadRequest
	case apperror.CodeStockNotFound, apperror.CodeProductNotFound, apperror.CodeNotFound:
		return consts.StatusNotFound
	case apperror.CodeStockInsufficient:
		return consts.StatusConflict
	default:
		return consts.StatusBadGateway
	}
}
