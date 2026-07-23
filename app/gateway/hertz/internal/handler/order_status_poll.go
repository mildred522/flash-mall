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

func OrderStatusPollHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity, found := authctx.IdentityFrom(ctx)
		if !found || identity.UserID <= 0 {
			fail(ctx, c, consts.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "user login required"))
			return
		}
		requestID := strings.TrimSpace(c.Query("request_id"))
		if requestID == "" {
			ok(ctx, c, OrderStatusPollResp{Status: "missing_request_id"})
			return
		}
		service, err := orderQueryService(svcCtx)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "order query service unavailable", err))
			return
		}
		status, found, err := service.StatusByRequest(ctx, requestID, identity.UserID)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "order status lookup failed", err))
			return
		}
		if !found {
			ok(ctx, c, OrderStatusPollResp{RequestID: requestID, Status: "processing"})
			return
		}
		ok(ctx, c, OrderStatusPollResp{RequestID: status.RequestID, OrderID: status.OrderID, Status: status.Status})
	}
}
