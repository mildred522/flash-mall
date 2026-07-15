package logic

import (
	"context"
	"fmt"
	"testing"

	"flash-mall/app/product/rpc/product"
)

func TestListProductsLogic_PartialSnapshotFallsBackWithoutDroppingProducts(t *testing.T) {
	svcCtx := newTestServiceContext(t)
	ensureProductCardSchema(t, svcCtx)

	productIDs := []int64{9301, 9302}
	supplierID := int64(9300)
	statements := []string{
		fmt.Sprintf("DELETE FROM product_card_snapshot WHERE product_id IN (%d, %d)", productIDs[0], productIDs[1]),
		fmt.Sprintf("DELETE FROM product_stock_snapshot WHERE product_id IN (%d, %d)", productIDs[0], productIDs[1]),
		fmt.Sprintf("DELETE FROM product_stock_bucket WHERE product_id IN (%d, %d)", productIDs[0], productIDs[1]),
		fmt.Sprintf("DELETE FROM promotion_rule WHERE product_id IN (%d, %d)", productIDs[0], productIDs[1]),
		fmt.Sprintf("DELETE FROM product WHERE id IN (%d, %d)", productIDs[0], productIDs[1]),
		fmt.Sprintf("DELETE FROM supplier WHERE id = %d", supplierID),
		fmt.Sprintf("INSERT INTO supplier (id, name, status) VALUES (%d, 'Partial Snapshot Supplier', 1)", supplierID),
		fmt.Sprintf("INSERT INTO product (id, name, stock, version, origin_price_fen, sale_price_fen, status, supplier_id) VALUES (%d, 'Snapshot Product', 4, 0, 1200, 1000, 1, %d), (%d, 'Database Product', 6, 0, 2200, 2000, 1, %d)", productIDs[0], supplierID, productIDs[1], supplierID),
		fmt.Sprintf("INSERT INTO product_stock_bucket (product_id, bucket_idx, stock, version) VALUES (%d, 0, 4, 0), (%d, 0, 6, 0)", productIDs[0], productIDs[1]),
		fmt.Sprintf("INSERT INTO product_card_snapshot (product_id, name, origin_price_fen, final_price_fen, stock_available, supplier_id, status, version) VALUES (%d, 'Snapshot Product', 1200, 1000, 4, %d, 1, 1)", productIDs[0], supplierID),
	}
	t.Cleanup(func() {
		for _, statement := range statements[:6] {
			_, _ = svcCtx.SqlConn.ExecCtx(context.Background(), statement)
		}
	})
	for _, statement := range statements {
		if _, err := svcCtx.SqlConn.ExecCtx(context.Background(), statement); err != nil {
			t.Fatalf("seed failed for %q: %v", statement, err)
		}
	}

	resp, err := NewListProductsLogic(context.Background(), svcCtx).ListProducts(&product.ListProductsReq{ProductIds: productIDs})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertProductIDsPresent(t, resp, productIDs)

	var counts struct {
		Products  int64 `db:"products"`
		Snapshots int64 `db:"snapshots"`
	}
	if err := svcCtx.SqlConn.QueryRowCtx(context.Background(), &counts, `SELECT
  (SELECT COUNT(*) FROM product WHERE status = 1) AS products,
  (SELECT COUNT(*) FROM product_card_snapshot WHERE status = 1) AS snapshots`); err != nil {
		t.Fatalf("count active products and snapshots: %v", err)
	}
	gap := counts.Products - counts.Snapshots
	if gap <= 0 {
		t.Fatalf("test requires at least one missing snapshot, products=%d snapshots=%d", counts.Products, counts.Snapshots)
	}
	staleSnapshotIDs := make([]int64, 0, gap)
	t.Cleanup(func() {
		for _, id := range staleSnapshotIDs {
			_, _ = svcCtx.SqlConn.ExecCtx(context.Background(), "DELETE FROM product_card_snapshot WHERE product_id = ?", id)
		}
	})
	for i := int64(0); i < gap; i++ {
		id := int64(940000) + i
		staleSnapshotIDs = append(staleSnapshotIDs, id)
		_, _ = svcCtx.SqlConn.ExecCtx(context.Background(), "DELETE FROM product_card_snapshot WHERE product_id = ?", id)
		if _, err := svcCtx.SqlConn.ExecCtx(context.Background(), `INSERT INTO product_card_snapshot
  (product_id, name, origin_price_fen, final_price_fen, stock_available, supplier_id, status, version)
VALUES (?, 'Stale Snapshot', 100, 100, 0, 0, 1, 1)`, id); err != nil {
			t.Fatalf("seed stale snapshot %d: %v", id, err)
		}
	}

	allResp, err := NewListProductsLogic(context.Background(), svcCtx).ListProducts(&product.ListProductsReq{})
	if err != nil {
		t.Fatalf("unexpected list-all error: %v", err)
	}
	assertProductIDsPresent(t, allResp, productIDs)
}

func assertProductIDsPresent(t *testing.T, resp *product.ListProductsResp, expected []int64) {
	t.Helper()
	actual := make(map[int64]struct{}, len(resp.Items))
	for _, item := range resp.Items {
		actual[item.ProductId] = struct{}{}
	}
	for _, id := range expected {
		if _, ok := actual[id]; !ok {
			t.Fatalf("partial snapshot must not drop product %d, got IDs %#v", id, actual)
		}
	}
}
