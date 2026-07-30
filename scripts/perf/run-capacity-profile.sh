#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
control="$repo_root/scripts/local/flash-mall-control.sh"
compose_file="$repo_root/deploy/docker-compose.yml"
base_url="http://127.0.0.1:8889"
read_rps="100,300,600"
order_rps="2,5,10"
stage_duration=15
warmup_duration=2
payment_requests=8
idempotency_requests=40
allow_mutation=0
confirm_reset=0
output_dir=""
restore_needed=0
rabbit_paused=0
upload_volume_before=""

usage() {
  cat <<'EOF'
Usage: scripts/perf/run-capacity-profile.sh --allow-mutation --confirm-reset [options]

Options:
  --base-url URL              Hertz loopback URL (default http://127.0.0.1:8889)
  --read-rps CSV              Read stages (default 100,300,600)
  --order-rps CSV             Order lifecycle stages (default 2,5,10)
  --stage-duration SECONDS    Seconds per ramp stage (default 15)
  --warmup SECONDS            Warmup seconds per stage (default 2)
  --payment-requests COUNT    Local sandbox payment samples (default 8)
  --idempotency-requests N    Concurrent replay samples (default 40)
  --output-dir PATH           Result directory under .runtime by default

The full profile resets the fixed interview fixture before and after the run.
It refuses remote targets and preserves the flash-mall-uploads volume.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --base-url) base_url="${2:-}"; shift ;;
    --read-rps) read_rps="${2:-}"; shift ;;
    --order-rps) order_rps="${2:-}"; shift ;;
    --stage-duration) stage_duration="${2:-}"; shift ;;
    --warmup) warmup_duration="${2:-}"; shift ;;
    --payment-requests) payment_requests="${2:-}"; shift ;;
    --idempotency-requests) idempotency_requests="${2:-}"; shift ;;
    --output-dir) output_dir="${2:-}"; shift ;;
    --allow-mutation) allow_mutation=1 ;;
    --confirm-reset) confirm_reset=1 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
  shift
done

case "$base_url" in
  http://127.0.0.1:*|http://localhost:*|http://[[]::1[]]:*) ;;
  *) echo "capacity writes only allow a local loopback Hertz URL" >&2; exit 2 ;;
esac
[[ "$allow_mutation" -eq 1 ]] || { echo "--allow-mutation is required" >&2; exit 2; }
[[ "$confirm_reset" -eq 1 ]] || { echo "--confirm-reset is required" >&2; exit 2; }
[[ "$stage_duration" =~ ^[1-9][0-9]*$ ]] || { echo "invalid stage duration" >&2; exit 2; }
[[ "$warmup_duration" =~ ^[0-9]+$ ]] || { echo "invalid warmup duration" >&2; exit 2; }

timestamp="$(date +%Y%m%d-%H%M%S)"
output_dir="${output_dir:-$repo_root/.runtime/capacity/$timestamp}"
mkdir -p "$output_dir"
tool="$output_dir/capacitybench"
violations_file="$output_dir/violations.txt"
: > "$violations_file"

mysql_scalar() {
  docker exec mysql mysql --default-character-set=utf8mb4 -N -uroot \
    -p"${FLASH_MALL_MYSQL_ROOT_PASSWORD:-6494kj06}" -e "$1" 2>/dev/null
}

restore_demo() {
  exit_code=$?
  trap - EXIT INT TERM
  if [[ "$rabbit_paused" -eq 1 ]]; then
    docker unpause rabbitmq >/dev/null 2>&1 || true
    rabbit_paused=0
  fi
  if [[ "$restore_needed" -eq 1 ]]; then
    "$control" reset-demo --confirm-reset --profile interview >/dev/null || exit_code=1
    restore_needed=0
  fi
  current_upload="$(docker volume inspect -f '{{.Name}}' flash-mall-uploads 2>/dev/null || true)"
  if [[ -n "$upload_volume_before" && "$current_upload" != "$upload_volume_before" ]]; then
    echo "flash-mall-uploads volume identity changed" >&2
    exit_code=1
  fi
  exit "$exit_code"
}
trap restore_demo EXIT INT TERM

wait_outbox_drained() {
  local deadline=$((SECONDS + 90))
  while (( SECONDS < deadline )); do
    pending="$(mysql_scalar "
      SELECT COUNT(*) FROM mall_order.order_outbox
      WHERE aggregate_id LIKE 'capacity-%' AND status IN (0,2,3);
    ")"
    [[ "$pending" == "0" ]] && return 0
    sleep 1
  done
  return 1
}

run_stage() {
  local scenario="$1"
  local rps="$2"
  local suffix="$3"
  "$tool" -base-url "$base_url" -scenario "$scenario" -rps "$rps" \
    -duration "${stage_duration}s" -warmup "${warmup_duration}s" \
    -concurrency "$((rps > 40 ? 40 : rps + 4))" -allow-mutation \
    -out "$output_dir/stage-${scenario}-${suffix}.json" >/dev/null
}

record_violation() {
  printf '%s\n' "$1" >> "$violations_file"
}

verify_invariants() {
  [[ "$(mysql_scalar "
    SELECT COUNT(*) FROM (
      SELECT request_id FROM mall_order.orders
      WHERE request_id LIKE 'capacity-%'
      GROUP BY request_id HAVING COUNT(*) > 1
    ) duplicated;
  ")" == "0" ]] || record_violation duplicate_order

  [[ "$(mysql_scalar "
    SELECT COUNT(*) FROM (
      SELECT order_id FROM mall_order.payment_callback_event
      WHERE order_id LIKE 'capacity-%'
      GROUP BY order_id HAVING COUNT(*) > 1
    ) duplicated;
  ")" == "0" ]] || record_violation duplicate_payment_callback

  [[ "$(mysql_scalar "
    SELECT COUNT(*) FROM mall_product.product_stock_bucket WHERE stock < 0;
  ")" == "0" ]] || record_violation negative_stock

  [[ "$(mysql_scalar "
    SELECT COUNT(*) FROM mall_product.inventory_reservation
    WHERE order_id LIKE 'capacity-%' AND status = 'RESERVED';
  ")" == "0" ]] || record_violation reserved_reservations

  [[ "$(mysql_scalar "
    SELECT COUNT(*) FROM mall_order.order_outbox
    WHERE aggregate_id LIKE 'capacity-%' AND status IN (0,2,3);
  ")" == "0" ]] || record_violation outbox_not_drained

  [[ "$(mysql_scalar "
    SELECT COUNT(*) FROM mall_order.payment_order
    WHERE order_id LIKE 'capacity-%' AND status = 1 AND inventory_finalize_status <> 1;
  ")" == "0" ]] || record_violation payment_inventory_not_finalized

  while IFS=$'\t' read -r product bucket expected; do
    [[ -n "$product" ]] || continue
    actual="$(docker exec redis redis-cli --raw GET "stock:${product}:${bucket}" 2>/dev/null || true)"
    [[ "$actual" == "$expected" ]] || record_violation "redis_mysql_stock_${product}_${bucket}"
  done < <(mysql_scalar "
    SELECT product_id,bucket_idx,stock
    FROM mall_product.product_stock_bucket
    WHERE product_id IN (100,101,102,103,104,201,202,211,212)
    ORDER BY product_id,bucket_idx;
  ")

  VIOLATIONS_FILE="$violations_file" OUTPUT_FILE="$output_dir/invariants.json" node <<'NODE'
const fs = require('node:fs');
const values = fs.readFileSync(process.env.VIOLATIONS_FILE, 'utf8')
  .split(/\r?\n/).map((value) => value.trim()).filter(Boolean);
fs.writeFileSync(process.env.OUTPUT_FILE, `${JSON.stringify({
  passed: values.length === 0,
  violations: [...new Set(values)],
}, null, 2)}\n`);
NODE
}

echo "[capacity] verifying and resetting the fixed interview fixture"
if ! "$control" verify-demo --profile interview >/dev/null; then
  echo "[capacity] existing demo fixture is inconsistent; the confirmed reset will repair it" >&2
fi
upload_volume_before="$(docker volume inspect -f '{{.Name}}' flash-mall-uploads)"
"$control" reset-demo --confirm-reset --profile interview >/dev/null
restore_needed=1
"$control" verify-demo --profile interview >/dev/null

echo "[capacity] rebuilding Hertz and enabling observability"
"$repo_root/scripts/local/rebuild-compose-service.sh" hertz-gateway >/dev/null
docker compose -f "$compose_file" --profile observability up -d --force-recreate prometheus grafana >/dev/null
"/home/mildred/.local/go/bin/go" build -o "$tool" "$repo_root/tools/capacitybench"

commit="$(git -C "$repo_root" rev-parse HEAD)"
environment="Ubuntu WSL Docker Engine on a shared Windows development host"
COMMIT="$commit" ENVIRONMENT="$environment" OUTPUT_FILE="$output_dir/metadata.json" node <<'NODE'
const fs = require('node:fs');
const os = require('node:os');
fs.writeFileSync(process.env.OUTPUT_FILE, `${JSON.stringify({
  commit: process.env.COMMIT,
  environment: process.env.ENVIRONMENT,
  cpu_count: os.cpus().length,
  total_memory_bytes: os.totalmem(),
  demo_fixture: '20260730_demo_fixture_v1',
  recorded_at: new Date().toISOString(),
}, null, 2)}\n`);
NODE
docker stats --no-stream --format '{{json .}}' > "$output_dir/resources-before.jsonl"

echo "[capacity] running public read ramp"
IFS=',' read -ra read_stages <<< "$read_rps"
for rps in "${read_stages[@]}"; do
  run_stage read "$rps" "rps-${rps}"
done

echo "[capacity] running authenticated order lifecycle ramp"
IFS=',' read -ra order_stages <<< "$order_rps"
for rps in "${order_stages[@]}"; do
  run_stage order-cycle "$rps" "rps-${rps}"
done

echo "[capacity] running payment and idempotency probes"
"$tool" -base-url "$base_url" -scenario payment-cycle -requests "$payment_requests" \
  -rps 2 -warmup "${warmup_duration}s" -concurrency 6 -allow-mutation \
  -out "$output_dir/stage-payment-cycle-normal.json" >/dev/null
"$tool" -base-url "$base_url" -scenario idempotency -requests "$idempotency_requests" \
  -rps 20 -warmup 0s -concurrency "$idempotency_requests" -allow-mutation \
  -out "$output_dir/stage-idempotency-replay.json" >/dev/null

token="$(curl --noproxy '*' -fsS -X POST "$base_url/api/auth/login" \
  -H 'Content-Type: application/json' \
  -d '{"phone":"13800000001","password":"flashmall123","device_type":"capacity-cleanup"}' \
  | node -e 'let s="";process.stdin.on("data",d=>s+=d).on("end",()=>process.stdout.write(JSON.parse(s).access_token||""))')"
idempotency_order="$(mysql_scalar "
  SELECT id FROM mall_order.orders
  WHERE request_id LIKE 'capacity-%-idempotency'
  ORDER BY create_time DESC LIMIT 1;
")"
curl --noproxy '*' -fsS -X POST "$base_url/api/order/cancel" \
  -H 'Content-Type: application/json' -H "Authorization: Bearer $token" \
  -d "{\"order_id\":\"$idempotency_order\",\"reason\":\"capacity idempotency cleanup\"}" >/dev/null

echo "[capacity] verifying Outbox backlog and recovery"
docker pause rabbitmq >/dev/null
rabbit_paused=1
"$tool" -base-url "$base_url" -scenario payment-cycle -requests 4 \
  -rps 2 -warmup 0s -concurrency 4 -allow-mutation \
  -out "$output_dir/stage-payment-cycle-outbox-recovery.json" >/dev/null
docker unpause rabbitmq >/dev/null
rabbit_paused=0
wait_outbox_drained || record_violation outbox_recovery_timeout

docker stats --no-stream --format '{{json .}}' > "$output_dir/resources-after.jsonl"
verify_invariants
node "$repo_root/scripts/perf/summarize-capacity.mjs" "$output_dir" \
  "$output_dir/summary.json" "$output_dir/report.md"

echo "[capacity] restoring fixed interview fixture"
"$control" reset-demo --confirm-reset --profile interview >/dev/null
restore_needed=0
"$control" verify-demo --profile interview >/dev/null
[[ "$(docker volume inspect -f '{{.Name}}' flash-mall-uploads)" == "$upload_volume_before" ]]

echo "[capacity] report: $output_dir/report.md"
cat "$output_dir/report.md"
if ! node -e '
  const { readFileSync } = require("node:fs");
  const summary = JSON.parse(readFileSync(process.argv[1], "utf8"));
  process.exit(summary.overall_passed ? 0 : 1);
' "$output_dir/summary.json"; then
  echo "[capacity] one or more SLO or correctness gates failed" >&2
  exit 1
fi
