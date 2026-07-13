package handler

import (
	"context"
	"database/sql"
	"sync"
)

type storefrontSchemaState struct {
	mu    sync.Mutex
	ready bool
}

var storefrontSchemaStates sync.Map

func ensureStorefrontSchema(ctx context.Context, db *sql.DB) error {
	value, _ := storefrontSchemaStates.LoadOrStore(db, &storefrontSchemaState{})
	state := value.(*storefrontSchemaState)
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.ready {
		return nil
	}
	if err := ensureMerchantStoreProfileTable(ctx, db); err != nil {
		return err
	}
	if err := ensureProductCreateTimeColumn(ctx, db); err != nil {
		return err
	}
	if err := ensureHomepageShowcaseTables(ctx, db); err != nil {
		return err
	}
	if err := seedDefaultShowcase(ctx, db); err != nil {
		return err
	}
	state.ready = true
	return nil
}

func ensureMerchantStoreProfileTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS mall_order.merchant_store_profile (
  merchant_id bigint NOT NULL,
  logo_url varchar(512) NOT NULL DEFAULT '',
  banner_url varchar(512) NOT NULL DEFAULT '',
  description varchar(1000) NOT NULL DEFAULT '',
  version bigint NOT NULL DEFAULT 1,
  create_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (merchant_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`)
	return err
}

func ensureProductCreateTimeColumn(ctx context.Context, db *sql.DB) error {
	var exists int64
	if err := db.QueryRowContext(ctx, `
SELECT COUNT(1)
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? AND COLUMN_NAME = ?`,
		"mall_product", "product", "create_time").Scan(&exists); err != nil {
		return err
	}
	if exists > 0 {
		return nil
	}
	if _, err := db.ExecContext(ctx, "ALTER TABLE mall_product.product ADD COLUMN create_time datetime NULL"); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, "UPDATE mall_product.product SET create_time = NOW() - INTERVAL 31 DAY WHERE create_time IS NULL"); err != nil {
		return err
	}
	_, err := db.ExecContext(ctx, "ALTER TABLE mall_product.product MODIFY COLUMN create_time datetime NOT NULL DEFAULT CURRENT_TIMESTAMP")
	return err
}

func ensureHomepageShowcaseTables(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS mall_product.homepage_showcase (
  id bigint NOT NULL,
  version bigint NOT NULL DEFAULT 1,
  operator_id bigint NOT NULL DEFAULT 0,
  publish_time timestamp NULL DEFAULT NULL,
  update_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`); err != nil {
		return err
	}
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS mall_product.homepage_showcase_item (
  showcase_id bigint NOT NULL,
  slot_no tinyint NOT NULL,
  product_id bigint NOT NULL,
  create_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (showcase_id, slot_no),
  UNIQUE KEY uk_showcase_product (showcase_id, product_id),
  KEY ix_showcase_product (product_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`)
	return err
}

func seedDefaultShowcase(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `INSERT INTO mall_product.homepage_showcase (id, version, operator_id)
VALUES (1, 1, 0)
ON DUPLICATE KEY UPDATE id = VALUES(id)`); err != nil {
		return err
	}
	var count int64
	if err := db.QueryRowContext(ctx, `SELECT COUNT(1) FROM mall_product.homepage_showcase_item WHERE showcase_id = ?`, int64(1)).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	_, err := db.ExecContext(ctx, `INSERT INTO mall_product.homepage_showcase_item (showcase_id, slot_no, product_id) VALUES
(1, 1, 100), (1, 2, 101), (1, 3, 102), (1, 4, 103), (1, 5, 104)`)
	return err
}
