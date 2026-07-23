package productmysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"flash-mall/app/gateway/hertz/internal/application/promotion"
)

type PromotionRepository struct{ db *sql.DB }

var _ promotion.Repository = (*PromotionRepository)(nil)

func NewPromotionRepository(db *sql.DB) *PromotionRepository {
	return &PromotionRepository{db: db}
}

func (r *PromotionRepository) List(ctx context.Context, query promotion.ListQuery) ([]promotion.Record, int64, error) {
	where, args := promotionWhereClause(query)
	var total int64
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM mall_product.promotion_rule pr LEFT JOIN mall_product.product p ON p.id = pr.product_id WHERE %s", where)
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []promotion.Record{}, 0, nil
	}

	offset := (query.Page - 1) * query.PageSize
	statement := fmt.Sprintf(`
SELECT pr.id, pr.product_id, COALESCE(p.name, ''), COALESCE(p.origin_price_fen, 0), COALESCE(p.sale_price_fen, 0),
       pr.type, pr.discount_value, pr.threshold_amount, pr.starts_at, pr.ends_at, pr.status
FROM mall_product.promotion_rule pr
LEFT JOIN mall_product.product p ON p.id = pr.product_id
WHERE %s
ORDER BY pr.id DESC
LIMIT ? OFFSET ?`, where)
	queryArgs := append(append([]any{}, args...), query.PageSize, offset)
	rows, err := r.db.QueryContext(ctx, statement, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]promotion.Record, 0, query.PageSize)
	for rows.Next() {
		item, err := scanPromotion(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *PromotionRepository) Find(ctx context.Context, promotionID int64) (promotion.Record, bool, error) {
	record, err := scanPromotion(r.db.QueryRowContext(ctx, `
SELECT pr.id, pr.product_id, COALESCE(p.name, ''), COALESCE(p.origin_price_fen, 0), COALESCE(p.sale_price_fen, 0),
       pr.type, pr.discount_value, pr.threshold_amount, pr.starts_at, pr.ends_at, pr.status
FROM mall_product.promotion_rule pr
LEFT JOIN mall_product.product p ON p.id = pr.product_id
WHERE pr.id = ?`, promotionID))
	if errors.Is(err, sql.ErrNoRows) {
		return promotion.Record{}, false, nil
	}
	if err != nil {
		return promotion.Record{}, false, err
	}
	return record, true, nil
}

func (r *PromotionRepository) ProductSalePrice(ctx context.Context, productID int64) (int64, bool, error) {
	var salePrice int64
	err := r.db.QueryRowContext(ctx, "SELECT sale_price_fen FROM mall_product.product WHERE id = ?", productID).Scan(&salePrice)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return salePrice, true, nil
}

func (r *PromotionRepository) HasActiveConflict(ctx context.Context, productID, excludePromotionID int64, startsAt, endsAt *time.Time) (bool, error) {
	var count int64
	err := r.db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM mall_product.promotion_rule
WHERE product_id = ?
  AND type = ?
  AND status = 1
  AND id <> ?
  AND (? IS NULL OR ends_at IS NULL OR ends_at >= ?)
  AND (? IS NULL OR starts_at IS NULL OR starts_at <= ?)`,
		productID, promotion.TypeLimitedPrice, excludePromotionID,
		nullableTime(startsAt), nullableTime(startsAt),
		nullableTime(endsAt), nullableTime(endsAt),
	).Scan(&count)
	return count > 0, err
}

func (r *PromotionRepository) Insert(ctx context.Context, record promotion.Record) (int64, error) {
	result, err := r.db.ExecContext(ctx, `
INSERT INTO mall_product.promotion_rule (product_id, type, discount_value, threshold_amount, starts_at, ends_at, status)
VALUES (?, ?, ?, ?, ?, ?, ?)`, record.ProductID, record.Type, record.DiscountValue, record.ThresholdAmount,
		nullableTime(record.StartsAt), nullableTime(record.EndsAt), record.Status)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (r *PromotionRepository) Update(ctx context.Context, promotionID int64, changes promotion.Changes) (bool, error) {
	setClauses := make([]string, 0, 6)
	args := make([]any, 0, 7)
	appendChange := func(column string, value any) {
		setClauses = append(setClauses, column+" = ?")
		args = append(args, value)
	}
	if changes.ProductID != nil {
		appendChange("product_id", *changes.ProductID)
	}
	if changes.DiscountValue != nil {
		appendChange("discount_value", *changes.DiscountValue)
	}
	if changes.ThresholdAmount != nil {
		appendChange("threshold_amount", *changes.ThresholdAmount)
	}
	if changes.StartsAt.Set {
		appendChange("starts_at", nullableTime(changes.StartsAt.Value))
	}
	if changes.EndsAt.Set {
		appendChange("ends_at", nullableTime(changes.EndsAt.Value))
	}
	if changes.Status != nil {
		appendChange("status", *changes.Status)
	}
	if len(setClauses) == 0 {
		return false, errors.New("promotion update has no changes")
	}
	args = append(args, promotionID)
	result, err := r.db.ExecContext(ctx, fmt.Sprintf("UPDATE mall_product.promotion_rule SET %s WHERE id = ?", strings.Join(setClauses, ", ")), args...)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if affected > 0 {
		return true, nil
	}
	var count int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mall_product.promotion_rule WHERE id = ?", promotionID).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

type promotionScanner interface {
	Scan(...any) error
}

func scanPromotion(scanner promotionScanner) (promotion.Record, error) {
	var record promotion.Record
	var startsAt, endsAt sql.NullTime
	if err := scanner.Scan(
		&record.ID, &record.ProductID, &record.ProductName, &record.OriginPriceFen, &record.SalePriceFen,
		&record.Type, &record.DiscountValue, &record.ThresholdAmount, &startsAt, &endsAt, &record.Status,
	); err != nil {
		return promotion.Record{}, err
	}
	if startsAt.Valid {
		record.StartsAt = &startsAt.Time
	}
	if endsAt.Valid {
		record.EndsAt = &endsAt.Time
	}
	return record, nil
}

func promotionWhereClause(query promotion.ListQuery) (string, []any) {
	where := "1=1"
	args := make([]any, 0, 3)
	if query.ProductID > 0 {
		where += " AND pr.product_id = ?"
		args = append(args, query.ProductID)
	}
	if query.Status >= 0 {
		where += " AND pr.status = ?"
		args = append(args, query.Status)
	}
	switch strings.TrimSpace(query.EffectStatus) {
	case "active":
		where += " AND pr.status = 1 AND (pr.starts_at IS NULL OR pr.starts_at <= NOW()) AND (pr.ends_at IS NULL OR pr.ends_at >= NOW())"
	case "scheduled":
		where += " AND pr.status = 1 AND pr.starts_at IS NOT NULL AND pr.starts_at > NOW()"
	case "expired":
		where += " AND pr.status = 1 AND pr.ends_at IS NOT NULL AND pr.ends_at < NOW()"
	case "inactive":
		where += " AND pr.status <> 1"
	}
	if query.Keyword != "" {
		where += " AND p.name LIKE ?"
		args = append(args, "%"+query.Keyword+"%")
	}
	return where, args
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return *value
}
