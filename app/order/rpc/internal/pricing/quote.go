package pricing

import (
	"fmt"

	productpb "flash-mall/app/product/rpc/product"
)

type Quote struct {
	ProductId          int64
	ProductName        string
	SupplierId         int64
	Amount             int64
	OriginUnitPriceFen int64
	SaleUnitPriceFen   int64
	OriginAmountFen    int64
	PayableAmountFen   int64
	DiscountAmountFen  int64
	PromotionType      string
	PromotionTag       string
}

func BuildQuote(card *productpb.GetProductCardResp, amount int64) (*Quote, error) {
	if card == nil {
		return nil, fmt.Errorf("product card is required")
	}
	if amount <= 0 {
		return nil, fmt.Errorf("amount must be positive")
	}
	if card.ProductId <= 0 {
		return nil, fmt.Errorf("product_id is required")
	}
	if card.OriginPriceFen <= 0 {
		return nil, fmt.Errorf("origin_price_fen must be positive")
	}
	if card.FinalPriceFen <= 0 {
		return nil, fmt.Errorf("final_price_fen must be positive")
	}
	if card.FinalPriceFen > card.OriginPriceFen {
		return nil, fmt.Errorf("final_price_fen cannot exceed origin_price_fen")
	}

	originAmount := card.OriginPriceFen * amount
	payableAmount := card.FinalPriceFen * amount

	return &Quote{
		ProductId:          card.ProductId,
		ProductName:        card.Name,
		SupplierId:         card.SupplierId,
		Amount:             amount,
		OriginUnitPriceFen: card.OriginPriceFen,
		SaleUnitPriceFen:   card.FinalPriceFen,
		OriginAmountFen:    originAmount,
		PayableAmountFen:   payableAmount,
		DiscountAmountFen:  originAmount - payableAmount,
		PromotionType:      card.PromotionType,
		PromotionTag:       card.PromotionTag,
	}, nil
}
