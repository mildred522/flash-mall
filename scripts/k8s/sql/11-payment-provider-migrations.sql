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
