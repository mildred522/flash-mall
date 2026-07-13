package logic

import (
	"database/sql"
	"errors"
	"hash/crc32"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func stockBucketCount(configured int) int {
	if configured <= 0 {
		return 1
	}
	return configured
}

func stockBucketIndex(orderId string, bucketCount int) int {
	if bucketCount <= 1 {
		return 0
	}
	hash := crc32.ChecksumIEEE([]byte(orderId))
	return int(hash % uint32(bucketCount))
}

func ensureProductStockBucketsFromSnapshotTx(tx *sql.Tx, productID int64, bucketCount int) error {
	var bucketRows int64
	if err := tx.QueryRow("SELECT COUNT(*) FROM product_stock_bucket WHERE product_id = ?", productID).Scan(&bucketRows); err != nil {
		return err
	}
	if bucketRows > 0 {
		return nil
	}

	var total int64
	if err := tx.QueryRow("SELECT stock FROM product WHERE id = ? FOR UPDATE", productID).Scan(&total); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return status.Error(codes.NotFound, "product not found")
		}
		return err
	}
	values := splitStockIntoBuckets(total, stockBucketCount(bucketCount))
	for bucketIdx, value := range values {
		if _, err := tx.Exec(
			"INSERT INTO product_stock_bucket (product_id, bucket_idx, stock, version) VALUES (?, ?, ?, 0)",
			productID, bucketIdx, value,
		); err != nil {
			return err
		}
	}
	return nil
}

func syncProductStockSnapshotTx(tx *sql.Tx, productID int64) error {
	_, err := tx.Exec(
		"UPDATE product SET stock = (SELECT COALESCE(SUM(stock), 0) FROM product_stock_bucket WHERE product_id = ?), version = version + 1 WHERE id = ?",
		productID, productID,
	)
	return err
}

func splitStockIntoBuckets(total int64, bucketCount int) []int64 {
	if total < 0 {
		total = 0
	}
	if bucketCount <= 0 {
		bucketCount = 1
	}
	values := make([]int64, bucketCount)
	perBucket := total / int64(bucketCount)
	remain := total % int64(bucketCount)
	for idx := range values {
		values[idx] = perBucket
		if idx == 0 {
			values[idx] += remain
		}
	}
	return values
}
