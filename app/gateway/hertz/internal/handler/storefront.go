package handler

import (
	"context"
	"time"

	"flash-mall/app/common/apperror"
	"flash-mall/app/gateway/hertz/internal/svc"
	"flash-mall/app/product/rpc/productclient"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

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
			service, serviceErr := catalogQueryService(svcCtx)
			if serviceErr != nil {
				return PublicStoreDetail{}, apperror.Wrap(apperror.CodeInternal, "product datasource unavailable", serviceErr)
			}
			return service.StoreDetail(loadCtx, merchantID)
		})
		if apperror.CodeOf(err) == apperror.CodeMerchantNotFound {
			fail(ctx, c, consts.StatusNotFound, err)
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
			service, serviceErr := catalogQueryService(svcCtx)
			if serviceErr != nil {
				return StoreProductListResp{}, apperror.Wrap(apperror.CodeInternal, "product datasource unavailable", serviceErr)
			}
			if _, serviceErr = service.StoreDetail(loadCtx, merchantID); serviceErr != nil {
				return StoreProductListResp{}, serviceErr
			}
			idPage, loadErr := service.StoreProductIDs(loadCtx, merchantID, keyword, page, pageSize)
			if loadErr != nil {
				return StoreProductListResp{}, apperror.Wrap(apperror.CodeInternal, "store product query failed", loadErr)
			}
			ids, total := idPage.ProductIDs, idPage.Total
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
		if apperror.CodeOf(err) == apperror.CodeMerchantNotFound {
			fail(ctx, c, consts.StatusNotFound, err)
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
