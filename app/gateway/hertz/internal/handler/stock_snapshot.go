package handler

import (
	"context"

	"flash-mall/app/common/apperror"
	"flash-mall/app/gateway/hertz/internal/ports"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func AdminStockSnapshotRebuildHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var req AdminStockSnapshotRebuildReq
		if err := decodeJSONBody(c, &req); err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid stock snapshot rebuild request"))
			return
		}
		if req.ProductID < 0 {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "product_id must be positive"))
			return
		}
		if req.Limit <= 0 || req.Limit > 10000 {
			req.Limit = 1000
		}
		store, err := requireProductSnapshotStore(svcCtx)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, err)
			return
		}
		request := ports.SnapshotRebuildRequest{ProductID: req.ProductID, Limit: req.Limit}
		affected, err := store.RebuildStock(ctx, request)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "stock snapshot rebuild failed", err))
			return
		}
		cardAffected, err := store.RebuildCards(ctx, request)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "product card snapshot rebuild failed", err))
			return
		}
		ok(ctx, c, map[string]any{"affected": affected, "card_affected": cardAffected, "limit": req.Limit})
	}
}
