-- 初始化数据库与表结构（K8s MySQL）

SET NAMES utf8mb4;
SET CHARACTER SET utf8mb4;

CREATE DATABASE IF NOT EXISTS mall_order DEFAULT CHARSET utf8mb4;
CREATE DATABASE IF NOT EXISTS mall_product DEFAULT CHARSET utf8mb4;
CREATE DATABASE IF NOT EXISTS mall_auth DEFAULT CHARSET utf8mb4;
CREATE DATABASE IF NOT EXISTS dtm DEFAULT CHARSET utf8mb4;

USE mall_order;

CREATE TABLE IF NOT EXISTS orders (
  id varchar(64) NOT NULL COMMENT '订单id',
  request_id varchar(64) DEFAULT NULL COMMENT '幂等请求id',
  user_id bigint NOT NULL DEFAULT 0 COMMENT '用户id',
  product_id bigint NOT NULL DEFAULT 0 COMMENT '商品id',
  amount int NOT NULL DEFAULT 0 COMMENT '数量',
  status tinyint NOT NULL DEFAULT 0 COMMENT '订单状态 0-待支付 1-已支付 2-已关闭 3-已发货 4-已收货 5-申请退款 6-已退款',
  create_time timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  update_time timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uniq_request_id (request_id),
  KEY ix_user_id (user_id),
  KEY ix_create_time (create_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 订单生命周期：新增时间戳列（幂等迁移）
SET @has_col = (SELECT COUNT(1) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = 'mall_order' AND TABLE_NAME = 'orders' AND COLUMN_NAME = 'shipped_at');
SET @sql = IF(@has_col = 0, 'ALTER TABLE orders ADD COLUMN shipped_at timestamp NULL DEFAULT NULL AFTER status', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_col = (SELECT COUNT(1) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = 'mall_order' AND TABLE_NAME = 'orders' AND COLUMN_NAME = 'completed_at');
SET @sql = IF(@has_col = 0, 'ALTER TABLE orders ADD COLUMN completed_at timestamp NULL DEFAULT NULL AFTER shipped_at', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_col = (SELECT COUNT(1) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = 'mall_order' AND TABLE_NAME = 'orders' AND COLUMN_NAME = 'refund_requested_at');
SET @sql = IF(@has_col = 0, 'ALTER TABLE orders ADD COLUMN refund_requested_at timestamp NULL DEFAULT NULL AFTER completed_at', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_col = (SELECT COUNT(1) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = 'mall_order' AND TABLE_NAME = 'orders' AND COLUMN_NAME = 'refunded_at');
SET @sql = IF(@has_col = 0, 'ALTER TABLE orders ADD COLUMN refunded_at timestamp NULL DEFAULT NULL AFTER refund_requested_at', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 订单状态变更日志
CREATE TABLE IF NOT EXISTS order_status_log (
  id bigint NOT NULL AUTO_INCREMENT,
  order_id varchar(64) NOT NULL COMMENT '订单id',
  from_status tinyint NOT NULL COMMENT '原状态',
  to_status tinyint NOT NULL COMMENT '新状态',
  operator_id bigint NOT NULL DEFAULT 0 COMMENT '操作人id',
  remark varchar(255) NOT NULL DEFAULT '' COMMENT '备注',
  create_time timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY ix_order_id (order_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS order_price_snapshot (
  order_id varchar(64) NOT NULL COMMENT '订单id',
  product_id bigint NOT NULL DEFAULT 0 COMMENT '商品id',
  supplier_id bigint NOT NULL DEFAULT 0 COMMENT '供应商id',
  product_name varchar(128) NOT NULL DEFAULT '' COMMENT '商品名快照',
  amount int NOT NULL DEFAULT 0 COMMENT '购买数量',
  origin_unit_price_fen bigint NOT NULL DEFAULT 0 COMMENT '原价单价分',
  sale_unit_price_fen bigint NOT NULL DEFAULT 0 COMMENT '成交单价分',
  payable_amount_fen bigint NOT NULL DEFAULT 0 COMMENT '应付金额分',
  discount_amount_fen bigint NOT NULL DEFAULT 0 COMMENT '优惠金额分',
  promotion_type varchar(32) NOT NULL DEFAULT '' COMMENT '促销类型',
  promotion_tag varchar(64) NOT NULL DEFAULT '' COMMENT '促销标签',
  create_time timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  update_time timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (order_id),
  KEY ix_product_id (product_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS payment_order (
  id varchar(64) NOT NULL COMMENT '支付单id',
  order_id varchar(64) NOT NULL COMMENT '订单id',
  user_id bigint NOT NULL DEFAULT 0 COMMENT '用户id',
  payable_amount_fen bigint NOT NULL DEFAULT 0 COMMENT '应付金额分',
  status tinyint NOT NULL DEFAULT 0 COMMENT '支付单状态 0-init 1-success 2-failed 3-closed',
  out_trade_no varchar(64) NOT NULL DEFAULT '' COMMENT '外部交易号',
  create_time timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  update_time timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uniq_order_id (order_id),
  UNIQUE KEY uniq_out_trade_no (out_trade_no)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

SET @sql = IF(
  EXISTS(
    SELECT 1
    FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'payment_order'
      AND COLUMN_NAME = 'paid_at'
  ),
  'SELECT 1',
  'ALTER TABLE payment_order ADD COLUMN paid_at timestamp NULL DEFAULT NULL'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @sql = IF(
  EXISTS(
    SELECT 1
    FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'payment_order'
      AND COLUMN_NAME = 'callback_payload'
  ),
  'SELECT 1',
  'ALTER TABLE payment_order ADD COLUMN callback_payload json DEFAULT NULL'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS payment_callback_event (
  id bigint NOT NULL AUTO_INCREMENT,
  provider varchar(32) NOT NULL DEFAULT 'mock' COMMENT '支付渠道',
  event_id varchar(128) NOT NULL DEFAULT '' COMMENT '渠道事件id',
  payment_order_id varchar(64) NOT NULL COMMENT '支付单id',
  order_id varchar(64) NOT NULL COMMENT '订单id',
  out_trade_no varchar(64) NOT NULL COMMENT '外部交易号',
  paid_amount_fen bigint NOT NULL DEFAULT 0 COMMENT '实付金额分',
  signature_valid tinyint NOT NULL DEFAULT 1 COMMENT '签名是否有效',
  process_status varchar(32) NOT NULL DEFAULT 'SUCCESS' COMMENT '处理状态',
  error_message varchar(255) NOT NULL DEFAULT '' COMMENT '错误信息',
  raw_payload json DEFAULT NULL COMMENT '回调原文',
  create_time timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uniq_provider_event (provider, event_id),
  KEY ix_payment_order_id (payment_order_id),
  KEY ix_order_id (order_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS order_outbox (
  id bigint NOT NULL AUTO_INCREMENT,
  event_id varchar(128) NOT NULL,
  event_type varchar(64) NOT NULL,
  aggregate_id varchar(64) NOT NULL,
  payload json NOT NULL,
  status tinyint NOT NULL DEFAULT 0 COMMENT '0-pending 1-published 2-publishing 3-dead',
  attempt_count int NOT NULL DEFAULT 0,
  next_retry_at timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  published_at timestamp NULL DEFAULT NULL,
  last_error varchar(255) NOT NULL DEFAULT '',
  create_time timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  update_time timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uniq_event_id (event_id),
  KEY ix_status_retry (status, next_retry_at),
  KEY ix_aggregate_id (aggregate_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- DTM barrier 表（订单库）
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

USE mall_product;

CREATE TABLE IF NOT EXISTS product (
  id bigint NOT NULL,
  name varchar(128) NOT NULL DEFAULT '',
  stock int NOT NULL DEFAULT 0,
  version bigint NOT NULL DEFAULT 0,
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

-- CHG 2026-02-24: 变更=新增库存分桶表; 之前=单行 product 表扣减; 原因=降低热点行冲突。
CREATE TABLE IF NOT EXISTS product_stock_bucket (
  product_id bigint NOT NULL,
  bucket_idx int NOT NULL,
  stock int NOT NULL DEFAULT 0,
  version bigint NOT NULL DEFAULT 0,
  PRIMARY KEY (product_id, bucket_idx)
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

INSERT INTO supplier (id, name, status)
VALUES (200, 'Flash Supplier', 1)
ON DUPLICATE KEY UPDATE name = VALUES(name), status = VALUES(status);

-- 初始化示例商品
INSERT INTO product (id, name, stock, version, origin_price_fen, sale_price_fen, status, supplier_id)
VALUES (100, '首发风衣', 10000, 0, 12900, 11900, 1, 200)
ON DUPLICATE KEY UPDATE
  name = VALUES(name),
  stock = VALUES(stock),
  origin_price_fen = VALUES(origin_price_fen),
  sale_price_fen = VALUES(sale_price_fen),
  status = VALUES(status),
  supplier_id = VALUES(supplier_id);

-- 初始化分桶库存（默认 4 桶）
INSERT INTO product_stock_bucket (product_id, bucket_idx, stock, version)
VALUES
  (100, 0, 2500, 0),
  (100, 1, 2500, 0),
  (100, 2, 2500, 0),
  (100, 3, 2500, 0)
ON DUPLICATE KEY UPDATE stock = VALUES(stock), version = VALUES(version);

DELETE FROM promotion_rule
WHERE product_id = 100
  AND type = 'LIMITED_PRICE';

INSERT INTO promotion_rule (product_id, type, discount_value, threshold_amount, starts_at, ends_at, status)
VALUES (100, 'LIMITED_PRICE', 9900, 0, DATE_SUB(NOW(), INTERVAL 1 DAY), DATE_ADD(NOW(), INTERVAL 30 DAY), 1);

-- 新增商品 101-104
INSERT INTO product (id, name, stock, version, origin_price_fen, sale_price_fen, status, supplier_id)
VALUES
  (101, '轻薄羽绒服', 10000, 0, 39900, 25900, 1, 200),
  (102, '纯棉T恤三件套', 10000, 0, 15900, 9900, 1, 200),
  (103, '运动休闲鞋', 10000, 0, 49900, 32900, 1, 200),
  (104, '便携充电宝', 10000, 0, 12900, 7900, 1, 200)
ON DUPLICATE KEY UPDATE
  name = VALUES(name), stock = VALUES(stock),
  origin_price_fen = VALUES(origin_price_fen), sale_price_fen = VALUES(sale_price_fen),
  status = VALUES(status), supplier_id = VALUES(supplier_id);

INSERT INTO product_stock_bucket (product_id, bucket_idx, stock, version) VALUES
  (101, 0, 2500, 0), (101, 1, 2500, 0), (101, 2, 2500, 0), (101, 3, 2500, 0),
  (102, 0, 2500, 0), (102, 1, 2500, 0), (102, 2, 2500, 0), (102, 3, 2500, 0),
  (103, 0, 2500, 0), (103, 1, 2500, 0), (103, 2, 2500, 0), (103, 3, 2500, 0),
  (104, 0, 2500, 0), (104, 1, 2500, 0), (104, 2, 2500, 0), (104, 3, 2500, 0)
ON DUPLICATE KEY UPDATE stock = VALUES(stock), version = VALUES(version);

DELETE FROM promotion_rule WHERE product_id IN (101, 102, 103, 104) AND type = 'LIMITED_PRICE';
INSERT INTO promotion_rule (product_id, type, discount_value, threshold_amount, starts_at, ends_at, status) VALUES
  (101, 'LIMITED_PRICE', 25900, 0, DATE_SUB(NOW(), INTERVAL 1 DAY), DATE_ADD(NOW(), INTERVAL 30 DAY), 1),
  (102, 'LIMITED_PRICE', 9900, 0, DATE_SUB(NOW(), INTERVAL 1 DAY), DATE_ADD(NOW(), INTERVAL 30 DAY), 1),
  (103, 'LIMITED_PRICE', 32900, 0, DATE_SUB(NOW(), INTERVAL 1 DAY), DATE_ADD(NOW(), INTERVAL 30 DAY), 1),
  (104, 'LIMITED_PRICE', 7900, 0, DATE_SUB(NOW(), INTERVAL 1 DAY), DATE_ADD(NOW(), INTERVAL 30 DAY), 1);

USE mall_auth;

CREATE TABLE IF NOT EXISTS users (
  id bigint NOT NULL AUTO_INCREMENT COMMENT '用户ID',
  display_name varchar(64) NOT NULL DEFAULT '' COMMENT '展示昵称',
  status tinyint NOT NULL DEFAULT 1 COMMENT '状态 1-正常 2-禁用',
  session_version int NOT NULL DEFAULT 1 COMMENT '会话版本号',
  create_time timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  update_time timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY ix_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS user_identities (
  id bigint NOT NULL AUTO_INCREMENT COMMENT '身份ID',
  user_id bigint NOT NULL COMMENT '用户ID',
  identity_type varchar(32) NOT NULL COMMENT '身份类型',
  identity_value varchar(128) NOT NULL COMMENT '身份值',
  is_verified tinyint(1) NOT NULL DEFAULT 0 COMMENT '是否已验证',
  verified_at timestamp NULL DEFAULT NULL COMMENT '验证时间',
  create_time timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  update_time timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uniq_identity_type_value (identity_type, identity_value),
  KEY ix_user_id (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS user_credentials (
  id bigint NOT NULL AUTO_INCREMENT COMMENT '凭证ID',
  user_id bigint NOT NULL COMMENT '用户ID',
  credential_type varchar(32) NOT NULL COMMENT '凭证类型',
  password_hash varchar(255) NOT NULL COMMENT '密码哈希',
  hash_algo varchar(32) NOT NULL DEFAULT 'bcrypt' COMMENT '哈希算法',
  password_updated_at timestamp NULL DEFAULT CURRENT_TIMESTAMP COMMENT '密码更新时间',
  create_time timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  update_time timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uniq_user_credential_type (user_id, credential_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS user_sessions (
  id varchar(64) NOT NULL COMMENT '会话ID sid',
  user_id bigint NOT NULL COMMENT '用户ID',
  device_type varchar(32) NOT NULL DEFAULT 'web' COMMENT '设备类型',
  session_version int NOT NULL DEFAULT 1 COMMENT '会话版本号',
  refresh_token_hash char(64) NOT NULL COMMENT 'refresh token hash',
  previous_refresh_token_hash char(64) NOT NULL DEFAULT '' COMMENT 'previous refresh token hash',
  refresh_family_secret char(64) NOT NULL DEFAULT '' COMMENT 'refresh family secret',
  refresh_generation bigint NOT NULL DEFAULT 1 COMMENT 'refresh generation',
  status tinyint NOT NULL DEFAULT 1 COMMENT '状态 1-活跃 2-失效 3-登出',
  expires_at timestamp NULL DEFAULT NULL COMMENT 'refresh 过期时间',
  last_seen_at timestamp NULL DEFAULT CURRENT_TIMESTAMP COMMENT '最近活跃时间',
  revoked_at timestamp NULL DEFAULT NULL COMMENT '失效时间',
  create_time timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  update_time timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uniq_refresh_token_hash (refresh_token_hash),
  KEY ix_user_id_status (user_id, status),
  KEY ix_user_id_device_type (user_id, device_type),
  KEY ix_previous_refresh_token_hash (previous_refresh_token_hash),
  KEY ix_refresh_generation (refresh_generation)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS verification_codes (
  id bigint NOT NULL AUTO_INCREMENT COMMENT '验证码ID',
  target varchar(128) NOT NULL COMMENT '接收目标',
  scene varchar(32) NOT NULL COMMENT '场景',
  code_hash char(64) NOT NULL COMMENT '验证码哈希',
  status tinyint NOT NULL DEFAULT 1 COMMENT '状态 1-待使用 2-已使用 3-已过期',
  attempt_count int NOT NULL DEFAULT 0 COMMENT '验证码失败尝试次数',
  expires_at timestamp NULL DEFAULT NULL COMMENT '过期时间',
  consumed_at timestamp NULL DEFAULT NULL COMMENT '消费时间',
  send_count int NOT NULL DEFAULT 1 COMMENT '发送次数',
  create_time timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  update_time timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY ix_target_scene_status (target, scene, status),
  KEY ix_expires_at (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS auth_audit_logs (
  id bigint NOT NULL AUTO_INCREMENT COMMENT '审计日志ID',
  user_id bigint DEFAULT NULL COMMENT '用户ID',
  identity_value varchar(128) NOT NULL DEFAULT '' COMMENT '身份值快照',
  event_type varchar(32) NOT NULL COMMENT '事件类型',
  result varchar(16) NOT NULL COMMENT '结果',
  ip varchar(64) NOT NULL DEFAULT '' COMMENT 'IP地址',
  user_agent varchar(255) NOT NULL DEFAULT '' COMMENT '客户端UA',
  detail_json json DEFAULT NULL COMMENT '扩展详情',
  create_time timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY ix_user_id (user_id),
  KEY ix_event_type_result (event_type, result),
  KEY ix_create_time (create_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- ========== auth migration: add missing columns if tables already exist ==========

SET @has_col = (SELECT COUNT(1) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'user_credentials' AND COLUMN_NAME = 'credential_type');
SET @sql = IF(@has_col = 0, 'ALTER TABLE user_credentials ADD COLUMN credential_type varchar(32) NOT NULL DEFAULT ''password'' AFTER user_id', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_col = (SELECT COUNT(1) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'user_credentials' AND COLUMN_NAME = 'password_hash');
SET @sql = IF(@has_col = 0, 'ALTER TABLE user_credentials ADD COLUMN password_hash varchar(255) NOT NULL DEFAULT '''' AFTER credential_type', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_col = (SELECT COUNT(1) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'user_credentials' AND COLUMN_NAME = 'hash_algo');
SET @sql = IF(@has_col = 0, 'ALTER TABLE user_credentials ADD COLUMN hash_algo varchar(32) NOT NULL DEFAULT ''bcrypt'' AFTER password_hash', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_col = (SELECT COUNT(1) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'user_credentials' AND COLUMN_NAME = 'password_updated_at');
SET @sql = IF(@has_col = 0, 'ALTER TABLE user_credentials ADD COLUMN password_updated_at timestamp NULL DEFAULT CURRENT_TIMESTAMP AFTER hash_algo', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_col = (SELECT COUNT(1) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'user_sessions' AND COLUMN_NAME = 'previous_refresh_token_hash');
SET @sql = IF(@has_col = 0, 'ALTER TABLE user_sessions ADD COLUMN previous_refresh_token_hash char(64) NOT NULL DEFAULT '''' AFTER refresh_token_hash', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_col = (SELECT COUNT(1) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'user_sessions' AND COLUMN_NAME = 'refresh_family_secret');
SET @sql = IF(@has_col = 0, 'ALTER TABLE user_sessions ADD COLUMN refresh_family_secret char(64) NOT NULL DEFAULT '''' AFTER previous_refresh_token_hash', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_col = (SELECT COUNT(1) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'user_sessions' AND COLUMN_NAME = 'refresh_generation');
SET @sql = IF(@has_col = 0, 'ALTER TABLE user_sessions ADD COLUMN refresh_generation bigint NOT NULL DEFAULT 1 AFTER refresh_family_secret', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_col = (SELECT COUNT(1) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'user_sessions' AND COLUMN_NAME = 'last_seen_at');
SET @sql = IF(@has_col = 0, 'ALTER TABLE user_sessions ADD COLUMN last_seen_at timestamp NULL DEFAULT CURRENT_TIMESTAMP AFTER expires_at', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_col = (SELECT COUNT(1) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'user_sessions' AND COLUMN_NAME = 'session_version');
SET @sql = IF(@has_col = 0, 'ALTER TABLE user_sessions ADD COLUMN session_version int NOT NULL DEFAULT 1 AFTER device_type', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_col = (SELECT COUNT(1) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'user_sessions' AND COLUMN_NAME = 'revoked_at');
SET @sql = IF(@has_col = 0, 'ALTER TABLE user_sessions ADD COLUMN revoked_at timestamp NULL DEFAULT NULL AFTER last_seen_at', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- ========== end auth migration ==========
