package handler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"flash-mall/app/common/apperror"
	"flash-mall/app/gateway/hertz/internal/inventoryclient"
	"flash-mall/app/gateway/hertz/internal/svc"
	"flash-mall/app/product/rpc/productclient"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/zeromicro/go-zero/core/logx"
)

const (
	defaultProductPage     int64 = 1
	defaultProductPageSize int64 = 20
	maxProductPageSize     int64 = 100
)

type productListQuery struct {
	Page       int64
	PageSize   int64
	Keyword    string
	ProductID  int64
	MerchantID int64
	SupplierID int64
	CategoryID int64
	Status     int64
}

func CatalogHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return productListHandler(svcCtx, true, true)
}

func ProductListHandler(svcCtx *svc.ServiceContext, activeOnly bool) app.HandlerFunc {
	return productListHandler(svcCtx, activeOnly, false)
}

func productListHandler(svcCtx *svc.ServiceContext, activeOnly bool, preferConfiguredCatalog bool) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		req, appErr := parseProductListQuery(c, activeOnly)
		if appErr != nil {
			fail(ctx, c, consts.StatusBadRequest, appErr)
			return
		}

		productIDs, total, err := productIDsForList(ctx, svcCtx, req, preferConfiguredCatalog)
		if err != nil {
			var appErr *apperror.Error
			if errors.As(err, &appErr) && appErr.Code == apperror.CodeInvalidArgument {
				fail(ctx, c, consts.StatusBadRequest, appErr)
				return
			}
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "product query failed", err))
			return
		}
		if len(productIDs) == 0 {
			ok(ctx, c, ProductListResp{Items: []ProductCard{}, Total: total, Page: req.Page, PageSize: req.PageSize})
			return
		}

		resp, err := svcCtx.ProductRpc.ListProducts(ctx, &productclient.ListProductsReq{ProductIds: productIDs})
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "product service unavailable", err))
			return
		}

		cards := buildProductCards(resp.Items, loadProductMeta(ctx, svcCtx, productIDs), loadCatalogInventoryStocks(ctx, svcCtx, productIDs))
		ok(ctx, c, ProductListResp{Items: orderProductCards(productIDs, cards), Total: total, Page: req.Page, PageSize: req.PageSize})
	}
}

func productIDsForList(ctx context.Context, svcCtx *svc.ServiceContext, req productListQuery, preferConfiguredCatalog bool) ([]int64, int64, error) {
	if preferConfiguredCatalog && req.hasNoFilters() && len(svcCtx.Config.CatalogProductIDs) > 0 {
		productIDs := append([]int64{}, svcCtx.Config.CatalogProductIDs...)
		return productIDs, int64(len(productIDs)), nil
	}
	return loadProductIDs(ctx, svcCtx, req)
}

func ProductDetailHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		productID, err := parsePositiveInt64(c.Query("product_id"))
		if err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "product_id required"))
			return
		}

		resp, err := svcCtx.ProductRpc.GetProductCard(ctx, &productclient.GetProductCardReq{ProductId: productID})
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "product service unavailable", err))
			return
		}
		if resp == nil || resp.ProductId == 0 {
			fail(ctx, c, consts.StatusNotFound, apperror.New(apperror.CodeProductNotFound, "product not found"))
			return
		}

		meta := loadProductMeta(ctx, svcCtx, []int64{productID})
		cards := buildProductCards([]*productclient.GetProductCardResp{resp}, meta, loadCatalogInventoryStocks(ctx, svcCtx, []int64{productID}))
		ok(ctx, c, map[string]any{"item": cards[productID]})
	}
}

func parseProductListQuery(c *app.RequestContext, activeOnly bool) (productListQuery, *apperror.Error) {
	page, err := parseInt64Default(c.Query("page"), defaultProductPage)
	if err != nil || page <= 0 {
		return productListQuery{}, apperror.New(apperror.CodeInvalidArgument, "page must be positive")
	}
	pageSize, err := parseInt64Default(c.Query("page_size"), defaultProductPageSize)
	if err != nil || pageSize <= 0 {
		return productListQuery{}, apperror.New(apperror.CodeInvalidArgument, "page_size must be positive")
	}
	if pageSize > maxProductPageSize {
		pageSize = maxProductPageSize
	}
	productID, err := parseOptionalInt64(c.Query("product_id"))
	if err != nil {
		return productListQuery{}, apperror.New(apperror.CodeInvalidArgument, "product_id must be numeric")
	}
	merchantID, err := parseOptionalInt64(c.Query("merchant_id"))
	if err != nil {
		return productListQuery{}, apperror.New(apperror.CodeInvalidArgument, "merchant_id must be numeric")
	}
	supplierID, err := parseOptionalInt64(c.Query("supplier_id"))
	if err != nil {
		return productListQuery{}, apperror.New(apperror.CodeInvalidArgument, "supplier_id must be numeric")
	}
	categoryID, err := parseOptionalInt64(c.Query("category_id"))
	if err != nil {
		return productListQuery{}, apperror.New(apperror.CodeInvalidArgument, "category_id must be numeric")
	}
	status := int64(1)
	if !activeOnly {
		status, err = parseInt64Default(c.Query("status"), -1)
		if err != nil {
			return productListQuery{}, apperror.New(apperror.CodeInvalidArgument, "status must be numeric")
		}
	}
	return productListQuery{
		Page:       page,
		PageSize:   pageSize,
		Keyword:    strings.TrimSpace(c.Query("keyword")),
		ProductID:  productID,
		MerchantID: merchantID,
		SupplierID: supplierID,
		CategoryID: categoryID,
		Status:     status,
	}, nil
}

func (q productListQuery) hasNoFilters() bool {
	return q.Keyword == "" &&
		q.ProductID == 0 &&
		q.MerchantID == 0 &&
		q.SupplierID == 0 &&
		q.CategoryID == 0
}

func loadProductIDs(ctx context.Context, svcCtx *svc.ServiceContext, req productListQuery) ([]int64, int64, error) {
	db, err := svcCtx.SqlConn.RawDB()
	if err != nil {
		return nil, 0, err
	}
	where, args, err := productWhereClause(ctx, db, req)
	if err != nil {
		return nil, 0, err
	}

	var total int64
	if err := db.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM mall_product.product p WHERE %s", where), args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}

	offset := (req.Page - 1) * req.PageSize
	queryArgs := append(append([]any{}, args...), req.PageSize, offset)
	rows, err := db.QueryContext(ctx, fmt.Sprintf("SELECT p.id FROM mall_product.product p WHERE %s ORDER BY p.id DESC LIMIT ? OFFSET ?", where), queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	productIDs := make([]int64, 0, req.PageSize)
	for rows.Next() {
		var productID int64
		if err := rows.Scan(&productID); err != nil {
			return nil, 0, err
		}
		productIDs = append(productIDs, productID)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return productIDs, total, nil
}

func productWhereClause(ctx context.Context, db *sql.DB, req productListQuery) (string, []any, error) {
	where := "1=1"
	args := make([]any, 0, 5)
	if req.Status >= 0 {
		where += " AND p.status = ?"
		args = append(args, req.Status)
	}
	if req.ProductID > 0 {
		where += " AND p.id = ?"
		args = append(args, req.ProductID)
	}
	if req.MerchantID > 0 {
		where += " AND p.merchant_id = ?"
		args = append(args, req.MerchantID)
	}
	if req.SupplierID > 0 {
		where += " AND p.supplier_id = ?"
		args = append(args, req.SupplierID)
	}
	if req.Keyword != "" {
		where += " AND p.name LIKE ?"
		args = append(args, "%"+req.Keyword+"%")
	}
	if req.CategoryID > 0 {
		ok, err := productColumnExists(ctx, db, "category_id")
		if err != nil {
			return "", nil, err
		}
		if !ok {
			return "", nil, apperror.New(apperror.CodeInvalidArgument, "category filter is not available")
		}
		where += " AND p.category_id = ?"
		args = append(args, req.CategoryID)
	}
	return where, args, nil
}

type productMeta struct {
	ImageURL     string
	SupplierName string
}

func loadProductMeta(ctx context.Context, svcCtx *svc.ServiceContext, productIDs []int64) map[int64]productMeta {
	result := make(map[int64]productMeta, len(productIDs))
	if len(productIDs) == 0 {
		return result
	}
	db, err := svcCtx.SqlConn.RawDB()
	if err != nil {
		logx.WithContext(ctx).Errorf("gateway product meta db failed: %v", err)
		return result
	}

	placeholders := make([]string, 0, len(productIDs))
	args := make([]any, 0, len(productIDs))
	for _, productID := range productIDs {
		placeholders = append(placeholders, "?")
		args = append(args, productID)
	}
	rows, err := db.QueryContext(ctx, fmt.Sprintf(`
SELECT p.id, COALESCE(p.image_url, ''), COALESCE(s.name, '')
FROM mall_product.product p
LEFT JOIN mall_product.supplier s ON s.id = p.supplier_id
WHERE p.id IN (%s)`, strings.Join(placeholders, ",")), args...)
	if err != nil {
		logx.WithContext(ctx).Errorf("gateway product meta query failed: %v", err)
		return result
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var productID int64
		var meta productMeta
		if err := rows.Scan(&productID, &meta.ImageURL, &meta.SupplierName); err == nil {
			result[productID] = meta
		}
	}
	return result
}

func buildProductCards(items []*productclient.GetProductCardResp, meta map[int64]productMeta, stocks map[int64]inventoryclient.Stock) map[int64]ProductCard {
	cards := make(map[int64]ProductCard, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		m := meta[item.ProductId]
		stockAvailable := item.StockAvailable
		var stockReserved int64
		var stockTotal int64
		stockSource := "product-rpc"
		if stock, ok := stocks[item.ProductId]; ok {
			stockAvailable = stock.Available
			stockReserved = stock.Reserved
			stockTotal = stock.Total
			stockSource = stockSourceInventoryKitex
		}
		cards[item.ProductId] = ProductCard{
			ProductID:      item.ProductId,
			Name:           item.Name,
			ImageURL:       m.ImageURL,
			OriginPriceFen: item.OriginPriceFen,
			FinalPriceFen:  item.FinalPriceFen,
			SupplierID:     item.SupplierId,
			SupplierName:   m.SupplierName,
			PromotionTag:   item.PromotionTag,
			StockAvailable: stockAvailable,
			StockReserved:  stockReserved,
			StockTotal:     stockTotal,
			StockSource:    stockSource,
		}
	}
	return cards
}

func loadCatalogInventoryStocks(ctx context.Context, svcCtx *svc.ServiceContext, productIDs []int64) map[int64]inventoryclient.Stock {
	result := make(map[int64]inventoryclient.Stock, len(productIDs))
	if !svcCtx.Config.EnableLiveStockOverlay || svcCtx.InventoryRpc == nil || len(productIDs) == 0 {
		return result
	}
	stocks, err := svcCtx.InventoryRpc.BatchGetStock(ctx, productIDs, inventoryRequestMeta(ctx))
	if err != nil {
		logx.WithContext(ctx).Errorf("gateway batch inventory stock query failed: count=%d err=%v", len(productIDs), err)
		return result
	}
	return stocks
}

func orderProductCards(productIDs []int64, cards map[int64]ProductCard) []ProductCard {
	ordered := make([]ProductCard, 0, len(cards))
	for _, productID := range productIDs {
		if card, ok := cards[productID]; ok {
			ordered = append(ordered, card)
		}
	}
	return ordered
}

func productColumnExists(ctx context.Context, db *sql.DB, column string) (bool, error) {
	var exists int64
	err := db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = 'mall_product'
  AND TABLE_NAME = 'product'
  AND COLUMN_NAME = ?`, column).Scan(&exists)
	return exists > 0, err
}

func parsePositiveInt64(raw string) (int64, error) {
	value, err := parseOptionalInt64(raw)
	if err != nil {
		return 0, err
	}
	if value <= 0 {
		return 0, strconv.ErrSyntax
	}
	return value, nil
}

func parseOptionalInt64(raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	return strconv.ParseInt(raw, 10, 64)
}

func parseInt64Default(raw string, fallback int64) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, nil
	}
	return strconv.ParseInt(raw, 10, 64)
}
