package handler

import (
	"context"
	"errors"

	"flash-mall/app/common/apperror"
	"flash-mall/app/common/authctx"
	"flash-mall/app/gateway/hertz/internal/application/merchantquery"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func MerchantProductListHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity, hasIdentity := authctx.IdentityFrom(ctx)
		if !hasIdentity || identity.UserID <= 0 {
			fail(ctx, c, consts.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "merchant login required"))
			return
		}

		merchantID, err := selectedMerchantIDFromService(ctx, svcCtx, identity)
		if err != nil {
			statusCode := consts.StatusForbidden
			var appErr *apperror.Error
			if errors.As(err, &appErr) && appErr.Code == apperror.CodeMerchantNotFound {
				statusCode = consts.StatusNotFound
			}
			fail(ctx, c, statusCode, err)
			return
		}

		req, appErr := parseProductListQuery(c, false)
		if appErr != nil {
			fail(ctx, c, consts.StatusBadRequest, appErr)
			return
		}
		req.MerchantID = merchantID

		items, total, err := loadAdminProducts(ctx, svcCtx, req)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "merchant product query failed", err))
			return
		}
		ok(ctx, c, AdminProductListResp{Items: items, Total: total, Page: req.Page, PageSize: req.PageSize})
	}
}

func MerchantProductStockAdjustHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
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

		var req AdminProductStockAdjustReq
		if err := decodeJSONBody(c, &req); err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid product stock adjust request"))
			return
		}
		if req.ProductID <= 0 || req.Delta == 0 {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "product_id and non-zero delta are required"))
			return
		}
		if req.BucketIdx < 0 {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "bucket_idx must be non-negative"))
			return
		}
		owns, err := merchantOwnsGatewayProduct(ctx, svcCtx, merchantID, req.ProductID)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "merchant product stock adjust failed", err))
			return
		}
		if !owns {
			fail(ctx, c, consts.StatusNotFound, apperror.New(apperror.CodeProductNotFound, "product not found for merchant"))
			return
		}

		identity.MerchantID = merchantID
		callCtx := authctx.WithIdentity(ctx, identity)
		inventoryRpc, err := requireInventoryClient(svcCtx)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, err)
			return
		}
		after, err := inventoryRpc.AdjustStock(callCtx, req.ProductID, req.Delta, int(req.BucketIdx), "merchant stock adjust", inventoryRequestMeta(callCtx))
		if err != nil {
			writeStockAdjustError(ctx, c, svcCtx, req, err)
			return
		}
		refreshProductCardSnapshotsBestEffort(ctx, svcCtx, req.ProductID)
		invalidateProductReadCaches(ctx, svcCtx, req.ProductID, merchantID)
		ok(ctx, c, AdminProductStockAdjustResp{ProductID: req.ProductID, StockAvailable: after.Total})
	}
}

func selectedMerchantIDFromService(ctx context.Context, svcCtx *svc.ServiceContext, identity authctx.Identity) (int64, error) {
	if svcCtx.MerchantQueries == nil {
		return 0, errors.New("merchant query service unavailable")
	}
	return svcCtx.MerchantQueries.ResolveScope(ctx, merchantquery.ScopeRequest{
		UserID: identity.UserID, MerchantID: identity.MerchantID, Admin: identity.CanAdmin(),
	})
}

func merchantOwnsGatewayProduct(ctx context.Context, svcCtx *svc.ServiceContext, merchantID, productID int64) (bool, error) {
	if svcCtx.CatalogQueries == nil {
		return false, errors.New("catalog query service unavailable")
	}
	return svcCtx.CatalogQueries.OwnsProduct(ctx, merchantID, productID)
}
