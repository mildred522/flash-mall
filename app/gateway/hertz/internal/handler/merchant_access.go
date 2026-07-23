package handler

import (
	"context"

	"flash-mall/app/common/authctx"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func merchantScope(ctx context.Context, c *app.RequestContext, svcCtx *svc.ServiceContext, identity authctx.Identity) (int64, bool) {
	merchantID, err := selectedMerchantIDFromService(ctx, svcCtx, identity)
	if err != nil {
		fail(ctx, c, consts.StatusForbidden, err)
		return 0, false
	}
	return merchantID, true
}
