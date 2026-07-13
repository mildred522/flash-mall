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
  logs [SERVICE]              Stream recent compose logs.
  help                        Show this help.

Options:
  --wait-timeout SECONDS      Health timeout from 30 to 900 (default: 180).

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
  emit progress startup info "starting compose services from existing images"
  "$script_dir/start-compose-all.sh" --no-build --wait-timeout "$wait_timeout"
  emit_service_statuses || true
  emit state health info "Flash Mall is ready" "" ready
}

run_rebuild() {
  check_docker
  emit progress build info "building Flash Mall business services"
  # shellcheck disable=SC2086
  "$script_dir/build-compose-images.sh" --tag "$FLASH_MALL_IMAGE_TAG" $business_services
  emit progress startup info "starting rebuilt compose services"
  "$script_dir/start-compose-all.sh" --no-build --wait-timeout "$wait_timeout"
  emit_service_statuses || true
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
      *) fail arguments invalid_option "unknown option: $1" ;;
    esac
    shift
  done

  validate_timeout
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
