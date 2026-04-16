package logic

import (
	"context"
	"fmt"
	"testing"

	"flash-mall/app/product/rpc/internal/config"
	"flash-mall/app/product/rpc/internal/svc"
	"flash-mall/app/product/rpc/product"
)

const productCardTestDSN = "root:6494kj06@tcp(127.0.0.1:3306)/mall_product?charset=utf8mb4&parseTime=true&loc=Local"

const (
	productCardHappyProductID      int64 = 9100
	productCardHappySupplierID     int64 = 9200
	productCardNullBoundProductID  int64 = 9101
	productCardNullBoundSupplierID int64 = 9201
)

func TestGetProductCardLogic_GetProductCard_UsesPromotionAndStockSummary(t *testing.T) {
	svcCtx := newTestServiceContext(t)
	seedProductCardData(
		t,
		svcCtx,
		productCardHappyProductID,
		productCardHappySupplierID,
		fmt.Sprintf("INSERT INTO promotion_rule (product_id, type, discount_value, threshold_amount, starts_at, ends_at, status) VALUES (%d, 'LIMITED_PRICE', 9900, 0, DATE_SUB(NOW(), INTERVAL 1 HOUR), DATE_ADD(NOW(), INTERVAL 1 HOUR), 1)", productCardHappyProductID),
	)

	resp, err := NewGetProductCardLogic(context.Background(), svcCtx).GetProductCard(&product.GetProductCardReq{
		ProductId: productCardHappyProductID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ProductId != productCardHappyProductID || resp.Name == "" {
		t.Fatalf("unexpected card payload: %#v", resp)
	}
	if resp.OriginPriceFen != 12900 {
		t.Fatalf("unexpected origin price: %d", resp.OriginPriceFen)
	}
	if resp.FinalPriceFen != 9900 {
		t.Fatalf("expected limited price to win, got %d", resp.FinalPriceFen)
	}
	if resp.StockAvailable <= 0 {
		t.Fatalf("expected positive stock summary, got %d", resp.StockAvailable)
	}
}

func TestGetProductCardLogic_GetProductCard_TreatsNullBoundsAsActive(t *testing.T) {
	svcCtx := newTestServiceContext(t)
	seedProductCardData(
		t,
		svcCtx,
		productCardNullBoundProductID,
		productCardNullBoundSupplierID,
		fmt.Sprintf("INSERT INTO promotion_rule (product_id, type, discount_value, threshold_amount, starts_at, ends_at, status) VALUES (%d, 'LIMITED_PRICE', 8800, 0, NULL, DATE_ADD(NOW(), INTERVAL 1 HOUR), 1)", productCardNullBoundProductID),
	)

	resp, err := NewGetProductCardLogic(context.Background(), svcCtx).GetProductCard(&product.GetProductCardReq{
		ProductId: productCardNullBoundProductID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.FinalPriceFen != 8800 {
		t.Fatalf("expected null-bound promotion to stay active, got %d", resp.FinalPriceFen)
	}
	if resp.PromotionType != limitedPricePromotionType {
		t.Fatalf("expected limited price promotion type, got %q", resp.PromotionType)
	}
}

func newTestServiceContext(t *testing.T) *svc.ServiceContext {
	t.Helper()

	return svc.NewServiceContext(config.Config{
		DataSource:       productCardTestDSN,
		StockBucketCount: 4,
	})
}

func seedProductCardData(t *testing.T, svcCtx *svc.ServiceContext, productID, supplierID int64, promotionSQL string) {
	t.Helper()

	ensureProductCardSchema(t, svcCtx)

	statements := []string{
		fmt.Sprintf("DELETE FROM promotion_rule WHERE product_id = %d", productID),
		fmt.Sprintf("DELETE FROM product_stock_bucket WHERE product_id = %d", productID),
		fmt.Sprintf("DELETE FROM product WHERE id = %d", productID),
		fmt.Sprintf("DELETE FROM supplier WHERE id = %d", supplierID),
		fmt.Sprintf("INSERT INTO supplier (id, name, status) VALUES (%d, 'Flash Supplier Test', 1)", supplierID),
		fmt.Sprintf("INSERT INTO product (id, name, stock, version, origin_price_fen, sale_price_fen, status, supplier_id) VALUES (%d, 'Flash Coat Test', 0, 0, 12900, 11900, 1, %d)", productID, supplierID),
		fmt.Sprintf("INSERT INTO product_stock_bucket (product_id, bucket_idx, stock, version) VALUES (%d, 0, 4, 0), (%d, 1, 6, 0)", productID, productID),
		promotionSQL,
	}

	for _, statement := range statements {
		if _, err := svcCtx.SqlConn.ExecCtx(context.Background(), statement); err != nil {
			t.Fatalf("seed failed for %q: %v", statement, err)
		}
	}

	t.Cleanup(func() {
		cleanupStatements := []string{
			fmt.Sprintf("DELETE FROM promotion_rule WHERE product_id = %d", productID),
			fmt.Sprintf("DELETE FROM product_stock_bucket WHERE product_id = %d", productID),
			fmt.Sprintf("DELETE FROM product WHERE id = %d", productID),
			fmt.Sprintf("DELETE FROM supplier WHERE id = %d", supplierID),
		}
		for _, statement := range cleanupStatements {
			_, _ = svcCtx.SqlConn.ExecCtx(context.Background(), statement)
		}
	})
}

func ensureProductCardSchema(t *testing.T, svcCtx *svc.ServiceContext) {
	t.Helper()

	statements := []string{
		`CREATE TABLE IF NOT EXISTS promotion_rule (
			id BIGINT NOT NULL AUTO_INCREMENT,
			product_id BIGINT NOT NULL,
			type VARCHAR(32) NOT NULL,
			discount_value BIGINT NOT NULL DEFAULT 0,
			threshold_amount BIGINT NOT NULL DEFAULT 0,
			starts_at TIMESTAMP NULL DEFAULT NULL,
			ends_at TIMESTAMP NULL DEFAULT NULL,
			status TINYINT NOT NULL DEFAULT 1,
			PRIMARY KEY (id),
			KEY ix_product_status_time (product_id, status, starts_at, ends_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS supplier (
			id BIGINT NOT NULL AUTO_INCREMENT,
			name VARCHAR(128) NOT NULL,
			status TINYINT NOT NULL DEFAULT 1,
			PRIMARY KEY (id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}

	for _, statement := range statements {
		if _, err := svcCtx.SqlConn.ExecCtx(context.Background(), statement); err != nil {
			t.Fatalf("schema ensure failed for %q: %v", statement, err)
		}
	}

	ensureProductColumn(t, svcCtx, "origin_price_fen", "ALTER TABLE product ADD COLUMN origin_price_fen BIGINT NOT NULL DEFAULT 0")
	ensureProductColumn(t, svcCtx, "sale_price_fen", "ALTER TABLE product ADD COLUMN sale_price_fen BIGINT NOT NULL DEFAULT 0")
	ensureProductColumn(t, svcCtx, "status", "ALTER TABLE product ADD COLUMN status TINYINT NOT NULL DEFAULT 1")
	ensureProductColumn(t, svcCtx, "supplier_id", "ALTER TABLE product ADD COLUMN supplier_id BIGINT NOT NULL DEFAULT 0")
}

func ensureProductColumn(t *testing.T, svcCtx *svc.ServiceContext, columnName string, alterSQL string) {
	t.Helper()

	var count int64
	query := `SELECT COUNT(*) AS count
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = DATABASE()
  AND TABLE_NAME = 'product'
  AND COLUMN_NAME = ?`
	if err := svcCtx.SqlConn.QueryRowCtx(context.Background(), &count, query, columnName); err != nil {
		t.Fatalf("column check failed for %s: %v", columnName, err)
	}
	if count > 0 {
		return
	}

	if _, err := svcCtx.SqlConn.ExecCtx(context.Background(), alterSQL); err != nil {
		t.Fatalf("column add failed for %s using %q: %v", columnName, alterSQL, err)
	}

	if err := svcCtx.SqlConn.QueryRowCtx(context.Background(), &count, query, columnName); err != nil {
		t.Fatalf("column recheck failed for %s: %v", columnName, err)
	}
	if count == 0 {
		t.Fatalf("column %s still missing after alter", columnName)
	}
}
