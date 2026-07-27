package handler

import (
	"context"
	"fmt"

	"flash-mall/app/common/apperror"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

const defaultGatewayMerchantID int64 = 1000

func AdminProductUpdateHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var req AdminProductUpdateReq
		if err := decodeJSONBody(c, &req); err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid product update request"))
			return
		}
		if svcCtx.ProductCommands == nil {
			failProductCommand(ctx, c, "admin product update failed", errProductCommandUnavailable)
			return
		}
		if req.Status != nil && *req.Status == 1 {
			if err := prepareProductVisibility(ctx, svcCtx, req.ProductID); err != nil {
				fail(ctx, c, consts.StatusBadGateway, err)
				return
			}
		}
		if err := svcCtx.ProductCommands.Update(ctx, req); err != nil {
			if reason := productCommandReason(err); reason != "" {
				recordGatewayAdminAuditFailure(c, svcCtx, productUpdateAuditEvent(req.Status),
					fmt.Sprintf("product:%d reason:%s", req.ProductID, reason))
			}
			failProductCommand(ctx, c, "admin product update failed", err)
			return
		}
		refreshProductCardSnapshotsBestEffort(ctx, svcCtx, req.ProductID)
		invalidateProductReadCaches(ctx, svcCtx, req.ProductID, 0)
		recordGatewayAdminAuditEvent(c, svcCtx, productUpdateAuditEvent(req.Status), fmt.Sprintf("product:%d", req.ProductID))
		ok(ctx, c, map[string]any{"ok": true})
	}
}

func AdminProductCreateHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var req AdminProductCreateReq
		if err := decodeJSONBody(c, &req); err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid product create request"))
			return
		}
		if req.MerchantID <= 0 {
			req.MerchantID = defaultGatewayMerchantID
		}
		initializer, err := requireProductInventoryInitializer(svcCtx)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, err)
			return
		}
		if svcCtx.ProductCommands == nil {
			failProductCommand(ctx, c, "admin product create failed", errProductCommandUnavailable)
			return
		}
		result, err := svcCtx.ProductCommands.Create(ctx, req)
		if err != nil {
			if reason := productCommandReason(err); reason != "" {
				recordGatewayAdminAuditFailure(c, svcCtx, adminAuditProductCreated, fmt.Sprintf("reason:%s", reason))
			}
			failProductCommand(ctx, c, "admin product create failed", err)
			return
		}
		if err = prepareProductVisibility(ctx, svcCtx, result.ProductID); err != nil {
			recordGatewayAdminAuditFailure(c, svcCtx, adminAuditProductCreated,
				fmt.Sprintf("product:%d reason:existence_filter_failed", result.ProductID))
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal,
				fmt.Sprintf("product %d created offline; publication preparation can be retried", result.ProductID), err))
			return
		}
		if _, err = initializer.Initialize(ctx, result.ProductID, inventoryRequestMeta(ctx)); err != nil {
			recordGatewayAdminAuditFailure(c, svcCtx, adminAuditProductCreated,
				fmt.Sprintf("product:%d reason:inventory_seed_failed", result.ProductID))
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeStockReconcileFailed,
				fmt.Sprintf("product %d created offline; inventory seed can be retried", result.ProductID), err))
			return
		}
		refreshProductCardSnapshotsBestEffort(ctx, svcCtx, result.ProductID)
		invalidateProductReadCaches(ctx, svcCtx, result.ProductID, req.MerchantID)
		recordGatewayAdminAuditEvent(c, svcCtx, adminAuditProductCreated,
			fmt.Sprintf("product:%d merchant:%d name:%s", result.ProductID, req.MerchantID, req.Name))
		ok(ctx, c, AdminProductCreateResp{ProductID: result.ProductID})
	}
}

func AdminProductStockAdjustHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
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
		inventoryRPC, err := requireInventoryClient(svcCtx)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, err)
			return
		}
		after, err := inventoryRPC.AdjustStock(ctx, req.ProductID, req.Delta, int(req.BucketIdx),
			"admin stock adjust", inventoryRequestMeta(ctx))
		if err != nil {
			writeStockAdjustError(ctx, c, svcCtx, req, err)
			return
		}
		refreshProductCardSnapshotsBestEffort(ctx, svcCtx, req.ProductID)
		invalidateProductReadCaches(ctx, svcCtx, req.ProductID, 0)
		recordGatewayAdminAuditEvent(c, svcCtx, adminAuditProductStockAdjusted,
			fmt.Sprintf("product:%d delta:%d bucket:%d", req.ProductID, req.Delta, req.BucketIdx))
		ok(ctx, c, AdminProductStockAdjustResp{ProductID: req.ProductID, StockAvailable: after.Total})
	}
}
