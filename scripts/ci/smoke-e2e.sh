#!/usr/bin/env bash

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
LOG_DIR="${REPO_ROOT}/.runtime/ci-smoke"
mkdir -p "${LOG_DIR}"

PIDS=()

cleanup() {
  local exit_code=$?

  for pid in "${PIDS[@]:-}"; do
    if kill -0 "${pid}" >/dev/null 2>&1; then
      kill "${pid}" >/dev/null 2>&1 || true
      wait "${pid}" >/dev/null 2>&1 || true
    fi
  done

  if command -v docker >/dev/null 2>&1; then
    docker compose -f "${REPO_ROOT}/deploy/docker-compose.yml" logs --no-color > "${LOG_DIR}/docker-compose.log" 2>&1 || true
    docker compose -f "${REPO_ROOT}/deploy/docker-compose.yml" down -v > "${LOG_DIR}/docker-compose-down.log" 2>&1 || true
  fi

  exit "${exit_code}"
}

trap cleanup EXIT

wait_for_port() {
  local name="$1"
  local host="$2"
  local port="$3"
  local timeout="${4:-90}"
  local deadline=$((SECONDS + timeout))

  while (( SECONDS < deadline )); do
    if python3 - "$host" "$port" <<'PY'
import socket
import sys

host = sys.argv[1]
port = int(sys.argv[2])

try:
    with socket.create_connection((host, port), timeout=1):
        pass
except OSError:
    raise SystemExit(1)
PY
    then
      echo "[ok] ${name} ready at ${host}:${port}"
      return 0
    fi
    sleep 1
  done

  echo "[error] ${name} not ready at ${host}:${port}" >&2
  return 1
}

wait_for_http() {
  local name="$1"
  local url="$2"
  local timeout="${3:-90}"
  local deadline=$((SECONDS + timeout))

  while (( SECONDS < deadline )); do
    if curl -fsS "${url}" >/dev/null 2>&1; then
      echo "[ok] ${name} ready at ${url}"
      return 0
    fi
    sleep 1
  done

  echo "[error] ${name} not ready at ${url}" >&2
  return 1
}

wait_for_mysql() {
  local timeout="${1:-120}"
  local deadline=$((SECONDS + timeout))

  while (( SECONDS < deadline )); do
    if docker exec mysql mysqladmin ping -uroot -p6494kj06 --silent >/dev/null 2>&1; then
      echo "[ok] mysql accepting connections"
      return 0
    fi
    sleep 2
  done

  echo "[error] mysql not ready" >&2
  return 1
}

wait_for_order_in_db() {
  local order_id="$1"
  local timeout="${2:-30}"
  local deadline=$((SECONDS + timeout))

  while (( SECONDS < deadline )); do
    local count
    count="$(docker exec mysql mysql -N -uroot -p6494kj06 -e "SELECT COUNT(*) FROM mall_order.orders WHERE id='${order_id}'" 2>/dev/null || true)"
    if [[ "${count}" == "1" ]]; then
      echo "[ok] order persisted: ${order_id}"
      return 0
    fi
    sleep 1
  done

  echo "[error] order not found in db: ${order_id}" >&2
  return 1
}

start_go_service() {
  local name="$1"
  local entry="$2"
  local config="$3"
  local log_file="${LOG_DIR}/${name}.log"

  (
    cd "${REPO_ROOT}"
    exec go run "${entry}" -f "${config}"
  ) >"${log_file}" 2>&1 &

  local pid=$!
  PIDS+=("${pid}")
  echo "[start] ${name} pid=${pid}"
}

cd "${REPO_ROOT}"

docker compose -f deploy/docker-compose.yml up -d etcd mysql redis dtm rabbitmq

wait_for_port "etcd" "127.0.0.1" "2379" 90
wait_for_port "mysql" "127.0.0.1" "3306" 90
wait_for_port "redis" "127.0.0.1" "6379" 90
wait_for_port "dtm-grpc" "127.0.0.1" "36790" 90
wait_for_port "rabbitmq" "127.0.0.1" "5672" 90
wait_for_mysql 120

docker exec -i mysql mysql --force -uroot -p6494kj06 < scripts/k8s/init-db.sql

go run ./app/order/api/scripts/seed/seed_stock.go -product 100 -stock 10000 -shards 4

start_go_service "product-rpc" "./app/product/rpc/product.go" "./app/product/rpc/etc/product.yaml"
wait_for_port "product-rpc" "127.0.0.1" "8080" 90

start_go_service "order-rpc" "./app/order/rpc/order.go" "./app/order/rpc/etc/order.yaml"
wait_for_port "order-rpc" "127.0.0.1" "8090" 90

start_go_service "order-api" "./app/order/api/order.go" "./app/order/api/etc/order-api.yaml"
wait_for_http "order-api" "http://127.0.0.1:8888/api/system/health" 90

health_json="$(curl -fsS http://127.0.0.1:8888/api/system/health)"
python3 - "${health_json}" <<'PY'
import json
import sys

payload = json.loads(sys.argv[1])
if not payload.get("overall"):
    raise SystemExit("system health overall=false")
PY

login_json="$(curl -fsS -X POST http://127.0.0.1:8888/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"user_id":1001,"password":"flashmall123"}')"

token="$(python3 - "${login_json}" <<'PY'
import json
import sys

payload = json.loads(sys.argv[1])
token = payload.get("access_token", "")
if not token:
    raise SystemExit("missing access token")
print(token)
PY
)"

request_id="ci-$(date +%s)-$$"
create_json="$(curl -fsS -X POST http://127.0.0.1:8888/api/order/create \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer ${token}" \
  -d "{\"request_id\":\"${request_id}\",\"user_id\":9999,\"product_id\":100,\"amount\":1}")"

order_id="$(python3 - "${create_json}" <<'PY'
import json
import sys

payload = json.loads(sys.argv[1])
order_id = payload.get("order_id", "")
if not order_id:
    raise SystemExit("missing order_id")
print(order_id)
PY
)"

wait_for_order_in_db "${order_id}" 30

echo "[ok] smoke test passed with order_id=${order_id}"
