package handler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"flash-mall/app/common/apperror"
	"flash-mall/app/gateway/hertz/internal/svc"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func gatewayPromotionWindowAffectedProductIDs(ctx context.Context, db *sql.DB, windowMinutes int64, limit int64) ([]int64, error) {
	now := time.Now()
	window := time.Duration(windowMinutes) * time.Minute
	rows, err := db.QueryContext(ctx, `
SELECT DISTINCT product_id
FROM mall_product.promotion_rule
WHERE status = 1
  AND (
    ((starts_at IS NULL OR starts_at <= NOW()) AND (ends_at IS NULL OR ends_at >= NOW()))
    OR (starts_at IS NOT NULL AND starts_at BETWEEN ? AND ?)
    OR (ends_at IS NOT NULL AND ends_at BETWEEN ? AND ?)
  )
ORDER BY product_id
LIMIT ?`, now.Add(-window), now.Add(window), now.Add(-window), now.Add(window), limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	productIDs := make([]int64, 0)
	for rows.Next() {
		var productID int64
		if err := rows.Scan(&productID); err != nil {
			return nil, err
		}
		if productID > 0 {
			productIDs = append(productIDs, productID)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return productIDs, nil
}

func uniquePositiveInt64s(values []int64) []int64 {
	seen := make(map[int64]struct{}, len(values))
	result := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

type sqlQueryRower interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func activeSupplierExists(ctx context.Context, q sqlQueryRower, supplierID int64) (bool, error) {
	var exists int64
	if err := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM mall_product.supplier WHERE id = ? AND status = 1", supplierID).Scan(&exists); err != nil {
		return false, err
	}
	return exists > 0, nil
}

func activeMerchantExists(ctx context.Context, q sqlQueryRower, merchantID int64) (bool, error) {
	var exists int64
	if err := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM mall_order.merchant WHERE id = ? AND status = 1", merchantID).Scan(&exists); err != nil {
		return false, err
	}
	return exists > 0, nil
}

func nextProductID(ctx context.Context, tx *sql.Tx) (int64, error) {
	var maxProductID int64
	if err := tx.QueryRowContext(ctx, "SELECT id FROM mall_product.product ORDER BY id DESC LIMIT 1 FOR UPDATE").Scan(&maxProductID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 100, nil
		}
		return 0, err
	}
	return maxProductID + 1, nil
}

func writeStockAdjustError(ctx context.Context, c *app.RequestContext, svcCtx *svc.ServiceContext, req AdminProductStockAdjustReq, err error) {
	switch apperror.CodeOf(err) {
	case apperror.CodeStockNotFound, apperror.CodeProductNotFound, apperror.CodeNotFound:
		recordGatewayAdminAuditFailure(c, svcCtx, adminAuditProductStockAdjusted, fmt.Sprintf("product:%d reason:%s", req.ProductID, adminAuditReasonNotFound))
		fail(ctx, c, consts.StatusNotFound, apperror.New(apperror.CodeProductNotFound, "product not found"))
	case apperror.CodeStockInsufficient:
		recordGatewayAdminAuditFailure(c, svcCtx, adminAuditProductStockAdjusted, fmt.Sprintf("product:%d delta:%d bucket:%d reason:%s", req.ProductID, req.Delta, req.BucketIdx, adminAuditReasonInsufficientOrMissingStock))
		fail(ctx, c, consts.StatusConflict, apperror.New(apperror.CodeStockInsufficient, "stock bucket not found or insufficient stock"))
	default:
		fail(ctx, c, consts.StatusBadGateway, apperror.Wrap(apperror.CodeInternal, "admin product stock adjust failed", err))
	}
}

func productPricePairValid(ctx context.Context, db *sql.DB, productID int64, originPriceFen, salePriceFen *int64) (bool, bool, error) {
	var origin int64
	var sale int64
	if originPriceFen == nil || salePriceFen == nil {
		if err := db.QueryRowContext(ctx, "SELECT origin_price_fen, sale_price_fen FROM mall_product.product WHERE id = ?", productID).Scan(&origin, &sale); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return false, false, nil
			}
			return false, false, err
		}
	}
	if originPriceFen != nil {
		origin = *originPriceFen
	}
	if salePriceFen != nil {
		sale = *salePriceFen
	}
	return sale <= origin, true, nil
}
