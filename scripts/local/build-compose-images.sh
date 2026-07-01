#!/usr/bin/env sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(CDPATH= cd -- "$script_dir/../.." && pwd)
tag="${1:-${FLASH_MALL_IMAGE_TAG:-dev}}"
context_root="$repo_root/.runtime/docker-context"
dockerfile="$repo_root/build/docker/local-binary.Dockerfile"

services='auth-api:./app/auth/api
product-rpc:./app/product/rpc
order-rpc:./app/order/rpc
inventory-kitex:./app/inventory/kitex
entry-api:./app/entry/api'

command -v go >/dev/null 2>&1 || {
  echo "go not found in PATH" >&2
  exit 1
}
command -v docker >/dev/null 2>&1 || {
  echo "docker not found in PATH" >&2
  exit 1
}

mkdir -p "$context_root"

old_goos="${GOOS:-}"
old_goarch="${GOARCH:-}"
old_cgo="${CGO_ENABLED:-}"
export GOOS=linux
export GOARCH=amd64
export CGO_ENABLED=0

restore_env() {
  if [ -n "$old_goos" ]; then export GOOS="$old_goos"; else unset GOOS; fi
  if [ -n "$old_goarch" ]; then export GOARCH="$old_goarch"; else unset GOARCH; fi
  if [ -n "$old_cgo" ]; then export CGO_ENABLED="$old_cgo"; else unset CGO_ENABLED; fi
}
trap restore_env EXIT INT TERM

printf '%s\n' "$services" | while IFS=: read -r name package_path; do
  svc_context="$context_root/$name"
  mkdir -p "$svc_context"

  echo "[GO BUILD] $name"
  go build -trimpath -tags timetzdata -o "$svc_context/app" "$package_path"

  echo "[DOCKER BUILD] flash-mall/$name:$tag"
  if docker buildx version >/dev/null 2>&1; then
    docker buildx build --load -f "$dockerfile" -t "flash-mall/$name:$tag" "$svc_context"
  else
    docker build -f "$dockerfile" -t "flash-mall/$name:$tag" "$svc_context"
  fi
done
