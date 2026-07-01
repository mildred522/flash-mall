#!/usr/bin/env sh
set -eu

cluster_name="${FLASH_MALL_KIND_CLUSTER:-flash-mall}"

while [ "$#" -gt 0 ]; do
  case "$1" in
    --cluster)
      shift
      cluster_name="${1:-}"
      [ -n "$cluster_name" ] || { echo "--cluster requires name" >&2; exit 2; }
      ;;
    -h|--help)
      cat <<'EOF'
Usage: scripts/k8s/load-images.sh [options]

Options:
  --cluster NAME  kind cluster name. Default: flash-mall.
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

command -v docker >/dev/null 2>&1 || {
  echo "docker not found in PATH" >&2
  exit 1
}
command -v kind >/dev/null 2>&1 || {
  echo "kind not found in PATH" >&2
  exit 1
}

images='flash-mall/auth-api:dev
flash-mall/entry-api:dev
flash-mall/order-rpc:dev
flash-mall/product-rpc:dev
flash-mall/inventory-kitex:dev
mysql:8.0
redis:7
rabbitmq:3.13-management
bitnamilegacy/etcd:3.5
yedf/dtm:latest'

printf '%s\n' "$images" | while IFS= read -r image; do
  [ -n "$image" ] || continue
  if ! docker image inspect "$image" >/dev/null 2>&1; then
    echo "[PULL] $image"
    docker pull "$image"
  fi
  echo "[KIND LOAD] $image"
  kind load docker-image "$image" --name "$cluster_name"
done
