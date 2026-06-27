package handler

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"sync"

	"flash-mall/app/entry/api/internal/svc"
)

var productImageColumnMu sync.Mutex
var productImageColumnReady bool

func ensureProductImageColumn(ctx context.Context, db *sql.DB) error {
	productImageColumnMu.Lock()
	defer productImageColumnMu.Unlock()
	if productImageColumnReady {
		return nil
	}

	var exists int64
	err := db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = 'mall_product'
  AND TABLE_NAME = 'product'
  AND COLUMN_NAME = 'image_url'`).Scan(&exists)
	if err != nil {
		return err
	}
	if exists == 0 {
		if _, err := db.ExecContext(ctx, "ALTER TABLE mall_product.product ADD COLUMN image_url varchar(512) NOT NULL DEFAULT ''"); err != nil {
			return err
		}
	}
	productImageColumnReady = true
	return nil
}

func productUploadDir(svcCtx *svc.ServiceContext) string {
	if dir := strings.TrimSpace(svcCtx.Config.UploadDir); dir != "" {
		return dir
	}
	return filepath.Join(".runtime", "uploads")
}
