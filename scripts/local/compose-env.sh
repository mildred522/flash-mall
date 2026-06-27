#!/usr/bin/env sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(CDPATH= cd -- "$script_dir/../.." && pwd)
env_file="$repo_root/deploy/.env"

if [ -f "$env_file" ]; then
  set -a
  # shellcheck disable=SC1090
  . "$env_file"
  set +a
fi

set_default_env() {
  name="$1"
  value="$2"
  eval "current=\${$name:-}"
  if [ -z "$current" ]; then
    export "$name=$value"
  fi
}

set_default_env FLASH_MALL_MYSQL_ROOT_PASSWORD "6494kj06"
set_default_env FLASH_MALL_JWT_AUTH_SECRET "flash-mall-local-jwt-secret"
set_default_env FLASH_MALL_PAYMENT_CALLBACK_SECRET "flash-mall-local-payment-secret"
set_default_env FLASH_MALL_DEMO_PASSWORD "flashmall123"
set_default_env FLASH_MALL_RABBITMQ_USER "flashmall"
set_default_env FLASH_MALL_RABBITMQ_PASSWORD "flashmall-local"
set_default_env FLASH_MALL_IMAGE_TAG "dev"
