#!/usr/bin/env sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(CDPATH= cd -- "$script_dir/../.." && pwd)
tag="${FLASH_MALL_IMAGE_TAG:-dev}"
context_root="$repo_root/.runtime/docker-context"
dockerfile="$repo_root/build/docker/local-binary.Dockerfile"
all_services="auth-api product-rpc order-rpc inventory-kitex entry-api hertz-gateway"
requested_services=""

usage() {
  cat <<'EOF'
Usage: scripts/local/build-compose-images.sh [--tag TAG] [SERVICE...]

Build all services when SERVICE is omitted. Supported services:
  auth-api product-rpc order-rpc inventory-kitex entry-api hertz-gateway
EOF
}

is_service() {
  case "$1" in
    auth-api|product-rpc|order-rpc|inventory-kitex|entry-api|hertz-gateway) return 0 ;;
    *) return 1 ;;
  esac
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --tag)
      shift
      if [ "$#" -eq 0 ] || [ -z "$1" ]; then
        echo "--tag requires a value" >&2
        exit 2
      fi
      tag="$1"
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    --*)
      echo "unknown option: $1" >&2
      usage >&2
      exit 2
      ;;
    *)
      if ! is_service "$1"; then
        echo "unknown service: $1" >&2
        usage >&2
        exit 2
      fi
      requested_services="$requested_services $1"
      ;;
  esac
  shift
done

services="${requested_services# }"
if [ -z "$services" ]; then
  services="$all_services"
fi

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

for name in $services; do
  case "$name" in
    auth-api) package_path="./app/auth/api" ;;
    product-rpc) package_path="./app/product/rpc" ;;
    order-rpc) package_path="./app/order/rpc" ;;
    inventory-kitex) package_path="./app/inventory/kitex" ;;
    entry-api) package_path="./app/entry/api" ;;
    hertz-gateway) package_path="./app/gateway/hertz" ;;
  esac

  svc_context="$context_root/$name"
  case "$svc_context" in
    "$context_root"/*) ;;
    *) echo "unsafe service context: $svc_context" >&2; exit 1 ;;
  esac
  rm -rf "$svc_context"
  mkdir -p "$svc_context/web"

  if [ "$name" = "hertz-gateway" ]; then
    cp -R "$repo_root/app/entry/api/internal/handler/web/." "$svc_context/web/"
  fi

  echo "[GO BUILD] $name"
  go build -trimpath -tags timetzdata -o "$svc_context/app" "$package_path"

  echo "[DOCKER BUILD] flash-mall/$name:$tag"
  if docker buildx version >/dev/null 2>&1; then
    docker buildx build --load -f "$dockerfile" -t "flash-mall/$name:$tag" "$svc_context"
  else
    docker build -f "$dockerfile" -t "flash-mall/$name:$tag" "$svc_context"
  fi
done
