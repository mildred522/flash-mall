#!/usr/bin/env sh
set -eu

namespace="${FLASH_MALL_K8S_NAMESPACE:-flash-mall}"

while [ "$#" -gt 0 ]; do
  case "$1" in
    --namespace|-n)
      shift
      namespace="${1:-}"
      [ -n "$namespace" ] || { echo "--namespace requires name" >&2; exit 2; }
      ;;
    -h|--help)
      cat <<'EOF'
Usage: scripts/k8s/apply-local-profile.sh [options]

Options:
  -n, --namespace NAME  Kubernetes namespace. Default: flash-mall.

Applies local single-node overrides after manifests are applied.
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

for deploy in auth-api entry-api order-rpc product-rpc inventory-kitex; do
  echo "[LOCAL PROFILE] scale deployment/$deploy to 1"
  kubectl -n "$namespace" scale "deployment/$deploy" --replicas=1
done

for hpa in auth-api-hpa entry-api-hpa order-rpc-hpa product-rpc-hpa inventory-kitex-hpa; do
  if kubectl -n "$namespace" get hpa "$hpa" >/dev/null 2>&1; then
    echo "[LOCAL PROFILE] constrain hpa/$hpa to 1 replica"
    kubectl -n "$namespace" patch hpa "$hpa" --type merge -p '{"spec":{"minReplicas":1,"maxReplicas":1}}'
  fi
done
