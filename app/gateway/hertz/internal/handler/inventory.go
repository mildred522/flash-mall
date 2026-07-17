package handler

import (
	"context"
	"strconv"
	"strings"

	"flash-mall/app/common/apperror"
	"flash-mall/app/common/authctx"
	"flash-mall/app/common/tracectx"
	"flash-mall/app/gateway/hertz/internal/ports"
	"flash-mall/app/gateway/hertz/internal/svc"
	"flash-mall/app/gateway/hertz/internal/transport/httpx"

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

func getInventoryStock(ctx context.Context, svcCtx *svc.ServiceContext, productID int64) (ports.Stock, error) {
	client, err := requireInventoryClient(svcCtx)
	if err != nil {
		return ports.Stock{}, err
	}
	return client.GetStock(ctx, productID, inventoryRequestMeta(ctx))
}

func requireInventoryClient(svcCtx *svc.ServiceContext) (ports.InventoryService, error) {
	if svcCtx.InventoryRpc == nil {
		return nil, apperror.New(apperror.CodeInternal, inventoryKitexClientNotConfigured)
	}
	return svcCtx.InventoryRpc, nil
}

func inventoryRequestMeta(ctx context.Context) ports.RequestMeta {
	meta := ports.RequestMeta{}
	if trace, ok := tracectx.FromContext(ctx); ok {
		meta.RequestID = trace.RequestID
		meta.TraceID = trace.TraceID
		if userID, err := strconv.ParseInt(strings.TrimSpace(trace.UserID), 10, 64); err == nil && userID > 0 {
			meta.UserID = userID
		}
		if merchantID, err := strconv.ParseInt(strings.TrimSpace(trace.MerchantID), 10, 64); err == nil && merchantID > 0 {
			meta.MerchantID = merchantID
		}
	}
	if identity, ok := authctx.IdentityFrom(ctx); ok {
		if identity.UserID > 0 {
			meta.UserID = identity.UserID
		}
		if identity.MerchantID > 0 {
			meta.MerchantID = identity.MerchantID
		}
		if identity.Role != "" {
			meta.Role = identity.Role
		}
	}
	return meta
}

func decodeJSONBody(c *app.RequestContext, v any) error {
	return httpx.DecodeJSONBody(c, v)
}

func toInventorySummary(stock ports.Stock) InventorySummary {
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
