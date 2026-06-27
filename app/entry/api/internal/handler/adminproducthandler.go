package handler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"flash-mall/app/entry/api/internal/svc"
	"flash-mall/app/entry/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func AdminProductListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminProductListReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		if req.Page <= 0 {
			req.Page = 1
		}
		if req.PageSize <= 0 || req.PageSize > 100 {
			req.PageSize = 20
		}
		offset := (req.Page - 1) * req.PageSize

		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if err := ensureProductImageColumn(r.Context(), db); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		where := "1=1"
		args := []interface{}{}
		if req.ProductId > 0 {
			where += " AND p.id = ?"
			args = append(args, req.ProductId)
		}
		if req.SupplierId > 0 {
			where += " AND p.supplier_id = ?"
			args = append(args, req.SupplierId)
		}
		if req.Status >= 0 {
			where += " AND p.status = ?"
			args = append(args, req.Status)
		}
		if req.PromotionStatus == 1 {
			where += ` AND EXISTS (
				SELECT 1 FROM mall_product.promotion_rule pr_filter
				WHERE pr_filter.product_id = p.id
				  AND pr_filter.type = 'LIMITED_PRICE'
				  AND pr_filter.status = 1
				  AND (pr_filter.starts_at IS NULL OR pr_filter.starts_at <= NOW())
				  AND (pr_filter.ends_at IS NULL OR pr_filter.ends_at >= NOW())
			)`
		} else if req.PromotionStatus == 2 {
			where += ` AND NOT EXISTS (
				SELECT 1 FROM mall_product.promotion_rule pr_filter
				WHERE pr_filter.product_id = p.id
				  AND pr_filter.type = 'LIMITED_PRICE'
				  AND pr_filter.status = 1
				  AND (pr_filter.starts_at IS NULL OR pr_filter.starts_at <= NOW())
				  AND (pr_filter.ends_at IS NULL OR pr_filter.ends_at >= NOW())
			)`
		}
		stockStatus := normalizeAdminProductStockStatus(req.StockStatus)
		if stockStatus == 1 {
			where += " AND COALESCE((SELECT SUM(stock) FROM mall_product.product_stock_bucket b_filter WHERE b_filter.product_id = p.id), 0) > 100"
		} else if stockStatus == 2 {
			where += " AND COALESCE((SELECT SUM(stock) FROM mall_product.product_stock_bucket b_filter WHERE b_filter.product_id = p.id), 0) > 0 AND COALESCE((SELECT SUM(stock) FROM mall_product.product_stock_bucket b_filter WHERE b_filter.product_id = p.id), 0) <= 100"
		} else if stockStatus == 3 {
			where += " AND COALESCE((SELECT SUM(stock) FROM mall_product.product_stock_bucket b_filter WHERE b_filter.product_id = p.id), 0) = 0"
		}
		if keyword := strings.TrimSpace(req.Keyword); keyword != "" {
			where += " AND p.name LIKE ?"
			args = append(args, "%"+keyword+"%")
		}

		var total int64
		countQuery := fmt.Sprintf("SELECT COUNT(*) FROM mall_product.product p WHERE %s", where)
		if err := db.QueryRowContext(r.Context(), countQuery, args...).Scan(&total); err != nil {
			logx.WithContext(r.Context()).Errorf("admin product count query failed: %v", err)
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		query := fmt.Sprintf(
			`SELECT p.id, p.name, COALESCE(p.image_url, ''), p.origin_price_fen, p.sale_price_fen, p.supplier_id, COALESCE(s.name, ''),
			        COALESCE(stock.stock_available, 0),
			        COALESCE(promo.promotion_price_fen, 0),
			        p.status
			 FROM mall_product.product p
			 LEFT JOIN mall_product.supplier s ON s.id = p.supplier_id
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
		queryArgs := append(append([]interface{}{}, args...), req.PageSize, offset)
		rows, err := db.QueryContext(r.Context(), query, queryArgs...)
		if err != nil {
			logx.WithContext(r.Context()).Errorf("admin product list query failed: %v", err)
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		defer func() { _ = rows.Close() }()

		items := make([]types.AdminProductItem, 0)
		for rows.Next() {
			var item types.AdminProductItem
			if err := rows.Scan(&item.ProductId, &item.Name, &item.ImageUrl, &item.OriginPriceFen, &item.SalePriceFen, &item.SupplierId, &item.SupplierName, &item.StockAvailable, &item.PromotionPriceFen, &item.Status); err != nil {
				logx.WithContext(r.Context()).Errorf("admin product list scan failed: %v", err)
				httpx.ErrorCtx(r.Context(), w, err)
				return
			}
			item.StatusText = adminProductStatusText(item.Status)
			applyAdminProductPromotionText(&item)
			items = append(items, item)
		}
		if err := rows.Err(); err != nil {
			logx.WithContext(r.Context()).Errorf("admin product list rows failed: %v", err)
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		httpx.OkJsonCtx(r.Context(), w, types.AdminProductListResp{Items: items, Total: total})
	}
}

func normalizeAdminProductStockStatus(value string) int64 {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "enough", "normal", "sufficient":
		return 1
	case "2", "low", "low_stock":
		return 2
	case "3", "out", "out_of_stock", "empty":
		return 3
	default:
		return -1
	}
}

func AdminProductDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		productID, err := parsePositiveInt64Query(r, "product_id")
		if err != nil {
			writeBadRequest(w, "product_id required")
			return
		}

		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if err := ensureProductImageColumn(r.Context(), db); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		var item types.AdminProductItem
		err = db.QueryRowContext(r.Context(),
			`SELECT p.id, p.name, COALESCE(p.image_url, ''), p.origin_price_fen, p.sale_price_fen, p.supplier_id, COALESCE(s.name, ''),
			        COALESCE(stock.stock_available, 0),
			        COALESCE(promo.promotion_price_fen, 0),
			        p.status
			 FROM mall_product.product p
			 LEFT JOIN mall_product.supplier s ON s.id = p.supplier_id
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
			 WHERE p.id = ?`,
			productID,
			productID,
			productID,
		).Scan(&item.ProductId, &item.Name, &item.ImageUrl, &item.OriginPriceFen, &item.SalePriceFen, &item.SupplierId, &item.SupplierName, &item.StockAvailable, &item.PromotionPriceFen, &item.Status)
		if err != nil {
			writeNotFound(w, "product not found")
			return
		}
		item.StatusText = adminProductStatusText(item.Status)
		applyAdminProductPromotionText(&item)
		httpx.OkJsonCtx(r.Context(), w, item)
	}
}

func AdminProductUpdateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ProductId      int64  `json:"product_id"`
			Name           string `json:"name,optional"`             //nolint:staticcheck // go-zero httpx.Parse uses optional in json tags.
			ImageUrl       string `json:"image_url,optional"`        //nolint:staticcheck // go-zero httpx.Parse uses optional in json tags.
			SalePriceFen   *int64 `json:"sale_price_fen,optional"`   //nolint:staticcheck // go-zero httpx.Parse uses optional in json tags.
			OriginPriceFen *int64 `json:"origin_price_fen,optional"` //nolint:staticcheck // go-zero httpx.Parse uses optional in json tags.
			SupplierId     *int64 `json:"supplier_id,optional"`      //nolint:staticcheck // go-zero httpx.Parse uses optional in json tags.
			Status         *int64 `json:"status,optional"`           //nolint:staticcheck // go-zero httpx.Parse uses optional in json tags.
		}
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		req.Name = strings.TrimSpace(req.Name)
		req.ImageUrl = strings.TrimSpace(req.ImageUrl)
		if req.ProductId == 0 {
			writeBadRequest(w, "product_id required")
			return
		}

		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if err := ensureProductImageColumn(r.Context(), db); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		setClauses := ""
		args := []interface{}{}
		if req.Name != "" {
			setClauses += "name = ?"
			args = append(args, req.Name)
		}
		if req.ImageUrl != "" {
			if setClauses != "" {
				setClauses += ", "
			}
			setClauses += "image_url = ?"
			args = append(args, req.ImageUrl)
		}
		if req.SalePriceFen != nil {
			if *req.SalePriceFen < 0 {
				writeBadRequest(w, "sale_price_fen must be non-negative")
				return
			}
			if setClauses != "" {
				setClauses += ", "
			}
			setClauses += "sale_price_fen = ?"
			args = append(args, *req.SalePriceFen)
		}
		if req.OriginPriceFen != nil {
			if *req.OriginPriceFen < 0 {
				writeBadRequest(w, "origin_price_fen must be non-negative")
				return
			}
			if setClauses != "" {
				setClauses += ", "
			}
			setClauses += "origin_price_fen = ?"
			args = append(args, *req.OriginPriceFen)
		}
		if req.SupplierId != nil {
			if *req.SupplierId <= 0 {
				writeBadRequest(w, "supplier_id required")
				return
			}
			exists, err := adminActiveSupplierExists(r.Context(), db, *req.SupplierId)
			if err != nil {
				httpx.ErrorCtx(r.Context(), w, err)
				return
			}
			if !exists {
				writeNotFound(w, "active supplier not found")
				return
			}
			if setClauses != "" {
				setClauses += ", "
			}
			setClauses += "supplier_id = ?"
			args = append(args, *req.SupplierId)
		}
		if req.Status != nil {
			if *req.Status != 1 && *req.Status != 2 {
				writeBadRequest(w, "status must be 1 or 2")
				return
			}
			if setClauses != "" {
				setClauses += ", "
			}
			setClauses += "status = ?"
			args = append(args, *req.Status)
		}

		if req.OriginPriceFen != nil || req.SalePriceFen != nil {
			validPrice, found, err := adminProductPricePairValid(r.Context(), db, req.ProductId, req.OriginPriceFen, req.SalePriceFen)
			if err != nil {
				httpx.ErrorCtx(r.Context(), w, err)
				return
			}
			if !found {
				recordAdminAuditFailure(r, svcCtx, adminProductUpdateAuditEvent(req.Status), fmt.Sprintf("product:%d reason:%s", req.ProductId, adminAuditReasonNotFound))
				writeNotFound(w, "product not found")
				return
			}
			if !validPrice {
				recordAdminAuditFailure(r, svcCtx, adminProductUpdateAuditEvent(req.Status), fmt.Sprintf("product:%d reason:%s", req.ProductId, adminAuditReasonInvalidPrice))
				writeBadRequest(w, "sale_price_fen must be <= origin_price_fen")
				return
			}
		}

		if setClauses == "" {
			writeBadRequest(w, "no fields to update")
			return
		}

		query := fmt.Sprintf("UPDATE mall_product.product SET %s WHERE id = ?", setClauses)
		args = append(args, req.ProductId)
		result, err := db.ExecContext(r.Context(), query, args...)
		if err != nil {
			logx.WithContext(r.Context()).Errorf("admin product update failed: %v", err)
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		rows, err := result.RowsAffected()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if rows == 0 {
			var exists int64
			if err := db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM mall_product.product WHERE id = ?", req.ProductId).Scan(&exists); err != nil {
				httpx.ErrorCtx(r.Context(), w, err)
				return
			}
			if exists == 0 {
				recordAdminAuditFailure(r, svcCtx, adminProductUpdateAuditEvent(req.Status), fmt.Sprintf("product:%d reason:%s", req.ProductId, adminAuditReasonNotFound))
				writeNotFound(w, "product not found")
				return
			}
		}

		invalidateAdminCatalogCache(r.Context(), svcCtx)
		recordAdminAuditEvent(r, svcCtx, adminProductUpdateAuditEvent(req.Status), fmt.Sprintf("product:%d", req.ProductId))
		httpx.OkJsonCtx(r.Context(), w, map[string]any{"ok": true})
	}
}

func AdminProductCreateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminProductCreateReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		req.Name = strings.TrimSpace(req.Name)
		req.ImageUrl = strings.TrimSpace(req.ImageUrl)
		if req.Name == "" || req.OriginPriceFen < 0 || req.SalePriceFen < 0 || req.StockAvailable < 0 || req.SupplierId <= 0 {
			writeBadRequest(w, "name, non-negative prices, stock and supplier_id are required")
			return
		}
		if req.SalePriceFen > req.OriginPriceFen {
			writeBadRequest(w, "sale_price_fen must be <= origin_price_fen")
			return
		}
		if req.Status == 0 {
			req.Status = 1
		}
		if req.Status != 1 && req.Status != 2 {
			writeBadRequest(w, "status must be 1 or 2")
			return
		}

		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if err := ensureProductImageColumn(r.Context(), db); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		defer func() { _ = tx.Rollback() }()

		supplierExists, err := adminActiveSupplierExists(r.Context(), tx, req.SupplierId)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if !supplierExists {
			recordAdminAuditFailure(r, svcCtx, adminAuditProductCreated, fmt.Sprintf("supplier:%d reason:%s", req.SupplierId, adminAuditReasonActiveSupplierNotFound))
			writeNotFound(w, "active supplier not found")
			return
		}

		productID, err := nextAdminProductID(r.Context(), tx)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if _, err = tx.ExecContext(r.Context(),
			"INSERT INTO mall_product.product (id, name, image_url, stock, version, origin_price_fen, sale_price_fen, status, supplier_id) VALUES (?, ?, ?, 0, 0, ?, ?, ?, ?)",
			productID, req.Name, req.ImageUrl, req.OriginPriceFen, req.SalePriceFen, req.Status, req.SupplierId,
		); err != nil {
			logx.WithContext(r.Context()).Errorf("admin product create failed: %v", err)
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if req.StockAvailable > 0 {
			if err = insertAdminProductStockBuckets(r.Context(), tx, productID, req.StockAvailable); err != nil {
				httpx.ErrorCtx(r.Context(), w, err)
				return
			}
		}
		if err = tx.Commit(); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		invalidateAdminCatalogCache(r.Context(), svcCtx)
		recordAdminAuditEvent(r, svcCtx, adminAuditProductCreated, fmt.Sprintf("product:%d name:%s", productID, req.Name))
		httpx.OkJsonCtx(r.Context(), w, types.AdminProductCreateResp{ProductId: productID})
	}
}

func AdminProductStockAdjustHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminProductStockAdjustReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if req.ProductId <= 0 || req.Delta == 0 {
			writeBadRequest(w, "product_id and non-zero delta are required")
			return
		}
		if req.BucketIdx < 0 {
			writeBadRequest(w, "bucket_idx must be non-negative")
			return
		}

		db, err := svcCtx.SqlConn.RawDB()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		defer func() { _ = tx.Rollback() }()

		exists, err := adminProductExists(r.Context(), tx, req.ProductId)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if !exists {
			recordAdminAuditFailure(r, svcCtx, adminAuditProductStockAdjusted, fmt.Sprintf("product:%d reason:%s", req.ProductId, adminAuditReasonNotFound))
			writeNotFound(w, "product not found")
			return
		}

		var result sql.Result
		if req.Delta > 0 {
			result, err = tx.ExecContext(r.Context(),
				"INSERT INTO mall_product.product_stock_bucket (product_id, bucket_idx, stock, version) VALUES (?, ?, ?, 0) ON DUPLICATE KEY UPDATE stock = stock + VALUES(stock), version = version + 1",
				req.ProductId, req.BucketIdx, req.Delta,
			)
		} else {
			result, err = tx.ExecContext(r.Context(),
				"UPDATE mall_product.product_stock_bucket SET stock = stock + ?, version = version + 1 WHERE product_id = ? AND bucket_idx = ? AND stock + ? >= 0",
				req.Delta, req.ProductId, req.BucketIdx, req.Delta,
			)
		}
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		rows, err := result.RowsAffected()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if rows == 0 {
			recordAdminAuditFailure(r, svcCtx, adminAuditProductStockAdjusted, fmt.Sprintf("product:%d delta:%d bucket:%d reason:%s", req.ProductId, req.Delta, req.BucketIdx, adminAuditReasonInsufficientOrMissingStock))
			writeConflict(w, "stock bucket not found or insufficient stock")
			return
		}

		total, err := adminProductStockTotal(r.Context(), tx, req.ProductId)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if err = tx.Commit(); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		invalidateAdminCatalogCache(r.Context(), svcCtx)
		recordAdminAuditEvent(r, svcCtx, adminAuditProductStockAdjusted, fmt.Sprintf("product:%d delta:%d bucket:%d", req.ProductId, req.Delta, req.BucketIdx))
		httpx.OkJsonCtx(r.Context(), w, types.AdminProductStockAdjustResp{
			ProductId:      req.ProductId,
			StockAvailable: total,
		})
	}
}

func nextAdminProductID(ctx context.Context, tx *sql.Tx) (int64, error) {
	var maxProductID int64
	if err := tx.QueryRowContext(ctx, "SELECT id FROM mall_product.product ORDER BY id DESC LIMIT 1 FOR UPDATE").Scan(&maxProductID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 100, nil
		}
		return 0, err
	}
	return maxProductID + 1, nil
}

func adminProductStockTotal(ctx context.Context, tx *sql.Tx, productID int64) (int64, error) {
	var total int64
	if err := tx.QueryRowContext(ctx, "SELECT COALESCE(SUM(stock), 0) FROM mall_product.product_stock_bucket WHERE product_id = ?", productID).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func insertAdminProductStockBuckets(ctx context.Context, tx *sql.Tx, productID int64, totalStock int64) error {
	const bucketCount = 4
	perBucket := totalStock / bucketCount
	remain := totalStock % bucketCount
	for bucketIdx := int64(0); bucketIdx < bucketCount; bucketIdx++ {
		stock := perBucket
		if bucketIdx == 0 {
			stock += remain
		}
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO mall_product.product_stock_bucket (product_id, bucket_idx, stock, version) VALUES (?, ?, ?, 0)",
			productID, bucketIdx, stock,
		); err != nil {
			return err
		}
	}
	return nil
}

func adminProductPricePairValid(ctx context.Context, q adminQueryRower, productID int64, originPriceFen, salePriceFen *int64) (bool, bool, error) {
	var origin int64
	var sale int64
	if originPriceFen == nil || salePriceFen == nil {
		if err := q.QueryRowContext(ctx, "SELECT origin_price_fen, sale_price_fen FROM mall_product.product WHERE id = ?", productID).Scan(&origin, &sale); err != nil {
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

func adminProductExists(ctx context.Context, tx *sql.Tx, productID int64) (bool, error) {
	var exists int64
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM mall_product.product WHERE id = ?", productID).Scan(&exists); err != nil {
		return false, err
	}
	return exists > 0, nil
}

func adminProductStatusText(status int64) string {
	switch status {
	case 1:
		return "active"
	case 2:
		return "inactive"
	default:
		return "unknown"
	}
}

func adminProductUpdateAuditEvent(status *int64) string {
	if status == nil {
		return adminAuditProductUpdated
	}
	if *status == 1 {
		return adminAuditProductEnabled
	}
	return adminAuditProductDisabled
}

func applyAdminProductPromotionText(item *types.AdminProductItem) {
	if item.PromotionPriceFen <= 0 {
		return
	}
	item.PromotionType = "LIMITED_PRICE"
	item.PromotionTag = "限时价"
}
