package handler

import (
	"context"
	"database/sql"
)

const defaultMerchantID int64 = 1000

func ensureMerchantBaseSchema(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS merchant (
  id bigint NOT NULL AUTO_INCREMENT,
  name varchar(128) NOT NULL DEFAULT '',
  owner_user_id bigint NOT NULL DEFAULT 0,
  status tinyint NOT NULL DEFAULT 1,
  contact_phone varchar(32) NOT NULL DEFAULT '',
  create_time timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  update_time timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY ix_status (status),
  KEY ix_owner_user_id (owner_user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS merchant_user (
  id bigint NOT NULL AUTO_INCREMENT,
  merchant_id bigint NOT NULL,
  user_id bigint NOT NULL,
  role varchar(32) NOT NULL DEFAULT 'owner',
  status tinyint NOT NULL DEFAULT 1,
  create_time timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  update_time timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uniq_merchant_user (merchant_id, user_id),
  KEY ix_user_status (user_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS merchant_apply (
  id bigint NOT NULL AUTO_INCREMENT,
  user_id bigint NOT NULL,
  merchant_name varchar(128) NOT NULL DEFAULT '',
  contact_phone varchar(32) NOT NULL DEFAULT '',
  status tinyint NOT NULL DEFAULT 0,
  merchant_id bigint NOT NULL DEFAULT 0,
  audit_remark varchar(255) NOT NULL DEFAULT '',
  operator_id bigint NOT NULL DEFAULT 0,
  create_time timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  audit_time timestamp NULL DEFAULT NULL,
  update_time timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY ix_user_status (user_id, status),
  KEY ix_status_time (status, create_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`); err != nil {
		return err
	}
	_, err := db.ExecContext(ctx, "INSERT INTO merchant (id, name, owner_user_id, status, contact_phone) VALUES (?, ?, 0, 1, '') ON DUPLICATE KEY UPDATE name = VALUES(name), status = VALUES(status)", defaultMerchantID, "Flash Mall 自营店")
	return err
}

func ensureProductMerchantSchema(ctx context.Context, db *sql.DB) error {
	if err := ensureMerchantBaseSchema(ctx, db); err != nil {
		return err
	}
	return ensureProductMerchantColumn(ctx, db)
}

func ensureOrderMerchantSchema(ctx context.Context, db *sql.DB) error {
	if err := ensureMerchantBaseSchema(ctx, db); err != nil {
		return err
	}
	return ensureOrderMerchantColumns(ctx, db)
}

func ensureMerchantSchema(ctx context.Context, db *sql.DB) error {
	if err := ensureProductMerchantSchema(ctx, db); err != nil {
		return err
	}
	return ensureOrderMerchantSchema(ctx, db)
}

func ensureProductMerchantColumn(ctx context.Context, db *sql.DB) error {
	var exists int64
	err := db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = 'mall_product'
  AND TABLE_NAME = 'product'
  AND COLUMN_NAME = 'merchant_id'`).Scan(&exists)
	if err != nil {
		return err
	}
	if exists == 0 {
		if _, err := db.ExecContext(ctx, "ALTER TABLE mall_product.product ADD COLUMN merchant_id bigint NOT NULL DEFAULT 1000 AFTER id"); err != nil {
			return err
		}
	}
	return nil
}

func ensureOrderMerchantColumns(ctx context.Context, db *sql.DB) error {
	statements := []struct {
		table string
		after string
	}{
		{table: "orders", after: "user_id"},
		{table: "order_price_snapshot", after: "product_id"},
		{table: "refund_order", after: "user_id"},
	}
	for _, statement := range statements {
		var exists int64
		err := db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = DATABASE()
  AND TABLE_NAME = ?
  AND COLUMN_NAME = 'merchant_id'`, statement.table).Scan(&exists)
		if err != nil {
			return err
		}
		if exists == 0 {
			if _, err := db.ExecContext(ctx, "ALTER TABLE "+statement.table+" ADD COLUMN merchant_id bigint NOT NULL DEFAULT 1000 AFTER "+statement.after); err != nil {
				return err
			}
		}
	}
	return nil
}

func adminActiveMerchantExists(ctx context.Context, q adminQueryRower, merchantID int64) (bool, error) {
	var exists int64
	if err := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM merchant WHERE id = ? AND status = 1", merchantID).Scan(&exists); err != nil {
		return false, err
	}
	return exists > 0, nil
}

func userCanAccessMerchant(ctx context.Context, q adminQueryRower, userID, merchantID int64, role string) (bool, error) {
	if role == "admin" {
		return true, nil
	}
	var exists int64
	if err := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM merchant_user WHERE user_id = ? AND merchant_id = ? AND status = 1", userID, merchantID).Scan(&exists); err != nil {
		return false, err
	}
	return exists > 0, nil
}
