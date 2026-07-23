package handler

import (
	"context"
	"errors"

	"flash-mall/app/common/apperror"
	"flash-mall/app/common/authctx"
	"flash-mall/app/gateway/hertz/internal/application/stockaudit"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

var errStockAuditServiceUnavailable = errors.New("stock audit service unavailable")

func AdminStockChangeLogHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		writeStockChangeLog(ctx, c, svcCtx, 0)
	}
}

func MerchantStockChangeLogHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity, found := authctx.IdentityFrom(ctx)
		if !found || identity.UserID <= 0 {
			fail(ctx, c, consts.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "merchant login required"))
			return
		}
		merchantID, err := selectedMerchantIDFromService(ctx, svcCtx, identity)
		if err != nil {
			fail(ctx, c, consts.StatusForbidden, err)
			return
		}
		writeStockChangeLog(ctx, c, svcCtx, merchantID)
	}
}

func writeStockChangeLog(ctx context.Context, c *app.RequestContext, svcCtx *svc.ServiceContext, merchantID int64) {
	page, err := parseInt64Default(c.Query("page"), 1)
	if err != nil {
		fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "page must be numeric"))
		return
	}
	pageSize, err := parseInt64Default(c.Query("page_size"), 20)
	if err != nil {
		fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "page_size must be numeric"))
		return
	}
	productID, err := parseInt64Default(c.Query("product_id"), 0)
	if err != nil {
		fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "product_id must be numeric"))
		return
	}
	if svcCtx.StockAudits == nil {
		fail(ctx, c, consts.StatusBadGateway, errStockAuditServiceUnavailable)
		return
	}
	result, err := svcCtx.StockAudits.List(ctx, stockaudit.Query{
		Page: page, PageSize: pageSize, ProductID: productID, MerchantID: merchantID,
		OrderID: c.Query("order_id"), ChangeType: c.Query("change_type"),
	})
	if err != nil {
		fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "stock audit query failed", err))
		return
	}
	ok(ctx, c, result)
}
