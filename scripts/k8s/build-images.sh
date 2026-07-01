#!/usr/bin/env sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(CDPATH= cd -- "$script_dir/../.." && pwd)
tag="${1:-${FLASH_MALL_IMAGE_TAG:-dev}}"

cd "$repo_root"

command -v docker >/dev/null 2>&1 || {
  echo "docker not found in PATH" >&2
  exit 1
}

services='auth-api:build/docker/auth-api.Dockerfile
entry-api:build/docker/entry-api.Dockerfile
order-rpc:build/docker/order-rpc.Dockerfile
product-rpc:build/docker/product-rpc.Dockerfile
inventory-kitex:build/docker/inventory-kitex.Dockerfile'

idx=1
total=5
printf '%s\n' "$services" | while IFS=: read -r name dockerfile; do
  echo "[$idx/$total] build flash-mall/$name:$tag"
  docker build -f "$dockerfile" -t "flash-mall/$name:$tag" .
  idx=$((idx + 1))
done
