package handler

import (
	"context"
	"fmt"

	"flash-mall/app/gateway/hertz/internal/svc"
)

func loadAdminProducts(ctx context.Context, svcCtx *svc.ServiceContext, req productListQuery) ([]AdminProductItem, int64, error) {
	db, err := svcCtx.SqlConn.RawDB()
	if err != nil {
		return nil, 0, err
	}
	if err := requireGatewayProductReadSchema(ctx, db); err != nil {
		return nil, 0, err
	}
	where, args, err := productWhereClause(ctx, db, req)
	if err != nil {
		return nil, 0, err
	}

	var total int64
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM mall_product.product p WHERE %s", where)
	if err := db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []AdminProductItem{}, 0, nil
	}

	offset := (req.Page - 1) * req.PageSize
	query := fmt.Sprintf(`
SELECT p.id, p.merchant_id, COALESCE(m.name, ''), p.name, COALESCE(p.image_url, ''),
       p.origin_price_fen, p.sale_price_fen, p.supplier_id, COALESCE(s.name, ''),
       COALESCE(snap.available, stock.stock_available, 0),
       COALESCE(promo.promotion_price_fen, 0),
       p.status
FROM mall_product.product p
LEFT JOIN mall_order.merchant m ON m.id = p.merchant_id
LEFT JOIN mall_product.supplier s ON s.id = p.supplier_id
LEFT JOIN mall_product.product_stock_snapshot snap ON snap.product_id = p.id
LEFT JOIN (
  SELECT product_id, COALESCE(SUM(stock), 0) AS stock_available
  FROM mall_product.product_stock_bucket
  GROUP BY product_id
) stock ON stock.product_id = p.id
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
ORDER BY p.id DESC LIMIT ? OFFSET ?`, where)

	queryArgs := append(append([]any{}, args...), req.PageSize, offset)
	rows, err := db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]AdminProductItem, 0, req.PageSize)
	for rows.Next() {
		var item AdminProductItem
		if err := rows.Scan(
			&item.ProductID,
			&item.MerchantID,
			&item.MerchantName,
			&item.Name,
			&item.ImageURL,
			&item.OriginPriceFen,
			&item.SalePriceFen,
			&item.SupplierID,
			&item.SupplierName,
			&item.StockAvailable,
			&item.PromotionPriceFen,
			&item.Status,
		); err != nil {
			return nil, 0, err
		}
		item.StatusText = productStatusText(item.Status)
		item.PromotionText = productPromotionText(item.PromotionPriceFen)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func loadAdminProductDetail(ctx context.Context, svcCtx *svc.ServiceContext, productID int64) (AdminProductItem, error) {
	db, err := svcCtx.SqlConn.RawDB()
	if err != nil {
		return AdminProductItem{}, err
	}
	if err := requireGatewayProductReadSchema(ctx, db); err != nil {
		return AdminProductItem{}, err
	}

	var item AdminProductItem
	err = db.QueryRowContext(ctx, `
SELECT p.id, p.merchant_id, COALESCE(m.name, ''), p.name, COALESCE(p.image_url, ''),
       p.origin_price_fen, p.sale_price_fen, p.supplier_id, COALESCE(s.name, ''),
       COALESCE(snap.available, stock.stock_available, 0),
       COALESCE(promo.promotion_price_fen, 0),
       p.status
FROM mall_product.product p
LEFT JOIN mall_order.merchant m ON m.id = p.merchant_id
LEFT JOIN mall_product.supplier s ON s.id = p.supplier_id
LEFT JOIN mall_product.product_stock_snapshot snap ON snap.product_id = p.id
LEFT JOIN (
  SELECT product_id, COALESCE(SUM(stock), 0) AS stock_available
  FROM mall_product.product_stock_bucket
  WHERE product_id = ?
  GROUP BY product_id
) stock ON stock.product_id = p.id
LEFT JOIN (
  SELECT product_id, MIN(discount_value) AS promotion_price_fen
  FROM mall_product.promotion_rule
  WHERE product_id = ?
    AND type = 'LIMITED_PRICE'
    AND status = 1
    AND (starts_at IS NULL OR starts_at <= NOW())
    AND (ends_at IS NULL OR ends_at >= NOW())
  GROUP BY product_id
) promo ON promo.product_id = p.id
WHERE p.id = ?`, productID, productID, productID).Scan(
		&item.ProductID,
		&item.MerchantID,
		&item.MerchantName,
		&item.Name,
		&item.ImageURL,
		&item.OriginPriceFen,
		&item.SalePriceFen,
		&item.SupplierID,
		&item.SupplierName,
		&item.StockAvailable,
		&item.PromotionPriceFen,
		&item.Status,
	)
	if err != nil {
		return AdminProductItem{}, err
	}
	item.StatusText = productStatusText(item.Status)
	item.PromotionText = productPromotionText(item.PromotionPriceFen)
	return item, nil
}

func productStatusText(status int64) string {
	switch status {
	case 1:
		return "上架"
	case 2:
		return "下架"
	default:
		return "未知"
	}
}

func productPromotionText(priceFen int64) string {
	if priceFen > 0 {
		return "限时价"
	}
	return "无活动"
}
