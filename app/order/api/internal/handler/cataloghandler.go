package handler

import (
	"net/http"
	"time"

	"flash-mall/app/order/api/internal/metrics"
	"flash-mall/app/order/api/internal/svc"
	"flash-mall/app/order/api/internal/types"
	"flash-mall/app/product/rpc/productclient"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func CatalogHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		t0 := time.Now()

		// Try cache first
		if svcCtx.CatalogCache != nil {
			if cached := svcCtx.CatalogCache.Get(ctx); cached != nil {
				metrics.CatalogCacheHitTotal.WithLabelValues("hit").Inc()
				metrics.CatalogRequestTotal.WithLabelValues("ok").Inc()
				metrics.CatalogRequestDuration.Observe(time.Since(t0).Seconds())
				httpx.OkJsonCtx(ctx, w, map[string]any{"items": cached})
				return
			}
			metrics.CatalogCacheHitTotal.WithLabelValues("miss").Inc()
		}

		productIDs := svcCtx.Config.CatalogProductIDs
		resp, err := svcCtx.ProductRpc.ListProducts(ctx, &productclient.ListProductsReq{
			ProductIds: productIDs,
		})
		if err != nil {
			logx.WithContext(ctx).Errorf("ListProducts RPC failed: %v", err)
			metrics.CatalogRequestTotal.WithLabelValues("error").Inc()
			metrics.CatalogRequestDuration.Observe(time.Since(t0).Seconds())
			httpx.ErrorCtx(ctx, w, err)
			return
		}

		items := make([]types.ProductCard, 0, len(resp.Items))
		for _, card := range resp.Items {
			items = append(items, types.ProductCard{
				ProductId:      card.ProductId,
				Name:           card.Name,
				OriginPriceFen: card.OriginPriceFen,
				FinalPriceFen:  card.FinalPriceFen,
				PromotionTag:   card.PromotionTag,
				StockAvailable: card.StockAvailable,
			})
		}

		// Write cache async
		if svcCtx.CatalogCache != nil {
			go svcCtx.CatalogCache.Set(ctx, items)
		}

		metrics.CatalogRequestTotal.WithLabelValues("ok").Inc()
		metrics.CatalogRequestDuration.Observe(time.Since(t0).Seconds())
		httpx.OkJsonCtx(ctx, w, map[string]any{"items": items})
	}
}
