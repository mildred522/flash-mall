#!/usr/bin/env bash

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
LOG_DIR="${REPO_ROOT}/.runtime/ci-smoke"
mkdir -p "${LOG_DIR}"

PIDS=()

cleanup() {
  local exit_code=$?

  if [[ "${exit_code}" -ne 0 ]]; then
    for log_file in "${LOG_DIR}"/*.log; do
      if [[ -f "${log_file}" ]]; then
        echo "========== ${log_file} =========="
        tail -n 200 "${log_file}" || true
      fi
    done
  fi

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
    if docker exec mysql mysql -N -uroot -p6494kj06 -e "SELECT 1" >/dev/null 2>&1; then
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
  local config="${3:-}"
  local log_file="${LOG_DIR}/${name}.log"

  (
    cd "${REPO_ROOT}"
    if [[ -n "${config}" ]]; then
      exec go run "${entry}" -f "${config}"
    fi
    exec go run "${entry}"
  ) >"${log_file}" 2>&1 &

  local pid=$!
  PIDS+=("${pid}")
  echo "[start] ${name} pid=${pid}"
}

cd "${REPO_ROOT}"

export FLASH_MALL_AUTH_DATASOURCE="root:6494kj06@tcp(127.0.0.1:3307)/mall_auth?charset=utf8mb4&parseTime=true&loc=Asia%2FShanghai"
export FLASH_MALL_PRODUCT_DATASOURCE="root:6494kj06@tcp(127.0.0.1:3307)/mall_product?charset=utf8mb4&parseTime=true&loc=Asia%2FShanghai"
export FLASH_MALL_ORDER_DATASOURCE="root:6494kj06@tcp(127.0.0.1:3307)/mall_order?charset=utf8mb4&parseTime=true&loc=Asia%2FShanghai"
export FLASH_MALL_RABBITMQ_URL="amqp://flashmall:flashmall-local@127.0.0.1:5672/"
export FLASH_MALL_JWT_AUTH_SECRET="flash-mall-ci-jwt-secret"
export FLASH_MALL_PAYMENT_CALLBACK_SECRET="flash-mall-ci-payment-secret"
export FLASH_MALL_DEMO_PASSWORD="flashmall123"
export FLASH_MALL_AUTH_SERVICE_BASE_URL="http://127.0.0.1:8890"
export FLASH_MALL_INVENTORY_KITEX_ENDPOINT="127.0.0.1:8093"
export INVENTORY_REDIS_HOST="127.0.0.1:6379"
export INVENTORY_DATASOURCE="${FLASH_MALL_PRODUCT_DATASOURCE}"
export INVENTORY_STOCK_SHARD_COUNT="4"
export INVENTORY_FINAL_DEDUCT_ENABLED="false"

smoke_gateway="${FLASH_MALL_SMOKE_GATEWAY:-hertz}"
case "${smoke_gateway}" in
  hertz|entry) ;;
  *)
    echo "[error] FLASH_MALL_SMOKE_GATEWAY must be hertz or entry" >&2
    exit 2
    ;;
esac

if [[ "${CI:-}" == "true" ]]; then
  for image in \
    yedf/dtm \
    bitnamilegacy/etcd:3.5 \
    mysql:5.7 \
    redis:latest \
    rabbitmq:3.12-management; do
    docker pull "${image}"
  done
fi

docker compose -f deploy/docker-compose.yml up -d etcd mysql redis dtm rabbitmq

wait_for_port "etcd" "127.0.0.1" "2379" 90
wait_for_port "mysql" "127.0.0.1" "3307" 90
wait_for_port "redis" "127.0.0.1" "6379" 90
wait_for_port "dtm-grpc" "127.0.0.1" "36790" 90
wait_for_port "rabbitmq" "127.0.0.1" "5672" 90
wait_for_mysql 600

docker exec -i mysql mysql --force --default-character-set=utf8mb4 -uroot -p6494kj06 < scripts/k8s/init-db.sql

go run ./app/entry/api/scripts/seed/seed_stock.go -product 100 -stock 10000 -shards 4

start_go_service "inventory-kitex" "./app/inventory/kitex"
wait_for_port "inventory-kitex" "127.0.0.1" "8093" 90

start_go_service "product-rpc" "./app/product/rpc/product.go" "./app/product/rpc/etc/product.yaml"
wait_for_port "product-rpc" "127.0.0.1" "8080" 90

order_rpc_smoke_config="${LOG_DIR}/order-rpc-smoke.yaml"
sed \
  -e "s|InventoryKitexEndpoint: ''|InventoryKitexEndpoint: '127.0.0.1:8093'|" \
  -e 's/RequireInventoryReserve: false/RequireInventoryReserve: true/' \
  ./app/order/rpc/etc/order.yaml > "${order_rpc_smoke_config}"

start_go_service "order-rpc" "./app/order/rpc/order.go" "${order_rpc_smoke_config}"
wait_for_port "order-rpc" "127.0.0.1" "8090" 90

start_go_service "auth-api" "./app/auth/api/auth.go" "./app/auth/api/etc/auth-api.yaml"
wait_for_port "auth-api" "127.0.0.1" "8890" 90

if [[ "${smoke_gateway}" == "hertz" ]]; then
  gateway_name="hertz-gateway"
  gateway_base_url="http://127.0.0.1:8889"
  hertz_smoke_config="${LOG_DIR}/hertz-gateway-smoke.yaml"
  sed \
    -e 's/OrderRpcTarget: 127.0.0.1:8090/OrderRpcTarget: host.docker.internal:8090/' \
    ./app/gateway/hertz/etc/gateway.yaml > "${hertz_smoke_config}"
  start_go_service "${gateway_name}" "./app/gateway/hertz/gateway.go" "${hertz_smoke_config}"
else
  gateway_name="entry-api"
  gateway_base_url="http://127.0.0.1:8888"
  entry_api_smoke_config="${LOG_DIR}/entry-api-smoke.yaml"
  sed \
    -e 's/ProductRpcTarget: 127.0.0.1:8080/ProductRpcTarget: host.docker.internal:8080/' \
    -e 's/OrderRpcTarget: 127.0.0.1:8090/OrderRpcTarget: host.docker.internal:8090/' \
    ./app/entry/api/etc/entry-api.yaml > "${entry_api_smoke_config}"
  start_go_service "${gateway_name}" "./app/entry/api/entry.go" "${entry_api_smoke_config}"
fi

wait_for_http "${gateway_name}" "${gateway_base_url}/api/system/health" 90

health_json="$(curl -fsS "${gateway_base_url}/api/system/health")"
python3 - "${health_json}" "${smoke_gateway}" <<'PY'
import json
import sys

payload = json.loads(sys.argv[1])
gateway = sys.argv[2]
if gateway == "hertz":
    data = payload.get("data") or {}
    if payload.get("code") != "OK" or data.get("status") != "ok":
        raise SystemExit("hertz gateway health failed")
elif not payload.get("overall"):
    raise SystemExit("entry-api system health overall=false")
PY

phone="$(printf "139%08d" $(( $(date +%s) % 100000000 )))"
code_json="$(curl -fsS -X POST "${gateway_base_url}/api/auth/code/send" \
  -H 'Content-Type: application/json' \
  -d "{\"phone\":\"${phone}\",\"scene\":\"register\"}")"

code="$(python3 - "${code_json}" <<'PY'
import json
import sys

payload = json.loads(sys.argv[1])
code = payload.get("debug_code", "")
if not code:
    raise SystemExit("missing debug_code")
print(code)
PY
)"

login_json="$(curl -fsS -X POST "${gateway_base_url}/api/auth/register" \
  -H 'Content-Type: application/json' \
  -d "{\"phone\":\"${phone}\",\"password\":\"flashmall123\",\"code\":\"${code}\",\"display_name\":\"CI Smoke User\"}")"

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

user_id="$(python3 - "${login_json}" <<'PY'
import json
import sys

payload = json.loads(sys.argv[1])
user_id = payload.get("user_id", 0)
if not user_id:
    raise SystemExit("missing user_id")
print(user_id)
PY
)"

request_id="ci-$(date +%s)-$$"
create_json="$(curl -fsS -X POST "${gateway_base_url}/api/order/create" \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer ${token}" \
  -d "{\"request_id\":\"${request_id}\",\"user_id\":${user_id},\"product_id\":100,\"amount\":1}")"

order_id="$(python3 - "${create_json}" <<'PY'
import json
import sys

payload = json.loads(sys.argv[1])
data = payload.get("data") or payload
order_id = data.get("order_id", "")
if not order_id:
    raise SystemExit("missing order_id")
print(order_id)
PY
)"

wait_for_order_in_db "${order_id}" 30

echo "[ok] ${gateway_name} smoke test passed with order_id=${order_id}"
