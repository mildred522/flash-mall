package handler

import (
	"context"

	"flash-mall/app/common/apperror"
	"flash-mall/app/common/authctx"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func MerchantMeHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity, hasIdentity := authctx.IdentityFrom(ctx)
		if !hasIdentity || identity.UserID <= 0 {
			fail(ctx, c, consts.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "merchant login required"))
			return
		}
		if svcCtx.MerchantQueries == nil {
			fail(ctx, c, consts.StatusBadGateway,
				apperror.New(apperror.CodeInternal, "merchant query service unavailable"))
			return
		}
		resp, err := svcCtx.MerchantQueries.Me(ctx, identity.UserID)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "merchant profile query failed", err))
			return
		}
		ok(ctx, c, resp)
	}
}

func MerchantApplicationHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity, hasIdentity := authctx.IdentityFrom(ctx)
		if !hasIdentity || identity.UserID <= 0 {
			fail(ctx, c, consts.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "merchant login required"))
			return
		}
		if svcCtx.MerchantQueries == nil {
			fail(ctx, c, consts.StatusBadGateway,
				apperror.New(apperror.CodeInternal, "merchant query service unavailable"))
			return
		}
		resp, err := svcCtx.MerchantQueries.LatestApplication(ctx, identity.UserID)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "merchant application query failed", err))
			return
		}
		ok(ctx, c, resp)
	}
}

func MerchantDashboardStatsHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity, hasIdentity := authctx.IdentityFrom(ctx)
		if !hasIdentity || identity.UserID <= 0 {
			fail(ctx, c, consts.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "merchant login required"))
			return
		}
		merchantID, ready := merchantScope(ctx, c, svcCtx, identity)
		if !ready {
			return
		}
		if svcCtx.MerchantQueries == nil {
			fail(ctx, c, consts.StatusBadGateway,
				apperror.New(apperror.CodeInternal, "merchant query service unavailable"))
			return
		}
		resp, err := svcCtx.MerchantQueries.Dashboard(ctx, merchantID)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "merchant dashboard query failed", err))
			return
		}
		ok(ctx, c, resp)
	}
}

func merchantApplicationStatusText(status int64) string {
	switch status {
	case 0:
		return "pending"
	case 1:
		return "approved"
	case 2:
		return "rejected"
	default:
		return "unknown"
	}
}
