package handler

import (
	"context"
	"fmt"

	"flash-mall/app/gateway/hertz/internal/ports"
	"flash-mall/app/gateway/hertz/internal/svc"
	"flash-mall/app/product/rpc/productclient"

	"github.com/zeromicro/go-zero/core/logx"
)

func buildProductCards(items []*productclient.GetProductCardResp, meta map[int64]productMeta, stocks map[int64]ports.Stock) map[int64]ProductCard {
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
			ProductID: item.ProductId, Name: item.Name, ImageURL: m.ImageURL,
			OriginPriceFen: item.OriginPriceFen, FinalPriceFen: item.FinalPriceFen,
			SupplierID: item.SupplierId, SupplierName: m.SupplierName, PromotionTag: item.PromotionTag,
			StockAvailable: stockAvailable, StockReserved: stockReserved, StockTotal: stockTotal, StockSource: stockSource,
			MerchantID: m.MerchantID, MerchantName: m.MerchantName, MerchantLogo: m.MerchantLogo,
			StoreURL: fmt.Sprintf("/store/%d", m.MerchantID), StoreStatus: m.StoreStatus,
		}
	}
	return cards
}

func loadCatalogInventoryStocks(ctx context.Context, svcCtx *svc.ServiceContext, productIDs []int64) map[int64]ports.Stock {
	result := make(map[int64]ports.Stock, len(productIDs))
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
