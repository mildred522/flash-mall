package pricing

import (
	"testing"

	productpb "flash-mall/app/product/rpc/product"
)

func TestBuildQuote_UsesLimitedPriceSnapshot(t *testing.T) {
	card := &productpb.GetProductCardResp{
		ProductId:      100,
		Name:           "Flash Coat",
		OriginPriceFen: 12900,
		FinalPriceFen:  9900,
		PromotionType:  "LIMITED_PRICE",
		PromotionTag:   "限时价",
		SupplierId:     200,
	}

	quote, err := BuildQuote(card, 3)
	if err != nil {
		t.Fatalf("BuildQuote returned error: %v", err)
	}

	if quote.ProductId != 100 || quote.SupplierId != 200 {
		t.Fatalf("unexpected quote identity: %#v", quote)
	}
	if quote.Amount != 3 {
		t.Fatalf("expected amount=3, got %d", quote.Amount)
	}
	if quote.OriginUnitPriceFen != 12900 {
		t.Fatalf("expected origin unit price 12900, got %d", quote.OriginUnitPriceFen)
	}
	if quote.SaleUnitPriceFen != 9900 {
		t.Fatalf("expected sale unit price 9900, got %d", quote.SaleUnitPriceFen)
	}
	if quote.PayableAmountFen != 29700 {
		t.Fatalf("expected payable amount 29700, got %d", quote.PayableAmountFen)
	}
	if quote.DiscountAmountFen != 9000 {
		t.Fatalf("expected discount amount 9000, got %d", quote.DiscountAmountFen)
	}
	if quote.PromotionType != "LIMITED_PRICE" || quote.PromotionTag != "限时价" {
		t.Fatalf("unexpected promotion snapshot: %#v", quote)
	}
}

func TestBuildQuote_RejectsInvalidInputs(t *testing.T) {
	tests := []struct {
		name   string
		card   *productpb.GetProductCardResp
		amount int64
	}{
		{
			name:   "nil card",
			card:   nil,
			amount: 1,
		},
		{
			name: "non-positive amount",
			card: &productpb.GetProductCardResp{
				ProductId:      100,
				OriginPriceFen: 12900,
				FinalPriceFen:  9900,
			},
			amount: 0,
		},
		{
			name: "missing product id",
			card: &productpb.GetProductCardResp{
				OriginPriceFen: 12900,
				FinalPriceFen:  9900,
			},
			amount: 1,
		},
		{
			name: "invalid final price",
			card: &productpb.GetProductCardResp{
				ProductId:      100,
				OriginPriceFen: 12900,
				FinalPriceFen:  0,
			},
			amount: 1,
		},
		{
			name: "final price above origin",
			card: &productpb.GetProductCardResp{
				ProductId:      100,
				OriginPriceFen: 9900,
				FinalPriceFen:  12900,
			},
			amount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := BuildQuote(tt.card, tt.amount); err == nil {
				t.Fatalf("expected BuildQuote to fail")
			}
		})
	}
}
