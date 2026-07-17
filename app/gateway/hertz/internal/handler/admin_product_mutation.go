package handler

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

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
		req.Name = strings.TrimSpace(req.Name)
		req.ImageURL = strings.TrimSpace(req.ImageURL)
		if req.ProductID <= 0 {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "product_id required"))
			return
		}

		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin product update failed", err))
			return
		}
		if err := requireGatewayProductImageColumn(ctx, db); err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "product image schema unavailable", err))
			return
		}

		setClauses := make([]string, 0, 6)
		args := make([]any, 0, 7)
		if req.Name != "" {
			setClauses = append(setClauses, "name = ?")
			args = append(args, req.Name)
		}
		if req.ImageURL != "" {
			setClauses = append(setClauses, "image_url = ?")
			args = append(args, req.ImageURL)
		}
		if req.SalePriceFen != nil {
			if *req.SalePriceFen < 0 {
				fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "sale_price_fen must be non-negative"))
				return
			}
			setClauses = append(setClauses, "sale_price_fen = ?")
			args = append(args, *req.SalePriceFen)
		}
		if req.OriginPriceFen != nil {
			if *req.OriginPriceFen < 0 {
				fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "origin_price_fen must be non-negative"))
				return
			}
			setClauses = append(setClauses, "origin_price_fen = ?")
			args = append(args, *req.OriginPriceFen)
		}
		if req.SupplierID != nil {
			if *req.SupplierID <= 0 {
				fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "supplier_id required"))
				return
			}
			exists, err := activeSupplierExists(ctx, db, *req.SupplierID)
			if err != nil {
				fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin product update failed", err))
				return
			}
			if !exists {
				fail(ctx, c, consts.StatusNotFound, apperror.New(apperror.CodeNotFound, "active supplier not found"))
				return
			}
			setClauses = append(setClauses, "supplier_id = ?")
			args = append(args, *req.SupplierID)
		}
		if req.Status != nil {
			if *req.Status != 1 && *req.Status != 2 {
				fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "status must be 1 or 2"))
				return
			}
			setClauses = append(setClauses, "status = ?")
			args = append(args, *req.Status)
		}
		if req.OriginPriceFen != nil || req.SalePriceFen != nil {
			validPrice, found, err := productPricePairValid(ctx, db, req.ProductID, req.OriginPriceFen, req.SalePriceFen)
			if err != nil {
				fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin product update failed", err))
				return
			}
			if !found {
				recordGatewayAdminAuditFailure(c, svcCtx, productUpdateAuditEvent(req.Status), fmt.Sprintf("product:%d reason:%s", req.ProductID, adminAuditReasonNotFound))
				fail(ctx, c, consts.StatusNotFound, apperror.New(apperror.CodeProductNotFound, "product not found"))
				return
			}
			if !validPrice {
				recordGatewayAdminAuditFailure(c, svcCtx, productUpdateAuditEvent(req.Status), fmt.Sprintf("product:%d reason:%s", req.ProductID, adminAuditReasonInvalidPrice))
				fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "sale_price_fen must be <= origin_price_fen"))
				return
			}
		}
		if len(setClauses) == 0 {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "no fields to update"))
			return
		}

		query := fmt.Sprintf("UPDATE mall_product.product SET %s WHERE id = ?", strings.Join(setClauses, ", "))
		args = append(args, req.ProductID)
		result, err := db.ExecContext(ctx, query, args...)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin product update failed", err))
			return
		}
		rows, err := result.RowsAffected()
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin product update failed", err))
			return
		}
		if rows == 0 {
			exists, err := productExists(ctx, db, req.ProductID)
			if err != nil {
				fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin product update failed", err))
				return
			}
			if !exists {
				recordGatewayAdminAuditFailure(c, svcCtx, productUpdateAuditEvent(req.Status), fmt.Sprintf("product:%d reason:%s", req.ProductID, adminAuditReasonNotFound))
				fail(ctx, c, consts.StatusNotFound, apperror.New(apperror.CodeProductNotFound, "product not found"))
				return
			}
		}

		refreshProductCardSnapshotsBestEffort(ctx, svcCtx, req.ProductID)
		invalidateProductReadCaches(ctx, svcCtx, req.ProductID, 0)
		recordGatewayAdminAuditEvent(c, svcCtx, productUpdateAuditEvent(req.Status), fmt.Sprintf("product:%d", req.ProductID))
		ok(ctx, c, map[string]any{"ok": true})
	}
}

func updateGatewayProductMetadata(ctx context.Context, db *sql.DB, req AdminProductUpdateReq) error {
	req.Name = strings.TrimSpace(req.Name)
	req.ImageURL = strings.TrimSpace(req.ImageURL)
	if req.ProductID <= 0 {
		return apperror.New(apperror.CodeInvalidArgument, "product_id required")
	}
	if err := requireGatewayProductImageColumn(ctx, db); err != nil {
		return apperror.Wrap(apperror.CodeInternal, "product image schema unavailable", err)
	}

	setClauses := make([]string, 0, 6)
	args := make([]any, 0, 7)
	if req.Name != "" {
		setClauses = append(setClauses, "name = ?")
		args = append(args, req.Name)
	}
	if req.ImageURL != "" {
		setClauses = append(setClauses, "image_url = ?")
		args = append(args, req.ImageURL)
	}
	if req.SalePriceFen != nil {
		if *req.SalePriceFen < 0 {
			return apperror.New(apperror.CodeInvalidArgument, "sale_price_fen must be non-negative")
		}
		setClauses = append(setClauses, "sale_price_fen = ?")
		args = append(args, *req.SalePriceFen)
	}
	if req.OriginPriceFen != nil {
		if *req.OriginPriceFen < 0 {
			return apperror.New(apperror.CodeInvalidArgument, "origin_price_fen must be non-negative")
		}
		setClauses = append(setClauses, "origin_price_fen = ?")
		args = append(args, *req.OriginPriceFen)
	}
	if req.SupplierID != nil {
		if *req.SupplierID <= 0 {
			return apperror.New(apperror.CodeInvalidArgument, "supplier_id required")
		}
		exists, err := activeSupplierExists(ctx, db, *req.SupplierID)
		if err != nil {
			return err
		}
		if !exists {
			return apperror.New(apperror.CodeNotFound, "active supplier not found")
		}
		setClauses = append(setClauses, "supplier_id = ?")
		args = append(args, *req.SupplierID)
	}
	if req.Status != nil {
		if *req.Status != 1 && *req.Status != 2 {
			return apperror.New(apperror.CodeInvalidArgument, "status must be 1 or 2")
		}
		setClauses = append(setClauses, "status = ?")
		args = append(args, *req.Status)
	}
	if req.OriginPriceFen != nil || req.SalePriceFen != nil {
		validPrice, found, err := productPricePairValid(ctx, db, req.ProductID, req.OriginPriceFen, req.SalePriceFen)
		if err != nil {
			return err
		}
		if !found {
			return apperror.New(apperror.CodeProductNotFound, "product not found")
		}
		if !validPrice {
			return apperror.New(apperror.CodeInvalidArgument, "sale_price_fen must be <= origin_price_fen")
		}
	}
	if len(setClauses) == 0 {
		return apperror.New(apperror.CodeInvalidArgument, "no fields to update")
	}

	query := fmt.Sprintf("UPDATE mall_product.product SET %s WHERE id = ?", strings.Join(setClauses, ", "))
	args = append(args, req.ProductID)
	result, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		exists, err := productExists(ctx, db, req.ProductID)
		if err != nil {
			return err
		}
		if !exists {
			return apperror.New(apperror.CodeProductNotFound, "product not found")
		}
	}
	return nil
}

func AdminProductCreateHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
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
		if req.MerchantID <= 0 {
			req.MerchantID = defaultGatewayMerchantID
		}
		initializer, err := requireProductInventoryInitializer(svcCtx)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, err)
			return
		}

		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin product create failed", err))
			return
		}
		if err := requireGatewayProductMerchantSchema(ctx, db); err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "product schema unavailable", err))
			return
		}

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin product create failed", err))
			return
		}
		defer func() { _ = tx.Rollback() }()

		supplierExists, err := activeSupplierExists(ctx, tx, req.SupplierID)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin product create failed", err))
			return
		}
		if !supplierExists {
			recordGatewayAdminAuditFailure(c, svcCtx, adminAuditProductCreated, fmt.Sprintf("supplier:%d reason:%s", req.SupplierID, adminAuditReasonActiveSupplierNotFound))
			fail(ctx, c, consts.StatusNotFound, apperror.New(apperror.CodeNotFound, "active supplier not found"))
			return
		}
		merchantExists, err := activeMerchantExists(ctx, tx, req.MerchantID)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin product create failed", err))
			return
		}
		if !merchantExists {
			recordGatewayAdminAuditFailure(c, svcCtx, adminAuditProductCreated, fmt.Sprintf("merchant:%d reason:%s", req.MerchantID, adminAuditReasonNotFound))
			fail(ctx, c, consts.StatusNotFound, apperror.New(apperror.CodeNotFound, "active merchant not found"))
			return
		}

		productID, err := nextProductID(ctx, tx)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin product create failed", err))
			return
		}
		desiredStatus := req.Status
		if _, err = tx.ExecContext(ctx,
			"INSERT INTO mall_product.product (id, merchant_id, name, image_url, stock, version, origin_price_fen, sale_price_fen, status, supplier_id) VALUES (?, ?, ?, ?, ?, 0, ?, ?, ?, ?)",
			productID, req.MerchantID, req.Name, req.ImageURL, req.StockAvailable, req.OriginPriceFen, req.SalePriceFen, int64(2), req.SupplierID,
		); err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin product create failed", err))
			return
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO mall_product.product_inventory_seed
(product_id, desired_total, shard_count, desired_product_status, status)
VALUES (?, ?, 4, ?, 0)`, productID, req.StockAvailable, desiredStatus); err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin product seed task create failed", err))
			return
		}
		if err = tx.Commit(); err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin product create failed", err))
			return
		}

		if _, err := initializer.Initialize(ctx, productID, inventoryRequestMeta(ctx)); err != nil {
			recordGatewayAdminAuditFailure(c, svcCtx, adminAuditProductCreated, fmt.Sprintf("product:%d reason:inventory_seed_failed", productID))
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeStockReconcileFailed, fmt.Sprintf("product %d created offline; inventory seed can be retried", productID), err))
			return
		}
		refreshProductCardSnapshotsBestEffort(ctx, svcCtx, productID)
		invalidateProductReadCaches(ctx, svcCtx, productID, req.MerchantID)
		recordGatewayAdminAuditEvent(c, svcCtx, adminAuditProductCreated, fmt.Sprintf("product:%d merchant:%d name:%s", productID, req.MerchantID, req.Name))
		ok(ctx, c, AdminProductCreateResp{ProductID: productID})
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

		inventoryRpc, err := requireInventoryClient(svcCtx)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, err)
			return
		}

		after, err := inventoryRpc.AdjustStock(ctx, req.ProductID, req.Delta, int(req.BucketIdx), "admin stock adjust", inventoryRequestMeta(ctx))
		if err != nil {
			writeStockAdjustError(ctx, c, svcCtx, req, err)
			return
		}
		refreshProductCardSnapshotsBestEffort(ctx, svcCtx, req.ProductID)
		invalidateProductReadCaches(ctx, svcCtx, req.ProductID, 0)
		recordGatewayAdminAuditEvent(c, svcCtx, adminAuditProductStockAdjusted, fmt.Sprintf("product:%d delta:%d bucket:%d", req.ProductID, req.Delta, req.BucketIdx))
		ok(ctx, c, AdminProductStockAdjustResp{ProductID: req.ProductID, StockAvailable: after.Total})
	}
}
