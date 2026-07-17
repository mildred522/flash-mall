package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"flash-mall/app/common/apperror"
	"flash-mall/app/gateway/hertz/internal/ports"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func AdminProductListHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		req, appErr := parseProductListQuery(c, false)
		if appErr != nil {
			fail(ctx, c, consts.StatusBadRequest, appErr)
			return
		}

		items, total, err := loadAdminProducts(ctx, svcCtx, req)
		if err != nil {
			var appErr *apperror.Error
			if errors.As(err, &appErr) && appErr.Code == apperror.CodeInvalidArgument {
				fail(ctx, c, consts.StatusBadRequest, appErr)
				return
			}
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin product query failed", err))
			return
		}
		ok(ctx, c, AdminProductListResp{Items: items, Total: total, Page: req.Page, PageSize: req.PageSize})
	}
}

func AdminProductDetailHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		productID, err := parsePositiveInt64(c.Query("product_id"))
		if err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "product_id required"))
			return
		}

		item, err := loadAdminProductDetail(ctx, svcCtx, productID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				fail(ctx, c, consts.StatusNotFound, apperror.New(apperror.CodeProductNotFound, "product not found"))
				return
			}
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin product detail query failed", err))
			return
		}
		ok(ctx, c, item)
	}
}

func AdminProductCardSnapshotRefreshHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var req AdminProductCardSnapshotRefreshReq
		if body, err := c.Body(); err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid snapshot refresh request"))
			return
		} else if len(body) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid snapshot refresh request"))
				return
			}
		}
		if req.ProductID < 0 {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "product_id must be positive"))
			return
		}
		if req.Limit <= 0 || req.Limit > 10000 {
			req.Limit = 1000
		}
		if req.WindowMinutes <= 0 || req.WindowMinutes > 24*60 {
			req.WindowMinutes = 120
		}

		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "product snapshot refresh failed", err))
			return
		}
		if err := requireGatewayProductReadSchema(ctx, db); err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "product snapshot schema unavailable", err))
			return
		}

		productIDs := []int64{req.ProductID}
		if req.ProductID == 0 {
			productIDs, err = gatewayPromotionWindowAffectedProductIDs(ctx, db, req.WindowMinutes, req.Limit)
			if err != nil {
				fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "promotion window scan failed", err))
				return
			}
		}
		productIDs = uniquePositiveInt64s(productIDs)

		var affected int64
		for _, productID := range productIDs {
			store, storeErr := requireProductSnapshotStore(svcCtx)
			if storeErr != nil {
				fail(ctx, c, consts.StatusBadGateway, storeErr)
				return
			}
			rows, err := store.RebuildCards(ctx, ports.SnapshotRebuildRequest{ProductID: productID, Limit: 1})
			if err != nil {
				fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "product card snapshot rebuild failed", err))
				return
			}
			affected += rows
			invalidateProductReadCaches(ctx, svcCtx, productID, 0)
		}
		ok(ctx, c, AdminProductCardSnapshotRefreshResp{
			ProductCount:  int64(len(productIDs)),
			Affected:      affected,
			Limit:         req.Limit,
			WindowMinutes: req.WindowMinutes,
		})
	}
}
