#!/usr/bin/env sh
set -eu

namespace="${1:-${FLASH_MALL_K8S_NAMESPACE:-flash-mall}}"
health_url="${FLASH_MALL_HEALTH_URL:-http://127.0.0.1:8888/api/system/health}"

command -v kubectl >/dev/null 2>&1 || {
  echo "kubectl not found in PATH" >&2
  exit 1
}

echo "[STATUS] pods"
kubectl -n "$namespace" get pods -o wide || true

echo "[STATUS] services"
kubectl -n "$namespace" get svc || true

if command -v curl >/dev/null 2>&1; then
  echo "[HEALTH] $health_url"
  curl --noproxy "*" -fsS -m 5 "$health_url" || true
  echo
fi

echo "[MYSQL] product stock buckets"
mysql_pod=$(kubectl -n "$namespace" get pods -l app=mysql -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
if [ -n "$mysql_pod" ]; then
  kubectl -n "$namespace" exec "$mysql_pod" -- sh -lc 'MYSQL_PWD=flashmall mysql -uroot --default-character-set=utf8mb4 -e "SELECT p.id,p.name,COALESCE(SUM(b.stock),0) AS stock_available FROM mall_product.product p LEFT JOIN mall_product.product_stock_bucket b ON b.product_id=p.id GROUP BY p.id,p.name ORDER BY p.id LIMIT 50"' || true
else
  echo "mysql pod not found"
fi

echo "[REDIS] optional flash-sale stock keys"
redis_pod=$(kubectl -n "$namespace" get pods -l app=redis -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
if [ -n "$redis_pod" ]; then
  kubectl -n "$namespace" exec "$redis_pod" -- sh -lc 'redis-cli --scan --pattern "stock:*" | sort | head -40 | while read key; do printf "%s=" "$key"; redis-cli GET "$key"; done' || true
else
  echo "redis pod not found"
fi
