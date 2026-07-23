package productmysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"flash-mall/app/gateway/hertz/internal/application/showcase"
)

type ShowcaseRepository struct{ db *sql.DB }

var _ showcase.Repository = (*ShowcaseRepository)(nil)

func NewShowcaseRepository(db *sql.DB) *ShowcaseRepository { return &ShowcaseRepository{db: db} }

func (r *ShowcaseRepository) Load(ctx context.Context) (showcase.Layout, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT showcase.version, showcase.operator_id,
       COALESCE(DATE_FORMAT(showcase.publish_time, '%Y-%m-%d %H:%i:%s'), ''),
       COALESCE(item.slot_no, 0), COALESCE(item.product_id, 0),
       IF(product.id IS NULL, 0, 1), COALESCE(product.status, 0),
       COALESCE(product.merchant_id, 0), IF(merchant.id IS NULL, 0, 1),
       COALESCE(merchant.status, 0), COALESCE(stock.available, product.stock, 0)
FROM mall_product.homepage_showcase showcase
LEFT JOIN mall_product.homepage_showcase_item item ON item.showcase_id = showcase.id
LEFT JOIN mall_product.product product ON product.id = item.product_id
LEFT JOIN mall_order.merchant merchant ON merchant.id = product.merchant_id
LEFT JOIN mall_product.product_stock_snapshot stock ON stock.product_id = product.id
WHERE showcase.id = ?
ORDER BY item.slot_no ASC`, showcase.CurrentID)
	if err != nil {
		return showcase.Layout{}, err
	}
	defer func() { _ = rows.Close() }()

	result := emptyShowcaseLayout()
	found := false
	for rows.Next() {
		found = true
		var slotNo, productID, productExists, productStatus int64
		var merchantID, merchantExists, merchantStatus, stockAvailable int64
		if err := rows.Scan(&result.Version, &result.OperatorID, &result.PublishTime,
			&slotNo, &productID, &productExists, &productStatus, &merchantID,
			&merchantExists, &merchantStatus, &stockAvailable); err != nil {
			return showcase.Layout{}, err
		}
		if slotNo < 1 || slotNo > 12 || productID <= 0 {
			continue
		}
		reason := showcase.InvalidReason(showcase.SlotState{
			ProductExists: productExists == 1, ProductStatus: productStatus,
			MerchantExists: merchantExists == 1, MerchantStatus: merchantStatus,
			StockAvailable: stockAvailable,
		})
		result.Items[slotNo-1] = showcase.Slot{SlotNo: slotNo, ProductID: productID, Valid: reason == "", InvalidReason: reason}
	}
	if err := rows.Err(); err != nil {
		return showcase.Layout{}, err
	}
	if !found {
		return showcase.Layout{}, sql.ErrNoRows
	}
	return result, nil
}

func (r *ShowcaseRepository) WithPublishSession(ctx context.Context, operation func(showcase.PublishSession) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := operation(&showcasePublishSession{tx: tx}); err != nil {
		return err
	}
	return tx.Commit()
}

type showcasePublishSession struct{ tx *sql.Tx }

func (s *showcasePublishSession) CurrentVersion(ctx context.Context) (int64, error) {
	var version int64
	err := s.tx.QueryRowContext(ctx, `SELECT version FROM mall_product.homepage_showcase WHERE id = ? FOR UPDATE`, showcase.CurrentID).Scan(&version)
	return version, err
}

func (s *showcasePublishSession) ProductStates(ctx context.Context, productIDs []int64) (map[int64]showcase.ProductState, error) {
	states := make(map[int64]showcase.ProductState, len(productIDs))
	if len(productIDs) == 0 {
		return states, nil
	}
	placeholders := make([]string, 0, len(productIDs))
	args := make([]any, 0, len(productIDs))
	for _, productID := range productIDs {
		placeholders = append(placeholders, "?")
		args = append(args, productID)
	}
	rows, err := s.tx.QueryContext(ctx, fmt.Sprintf(`SELECT product.id, product.status, product.merchant_id,
       IF(merchant.id IS NULL, 0, 1), COALESCE(merchant.status, 0),
       COALESCE(stock.available, product.stock, 0)
FROM mall_product.product product
LEFT JOIN mall_order.merchant merchant ON merchant.id = product.merchant_id
LEFT JOIN mall_product.product_stock_snapshot stock ON stock.product_id = product.id
WHERE product.id IN (%s)`, strings.Join(placeholders, ",")), args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var state showcase.ProductState
		var merchantExists int64
		if err := rows.Scan(&state.ProductID, &state.ProductStatus, &state.MerchantID, &merchantExists, &state.MerchantStatus, &state.StockAvailable); err != nil {
			return nil, err
		}
		state.ProductExists = true
		state.MerchantExists = merchantExists == 1
		states[state.ProductID] = state
	}
	return states, rows.Err()
}

func (s *showcasePublishSession) Replace(ctx context.Context, operatorID int64, items []showcase.PublishItem) error {
	if _, err := s.tx.ExecContext(ctx, `DELETE FROM mall_product.homepage_showcase_item WHERE showcase_id = ?`, showcase.CurrentID); err != nil {
		return err
	}
	if len(items) > 0 {
		values := make([]string, 0, len(items))
		args := make([]any, 0, len(items)*3)
		for _, item := range items {
			values = append(values, "(?, ?, ?)")
			args = append(args, showcase.CurrentID, item.SlotNo, item.ProductID)
		}
		if _, err := s.tx.ExecContext(ctx, `INSERT INTO mall_product.homepage_showcase_item (showcase_id, slot_no, product_id) VALUES `+strings.Join(values, ","), args...); err != nil {
			return err
		}
	}
	_, err := s.tx.ExecContext(ctx, `UPDATE mall_product.homepage_showcase
SET version = version + 1, operator_id = ?, publish_time = NOW()
WHERE id = ?`, operatorID, showcase.CurrentID)
	return err
}

func emptyShowcaseLayout() showcase.Layout {
	layout := showcase.Layout{Items: make([]showcase.Slot, 12)}
	for index := range layout.Items {
		layout.Items[index] = showcase.Slot{SlotNo: int64(index + 1), Empty: true}
	}
	return layout
}
