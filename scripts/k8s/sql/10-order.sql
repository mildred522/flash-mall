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

INSERT INTO merchant (id, name, owner_user_id, status, contact_phone)
VALUES (1000, 'Flash Mall 自营店', 1001, 1, '13800000001')
ON DUPLICATE KEY UPDATE
  name = VALUES(name),
  owner_user_id = VALUES(owner_user_id),
  status = VALUES(status),
  contact_phone = VALUES(contact_phone);

INSERT INTO merchant_user (merchant_id, user_id, role, status)
VALUES (1000, 1001, 'owner', 1)
ON DUPLICATE KEY UPDATE role = VALUES(role), status = VALUES(status);

-- 可直接演示的真实感商家：公开店铺、商家账号和商品数据使用固定 ID，重复初始化不会产生副本。
INSERT INTO merchant (id, name, owner_user_id, status, contact_phone)
VALUES
  (1101, '山岚烘焙研究所', 1101, 1, '13800001101'),
  (1102, '北纬三十六户外', 1102, 1, '13800001102')
ON DUPLICATE KEY UPDATE
  name = VALUES(name),
  owner_user_id = VALUES(owner_user_id),
  status = VALUES(status),
  contact_phone = VALUES(contact_phone);

INSERT INTO merchant_user (merchant_id, user_id, role, status)
VALUES
  (1101, 1101, 'owner', 1),
  (1102, 1102, 'owner', 1)
ON DUPLICATE KEY UPDATE role = VALUES(role), status = VALUES(status);

INSERT INTO merchant_store_profile (merchant_id, logo_url, banner_url, description, version)
VALUES
  (1101, '/products/demo/shanlan-logo.svg', '/products/demo/shanlan-banner.webp',
   '从山城清晨的香气出发，坚持小批次手作。我们把茶、谷物与当季风味做进每天都愿意分享的烘焙点心。', 1),
  (1102, '/products/demo/north36-logo.svg', '/products/demo/north36-banner.webp',
   '为周末山野与城市通勤挑选克制、耐用的装备。少一点负担，多一点可靠，把每件器具真正带到户外。', 1)
ON DUPLICATE KEY UPDATE
  logo_url = VALUES(logo_url),
  banner_url = VALUES(banner_url),
  description = VALUES(description);

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
