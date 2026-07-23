package handler

import (
	"context"
	"errors"

	"flash-mall/app/common/apperror"
	"flash-mall/app/gateway/hertz/internal/application/campaign"
	"flash-mall/app/gateway/hertz/internal/svc"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

var errCampaignServiceUnavailable = errors.New("campaign service unavailable")

type adminCampaignReq = campaign.UpsertInput

func AdminCampaignListHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		if svcCtx.Campaigns == nil {
			fail(ctx, c, consts.StatusBadGateway, errCampaignServiceUnavailable)
			return
		}
		items, err := svcCtx.Campaigns.List(ctx)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, err)
			return
		}
		ok(ctx, c, map[string]any{"items": items})
	}
}

func AdminCampaignUpsertHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var req adminCampaignReq
		if err := decodeJSONBody(c, &req); err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid campaign request"))
			return
		}
		if svcCtx.Campaigns == nil {
			fail(ctx, c, consts.StatusBadGateway, errCampaignServiceUnavailable)
			return
		}
		id, err := svcCtx.Campaigns.Upsert(ctx, req)
		if err != nil {
			if fault, ok := campaign.AsFault(err); ok {
				fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, fault.Message))
				return
			}
			fail(ctx, c, consts.StatusBadGateway, err)
			return
		}
		ok(ctx, c, map[string]any{"campaign_id": id})
	}
}
