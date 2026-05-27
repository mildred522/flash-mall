package handler

import (
	"net/http"

	"flash-mall/app/order/api/internal/svc"
	"flash-mall/app/order/api/internal/types"
	"flash-mall/app/product/rpc/productclient"

	"github.com/zeromicro/go-zero/rest/httpx"
)

func CatalogHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		productIDs := svcCtx.Config.CatalogProductIDs
		if len(productIDs) == 0 {
			productIDs = []int64{100}
		}
		items := make([]types.ProductCard, 0, len(productIDs))
		for _, productID := range productIDs {
			card, err := svcCtx.ProductRpc.GetProductCard(r.Context(), &productclient.GetProductCardReq{
				ProductId: productID,
			})
			if err != nil {
				httpx.ErrorCtx(r.Context(), w, err)
				return
			}
			items = append(items, types.ProductCard{
				ProductId:      card.ProductId,
				Name:           card.Name,
				OriginPriceFen: card.OriginPriceFen,
				FinalPriceFen:  card.FinalPriceFen,
				PromotionTag:   card.PromotionTag,
				StockAvailable: card.StockAvailable,
			})
		}

		httpx.OkJsonCtx(r.Context(), w, map[string]any{
			"items": items,
		})
	}
}
