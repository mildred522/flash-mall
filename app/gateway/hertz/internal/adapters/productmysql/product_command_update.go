package productmysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"flash-mall/app/gateway/hertz/internal/application/productcommand"
)

func (r *ProductCommandRepository) Update(ctx context.Context, command productcommand.UpdateCommand) (productcommand.UpdateResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return productcommand.UpdateResult{}, err
	}
	rollback := func(result productcommand.UpdateResult, cause error) (productcommand.UpdateResult, error) {
		_ = tx.Rollback()
		return result, cause
	}
	where := "id = ?"
	lookupArgs := []any{command.ProductID}
	if command.MerchantID > 0 {
		where += " AND merchant_id = ?"
		lookupArgs = append(lookupArgs, command.MerchantID)
	}
	var originPrice, salePrice int64
	err = tx.QueryRowContext(ctx,
		"SELECT origin_price_fen, sale_price_fen FROM mall_product.product WHERE "+where+" FOR UPDATE", lookupArgs...,
	).Scan(&originPrice, &salePrice)
	if errors.Is(err, sql.ErrNoRows) {
		return rollback(productcommand.UpdateResult{}, nil)
	}
	if err != nil {
		return rollback(productcommand.UpdateResult{}, err)
	}
	if command.SupplierID != nil {
		var supplierID int64
		err = tx.QueryRowContext(ctx,
			"SELECT id FROM mall_product.supplier WHERE id = ? AND status = 1 FOR UPDATE", *command.SupplierID,
		).Scan(&supplierID)
		if errors.Is(err, sql.ErrNoRows) {
			return rollback(productcommand.UpdateResult{Found: true, Rejection: productcommand.RejectSupplierNotFound}, nil)
		}
		if err != nil {
			return rollback(productcommand.UpdateResult{}, err)
		}
	}
	if command.OriginPriceFen != nil {
		originPrice = *command.OriginPriceFen
	}
	if command.SalePriceFen != nil {
		salePrice = *command.SalePriceFen
	}
	if salePrice > originPrice {
		return rollback(productcommand.UpdateResult{Found: true, Rejection: productcommand.RejectInvalidPrice}, nil)
	}
	clauses, args := productUpdateClauses(command)
	args = append(args, command.ProductID)
	if command.MerchantID > 0 {
		args = append(args, command.MerchantID)
	}
	if _, err = tx.ExecContext(ctx, fmt.Sprintf(
		"UPDATE mall_product.product SET %s WHERE %s", strings.Join(clauses, ", "), where,
	), args...); err != nil {
		return rollback(productcommand.UpdateResult{}, err)
	}
	if err = tx.Commit(); err != nil {
		return productcommand.UpdateResult{}, err
	}
	return productcommand.UpdateResult{Found: true}, nil
}

func productUpdateClauses(command productcommand.UpdateCommand) ([]string, []any) {
	clauses := make([]string, 0, 6)
	args := make([]any, 0, 6)
	appendValue := func(column string, value any) {
		clauses = append(clauses, column+" = ?")
		args = append(args, value)
	}
	if command.Name != "" {
		appendValue("name", command.Name)
	}
	if command.ImageURL != "" {
		appendValue("image_url", command.ImageURL)
	}
	if command.OriginPriceFen != nil {
		appendValue("origin_price_fen", *command.OriginPriceFen)
	}
	if command.SalePriceFen != nil {
		appendValue("sale_price_fen", *command.SalePriceFen)
	}
	if command.SupplierID != nil {
		appendValue("supplier_id", *command.SupplierID)
	}
	if command.Status != nil {
		appendValue("status", *command.Status)
	}
	return clauses, args
}
