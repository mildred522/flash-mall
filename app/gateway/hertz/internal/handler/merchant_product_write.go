package handler

import (
	"context"
	"fmt"

	"flash-mall/app/common/apperror"
	"flash-mall/app/common/authctx"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func MerchantProductCreateHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity, hasIdentity := authctx.IdentityFrom(ctx)
		if !hasIdentity || identity.UserID <= 0 {
			fail(ctx, c, consts.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "merchant login required"))
			return
		}
		merchantID, err := selectedMerchantIDFromService(ctx, svcCtx, identity)
		if err != nil {
			fail(ctx, c, consts.StatusForbidden, err)
			return
		}
		var req AdminProductCreateReq
		if err := decodeJSONBody(c, &req); err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid product create request"))
			return
		}
		req.MerchantID = merchantID
		initializer, err := requireProductInventoryInitializer(svcCtx)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, err)
			return
		}
		if svcCtx.ProductCommands == nil {
			failProductCommand(ctx, c, "merchant product create failed", errProductCommandUnavailable)
			return
		}
		result, err := svcCtx.ProductCommands.Create(ctx, req)
		if err != nil {
			failProductCommand(ctx, c, "merchant product create failed", err)
			return
		}
		identity.MerchantID = merchantID
		callCtx := authctx.WithIdentity(ctx, identity)
		if _, err = initializer.Initialize(callCtx, result.ProductID, inventoryRequestMeta(callCtx)); err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeStockReconcileFailed,
				fmt.Sprintf("product %d created offline; inventory seed can be retried", result.ProductID), err))
			return
		}
		refreshProductCardSnapshotsBestEffort(ctx, svcCtx, result.ProductID)
		invalidateProductReadCaches(ctx, svcCtx, result.ProductID, merchantID)
		ok(ctx, c, AdminProductCreateResp{ProductID: result.ProductID})
	}
}

func MerchantProductUpdateHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity, hasIdentity := authctx.IdentityFrom(ctx)
		if !hasIdentity || identity.UserID <= 0 {
			fail(ctx, c, consts.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "merchant login required"))
			return
		}
		merchantID, err := selectedMerchantIDFromService(ctx, svcCtx, identity)
		if err != nil {
			fail(ctx, c, consts.StatusForbidden, err)
			return
		}
		var req AdminProductUpdateReq
		if err := decodeJSONBody(c, &req); err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid product update request"))
			return
		}
		req.MerchantID = merchantID
		if svcCtx.ProductCommands == nil {
			failProductCommand(ctx, c, "merchant product update failed", errProductCommandUnavailable)
			return
		}
		if err := svcCtx.ProductCommands.Update(ctx, req); err != nil {
			failProductCommand(ctx, c, "merchant product update failed", err)
			return
		}
		refreshProductCardSnapshotsBestEffort(ctx, svcCtx, req.ProductID)
		invalidateProductReadCaches(ctx, svcCtx, req.ProductID, merchantID)
		ok(ctx, c, map[string]any{"ok": true})
	}
}
