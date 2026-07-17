package handler

import (
	"context"

	"flash-mall/app/common/apperror"
	"flash-mall/app/common/authctx"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func AdminProductInventorySeedRetryHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var req ProductInventorySeedRetryReq
		if err := decodeJSONBody(c, &req); err != nil || req.ProductID <= 0 {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "positive product_id is required"))
			return
		}
		if err := retryProductInventorySeed(ctx, svcCtx, req.ProductID); err != nil {
			fail(ctx, c, productMutationStatusCode(err), err)
			return
		}
		ok(ctx, c, ProductInventorySeedRetryResp{ProductID: req.ProductID, Status: "succeeded"})
	}
}

func MerchantProductInventorySeedRetryHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity, hasIdentity := authctx.IdentityFrom(ctx)
		if !hasIdentity || identity.UserID <= 0 {
			fail(ctx, c, consts.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "merchant login required"))
			return
		}
		var req ProductInventorySeedRetryReq
		if err := decodeJSONBody(c, &req); err != nil || req.ProductID <= 0 {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "positive product_id is required"))
			return
		}
		merchantID, err := selectedMerchantIDFromService(ctx, svcCtx, identity)
		if err != nil {
			fail(ctx, c, consts.StatusForbidden, err)
			return
		}
		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, err)
			return
		}
		owns, err := merchantOwnsGatewayProduct(ctx, db, merchantID, req.ProductID)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, err)
			return
		}
		if !owns {
			fail(ctx, c, consts.StatusNotFound, apperror.New(apperror.CodeProductNotFound, "product not found for merchant"))
			return
		}
		identity.MerchantID = merchantID
		callCtx := authctx.WithIdentity(ctx, identity)
		if err := retryProductInventorySeed(callCtx, svcCtx, req.ProductID); err != nil {
			fail(ctx, c, productMutationStatusCode(err), err)
			return
		}
		ok(ctx, c, ProductInventorySeedRetryResp{ProductID: req.ProductID, Status: "succeeded"})
	}
}

func retryProductInventorySeed(ctx context.Context, svcCtx *svc.ServiceContext, productID int64) error {
	initializer, err := requireProductInventoryInitializer(svcCtx)
	if err != nil {
		return err
	}
	if _, err = initializer.Initialize(ctx, productID, inventoryRequestMeta(ctx)); err != nil {
		return err
	}
	refreshProductCardSnapshotsBestEffort(ctx, svcCtx, productID)
	invalidateProductReadCaches(ctx, svcCtx, productID, 0)
	return nil
}
