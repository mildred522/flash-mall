package handler

import (
	"context"
	"database/sql"
	"fmt"

	"flash-mall/app/common/apperror"
)

func requireSchemaTable(ctx context.Context, db *sql.DB, schema string, table string) error {
	var count int64
	if err := db.QueryRowContext(ctx, `SELECT COUNT(1)
FROM information_schema.TABLES
WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?`, schema, table).Scan(&count); err != nil {
		return err
	}
	if count != 1 {
		return apperror.New(apperror.CodeInternal, fmt.Sprintf("required table %s.%s is missing; run scripts/k8s/init-db.sql", schema, table))
	}
	return nil
}

func requireSchemaColumn(ctx context.Context, db *sql.DB, schema string, table string, column string) error {
	var count int64
	if err := db.QueryRowContext(ctx, `SELECT COUNT(1)
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? AND COLUMN_NAME = ?`, schema, table, column).Scan(&count); err != nil {
		return err
	}
	if count != 1 {
		return apperror.New(apperror.CodeInternal, fmt.Sprintf("required column %s.%s.%s is missing; run scripts/k8s/init-db.sql", schema, table, column))
	}
	return nil
}
