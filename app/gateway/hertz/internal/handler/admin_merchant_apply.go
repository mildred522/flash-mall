package handler

import (
	"context"

	"flash-mall/app/common/apperror"
	"flash-mall/app/common/authctx"
	"flash-mall/app/gateway/hertz/internal/application/merchantonboarding"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

type adminMerchantApplyAuditReq = merchantonboarding.AuditInput

func AdminMerchantApplyListHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		query, err := adminMerchantApplicationQueryFromRequest(c)
		if err != nil {
			fail(ctx, c, consts.StatusBadRequest, err)
			return
		}
		if svcCtx.MerchantOnboarding == nil {
			failMerchantOnboarding(ctx, c, "merchant applications query failed", errMerchantOnboardingUnavailable)
			return
		}
		result, err := svcCtx.MerchantOnboarding.List(ctx, query)
		if err != nil {
			failMerchantOnboarding(ctx, c, "merchant applications query failed", err)
			return
		}
		ok(ctx, c, result)
	}
}

func adminMerchantApplicationQueryFromRequest(c *app.RequestContext) (AdminMerchantApplicationListReq, error) {
	status, err := parseInt64Default(c.Query("status"), -1)
	if err != nil {
		return AdminMerchantApplicationListReq{},
			apperror.New(apperror.CodeInvalidArgument, "status must be numeric")
	}
	page, err := parseInt64Default(c.Query("page"), 1)
	if err != nil {
		return AdminMerchantApplicationListReq{}, apperror.New(apperror.CodeInvalidArgument, "page must be numeric")
	}
	pageSize, err := parseInt64Default(c.Query("page_size"), 20)
	if err != nil {
		return AdminMerchantApplicationListReq{},
			apperror.New(apperror.CodeInvalidArgument, "page_size must be numeric")
	}
	return AdminMerchantApplicationListReq{Status: status, Page: page, PageSize: pageSize}, nil
}

func AdminMerchantApplyAuditHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var req adminMerchantApplyAuditReq
		if err := decodeJSONBody(c, &req); err != nil {
			fail(ctx, c, consts.StatusBadRequest,
				apperror.New(apperror.CodeInvalidArgument, "invalid merchant audit request"))
			return
		}
		if svcCtx.MerchantOnboarding == nil {
			failMerchantOnboarding(ctx, c, "merchant application audit failed", errMerchantOnboardingUnavailable)
			return
		}
		if identity, exists := authctx.IdentityFrom(ctx); exists {
			req.OperatorID = identity.UserID
		}
		result, err := svcCtx.MerchantOnboarding.Audit(ctx, req)
		if err != nil {
			failMerchantOnboarding(ctx, c, "merchant application audit failed", err)
			return
		}
		ok(ctx, c, result)
	}
}
