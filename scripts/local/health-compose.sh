#!/usr/bin/env sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(CDPATH= cd -- "$script_dir/../.." && pwd)
deploy_dir="$repo_root/deploy"
compose_file="docker-compose.yml"
health_url="${FLASH_MALL_HEALTH_URL:-http://127.0.0.1:8888/api/system/health}"
wait_seconds=0
logs_on_failure=0

while [ "$#" -gt 0 ]; do
  case "$1" in
    --wait)
      shift
      wait_seconds="${1:-}"
      if [ -z "$wait_seconds" ]; then
        echo "--wait requires seconds" >&2
        exit 2
      fi
      ;;
    --logs-on-failure) logs_on_failure=1 ;;
    -h|--help)
      cat <<'EOF'
Usage: scripts/local/health-compose.sh [options]

Options:
  --wait SECONDS       Poll entry-api health until healthy or timeout.
  --logs-on-failure    Print key container logs when health fails.
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

print_compose_status() {
  echo "[STATUS] docker compose ps"
  (cd "$deploy_dir" && docker compose -f "$compose_file" ps) || true
}

print_key_logs() {
  echo "[LOGS] key containers"
  for container in flash-mall-mysql-init flash-mall-redis-init auth-api product-rpc order-rpc entry-api dtm mysql redis rabbitmq etcd; do
    echo "--- $container ---"
    docker logs --tail 80 "$container" 2>&1 || true
  done
}

check_redis_stock() {
  values=$(docker exec redis redis-cli MGET stock:100:0 stock:100:1 stock:100:2 stock:100:3 2>/dev/null || true)
  count=$(printf '%s\n' "$values" | awk 'NF > 0 {count++} END {print count + 0}')
  if [ "$count" -lt 4 ]; then
    echo "[FAIL] redis stock keys missing for product 100"
    return 1
  fi
  echo "[OK] redis stock keys ready: $(printf '%s' "$values" | tr '\n' ' ')"
}

check_entry_health_once() {
  body=$(curl --noproxy "*" -fsS -m 5 "$health_url" 2>/tmp/flash-mall-health.err || true)
  if printf '%s' "$body" | grep -q '"overall":true'; then
    echo "[OK] entry-api health overall=true"
    printf '%s\n' "$body"
    return 0
  fi

  echo "[WAIT] entry-api health not ready"
  if [ -s /tmp/flash-mall-health.err ]; then
    cat /tmp/flash-mall-health.err
  elif [ -n "$body" ]; then
    printf '%s\n' "$body"
  fi
  return 1
}

print_compose_status

deadline=$(( $(date +%s) + wait_seconds ))
while :; do
  if check_entry_health_once && check_redis_stock; then
    exit 0
  fi

  if [ "$wait_seconds" -le 0 ] || [ "$(date +%s)" -ge "$deadline" ]; then
    echo "[FAIL] compose stack is not healthy"
    print_compose_status
    if [ "$logs_on_failure" -eq 1 ]; then
      print_key_logs
    fi
    exit 1
  fi

  sleep 2
done
