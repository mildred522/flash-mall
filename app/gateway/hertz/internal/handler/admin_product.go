package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"flash-mall/app/common/apperror"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

const defaultGatewayMerchantID int64 = 1000

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
		if err := ensureGatewayProductReadTables(ctx, db); err != nil {
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
			rows, err := rebuildGatewayProductCardSnapshots(ctx, db, productID, 1)
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
		if err := ensureGatewayProductImageColumn(ctx, db); err != nil {
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
	if err := ensureGatewayProductImageColumn(ctx, db); err != nil {
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
		inventoryRpc, err := requireInventoryClient(svcCtx)
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, err)
			return
		}

		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin product create failed", err))
			return
		}
		if err := ensureGatewayProductMerchantSchema(ctx, db); err != nil {
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
		if _, err = tx.ExecContext(ctx,
			"INSERT INTO mall_product.product (id, merchant_id, name, image_url, stock, version, origin_price_fen, sale_price_fen, status, supplier_id) VALUES (?, ?, ?, ?, ?, 0, ?, ?, ?, ?)",
			productID, req.MerchantID, req.Name, req.ImageURL, req.StockAvailable, req.OriginPriceFen, req.SalePriceFen, req.Status, req.SupplierID,
		); err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin product create failed", err))
			return
		}
		if err = tx.Commit(); err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin product create failed", err))
			return
		}

		if err := inventoryRpc.SeedStock(ctx, productID, req.StockAvailable, 4, inventoryRequestMeta(ctx)); err != nil {
			recordGatewayAdminAuditFailure(c, svcCtx, adminAuditProductCreated, fmt.Sprintf("product:%d reason:inventory_seed_failed", productID))
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin product stock seed failed", err))
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

func loadAdminProducts(ctx context.Context, svcCtx *svc.ServiceContext, req productListQuery) ([]AdminProductItem, int64, error) {
	db, err := svcCtx.SqlConn.RawDB()
	if err != nil {
		return nil, 0, err
	}
	if err := ensureGatewayProductReadTables(ctx, db); err != nil {
		return nil, 0, err
	}
	where, args, err := productWhereClause(ctx, db, req)
	if err != nil {
		return nil, 0, err
	}

	var total int64
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM mall_product.product p WHERE %s", where)
	if err := db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []AdminProductItem{}, 0, nil
	}

	offset := (req.Page - 1) * req.PageSize
	query := fmt.Sprintf(`
SELECT p.id, p.merchant_id, COALESCE(m.name, ''), p.name, COALESCE(p.image_url, ''),
       p.origin_price_fen, p.sale_price_fen, p.supplier_id, COALESCE(s.name, ''),
       COALESCE(snap.available, stock.stock_available, 0),
       COALESCE(promo.promotion_price_fen, 0),
       p.status
FROM mall_product.product p
LEFT JOIN mall_order.merchant m ON m.id = p.merchant_id
LEFT JOIN mall_product.supplier s ON s.id = p.supplier_id
LEFT JOIN mall_product.product_stock_snapshot snap ON snap.product_id = p.id
LEFT JOIN (
  SELECT product_id, COALESCE(SUM(stock), 0) AS stock_available
  FROM mall_product.product_stock_bucket
  GROUP BY product_id
) stock ON stock.product_id = p.id
LEFT JOIN (
  SELECT product_id, MIN(discount_value) AS promotion_price_fen
  FROM mall_product.promotion_rule
  WHERE type = 'LIMITED_PRICE'
    AND status = 1
    AND (starts_at IS NULL OR starts_at <= NOW())
    AND (ends_at IS NULL OR ends_at >= NOW())
  GROUP BY product_id
) promo ON promo.product_id = p.id
WHERE %s
ORDER BY p.id DESC LIMIT ? OFFSET ?`, where)

	queryArgs := append(append([]any{}, args...), req.PageSize, offset)
	rows, err := db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]AdminProductItem, 0, req.PageSize)
	for rows.Next() {
		var item AdminProductItem
		if err := rows.Scan(
			&item.ProductID,
			&item.MerchantID,
			&item.MerchantName,
			&item.Name,
			&item.ImageURL,
			&item.OriginPriceFen,
			&item.SalePriceFen,
			&item.SupplierID,
			&item.SupplierName,
			&item.StockAvailable,
			&item.PromotionPriceFen,
			&item.Status,
		); err != nil {
			return nil, 0, err
		}
		item.StatusText = productStatusText(item.Status)
		item.PromotionText = productPromotionText(item.PromotionPriceFen)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func loadAdminProductDetail(ctx context.Context, svcCtx *svc.ServiceContext, productID int64) (AdminProductItem, error) {
	db, err := svcCtx.SqlConn.RawDB()
	if err != nil {
		return AdminProductItem{}, err
	}
	if err := ensureGatewayProductReadTables(ctx, db); err != nil {
		return AdminProductItem{}, err
	}

	var item AdminProductItem
	err = db.QueryRowContext(ctx, `
SELECT p.id, p.merchant_id, COALESCE(m.name, ''), p.name, COALESCE(p.image_url, ''),
       p.origin_price_fen, p.sale_price_fen, p.supplier_id, COALESCE(s.name, ''),
       COALESCE(snap.available, stock.stock_available, 0),
       COALESCE(promo.promotion_price_fen, 0),
       p.status
FROM mall_product.product p
LEFT JOIN mall_order.merchant m ON m.id = p.merchant_id
LEFT JOIN mall_product.supplier s ON s.id = p.supplier_id
LEFT JOIN mall_product.product_stock_snapshot snap ON snap.product_id = p.id
LEFT JOIN (
  SELECT product_id, COALESCE(SUM(stock), 0) AS stock_available
  FROM mall_product.product_stock_bucket
  WHERE product_id = ?
  GROUP BY product_id
) stock ON stock.product_id = p.id
LEFT JOIN (
  SELECT product_id, MIN(discount_value) AS promotion_price_fen
  FROM mall_product.promotion_rule
  WHERE product_id = ?
    AND type = 'LIMITED_PRICE'
    AND status = 1
    AND (starts_at IS NULL OR starts_at <= NOW())
    AND (ends_at IS NULL OR ends_at >= NOW())
  GROUP BY product_id
) promo ON promo.product_id = p.id
WHERE p.id = ?`, productID, productID, productID).Scan(
		&item.ProductID,
		&item.MerchantID,
		&item.MerchantName,
		&item.Name,
		&item.ImageURL,
		&item.OriginPriceFen,
		&item.SalePriceFen,
		&item.SupplierID,
		&item.SupplierName,
		&item.StockAvailable,
		&item.PromotionPriceFen,
		&item.Status,
	)
	if err != nil {
		return AdminProductItem{}, err
	}
	item.StatusText = productStatusText(item.Status)
	item.PromotionText = productPromotionText(item.PromotionPriceFen)
	return item, nil
}

func productStatusText(status int64) string {
	switch status {
	case 1:
		return "上架"
	case 2:
		return "下架"
	default:
		return "未知"
	}
}

func productPromotionText(priceFen int64) string {
	if priceFen > 0 {
		return "限时价"
	}
	return "无活动"
}

func ensureGatewayProductReadTables(ctx context.Context, db *sql.DB) error {
	if err := ensureGatewayProductStockSnapshotTable(ctx, db); err != nil {
		return err
	}
	return ensureGatewayProductCardSnapshotTable(ctx, db)
}

func ensureGatewayProductMerchantSchema(ctx context.Context, db *sql.DB) error {
	if err := ensureGatewayMerchantBaseSchema(ctx, db); err != nil {
		return err
	}
	if err := ensureGatewayProductMerchantColumn(ctx, db); err != nil {
		return err
	}
	if err := ensureGatewayProductImageColumn(ctx, db); err != nil {
		return err
	}
	return ensureGatewayProductReadTables(ctx, db)
}

func ensureGatewayMerchantBaseSchema(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS merchant (
  id bigint NOT NULL AUTO_INCREMENT,
  name varchar(128) NOT NULL DEFAULT '',
  owner_user_id bigint NOT NULL DEFAULT 0,
  status tinyint NOT NULL DEFAULT 1,
  contact_phone varchar(32) NOT NULL DEFAULT '',
  create_time timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  update_time timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY ix_status (status),
  KEY ix_owner_user_id (owner_user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`); err != nil {
		return err
	}
	_, err := db.ExecContext(ctx, "INSERT INTO merchant (id, name, owner_user_id, status, contact_phone) VALUES (?, ?, 0, 1, '') ON DUPLICATE KEY UPDATE name = VALUES(name), status = VALUES(status)", defaultGatewayMerchantID, "Flash Mall 自营店")
	return err
}

func ensureGatewayProductMerchantColumn(ctx context.Context, db *sql.DB) error {
	var exists int64
	err := db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = 'mall_product'
  AND TABLE_NAME = 'product'
  AND COLUMN_NAME = 'merchant_id'`).Scan(&exists)
	if err != nil {
		return err
	}
	if exists > 0 {
		return nil
	}
	_, err = db.ExecContext(ctx, "ALTER TABLE mall_product.product ADD COLUMN merchant_id bigint NOT NULL DEFAULT 1000 AFTER id")
	return err
}

func ensureGatewayProductImageColumn(ctx context.Context, db *sql.DB) error {
	var exists int64
	err := db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = 'mall_product'
  AND TABLE_NAME = 'product'
  AND COLUMN_NAME = 'image_url'`).Scan(&exists)
	if err != nil {
		return err
	}
	if exists > 0 {
		return nil
	}
	_, err = db.ExecContext(ctx, "ALTER TABLE mall_product.product ADD COLUMN image_url varchar(512) NOT NULL DEFAULT '' AFTER name")
	return err
}

func ensureGatewayProductStockSnapshotTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS mall_product.product_stock_snapshot (
  product_id bigint NOT NULL,
  available bigint NOT NULL DEFAULT 0,
  reserved bigint NOT NULL DEFAULT 0,
  total bigint NOT NULL DEFAULT 0,
  source varchar(32) NOT NULL DEFAULT 'inventory-kitex',
  version bigint NOT NULL DEFAULT 0,
  update_time timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (product_id),
  KEY ix_update_time (update_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`)
	return err
}

func ensureGatewayProductCardSnapshotTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS mall_product.product_card_snapshot (
  product_id bigint NOT NULL,
  name varchar(255) NOT NULL DEFAULT '',
  origin_price_fen bigint NOT NULL DEFAULT 0,
  final_price_fen bigint NOT NULL DEFAULT 0,
  promotion_type varchar(32) NOT NULL DEFAULT '',
  promotion_tag varchar(32) NOT NULL DEFAULT '',
  stock_available bigint NOT NULL DEFAULT 0,
  supplier_id bigint NOT NULL DEFAULT 0,
  status tinyint NOT NULL DEFAULT 1,
  version bigint NOT NULL DEFAULT 0,
  update_time timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (product_id),
  KEY ix_status_product (status, product_id),
  KEY ix_update_time (update_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`)
	return err
}

func rebuildGatewayProductCardSnapshots(ctx context.Context, db *sql.DB, productID int64, limit int64) (int64, error) {
	where := "1=1"
	args := make([]any, 0, 2)
	if productID > 0 {
		where += " AND p.id = ?"
		args = append(args, productID)
	}
	args = append(args, limit)
	result, err := db.ExecContext(ctx, fmt.Sprintf(`
INSERT INTO mall_product.product_card_snapshot (product_id, name, origin_price_fen, final_price_fen, promotion_type, promotion_tag, stock_available, supplier_id, status, version)
SELECT p.id,
       p.name,
       p.origin_price_fen,
       CASE WHEN promo.promotion_price_fen > 0 THEN promo.promotion_price_fen ELSE p.sale_price_fen END AS final_price_fen,
       CASE WHEN promo.promotion_price_fen > 0 THEN 'LIMITED_PRICE' ELSE '' END AS promotion_type,
       CASE WHEN promo.promotion_price_fen > 0 THEN '限时价' ELSE '' END AS promotion_tag,
       COALESCE(stock.available, bucket.stock_available, 0) AS stock_available,
       p.supplier_id,
       p.status,
       0 AS version
FROM mall_product.product p
LEFT JOIN mall_product.product_stock_snapshot stock ON stock.product_id = p.id
LEFT JOIN (
  SELECT product_id, COALESCE(SUM(stock), 0) AS stock_available
  FROM mall_product.product_stock_bucket
  GROUP BY product_id
) bucket ON bucket.product_id = p.id
LEFT JOIN (
  SELECT product_id, MIN(discount_value) AS promotion_price_fen
  FROM mall_product.promotion_rule
  WHERE type = 'LIMITED_PRICE'
    AND status = 1
    AND (starts_at IS NULL OR starts_at <= NOW())
    AND (ends_at IS NULL OR ends_at >= NOW())
  GROUP BY product_id
) promo ON promo.product_id = p.id
WHERE %s
ORDER BY p.id
LIMIT ?
ON DUPLICATE KEY UPDATE
  name = VALUES(name),
  origin_price_fen = VALUES(origin_price_fen),
  final_price_fen = VALUES(final_price_fen),
  promotion_type = VALUES(promotion_type),
  promotion_tag = VALUES(promotion_tag),
  stock_available = VALUES(stock_available),
  supplier_id = VALUES(supplier_id),
  status = VALUES(status),
  version = version + 1`, where), args...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func gatewayPromotionWindowAffectedProductIDs(ctx context.Context, db *sql.DB, windowMinutes int64, limit int64) ([]int64, error) {
	now := time.Now()
	window := time.Duration(windowMinutes) * time.Minute
	rows, err := db.QueryContext(ctx, `
SELECT DISTINCT product_id
FROM mall_product.promotion_rule
WHERE status = 1
  AND (
    ((starts_at IS NULL OR starts_at <= NOW()) AND (ends_at IS NULL OR ends_at >= NOW()))
    OR (starts_at IS NOT NULL AND starts_at BETWEEN ? AND ?)
    OR (ends_at IS NOT NULL AND ends_at BETWEEN ? AND ?)
  )
ORDER BY product_id
LIMIT ?`, now.Add(-window), now.Add(window), now.Add(-window), now.Add(window), limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	productIDs := make([]int64, 0)
	for rows.Next() {
		var productID int64
		if err := rows.Scan(&productID); err != nil {
			return nil, err
		}
		if productID > 0 {
			productIDs = append(productIDs, productID)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return productIDs, nil
}

func uniquePositiveInt64s(values []int64) []int64 {
	seen := make(map[int64]struct{}, len(values))
	result := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

type sqlQueryRower interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func activeSupplierExists(ctx context.Context, q sqlQueryRower, supplierID int64) (bool, error) {
	var exists int64
	if err := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM mall_product.supplier WHERE id = ? AND status = 1", supplierID).Scan(&exists); err != nil {
		return false, err
	}
	return exists > 0, nil
}

func activeMerchantExists(ctx context.Context, q sqlQueryRower, merchantID int64) (bool, error) {
	var exists int64
	if err := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM merchant WHERE id = ? AND status = 1", merchantID).Scan(&exists); err != nil {
		return false, err
	}
	return exists > 0, nil
}

func nextProductID(ctx context.Context, tx *sql.Tx) (int64, error) {
	var maxProductID int64
	if err := tx.QueryRowContext(ctx, "SELECT id FROM mall_product.product ORDER BY id DESC LIMIT 1 FOR UPDATE").Scan(&maxProductID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 100, nil
		}
		return 0, err
	}
	return maxProductID + 1, nil
}

func writeStockAdjustError(ctx context.Context, c *app.RequestContext, svcCtx *svc.ServiceContext, req AdminProductStockAdjustReq, err error) {
	switch apperror.CodeOf(err) {
	case apperror.CodeStockNotFound, apperror.CodeProductNotFound, apperror.CodeNotFound:
		recordGatewayAdminAuditFailure(c, svcCtx, adminAuditProductStockAdjusted, fmt.Sprintf("product:%d reason:%s", req.ProductID, adminAuditReasonNotFound))
		fail(ctx, c, consts.StatusNotFound, apperror.New(apperror.CodeProductNotFound, "product not found"))
	case apperror.CodeStockInsufficient:
		recordGatewayAdminAuditFailure(c, svcCtx, adminAuditProductStockAdjusted, fmt.Sprintf("product:%d delta:%d bucket:%d reason:%s", req.ProductID, req.Delta, req.BucketIdx, adminAuditReasonInsufficientOrMissingStock))
		fail(ctx, c, consts.StatusConflict, apperror.New(apperror.CodeStockInsufficient, "stock bucket not found or insufficient stock"))
	default:
		fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin product stock adjust failed", err))
	}
}

func productPricePairValid(ctx context.Context, db *sql.DB, productID int64, originPriceFen, salePriceFen *int64) (bool, bool, error) {
	var origin int64
	var sale int64
	if originPriceFen == nil || salePriceFen == nil {
		if err := db.QueryRowContext(ctx, "SELECT origin_price_fen, sale_price_fen FROM mall_product.product WHERE id = ?", productID).Scan(&origin, &sale); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return false, false, nil
			}
			return false, false, err
		}
	}
	if originPriceFen != nil {
		origin = *originPriceFen
	}
	if salePriceFen != nil {
		sale = *salePriceFen
	}
	return sale <= origin, true, nil
}
