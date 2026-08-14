#!/usr/bin/env bash
set -euo pipefail

stage="${1:?stage is required}"
output_dir="${2:?output directory is required}"
interval="${3:-5}"

[[ "$stage" =~ ^[a-zA-Z0-9._-]+$ ]] || { echo "invalid stage: $stage" >&2; exit 2; }
[[ "$interval" =~ ^[1-9][0-9]*$ ]] || { echo "invalid interval: $interval" >&2; exit 2; }
mkdir -p "$output_dir"

docker_file="$output_dir/resources-docker.jsonl"
service_file="$output_dir/resources-services.tsv"
host_file="$output_dir/resources-host.tsv"
stopping=0
trap 'stopping=1' INT TERM

record_service_metrics() {
  local timestamp="$1"
  docker exec mysql mysql --default-character-set=utf8mb4 -N -uroot \
    -p"${FLASH_MALL_MYSQL_ROOT_PASSWORD:-6494kj06}" -e "
      SHOW GLOBAL STATUS WHERE Variable_name IN (
        'Threads_connected','Threads_running','Slow_queries',
        'Innodb_row_lock_current_waits','Innodb_row_lock_time',
        'Innodb_buffer_pool_reads','Innodb_buffer_pool_read_requests'
      );
    " 2>/dev/null | while IFS=$'\t' read -r name value; do
      printf '%s\t%s\tmysql\t%s\t%s\n' "$timestamp" "$stage" "$name" "$value"
    done >> "$service_file" || true

  docker exec redis redis-cli --raw INFO stats memory clients 2>/dev/null \
    | tr -d '\r' \
    | awk -F: '/^(instantaneous_ops_per_sec|blocked_clients|connected_clients|used_memory|used_memory_rss|keyspace_hits|keyspace_misses):/ {print $1 "\t" $2}' \
    | while IFS=$'\t' read -r name value; do
      printf '%s\t%s\tredis\t%s\t%s\n' "$timestamp" "$stage" "$name" "$value"
    done >> "$service_file" || true
}

while [[ "$stopping" -eq 0 ]]; do
  timestamp="$(date -u +%Y-%m-%dT%H:%M:%S.%3NZ)"
  docker stats --no-stream --format '{{json .}}' 2>/dev/null \
    | sed "s/^{/{\"timestamp\":\"$timestamp\",\"stage\":\"$stage\",/" \
    >> "$docker_file" || true
  record_service_metrics "$timestamp"
  read -r load1 load5 load15 _ < /proc/loadavg
  available_kb="$(awk '/^MemAvailable:/ {print $2}' /proc/meminfo)"
  printf '%s\t%s\t%s\t%s\t%s\t%s\n' \
    "$timestamp" "$stage" "$load1" "$load5" "$load15" "$available_kb" >> "$host_file"
  sleep "$interval" &
  wait $! || true
done
