package handler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"flash-mall/app/common/apperror"
	"flash-mall/app/common/authctx"
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

		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "merchant product query failed", err))
			return
		}

		merchantID, err := selectedMerchantID(ctx, db, identity)
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

func MerchantProductCreateHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity, hasIdentity := authctx.IdentityFrom(ctx)
		if !hasIdentity || identity.UserID <= 0 {
			fail(ctx, c, consts.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "merchant login required"))
			return
		}

		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "merchant product create failed", err))
			return
		}
		if err := ensureGatewayProductMerchantSchema(ctx, db); err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "product schema unavailable", err))
			return
		}
		merchantID, err := selectedMerchantID(ctx, db, identity)
		if err != nil {
			fail(ctx, c, consts.StatusForbidden, err)
			return
		}

		var req AdminProductCreateReq
		if err := decodeJSONBody(c, &req); err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid product create request"))
			return
		}
		req.Name = strings.TrimSpace(req.Name)
		req.ImageURL = strings.TrimSpace(req.ImageURL)
		if req.Name == "" || req.OriginPriceFen < 0 || req.SalePriceFen < 0 || req.StockAvailable < 0 || req.SupplierID <= 0 {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "name, non-negative prices, stock and supplier_id are required"))
			return
		}
		if req.SalePriceFen > req.OriginPriceFen {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "sale_price_fen must be <= origin_price_fen"))
			return
		}
		if req.Status == 0 {
			req.Status = 1
		}
		if req.Status != 1 && req.Status != 2 {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "status must be 1 or 2"))
			return
		}
		inventoryRpc, err := requireInventoryClient(svcCtx)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, err)
			return
		}

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "merchant product create failed", err))
			return
		}
		defer func() { _ = tx.Rollback() }()
		supplierExists, err := activeSupplierExists(ctx, tx, req.SupplierID)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "merchant product create failed", err))
			return
		}
		if !supplierExists {
			fail(ctx, c, consts.StatusNotFound, apperror.New(apperror.CodeNotFound, "active supplier not found"))
			return
		}
		productID, err := nextProductID(ctx, tx)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "merchant product create failed", err))
			return
		}
		if _, err = tx.ExecContext(ctx,
			"INSERT INTO mall_product.product (id, merchant_id, name, image_url, stock, version, origin_price_fen, sale_price_fen, status, supplier_id) VALUES (?, ?, ?, ?, ?, 0, ?, ?, ?, ?)",
			productID, merchantID, req.Name, req.ImageURL, req.StockAvailable, req.OriginPriceFen, req.SalePriceFen, req.Status, req.SupplierID,
		); err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "merchant product create failed", err))
			return
		}
		if err = tx.Commit(); err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "merchant product create failed", err))
			return
		}

		identity.MerchantID = merchantID
		callCtx := authctx.WithIdentity(ctx, identity)
		if err := inventoryRpc.SeedStock(callCtx, productID, req.StockAvailable, 4, inventoryRequestMeta(callCtx)); err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "merchant product stock seed failed", err))
			return
		}
		refreshProductCardSnapshotsBestEffort(ctx, svcCtx, productID)
		ok(ctx, c, AdminProductCreateResp{ProductID: productID})
	}
}

func MerchantProductUpdateHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity, hasIdentity := authctx.IdentityFrom(ctx)
		if !hasIdentity || identity.UserID <= 0 {
			fail(ctx, c, consts.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "merchant login required"))
			return
		}

		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "merchant product update failed", err))
			return
		}
		if err := ensureGatewayProductMerchantSchema(ctx, db); err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "product schema unavailable", err))
			return
		}
		merchantID, err := selectedMerchantID(ctx, db, identity)
		if err != nil {
			fail(ctx, c, consts.StatusForbidden, err)
			return
		}

		var req AdminProductUpdateReq
		if err := decodeJSONBody(c, &req); err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "invalid product update request"))
			return
		}
		if req.ProductID <= 0 {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "product_id required"))
			return
		}
		owns, err := merchantOwnsGatewayProduct(ctx, db, merchantID, req.ProductID)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "merchant product update failed", err))
			return
		}
		if !owns {
			fail(ctx, c, consts.StatusNotFound, apperror.New(apperror.CodeProductNotFound, "product not found for merchant"))
			return
		}

		if err := updateGatewayProductMetadata(ctx, db, req); err != nil {
			fail(ctx, c, productMutationStatusCode(err), err)
			return
		}
		refreshProductCardSnapshotsBestEffort(ctx, svcCtx, req.ProductID)
		ok(ctx, c, map[string]any{"ok": true})
	}
}

func MerchantProductStockAdjustHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity, hasIdentity := authctx.IdentityFrom(ctx)
		if !hasIdentity || identity.UserID <= 0 {
			fail(ctx, c, consts.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "merchant login required"))
			return
		}
		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "merchant product stock adjust failed", err))
			return
		}
		if err := ensureGatewayProductMerchantSchema(ctx, db); err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "product schema unavailable", err))
			return
		}
		merchantID, err := selectedMerchantID(ctx, db, identity)
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
		owns, err := merchantOwnsGatewayProduct(ctx, db, merchantID, req.ProductID)
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
		ok(ctx, c, AdminProductStockAdjustResp{ProductID: req.ProductID, StockAvailable: after.Total})
	}
}

func productMutationStatusCode(err error) int {
	switch apperror.CodeOf(err) {
	case apperror.CodeInvalidArgument:
		return consts.StatusBadRequest
	case apperror.CodeProductNotFound, apperror.CodeNotFound:
		return consts.StatusNotFound
	default:
		return consts.StatusBadGateway
	}
}

func selectedMerchantID(ctx context.Context, db *sql.DB, identity authctx.Identity) (int64, error) {
	if identity.CanAdmin() {
		if identity.MerchantID > 0 {
			return identity.MerchantID, nil
		}
		return defaultMerchantID(ctx, db)
	}
	if identity.MerchantID > 0 {
		allowed, err := userCanAccessMerchant(ctx, db, identity.UserID, identity.MerchantID)
		if err != nil {
			return 0, err
		}
		if !allowed {
			return 0, apperror.New(apperror.CodeForbidden, "merchant access denied")
		}
		return identity.MerchantID, nil
	}
	return firstMerchantID(ctx, db, identity.UserID)
}

func merchantOwnsGatewayProduct(ctx context.Context, db *sql.DB, merchantID int64, productID int64) (bool, error) {
	var exists int64
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mall_product.product WHERE id = ? AND merchant_id = ?", productID, merchantID).Scan(&exists)
	return exists > 0, err
}

func firstMerchantID(ctx context.Context, db *sql.DB, userID int64) (int64, error) {
	var merchantID int64
	err := db.QueryRowContext(ctx, `
SELECT merchant_id
FROM merchant_user
WHERE user_id = ? AND status = 1
ORDER BY id ASC
LIMIT 1`, userID).Scan(&merchantID)
	if err == nil {
		return merchantID, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return 0, apperror.New(apperror.CodeMerchantNotBound, "merchant access required")
	}
	return 0, err
}

func userCanAccessMerchant(ctx context.Context, db *sql.DB, userID, merchantID int64) (bool, error) {
	var exists int64
	err := db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM merchant_user
WHERE user_id = ? AND merchant_id = ? AND status = 1`, userID, merchantID).Scan(&exists)
	return exists > 0, err
}

func defaultMerchantID(ctx context.Context, db *sql.DB) (int64, error) {
	var merchantID int64
	err := db.QueryRowContext(ctx, `
SELECT id
FROM merchant
WHERE status = 1
ORDER BY id ASC
LIMIT 1`).Scan(&merchantID)
	if err == nil {
		return merchantID, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return 0, apperror.New(apperror.CodeMerchantNotFound, "active merchant not found")
	}
	return 0, fmt.Errorf("default merchant query failed: %w", err)
}
