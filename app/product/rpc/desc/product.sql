-- CHG 2026-02-07: 变更=新增库存版本号; 之前=无乐观锁字段; 原因=高并发下避免写覆盖。
ALTER TABLE `product`
  ADD COLUMN `version` bigint NOT NULL DEFAULT 0 COMMENT '库存版本号' AFTER `stock`;

-- 商品创建时间用于首页候选的新鲜度评分；历史数据由正式初始化脚本回填为 31 天前。
ALTER TABLE `product`
  ADD COLUMN `create_time` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  ADD KEY `ix_product_create_time` (`create_time`);
