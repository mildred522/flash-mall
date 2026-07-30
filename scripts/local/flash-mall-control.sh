#!/usr/bin/env sh
set -eu

script_path="$0"
if [ "${FLASH_MALL_CONTROL_SOURCE_ONLY:-0}" = "1" ] && [ -n "${1:-}" ] && [ -f "$1" ]; then
  script_path="$1"
fi
script_dir=$(CDPATH= cd -- "$(dirname -- "$script_path")" && pwd)
repo_root=$(CDPATH= cd -- "$script_dir/../.." && pwd)
deploy_dir="$repo_root/deploy"
compose_file="docker-compose.yml"
business_services="auth-api product-rpc order-rpc inventory-kitex hertz-gateway"
log_services="$business_services mysql mysql-init redis redis-init rabbitmq etcd dtm jaeger"
wait_timeout=180
error_emitted=0
command_name=""
service_name=""
run_profile="development"
observability=0
confirm_reset=0

if [ -n "${HOME:-}" ]; then
  PATH="$HOME/.local/go/bin:$HOME/go/bin:/usr/local/go/bin:$HOME/.local/bin:$PATH"
  export PATH
fi

. "$script_dir/compose-env.sh"

json_escape() {
  awk 'BEGIN { ORS="" }
    {
      if (NR > 1) printf "\\n"
      gsub(/\\/, "\\\\")
      gsub(/"/, "\\\"")
      gsub(/\r/, "\\r")
      gsub(/\t/, "\\t")
      printf "%s", $0
    }'
}

emit() {
  type="$1"
  phase="$2"
  level="$3"
  message="$4"
  service="${5:-}"
  code="${6:-}"
  escaped_type=$(printf '%s' "$type" | json_escape)
  escaped_phase=$(printf '%s' "$phase" | json_escape)
  escaped_level=$(printf '%s' "$level" | json_escape)
  escaped_message=$(printf '%s' "$message" | json_escape)
  escaped_service=$(printf '%s' "$service" | json_escape)
  escaped_code=$(printf '%s' "$code" | json_escape)
  printf '{"type":"%s","phase":"%s","level":"%s","message":"%s","service":"%s","code":"%s"}\n' \
    "$escaped_type" "$escaped_phase" "$escaped_level" "$escaped_message" "$escaped_service" "$escaped_code"
}

fail() {
  phase="$1"
  code="$2"
  message="$3"
  error_emitted=1
  emit error "$phase" error "$message" "${service_name:-}" "$code"
  return 1
}

usage() {
  cat <<'EOF'
Usage: scripts/local/flash-mall-control.sh COMMAND [SERVICE] [options]

Commands:
  start                       Start from existing images.
  rebuild                     Rebuild business images and start the stack.
  rebuild-service SERVICE     Rebuild and recreate one business service.
  stop                        Stop the stack while preserving volumes.
  status                      Report compose and application health.
  verify-demo                 Verify fixture version, accounts and stock consistency.
  reset-demo --confirm-reset  Back up data, reset business volumes and start clean fixtures.
  logs [SERVICE]              Stream recent compose logs.
  help                        Show this help.

Options:
  --wait-timeout SECONDS      Health timeout from 30 to 900 (default: 180).
  --profile NAME              development or interview (default: development).
  --observability             Start Prometheus and Grafana with the business stack.

Business services:
  auth-api product-rpc order-rpc inventory-kitex hertz-gateway
EOF
}

contains_word() {
  needle="$1"
  shift
  for candidate in "$@"; do
    if [ "$candidate" = "$needle" ]; then
      return 0
    fi
  done
  return 1
}

is_business_service() {
  # shellcheck disable=SC2086
  contains_word "$1" $business_services
}

is_log_service() {
  # shellcheck disable=SC2086
  contains_word "$1" $log_services
}

validate_timeout() {
  case "$wait_timeout" in
    ''|*[!0-9]*) fail arguments invalid_timeout "wait timeout must be an integer from 30 to 900" ;;
    *)
      if [ "$wait_timeout" -lt 30 ] || [ "$wait_timeout" -gt 900 ]; then
        fail arguments invalid_timeout "wait timeout must be between 30 and 900 seconds"
      fi
      ;;
  esac
}

validate_profile() {
  case "$run_profile" in
    development|interview) ;;
    *) fail arguments invalid_profile "profile must be development or interview" ;;
  esac
}

check_docker() {
  emit progress preflight info "checking WSL Docker engine"
  if ! command -v docker >/dev/null 2>&1 || ! timeout 10 docker info >/dev/null 2>&1; then
    fail preflight docker_unreachable "WSL Docker engine is unreachable"
  fi
}

check_images() {
  missing=""
  for service in $business_services; do
    image="flash-mall/$service:$FLASH_MALL_IMAGE_TAG"
    if ! docker image inspect "$image" >/dev/null 2>&1; then
      missing="${missing}${missing:+, }$image"
    fi
  done
  if [ -n "$missing" ]; then
    fail preflight images_missing "missing images: $missing; run rebuild first"
  fi
}

emit_service_statuses() {
  rows=$(cd "$deploy_dir" && docker compose -f "$compose_file" ps --all --format '{{.Service}}|{{.State}}|{{.Status}}' 2>/dev/null || true)
  if [ -z "$rows" ]; then
    return 1
  fi
  printf '%s\n' "$rows" | while IFS='|' read -r service state status; do
    [ -n "$service" ] || continue
    emit service status info "${status:-$state}" "$service" "$state"
  done
}

run_status() {
  check_docker
  if ! emit_service_statuses; then
    emit state status info "Flash Mall is stopped" "" stopped
    return 0
  fi

  emit progress health info "checking gateway and stock health"
  if "$script_dir/health-compose.sh"; then
    emit state health info "Flash Mall is ready" "" ready
    return 0
  fi

  error_emitted=1
  emit state health warning "Flash Mall is partially available" "" partial
  emit error health warning "one or more health checks failed" "" health_failed
  return 1
}

run_start() {
  check_docker
  check_images
  if [ "$observability" -eq 1 ]; then
    export COMPOSE_PROFILES=observability
  fi
  emit progress startup info "starting compose services from existing images"
  "$script_dir/start-compose-all.sh" --no-build --wait-timeout "$wait_timeout"
  emit_service_statuses || true
  if [ "$run_profile" = "interview" ]; then
    run_verify_demo
  fi
  emit state health info "Flash Mall is ready" "" ready
}

run_rebuild() {
  check_docker
  if [ "$observability" -eq 1 ]; then
    export COMPOSE_PROFILES=observability
  fi
  emit progress build info "building Flash Mall business services"
  # shellcheck disable=SC2086
  "$script_dir/build-compose-images.sh" --tag "$FLASH_MALL_IMAGE_TAG" $business_services
  emit progress startup info "starting rebuilt compose services"
  "$script_dir/start-compose-all.sh" --no-build --wait-timeout "$wait_timeout"
  emit_service_statuses || true
  if [ "$run_profile" = "interview" ]; then
    run_verify_demo
  fi
  emit state health info "Flash Mall is ready" "" ready
}

run_rebuild_service() {
  check_docker
  emit progress build info "rebuilding $service_name" "$service_name"
  "$script_dir/rebuild-compose-service.sh" --tag "$FLASH_MALL_IMAGE_TAG" "$service_name"
  emit progress health info "waiting for stack health" "$service_name"
  "$script_dir/health-compose.sh" --wait "$wait_timeout" --logs-on-failure
  emit_service_statuses || true
  emit state health info "Flash Mall is ready" "" ready
}

run_stop() {
  check_docker
  emit progress shutdown info "stopping compose services and preserving data volumes"
  "$script_dir/stop-compose-all.sh"
  emit state shutdown info "Flash Mall is stopped" "" stopped
}

mysql_scalar() {
  query="$1"
  docker exec mysql mysql --default-character-set=utf8mb4 -N -uroot \
    -p"$FLASH_MALL_MYSQL_ROOT_PASSWORD" -e "$query" 2>/dev/null
}

demo_redis_stock_matches() {
  rows=$(mysql_scalar "
    SELECT product_id,bucket_idx,stock
    FROM mall_product.product_stock_bucket
    WHERE product_id IN (100,101,102,103,104,201,202,211,212)
    ORDER BY product_id,bucket_idx;
  ")
  printf '%s\n' "$rows" | while IFS="$(printf '\t')" read -r product bucket expected; do
    [ -n "$product" ] || continue
    actual=$(docker exec redis redis-cli --raw GET "stock:${product}:${bucket}" 2>/dev/null)
    [ "$actual" = "$expected" ] || return 1
  done
}

run_verify_demo() {
  check_docker
  if ! docker inspect mysql >/dev/null 2>&1; then
    fail demo demo_mysql_unavailable "MySQL is not running; start Flash Mall first"
    return 1
  fi

  version_count=$(mysql_scalar "
    SELECT COUNT(*) FROM information_schema.tables
    WHERE table_schema='mall_order' AND table_name='demo_fixture_state';
  ")
  if [ "$version_count" != "1" ]; then
    fail demo demo_not_initialized "demo fixture state is missing"
    return 1
  fi
  version_count=$(mysql_scalar "
    SELECT COUNT(*) FROM mall_order.demo_fixture_state
    WHERE fixture_version='20260730_demo_fixture_v1';
  ")
  if [ "$version_count" != "1" ]; then
    fail demo demo_version_stale "demo fixture version is stale; reset is required"
    return 1
  fi

  core_count=$(mysql_scalar "
    SELECT
      (SELECT COUNT(*) FROM mall_auth.users WHERE id IN (1001,1002,1101,1102) AND status=1) +
      (SELECT COUNT(*) FROM mall_order.merchant WHERE id IN (1000,1101,1102) AND status=1) +
      (SELECT COUNT(*) FROM mall_product.product WHERE id IN (100,101,102,103,104,201,202,211,212) AND status=1);
  ")
  if [ "$core_count" != "16" ]; then
    fail demo demo_core_incomplete "demo accounts, merchants or products are incomplete (${core_count}/16)"
    return 1
  fi

  mismatch_count=$(mysql_scalar "
    SELECT COUNT(*) FROM (
      SELECT p.id
      FROM mall_product.product p
      LEFT JOIN mall_product.product_stock_snapshot s ON s.product_id=p.id
      LEFT JOIN mall_product.product_stock_bucket b ON b.product_id=p.id
      WHERE p.id IN (100,101,102,103,104,201,202,211,212)
      GROUP BY p.id,p.stock,s.product_id,s.available,s.reserved,s.total
      HAVING s.product_id IS NULL OR p.stock<>s.total OR s.available+s.reserved<>s.total OR COALESCE(SUM(b.stock),0)<>s.total
    ) inconsistent;
  ")
  if [ "$mismatch_count" != "0" ]; then
    fail demo demo_stock_inconsistent "demo stock facts are inconsistent for ${mismatch_count} products"
    return 1
  fi
  if ! demo_redis_stock_matches; then
    fail demo demo_redis_stock_inconsistent "Redis stock does not match MySQL stock buckets"
    return 1
  fi

  emit demo demo info "demo fixture 20260730_demo_fixture_v1 is ready" "" demo_ready
}

find_compose_volume() {
  logical_name="$1"
  project_name="${COMPOSE_PROJECT_NAME:-$(basename "$deploy_dir")}"
  volumes=$(docker volume ls \
    --filter "label=com.docker.compose.project=$project_name" \
    --filter "label=com.docker.compose.volume=$logical_name" -q)
  [ "$(printf '%s\n' "$volumes" | grep -c .)" = "1" ] || return 1
  printf '%s' "$volumes"
}

run_reset_demo() {
  check_docker
  umask 077
  backup_dir="$repo_root/.runtime/demo-backups"
  mkdir -p "$backup_dir"
  if docker inspect mysql >/dev/null 2>&1; then
    backup="$backup_dir/mysql-$(date +%Y%m%d-%H%M%S).sql.gz"
    emit progress demo info "backing up MySQL before demo reset"
    docker exec mysql sh -c \
      'exec mysqldump -uroot -p"$MYSQL_ROOT_PASSWORD" --single-transaction --routines --databases mall_auth mall_order mall_product' \
      | gzip > "$backup"
    chmod 600 "$backup"
    emit progress demo info "backup saved to $backup"
  fi

  mysql_volume=$(find_compose_volume mysql-data) ||
    fail demo unsafe_volume_target "cannot identify the Flash Mall MySQL volume"
  redis_volume=$(find_compose_volume redis-data) ||
    fail demo unsafe_volume_target "cannot identify the Flash Mall Redis volume"
  upload_id=$(docker volume inspect -f '{{.Name}}' flash-mall-uploads 2>/dev/null || true)

  emit progress demo warning "stopping services before resetting MySQL and Redis"
  (cd "$deploy_dir" && docker compose -f "$compose_file" down --remove-orphans)
  docker volume rm "$mysql_volume" "$redis_volume" >/dev/null
  run_start

  current_upload_id=$(docker volume inspect -f '{{.Name}}' flash-mall-uploads 2>/dev/null || true)
  if [ "$upload_id" != "$current_upload_id" ]; then
    fail demo upload_volume_changed "upload volume identity changed during demo reset"
  fi
  if [ "$run_profile" != "interview" ]; then
    run_verify_demo
  fi
}

run_logs() {
  check_docker
  emit progress logs info "reading recent compose logs" "$service_name"
  if [ -n "$service_name" ]; then
    (cd "$deploy_dir" && docker compose -f "$compose_file" logs --tail 300 "$service_name")
  else
    (cd "$deploy_dir" && docker compose -f "$compose_file" logs --tail 300)
  fi
}

on_exit() {
  exit_code=$?
  if [ "$exit_code" -ne 0 ] && [ "$error_emitted" -eq 0 ]; then
    emit error "${command_name:-control}" error "command failed with exit code $exit_code" "${service_name:-}" command_failed
  fi
  exit "$exit_code"
}

main() {
  command_name="${1:-help}"
  if [ "$#" -gt 0 ]; then shift; fi

  case "$command_name" in
    rebuild-service|logs)
      if [ "$#" -gt 0 ] && [ "${1#--}" = "$1" ]; then
        service_name="$1"
        shift
      fi
      ;;
  esac

  while [ "$#" -gt 0 ]; do
    case "$1" in
      --wait-timeout)
        shift
        if [ "$#" -eq 0 ]; then
          fail arguments invalid_timeout "--wait-timeout requires seconds"
        fi
        wait_timeout="$1"
        ;;
      --profile)
        shift
        if [ "$#" -eq 0 ]; then
          fail arguments invalid_profile "--profile requires development or interview"
        fi
        run_profile="$1"
        ;;
      --observability) observability=1 ;;
      --confirm-reset) confirm_reset=1 ;;
      *) fail arguments invalid_option "unknown option: $1" ;;
    esac
    shift
  done

  validate_timeout
  validate_profile
  case "$command_name" in
    help|-h|--help) usage ;;
    start) run_start ;;
    rebuild) run_rebuild ;;
    rebuild-service)
      if [ -z "$service_name" ] || ! is_business_service "$service_name"; then
        fail arguments invalid_service "invalid_service: choose one of $business_services"
      fi
      run_rebuild_service
      ;;
    stop) run_stop ;;
    status) run_status ;;
    verify-demo) run_verify_demo ;;
    reset-demo)
      if [ "$confirm_reset" -ne 1 ]; then
        fail arguments reset_confirmation_required "reset-demo requires --confirm-reset"
      fi
      run_reset_demo
      ;;
    logs)
      if [ -n "$service_name" ] && ! is_log_service "$service_name"; then
        fail arguments invalid_service "invalid_service: unknown compose service $service_name"
      fi
      run_logs
      ;;
    *) fail arguments invalid_command "unknown command: $command_name" ;;
  esac
}

if [ "${FLASH_MALL_CONTROL_SOURCE_ONLY:-0}" != "1" ]; then
  trap on_exit EXIT
  main "$@"
  trap - EXIT
fi
