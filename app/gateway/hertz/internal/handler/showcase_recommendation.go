package handler

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"flash-mall/app/common/apperror"
	showcaseapp "flash-mall/app/gateway/hertz/internal/application/showcase"
	"flash-mall/app/gateway/hertz/internal/svc"
	"flash-mall/app/product/rpc/productclient"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

type showcaseCandidateFeature struct {
	Product              ProductCard
	Sales7d              int64
	StockAvailable       int64
	HasPromotion         bool
	CreatedAt            time.Time
	AgeDays              int
	CurrentMerchantSlots int
}

func salesScore(sales7d int64) int {
	switch {
	case sales7d >= 21:
		return 40
	case sales7d >= 11:
		return 32
	case sales7d >= 6:
		return 24
	case sales7d >= 3:
		return 16
	case sales7d >= 1:
		return 8
	default:
		return 0
	}
}

func stockScore(stock int64) int {
	switch {
	case stock >= 51:
		return 15
	case stock >= 21:
		return 11
	case stock >= 6:
		return 7
	case stock >= 1:
		return 3
	default:
		return 0
	}
}

func freshnessScore(ageDays int) int {
	switch {
	case ageDays <= 7:
		return 20
	case ageDays <= 14:
		return 14
	case ageDays <= 30:
		return 7
	default:
		return 0
	}
}

func diversityScore(currentMerchantSlots int) int {
	if currentMerchantSlots == 0 {
		return 15
	}
	if currentMerchantSlots == 1 {
		return 6
	}
	return -1
}

func rankShowcaseCandidates(features []showcaseCandidateFeature) []ShowcaseCandidate {
	type ranked struct {
		candidate ShowcaseCandidate
		createdAt time.Time
	}
	items := make([]ranked, 0, len(features))
	for _, feature := range features {
		diversity := diversityScore(feature.CurrentMerchantSlots)
		if diversity < 0 || feature.StockAvailable <= 0 {
			continue
		}
		sales := salesScore(feature.Sales7d)
		stock := stockScore(feature.StockAvailable)
		promotion := 0
		if feature.HasPromotion {
			promotion = 10
		}
		freshness := freshnessScore(feature.AgeDays)
		reasons := []string{fmt.Sprintf("近7天销量%d件", feature.Sales7d), fmt.Sprintf("可售库存%d件", feature.StockAvailable)}
		if feature.HasPromotion {
			reasons = append(reasons, "当前有限时促销")
		}
		if feature.AgeDays <= 7 {
			reasons = append(reasons, "7天内新品")
		}
		if feature.CurrentMerchantSlots == 0 {
			reasons = append(reasons, "该商家当前未获得首页曝光")
		}
		items = append(items, ranked{candidate: ShowcaseCandidate{
			Product: feature.Product, Score: sales + stock + promotion + freshness + diversity,
			Sales7d: feature.Sales7d, SalesScore: sales, StockScore: stock,
			PromotionScore: promotion, FreshnessScore: freshness, DiversityScore: diversity,
			Reasons: reasons,
		}, createdAt: feature.CreatedAt})
	}
	sort.SliceStable(items, func(i, j int) bool {
		left, right := items[i], items[j]
		if left.candidate.Score != right.candidate.Score {
			return left.candidate.Score > right.candidate.Score
		}
		if left.candidate.Sales7d != right.candidate.Sales7d {
			return left.candidate.Sales7d > right.candidate.Sales7d
		}
		if !left.createdAt.Equal(right.createdAt) {
			return left.createdAt.After(right.createdAt)
		}
		return left.candidate.Product.ProductID > right.candidate.Product.ProductID
	})
	result := make([]ShowcaseCandidate, len(items))
	for index := range items {
		result[index] = items[index].candidate
	}
	return result
}

func AdminShowcaseCandidatesHandler(svcCtx *svc.ServiceContext) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		startedAt := time.Now()
		metricResult := "error"
		defer func() { recordShowcaseCandidate(metricResult, time.Since(startedAt)) }()
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
		if pageSize > 50 {
			pageSize = 50
		}
		merchantID, err := parseOptionalInt64(c.Query("merchant_id"))
		if err != nil {
			fail(ctx, c, consts.StatusBadRequest, apperror.New(apperror.CodeInvalidArgument, "merchant_id must be numeric"))
			return
		}
		keyword := strings.TrimSpace(c.Query("keyword"))
		result, source, err := loadCachedJSON(ctx, svcCtx, showcaseCandidatesCacheKey(page, pageSize, merchantID, keyword), func(loadCtx context.Context) (ShowcaseCandidatesResp, error) {
			if svcCtx.Showcases == nil {
				return ShowcaseCandidatesResp{}, apperror.New(apperror.CodeInternal, "showcase service unavailable")
			}
			features, loadErr := svcCtx.Showcases.CandidateFeatures(loadCtx, showcaseapp.CandidateQuery{Keyword: keyword, MerchantID: merchantID})
			if loadErr != nil {
				return ShowcaseCandidatesResp{}, apperror.Wrap(apperror.CodeInternal, "showcase candidate query failed", loadErr)
			}
			productIDs := make([]int64, 0, len(features))
			for _, feature := range features {
				productIDs = append(productIDs, feature.ProductID)
			}
			cards := make(map[int64]ProductCard)
			if len(productIDs) > 0 {
				resp, rpcErr := svcCtx.ProductRpc.ListProducts(loadCtx, &productclient.ListProductsReq{ProductIds: productIDs})
				if rpcErr != nil {
					return ShowcaseCandidatesResp{}, apperror.Wrap(apperror.CodeInternal, "product service unavailable", rpcErr)
				}
				cards = buildProductCards(resp.Items, loadProductMeta(loadCtx, svcCtx, productIDs), nil)
			}
			hydrated := make([]showcaseCandidateFeature, 0, len(features))
			for _, feature := range features {
				card, exists := cards[feature.ProductID]
				if exists {
					hydrated = append(hydrated, showcaseCandidateFeature{
						Product: card, Sales7d: feature.Sales7d, StockAvailable: feature.StockAvailable,
						HasPromotion: feature.HasPromotion, CreatedAt: feature.CreatedAt, AgeDays: feature.AgeDays,
						CurrentMerchantSlots: feature.CurrentMerchantSlots,
					})
				}
			}
			ranked := rankShowcaseCandidates(hydrated)
			total := int64(len(ranked))
			start, end := (page-1)*pageSize, page*pageSize
			if start > total {
				start = total
			}
			if end > total {
				end = total
			}
			return ShowcaseCandidatesResp{Items: ranked[int(start):int(end)], Total: total, Page: page, PageSize: pageSize}, nil
		})
		if err != nil {
			showcaseCandidateCacheTotal.WithLabelValues("miss").Inc()
			fail(ctx, c, consts.StatusBadGateway, err)
			return
		}
		if source == "origin" {
			showcaseCandidateCacheTotal.WithLabelValues("miss").Inc()
		} else {
			showcaseCandidateCacheTotal.WithLabelValues("hit").Inc()
		}
		metricResult = "success"
		ok(ctx, c, result)
	}
}
