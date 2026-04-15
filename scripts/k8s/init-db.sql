-- 初始化数据库与表结构（K8s MySQL）

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
  status tinyint NOT NULL DEFAULT 0 COMMENT '订单状态 0-待支付 1-已支付 2-已关闭',
  create_time timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  update_time timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uniq_request_id (request_id),
  KEY ix_user_id (user_id),
  KEY ix_create_time (create_time)
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

-- CHG 2026-02-24: 变更=新增库存分桶表; 之前=单行 product 表扣减; 原因=降低热点行冲突。
CREATE TABLE IF NOT EXISTS product_stock_bucket (
  product_id bigint NOT NULL,
  bucket_idx int NOT NULL,
  stock int NOT NULL DEFAULT 0,
  version bigint NOT NULL DEFAULT 0,
  PRIMARY KEY (product_id, bucket_idx)
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

-- 初始化示例商品
-- 兼容旧表结构：仅写入必需字段，避免历史库缺少 name/version 时初始化失败
INSERT INTO product (id, stock)
VALUES (100, 10000)
ON DUPLICATE KEY UPDATE stock = VALUES(stock);

-- 初始化分桶库存（默认 4 桶）
INSERT INTO product_stock_bucket (product_id, bucket_idx, stock, version)
VALUES
  (100, 0, 2500, 0),
  (100, 1, 2500, 0),
  (100, 2, 2500, 0),
  (100, 3, 2500, 0)
ON DUPLICATE KEY UPDATE stock = VALUES(stock), version = VALUES(version);

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
  status tinyint NOT NULL DEFAULT 1 COMMENT '状态 1-活跃 2-失效 3-登出',
  expires_at timestamp NULL DEFAULT NULL COMMENT 'refresh 过期时间',
  last_seen_at timestamp NULL DEFAULT CURRENT_TIMESTAMP COMMENT '最近活跃时间',
  revoked_at timestamp NULL DEFAULT NULL COMMENT '失效时间',
  create_time timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  update_time timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uniq_refresh_token_hash (refresh_token_hash),
  KEY ix_user_id_status (user_id, status),
  KEY ix_user_id_device_type (user_id, device_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS verification_codes (
  id bigint NOT NULL AUTO_INCREMENT COMMENT '验证码ID',
  target varchar(128) NOT NULL COMMENT '接收目标',
  scene varchar(32) NOT NULL COMMENT '场景',
  code_hash char(64) NOT NULL COMMENT '验证码哈希',
  status tinyint NOT NULL DEFAULT 1 COMMENT '状态 1-待使用 2-已使用 3-已过期',
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
