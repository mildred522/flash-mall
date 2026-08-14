#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
control="$repo_root/scripts/local/flash-mall-control.sh"
collector="$repo_root/scripts/perf/collect-stage-resources.sh"
compose_file="$repo_root/deploy/docker-compose.yml"
base_url="http://127.0.0.1:8889"
output_dir=""
suite="full"
allow_mutation=0
confirm_reset=0
skip_rebuild=0
restore_needed=0
rabbit_paused=0
sampler_pid=""
profile_pids=()
upload_volume_before=""

usage() {
  cat <<'EOF'
Usage: scripts/perf/run-performance-suite.sh --allow-mutation --confirm-reset [options]

Options:
  --suite full|quick       Full evidence run or short pipeline verification (default: full)
  --base-url URL           Loopback Hertz URL (default: http://127.0.0.1:8889)
  --output-dir PATH        Result directory under .runtime by default
  --skip-rebuild           Reuse current business images
  --allow-mutation         Required: permits local order/payment traffic
  --confirm-reset          Required: permits fixture reset before and after the suite

Full suite:
  baseline: 3 repeated low-load samples
  load: 60-second expected-load stages
  stress: increasing stages until two failed levels or the configured ceiling
  stability: 5-minute mixed read/write soak
  recovery: payment, idempotency, RabbitMQ pause/recovery and business invariants
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --suite) suite="${2:-}"; shift ;;
    --base-url) base_url="${2:-}"; shift ;;
    --output-dir) output_dir="${2:-}"; shift ;;
    --skip-rebuild) skip_rebuild=1 ;;
    --allow-mutation) allow_mutation=1 ;;
    --confirm-reset) confirm_reset=1 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
  shift
done

case "$base_url" in
  http://127.0.0.1:*|http://localhost:*|http://[[]::1[]]:*) ;;
  *) echo "performance writes only allow a local loopback Hertz URL" >&2; exit 2 ;;
esac
case "$suite" in full|quick) ;; *) echo "invalid suite: $suite" >&2; exit 2 ;; esac
[[ "$allow_mutation" -eq 1 ]] || { echo "--allow-mutation is required" >&2; exit 2; }
[[ "$confirm_reset" -eq 1 ]] || { echo "--confirm-reset is required" >&2; exit 2; }

# Keep performance runs isolated from unrelated local development servers while
# preserving port 3000 as the normal project default outside this script.
export FLASH_MALL_GRAFANA_PORT="${FLASH_MALL_PERF_GRAFANA_PORT:-3300}"
export FLASH_MALL_GRAFANA_URL="http://127.0.0.1:${FLASH_MALL_GRAFANA_PORT}"

timestamp="$(date +%Y%m%d-%H%M%S)"
output_dir="${output_dir:-$repo_root/.runtime/performance/$timestamp}"
mkdir -p "$output_dir/stages"
output_dir="$(cd "$output_dir" && pwd)"
tool="$output_dir/capacitybench"
violations_file="$output_dir/violations.txt"
stage_log="$output_dir/stages.tsv"
: > "$violations_file"
: > "$stage_log"

mysql_scalar() {
  docker exec mysql mysql --default-character-set=utf8mb4 -N -uroot \
    -p"${FLASH_MALL_MYSQL_ROOT_PASSWORD:-6494kj06}" -e "$1" 2>/dev/null
}

stop_sampler() {
  if [[ -n "$sampler_pid" ]]; then
    kill -TERM "$sampler_pid" >/dev/null 2>&1 || true
    wait "$sampler_pid" >/dev/null 2>&1 || true
    sampler_pid=""
  fi
}

stop_profiles() {
  local pid
  for pid in "${profile_pids[@]}"; do
    wait "$pid" >/dev/null 2>&1 || true
  done
  profile_pids=()
}

abort_profiles() {
  local pid
  for pid in "${profile_pids[@]}"; do
    kill -TERM "$pid" >/dev/null 2>&1 || true
  done
  stop_profiles
}

start_profile() {
  local service="$1" port="$2" stage="$3" seconds="$4"
  local profile_dir="$output_dir/profiles/$stage"
  mkdir -p "$profile_dir"
  curl --noproxy '*' -fsS "http://127.0.0.1:${port}/debug/pprof/profile?seconds=${seconds}" \
    -o "$profile_dir/${service}-cpu.pb.gz" &
  profile_pids+=("$!")
}

start_profiles() {
  local scenario="$1" stage="$2" seconds="$3"
  profile_pids=()
  start_profile hertz-gateway 6064 "$stage" "$seconds"
  if [[ "$scenario" == "read" ]]; then
    start_profile product-rpc 6062 "$stage" "$seconds"
  else
    start_profile order-rpc 6061 "$stage" "$seconds"
    start_profile inventory-kitex 6063 "$stage" "$seconds"
  fi
}

render_profiles() {
  local profile
  while IFS= read -r -d '' profile; do
    "/home/mildred/.local/go/bin/go" tool pprof -top -nodecount=25 "$profile" \
      > "${profile%.pb.gz}.top.txt" 2>&1 || true
  done < <(find "$output_dir/profiles" -type f -name '*.pb.gz' -print0 2>/dev/null)
}

restore_demo() {
  exit_code=$?
  trap - EXIT INT TERM
  stop_sampler
  abort_profiles
  docker ps -aq --filter "label=flashmall.performance.run=$timestamp" \
    | xargs -r docker rm -f >/dev/null 2>&1 || true
  if [[ "$rabbit_paused" -eq 1 ]]; then
    docker unpause rabbitmq >/dev/null 2>&1 || true
    rabbit_paused=0
  fi
  if [[ "$restore_needed" -eq 1 ]]; then
    "$control" reset-demo --confirm-reset --profile interview --observability >/dev/null || exit_code=1
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

start_sampler() {
  local stage="$1"
  stop_sampler
  "$collector" "$stage" "$output_dir" 5 &
  sampler_pid=$!
}

run_capacity() {
  local container_name="$1"
  shift
  docker run --rm --init --name "flash-mall-loadgen-${container_name}" \
    --label "flashmall.performance.run=$timestamp" --network "$compose_network" \
    -v "$output_dir:/results" alpine:3.20 /results/capacitybench \
    -base-url http://hertz-gateway:8889 -allow-compose-target "$@"
}

run_stage() {
  local kind="$1" scenario="$2" rps="$3" duration="$4" warmup="$5" concurrency="$6" stage="$7"
  local output="$output_dir/stages/$stage.json"
  printf '%s\t%s\t%s\t%s\t%s\tstart\n' "$(date -u +%Y-%m-%dT%H:%M:%S.%3NZ)" "$stage" "$kind" "$scenario" "$rps" >> "$stage_log"
  start_sampler "$stage"
  if [[ "$kind" == "stress" ]]; then
    start_profiles "$scenario" "$stage" "$profile_seconds"
  fi
  set +e
  run_capacity "$stage" -scenario "$scenario" -rps "$rps" \
    -duration "$duration" -warmup "$warmup" -concurrency "$concurrency" \
    -test-kind "$kind" -stage "$stage" -allow-mutation -out "/results/stages/$stage.json" >/dev/null
  status=$?
  set -e
  stop_sampler
  stop_profiles
  printf '%s\t%s\t%s\t%s\t%s\tend:%s\n' "$(date -u +%Y-%m-%dT%H:%M:%S.%3NZ)" "$stage" "$kind" "$scenario" "$rps" "$status" >> "$stage_log"
  return "$status"
}

run_fixed_stage() {
  local kind="$1" scenario="$2" requests="$3" rps="$4" concurrency="$5" stage="$6"
  local output="$output_dir/stages/$stage.json"
  printf '%s\t%s\t%s\t%s\t%s\tstart\n' "$(date -u +%Y-%m-%dT%H:%M:%S.%3NZ)" "$stage" "$kind" "$scenario" "$rps" >> "$stage_log"
  start_sampler "$stage"
  set +e
  run_capacity "$stage" -scenario "$scenario" -requests "$requests" -rps "$rps" \
    -warmup 0s -concurrency "$concurrency" -test-kind "$kind" -stage "$stage" \
    -allow-mutation -out "/results/stages/$stage.json" >/dev/null
  status=$?
  set -e
  stop_sampler
  printf '%s\t%s\t%s\t%s\t%s\tend:%s\n' "$(date -u +%Y-%m-%dT%H:%M:%S.%3NZ)" "$stage" "$kind" "$scenario" "$rps" "$status" >> "$stage_log"
  return "$status"
}

stage_failed() {
  [[ -s "$1" ]] || return 0
  if node "$repo_root/scripts/perf/check-stage-performance.mjs" "$1"; then
    return 1
  fi
  return 0
}

run_stress_ladder() {
  local scenario="$1" duration="$2" warmup="$3" concurrency="$4"
  shift 4
  local failures=0
  for rps in "$@"; do
    stage="stress-${scenario}-rps-${rps}"
    echo "[performance] stress $scenario at $rps RPS"
    run_stage stress "$scenario" "$rps" "$duration" "$warmup" "$concurrency" "$stage" || true
    if stage_failed "$output_dir/stages/$stage.json"; then
      failures=$((failures + 1))
      [[ "$failures" -ge 2 ]] && break
    fi
  done
}

run_stability_mix() {
  local duration="$1" warmup="$2"
  local stage="stability-mixed"
  printf '%s\t%s\tstability\tmixed\tread+order\tstart\n' "$(date -u +%Y-%m-%dT%H:%M:%S.%3NZ)" "$stage" >> "$stage_log"
  start_sampler "$stage"
  run_capacity stability-read -scenario read -rps 500 -duration "$duration" -warmup "$warmup" \
    -concurrency 80 -test-kind stability -stage stability-read -allow-mutation \
    -out "/results/stages/stability-read.json" >/dev/null &
  read_pid=$!
  run_capacity stability-order -scenario order-cycle -rps 5 -duration "$duration" -warmup "$warmup" \
    -concurrency 20 -test-kind stability -stage stability-order -allow-mutation \
    -out "/results/stages/stability-order.json" >/dev/null &
  order_pid=$!
  read_status=0
  order_status=0
  wait "$read_pid" || read_status=$?
  wait "$order_pid" || order_status=$?
  stop_sampler
  status=$((read_status != 0 || order_status != 0))
  printf '%s\t%s\tstability\tmixed\tread+order\tend:%s\n' "$(date -u +%Y-%m-%dT%H:%M:%S.%3NZ)" "$stage" "$status" >> "$stage_log"
  return "$status"
}

record_violation() {
  printf '%s\n' "$1" >> "$violations_file"
}

wait_outbox_drained() {
  local deadline=$((SECONDS + 90))
  while (( SECONDS < deadline )); do
    pending="$(mysql_scalar "SELECT COUNT(*) FROM mall_order.order_outbox WHERE aggregate_id LIKE 'capacity-%' AND status IN (0,2,3);")"
    [[ "$pending" == "0" ]] && return 0
    sleep 1
  done
  return 1
}

verify_invariants() {
  [[ "$(mysql_scalar "
    SELECT COUNT(*) FROM (
      SELECT request_id FROM mall_order.orders WHERE request_id LIKE 'capacity-%'
      GROUP BY request_id HAVING COUNT(*) > 1
    ) duplicated;
  ")" == "0" ]] || record_violation duplicate_order

  [[ "$(mysql_scalar "
    SELECT COUNT(*) FROM (
      SELECT order_id FROM mall_order.payment_callback_event WHERE order_id LIKE 'capacity-%'
      GROUP BY order_id HAVING COUNT(*) > 1
    ) duplicated;
  ")" == "0" ]] || record_violation duplicate_payment_callback

  [[ "$(mysql_scalar "SELECT COUNT(*) FROM mall_product.product_stock_bucket WHERE stock < 0;")" == "0" ]] \
    || record_violation negative_stock
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
    SELECT product_id,bucket_idx,stock FROM mall_product.product_stock_bucket
    WHERE product_id IN (100,101,102,103,104,201,202,211,212)
    ORDER BY product_id,bucket_idx;
  ")

  VIOLATIONS_FILE="$violations_file" OUTPUT_FILE="$output_dir/invariants.json" node <<'NODE'
const fs = require('node:fs');
const violations = fs.readFileSync(process.env.VIOLATIONS_FILE, 'utf8')
  .split(/\r?\n/).map((value) => value.trim()).filter(Boolean);
fs.writeFileSync(process.env.OUTPUT_FILE, `${JSON.stringify({
  passed: violations.length === 0,
  violations: [...new Set(violations)],
}, null, 2)}\n`);
NODE
}

write_metadata() {
  COMMIT="$(git -C "$repo_root" rev-parse HEAD)" SUITE="$suite" OUTPUT_FILE="$output_dir/metadata.json" node <<'NODE'
const fs = require('node:fs');
const os = require('node:os');
fs.writeFileSync(process.env.OUTPUT_FILE, `${JSON.stringify({
  schema_version: 2,
  commit: process.env.COMMIT,
  suite: process.env.SUITE,
  environment: 'Ubuntu WSL Docker Engine on a shared Windows development host',
  cpu_count: os.cpus().length,
  total_memory_bytes: os.totalmem(),
  demo_fixture: '20260730_demo_fixture_v1',
  recorded_at: new Date().toISOString(),
}, null, 2)}\n`);
NODE
}

if [[ "$suite" == "quick" ]]; then
  baseline_repeats=1
  baseline_duration=5s
  load_duration=10s
  stress_duration=8s
  stability_duration=20s
  warmup=1s
  payment_requests=2
  rabbit_recovery_requests=4
  idempotency_requests=5
  profile_seconds=5
  read_stress=(800 1200)
  order_stress=(12 20)
else
  baseline_repeats=3
  baseline_duration=30s
  load_duration=60s
  stress_duration=45s
  stability_duration=300s
  warmup=10s
  payment_requests=8
  rabbit_recovery_requests=12
  idempotency_requests=40
  profile_seconds=15
  read_stress=(800 1200 2000 3000)
  order_stress=(15 25 40 60)
fi

echo "[performance] preparing current source and fixed fixture"
if [[ "$skip_rebuild" -eq 1 ]]; then
  "$control" start --profile interview --observability >/dev/null
else
  "$control" rebuild --profile interview --observability >/dev/null
fi
if ! "$control" verify-demo --profile interview --observability >/dev/null; then
  echo "[performance] current fixture needs the confirmed reset" >&2
fi
upload_volume_before="$(docker volume inspect -f '{{.Name}}' flash-mall-uploads)"
"$control" reset-demo --confirm-reset --profile interview --observability >/dev/null
restore_needed=1
"$control" verify-demo --profile interview --observability >/dev/null
compose_network="$(docker inspect -f '{{range $name, $_ := .NetworkSettings.Networks}}{{$name}}{{end}}' hertz-gateway)"
[[ -n "$compose_network" ]] || { echo "cannot resolve Hertz Compose network" >&2; exit 1; }
CGO_ENABLED=0 "/home/mildred/.local/go/bin/go" build -o "$tool" "$repo_root/tools/capacitybench"
write_metadata

echo "[performance] baseline: repeated low-load latency"
for repeat in $(seq 1 "$baseline_repeats"); do
  run_stage baseline read 100 "$baseline_duration" "$warmup" 24 "baseline-read-${repeat}"
  run_stage baseline order-cycle 2 "$baseline_duration" "$warmup" 8 "baseline-order-${repeat}"
done

echo "[performance] load: expected operating levels"
run_stage load read 600 "$load_duration" "$warmup" 64 load-read
run_stage load order-cycle 10 "$load_duration" "$warmup" 24 load-order

echo "[performance] stress: searching for latency or throughput knee"
run_stress_ladder read "$stress_duration" "$warmup" 120 "${read_stress[@]}"
run_stress_ladder order-cycle "$stress_duration" "$warmup" 80 "${order_stress[@]}"

echo "[performance] stability: sustained mixed traffic"
run_stability_mix "$stability_duration" "$warmup"

echo "[performance] recovery: payment, idempotency and RabbitMQ interruption"
run_fixed_stage recovery payment-cycle "$payment_requests" 2 8 recovery-payment-normal
run_fixed_stage recovery idempotency "$idempotency_requests" 20 "$idempotency_requests" recovery-idempotency

token="$(curl --noproxy '*' -fsS -X POST "$base_url/api/auth/login" \
  -H 'Content-Type: application/json' \
  -d '{"phone":"13800000001","password":"flashmall123","device_type":"performance-cleanup"}' \
  | node -e 'let s="";process.stdin.on("data",d=>s+=d).on("end",()=>process.stdout.write(JSON.parse(s).access_token||""))')"
idempotency_order="$(mysql_scalar "
  SELECT id FROM mall_order.orders WHERE request_id LIKE 'capacity-%-idempotency'
  ORDER BY create_time DESC LIMIT 1;
")"
if [[ -n "$idempotency_order" ]]; then
  curl --noproxy '*' -fsS -X POST "$base_url/api/order/cancel" \
    -H 'Content-Type: application/json' -H "Authorization: Bearer $token" \
    -d "{\"order_id\":\"$idempotency_order\",\"reason\":\"performance idempotency cleanup\"}" >/dev/null
fi

docker pause rabbitmq >/dev/null
rabbit_paused=1
run_fixed_stage recovery payment-cycle "$rabbit_recovery_requests" 2 6 recovery-payment-rabbitmq-paused
docker unpause rabbitmq >/dev/null
rabbit_paused=0
wait_outbox_drained || record_violation outbox_recovery_timeout

verify_invariants
render_profiles
node "$repo_root/scripts/perf/summarize-performance.mjs" "$output_dir" \
  "$output_dir/summary.json" "$output_dir/report.md"

echo "[performance] restoring fixed interview fixture"
"$control" reset-demo --confirm-reset --profile interview --observability >/dev/null
restore_needed=0
"$control" verify-demo --profile interview --observability >/dev/null
[[ "$(docker volume inspect -f '{{.Name}}' flash-mall-uploads)" == "$upload_volume_before" ]]

cat "$output_dir/report.md"
node -e '
  const summary = JSON.parse(require("node:fs").readFileSync(process.argv[1], "utf8"));
  process.exit(summary.overall_passed ? 0 : 1);
' "$output_dir/summary.json"
