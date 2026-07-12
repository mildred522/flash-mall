#!/usr/bin/env sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(CDPATH= cd -- "$script_dir/../.." && pwd)
tag="${FLASH_MALL_IMAGE_TAG:-dev}"
service=""

usage() {
  cat <<'EOF'
Usage: scripts/local/rebuild-compose-service.sh [--tag TAG] SERVICE

Rebuild and recreate one service, then enforce the BuildKit cache budget.
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
      if [ -n "$service" ]; then
        echo "only one service may be rebuilt" >&2
        exit 2
      fi
      service="$1"
      ;;
  esac
  shift
done

if [ -z "$service" ]; then
  usage >&2
  exit 2
fi
if ! is_service "$service"; then
  echo "unknown service: $service" >&2
  usage >&2
  exit 2
fi

"$script_dir/build-compose-images.sh" --tag "$tag" "$service"

cd "$repo_root/deploy"
docker compose up -d --no-deps --force-recreate "$service"

"$script_dir/prune-build-cache.sh"
