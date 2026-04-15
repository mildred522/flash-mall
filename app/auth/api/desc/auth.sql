CREATE TABLE `users` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT COMMENT '用户ID',
  `display_name` varchar(64) NOT NULL DEFAULT '' COMMENT '展示昵称',
  `status` tinyint(3) NOT NULL DEFAULT '1' COMMENT '状态 1-正常 2-禁用',
  `session_version` int(11) NOT NULL DEFAULT '1' COMMENT '会话版本号',
  `create_time` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `ix_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户主表';

CREATE TABLE `user_identities` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT COMMENT '身份ID',
  `user_id` bigint(20) NOT NULL COMMENT '用户ID',
  `identity_type` varchar(32) NOT NULL COMMENT '身份类型 phone/email',
  `identity_value` varchar(128) NOT NULL COMMENT '身份值',
  `is_verified` tinyint(1) NOT NULL DEFAULT '0' COMMENT '是否已验证',
  `verified_at` timestamp NULL DEFAULT NULL COMMENT '验证时间',
  `create_time` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_identity_type_value` (`identity_type`, `identity_value`),
  KEY `ix_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户身份表';

CREATE TABLE `user_credentials` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT COMMENT '凭证ID',
  `user_id` bigint(20) NOT NULL COMMENT '用户ID',
  `credential_type` varchar(32) NOT NULL COMMENT '凭证类型 password',
  `password_hash` varchar(255) NOT NULL COMMENT '密码哈希',
  `hash_algo` varchar(32) NOT NULL DEFAULT 'bcrypt' COMMENT '哈希算法',
  `password_updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP COMMENT '密码更新时间',
  `create_time` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_user_credential_type` (`user_id`, `credential_type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户凭证表';

CREATE TABLE `user_sessions` (
  `id` varchar(64) NOT NULL COMMENT '会话ID sid',
  `user_id` bigint(20) NOT NULL COMMENT '用户ID',
  `device_type` varchar(32) NOT NULL DEFAULT 'web' COMMENT '设备类型',
  `session_version` int(11) NOT NULL DEFAULT '1' COMMENT '会话版本号',
  `refresh_token_hash` char(64) NOT NULL COMMENT 'refresh token hash',
  `status` tinyint(3) NOT NULL DEFAULT '1' COMMENT '状态 1-活跃 2-失效 3-登出',
  `expires_at` timestamp NULL DEFAULT NULL COMMENT 'refresh 过期时间',
  `last_seen_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP COMMENT '最近活跃时间',
  `revoked_at` timestamp NULL DEFAULT NULL COMMENT '失效时间',
  `create_time` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_refresh_token_hash` (`refresh_token_hash`),
  KEY `ix_user_id_status` (`user_id`, `status`),
  KEY `ix_user_id_device_type` (`user_id`, `device_type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户会话表';

CREATE TABLE `verification_codes` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT COMMENT '验证码ID',
  `target` varchar(128) NOT NULL COMMENT '接收目标 手机号或邮箱',
  `scene` varchar(32) NOT NULL COMMENT '场景 register/login/reset-password',
  `code_hash` char(64) NOT NULL COMMENT '验证码哈希',
  `status` tinyint(3) NOT NULL DEFAULT '1' COMMENT '状态 1-待使用 2-已使用 3-已过期',
  `expires_at` timestamp NULL DEFAULT NULL COMMENT '过期时间',
  `consumed_at` timestamp NULL DEFAULT NULL COMMENT '消费时间',
  `send_count` int(11) NOT NULL DEFAULT '1' COMMENT '发送次数',
  `create_time` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `ix_target_scene_status` (`target`, `scene`, `status`),
  KEY `ix_expires_at` (`expires_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='验证码表';

CREATE TABLE `auth_audit_logs` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT COMMENT '审计日志ID',
  `user_id` bigint(20) DEFAULT NULL COMMENT '用户ID',
  `identity_value` varchar(128) NOT NULL DEFAULT '' COMMENT '身份值快照',
  `event_type` varchar(32) NOT NULL COMMENT '事件 login/register/refresh/logout/reset-password',
  `result` varchar(16) NOT NULL COMMENT '结果 success/fail',
  `ip` varchar(64) NOT NULL DEFAULT '' COMMENT 'IP地址',
  `user_agent` varchar(255) NOT NULL DEFAULT '' COMMENT '客户端UA',
  `detail_json` json DEFAULT NULL COMMENT '扩展详情',
  `create_time` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `ix_user_id` (`user_id`),
  KEY `ix_event_type_result` (`event_type`, `result`),
  KEY `ix_create_time` (`create_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='认证审计日志表';
