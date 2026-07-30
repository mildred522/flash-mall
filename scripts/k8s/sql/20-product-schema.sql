USE mall_product;

CREATE TABLE IF NOT EXISTS product (
  id bigint NOT NULL,
  name varchar(128) NOT NULL DEFAULT '',
  stock int NOT NULL DEFAULT 0,
  version bigint NOT NULL DEFAULT 0,
  create_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

SET @sql = IF(
  EXISTS(
    SELECT 1
    FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'product'
      AND COLUMN_NAME = 'name'
  ),
  'SELECT 1',
  'ALTER TABLE product ADD COLUMN name varchar(128) NOT NULL DEFAULT '''' AFTER id'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @sql = IF(
  EXISTS(
    SELECT 1
    FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'product'
      AND COLUMN_NAME = 'image_url'
  ),
  'SELECT 1',
  'ALTER TABLE product ADD COLUMN image_url varchar(512) NOT NULL DEFAULT ""'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @sql = IF(
  EXISTS(
    SELECT 1
    FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'product'
      AND COLUMN_NAME = 'merchant_id'
  ),
  'SELECT 1',
  'ALTER TABLE product ADD COLUMN merchant_id bigint NOT NULL DEFAULT 1000 AFTER id'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @sql = IF(
  EXISTS(
    SELECT 1
    FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'product'
      AND COLUMN_NAME = 'origin_price_fen'
  ),
  'SELECT 1',
  'ALTER TABLE product ADD COLUMN origin_price_fen bigint NOT NULL DEFAULT 0'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @sql = IF(
  EXISTS(
    SELECT 1
    FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'product'
      AND COLUMN_NAME = 'sale_price_fen'
  ),
  'SELECT 1',
  'ALTER TABLE product ADD COLUMN sale_price_fen bigint NOT NULL DEFAULT 0'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @sql = IF(
  EXISTS(
    SELECT 1
    FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'product'
      AND COLUMN_NAME = 'status'
  ),
  'SELECT 1',
  'ALTER TABLE product ADD COLUMN status tinyint NOT NULL DEFAULT 1'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @has_idx = (SELECT COUNT(1) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'product' AND INDEX_NAME = 'ix_merchant_status');
SET @sql = IF(@has_idx = 0, 'ALTER TABLE product ADD KEY ix_merchant_status (merchant_id, status)', 'SELECT 1');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @sql = IF(
  EXISTS(
    SELECT 1
    FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'product'
      AND COLUMN_NAME = 'supplier_id'
  ),
  'SELECT 1',
  'ALTER TABLE product ADD COLUMN supplier_id bigint NOT NULL DEFAULT 0'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @has_col = (
  SELECT COUNT(1)
  FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'product'
    AND COLUMN_NAME = 'create_time'
);
SET @sql = IF(
  @has_col = 0,
  'ALTER TABLE product ADD COLUMN create_time datetime NULL',
  'SELECT 1'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

UPDATE product
SET create_time = DATE_SUB(NOW(), INTERVAL 31 DAY)
WHERE create_time IS NULL;

ALTER TABLE product
  MODIFY COLUMN create_time datetime NOT NULL DEFAULT CURRENT_TIMESTAMP;

SET @has_idx = (
  SELECT COUNT(1)
  FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'product'
    AND INDEX_NAME = 'ix_product_create_time'
);
SET @sql = IF(
  @has_idx = 0,
  'ALTER TABLE product ADD KEY ix_product_create_time (create_time)',
  'SELECT 1'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

-- 库存分桶用于分散热点商品的并发写入。
CREATE TABLE IF NOT EXISTS product_stock_bucket (
  product_id bigint NOT NULL,
  bucket_idx int NOT NULL,
  stock int NOT NULL DEFAULT 0,
  version bigint NOT NULL DEFAULT 0,
  PRIMARY KEY (product_id, bucket_idx)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS product_stock_snapshot (
  product_id bigint NOT NULL,
  available bigint NOT NULL DEFAULT 0,
  reserved bigint NOT NULL DEFAULT 0,
  total bigint NOT NULL DEFAULT 0,
  source varchar(32) NOT NULL DEFAULT 'inventory-kitex',
  version bigint NOT NULL DEFAULT 0,
  update_time timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (product_id),
  KEY ix_update_time (update_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS inventory_reservation (
  order_id varchar(64) NOT NULL,
  product_id bigint NOT NULL,
  quantity bigint NOT NULL,
  shard_index int NOT NULL,
  status varchar(16) NOT NULL,
  expires_at datetime(6) NOT NULL,
  version bigint NOT NULL DEFAULT 0,
  request_id varchar(64) NOT NULL DEFAULT '',
  trace_id varchar(64) NOT NULL DEFAULT '',
  create_time datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  update_time datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (order_id),
  KEY ix_status_expires (status, expires_at),
  KEY ix_product_status (product_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS product_card_snapshot (
  product_id bigint NOT NULL,
  name varchar(255) NOT NULL DEFAULT '',
  origin_price_fen bigint NOT NULL DEFAULT 0,
  final_price_fen bigint NOT NULL DEFAULT 0,
  promotion_type varchar(32) NOT NULL DEFAULT '',
  promotion_tag varchar(32) NOT NULL DEFAULT '',
  stock_available bigint NOT NULL DEFAULT 0,
  supplier_id bigint NOT NULL DEFAULT 0,
  status tinyint NOT NULL DEFAULT 1,
  version bigint NOT NULL DEFAULT 0,
  update_time timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (product_id),
  KEY ix_status_product (status, product_id),
  KEY ix_update_time (update_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS product_inventory_seed (
  product_id bigint NOT NULL,
  desired_total bigint NOT NULL DEFAULT 0,
  shard_count int NOT NULL DEFAULT 4,
  desired_product_status tinyint NOT NULL DEFAULT 1,
  status tinyint NOT NULL DEFAULT 0 COMMENT '0-pending 1-succeeded 2-failed',
  attempt_count int NOT NULL DEFAULT 0,
  last_error varchar(512) NOT NULL DEFAULT '',
  next_retry_time datetime NULL DEFAULT NULL,
  seeded_at datetime NULL DEFAULT NULL,
  create_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (product_id),
  KEY ix_seed_retry (status, next_retry_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS homepage_showcase (
  id bigint NOT NULL,
  version bigint NOT NULL DEFAULT 1,
  operator_id bigint NOT NULL DEFAULT 0,
  publish_time timestamp NULL DEFAULT NULL,
  update_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS homepage_showcase_item (
  showcase_id bigint NOT NULL,
  slot_no tinyint NOT NULL,
  product_id bigint NOT NULL,
  create_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (showcase_id, slot_no),
  UNIQUE KEY uk_showcase_product (showcase_id, product_id),
  KEY ix_showcase_product (product_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS inventory_stock_change_log (
  id bigint NOT NULL AUTO_INCREMENT,
  product_id bigint NOT NULL,
  order_id varchar(64) NOT NULL DEFAULT '',
  change_type varchar(32) NOT NULL,
  delta bigint NOT NULL DEFAULT 0,
  bucket_idx int NOT NULL DEFAULT 0,
  before_available bigint NOT NULL DEFAULT 0,
  before_reserved bigint NOT NULL DEFAULT 0,
  before_total bigint NOT NULL DEFAULT 0,
  after_available bigint NOT NULL DEFAULT 0,
  after_reserved bigint NOT NULL DEFAULT 0,
  after_total bigint NOT NULL DEFAULT 0,
  reason varchar(255) NOT NULL DEFAULT '',
  request_id varchar(64) NOT NULL DEFAULT '',
  trace_id varchar(64) NOT NULL DEFAULT '',
  operator_user_id bigint NOT NULL DEFAULT 0,
  operator_merchant_id bigint NOT NULL DEFAULT 0,
  operator_role varchar(32) NOT NULL DEFAULT '',
  create_time timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY ix_product_time (product_id, create_time),
  KEY ix_order_id (order_id),
  KEY ix_request_id (request_id),
  KEY ix_operator (operator_user_id, operator_merchant_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS seckill_campaign (
  id bigint NOT NULL AUTO_INCREMENT,
  product_id bigint NOT NULL,
  name varchar(128) NOT NULL DEFAULT '',
  campaign_stock int NOT NULL DEFAULT 0,
  per_user_limit int NOT NULL DEFAULT 1,
  starts_at timestamp NULL DEFAULT NULL,
  ends_at timestamp NULL DEFAULT NULL,
  status tinyint NOT NULL DEFAULT 0 COMMENT '0-draft 1-active 2-paused 3-ended',
  create_time timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  update_time timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY ix_product_status (product_id, status),
  KEY ix_time_window (starts_at, ends_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS seckill_user_limit (
  id bigint NOT NULL AUTO_INCREMENT,
  campaign_id bigint NOT NULL,
  user_id bigint NOT NULL,
  ordered_amount int NOT NULL DEFAULT 0,
  update_time timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uniq_campaign_user (campaign_id, user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS promotion_rule (
  id bigint NOT NULL AUTO_INCREMENT,
  product_id bigint NOT NULL,
  type varchar(32) NOT NULL,
  discount_value bigint NOT NULL DEFAULT 0,
  threshold_amount bigint NOT NULL DEFAULT 0,
  starts_at timestamp NULL DEFAULT NULL,
  ends_at timestamp NULL DEFAULT NULL,
  status tinyint NOT NULL DEFAULT 1,
  PRIMARY KEY (id),
  KEY ix_product_status_time (product_id, status, starts_at, ends_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS supplier (
  id bigint NOT NULL AUTO_INCREMENT,
  name varchar(128) NOT NULL,
  status tinyint NOT NULL DEFAULT 1,
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS stock_log (
  id bigint NOT NULL AUTO_INCREMENT,
  order_id varchar(64) NOT NULL,
  type varchar(32) NOT NULL,
  create_time timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uniq_order_type (order_id, type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- DTM barrier 表（商品库）
CREATE TABLE IF NOT EXISTS barrier (
  id bigint NOT NULL AUTO_INCREMENT,
  trans_type varchar(45) NOT NULL,
  gid varchar(128) NOT NULL,
  branch_id varchar(128) NOT NULL,
  op varchar(45) NOT NULL,
  barrier_id varchar(45) NOT NULL,
  reason varchar(45) DEFAULT '',
  create_time datetime DEFAULT CURRENT_TIMESTAMP,
  update_time datetime DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uniq_barrier (gid, branch_id, op, barrier_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
