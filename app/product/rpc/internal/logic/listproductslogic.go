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
       MIN(CASE WHEN pr.type = 'LIMITED_PRICE' THEN pr.discount_value END) AS limited_price_fen
FROM product p
LEFT JOIN promotion_rule pr
  ON pr.product_id = p.id
 AND pr.status = 1
 AND (pr.starts_at IS NULL OR pr.starts_at <= NOW())
 AND (pr.ends_at IS NULL OR pr.ends_at >= NOW())
WHERE p.id IN (%s)
  AND p.status = 1
GROUP BY p.id, p.name, p.origin_price_fen, p.sale_price_fen, p.supplier_id`,
			strings.Join(placeholders, ","))
		if err := l.svcCtx.SqlConn.QueryRowsCtx(l.ctx, &rows, query, args...); err != nil {
			return nil, err
		}
	} else {
		// Query all active products (with optional pagination)
		query := `
SELECT p.id, p.name, p.origin_price_fen, p.sale_price_fen, p.supplier_id,
       MIN(CASE WHEN pr.type = 'LIMITED_PRICE' THEN pr.discount_value END) AS limited_price_fen
FROM product p
LEFT JOIN promotion_rule pr
  ON pr.product_id = p.id
 AND pr.status = 1
 AND (pr.starts_at IS NULL OR pr.starts_at <= NOW())
 AND (pr.ends_at IS NULL OR pr.ends_at >= NOW())
WHERE p.status = 1
GROUP BY p.id, p.name, p.origin_price_fen, p.sale_price_fen, p.supplier_id
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
	stockQuery := fmt.Sprintf(
		"SELECT product_id, COALESCE(SUM(stock), 0) AS stock FROM product_stock_bucket WHERE product_id IN (%s) GROUP BY product_id",
		strings.Join(placeholders, ","),
	)
	type stockRow struct {
		ProductID int64 `db:"product_id"`
		Stock     int64 `db:"stock"`
	}
	var stockRows []stockRow
	if err := l.svcCtx.SqlConn.QueryRowsCtx(l.ctx, &stockRows, stockQuery, productIDs...); err != nil && err != sqlx.ErrNotFound {
		return nil, err
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
