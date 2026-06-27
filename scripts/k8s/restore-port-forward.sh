#!/usr/bin/env sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(CDPATH= cd -- "$script_dir/../.." && pwd)
namespace="${FLASH_MALL_K8S_NAMESPACE:-flash-mall}"
local_port="${FLASH_MALL_ENTRY_API_PORT:-8888}"
target_port="8888"
log_dir="$repo_root/.runtime/k8s"

while [ "$#" -gt 0 ]; do
  case "$1" in
    --namespace|-n)
      shift
      namespace="${1:-}"
      [ -n "$namespace" ] || { echo "--namespace requires name" >&2; exit 2; }
      ;;
    --port)
      shift
      local_port="${1:-}"
      [ -n "$local_port" ] || { echo "--port requires value" >&2; exit 2; }
      ;;
    -h|--help)
      cat <<'EOF'
Usage: scripts/k8s/restore-port-forward.sh [options]

Options:
  -n, --namespace NAME Kubernetes namespace. Default: flash-mall.
  --port PORT          Local port. Default: 8888.
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

mkdir -p "$log_dir"

if ss -ltnp 2>/dev/null | grep -q ":$local_port "; then
  if pgrep -f "kubectl -n $namespace port-forward svc/entry-api $local_port:$target_port" >/dev/null 2>&1; then
    if curl --noproxy "*" -fsS -m 3 "http://127.0.0.1:$local_port/api/system/health" >/dev/null 2>&1; then
      echo "[PORT-FORWARD] already running on 127.0.0.1:$local_port"
      exit 0
    fi
    echo "[PORT-FORWARD] stale listener on 127.0.0.1:$local_port, restarting"
    pkill -f "kubectl -n $namespace port-forward svc/entry-api $local_port:$target_port" 2>/dev/null || true
    sleep 1
  else
    echo "[PORT-FORWARD] port $local_port is already in use by another process" >&2
    exit 1
  fi
fi

echo "[PORT-FORWARD] svc/entry-api $local_port:$target_port"
(nohup kubectl -n "$namespace" port-forward svc/entry-api "$local_port:$target_port" > "$log_dir/entry-api-port-forward.log" 2>&1 &
  echo $! > "$log_dir/entry-api-port-forward.pid")

sleep 2
cat "$log_dir/entry-api-port-forward.log"
