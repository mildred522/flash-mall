package productmysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"flash-mall/app/gateway/hertz/internal/application/supplier"
)

func (r *SupplierRepository) Insert(ctx context.Context, record supplier.Record) (int64, error) {
	result, err := r.db.ExecContext(ctx,
		"INSERT INTO mall_product.supplier (name, status) VALUES (?, ?)", record.Name, record.Status)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (r *SupplierRepository) Update(ctx context.Context, supplierID int64, changes supplier.Changes) (supplier.UpdateResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return supplier.UpdateResult{}, err
	}
	rollback := func(result supplier.UpdateResult, cause error) (supplier.UpdateResult, error) {
		_ = tx.Rollback()
		return result, cause
	}

	var lockedID int64
	err = tx.QueryRowContext(ctx,
		"SELECT id FROM mall_product.supplier WHERE id = ? FOR UPDATE", supplierID,
	).Scan(&lockedID)
	if errors.Is(err, sql.ErrNoRows) {
		return rollback(supplier.UpdateResult{}, nil)
	}
	if err != nil {
		return rollback(supplier.UpdateResult{}, err)
	}

	if changes.Status != nil && *changes.Status == supplier.StatusInactive {
		var productID int64
		err = tx.QueryRowContext(ctx, `SELECT id FROM mall_product.product
WHERE supplier_id = ? AND status = 1 LIMIT 1 FOR UPDATE`, supplierID).Scan(&productID)
		if err == nil {
			return rollback(supplier.UpdateResult{Found: true, HasActiveProducts: true}, nil)
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return rollback(supplier.UpdateResult{}, err)
		}
	}

	setClauses := make([]string, 0, 2)
	args := make([]any, 0, 3)
	if changes.Name != nil {
		setClauses = append(setClauses, "name = ?")
		args = append(args, *changes.Name)
	}
	if changes.Status != nil {
		setClauses = append(setClauses, "status = ?")
		args = append(args, *changes.Status)
	}
	if len(setClauses) == 0 {
		return rollback(supplier.UpdateResult{}, errors.New("supplier update has no changes"))
	}
	args = append(args, supplierID)
	if _, err = tx.ExecContext(ctx, fmt.Sprintf(
		"UPDATE mall_product.supplier SET %s WHERE id = ?", strings.Join(setClauses, ", "),
	), args...); err != nil {
		return rollback(supplier.UpdateResult{}, err)
	}
	if err = tx.Commit(); err != nil {
		return supplier.UpdateResult{}, err
	}
	return supplier.UpdateResult{Found: true}, nil
}
