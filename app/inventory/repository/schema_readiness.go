package repository

import (
	"context"
	"strings"

	"flash-mall/app/common/apperror"
)

var inventoryRequiredTables = []string{
	"product",
	"stock_log",
	"product_stock_bucket",
	"product_stock_snapshot",
	"inventory_reservation",
	"inventory_stock_change_log",
}

func (r *RedisMySQLRepository) checkSchema(ctx context.Context) error {
	rows, err := r.db.QueryContext(ctx, `
SELECT TABLE_NAME FROM information_schema.TABLES
WHERE TABLE_SCHEMA = DATABASE()
  AND TABLE_NAME IN (?, ?, ?, ?, ?, ?)`,
		inventoryRequiredTables[0],
		inventoryRequiredTables[1],
		inventoryRequiredTables[2],
		inventoryRequiredTables[3],
		inventoryRequiredTables[4],
		inventoryRequiredTables[5],
	)
	if err != nil {
		return apperror.Wrap(apperror.CodeInternal, "inspect inventory schema failed", err)
	}
	defer rows.Close()

	found := make(map[string]struct{}, len(inventoryRequiredTables))
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return apperror.Wrap(apperror.CodeInternal, "read inventory schema failed", err)
		}
		found[table] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return apperror.Wrap(apperror.CodeInternal, "read inventory schema failed", err)
	}

	missing := make([]string, 0)
	for _, table := range inventoryRequiredTables {
		if _, ok := found[table]; !ok {
			missing = append(missing, table)
		}
	}
	if len(missing) > 0 {
		return apperror.New(apperror.CodeInternal, "inventory schema missing tables: "+strings.Join(missing, ", "))
	}
	return nil
}
