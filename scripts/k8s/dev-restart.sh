#!/usr/bin/env sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
namespace="${FLASH_MALL_K8S_NAMESPACE:-flash-mall}"
profile="local"
port_forward=1

while [ "$#" -gt 0 ]; do
  case "$1" in
    --namespace|-n)
      shift
      namespace="${1:-}"
      [ -n "$namespace" ] || { echo "--namespace requires name" >&2; exit 2; }
      ;;
    --profile)
      shift
      profile="${1:-}"
      case "$profile" in
        local|demo) ;;
        *) echo "--profile must be local or demo" >&2; exit 2 ;;
      esac
      ;;
    --no-port-forward) port_forward=0 ;;
    -h|--help)
      cat <<'EOF'
Usage: scripts/k8s/dev-restart.sh [options]

Options:
  -n, --namespace NAME Kubernetes namespace. Default: flash-mall.
  --profile local|demo Apply local single-node overrides or keep demo replicas.
  --no-port-forward    Do not restore entry-api port-forward.
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

echo "[CLEAN] delete Unknown pods"
kubectl -n "$namespace" get pods --field-selector=status.phase=Unknown -o name 2>/dev/null |
  xargs -r kubectl -n "$namespace" delete --force --grace-period=0

"$script_dir/apply.sh" "$namespace"

if [ "$profile" = "local" ]; then
  "$script_dir/apply-local-profile.sh" --namespace "$namespace"
fi

"$script_dir/wait-ready.sh" --namespace "$namespace"

if [ "$port_forward" -eq 1 ]; then
  "$script_dir/restore-port-forward.sh" --namespace "$namespace"
fi

"$script_dir/health.sh" "$namespace"
