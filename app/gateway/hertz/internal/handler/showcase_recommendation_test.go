package handler

import (
	"context"
	"testing"
	"time"

	"flash-mall/app/gateway/hertz/internal/config"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func TestShowcaseScoreBoundaries(t *testing.T) {
	for input, want := range map[int64]int{0: 0, 1: 8, 2: 8, 3: 16, 5: 16, 6: 24, 10: 24, 11: 32, 20: 32, 21: 40} {
		if got := salesScore(input); got != want {
			t.Errorf("salesScore(%d)=%d want=%d", input, got, want)
		}
	}
	for input, want := range map[int64]int{0: 0, 1: 3, 5: 3, 6: 7, 20: 7, 21: 11, 50: 11, 51: 15} {
		if got := stockScore(input); got != want {
			t.Errorf("stockScore(%d)=%d want=%d", input, got, want)
		}
	}
	for input, want := range map[int]int{0: 20, 7: 20, 8: 14, 14: 14, 15: 7, 30: 7, 31: 0} {
		if got := freshnessScore(input); got != want {
			t.Errorf("freshnessScore(%d)=%d want=%d", input, got, want)
		}
	}
	for input, want := range map[int]int{0: 15, 1: 6, 2: -1} {
		if got := diversityScore(input); got != want {
			t.Errorf("diversityScore(%d)=%d want=%d", input, got, want)
		}
	}
}

func TestRankShowcaseCandidatesIsStableAndExplainable(t *testing.T) {
	created := time.Date(2026, 7, 13, 12, 0, 0, 0, time.Local)
	features := []showcaseCandidateFeature{
		{Product: ProductCard{ProductID: 101, MerchantID: 10}, Sales7d: 6, StockAvailable: 21, CreatedAt: created, AgeDays: 5, CurrentMerchantSlots: 1},
		{Product: ProductCard{ProductID: 102, MerchantID: 11}, Sales7d: 6, StockAvailable: 21, CreatedAt: created, AgeDays: 5, CurrentMerchantSlots: 0},
		{Product: ProductCard{ProductID: 103, MerchantID: 12}, Sales7d: 30, StockAvailable: 80, CreatedAt: created, AgeDays: 2, CurrentMerchantSlots: 2},
	}
	got := rankShowcaseCandidates(features)
	if len(got) != 2 || got[0].Product.ProductID != 102 || got[1].Product.ProductID != 101 {
		t.Fatalf("unexpected rank: %#v", got)
	}
	if got[0].Score != 70 || len(got[0].Reasons) == 0 || got[0].DiversityScore != 15 {
		t.Fatalf("unexpected explanation: %#v", got[0])
	}
}

func TestRankShowcaseCandidatesUsesTieBreakers(t *testing.T) {
	older := time.Date(2026, 7, 1, 12, 0, 0, 0, time.Local)
	newer := older.Add(time.Hour)
	features := []showcaseCandidateFeature{
		{Product: ProductCard{ProductID: 100, MerchantID: 10}, Sales7d: 3, StockAvailable: 6, CreatedAt: older, AgeDays: 20},
		{Product: ProductCard{ProductID: 102, MerchantID: 11}, Sales7d: 3, StockAvailable: 6, CreatedAt: newer, AgeDays: 20},
		{Product: ProductCard{ProductID: 101, MerchantID: 12}, Sales7d: 3, StockAvailable: 6, CreatedAt: newer, AgeDays: 20},
	}
	got := rankShowcaseCandidates(features)
	if got[0].Product.ProductID != 102 || got[1].Product.ProductID != 101 || got[2].Product.ProductID != 100 {
		t.Fatalf("unexpected tie order: %#v", got)
	}
}

func TestShowcaseCandidateCacheKeyNormalizesKeyword(t *testing.T) {
	left := showcaseCandidatesCacheKey(1, 20, 7, " 风衣 ")
	right := showcaseCandidatesCacheKey(1, 20, 7, "风衣")
	if left != right {
		t.Fatalf("candidate keys differ: %q %q", left, right)
	}
}

func TestLoadShowcaseCandidateFeaturesFiltersEligibleProducts(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	createdAt := time.Date(2026, 7, 10, 10, 0, 0, 0, time.Local)
	mock.ExpectQuery("SELECT product.id, product.merchant_id").
		WithArgs("%风衣%", "%风衣%", int64(1000)).
		WillReturnRows(sqlmock.NewRows([]string{
			"product_id", "merchant_id", "sales_7d", "stock_available", "has_promotion", "create_time", "age_days", "current_slots",
		}).AddRow(105, 1000, 12, 30, 1, createdAt, 3, 1))

	got, err := loadShowcaseCandidateFeatures(context.Background(), db, showcaseCandidateQuery{Keyword: "风衣", MerchantID: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Product.ProductID != 105 || got[0].Sales7d != 12 || !got[0].HasPromotion || got[0].CurrentMerchantSlots != 1 {
		t.Fatalf("unexpected features: %#v", got)
	}
}

func TestAdminShowcaseCandidatesRouteRequiresAuthentication(t *testing.T) {
	h := server.Default()
	registerAdminRoutes(h, &svc.ServiceContext{Config: config.Config{JwtAuthSecret: "jwt-secret"}})
	resp := ut.PerformRequest(h.Engine, "GET", "/api/admin/showcase/candidates", nil).Result()
	if resp.StatusCode() != consts.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", resp.StatusCode(), resp.Body())
	}
}
