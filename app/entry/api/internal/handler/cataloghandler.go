package handler

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"flash-mall/app/entry/api/internal/metrics"
	"flash-mall/app/entry/api/internal/svc"
	"flash-mall/app/entry/api/internal/types"
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

		imageURLs := loadCatalogImageURLs(ctx, svcCtx, resp.Items)
		items := make([]types.ProductCard, 0, len(resp.Items))
		for _, card := range resp.Items {
			items = append(items, types.ProductCard{
				ProductId:      card.ProductId,
				Name:           card.Name,
				ImageUrl:       imageURLs[card.ProductId],
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

func loadCatalogImageURLs(ctx context.Context, svcCtx *svc.ServiceContext, cards []*productclient.GetProductCardResp) map[int64]string {
	result := make(map[int64]string, len(cards))
	if len(cards) == 0 {
		return result
	}
	if svcCtx == nil || svcCtx.SqlConn == nil {
		return result
	}
	db, err := svcCtx.SqlConn.RawDB()
	if err != nil {
		logx.WithContext(ctx).Errorf("catalog image db failed: %v", err)
		return result
	}
	if err := ensureProductImageColumn(ctx, db); err != nil {
		logx.WithContext(ctx).Errorf("catalog image column check failed: %v", err)
		return result
	}
	placeholders := make([]string, 0, len(cards))
	args := make([]any, 0, len(cards))
	for _, card := range cards {
		placeholders = append(placeholders, "?")
		args = append(args, card.ProductId)
	}
	rows, err := db.QueryContext(ctx,
		fmt.Sprintf("SELECT id, COALESCE(image_url, '') FROM mall_product.product WHERE id IN (%s)", strings.Join(placeholders, ",")),
		args...,
	)
	if err != nil {
		logx.WithContext(ctx).Errorf("catalog image query failed: %v", err)
		return result
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var productID int64
		var imageURL string
		if err := rows.Scan(&productID, &imageURL); err == nil {
			result[productID] = imageURL
		}
	}
	return result
}
