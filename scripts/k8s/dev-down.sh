#!/usr/bin/env sh
set -eu

namespace="${FLASH_MALL_K8S_NAMESPACE:-flash-mall}"
cluster_name="${FLASH_MALL_KIND_CLUSTER:-flash-mall}"
namespace_only=0

while [ "$#" -gt 0 ]; do
  case "$1" in
    --cluster)
      shift
      cluster_name="${1:-}"
      [ -n "$cluster_name" ] || { echo "--cluster requires name" >&2; exit 2; }
      ;;
    --namespace|-n)
      shift
      namespace="${1:-}"
      [ -n "$namespace" ] || { echo "--namespace requires name" >&2; exit 2; }
      ;;
    --namespace-only) namespace_only=1 ;;
    -h|--help)
      cat <<'EOF'
Usage: scripts/k8s/dev-down.sh [options]

Options:
  --cluster NAME       kind cluster name. Default: flash-mall.
  -n, --namespace NAME Kubernetes namespace. Default: flash-mall.
  --namespace-only     Delete only the flash-mall namespace in the current cluster.

Default behavior deletes the local kind cluster.
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

if [ "$namespace_only" -eq 1 ]; then
  command -v kubectl >/dev/null 2>&1 || {
    echo "kubectl not found in PATH" >&2
    exit 1
  }
  echo "[DELETE] namespace/$namespace"
  kubectl delete namespace "$namespace" --ignore-not-found=true
  exit 0
fi

command -v kind >/dev/null 2>&1 || {
  echo "kind not found in PATH" >&2
  exit 1
}

echo "[DELETE] kind cluster $cluster_name"
kind delete cluster --name "$cluster_name"
