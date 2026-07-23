package productmysql

import (
	"context"
	"database/sql"
	"errors"

	"flash-mall/app/gateway/hertz/internal/application/productcommand"
)

type ProductCommandRepository struct{ db *sql.DB }

var _ productcommand.Repository = (*ProductCommandRepository)(nil)

func NewProductCommandRepository(db *sql.DB) *ProductCommandRepository {
	return &ProductCommandRepository{db: db}
}

func (r *ProductCommandRepository) Create(ctx context.Context, record productcommand.CreateRecord) (productcommand.CreateResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return productcommand.CreateResult{}, err
	}
	rollback := func(result productcommand.CreateResult, cause error) (productcommand.CreateResult, error) {
		_ = tx.Rollback()
		return result, cause
	}
	var lockedID int64
	if err = tx.QueryRowContext(ctx,
		"SELECT id FROM mall_product.supplier WHERE id = ? AND status = 1 FOR UPDATE", record.SupplierID,
	).Scan(&lockedID); errors.Is(err, sql.ErrNoRows) {
		return rollback(productcommand.CreateResult{Rejection: productcommand.RejectSupplierNotFound}, nil)
	} else if err != nil {
		return rollback(productcommand.CreateResult{}, err)
	}
	if err = tx.QueryRowContext(ctx,
		"SELECT id FROM mall_order.merchant WHERE id = ? AND status = 1 FOR UPDATE", record.MerchantID,
	).Scan(&lockedID); errors.Is(err, sql.ErrNoRows) {
		return rollback(productcommand.CreateResult{Rejection: productcommand.RejectMerchantNotFound}, nil)
	} else if err != nil {
		return rollback(productcommand.CreateResult{}, err)
	}
	productID := int64(100)
	err = tx.QueryRowContext(ctx,
		"SELECT id FROM mall_product.product ORDER BY id DESC LIMIT 1 FOR UPDATE",
	).Scan(&productID)
	if err == nil {
		productID++
	} else if !errors.Is(err, sql.ErrNoRows) {
		return rollback(productcommand.CreateResult{}, err)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO mall_product.product
(id, merchant_id, name, image_url, stock, version, origin_price_fen, sale_price_fen, status, supplier_id)
VALUES (?, ?, ?, ?, ?, 0, ?, ?, 2, ?)`, productID, record.MerchantID, record.Name, record.ImageURL,
		record.StockAvailable, record.OriginPriceFen, record.SalePriceFen, record.SupplierID)
	if err != nil {
		return rollback(productcommand.CreateResult{}, err)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO mall_product.product_inventory_seed
(product_id, desired_total, shard_count, desired_product_status, status) VALUES (?, ?, 4, ?, 0)`,
		productID, record.StockAvailable, record.Status)
	if err != nil {
		return rollback(productcommand.CreateResult{}, err)
	}
	if err = tx.Commit(); err != nil {
		return productcommand.CreateResult{}, err
	}
	return productcommand.CreateResult{ProductID: productID}, nil
}
