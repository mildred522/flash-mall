-- 初始化示例商品
INSERT INTO product (id, merchant_id, name, image_url, stock, version, origin_price_fen, sale_price_fen, status, supplier_id)
VALUES (100, 1000, '首发风衣', '/products/100.svg', 10000, 0, 12900, 11900, 1, 200)
ON DUPLICATE KEY UPDATE
  merchant_id = VALUES(merchant_id),
  name = VALUES(name),
  image_url = VALUES(image_url),
  stock = VALUES(stock),
  origin_price_fen = VALUES(origin_price_fen),
  sale_price_fen = VALUES(sale_price_fen),
  status = VALUES(status),
  supplier_id = VALUES(supplier_id);

-- 初始化分桶库存（默认 4 桶）
INSERT INTO product_stock_bucket (product_id, bucket_idx, stock, version)
VALUES
  (100, 0, 2500, 0),
  (100, 1, 2500, 0),
  (100, 2, 2500, 0),
  (100, 3, 2500, 0)
ON DUPLICATE KEY UPDATE stock = VALUES(stock), version = VALUES(version);

DELETE FROM promotion_rule
WHERE product_id = 100
  AND type = 'LIMITED_PRICE';

INSERT INTO promotion_rule (product_id, type, discount_value, threshold_amount, starts_at, ends_at, status)
VALUES (100, 'LIMITED_PRICE', 9900, 0, DATE_SUB(NOW(), INTERVAL 1 DAY), DATE_ADD(NOW(), INTERVAL 30 DAY), 1);

-- 新增商品 101-104
INSERT INTO product (id, merchant_id, name, image_url, stock, version, origin_price_fen, sale_price_fen, status, supplier_id)
VALUES
  (101, 1000, '轻薄羽绒服', '/products/101.svg', 10000, 0, 39900, 25900, 1, 200),
  (102, 1000, '纯棉T恤三件套', '/products/102.svg', 10000, 0, 15900, 9900, 1, 200),
  (103, 1000, '运动休闲鞋', '/products/103.svg', 10000, 0, 49900, 32900, 1, 200),
  (104, 1000, '便携充电宝', '/products/104.svg', 10000, 0, 12900, 7900, 1, 200)
ON DUPLICATE KEY UPDATE
  merchant_id = VALUES(merchant_id),
  name = VALUES(name), image_url = VALUES(image_url), stock = VALUES(stock),
  origin_price_fen = VALUES(origin_price_fen), sale_price_fen = VALUES(sale_price_fen),
  status = VALUES(status), supplier_id = VALUES(supplier_id);

-- 两家示例店铺各保留两件核心商品，素材随前端构建进入 /products/demo/。
INSERT INTO product (id, merchant_id, name, image_url, stock, version, origin_price_fen, sale_price_fen, status, supplier_id)
VALUES
  (201, 1101, '桂花乌龙手工曲奇', '/products/demo/shanlan-cookie.webp', 36, 0, 6800, 5800, 1, 201),
  (202, 1101, '海盐黑巧布朗尼礼盒', '/products/demo/shanlan-brownie.webp', 28, 0, 8900, 7900, 1, 201),
  (211, 1102, '暮野轻量露营灯', '/products/demo/north36-lantern.webp', 42, 0, 23900, 19900, 1, 211),
  (212, 1102, '云岭真空保温瓶', '/products/demo/north36-bottle.webp', 55, 0, 18900, 15900, 1, 211)
ON DUPLICATE KEY UPDATE
  merchant_id = VALUES(merchant_id),
  name = VALUES(name),
  image_url = VALUES(image_url),
  origin_price_fen = VALUES(origin_price_fen),
  sale_price_fen = VALUES(sale_price_fen),
  status = VALUES(status),
  supplier_id = VALUES(supplier_id);

INSERT IGNORE INTO product_inventory_seed
  (product_id, desired_total, shard_count, desired_product_status, status, attempt_count, last_error, seeded_at)
SELECT id, stock, 4, status, 1, 1, '', NOW()
FROM product;

INSERT INTO homepage_showcase (id, version, operator_id)
VALUES (1, 1, 0)
ON DUPLICATE KEY UPDATE id = VALUES(id);

-- 早期默认布局将同一商家的五件商品全部放到首页，与“每商家最多两个槽位”冲突。
-- 只收敛从未人工发布过的精确旧种子，不覆盖管理员已经发布的布局。
DELETE item
FROM homepage_showcase_item item
JOIN homepage_showcase showcase ON showcase.id = item.showcase_id
WHERE showcase.id = 1
  AND showcase.version = 1
  AND showcase.operator_id = 0
  AND showcase.publish_time IS NULL
  AND (
    (item.slot_no = 3 AND item.product_id = 102)
    OR (item.slot_no = 4 AND item.product_id = 103)
    OR (item.slot_no = 5 AND item.product_id = 104)
  );

INSERT INTO homepage_showcase_item (showcase_id, slot_no, product_id)
SELECT seed.showcase_id, seed.slot_no, seed.product_id
FROM (
  SELECT 1 AS showcase_id, 1 AS slot_no, 100 AS product_id
  UNION ALL SELECT 1, 2, 101
) AS seed
WHERE NOT EXISTS (
  SELECT 1 FROM homepage_showcase_item existing WHERE existing.showcase_id = 1
);

-- 仅扩展从未由管理员发布过的默认橱窗；已发布布局保持原样。
INSERT IGNORE INTO homepage_showcase_item (showcase_id, slot_no, product_id)
SELECT demo.showcase_id, demo.slot_no, demo.product_id
FROM (
  SELECT 1 AS showcase_id, 3 AS slot_no, 201 AS product_id
  UNION ALL SELECT 1, 4, 202
  UNION ALL SELECT 1, 5, 211
  UNION ALL SELECT 1, 6, 212
) AS demo
JOIN homepage_showcase showcase
  ON showcase.id = demo.showcase_id
 AND showcase.version = 1
 AND showcase.operator_id = 0
 AND showcase.publish_time IS NULL;

INSERT INTO product_stock_bucket (product_id, bucket_idx, stock, version) VALUES
  (101, 0, 2500, 0), (101, 1, 2500, 0), (101, 2, 2500, 0), (101, 3, 2500, 0),
  (102, 0, 2500, 0), (102, 1, 2500, 0), (102, 2, 2500, 0), (102, 3, 2500, 0),
  (103, 0, 2500, 0), (103, 1, 2500, 0), (103, 2, 2500, 0), (103, 3, 2500, 0),
  (104, 0, 2500, 0), (104, 1, 2500, 0), (104, 2, 2500, 0), (104, 3, 2500, 0)
ON DUPLICATE KEY UPDATE stock = VALUES(stock), version = VALUES(version);

-- 示例商家库存只在首次初始化时创建；项目重启不能覆盖已经发生的预占和扣减。
INSERT IGNORE INTO product_stock_bucket (product_id, bucket_idx, stock, version) VALUES
  (201, 0, 9, 0), (201, 1, 9, 0), (201, 2, 9, 0), (201, 3, 9, 0),
  (202, 0, 7, 0), (202, 1, 7, 0), (202, 2, 7, 0), (202, 3, 7, 0),
  (211, 0, 11, 0), (211, 1, 11, 0), (211, 2, 10, 0), (211, 3, 10, 0),
  (212, 0, 14, 0), (212, 1, 14, 0), (212, 2, 14, 0), (212, 3, 13, 0);

INSERT IGNORE INTO product_stock_snapshot (product_id, available, reserved, total, source, version)
VALUES
  (201, 36, 0, 36, 'demo-seed', 1),
  (202, 28, 0, 28, 'demo-seed', 1),
  (211, 42, 0, 42, 'demo-seed', 1),
  (212, 55, 0, 55, 'demo-seed', 1);

INSERT INTO product_card_snapshot
  (product_id, name, origin_price_fen, final_price_fen, promotion_type, promotion_tag, stock_available, supplier_id, status, version)
VALUES
  (201, '桂花乌龙手工曲奇', 6800, 5800, 'LIMITED_PRICE', '限时价', 36, 201, 1, 1),
  (202, '海盐黑巧布朗尼礼盒', 8900, 7900, 'LIMITED_PRICE', '限时价', 28, 201, 1, 1),
  (211, '暮野轻量露营灯', 23900, 19900, 'LIMITED_PRICE', '限时价', 42, 211, 1, 1),
  (212, '云岭真空保温瓶', 18900, 15900, 'LIMITED_PRICE', '限时价', 55, 211, 1, 1)
ON DUPLICATE KEY UPDATE
  name = VALUES(name),
  origin_price_fen = VALUES(origin_price_fen),
  final_price_fen = VALUES(final_price_fen),
  promotion_type = VALUES(promotion_type),
  promotion_tag = VALUES(promotion_tag),
  supplier_id = VALUES(supplier_id),
  status = VALUES(status),
  version = version + 1;

DELETE FROM promotion_rule WHERE product_id IN (101, 102, 103, 104) AND type = 'LIMITED_PRICE';
INSERT INTO promotion_rule (product_id, type, discount_value, threshold_amount, starts_at, ends_at, status) VALUES
  (101, 'LIMITED_PRICE', 25900, 0, DATE_SUB(NOW(), INTERVAL 1 DAY), DATE_ADD(NOW(), INTERVAL 30 DAY), 1),
  (102, 'LIMITED_PRICE', 9900, 0, DATE_SUB(NOW(), INTERVAL 1 DAY), DATE_ADD(NOW(), INTERVAL 30 DAY), 1),
  (103, 'LIMITED_PRICE', 32900, 0, DATE_SUB(NOW(), INTERVAL 1 DAY), DATE_ADD(NOW(), INTERVAL 30 DAY), 1),
  (104, 'LIMITED_PRICE', 7900, 0, DATE_SUB(NOW(), INTERVAL 1 DAY), DATE_ADD(NOW(), INTERVAL 30 DAY), 1);

DELETE FROM promotion_rule WHERE product_id IN (201, 202, 211, 212) AND type = 'LIMITED_PRICE';
INSERT INTO promotion_rule (product_id, type, discount_value, threshold_amount, starts_at, ends_at, status) VALUES
  (201, 'LIMITED_PRICE', 5800, 0, DATE_SUB(NOW(), INTERVAL 1 DAY), DATE_ADD(NOW(), INTERVAL 180 DAY), 1),
  (202, 'LIMITED_PRICE', 7900, 0, DATE_SUB(NOW(), INTERVAL 1 DAY), DATE_ADD(NOW(), INTERVAL 180 DAY), 1),
  (211, 'LIMITED_PRICE', 19900, 0, DATE_SUB(NOW(), INTERVAL 1 DAY), DATE_ADD(NOW(), INTERVAL 180 DAY), 1),
  (212, 'LIMITED_PRICE', 15900, 0, DATE_SUB(NOW(), INTERVAL 1 DAY), DATE_ADD(NOW(), INTERVAL 180 DAY), 1);
