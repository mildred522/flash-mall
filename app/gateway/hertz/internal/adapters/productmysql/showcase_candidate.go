package productmysql

import (
	"context"
	"fmt"
	"strings"

	"flash-mall/app/gateway/hertz/internal/application/showcase"
)

var _ showcase.CandidateRepository = (*ShowcaseRepository)(nil)

func (r *ShowcaseRepository) CandidateFeatures(ctx context.Context, query showcase.CandidateQuery) ([]showcase.CandidateFeature, error) {
	where := []string{
		"product.status = 1",
		"merchant.status = 1",
		"COALESCE(stock.available, product.stock, 0) > 0",
		"existing.product_id IS NULL",
	}
	args := make([]any, 0, 3)
	if query.Keyword != "" {
		where = append(where, "(product.name LIKE ? OR merchant.name LIKE ?)")
		args = append(args, "%"+query.Keyword+"%", "%"+query.Keyword+"%")
	}
	if query.MerchantID > 0 {
		where = append(where, "product.merchant_id = ?")
		args = append(args, query.MerchantID)
	}
	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(`SELECT product.id, product.merchant_id,
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
	features := make([]showcase.CandidateFeature, 0)
	for rows.Next() {
		var feature showcase.CandidateFeature
		var hasPromotion int64
		if err := rows.Scan(&feature.ProductID, &feature.MerchantID, &feature.Sales7d,
			&feature.StockAvailable, &hasPromotion, &feature.CreatedAt, &feature.AgeDays,
			&feature.CurrentMerchantSlots); err != nil {
			return nil, err
		}
		feature.HasPromotion = hasPromotion == 1
		features = append(features, feature)
	}
	return features, rows.Err()
}
