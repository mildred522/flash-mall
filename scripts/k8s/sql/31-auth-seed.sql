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

-- 默认本地演示账号，保证 K8s 重建后商城和后台可直接登录。
INSERT INTO users (id, display_name, role, status, session_version)
VALUES
  (1001, 'Flash Mall User 1001', 'user', 1, 1),
  (1002, 'Flash Mall Admin', 'admin', 1, 1),
  (1101, '山岚店主', 'user', 1, 1),
  (1102, '北纬店主', 'user', 1, 1)
ON DUPLICATE KEY UPDATE
  display_name = VALUES(display_name),
  role = VALUES(role),
  status = VALUES(status),
  session_version = VALUES(session_version);

INSERT INTO user_identities (user_id, identity_type, identity_value, is_verified, verified_at)
VALUES
  (1001, 'phone', '13800000001', 1, NOW()),
  (1002, 'phone', '13800000002', 1, NOW()),
  (1101, 'phone', '13800001101', 1, NOW()),
  (1102, 'phone', '13800001102', 1, NOW())
ON DUPLICATE KEY UPDATE
  user_id = VALUES(user_id),
  is_verified = VALUES(is_verified),
  verified_at = VALUES(verified_at);

INSERT INTO user_credentials (user_id, credential_type, password_hash, hash_algo, password_updated_at)
VALUES
  (1001, 'password', '$2a$10$5.X2YBFgYtMcea4wccOGHOmPmDvtCTtyOIQ7IMkS5FGJNArFIj.Z.', 'bcrypt', NOW()),
  (1002, 'password', '$2a$10$FyWPrNrijW62LHfjVr7ROujdtlUFcdBz/im/Om7.6E66lb1/EemvC', 'bcrypt', NOW()),
  (1101, 'password', '$2a$10$5.X2YBFgYtMcea4wccOGHOmPmDvtCTtyOIQ7IMkS5FGJNArFIj.Z.', 'bcrypt', NOW()),
  (1102, 'password', '$2a$10$5.X2YBFgYtMcea4wccOGHOmPmDvtCTtyOIQ7IMkS5FGJNArFIj.Z.', 'bcrypt', NOW())
ON DUPLICATE KEY UPDATE
  password_hash = VALUES(password_hash),
  hash_algo = VALUES(hash_algo),
  password_updated_at = VALUES(password_updated_at);

-- ========== end auth migration ==========
