package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"flash-mall/app/entry/api/internal/svc"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const adminProductTestDSN = "root:6494kj06@tcp(127.0.0.1:3306)/mall_product?charset=utf8mb4&parseTime=true&loc=Local"

func TestAdminProductUpdateHandler_DoesNotOverwriteStatusWhenStatusOmitted(t *testing.T) {
	svcCtx := newAdminProductTestServiceContext(t)
	productID := int64(9401)
	seedAdminProduct(t, svcCtx, productID, 1)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/products/update", strings.NewReader(`{"product_id":9401,"name":"Updated Coat"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	AdminProductUpdateHandler(svcCtx).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	var gotStatus int64
	if err := svcCtx.SqlConn.QueryRowCtx(context.Background(), &gotStatus, "SELECT status FROM product WHERE id = ?", productID); err != nil {
		t.Fatalf("status query failed: %v", err)
	}
	if gotStatus != 1 {
		t.Fatalf("status = %d, want 1", gotStatus)
	}
}

func TestAdminProductUpdateHandler_AllowsStatusOnlyUpdate(t *testing.T) {
	svcCtx := newAdminProductTestServiceContext(t)
	productID := int64(9402)
	seedAdminProduct(t, svcCtx, productID, 1)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/products/update", strings.NewReader(`{"product_id":9402,"status":2}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	AdminProductUpdateHandler(svcCtx).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	var gotStatus int64
	if err := svcCtx.SqlConn.QueryRowCtx(context.Background(), &gotStatus, "SELECT status FROM product WHERE id = ?", productID); err != nil {
		t.Fatalf("status query failed: %v", err)
	}
	if gotStatus != 2 {
		t.Fatalf("status = %d, want 2", gotStatus)
	}
}

func newAdminProductTestServiceContext(t *testing.T) *svc.ServiceContext {
	t.Helper()
	return &svc.ServiceContext{
		SqlConn: sqlx.NewMysql(adminProductTestDSN),
	}
}

func seedAdminProduct(t *testing.T, svcCtx *svc.ServiceContext, productID, status int64) {
	t.Helper()
	ensureAdminProductSchema(t, svcCtx)
	if _, err := svcCtx.SqlConn.ExecCtx(context.Background(), "DELETE FROM product WHERE id = ?", productID); err != nil {
		t.Fatalf("product cleanup failed: %v", err)
	}
	if _, err := svcCtx.SqlConn.ExecCtx(
		context.Background(),
		"INSERT INTO product (id, name, stock, version, origin_price_fen, sale_price_fen, status) VALUES (?, 'Original Coat', 0, 0, 12900, 9900, ?)",
		productID,
		status,
	); err != nil {
		t.Fatalf("product seed failed: %v", err)
	}
	t.Cleanup(func() {
		_, _ = svcCtx.SqlConn.ExecCtx(context.Background(), "DELETE FROM product WHERE id = ?", productID)
	})
}

func ensureAdminProductSchema(t *testing.T, svcCtx *svc.ServiceContext) {
	t.Helper()
	statement := `CREATE TABLE IF NOT EXISTS product (
		id BIGINT NOT NULL AUTO_INCREMENT,
		name VARCHAR(128) NOT NULL,
		stock INT NOT NULL DEFAULT 0,
		version BIGINT NOT NULL DEFAULT 0,
		origin_price_fen BIGINT NOT NULL DEFAULT 0,
		sale_price_fen BIGINT NOT NULL DEFAULT 0,
		supplier_id BIGINT NOT NULL DEFAULT 0,
		status TINYINT NOT NULL DEFAULT 1,
		PRIMARY KEY (id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`
	if _, err := svcCtx.SqlConn.ExecCtx(context.Background(), statement); err != nil {
		t.Fatalf("schema ensure failed: %v", err)
	}
}
