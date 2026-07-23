package handler

import (
	"context"
	"strings"

	"flash-mall/app/common/apperror"
	"flash-mall/app/common/authctx"
	"flash-mall/app/common/orderstatus"
	"flash-mall/app/gateway/hertz/internal/application/orderquery"
	"flash-mall/app/gateway/hertz/internal/ports"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func MerchantOrderListHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity, hasIdentity := authctx.IdentityFrom(ctx)
		if !hasIdentity || identity.UserID <= 0 {
			fail(ctx, c, consts.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "merchant login required"))
			return
		}
		service, merchantID, ready := merchantOrderAccess(ctx, c, svcCtx, identity)
		if !ready {
			return
		}
		query, err := merchantOrderQueryFromRequest(c, merchantID)
		if err != nil {
			fail(ctx, c, consts.StatusBadRequest, err)
			return
		}
		resp, err := service.ListMerchantOrders(ctx, query)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "merchant order query failed", err))
			return
		}
		ok(ctx, c, resp)
	}
}

func MerchantRefundListHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity, hasIdentity := authctx.IdentityFrom(ctx)
		if !hasIdentity || identity.UserID <= 0 {
			fail(ctx, c, consts.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "merchant login required"))
			return
		}
		service, merchantID, ready := merchantOrderAccess(ctx, c, svcCtx, identity)
		if !ready {
			return
		}
		query, err := merchantRefundQueryFromRequest(c, merchantID)
		if err != nil {
			fail(ctx, c, consts.StatusBadRequest, err)
			return
		}
		resp, err := service.ListMerchantRefunds(ctx, query)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "merchant refund query failed", err))
			return
		}
		ok(ctx, c, resp)
	}
}

func MerchantShipOrderHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity, hasIdentity := authctx.IdentityFrom(ctx)
		if !hasIdentity || identity.UserID <= 0 {
			fail(ctx, c, consts.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "merchant login required"))
			return
		}
		_, merchantID, ready := merchantOrderAccess(ctx, c, svcCtx, identity)
		if !ready {
			return
		}

		var req MerchantShipOrderReq
		if err := decodeJSONBody(c, &req); err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid ship order request"))
			return
		}
		req.OrderID = strings.TrimSpace(req.OrderID)
		if req.OrderID == "" {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "order_id is required"))
			return
		}
		commands, err := requireOrderCommands(svcCtx)
		if err == nil {
			err = commands.ShipMerchant(ctx, ports.ShipMerchantOrderCommand{
				OrderID: req.OrderID, MerchantID: merchantID, Meta: inventoryRequestMeta(ctx),
			})
		}
		if err != nil {
			fail(ctx, c, createOrderStatusCode(err), err)
			return
		}
		ok(ctx, c, MerchantShipOrderResp{OrderID: req.OrderID, Status: orderstatus.Text(orderstatus.Shipped)})
	}
}

func merchantOrderAccess(ctx context.Context, c *app.RequestContext, svcCtx *svc.ServiceContext, identity authctx.Identity) (*orderquery.BackofficeService, int64, bool) {
	merchantID, ready := merchantScope(ctx, c, svcCtx, identity)
	if !ready {
		return nil, 0, false
	}
	service, err := backofficeOrderService(svcCtx)
	if err != nil {
		fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "order datasource unavailable", err))
		return nil, 0, false
	}
	return service, merchantID, true
}

func merchantOrderQueryFromRequest(c *app.RequestContext, merchantID int64) (MerchantOrderListReq, error) {
	page, err := parseInt64Default(c.Query("page"), 1)
	if err != nil {
		return MerchantOrderListReq{}, apperror.New(apperror.CodeInvalidArgument, "page must be numeric")
	}
	pageSize, err := parseInt64Default(c.Query("page_size"), 50)
	if err != nil {
		return MerchantOrderListReq{}, apperror.New(apperror.CodeInvalidArgument, "page_size must be numeric")
	}
	status, err := parseInt64Default(c.Query("status"), -1)
	if err != nil {
		return MerchantOrderListReq{}, apperror.New(apperror.CodeInvalidArgument, "status must be numeric")
	}
	userID, err := parseOptionalInt64(c.Query("user_id"))
	if err != nil {
		return MerchantOrderListReq{}, apperror.New(apperror.CodeInvalidArgument, "user_id must be numeric")
	}
	productID, err := parseOptionalInt64(c.Query("product_id"))
	if err != nil {
		return MerchantOrderListReq{}, apperror.New(apperror.CodeInvalidArgument, "product_id must be numeric")
	}
	return MerchantOrderListReq{
		MerchantID: merchantID,
		Page:       normalizePage(page),
		PageSize:   normalizePageSize(pageSize),
		Status:     status,
		UserID:     userID,
		ProductID:  productID,
		OrderID:    strings.TrimSpace(c.Query("order_id")),
	}, nil
}

func merchantRefundQueryFromRequest(c *app.RequestContext, merchantID int64) (MerchantRefundListReq, error) {
	page, err := parseInt64Default(c.Query("page"), 1)
	if err != nil {
		return MerchantRefundListReq{}, apperror.New(apperror.CodeInvalidArgument, "page must be numeric")
	}
	pageSize, err := parseInt64Default(c.Query("page_size"), 50)
	if err != nil {
		return MerchantRefundListReq{}, apperror.New(apperror.CodeInvalidArgument, "page_size must be numeric")
	}
	status, err := parseInt64Default(c.Query("status"), -1)
	if err != nil {
		return MerchantRefundListReq{}, apperror.New(apperror.CodeInvalidArgument, "status must be numeric")
	}
	userID, err := parseOptionalInt64(c.Query("user_id"))
	if err != nil {
		return MerchantRefundListReq{}, apperror.New(apperror.CodeInvalidArgument, "user_id must be numeric")
	}
	return MerchantRefundListReq{
		MerchantID: merchantID,
		Page:       normalizePage(page),
		PageSize:   normalizePageSize(pageSize),
		Status:     status,
		UserID:     userID,
		OrderID:    strings.TrimSpace(c.Query("order_id")),
	}, nil
}

func normalizePage(page int64) int64 {
	if page <= 0 {
		return 1
	}
	return page
}

func normalizePageSize(pageSize int64) int64 {
	if pageSize <= 0 || pageSize > 100 {
		return 50
	}
	return pageSize
}
