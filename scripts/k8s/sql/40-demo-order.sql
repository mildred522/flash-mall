USE mall_order;

CREATE TABLE IF NOT EXISTS demo_fixture_state (
  fixture_version varchar(64) NOT NULL,
  applied_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (fixture_version)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

INSERT INTO merchant (id, name, owner_user_id, status, contact_phone)
VALUES
  (1000, 'Flash Mall 自营店', 1001, 1, '13800000001'),
  (1101, '山岚烘焙研究所', 1101, 1, '13800001101'),
  (1102, '北纬三十六户外', 1102, 1, '13800001102')
ON DUPLICATE KEY UPDATE
  name = VALUES(name),
  owner_user_id = VALUES(owner_user_id),
  status = VALUES(status),
  contact_phone = VALUES(contact_phone);

INSERT INTO merchant_user (merchant_id, user_id, role, status)
VALUES
  (1000, 1001, 'owner', 1),
  (1101, 1101, 'owner', 1),
  (1102, 1102, 'owner', 1)
ON DUPLICATE KEY UPDATE role = VALUES(role), status = VALUES(status);

INSERT INTO merchant_store_profile (merchant_id, logo_url, banner_url, description, version)
VALUES
  (1101, '/products/demo/shanlan-logo.svg', '/products/demo/shanlan-banner.webp',
   '从山城清晨的香气出发，坚持小批次手作。我们把茶、谷物与当季风味做进每天都愿意分享的烘焙点心。', 1),
  (1102, '/products/demo/north36-logo.svg', '/products/demo/north36-banner.webp',
   '为周末山野与城市通勤挑选克制、耐用的装备。少一点负担，多一点可靠，把每件器具真正带到户外。', 1)
ON DUPLICATE KEY UPDATE
  logo_url = VALUES(logo_url),
  banner_url = VALUES(banner_url),
  description = VALUES(description);
