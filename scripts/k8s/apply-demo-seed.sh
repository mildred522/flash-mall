#!/usr/bin/env sh
set -eu

expected_version="${FLASH_MALL_DEMO_FIXTURE_VERSION:-20260730_demo_fixture_v1}"
enabled="${FLASH_MALL_DEMO_SEED_ENABLED:-false}"
host="${FLASH_MALL_MYSQL_HOST:-mysql}"
user="${FLASH_MALL_MYSQL_USER:-root}"
demo_sql="${FLASH_MALL_DEMO_SEED_SQL:-/init/demo-seed.sql}"

mysql_query() {
  mysql --default-character-set=utf8mb4 -N -h"$host" -u"$user" -p"$MYSQL_ROOT_PASSWORD" -e "$1"
}

if [ "$enabled" != "true" ]; then
  echo "[DEMO] demo fixtures disabled"
  exit 0
fi

has_state=$(mysql_query "
  SELECT COUNT(*)
  FROM information_schema.tables
  WHERE table_schema='mall_order' AND table_name='demo_fixture_state';
")
if [ "$has_state" = "1" ]; then
  active=$(mysql_query "
    SELECT COUNT(*) FROM mall_order.demo_fixture_state
    WHERE fixture_version='${expected_version}';
  ")
  if [ "$active" = "1" ]; then
    echo "[DEMO] fixture ${expected_version} already initialized"
    exit 0
  fi
  echo "[DEMO] fixture version is stale; run an explicit demo reset" >&2
  exit 3
fi

existing=$(mysql_query "
  SELECT
    (SELECT COUNT(*) FROM mall_auth.users WHERE id IN (1001,1002,1101,1102)) +
    (SELECT COUNT(*) FROM mall_order.merchant WHERE id IN (1000,1101,1102)) +
    (SELECT COUNT(*) FROM mall_product.product WHERE id IN (100,101,102,103,104,201,202,211,212));
")
if [ "$existing" = "16" ]; then
  auth_dependencies=$(mysql_query "
    SELECT
      (SELECT COUNT(*) FROM mall_auth.user_identities
       WHERE user_id IN (1001,1002,1101,1102) AND identity_type='phone') +
      (SELECT COUNT(*) FROM mall_auth.user_credentials
       WHERE user_id IN (1001,1002,1101,1102) AND credential_type='password');
  ")
  merchant_dependencies=$(mysql_query "
    SELECT
      (SELECT COUNT(*) FROM mall_order.merchant_user
       WHERE merchant_id IN (1000,1101,1102) AND status=1) +
      (SELECT COUNT(*) FROM mall_order.merchant_store_profile
       WHERE merchant_id IN (1101,1102));
  ")
  stock_mismatches=$(mysql_query "
    SELECT COUNT(*) FROM (
      SELECT p.id
      FROM mall_product.product p
      LEFT JOIN mall_product.product_stock_snapshot s ON s.product_id=p.id
      LEFT JOIN mall_product.product_stock_bucket b ON b.product_id=p.id
      WHERE p.id IN (100,101,102,103,104,201,202,211,212)
      GROUP BY p.id,p.stock,s.product_id,s.available,s.reserved,s.total
      HAVING s.product_id IS NULL
        OR p.stock<>s.total
        OR s.available+s.reserved<>s.total
        OR COALESCE(SUM(b.stock),0)<>s.total
    ) inconsistent;
  ")
  if [ "$auth_dependencies" != "8" ] ||
     [ "$merchant_dependencies" != "5" ] ||
     [ "$stock_mismatches" != "0" ]; then
    echo "[DEMO] existing core fixtures are incomplete; run an explicit demo reset" >&2
    exit 5
  fi
  mysql_query "
    CREATE TABLE mall_order.demo_fixture_state (
      fixture_version varchar(64) NOT NULL,
      applied_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
      PRIMARY KEY (fixture_version)
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
    INSERT INTO mall_order.demo_fixture_state (fixture_version)
    VALUES ('${expected_version}');
  " >/dev/null
  echo "[DEMO] adopted existing fixtures as ${expected_version} without rewriting business data"
  exit 0
fi

if [ "$existing" != "0" ]; then
  echo "[DEMO] partial fixtures detected (${existing}/16); run an explicit demo reset" >&2
  exit 4
fi

mysql --default-character-set=utf8mb4 -h"$host" -u"$user" -p"$MYSQL_ROOT_PASSWORD" < "$demo_sql"
echo "[DEMO] fixture ${expected_version} initialized"
