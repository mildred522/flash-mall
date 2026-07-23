package productmysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"flash-mall/app/gateway/hertz/internal/application/catalogquery"
)

type CatalogRepository struct{ db *sql.DB }

var _ catalogquery.Repository = (*CatalogRepository)(nil)

func NewCatalogRepository(db *sql.DB) *CatalogRepository { return &CatalogRepository{db: db} }

func (r *CatalogRepository) StoreDetail(ctx context.Context, merchantID int64) (catalogquery.StoreDetail, bool, error) {
	var detail catalogquery.StoreDetail
	err := r.db.QueryRowContext(ctx, `SELECT m.id, m.name,
       COALESCE(profile.logo_url, ''), COALESCE(profile.banner_url, ''),
       COALESCE(profile.description, ''), m.status,
       (SELECT COUNT(*) FROM mall_product.product p WHERE p.merchant_id = m.id AND p.status = 1)
FROM mall_order.merchant m
LEFT JOIN mall_order.merchant_store_profile profile ON profile.merchant_id = m.id
WHERE m.id = ? AND m.status = 1`, merchantID).Scan(
		&detail.MerchantID, &detail.MerchantName, &detail.LogoURL, &detail.BannerURL,
		&detail.Description, &detail.Status, &detail.ProductCount,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return catalogquery.StoreDetail{}, false, nil
	}
	return detail, err == nil, err
}

func (r *CatalogRepository) ProductMetadata(ctx context.Context, productIDs []int64) (map[int64]catalogquery.ProductMeta, error) {
	result := make(map[int64]catalogquery.ProductMeta, len(productIDs))
	if len(productIDs) == 0 {
		return result, nil
	}
	placeholders := make([]string, 0, len(productIDs))
	args := make([]any, 0, len(productIDs))
	for _, productID := range productIDs {
		placeholders = append(placeholders, "?")
		args = append(args, productID)
	}
	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(`
SELECT p.id, COALESCE(p.image_url, ''), COALESCE(s.name, ''),
       p.merchant_id, COALESCE(m.name, ''), COALESCE(profile.logo_url, ''),
       COALESCE(m.status, 0), p.status
FROM mall_product.product p
LEFT JOIN mall_product.supplier s ON s.id = p.supplier_id
LEFT JOIN mall_order.merchant m ON m.id = p.merchant_id
LEFT JOIN mall_order.merchant_store_profile profile ON profile.merchant_id = p.merchant_id
WHERE p.id IN (%s)`, strings.Join(placeholders, ",")), args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var productID int64
		var meta catalogquery.ProductMeta
		if err := rows.Scan(&productID, &meta.ImageURL, &meta.SupplierName, &meta.MerchantID,
			&meta.MerchantName, &meta.MerchantLogo, &meta.StoreStatus, &meta.ProductStatus); err != nil {
			return nil, err
		}
		result[productID] = meta
	}
	return result, rows.Err()
}
