#!/usr/bin/env sh
set -eu

namespace="${FLASH_MALL_K8S_NAMESPACE:-flash-mall}"
timeout="${FLASH_MALL_K8S_WAIT_TIMEOUT:-180s}"

while [ "$#" -gt 0 ]; do
  case "$1" in
    --namespace|-n)
      shift
      namespace="${1:-}"
      [ -n "$namespace" ] || { echo "--namespace requires value" >&2; exit 2; }
      ;;
    --timeout)
      shift
      timeout="${1:-}"
      [ -n "$timeout" ] || { echo "--timeout requires value" >&2; exit 2; }
      ;;
    -h|--help)
      cat <<'EOF'
Usage: scripts/k8s/wait-ready.sh [options]

Options:
  -n, --namespace NAME  Kubernetes namespace. Default: flash-mall.
  --timeout DURATION    kubectl wait timeout. Default: 180s.
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

command -v kubectl >/dev/null 2>&1 || {
  echo "kubectl not found in PATH" >&2
  exit 1
}

wait_deploy() {
  name="$1"
  echo "[WAIT] deployment/$name"
  kubectl -n "$namespace" rollout status "deployment/$name" --timeout="$timeout"
}

wait_job() {
  name="$1"
  echo "[WAIT] job/$name"
  kubectl -n "$namespace" wait --for=condition=complete "job/$name" --timeout="$timeout"
}

for deploy in mysql redis rabbitmq etcd dtm; do
  wait_deploy "$deploy"
done

for job in mysql-init redis-seed; do
  wait_job "$job"
done

for deploy in auth-api product-rpc order-rpc inventory-kitex entry-api; do
  wait_deploy "$deploy"
done

kubectl -n "$namespace" get pods -o wide
