package logic

import (
	"context"
	"fmt"
	"strings"

	"flash-mall/app/product/rpc/internal/svc"
	"flash-mall/app/product/rpc/product"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ListProductsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListProductsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListProductsLogic {
	return &ListProductsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 批量查询商品
func (l *ListProductsLogic) ListProducts(in *product.ListProductsReq) (*product.ListProductsResp, error) {
	if resp, ok, err := l.listProductCardSnapshots(in); err == nil && ok {
		return resp, nil
	}

	var rows []productCardRow

	if len(in.ProductIds) > 0 {
		// Batch query by specific IDs
		placeholders := make([]string, len(in.ProductIds))
		args := make([]interface{}, len(in.ProductIds))
		for i, id := range in.ProductIds {
			placeholders[i] = "?"
			args[i] = id
		}
		query := fmt.Sprintf(`
SELECT p.id, p.name, p.origin_price_fen, p.sale_price_fen, p.supplier_id,
       COALESCE(p.image_url, '') AS image_url, COALESCE(p.merchant_id, 1000) AS merchant_id,
       MIN(CASE WHEN pr.type = 'LIMITED_PRICE' THEN pr.discount_value END) AS limited_price_fen
FROM product p
LEFT JOIN promotion_rule pr
  ON pr.product_id = p.id
 AND pr.status = 1
 AND (pr.starts_at IS NULL OR pr.starts_at <= NOW())
 AND (pr.ends_at IS NULL OR pr.ends_at >= NOW())
WHERE p.id IN (%s)
  AND p.status = 1
GROUP BY p.id, p.name, p.origin_price_fen, p.sale_price_fen, p.supplier_id, p.image_url, p.merchant_id`,
			strings.Join(placeholders, ","))
		if err := l.svcCtx.SqlConn.QueryRowsCtx(l.ctx, &rows, query, args...); err != nil {
			return nil, err
		}
	} else {
		// Query all active products (with optional pagination)
		query := `
SELECT p.id, p.name, p.origin_price_fen, p.sale_price_fen, p.supplier_id,
       COALESCE(p.image_url, '') AS image_url, COALESCE(p.merchant_id, 1000) AS merchant_id,
       MIN(CASE WHEN pr.type = 'LIMITED_PRICE' THEN pr.discount_value END) AS limited_price_fen
FROM product p
LEFT JOIN promotion_rule pr
  ON pr.product_id = p.id
 AND pr.status = 1
 AND (pr.starts_at IS NULL OR pr.starts_at <= NOW())
 AND (pr.ends_at IS NULL OR pr.ends_at >= NOW())
WHERE p.status = 1
GROUP BY p.id, p.name, p.origin_price_fen, p.sale_price_fen, p.supplier_id, p.image_url, p.merchant_id
ORDER BY p.id`
		if in.PageSize > 0 {
			offset := (in.PageNum - 1) * in.PageSize
			if offset < 0 {
				offset = 0
			}
			query += fmt.Sprintf(" LIMIT %d OFFSET %d", in.PageSize, offset)
		}
		if err := l.svcCtx.SqlConn.QueryRowsCtx(l.ctx, &rows, query); err != nil {
			return nil, err
		}
	}

	if len(rows) == 0 {
		return &product.ListProductsResp{Items: nil, Total: 0}, nil
	}

	// Batch query stock for all products
	productIDs := make([]interface{}, len(rows))
	for i, row := range rows {
		productIDs[i] = row.ID
	}
	placeholders := make([]string, len(rows))
	for i := range rows {
		placeholders[i] = "?"
	}
	stockQuery := fmt.Sprintf(`
SELECT ids.product_id,
       COALESCE(s.available, bucket.stock, 0) AS stock
FROM (
  SELECT ? AS product_id%s
) ids
LEFT JOIN product_stock_snapshot s ON s.product_id = ids.product_id
LEFT JOIN (
  SELECT product_id, COALESCE(SUM(stock), 0) AS stock
  FROM product_stock_bucket
  WHERE product_id IN (%s)
  GROUP BY product_id
) bucket ON bucket.product_id = ids.product_id`,
		unionProductIDSelects(len(productIDs)-1),
		strings.Join(placeholders, ","),
	)
	type stockRow struct {
		ProductID int64 `db:"product_id"`
		Stock     int64 `db:"stock"`
	}
	var stockRows []stockRow
	stockArgs := append(append([]interface{}{}, productIDs...), productIDs...)
	if err := l.svcCtx.SqlConn.QueryRowsCtx(l.ctx, &stockRows, stockQuery, stockArgs...); err != nil && err != sqlx.ErrNotFound {
		fallbackQuery := fmt.Sprintf(`
SELECT product_id, COALESCE(SUM(stock), 0) AS stock
FROM product_stock_bucket
WHERE product_id IN (%s)
GROUP BY product_id`, strings.Join(placeholders, ","))
		if err := l.svcCtx.SqlConn.QueryRowsCtx(l.ctx, &stockRows, fallbackQuery, productIDs...); err != nil && err != sqlx.ErrNotFound {
			return nil, err
		}
	}
	stockMap := make(map[int64]int64, len(stockRows))
	for _, sr := range stockRows {
		stockMap[sr.ProductID] = sr.Stock
	}

	// Build response
	items := make([]*product.GetProductCardResp, 0, len(rows))
	for _, row := range rows {
		finalPrice := row.SalePriceFen
		promotionType := ""
		promotionTag := ""
		if row.LimitedPrice.Valid && row.LimitedPrice.Int64 > 0 {
			finalPrice = row.LimitedPrice.Int64
			promotionType = limitedPricePromotionType
			promotionTag = "限时价"
		}
		items = append(items, &product.GetProductCardResp{
			ProductId:      row.ID,
			Name:           row.Name,
			OriginPriceFen: row.OriginPriceFen,
			FinalPriceFen:  finalPrice,
			PromotionType:  promotionType,
			PromotionTag:   promotionTag,
			StockAvailable: stockMap[row.ID],
			SupplierId:     row.SupplierID,
			ImageUrl:       row.ImageURL,
			MerchantId:     row.MerchantID,
		})
	}

	// Get total count if pagination was requested
	var total int64
	if in.PageSize > 0 {
		var countRow struct {
			Count int64 `db:"count"`
		}
		if err := l.svcCtx.SqlConn.QueryRowCtx(l.ctx, &countRow, "SELECT COUNT(*) AS count FROM product WHERE status = 1"); err == nil {
			total = countRow.Count
		}
	} else {
		total = int64(len(items))
	}

	return &product.ListProductsResp{
		Items: items,
		Total: total,
	}, nil
}

func (l *ListProductsLogic) listProductCardSnapshots(in *product.ListProductsReq) (*product.ListProductsResp, bool, error) {
	var rows []productCardSnapshotRow
	if len(in.ProductIds) > 0 {
		placeholders := make([]string, len(in.ProductIds))
		args := make([]interface{}, len(in.ProductIds))
		for i, id := range in.ProductIds {
			placeholders[i] = "?"
			args[i] = id
		}
		query := fmt.Sprintf(`SELECT snapshot.product_id, snapshot.name, snapshot.origin_price_fen, snapshot.final_price_fen,
       snapshot.promotion_type, snapshot.promotion_tag, snapshot.stock_available, snapshot.supplier_id,
       COALESCE(product.image_url, '') AS image_url, COALESCE(product.merchant_id, 1000) AS merchant_id
FROM product_card_snapshot AS snapshot
JOIN product ON product.id = snapshot.product_id
WHERE snapshot.product_id IN (%s) AND snapshot.status = 1
ORDER BY snapshot.product_id`, strings.Join(placeholders, ","))
		if err := l.svcCtx.SqlConn.QueryRowsCtx(l.ctx, &rows, query, args...); err != nil && err != sqlx.ErrNotFound {
			return nil, false, err
		}
		// A snapshot hit is only complete when every requested product has an
		// active card. Treating a partial hit as complete silently drops the
		// remaining products from callers such as the homepage showcase.
		requestedIDs := make(map[int64]struct{}, len(in.ProductIds))
		for _, id := range in.ProductIds {
			requestedIDs[id] = struct{}{}
		}
		if len(rows) != len(requestedIDs) {
			return nil, false, nil
		}
		return &product.ListProductsResp{Items: productCardSnapshotsToResp(rows), Total: int64(len(rows))}, true, nil
	}

	query := `SELECT snapshot.product_id, snapshot.name, snapshot.origin_price_fen, snapshot.final_price_fen,
       snapshot.promotion_type, snapshot.promotion_tag, snapshot.stock_available, snapshot.supplier_id,
       COALESCE(product.image_url, '') AS image_url, COALESCE(product.merchant_id, 1000) AS merchant_id
FROM product_card_snapshot AS snapshot
JOIN product ON product.id = snapshot.product_id
WHERE snapshot.status = 1
ORDER BY snapshot.product_id`
	if in.PageSize > 0 {
		offset := (in.PageNum - 1) * in.PageSize
		if offset < 0 {
			offset = 0
		}
		query += fmt.Sprintf(" LIMIT %d OFFSET %d", in.PageSize, offset)
	}
	if err := l.svcCtx.SqlConn.QueryRowsCtx(l.ctx, &rows, query); err != nil && err != sqlx.ErrNotFound {
		return nil, false, err
	}
	if len(rows) == 0 {
		return nil, false, nil
	}
	var completeness struct {
		ProductCount     int64 `db:"product_count"`
		MissingSnapshots int64 `db:"missing_snapshots"`
		StaleSnapshots   int64 `db:"stale_snapshots"`
	}
	completenessQuery := `SELECT
  (SELECT COUNT(*) FROM product WHERE status = 1) AS product_count,
	(SELECT COUNT(*)
	 FROM product p
	 LEFT JOIN product_card_snapshot s ON s.product_id = p.id AND s.status = 1
	 WHERE p.status = 1 AND s.product_id IS NULL) AS missing_snapshots,
	(SELECT COUNT(*)
	 FROM product_card_snapshot s
	 LEFT JOIN product p ON p.id = s.product_id AND p.status = 1
	 WHERE s.status = 1 AND p.id IS NULL) AS stale_snapshots`
	if err := l.svcCtx.SqlConn.QueryRowCtx(l.ctx, &completeness, completenessQuery); err != nil {
		return nil, false, err
	}
	if completeness.MissingSnapshots > 0 || completeness.StaleSnapshots > 0 {
		return nil, false, nil
	}
	return &product.ListProductsResp{Items: productCardSnapshotsToResp(rows), Total: completeness.ProductCount}, true, nil
}

func productCardSnapshotsToResp(rows []productCardSnapshotRow) []*product.GetProductCardResp {
	items := make([]*product.GetProductCardResp, 0, len(rows))
	for _, row := range rows {
		items = append(items, productCardSnapshotToResp(row))
	}
	return items
}

func unionProductIDSelects(count int) string {
	if count <= 0 {
		return ""
	}
	parts := make([]string, 0, count)
	for i := 0; i < count; i++ {
		parts = append(parts, " UNION ALL SELECT ?")
	}
	return strings.Join(parts, "")
}
