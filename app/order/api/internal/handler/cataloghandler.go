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
		items := make([]types.ProductCard, 0, 1)
		for _, productID := range []int64{100} {
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
