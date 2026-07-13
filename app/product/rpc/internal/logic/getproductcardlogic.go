package logic

import (
	"context"
	"database/sql"
	"errors"

	"flash-mall/app/product/rpc/internal/svc"
	"flash-mall/app/product/rpc/product"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const limitedPricePromotionType = "LIMITED_PRICE"

type GetProductCardLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

type productCardRow struct {
	ID             int64         `db:"id"`
	Name           string        `db:"name"`
	OriginPriceFen int64         `db:"origin_price_fen"`
	SalePriceFen   int64         `db:"sale_price_fen"`
	SupplierID     int64         `db:"supplier_id"`
	LimitedPrice   sql.NullInt64 `db:"limited_price_fen"`
}

type productStockBucketRow struct {
	Stock int64 `db:"stock"`
}

func NewGetProductCardLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetProductCardLogic {
	return &GetProductCardLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetProductCardLogic) GetProductCard(in *product.GetProductCardReq) (*product.GetProductCardResp, error) {
	var row productCardRow
	query := `
SELECT p.id, p.name, p.origin_price_fen, p.sale_price_fen, p.supplier_id,
       MIN(CASE WHEN pr.type = 'LIMITED_PRICE' THEN pr.discount_value END) AS limited_price_fen
FROM product p
LEFT JOIN promotion_rule pr
  ON pr.product_id = p.id
 AND pr.status = 1
 AND (pr.starts_at IS NULL OR pr.starts_at <= NOW())
 AND (pr.ends_at IS NULL OR pr.ends_at >= NOW())
WHERE p.id = ?
  AND p.status = 1
GROUP BY p.id, p.name, p.origin_price_fen, p.sale_price_fen, p.supplier_id`
	if err := l.svcCtx.SqlConn.QueryRowCtx(l.ctx, &row, query, in.ProductId); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "product card not found")
		}
		return nil, err
	}

	var bucketRows []productStockBucketRow
	if err := l.svcCtx.SqlConn.QueryRowsCtx(l.ctx, &bucketRows, "SELECT stock FROM product_stock_bucket WHERE product_id = ?", row.ID); err != nil {
		return nil, err
	}

	var stockAvailable int64
	for _, bucketRow := range bucketRows {
		stockAvailable += bucketRow.Stock
	}

	finalPrice := row.SalePriceFen
	promotionType := ""
	promotionTag := ""
	if row.LimitedPrice.Valid && row.LimitedPrice.Int64 > 0 {
		finalPrice = row.LimitedPrice.Int64
		promotionType = limitedPricePromotionType
		promotionTag = "限时价"
	}

	return &product.GetProductCardResp{
		ProductId:      row.ID,
		Name:           row.Name,
		OriginPriceFen: row.OriginPriceFen,
		FinalPriceFen:  finalPrice,
		PromotionType:  promotionType,
		PromotionTag:   promotionTag,
		StockAvailable: stockAvailable,
		SupplierId:     row.SupplierID,
	}, nil
}
