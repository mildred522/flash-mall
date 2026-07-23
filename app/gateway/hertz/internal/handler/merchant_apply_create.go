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

func MerchantApplyCreateHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity, hasIdentity := authctx.IdentityFrom(ctx)
		if !hasIdentity || identity.UserID <= 0 {
			fail(ctx, c, consts.StatusUnauthorized,
				apperror.New(apperror.CodeUnauthorized, "merchant login required"))
			return
		}
		var req MerchantApplyReq
		if err := decodeJSONBody(c, &req); err != nil {
			fail(ctx, c, consts.StatusBadRequest,
				apperror.New(apperror.CodeInvalidArgument, "invalid merchant apply request"))
			return
		}
		if svcCtx.MerchantOnboarding == nil {
			failMerchantOnboarding(ctx, c, "merchant apply failed", errMerchantOnboardingUnavailable)
			return
		}
		req.UserID = identity.UserID
		result, err := svcCtx.MerchantOnboarding.Submit(ctx, req)
		if err != nil {
			failMerchantOnboarding(ctx, c, "merchant apply failed", err)
			return
		}
		ok(ctx, c, MerchantApplyResp{
			ApplyID: result.ApplyID,
			Status:  merchantonboarding.StatusText(result.Status),
		})
	}
}
