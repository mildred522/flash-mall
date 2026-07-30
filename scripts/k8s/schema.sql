-- 本目录 SQL 模块是数据库初始化源码；修改后运行 node scripts/k8s/build-sql-bundles.mjs。
-- scripts/k8s/schema.sql 仅聚合可重复执行的结构迁移；演示数据单独生成到 demo-seed.sql。
-- 初始化数据库与表结构（K8s MySQL）

SET NAMES utf8mb4;
SET character_set_client = utf8mb4;
SET character_set_connection = utf8mb4;
SET character_set_results = utf8mb4;
SET collation_connection = utf8mb4_general_ci;

CREATE DATABASE IF NOT EXISTS mall_order DEFAULT CHARSET utf8mb4;
CREATE DATABASE IF NOT EXISTS mall_product DEFAULT CHARSET utf8mb4;
CREATE DATABASE IF NOT EXISTS mall_auth DEFAULT CHARSET utf8mb4;
CREATE DATABASE IF NOT EXISTS dtm DEFAULT CHARSET utf8mb4;

ALTER DATABASE mall_order CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;
ALTER DATABASE mall_product CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;
ALTER DATABASE mall_auth CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;
ALTER DATABASE dtm CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;
USE mall_order;

CREATE TABLE IF NOT EXISTS merchant (
  id bigint NOT NULL AUTO_INCREMENT,
  name varchar(128) NOT NULL DEFAULT '',
  owner_user_id bigint NOT NULL DEFAULT 0,
  status tinyint NOT NULL DEFAULT 1 COMMENT '1-active 2-disabled 3-pending',
  contact_phone varchar(32) NOT NULL DEFAULT '',
  create_time timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  update_time timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY ix_status (status),
  KEY ix_owner_user_id (owner_user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS merchant_user (
  id bigint NOT NULL AUTO_INCREMENT,
  merchant_id bigint NOT NULL,
  user_id bigint NOT NULL,
  role varchar(32) NOT NULL DEFAULT 'owner',
  status tinyint NOT NULL DEFAULT 1 COMMENT '1-active 2-disabled',
  create_time timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  update_time timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uniq_merchant_user (merchant_id, user_id),
  KEY ix_user_status (user_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS merchant_apply (
  id bigint NOT NULL AUTO_INCREMENT,
  user_id bigint NOT NULL,
  merchant_name varchar(128) NOT NULL DEFAULT '',
  contact_phone varchar(32) NOT NULL DEFAULT '',
  status tinyint NOT NULL DEFAULT 0 COMMENT '0-pending 1-approved 2-rejected',
  merchant_id bigint NOT NULL DEFAULT 0,
  audit_remark varchar(255) NOT NULL DEFAULT '',
  operator_id bigint NOT NULL DEFAULT 0,
  create_time timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  audit_time timestamp NULL DEFAULT NULL,
  update_time timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY ix_user_status (user_id, status),
  KEY ix_status_time (status, create_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS merchant_store_profile (
  merchant_id bigint NOT NULL,
  logo_url varchar(512) NOT NULL DEFAULT '',
  banner_url varchar(512) NOT NULL DEFAULT '',
  description varchar(1000) NOT NULL DEFAULT '',
  version bigint NOT NULL DEFAULT 1,
  create_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (merchant_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS orders (
  id varchar(64) NOT NULL COMMENT '订单id',
  request_id varchar(64) DEFAULT NULL COMMENT '幂等请求id',
  user_id bigint NOT NULL DEFAULT 0 COMMENT '用户id',
  merchant_id bigint NOT NULL DEFAULT 1000 COMMENT '商家id',
  product_id bigint NOT NULL DEFAULT 0 COMMENT '商品id',
  amount int NOT NULL DEFAULT 0 COMMENT '数量',
  status tinyint NOT NULL DEFAULT 0 COMMENT '订单状态 0-待支付 1-已支付 2-已关闭 3-已发货 4-已收货 5-申请退款 6-已退款',
  create_time timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  update_time timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uniq_request_id (request_id),
  KEY ix_user_id (user_id),
  KEY ix_merchant_status (merchant_id, status),
  KEY ix_create_time (create_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

SET @has_col = (SELECT COUNT(1) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = 'mall_order' AND TABLE_NAME = 'orders' AND COLUMN_NAME = 'merchant_id');
SET @sql = IF(@has_col = 0, 'ALTER TABLE orders ADD COLUMN merchant_id bigint NOT NULL DEFAULT 1000 AFTER user_id', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_idx = (SELECT COUNT(1) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = 'mall_order' AND TABLE_NAME = 'orders' AND INDEX_NAME = 'ix_merchant_status');
SET @sql = IF(@has_idx = 0, 'ALTER TABLE orders ADD KEY ix_merchant_status (merchant_id, status)', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

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
  merchant_id bigint NOT NULL DEFAULT 1000 COMMENT '商家id',
  supplier_id bigint NOT NULL DEFAULT 0 COMMENT '供应商id',
  product_name varchar(128) NOT NULL DEFAULT '' COMMENT '商品名快照',
  product_image_url varchar(512) NOT NULL DEFAULT '' COMMENT '商品图快照',
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
  KEY ix_merchant_id (merchant_id),
  KEY ix_product_id (product_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

SET @has_col = (SELECT COUNT(1) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = 'mall_order' AND TABLE_NAME = 'order_price_snapshot' AND COLUMN_NAME = 'merchant_id');
SET @sql = IF(@has_col = 0, 'ALTER TABLE order_price_snapshot ADD COLUMN merchant_id bigint NOT NULL DEFAULT 1000 AFTER product_id', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_col = (SELECT COUNT(1) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = 'mall_order' AND TABLE_NAME = 'order_price_snapshot' AND COLUMN_NAME = 'product_image_url');
SET @sql = IF(@has_col = 0, 'ALTER TABLE order_price_snapshot ADD COLUMN product_image_url varchar(512) NOT NULL DEFAULT '''' COMMENT ''商品图快照'' AFTER product_name', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_idx = (SELECT COUNT(1) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = 'mall_order' AND TABLE_NAME = 'order_price_snapshot' AND INDEX_NAME = 'ix_merchant_id');
SET @sql = IF(@has_idx = 0, 'ALTER TABLE order_price_snapshot ADD KEY ix_merchant_id (merchant_id)', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS payment_order (
  id varchar(64) NOT NULL COMMENT '支付单id',
  order_id varchar(64) NOT NULL COMMENT '订单id',
  user_id bigint NOT NULL DEFAULT 0 COMMENT '用户id',
  payable_amount_fen bigint NOT NULL DEFAULT 0 COMMENT '应付金额分',
  status tinyint NOT NULL DEFAULT 0 COMMENT '支付单状态 0-init 1-success 2-failed 3-closed',
  out_trade_no varchar(64) NOT NULL DEFAULT '' COMMENT '外部交易号',
  provider varchar(32) NOT NULL DEFAULT 'local_sandbox' COMMENT '支付渠道',
  provider_trade_no varchar(64) NOT NULL DEFAULT '' COMMENT '渠道交易号',
  provider_qr_url varchar(1024) NOT NULL DEFAULT '' COMMENT '渠道付款二维码内容',
  provider_status varchar(32) NOT NULL DEFAULT '' COMMENT '渠道状态',
  expires_at timestamp NULL DEFAULT NULL COMMENT '付款截止时间',
  inventory_finalize_status tinyint NOT NULL DEFAULT 0 COMMENT '库存确认 0-pending 1-success 2-failed',
  inventory_finalize_attempts int NOT NULL DEFAULT 0,
  inventory_finalize_error varchar(255) NOT NULL DEFAULT '',
  inventory_finalized_at timestamp NULL DEFAULT NULL,
  inventory_release_status tinyint NOT NULL DEFAULT 0 COMMENT '库存释放 0-pending 1-success 2-failed',
  inventory_release_attempts int NOT NULL DEFAULT 0,
  inventory_release_error varchar(255) NOT NULL DEFAULT '',
  inventory_released_at timestamp NULL DEFAULT NULL,
  create_time timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  update_time timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uniq_order_id (order_id),
  UNIQUE KEY uniq_out_trade_no (out_trade_no),
  KEY ix_payment_expiry (status, expires_at),
  KEY ix_inventory_finalize (status, inventory_finalize_status, update_time),
  KEY ix_inventory_release (status, inventory_release_status, update_time)
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

CREATE TABLE IF NOT EXISTS refund_order (
  id varchar(64) NOT NULL COMMENT '退款单id',
  order_id varchar(64) NOT NULL COMMENT '订单id',
  payment_order_id varchar(64) NOT NULL DEFAULT '' COMMENT '支付单id',
  user_id bigint NOT NULL DEFAULT 0 COMMENT '用户id',
  merchant_id bigint NOT NULL DEFAULT 1000 COMMENT '商家id',
  product_id bigint NOT NULL DEFAULT 0 COMMENT '商品id',
  refund_amount_fen bigint NOT NULL DEFAULT 0 COMMENT '退款金额分',
  status tinyint NOT NULL DEFAULT 0 COMMENT '0-requested 1-approved 2-success 3-rejected 4-failed',
  reason varchar(255) NOT NULL DEFAULT '' COMMENT '申请原因',
  audit_remark varchar(255) NOT NULL DEFAULT '' COMMENT '审核备注',
  operator_id bigint NOT NULL DEFAULT 0 COMMENT '审核人',
  provider varchar(32) NOT NULL DEFAULT 'local_sandbox' COMMENT '退款渠道',
  provider_refund_id varchar(64) NOT NULL DEFAULT '' COMMENT '渠道退款请求号',
  provider_status varchar(32) NOT NULL DEFAULT '' COMMENT '渠道退款状态',
  provider_error varchar(255) NOT NULL DEFAULT '' COMMENT '渠道退款错误',
  request_time timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  audit_time timestamp NULL DEFAULT NULL,
  finish_time timestamp NULL DEFAULT NULL,
  create_time timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  update_time timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uniq_order_id (order_id),
  KEY ix_user_id (user_id),
  KEY ix_merchant_status (merchant_id, status),
  KEY ix_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

SET @has_col = (SELECT COUNT(1) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = 'mall_order' AND TABLE_NAME = 'refund_order' AND COLUMN_NAME = 'merchant_id');
SET @sql = IF(@has_col = 0, 'ALTER TABLE refund_order ADD COLUMN merchant_id bigint NOT NULL DEFAULT 1000 AFTER user_id', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_idx = (SELECT COUNT(1) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = 'mall_order' AND TABLE_NAME = 'refund_order' AND INDEX_NAME = 'ix_merchant_status');
SET @sql = IF(@has_idx = 0, 'ALTER TABLE refund_order ADD KEY ix_merchant_status (merchant_id, status)', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS reconciliation_issue (
  id bigint NOT NULL AUTO_INCREMENT,
  issue_key varchar(192) NULL COMMENT 'Hertz 对账稳定幂等键；旧入口兼容为空',
  issue_type varchar(64) NOT NULL COMMENT '问题类型',
  order_id varchar(64) NOT NULL DEFAULT '' COMMENT '订单id',
  payment_order_id varchar(64) NOT NULL DEFAULT '' COMMENT '支付单id',
  refund_order_id varchar(64) NOT NULL DEFAULT '' COMMENT '退款单id',
  expected_amount_fen bigint NOT NULL DEFAULT 0,
  actual_amount_fen bigint NOT NULL DEFAULT 0,
  severity tinyint NOT NULL DEFAULT 1 COMMENT '1-low 2-medium 3-high',
  status tinyint NOT NULL DEFAULT 0 COMMENT '0-open 1-resolved 2-ignored',
  detail varchar(512) NOT NULL DEFAULT '',
  create_time timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  update_time timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uniq_issue_key (issue_key),
  KEY ix_status (status),
  KEY ix_order_id (order_id),
  KEY ix_issue_type (issue_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

SET @has_col = (SELECT COUNT(1) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = 'mall_order' AND TABLE_NAME = 'reconciliation_issue' AND COLUMN_NAME = 'issue_key');
SET @sql = IF(@has_col = 0, 'ALTER TABLE reconciliation_issue ADD COLUMN issue_key varchar(192) NULL AFTER id', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

UPDATE reconciliation_issue
SET issue_key = CONCAT(issue_type, ':', order_id, ':', payment_order_id, ':', refund_order_id, ':legacy:', id)
WHERE issue_key IS NULL OR issue_key = '';

SET @has_idx = (SELECT COUNT(1) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = 'mall_order' AND TABLE_NAME = 'reconciliation_issue' AND INDEX_NAME = 'uniq_issue_key');
SET @sql = IF(@has_idx = 0, 'ALTER TABLE reconciliation_issue ADD UNIQUE KEY uniq_issue_key (issue_key)', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

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

CREATE TABLE IF NOT EXISTS event_process_log (
  id bigint NOT NULL AUTO_INCREMENT,
  event_id varchar(128) NOT NULL,
  event_type varchar(64) NOT NULL,
  aggregate_id varchar(64) NOT NULL DEFAULT '',
  consumer varchar(64) NOT NULL DEFAULT '',
  status tinyint NOT NULL DEFAULT 0 COMMENT '0-processing 1-success 2-failed 3-dead',
  attempt_count int NOT NULL DEFAULT 0,
  last_error varchar(255) NOT NULL DEFAULT '',
  create_time timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  update_time timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uniq_consumer_event (consumer, event_id),
  KEY ix_status (status),
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
USE mall_order;

-- 支付渠道和库存收尾字段：为已有数据库提供幂等迁移；新库由 10-order.sql 直接建出完整表结构。
SET @has_col = (SELECT COUNT(1) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'payment_order' AND COLUMN_NAME = 'provider');
SET @sql = IF(@has_col = 0, 'ALTER TABLE payment_order ADD COLUMN provider varchar(32) NOT NULL DEFAULT ''local_sandbox'' AFTER out_trade_no', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_col = (SELECT COUNT(1) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'payment_order' AND COLUMN_NAME = 'provider_trade_no');
SET @sql = IF(@has_col = 0, 'ALTER TABLE payment_order ADD COLUMN provider_trade_no varchar(64) NOT NULL DEFAULT '''' AFTER provider', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_col = (SELECT COUNT(1) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'payment_order' AND COLUMN_NAME = 'provider_qr_url');
SET @sql = IF(@has_col = 0, 'ALTER TABLE payment_order ADD COLUMN provider_qr_url varchar(1024) NOT NULL DEFAULT '''' AFTER provider_trade_no', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_col = (SELECT COUNT(1) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'payment_order' AND COLUMN_NAME = 'provider_status');
SET @sql = IF(@has_col = 0, 'ALTER TABLE payment_order ADD COLUMN provider_status varchar(32) NOT NULL DEFAULT '''' AFTER provider_qr_url', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_col = (SELECT COUNT(1) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'payment_order' AND COLUMN_NAME = 'expires_at');
SET @sql = IF(@has_col = 0, 'ALTER TABLE payment_order ADD COLUMN expires_at timestamp NULL DEFAULT NULL AFTER provider_status', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_col = (SELECT COUNT(1) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'payment_order' AND COLUMN_NAME = 'inventory_finalize_status');
SET @sql = IF(@has_col = 0, 'ALTER TABLE payment_order ADD COLUMN inventory_finalize_status tinyint NOT NULL DEFAULT 0 AFTER expires_at', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_col = (SELECT COUNT(1) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'payment_order' AND COLUMN_NAME = 'inventory_finalize_attempts');
SET @sql = IF(@has_col = 0, 'ALTER TABLE payment_order ADD COLUMN inventory_finalize_attempts int NOT NULL DEFAULT 0 AFTER inventory_finalize_status', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_col = (SELECT COUNT(1) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'payment_order' AND COLUMN_NAME = 'inventory_finalize_error');
SET @sql = IF(@has_col = 0, 'ALTER TABLE payment_order ADD COLUMN inventory_finalize_error varchar(255) NOT NULL DEFAULT '''' AFTER inventory_finalize_attempts', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_col = (SELECT COUNT(1) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'payment_order' AND COLUMN_NAME = 'inventory_finalized_at');
SET @sql = IF(@has_col = 0, 'ALTER TABLE payment_order ADD COLUMN inventory_finalized_at timestamp NULL DEFAULT NULL AFTER inventory_finalize_error', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_col = (SELECT COUNT(1) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'payment_order' AND COLUMN_NAME = 'inventory_release_status');
SET @sql = IF(@has_col = 0, 'ALTER TABLE payment_order ADD COLUMN inventory_release_status tinyint NOT NULL DEFAULT 0 AFTER inventory_finalized_at', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_col = (SELECT COUNT(1) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'payment_order' AND COLUMN_NAME = 'inventory_release_attempts');
SET @sql = IF(@has_col = 0, 'ALTER TABLE payment_order ADD COLUMN inventory_release_attempts int NOT NULL DEFAULT 0 AFTER inventory_release_status', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_col = (SELECT COUNT(1) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'payment_order' AND COLUMN_NAME = 'inventory_release_error');
SET @sql = IF(@has_col = 0, 'ALTER TABLE payment_order ADD COLUMN inventory_release_error varchar(255) NOT NULL DEFAULT '''' AFTER inventory_release_attempts', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_col = (SELECT COUNT(1) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'payment_order' AND COLUMN_NAME = 'inventory_released_at');
SET @sql = IF(@has_col = 0, 'ALTER TABLE payment_order ADD COLUMN inventory_released_at timestamp NULL DEFAULT NULL AFTER inventory_release_error', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_idx = (SELECT COUNT(1) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'payment_order' AND INDEX_NAME = 'ix_payment_expiry');
SET @sql = IF(@has_idx = 0, 'ALTER TABLE payment_order ADD KEY ix_payment_expiry (status, expires_at)', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_idx = (SELECT COUNT(1) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'payment_order' AND INDEX_NAME = 'ix_inventory_finalize');
SET @sql = IF(@has_idx = 0, 'ALTER TABLE payment_order ADD KEY ix_inventory_finalize (status, inventory_finalize_status, update_time)', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_idx = (SELECT COUNT(1) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'payment_order' AND INDEX_NAME = 'ix_inventory_release');
SET @sql = IF(@has_idx = 0, 'ALTER TABLE payment_order ADD KEY ix_inventory_release (status, inventory_release_status, update_time)', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_col = (SELECT COUNT(1) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'refund_order' AND COLUMN_NAME = 'provider');
SET @sql = IF(@has_col = 0, 'ALTER TABLE refund_order ADD COLUMN provider varchar(32) NOT NULL DEFAULT ''local_sandbox'' AFTER operator_id', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_col = (SELECT COUNT(1) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'refund_order' AND COLUMN_NAME = 'provider_refund_id');
SET @sql = IF(@has_col = 0, 'ALTER TABLE refund_order ADD COLUMN provider_refund_id varchar(64) NOT NULL DEFAULT '''' AFTER provider', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_col = (SELECT COUNT(1) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'refund_order' AND COLUMN_NAME = 'provider_status');
SET @sql = IF(@has_col = 0, 'ALTER TABLE refund_order ADD COLUMN provider_status varchar(32) NOT NULL DEFAULT '''' AFTER provider_refund_id', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_col = (SELECT COUNT(1) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'refund_order' AND COLUMN_NAME = 'provider_error');
SET @sql = IF(@has_col = 0, 'ALTER TABLE refund_order ADD COLUMN provider_error varchar(255) NOT NULL DEFAULT '''' AFTER provider_status', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
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
USE mall_auth;

CREATE TABLE IF NOT EXISTS users (
  id bigint NOT NULL AUTO_INCREMENT COMMENT '用户ID',
  display_name varchar(64) NOT NULL DEFAULT '' COMMENT '展示昵称',
  role varchar(32) NOT NULL DEFAULT 'user' COMMENT '用户角色',
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

CREATE TABLE IF NOT EXISTS user_member_profile (
  user_id bigint NOT NULL,
  member_level varchar(32) NOT NULL DEFAULT 'standard',
  risk_level tinyint NOT NULL DEFAULT 0 COMMENT '0-normal 1-watch 2-restricted 3-blocked',
  total_paid_orders int NOT NULL DEFAULT 0,
  total_paid_amount_fen bigint NOT NULL DEFAULT 0,
  last_order_at timestamp NULL DEFAULT NULL,
  create_time timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  update_time timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (user_id),
  KEY ix_risk_level (risk_level),
  KEY ix_member_level (member_level)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS user_address (
  id bigint NOT NULL AUTO_INCREMENT,
  user_id bigint NOT NULL,
  receiver_name varchar(64) NOT NULL DEFAULT '',
  receiver_phone varchar(32) NOT NULL DEFAULT '',
  province varchar(64) NOT NULL DEFAULT '',
  city varchar(64) NOT NULL DEFAULT '',
  district varchar(64) NOT NULL DEFAULT '',
  detail varchar(255) NOT NULL DEFAULT '',
  is_default tinyint NOT NULL DEFAULT 0,
  status tinyint NOT NULL DEFAULT 1 COMMENT '1-active 2-deleted',
  create_time timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  update_time timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY ix_user_status (user_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS user_risk_snapshot (
  id bigint NOT NULL AUTO_INCREMENT,
  user_id bigint NOT NULL,
  risk_score int NOT NULL DEFAULT 0,
  risk_level tinyint NOT NULL DEFAULT 0,
  reason varchar(255) NOT NULL DEFAULT '',
  source varchar(64) NOT NULL DEFAULT 'system',
  create_time timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY ix_user_time (user_id, create_time),
  KEY ix_risk_level (risk_level)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
-- ========== auth migration: add missing columns if tables already exist ==========

SET @has_col = (SELECT COUNT(1) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'users' AND COLUMN_NAME = 'role');
SET @sql = IF(@has_col = 0, 'ALTER TABLE users ADD COLUMN role varchar(32) NOT NULL DEFAULT ''user'' AFTER display_name', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

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
