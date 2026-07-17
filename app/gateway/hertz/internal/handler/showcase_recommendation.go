package handler

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"flash-mall/app/common/apperror"
	"flash-mall/app/gateway/hertz/internal/svc"
	"flash-mall/app/product/rpc/productclient"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

type showcaseCandidateQuery struct {
	Keyword    string
	MerchantID int64
}

type showcaseCandidateFeature struct {
	Product              ProductCard
	Sales7d              int64
	StockAvailable       int64
	HasPromotion         bool
	CreatedAt            time.Time
	AgeDays              int
	CurrentMerchantSlots int
}

func loadShowcaseCandidateFeatures(ctx context.Context, db *sql.DB, req showcaseCandidateQuery) ([]showcaseCandidateFeature, error) {
	where := []string{
		"product.status = 1",
		"merchant.status = 1",
		"COALESCE(stock.available, product.stock, 0) > 0",
		"existing.product_id IS NULL",
	}
	args := make([]any, 0, 3)
	if keyword := strings.TrimSpace(req.Keyword); keyword != "" {
		where = append(where, "(product.name LIKE ? OR merchant.name LIKE ?)")
		args = append(args, "%"+keyword+"%", "%"+keyword+"%")
	}
	if req.MerchantID > 0 {
		where = append(where, "product.merchant_id = ?")
		args = append(args, req.MerchantID)
	}
	rows, err := db.QueryContext(ctx, fmt.Sprintf(`SELECT product.id, product.merchant_id,
       (SELECT COALESCE(SUM(orders.amount), 0) FROM mall_order.orders orders
        WHERE orders.product_id = product.id AND orders.status IN (1,3,4,5,6)
          AND orders.create_time >= DATE_SUB(NOW(), INTERVAL 7 DAY)) AS sales_7d,
       COALESCE(stock.available, product.stock, 0) AS stock_available,
       IF(EXISTS(SELECT 1 FROM mall_product.promotion_rule promotion
                 WHERE promotion.product_id = product.id AND promotion.status = 1
                   AND promotion.starts_at <= NOW() AND promotion.ends_at >= NOW()), 1, 0) AS has_promotion,
       product.create_time, GREATEST(DATEDIFF(NOW(), product.create_time), 0) AS age_days,
       (SELECT COUNT(*) FROM mall_product.homepage_showcase_item current_item
        JOIN mall_product.product current_product ON current_product.id = current_item.product_id AND current_product.status = 1
        JOIN mall_order.merchant current_merchant ON current_merchant.id = current_product.merchant_id AND current_merchant.status = 1
        LEFT JOIN mall_product.product_stock_snapshot current_stock ON current_stock.product_id = current_product.id
        WHERE current_item.showcase_id = 1 AND current_product.merchant_id = product.merchant_id
          AND COALESCE(current_stock.available, current_product.stock, 0) > 0) AS current_slots
FROM mall_product.product product
JOIN mall_order.merchant merchant ON merchant.id = product.merchant_id
LEFT JOIN mall_product.product_stock_snapshot stock ON stock.product_id = product.id
LEFT JOIN mall_product.homepage_showcase_item existing
  ON existing.showcase_id = 1 AND existing.product_id = product.id
WHERE %s
HAVING current_slots < 2`, strings.Join(where, " AND ")), args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	features := make([]showcaseCandidateFeature, 0)
	for rows.Next() {
		var feature showcaseCandidateFeature
		var hasPromotion int64
		if err := rows.Scan(&feature.Product.ProductID, &feature.Product.MerchantID, &feature.Sales7d,
			&feature.StockAvailable, &hasPromotion, &feature.CreatedAt, &feature.AgeDays,
			&feature.CurrentMerchantSlots); err != nil {
			return nil, err
		}
		feature.HasPromotion = hasPromotion == 1
		features = append(features, feature)
	}
	return features, rows.Err()
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
			db, dbErr := svcCtx.SqlConn.RawDB()
			if dbErr != nil {
				return ShowcaseCandidatesResp{}, apperror.Wrap(apperror.CodeInternal, "product datasource unavailable", dbErr)
			}
			if dbErr = requireStorefrontSchema(loadCtx, db); dbErr != nil {
				return ShowcaseCandidatesResp{}, apperror.Wrap(apperror.CodeInternal, "showcase schema unavailable", dbErr)
			}
			features, loadErr := loadShowcaseCandidateFeatures(loadCtx, db, showcaseCandidateQuery{Keyword: keyword, MerchantID: merchantID})
			if loadErr != nil {
				return ShowcaseCandidatesResp{}, apperror.Wrap(apperror.CodeInternal, "showcase candidate query failed", loadErr)
			}
			productIDs := make([]int64, 0, len(features))
			for _, feature := range features {
				productIDs = append(productIDs, feature.Product.ProductID)
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
				card, exists := cards[feature.Product.ProductID]
				if exists {
					feature.Product = card
					hydrated = append(hydrated, feature)
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
