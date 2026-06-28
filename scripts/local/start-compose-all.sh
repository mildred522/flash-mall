#!/usr/bin/env sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(CDPATH= cd -- "$script_dir/../.." && pwd)
deploy_dir="$repo_root/deploy"
compose_file="docker-compose.yml"

no_build=0
compose_build=0
foreground=0
pull_deps=0
wait_health=1
health_timeout=120

while [ "$#" -gt 0 ]; do
  case "$1" in
    --no-build) no_build=1 ;;
    --compose-build) compose_build=1 ;;
    --foreground) foreground=1 ;;
    --pull-deps) pull_deps=1 ;;
    --no-wait) wait_health=0 ;;
    --wait-timeout)
      shift
      health_timeout="${1:-}"
      if [ -z "$health_timeout" ]; then
        echo "--wait-timeout requires seconds" >&2
        exit 2
      fi
      ;;
    -h|--help)
      cat <<'EOF'
Usage: scripts/local/start-compose-all.sh [options]

Options:
  --no-build       Start existing images without rebuilding Go services.
  --compose-build  Build service images through docker compose build.
  --foreground     Run docker compose up in the foreground.
  --pull-deps      Pull dependency images before startup.
  --no-wait        Do not wait for entry-api health after detached startup.
  --wait-timeout N Wait up to N seconds for health after detached startup.
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

. "$script_dir/compose-env.sh"
command -v docker >/dev/null 2>&1 || {
  echo "docker not found in PATH" >&2
  exit 1
}

if [ "$pull_deps" -eq 1 ]; then
  for image in \
    yedf/dtm \
    bitnamilegacy/etcd:3.5 \
    mysql:5.7 \
    redis:latest \
    rabbitmq:3.12-management \
    jaegertracing/all-in-one:1.57
  do
    echo "[PULL] $image"
    if ! timeout 180 docker pull "$image"; then
      echo "[ERROR] docker pull failed or timed out: $image" >&2
      exit 1
    fi
  done
fi

if [ "$no_build" -eq 0 ]; then
  if [ "$compose_build" -eq 1 ]; then
    docker compose -f "$compose_file" build auth-api product-rpc order-rpc entry-api
  else
    "$script_dir/build-compose-images.sh" "$FLASH_MALL_IMAGE_TAG"
  fi
fi

args="-f $compose_file up --no-build"
if [ "$foreground" -eq 0 ]; then
  args="$args -d"
fi

cd "$deploy_dir"

echo "[COMPOSE] docker compose $args"
# shellcheck disable=SC2086
if ! docker compose $args; then
  echo "[ERROR] docker compose startup failed" >&2
  "$script_dir/health-compose.sh" --logs-on-failure || true
  exit 1
fi

if [ "$foreground" -eq 0 ] && [ "$wait_health" -eq 1 ]; then
  "$script_dir/health-compose.sh" --wait "$health_timeout" --logs-on-failure
fi

cat <<'EOF'

entry-api: http://127.0.0.1:8888
auth-api:  http://127.0.0.1:8890
rabbitmq:  http://127.0.0.1:15672
EOF
