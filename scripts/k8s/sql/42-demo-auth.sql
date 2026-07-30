USE mall_auth;

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

-- The marker is deliberately written after every demo domain has been seeded.
-- A failed product or auth statement must never make a partial fixture look ready.
USE mall_order;

INSERT INTO demo_fixture_state (fixture_version)
VALUES ('20260730_demo_fixture_v1')
ON DUPLICATE KEY UPDATE applied_at = VALUES(applied_at);
