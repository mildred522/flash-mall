package productmysql

import (
	"context"
	"fmt"
	"strings"

	"flash-mall/app/common/apperror"
	"flash-mall/app/gateway/hertz/internal/application/catalogquery"
)

func (r *CatalogRepository) ProductIDs(ctx context.Context, query catalogquery.ListQuery) (catalogquery.IDPage, error) {
	where, args, err := r.productWhere(ctx, query)
	if err != nil {
		return catalogquery.IDPage{}, err
	}
	return r.queryProductIDs(ctx, where, args, query.Page, query.PageSize, "p.id DESC")
}

func (r *CatalogRepository) ProductIDBatch(ctx context.Context, afterID int64, limit int) ([]int64, error) {
	if limit <= 0 {
		limit = 1000
	}
	if limit > 10_000 {
		limit = 10_000
	}
	rows, err := r.db.QueryContext(ctx,
		"SELECT id FROM mall_product.product WHERE id > ? ORDER BY id LIMIT ?",
		afterID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	ids := make([]int64, 0, limit)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *CatalogRepository) OwnsProduct(ctx context.Context, merchantID, productID int64) (bool, error) {
	var count int64
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM mall_product.product WHERE id = ? AND merchant_id = ?",
		productID, merchantID,
	).Scan(&count)
	return count > 0, err
}

func (r *CatalogRepository) productWhere(ctx context.Context, query catalogquery.ListQuery) (string, []any, error) {
	where := "1=1"
	args := make([]any, 0, 6)
	filters := []struct {
		value  int64
		clause string
	}{{query.Status, "p.status = ?"}, {query.ProductID, "p.id = ?"}, {query.MerchantID, "p.merchant_id = ?"}, {query.SupplierID, "p.supplier_id = ?"}}
	for index, filter := range filters {
		if (index == 0 && filter.value >= 0) || (index > 0 && filter.value > 0) {
			where += " AND " + filter.clause
			args = append(args, filter.value)
		}
	}
	if query.Keyword != "" {
		where += " AND p.name LIKE ?"
		args = append(args, "%"+query.Keyword+"%")
	}
	if query.CategoryID > 0 {
		exists, err := r.productColumnExists(ctx, "category_id")
		if err != nil {
			return "", nil, err
		}
		if !exists {
			return "", nil, apperror.New(apperror.CodeInvalidArgument, "category filter is not available")
		}
		where += " AND p.category_id = ?"
		args = append(args, query.CategoryID)
	}
	return where, args, nil
}

func (r *CatalogRepository) StoreProductIDs(ctx context.Context, merchantID int64, keyword string, page, pageSize int64) (catalogquery.IDPage, error) {
	where := "p.merchant_id = ? AND p.status = ?"
	args := []any{merchantID, int64(1)}
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		where += " AND p.name LIKE ?"
		args = append(args, "%"+keyword+"%")
	}
	return r.queryProductIDs(ctx, where, args, page, pageSize, "p.create_time DESC, p.id DESC")
}

func (r *CatalogRepository) queryProductIDs(ctx context.Context, where string, args []any, page, pageSize int64, orderBy string) (catalogquery.IDPage, error) {
	var result catalogquery.IDPage
	if err := r.db.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM mall_product.product p WHERE %s", where), args...).Scan(&result.Total); err != nil {
		return result, err
	}
	if result.Total == 0 {
		result.ProductIDs = []int64{}
		return result, nil
	}
	queryArgs := append(append([]any{}, args...), pageSize, (page-1)*pageSize)
	rows, err := r.db.QueryContext(ctx, fmt.Sprintf("SELECT p.id FROM mall_product.product p WHERE %s ORDER BY %s LIMIT ? OFFSET ?", where, orderBy), queryArgs...)
	if err != nil {
		return result, err
	}
	defer func() { _ = rows.Close() }()
	result.ProductIDs = make([]int64, 0, pageSize)
	for rows.Next() {
		var productID int64
		if err := rows.Scan(&productID); err != nil {
			return result, err
		}
		result.ProductIDs = append(result.ProductIDs, productID)
	}
	return result, rows.Err()
}

func (r *CatalogRepository) productColumnExists(ctx context.Context, column string) (bool, error) {
	var count int64
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = 'mall_product' AND TABLE_NAME = 'product' AND COLUMN_NAME = ?`, column).Scan(&count)
	return count > 0, err
}
