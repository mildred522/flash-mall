-- 订单生命周期迁移：新增状态字段和状态日志表
-- 适用于已有 orders 表的数据库

USE mall_order;

SET NAMES utf8mb4;

-- 新增时间戳列
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

-- 状态日志表
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
