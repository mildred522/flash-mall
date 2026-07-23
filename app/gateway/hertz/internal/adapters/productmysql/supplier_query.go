package productmysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"flash-mall/app/gateway/hertz/internal/application/supplier"
)

type SupplierRepository struct{ db *sql.DB }

var _ supplier.Repository = (*SupplierRepository)(nil)

func NewSupplierRepository(db *sql.DB) *SupplierRepository { return &SupplierRepository{db: db} }

func (r *SupplierRepository) List(ctx context.Context, query supplier.ListQuery) ([]supplier.Record, int64, error) {
	where, args := supplierWhereClause(query)
	var total int64
	if err := r.db.QueryRowContext(ctx, fmt.Sprintf(
		"SELECT COUNT(*) FROM mall_product.supplier s WHERE %s", where,
	), args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []supplier.Record{}, 0, nil
	}
	statement := fmt.Sprintf(`SELECT s.id, s.name, s.status,
COALESCE(stats.product_count, 0), COALESCE(stats.active_products, 0)
FROM mall_product.supplier s
LEFT JOIN (
  SELECT supplier_id, COUNT(*) AS product_count,
         SUM(CASE WHEN status = 1 THEN 1 ELSE 0 END) AS active_products
  FROM mall_product.product GROUP BY supplier_id
) stats ON stats.supplier_id = s.id
WHERE %s ORDER BY s.id DESC LIMIT ? OFFSET ?`, where)
	queryArgs := append(append([]any{}, args...), query.PageSize, (query.Page-1)*query.PageSize)
	rows, err := r.db.QueryContext(ctx, statement, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]supplier.Record, 0, query.PageSize)
	for rows.Next() {
		item, err := scanSupplier(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (r *SupplierRepository) Find(ctx context.Context, supplierID int64) (supplier.Record, bool, error) {
	record, err := scanSupplier(r.db.QueryRowContext(ctx, `SELECT s.id, s.name, s.status,
COALESCE(stats.product_count, 0), COALESCE(stats.active_products, 0)
FROM mall_product.supplier s
LEFT JOIN (
  SELECT supplier_id, COUNT(*) AS product_count,
         SUM(CASE WHEN status = 1 THEN 1 ELSE 0 END) AS active_products
  FROM mall_product.product GROUP BY supplier_id
) stats ON stats.supplier_id = s.id
WHERE s.id = ?`, supplierID))
	if errors.Is(err, sql.ErrNoRows) {
		return supplier.Record{}, false, nil
	}
	return record, err == nil, err
}

type supplierScanner interface{ Scan(...any) error }

func scanSupplier(scanner supplierScanner) (supplier.Record, error) {
	var record supplier.Record
	err := scanner.Scan(&record.SupplierID, &record.Name, &record.Status, &record.ProductCount, &record.ActiveProducts)
	return record, err
}

func supplierWhereClause(query supplier.ListQuery) (string, []any) {
	where := "1=1"
	args := make([]any, 0, 2)
	if query.Status >= 0 {
		where += " AND s.status = ?"
		args = append(args, query.Status)
	}
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		where += " AND s.name LIKE ?"
		args = append(args, "%"+keyword+"%")
	}
	return where, args
}
