package handler

import (
	"context"
	"errors"

	"flash-mall/app/common/apperror"
	"flash-mall/app/gateway/hertz/internal/existencefilter"
	"flash-mall/app/gateway/hertz/internal/svc"
	"flash-mall/app/product/rpc/productclient"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func CatalogHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return ShowcaseCatalogHandler(svcCtx)
}

func ProductListHandler(svcCtx *svc.ServiceContext, activeOnly bool) app.HandlerFunc {
	return productListHandler(svcCtx, activeOnly)
}

func productListHandler(svcCtx *svc.ServiceContext, activeOnly bool) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		req, appErr := parseProductListQuery(c, activeOnly)
		if appErr != nil {
			fail(ctx, c, consts.StatusBadRequest, appErr)
			return
		}

		productIDs, total, err := loadProductIDs(ctx, svcCtx, req)
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

func ProductDetailHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		productID, err := parsePositiveInt64(c.Query("product_id"))
		if err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "product_id required"))
			return
		}

		detail, _, err := loadCachedJSON(ctx, svcCtx, productDetailCacheKey(productID), func(loadCtx context.Context) (ProductDetailResp, error) {
			filterPossible, guardErr := protectProductDetailOrigin(loadCtx, svcCtx, productID)
			if guardErr != nil {
				return ProductDetailResp{}, guardErr
			}
			service, loadErr := catalogQueryService(svcCtx)
			if loadErr != nil {
				return ProductDetailResp{}, apperror.Wrap(apperror.CodeInternal, "product datasource unavailable", loadErr)
			}
			meta, loadErr := service.ProductMetadata(loadCtx, []int64{productID})
			if loadErr != nil {
				return ProductDetailResp{}, apperror.Wrap(apperror.CodeInternal, "product metadata query failed", loadErr)
			}
			mainMeta, exists := meta[productID]
			if !exists || !productMetaPubliclyVisible(mainMeta) {
				if !exists && filterPossible {
					existencefilter.RecordFalsePositive()
				}
				markProductNotPublic(loadCtx, svcCtx, productID)
				return ProductDetailResp{}, productNotFound()
			}
			relatedPage, loadErr := service.StoreProductIDs(loadCtx, mainMeta.MerchantID, "", 1, 5)
			if loadErr != nil {
				return ProductDetailResp{}, apperror.Wrap(apperror.CodeInternal, "related product query failed", loadErr)
			}
			relatedIDs := relatedPage.ProductIDs
			relatedIDs = excludeProductID(relatedIDs, productID, 4)
			allIDs := append([]int64{productID}, relatedIDs...)
			resp, rpcErr := svcCtx.ProductRpc.ListProducts(loadCtx, &productclient.ListProductsReq{ProductIds: allIDs})
			if rpcErr != nil {
				return ProductDetailResp{}, apperror.Wrap(apperror.CodeInternal, "product service unavailable", rpcErr)
			}
			cards := buildProductCards(resp.Items, loadProductMeta(loadCtx, svcCtx, allIDs), nil)
			detail, exists := buildProductDetailResp(productID, relatedIDs, cards)
			if !exists {
				return ProductDetailResp{}, productNotFound()
			}
			return detail, nil
		})
		if apperror.CodeOf(err) == apperror.CodeProductNotFound {
			fail(ctx, c, consts.StatusNotFound, err)
			return
		}
		if err != nil {
			fail(ctx, c, consts.StatusBadGateway, err)
			return
		}
		ok(ctx, c, detail)
	}
}

func excludeProductID(ids []int64, excluded int64, limit int) []int64 {
	result := make([]int64, 0, limit)
	for _, id := range ids {
		if id == excluded {
			continue
		}
		result = append(result, id)
		if len(result) == limit {
			break
		}
	}
	return result
}

func loadProductIDs(ctx context.Context, svcCtx *svc.ServiceContext, req productListQuery) ([]int64, int64, error) {
	service, err := catalogQueryService(svcCtx)
	if err != nil {
		return nil, 0, err
	}
	page, err := service.ProductIDs(ctx, req)
	if err != nil {
		return nil, 0, err
	}
	return page.ProductIDs, page.Total, nil
}
