package logic

import (
	"context"
	"fmt"
	"testing"

	"flash-mall/app/product/rpc/internal/config"
	"flash-mall/app/product/rpc/internal/svc"
	"flash-mall/app/product/rpc/product"
)

const revertStockTestDSN = "root:6494kj06@tcp(127.0.0.1:3306)/mall_product?charset=utf8mb4&parseTime=true&loc=Local"

func TestRevertStock_EmptyOrderIDFallsBackPerProduct(t *testing.T) {
	svcCtx := newRevertStockTestServiceContext(t)
	productA := int64(9301)
	productB := int64(9302)
	ensureRevertStockSchema(t, svcCtx)
	cleanupRevertStockRows(t, svcCtx, productA, productB)

	logic := NewRevertStockLogic(context.Background(), svcCtx)
	if _, err := logic.RevertStock(&product.RevertStockReq{Id: productA, Num: 3}); err != nil {
		t.Fatalf("first revert failed: %v", err)
	}
	if _, err := logic.RevertStock(&product.RevertStockReq{Id: productB, Num: 4}); err != nil {
		t.Fatalf("second revert failed: %v", err)
	}

	if got := stockBucketTotal(t, svcCtx, productA); got != 3 {
		t.Fatalf("product A stock = %d, want 3", got)
	}
	if got := stockBucketTotal(t, svcCtx, productB); got != 4 {
		t.Fatalf("product B stock = %d, want 4", got)
	}
}

func TestRevertStock_ReturnsNonDuplicateStockLogError(t *testing.T) {
	svcCtx := newRevertStockTestServiceContext(t)
	ensureRevertStockSchema(t, svcCtx)
	orderID := "revert-non-duplicate-error"
	productID := int64(9303)
	cleanupRevertStockRows(t, svcCtx, productID)
	if _, err := svcCtx.SqlConn.ExecCtx(context.Background(), "DELETE FROM stock_log WHERE order_id = ?", orderID); err != nil {
		t.Fatalf("stock log cleanup failed: %v", err)
	}
	if _, err := svcCtx.SqlConn.ExecCtx(context.Background(), "RENAME TABLE stock_log TO stock_log_revert_test_backup"); err != nil {
		t.Fatalf("rename stock_log failed: %v", err)
	}
	t.Cleanup(func() {
		_, _ = svcCtx.SqlConn.ExecCtx(context.Background(), "RENAME TABLE stock_log_revert_test_backup TO stock_log")
	})

	logic := NewRevertStockLogic(context.Background(), svcCtx)
	if _, err := logic.RevertStock(&product.RevertStockReq{Id: productID, Num: 3, OrderId: orderID}); err == nil {
		t.Fatal("expected missing stock_log table error")
	}
}

func newRevertStockTestServiceContext(t *testing.T) *svc.ServiceContext {
	t.Helper()

	return svc.NewServiceContext(config.Config{
		DataSource:       revertStockTestDSN,
		StockBucketCount: 4,
	})
}

func ensureRevertStockSchema(t *testing.T, svcCtx *svc.ServiceContext) {
	t.Helper()

	statements := []string{
		`CREATE TABLE IF NOT EXISTS product_stock_bucket (
			product_id BIGINT NOT NULL,
			bucket_idx INT NOT NULL,
			stock INT NOT NULL DEFAULT 0,
			version BIGINT NOT NULL DEFAULT 0,
			PRIMARY KEY (product_id, bucket_idx)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS stock_log (
			id BIGINT NOT NULL AUTO_INCREMENT,
			order_id VARCHAR(64) NOT NULL,
			type VARCHAR(32) NOT NULL,
			create_time TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (id),
			UNIQUE KEY uniq_order_type (order_id, type)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}

	for _, statement := range statements {
		if _, err := svcCtx.SqlConn.ExecCtx(context.Background(), statement); err != nil {
			t.Fatalf("schema ensure failed for %q: %v", statement, err)
		}
	}
}

func cleanupRevertStockRows(t *testing.T, svcCtx *svc.ServiceContext, productIDs ...int64) {
	t.Helper()

	orderIDs := []string{""}
	for _, productID := range productIDs {
		if _, err := svcCtx.SqlConn.ExecCtx(context.Background(), fmt.Sprintf("DELETE FROM product_stock_bucket WHERE product_id = %d", productID)); err != nil {
			t.Fatalf("bucket cleanup failed: %v", err)
		}
		orderIDs = append(orderIDs, fmt.Sprintf("%d", productID))
	}

	for _, orderID := range orderIDs {
		if _, err := svcCtx.SqlConn.ExecCtx(context.Background(), "DELETE FROM stock_log WHERE order_id = ?", orderID); err != nil {
			t.Fatalf("stock log cleanup failed: %v", err)
		}
	}
}

func stockBucketTotal(t *testing.T, svcCtx *svc.ServiceContext, productID int64) int64 {
	t.Helper()

	var total int64
	if err := svcCtx.SqlConn.QueryRowCtx(context.Background(), &total, "SELECT COALESCE(SUM(stock), 0) FROM product_stock_bucket WHERE product_id = ?", productID); err != nil {
		t.Fatalf("stock query failed: %v", err)
	}
	return total
}
