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
