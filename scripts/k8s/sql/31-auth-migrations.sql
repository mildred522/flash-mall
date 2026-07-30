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
