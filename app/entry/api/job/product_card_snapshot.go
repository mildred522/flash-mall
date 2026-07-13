package job

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"flash-mall/app/entry/api/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProductCardSnapshotJob struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProductCardSnapshotJob(svcCtx *svc.ServiceContext) *ProductCardSnapshotJob {
	return &ProductCardSnapshotJob{
		ctx:    context.Background(),
		svcCtx: svcCtx,
		Logger: logx.WithContext(context.Background()),
	}
}

func (j *ProductCardSnapshotJob) Start() {
	if !j.svcCtx.Config.ProductCardSnapshotRefreshEnabled {
		return
	}
	interval := time.Duration(j.svcCtx.Config.ProductCardSnapshotRefreshIntervalSeconds) * time.Second
	if interval <= 0 {
		interval = time.Minute
	}
	j.Infof("product-card snapshot refresh job started: interval=%s window_minutes=%d limit=%d", interval, j.windowMinutes(), j.limit())
	go func() {
		timer := time.NewTimer(5 * time.Second)
		defer timer.Stop()
		for {
			select {
			case <-j.ctx.Done():
				return
			case <-timer.C:
				j.refreshOnce()
				timer.Reset(interval)
			}
		}
	}()
}

func (j *ProductCardSnapshotJob) refreshOnce() {
	db, err := j.svcCtx.SqlConn.RawDB()
	if err != nil {
		j.Errorf("product-card snapshot refresh skipped: raw db failed: %v", err)
		return
	}
	if err := ensureProductCardSnapshotTables(j.ctx, db); err != nil {
		j.Errorf("product-card snapshot refresh skipped: ensure tables failed: %v", err)
		return
	}
	productIDs, err := productCardSnapshotPromotionProductIDs(j.ctx, db, j.windowMinutes(), j.limit())
	if err != nil {
		j.Errorf("product-card snapshot refresh product query failed: %v", err)
		return
	}
	var affected int64
	for _, productID := range productIDs {
		rows, err := refreshProductCardSnapshot(j.ctx, db, productID)
		if err != nil {
			j.Errorf("product-card snapshot refresh failed: product_id=%d err=%v", productID, err)
			continue
		}
		affected += rows
	}
	if len(productIDs) > 0 {
		j.Infof("product-card snapshot refresh completed: products=%d affected=%d", len(productIDs), affected)
	}
}

func (j *ProductCardSnapshotJob) windowMinutes() int64 {
	if j.svcCtx.Config.ProductCardSnapshotRefreshWindowMinutes <= 0 {
		return 120
	}
	return j.svcCtx.Config.ProductCardSnapshotRefreshWindowMinutes
}

func (j *ProductCardSnapshotJob) limit() int64 {
	if j.svcCtx.Config.ProductCardSnapshotRefreshLimit <= 0 {
		return 1000
	}
	return j.svcCtx.Config.ProductCardSnapshotRefreshLimit
}

func productCardSnapshotPromotionProductIDs(ctx context.Context, db *sql.DB, windowMinutes int64, limit int64) ([]int64, error) {
	now := time.Now()
	window := time.Duration(windowMinutes) * time.Minute
	rows, err := db.QueryContext(ctx, `
SELECT DISTINCT product_id
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
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return productIDs, nil
}

func ensureProductCardSnapshotTables(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, productStockSnapshotDDL); err != nil {
		return err
	}
	_, err := db.ExecContext(ctx, productCardSnapshotDDL)
	return err
}

func refreshProductCardSnapshot(ctx context.Context, db *sql.DB, productID int64) (int64, error) {
	result, err := db.ExecContext(ctx, fmt.Sprintf(productCardSnapshotRefreshSQL, "p.id = ?"), productID, int64(1))
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

const productStockSnapshotDDL = `
CREATE TABLE IF NOT EXISTS mall_product.product_stock_snapshot (
  product_id bigint NOT NULL,
  available bigint NOT NULL DEFAULT 0,
  reserved bigint NOT NULL DEFAULT 0,
  total bigint NOT NULL DEFAULT 0,
  source varchar(32) NOT NULL DEFAULT 'inventory-kitex',
  version bigint NOT NULL DEFAULT 0,
  update_time timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (product_id),
  KEY ix_update_time (update_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`

const productCardSnapshotDDL = `
CREATE TABLE IF NOT EXISTS mall_product.product_card_snapshot (
  product_id bigint NOT NULL,
  name varchar(255) NOT NULL DEFAULT '',
  origin_price_fen bigint NOT NULL DEFAULT 0,
  final_price_fen bigint NOT NULL DEFAULT 0,
  promotion_type varchar(32) NOT NULL DEFAULT '',
  promotion_tag varchar(32) NOT NULL DEFAULT '',
  stock_available bigint NOT NULL DEFAULT 0,
  supplier_id bigint NOT NULL DEFAULT 0,
  status tinyint NOT NULL DEFAULT 1,
  version bigint NOT NULL DEFAULT 0,
  update_time timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (product_id),
  KEY ix_status_product (status, product_id),
  KEY ix_update_time (update_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`

const productCardSnapshotRefreshSQL = `
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
  version = version + 1`
