#!/usr/bin/env sh
set -eu

namespace="${FLASH_MALL_K8S_NAMESPACE:-flash-mall}"
products="${FLASH_MALL_STOCK_PRODUCTS:-100 101 102 103 104}"
total_stock="${FLASH_MALL_STOCK_TOTAL:-10000}"
shards="${FLASH_MALL_STOCK_SHARDS:-4}"
force=0

while [ "$#" -gt 0 ]; do
  case "$1" in
    --namespace|-n)
      shift
      namespace="${1:-}"
      [ -n "$namespace" ] || { echo "--namespace requires name" >&2; exit 2; }
      ;;
    --products)
      shift
      products="${1:-}"
      [ -n "$products" ] || { echo "--products requires value" >&2; exit 2; }
      ;;
    --total-stock)
      shift
      total_stock="${1:-}"
      [ -n "$total_stock" ] || { echo "--total-stock requires value" >&2; exit 2; }
      ;;
    --shards)
      shift
      shards="${1:-}"
      [ -n "$shards" ] || { echo "--shards requires value" >&2; exit 2; }
      ;;
    --force)
      force=1
      ;;
    -h|--help)
      cat <<'EOF'
Usage: scripts/k8s/seed-redis-stock.sh [options]

Options:
  -n, --namespace NAME       Kubernetes namespace. Default: flash-mall.
  --products "100 101"       Product ids to seed. Default: 100 101 102 103 104.
  --total-stock N            Total stock per product. Default: 10000.
  --shards N                 Redis stock shard count. Default: 4.
  --force                    Overwrite existing stock keys instead of SETNX.
EOF
      exit 0
      ;;
    *)
      echo "unknown option: $1" >&2
      exit 2
      ;;
  esac
  shift
done

command -v kubectl >/dev/null 2>&1 || {
  echo "kubectl not found in PATH" >&2
  exit 1
}

if [ "$shards" -le 0 ]; then
  shards=1
fi

kubectl -n "$namespace" wait --for=condition=Ready pod -l app=redis --timeout=60s >/dev/null
redis_pod="$(kubectl -n "$namespace" get pods -l app=redis -o jsonpath='{.items[0].metadata.name}')"
[ -n "$redis_pod" ] || { echo "Redis pod not found in namespace $namespace" >&2; exit 1; }

per=$((total_stock / shards))
remain=$((total_stock % shards))

for product in $products; do
  i=0
  while [ "$i" -lt "$shards" ]; do
    value="$per"
    if [ "$i" -eq 0 ]; then
      value=$((per + remain))
    fi
    key="stock:${product}:${i}"
    if [ "$force" -eq 1 ]; then
      kubectl -n "$namespace" exec "$redis_pod" -- redis-cli SET "$key" "$value" >/dev/null
    else
      kubectl -n "$namespace" exec "$redis_pod" -- redis-cli SETNX "$key" "$value" >/dev/null
    fi
    i=$((i + 1))
  done
done

echo "[REDIS STOCK] seeded products=[$products] total=$total_stock shards=$shards force=$force"
