#!/usr/bin/env bash
set -euo pipefail

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(CDPATH= cd -- "$script_dir/../.." && pwd)
baseline_root="${FLASH_MALL_BASELINE_ROOT:-/home/mildred/code/flash-mall-baseline-main}"
runs=5
requests=3000
concurrency=30
warmup=300
cooldown=3
baseline_container="entry-api-perf-baseline"
baseline_image="flash-mall/entry-api:perf-main"
compose_file="$repo_root/deploy/docker-compose.yml"
mysql_password="${FLASH_MALL_MYSQL_ROOT_PASSWORD:-6494kj06}"
rabbit_user="${FLASH_MALL_RABBITMQ_USER:-flashmall}"
rabbit_password="${FLASH_MALL_RABBITMQ_PASSWORD:-flashmall-local}"
output_root=""
stack_was_running=0

usage() {
  cat <<'EOF'
Usage: scripts/perf/compare-entry-hertz.sh [options]

Options:
  --runs N          Paired sample count (default 5)
  --requests N      Requests per measured sample (default 3000)
  --concurrency N   Concurrent workers (default 30)
  --warmup N        Warmup requests per sample (default 300)
  --output DIR      Runtime output directory
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --runs) shift; runs="${1:-}" ;;
    --requests) shift; requests="${1:-}" ;;
    --concurrency) shift; concurrency="${1:-}" ;;
    --warmup) shift; warmup="${1:-}" ;;
    --output) shift; output_root="${1:-}" ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
  shift
done

for value in "$runs" "$requests" "$concurrency"; do
  case "$value" in ''|*[!0-9]*) echo "numeric options must be positive integers" >&2; exit 2 ;; esac
  [ "$value" -gt 0 ] || { echo "numeric options must be positive integers" >&2; exit 2; }
done
case "$warmup" in ''|*[!0-9]*) echo "warmup must be a non-negative integer" >&2; exit 2 ;; esac

[ -d "$baseline_root/.git" ] || [ -f "$baseline_root/.git" ] || {
  echo "baseline worktree not found: $baseline_root" >&2
  exit 1
}
git -C "$baseline_root" diff --quiet && git -C "$baseline_root" diff --cached --quiet || {
  echo "baseline worktree must be clean" >&2
  exit 1
}
baseline_tree=$(git -C "$repo_root" rev-parse 'origin/main^{tree}')
worktree_tree=$(git -C "$baseline_root" rev-parse 'HEAD^{tree}')
[ "$baseline_tree" = "$worktree_tree" ] || {
  echo "baseline worktree tree does not match origin/main" >&2
  exit 1
}

timestamp=$(date +%Y%m%d-%H%M%S)
output_root="${output_root:-$repo_root/.runtime/perf-comparison/$timestamp}"
mkdir -p "$output_root"
tool_path="$output_root/httpbench"
entry_url="http://127.0.0.1:8888/api/shop/catalog"
hertz_url="http://127.0.0.1:8889/api/shop/catalog"

if [ "$(docker inspect -f '{{.State.Running}}' hertz-gateway 2>/dev/null || true)" = "true" ]; then
  stack_was_running=1
fi

cleanup() {
  docker rm -f "$baseline_container" >/dev/null 2>&1 || true
  if [ "$stack_was_running" -eq 0 ]; then
    docker compose -f "$compose_file" stop hertz-gateway auth-api product-rpc order-rpc inventory-kitex dtm rabbitmq redis mysql etcd >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

wait_http() {
  url="$1"
  for _ in $(seq 1 90); do
    if curl --noproxy "*" -fsS -m 3 "$url" >/dev/null 2>&1; then return 0; fi
    sleep 1
  done
  echo "timed out waiting for $url" >&2
  return 1
}

validate_catalog() {
  entry_file="$output_root/entry-catalog.json"
  hertz_file="$output_root/hertz-catalog.json"
  curl --noproxy "*" -fsS "$entry_url" > "$entry_file"
  curl --noproxy "*" -fsS "$hertz_url" > "$hertz_file"
  entry_items=$(grep -o '"product_id"' "$entry_file" | wc -l)
  hertz_items=$(grep -o '"product_id"' "$hertz_file" | wc -l)
  [ "$entry_items" -gt 0 ] && [ "$hertz_items" -gt 0 ] || { echo "catalog responses must contain products" >&2; return 1; }
  printf 'entry_items=%s entry_bytes=%s hertz_items=%s hertz_bytes=%s\n' \
    "$entry_items" "$(wc -c < "$entry_file")" "$hertz_items" "$(wc -c < "$hertz_file")"
}

echo "[BUILD] benchmark tool"
"${GO:-/home/mildred/.local/go/bin/go}" build -o "$tool_path" "$repo_root/tools/httpbench"

echo "[BUILD] current Hertz gateway"
docker compose -f "$compose_file" build hertz-gateway
docker compose -f "$compose_file" up -d hertz-gateway
wait_http "http://127.0.0.1:8889/api/system/health"

echo "[BUILD] origin/main Entry API"
docker build -f "$repo_root/build/docker/entry-api-baseline-perf.Dockerfile" -t "$baseline_image" "$baseline_root"
docker rm -f "$baseline_container" >/dev/null 2>&1 || true
docker run -d --name "$baseline_container" --network deploy_default \
  -p 127.0.0.1:8888:8888 \
  -e "FLASH_MALL_ORDER_DATASOURCE=root:${mysql_password}@tcp(mysql:3306)/mall_order?charset=utf8mb4&parseTime=true&loc=Asia%2FShanghai" \
  -e "FLASH_MALL_RABBITMQ_URL=amqp://${rabbit_user}:${rabbit_password}@rabbitmq:5672/" \
  -e 'FLASH_MALL_JWT_AUTH_SECRET=flash-mall-local-jwt-secret' \
  -e 'FLASH_MALL_PAYMENT_CALLBACK_SECRET=flash-mall-local-payment-secret' \
  -v "$baseline_root/deploy/config/entry-api.yaml:/etc/flash-mall/entry-api.yaml:ro" \
  "$baseline_image" -f /etc/flash-mall/entry-api.yaml >/dev/null
wait_http "$entry_url"

echo "[CONTRACT] public catalog responses"
catalog_contract=$(validate_catalog)
echo "$catalog_contract"

baseline_commit=$(git -C "$repo_root" rev-parse origin/main)
candidate_commit=$(git -C "$repo_root" rev-parse HEAD)
metadata_path="$output_root/metadata.json"
printf '{\n  "baseline_commit": "%s",\n  "candidate_commit": "%s",\n  "requests": %s,\n  "concurrency": %s,\n  "warmup": %s,\n  "catalog_contract": "%s"\n}\n' \
  "$baseline_commit" "$candidate_commit" "$requests" "$concurrency" "$warmup" "$catalog_contract" > "$metadata_path"

run_sample() {
  gateway="$1"
  run="$2"
  case "$gateway" in
    entry) url="$entry_url" ;;
    hertz) url="$hertz_url" ;;
    *) return 2 ;;
  esac
  name="$gateway-run-$run"
  echo "[RUN] $name"
  "$tool_path" -name "$name" -url "$url" -n "$requests" -c "$concurrency" \
    -warmup "$warmup" -timeout 5s -out "$output_root/run-$run-$gateway.json" >/dev/null
}

for run in $(seq 1 "$runs"); do
  if [ $((run % 2)) -eq 1 ]; then
    order="entry hertz"
  else
    order="hertz entry"
  fi
  for gateway in $order; do
    run_sample "$gateway" "$run"
    sleep "$cooldown"
  done
done

summary_path="$output_root/summary.json"
report_path="$output_root/report.md"
node "$repo_root/scripts/perf/summarize-comparison.mjs" "$output_root" "$summary_path" "$report_path" "$metadata_path"

echo "[PASS] paired performance comparison completed"
echo "[SUMMARY] $summary_path"
echo "[REPORT] $report_path"
cat "$report_path"
