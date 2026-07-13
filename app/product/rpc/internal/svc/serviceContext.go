package svc

import (
	"context"
	"database/sql"

	"flash-mall/app/product/rpc/internal/config"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceContext struct {
	Config  config.Config
	SqlConn sqlx.SqlConn
}

func NewServiceContext(c config.Config) *ServiceContext {
	sqlConn := sqlx.NewMysql(c.DataSource)
	if !c.DisableStockRepair {
		go repairProductStockState(context.Background(), sqlConn, c.StockBucketCount)
	}
	return &ServiceContext{
		Config:  c,
		SqlConn: sqlConn,
	}
}

func repairProductStockState(ctx context.Context, sqlConn sqlx.SqlConn, bucketCount int) {
	db, err := sqlConn.RawDB()
	if err != nil {
		logx.Errorf("repair product stock state skipped: raw db failed: %v", err)
		return
	}
	if bucketCount <= 0 {
		bucketCount = 1
	}
	rows, err := db.QueryContext(ctx, `
SELECT p.id, p.stock
FROM product p
LEFT JOIN product_stock_bucket b ON b.product_id = p.id
GROUP BY p.id, p.stock
HAVING COUNT(b.product_id) = 0`)
	if err != nil {
		logx.Errorf("repair product stock buckets query failed: %v", err)
		return
	}
	defer func() { _ = rows.Close() }()

	repaired := 0
	for rows.Next() {
		var productID int64
		var total int64
		if err := rows.Scan(&productID, &total); err != nil {
			logx.Errorf("repair product stock buckets scan failed: %v", err)
			return
		}
		if err := insertProductStockBuckets(ctx, db, productID, total, bucketCount); err != nil {
			logx.Errorf("repair product stock buckets failed: product_id=%d err=%v", productID, err)
			continue
		}
		repaired++
	}
	if err := rows.Err(); err != nil {
		logx.Errorf("repair product stock buckets rows failed: %v", err)
		return
	}

	if _, err := db.ExecContext(ctx, `
UPDATE product p
JOIN (
  SELECT product_id, COALESCE(SUM(stock), 0) AS stock_total
  FROM product_stock_bucket
  GROUP BY product_id
) s ON s.product_id = p.id
SET p.stock = s.stock_total`); err != nil {
		logx.Errorf("repair product stock snapshot failed: %v", err)
		return
	}
	if repaired > 0 {
		logx.Infof("repair product stock buckets completed: repaired=%d", repaired)
	}
}

func insertProductStockBuckets(ctx context.Context, db *sql.DB, productID int64, total int64, bucketCount int) error {
	if total < 0 {
		total = 0
	}
	perBucket := total / int64(bucketCount)
	remain := total % int64(bucketCount)
	for bucketIdx := 0; bucketIdx < bucketCount; bucketIdx++ {
		stock := perBucket
		if bucketIdx == 0 {
			stock += remain
		}
		if _, err := db.ExecContext(ctx,
			"INSERT INTO product_stock_bucket (product_id, bucket_idx, stock, version) VALUES (?, ?, ?, 0) ON DUPLICATE KEY UPDATE stock = VALUES(stock)",
			productID, bucketIdx, stock,
		); err != nil {
			return err
		}
	}
	return nil
}
