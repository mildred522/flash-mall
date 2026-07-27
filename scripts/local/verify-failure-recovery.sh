#!/usr/bin/env bash
set -euo pipefail

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(CDPATH= cd -- "$script_dir/../.." && pwd)
gateway_url="${FLASH_MALL_GATEWAY_URL:-http://127.0.0.1:8889}"
prometheus_url="${FLASH_MALL_PROMETHEUS_URL:-http://127.0.0.1:9099}"
mysql_password="${FLASH_MALL_MYSQL_ROOT_PASSWORD:-6494kj06}"
scenario="all"
allow_disruption=0
paused_containers=""
probe_event_id=""
run_id="failure-recovery-$(date +%s)-$$"
report_dir="$repo_root/.runtime/failure-recovery"
report_path="$report_dir/$run_id.log"

usage() {
  cat <<'EOF'
Usage: scripts/local/verify-failure-recovery.sh --allow-disruption [--scenario NAME]

Scenarios: all, inventory, redis, mysql, order-rpc, rabbitmq

The verifier briefly pauses local Compose containers. It always unpauses them
and removes its isolated Outbox probe on exit.
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --allow-disruption) allow_disruption=1 ;;
    --scenario)
      shift
      scenario="${1:-}"
      [ -n "$scenario" ] || { echo "--scenario requires a value" >&2; exit 2; }
      ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
  shift
done

[ "$allow_disruption" -eq 1 ] || {
  echo "refusing to pause services without --allow-disruption" >&2
  exit 2
}

case "$scenario" in
  all|inventory|redis|mysql|order-rpc|rabbitmq) ;;
  *) echo "unsupported scenario: $scenario" >&2; exit 2 ;;
esac

mkdir -p "$report_dir"
exec > >(tee "$report_path") 2>&1

mysql_scalar() {
  docker exec mysql mysql --default-character-set=utf8mb4 -uroot "-p$mysql_password" \
    -N -s -e "$1" 2>/dev/null
}

delete_probe() {
  [ -n "$probe_event_id" ] || return 0
  mysql_scalar "USE mall_order; DELETE FROM event_process_log WHERE event_id='$probe_event_id'; DELETE FROM order_outbox WHERE event_id='$probe_event_id';" >/dev/null || true
  probe_event_id=""
}

cleanup() {
  for container in $paused_containers; do
    if [ "$(docker inspect -f '{{.State.Paused}}' "$container" 2>/dev/null || true)" = "true" ]; then
      docker unpause "$container" >/dev/null || true
    fi
  done
  delete_probe
}
trap cleanup EXIT

require_owned_container() {
  container="$1"
  project=$(docker inspect -f '{{ index .Config.Labels "com.docker.compose.project" }}' "$container" 2>/dev/null || true)
  state=$(docker inspect -f '{{.State.Running}}' "$container" 2>/dev/null || true)
  [ "$project" = "deploy" ] || { echo "$container is not owned by the Flash Mall Compose project" >&2; exit 1; }
  [ "$state" = "true" ] || { echo "$container is not running" >&2; exit 1; }
}

wait_until() {
  description="$1"
  timeout="$2"
  shift 2
  deadline=$(( $(date +%s) + timeout ))
  while [ "$(date +%s)" -lt "$deadline" ]; do
    if "$@"; then
      echo "[OK] $description"
      return 0
    fi
    sleep 1
  done
  echo "[FAIL] timed out: $description" >&2
  return 1
}

gateway_ready() {
  curl --noproxy "*" -fsS -m 3 "$gateway_url/api/system/health" >/dev/null
}

gateway_unready() {
  ! gateway_ready
}

prometheus_job_state() {
  job="$1"
  expected="$2"
  body=$(curl --noproxy "*" -fsSG -m 3 \
    --data-urlencode "query=up{job=\"$job\"}" "$prometheus_url/api/v1/query" || true)
  printf '%s' "$body" | grep -Eq "\"value\":\\[[^]]*,\"$expected\"\\]"
}

pause_container() {
  container="$1"
  docker pause "$container" >/dev/null
  paused_containers="$paused_containers $container"
  echo "[INJECT] paused $container"
}

resume_container() {
  container="$1"
  docker unpause "$container" >/dev/null
  echo "[RECOVER] unpaused $container"
}

verify_readiness_dependency() {
  container="$1"
  require_owned_container "$container"
  pause_container "$container"
  wait_until "$container makes gateway unready" 20 gateway_unready
  resume_container "$container"
  wait_until "$container and gateway recover" 40 gateway_ready
}

verify_rpc_target() {
  container="$1"
  job="$2"
  require_owned_container "$container"
  pause_container "$container"
  wait_until "$job Prometheus target becomes down" 30 prometheus_job_state "$job" 0
  resume_container "$container"
  wait_until "$job Prometheus target recovers" 40 prometheus_job_state "$job" 1
}

outbox_retried() {
  [ "$(mysql_scalar "USE mall_order; SELECT COUNT(*) FROM order_outbox WHERE event_id='$probe_event_id' AND status=0 AND attempt_count>0;")" = "1" ]
}

outbox_published() {
  [ "$(mysql_scalar "USE mall_order; SELECT COUNT(*) FROM order_outbox WHERE event_id='$probe_event_id' AND status=1 AND attempt_count>0;")" = "1" ]
}

verify_rabbitmq_outbox() {
  require_owned_container rabbitmq
  probe_event_id="chaos.probe:$run_id"
  pause_container rabbitmq
  mysql_scalar "USE mall_order;
    INSERT INTO order_outbox(event_id,event_type,aggregate_id,payload,status,next_retry_at)
    VALUES('$probe_event_id','chaos.probe','$run_id',JSON_OBJECT('probe',true,'run_id','$run_id'),0,NOW());" >/dev/null
  echo "[PROBE] inserted isolated Outbox event $probe_event_id"
  wait_until "Outbox records a RabbitMQ retry" 50 outbox_retried
  resume_container rabbitmq
  wait_until "Outbox publishes after RabbitMQ recovery" 60 outbox_published
  delete_probe
  echo "[CLEANUP] removed isolated Outbox probe"
}

run_scenario() {
  selected="$1"
  echo "[SCENARIO] $selected"
  case "$selected" in
    inventory) verify_rpc_target inventory-kitex inventory-kitex; wait_until "gateway recovers after inventory" 40 gateway_ready ;;
    redis) verify_readiness_dependency redis ;;
    mysql) verify_readiness_dependency mysql ;;
    order-rpc) verify_rpc_target order-rpc order-rpc ;;
    rabbitmq) verify_rabbitmq_outbox ;;
  esac
}

for container in hertz-gateway inventory-kitex order-rpc redis mysql rabbitmq; do
  require_owned_container "$container"
done
wait_until "baseline gateway readiness" 10 gateway_ready
wait_until "baseline Prometheus inventory target" 15 prometheus_job_state inventory-kitex 1
wait_until "baseline Prometheus order target" 15 prometheus_job_state order-rpc 1

if [ "$scenario" = "all" ]; then
  for selected in inventory order-rpc redis mysql rabbitmq; do
    run_scenario "$selected"
  done
else
  run_scenario "$scenario"
fi

wait_until "final gateway readiness" 40 gateway_ready
echo "[PASS] failure recovery verification completed"
echo "[REPORT] $report_path"
