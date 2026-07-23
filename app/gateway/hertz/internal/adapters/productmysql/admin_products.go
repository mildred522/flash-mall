package productmysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"flash-mall/app/gateway/hertz/internal/application/catalogquery"
)

const adminProductSelect = `SELECT p.id, p.merchant_id, COALESCE(m.name, ''), p.name, COALESCE(p.image_url, ''),
       p.origin_price_fen, p.sale_price_fen, p.supplier_id, COALESCE(s.name, ''),
       COALESCE(snap.available, stock.stock_available, 0),
       COALESCE(promo.promotion_price_fen, 0), p.status
FROM mall_product.product p
LEFT JOIN mall_order.merchant m ON m.id = p.merchant_id
LEFT JOIN mall_product.supplier s ON s.id = p.supplier_id
LEFT JOIN mall_product.product_stock_snapshot snap ON snap.product_id = p.id
LEFT JOIN (
  SELECT product_id, COALESCE(SUM(stock), 0) AS stock_available
  FROM mall_product.product_stock_bucket GROUP BY product_id
) stock ON stock.product_id = p.id
LEFT JOIN (
  SELECT product_id, MIN(discount_value) AS promotion_price_fen
  FROM mall_product.promotion_rule
  WHERE type = 'LIMITED_PRICE' AND status = 1
    AND (starts_at IS NULL OR starts_at <= NOW())
    AND (ends_at IS NULL OR ends_at >= NOW())
  GROUP BY product_id
) promo ON promo.product_id = p.id`

func (r *CatalogRepository) AdminProducts(ctx context.Context, query catalogquery.ListQuery) ([]catalogquery.AdminProduct, int64, error) {
	where, args, err := r.productWhere(ctx, query)
	if err != nil {
		return nil, 0, err
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM mall_product.product p WHERE %s", where), args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []catalogquery.AdminProduct{}, 0, nil
	}
	queryArgs := append(append([]any{}, args...), query.PageSize, (query.Page-1)*query.PageSize)
	rows, err := r.db.QueryContext(ctx, adminProductSelect+fmt.Sprintf("\nWHERE %s ORDER BY p.id DESC LIMIT ? OFFSET ?", where), queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]catalogquery.AdminProduct, 0, query.PageSize)
	for rows.Next() {
		var item catalogquery.AdminProduct
		if err := scanAdminProduct(rows.Scan, &item); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (r *CatalogRepository) AdminProductDetail(ctx context.Context, productID int64) (catalogquery.AdminProduct, bool, error) {
	var item catalogquery.AdminProduct
	err := scanAdminProduct(r.db.QueryRowContext(ctx, adminProductSelect+"\nWHERE p.id = ?", productID).Scan, &item)
	if errors.Is(err, sql.ErrNoRows) {
		return catalogquery.AdminProduct{}, false, nil
	}
	return item, err == nil, err
}

type scanValues func(...any) error

func scanAdminProduct(scan scanValues, item *catalogquery.AdminProduct) error {
	return scan(&item.ProductID, &item.MerchantID, &item.MerchantName, &item.Name, &item.ImageURL,
		&item.OriginPriceFen, &item.SalePriceFen, &item.SupplierID, &item.SupplierName,
		&item.StockAvailable, &item.PromotionPriceFen, &item.Status)
}
