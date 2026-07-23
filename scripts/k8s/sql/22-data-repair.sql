-- Historical repair is deliberately narrow and idempotent:
-- only replace snapshot names containing at least three literal question marks
-- when the canonical product name is available and not corrupted.
USE mall_order;

CREATE TABLE IF NOT EXISTS schema_migrations (
  version varchar(128) NOT NULL,
  applied_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (version)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS order_snapshot_repair_audit (
  order_id varchar(64) NOT NULL,
  product_id bigint NOT NULL DEFAULT 0,
  old_product_name varchar(128) NOT NULL DEFAULT '',
  old_product_name_hex varchar(256) NOT NULL DEFAULT '',
  repaired_product_name varchar(128) NOT NULL DEFAULT '',
  repaired_product_image_url varchar(512) NOT NULL DEFAULT '',
  repaired_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (order_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

START TRANSACTION;

INSERT IGNORE INTO order_snapshot_repair_audit (
  order_id,
  product_id,
  old_product_name,
  old_product_name_hex,
  repaired_product_name,
  repaired_product_image_url
)
SELECT
  snapshot.order_id,
  snapshot.product_id,
  snapshot.product_name,
  HEX(snapshot.product_name),
  product.name,
  product.image_url
FROM mall_order.order_price_snapshot AS snapshot
JOIN mall_product.product AS product ON product.id = snapshot.product_id
WHERE snapshot.product_name LIKE '%???%'
  AND product.name NOT LIKE '%???%'
  AND product.name <> ''
  AND NOT EXISTS (
    SELECT 1
    FROM mall_order.schema_migrations
    WHERE version = '20260723_order_snapshot_utf8_asset_repair'
  );

UPDATE mall_order.order_price_snapshot AS snapshot
JOIN mall_product.product AS product ON product.id = snapshot.product_id
LEFT JOIN mall_order.schema_migrations AS migration
  ON migration.version = '20260723_order_snapshot_utf8_asset_repair'
SET snapshot.product_name = product.name
WHERE snapshot.product_name LIKE '%???%'
  AND product.name NOT LIKE '%???%'
  AND product.name <> ''
  AND migration.version IS NULL;

UPDATE mall_order.order_price_snapshot AS snapshot
JOIN mall_product.product AS product ON product.id = snapshot.product_id
LEFT JOIN mall_order.schema_migrations AS migration
  ON migration.version = '20260723_order_snapshot_utf8_asset_repair'
SET snapshot.product_image_url = product.image_url
WHERE snapshot.product_image_url = ''
  AND product.image_url <> ''
  AND migration.version IS NULL;

INSERT IGNORE INTO mall_order.schema_migrations (version)
VALUES ('20260723_order_snapshot_utf8_asset_repair');

COMMIT;
