package productmysql

import (
	"context"
	"database/sql"
	"fmt"

	"flash-mall/app/gateway/hertz/internal/ports"
)

type SnapshotStore struct{ db *sql.DB }

var _ ports.ProductSnapshotStore = (*SnapshotStore)(nil)

func NewSnapshotStore(db *sql.DB) *SnapshotStore { return &SnapshotStore{db: db} }

func (s *SnapshotStore) RebuildStock(ctx context.Context, request ports.SnapshotRebuildRequest) (int64, error) {
	where, args := "1=1", []any{}
	if request.ProductID > 0 {
		where += " AND p.id = ?"
		args = append(args, request.ProductID)
	}
	args = append(args, request.Limit)
	result, err := s.db.ExecContext(ctx, fmt.Sprintf(`INSERT INTO mall_product.product_stock_snapshot (product_id, available, reserved, total, source, version)
SELECT p.id, COALESCE(bucket.stock_available, 0), COALESCE(old.reserved, 0), COALESCE(bucket.stock_available, 0) + COALESCE(old.reserved, 0), 'admin-rebuild', 0
FROM mall_product.product p
LEFT JOIN (SELECT product_id, COALESCE(SUM(stock), 0) AS stock_available FROM mall_product.product_stock_bucket GROUP BY product_id) bucket ON bucket.product_id = p.id
LEFT JOIN mall_product.product_stock_snapshot old ON old.product_id = p.id
WHERE %s ORDER BY p.id LIMIT ?
ON DUPLICATE KEY UPDATE available=VALUES(available), reserved=VALUES(reserved), total=VALUES(total), source=VALUES(source), version=version+1`, where), args...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (s *SnapshotStore) RebuildCards(ctx context.Context, request ports.SnapshotRebuildRequest) (int64, error) {
	where := "1=1"
	args := make([]any, 0, 2)
	if request.ProductID > 0 {
		where += " AND p.id = ?"
		args = append(args, request.ProductID)
	}
	args = append(args, request.Limit)
	result, err := s.db.ExecContext(ctx, fmt.Sprintf(`
INSERT INTO mall_product.product_card_snapshot (product_id, name, origin_price_fen, final_price_fen, promotion_type, promotion_tag, stock_available, supplier_id, status, version)
SELECT p.id,
       p.name,
       p.origin_price_fen,
       CASE WHEN promo.promotion_price_fen > 0 THEN promo.promotion_price_fen ELSE p.sale_price_fen END AS final_price_fen,
       CASE WHEN promo.promotion_price_fen > 0 THEN 'LIMITED_PRICE' ELSE '' END AS promotion_type,
       CASE WHEN promo.promotion_price_fen > 0 THEN '限时价' ELSE '' END AS promotion_tag,
       COALESCE(stock.available, bucket.stock_available, 0) AS stock_available,
       p.supplier_id,
       p.status,
       0 AS version
FROM mall_product.product p
LEFT JOIN mall_product.product_stock_snapshot stock ON stock.product_id = p.id
LEFT JOIN (
  SELECT product_id, COALESCE(SUM(stock), 0) AS stock_available
  FROM mall_product.product_stock_bucket
  GROUP BY product_id
) bucket ON bucket.product_id = p.id
LEFT JOIN (
  SELECT product_id, MIN(discount_value) AS promotion_price_fen
  FROM mall_product.promotion_rule
  WHERE type = 'LIMITED_PRICE'
    AND status = 1
    AND (starts_at IS NULL OR starts_at <= NOW())
    AND (ends_at IS NULL OR ends_at >= NOW())
  GROUP BY product_id
) promo ON promo.product_id = p.id
WHERE %s
ORDER BY p.id
LIMIT ?
ON DUPLICATE KEY UPDATE
  name = VALUES(name),
  origin_price_fen = VALUES(origin_price_fen),
  final_price_fen = VALUES(final_price_fen),
  promotion_type = VALUES(promotion_type),
  promotion_tag = VALUES(promotion_tag),
  stock_available = VALUES(stock_available),
  supplier_id = VALUES(supplier_id),
  status = VALUES(status),
  version = version + 1`, where), args...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
