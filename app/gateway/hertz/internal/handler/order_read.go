package handler

import (
	"context"
	"strings"

	"flash-mall/app/common/apperror"
	"flash-mall/app/common/authctx"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func OrderListHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity, hasIdentity := authctx.IdentityFrom(ctx)
		if !hasIdentity || identity.UserID <= 0 {
			fail(ctx, c, consts.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "user login required"))
			return
		}

		queries, err := orderQueryService(svcCtx)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "order query failed", err))
			return
		}
		items, err := queries.List(ctx, identity.UserID)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "order query failed", err))
			return
		}
		ok(ctx, c, OrderListResp{Items: items})
	}
}

func OrderDetailHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity, hasIdentity := authctx.IdentityFrom(ctx)
		if !hasIdentity || identity.UserID <= 0 {
			fail(ctx, c, consts.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "user login required"))
			return
		}
		orderID := strings.TrimSpace(c.Query("order_id"))
		if orderID == "" {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "order_id is required"))
			return
		}

		resp, err := loadUserOrderDetail(ctx, svcCtx, orderID, identity.UserID)
		if err != nil {
			fail(ctx, c, createOrderStatusCode(err), err)
			return
		}
		ok(ctx, c, resp)
	}
}

func loadUserOrderDetail(ctx context.Context, svcCtx *svc.ServiceContext, orderID string, userID int64) (OrderDetailResp, error) {
	queries, err := orderQueryService(svcCtx)
	if err != nil {
		return OrderDetailResp{}, err
	}
	return queries.Detail(ctx, orderID, userID)
}
