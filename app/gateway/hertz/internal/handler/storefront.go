package handler

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"flash-mall/app/common/apperror"
	"flash-mall/app/gateway/hertz/internal/svc"
	"flash-mall/app/product/rpc/productclient"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func loadPublicStoreDetail(ctx context.Context, db *sql.DB, merchantID int64) (PublicStoreDetail, error) {
	var detail PublicStoreDetail
	err := db.QueryRowContext(ctx, `
SELECT m.id, m.name,
       COALESCE(profile.logo_url, ''), COALESCE(profile.banner_url, ''),
       COALESCE(profile.description, ''), m.status,
       (SELECT COUNT(*) FROM mall_product.product p WHERE p.merchant_id = m.id AND p.status = 1)
FROM mall_order.merchant m
LEFT JOIN mall_order.merchant_store_profile profile ON profile.merchant_id = m.id
WHERE m.id = ? AND m.status = 1`, merchantID).Scan(
		&detail.MerchantID,
		&detail.MerchantName,
		&detail.LogoURL,
		&detail.BannerURL,
		&detail.Description,
		&detail.Status,
		&detail.ProductCount,
	)
	return detail, err
}

func loadStoreProductIDs(ctx context.Context, db *sql.DB, merchantID int64, keyword string, page, pageSize int64) ([]int64, int64, error) {
	where := "p.merchant_id = ? AND p.status = ?"
	args := []any{merchantID, int64(1)}
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		where += " AND p.name LIKE ?"
		args = append(args, "%"+keyword+"%")
	}
	var total int64
	if err := db.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM mall_product.product p WHERE %s", where), args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []int64{}, 0, nil
	}
	offset := (page - 1) * pageSize
	queryArgs := append(append([]any{}, args...), pageSize, offset)
	rows, err := db.QueryContext(ctx, fmt.Sprintf(`SELECT p.id FROM mall_product.product p
WHERE %s ORDER BY p.create_time DESC, p.id DESC LIMIT ? OFFSET ?`, where), queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	ids := make([]int64, 0, pageSize)
	for rows.Next() {
		var productID int64
		if err := rows.Scan(&productID); err != nil {
			return nil, 0, err
		}
		ids = append(ids, productID)
	}
	return ids, total, rows.Err()
}

func productMetaPubliclyVisible(meta productMeta) bool {
	return meta.ProductStatus == 1 && meta.StoreStatus == 1
}

func buildProductDetailResp(productID int64, relatedIDs []int64, cards map[int64]ProductCard) (ProductDetailResp, bool) {
	item, ok := cards[productID]
	if !ok {
		return ProductDetailResp{}, false
	}
	related := make([]ProductCard, 0, len(relatedIDs))
	for _, relatedID := range relatedIDs {
		if card, exists := cards[relatedID]; exists {
			related = append(related, card)
		}
	}
	return ProductDetailResp{Item: item, StoreProducts: related}, true
}

func StoreDetailHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		startedAt := time.Now()
		result := "error"
		defer func() { recordStoreRequest("detail", result, time.Since(startedAt)) }()
		merchantID, err := parsePositiveInt64(c.Query("merchant_id"))
		if err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "merchant_id required"))
			return
		}
		detail, _, err := loadCachedJSON(ctx, svcCtx, storeDetailCacheKey(merchantID), func(loadCtx context.Context) (PublicStoreDetail, error) {
			db, dbErr := svcCtx.SqlConn.RawDB()
			if dbErr != nil {
				return PublicStoreDetail{}, apperror.Wrap(apperror.CodeInternal, "product datasource unavailable", dbErr)
			}
			if dbErr = requireMerchantStoreProfileSchema(loadCtx, db); dbErr != nil {
				return PublicStoreDetail{}, apperror.Wrap(apperror.CodeInternal, "merchant store schema unavailable", dbErr)
			}
			return loadPublicStoreDetail(loadCtx, db, merchantID)
		})
		if err == sql.ErrNoRows {
			fail(ctx, c, consts.StatusNotFound, apperror.New(apperror.CodeMerchantNotFound, "merchant store not found"))
			return
		}
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "merchant store query failed", err))
			return
		}
		result = "success"
		ok(ctx, c, detail)
	}
}

func StoreProductListHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		startedAt := time.Now()
		result := "error"
		defer func() { recordStoreRequest("products", result, time.Since(startedAt)) }()
		merchantID, err := parsePositiveInt64(c.Query("merchant_id"))
		if err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "merchant_id required"))
			return
		}
		page, err := parseInt64Default(c.Query("page"), 1)
		if err != nil || page <= 0 {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "page must be positive"))
			return
		}
		pageSize, err := parseInt64Default(c.Query("page_size"), 20)
		if err != nil || pageSize <= 0 {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "page_size must be positive"))
			return
		}
		if pageSize > 100 {
			pageSize = 100
		}
		keyword := c.Query("keyword")
		products, _, err := loadCachedJSON(ctx, svcCtx, storeProductsCacheKey(merchantID, page, pageSize, keyword), func(loadCtx context.Context) (StoreProductListResp, error) {
			db, dbErr := svcCtx.SqlConn.RawDB()
			if dbErr != nil {
				return StoreProductListResp{}, apperror.Wrap(apperror.CodeInternal, "product datasource unavailable", dbErr)
			}
			if _, dbErr = loadPublicStoreDetail(loadCtx, db, merchantID); dbErr != nil {
				return StoreProductListResp{}, dbErr
			}
			ids, total, loadErr := loadStoreProductIDs(loadCtx, db, merchantID, keyword, page, pageSize)
			if loadErr != nil {
				return StoreProductListResp{}, apperror.Wrap(apperror.CodeInternal, "store product query failed", loadErr)
			}
			if len(ids) == 0 {
				return StoreProductListResp{Items: []ProductCard{}, Total: total, Page: page, PageSize: pageSize}, nil
			}
			resp, rpcErr := svcCtx.ProductRpc.ListProducts(loadCtx, &productclient.ListProductsReq{ProductIds: ids})
			if rpcErr != nil {
				return StoreProductListResp{}, apperror.Wrap(apperror.CodeInternal, "product service unavailable", rpcErr)
			}
			cards := buildProductCards(resp.Items, loadProductMeta(loadCtx, svcCtx, ids), nil)
			return StoreProductListResp{Items: orderProductCards(ids, cards), Total: total, Page: page, PageSize: pageSize}, nil
		})
		if err == sql.ErrNoRows {
			fail(ctx, c, consts.StatusNotFound, apperror.New(apperror.CodeMerchantNotFound, "merchant store not found"))
			return
		}
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, err)
			return
		}
		result = "success"
		ok(ctx, c, products)
	}
}
