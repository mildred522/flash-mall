package productmysql

import (
	"context"
	"time"
)

func (r *PromotionRepository) WindowAffectedProductIDs(ctx context.Context, now time.Time, windowMinutes, limit int64) ([]int64, error) {
	window := time.Duration(windowMinutes) * time.Minute
	rows, err := r.db.QueryContext(ctx, `SELECT DISTINCT product_id
FROM mall_product.promotion_rule
WHERE status = 1
  AND (
    ((starts_at IS NULL OR starts_at <= NOW()) AND (ends_at IS NULL OR ends_at >= NOW()))
    OR (starts_at IS NOT NULL AND starts_at BETWEEN ? AND ?)
    OR (ends_at IS NOT NULL AND ends_at BETWEEN ? AND ?)
  )
ORDER BY product_id
LIMIT ?`, now.Add(-window), now.Add(window), now.Add(-window), now.Add(window), limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	productIDs := make([]int64, 0)
	for rows.Next() {
		var productID int64
		if err := rows.Scan(&productID); err != nil {
			return nil, err
		}
		if productID > 0 {
			productIDs = append(productIDs, productID)
		}
	}
	return productIDs, rows.Err()
}
