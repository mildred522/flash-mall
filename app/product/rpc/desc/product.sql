-- CHG 2026-02-07: 变更=新增库存版本号; 之前=无乐观锁字段; 原因=高并发下避免写覆盖。
ALTER TABLE `product`
  ADD COLUMN `version` bigint NOT NULL DEFAULT 0 COMMENT '库存版本号' AFTER `stock`;
