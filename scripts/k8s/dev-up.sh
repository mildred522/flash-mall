#!/usr/bin/env sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(CDPATH= cd -- "$script_dir/../.." && pwd)
namespace="${FLASH_MALL_K8S_NAMESPACE:-flash-mall}"
cluster_name="${FLASH_MALL_KIND_CLUSTER:-flash-mall}"
rebuild_images=0
existing_cluster=0
skip_apply=0
skip_wait=0
port_forward=1
profile="local"

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
    --rebuild-images) rebuild_images=1 ;;
    --existing-cluster) existing_cluster=1 ;;
    --skip-apply) skip_apply=1 ;;
    --skip-wait) skip_wait=1 ;;
    --port-forward) port_forward=1 ;;
    --no-port-forward) port_forward=0 ;;
    --profile)
      shift
      profile="${1:-}"
      case "$profile" in
        local|demo) ;;
        *) echo "--profile must be local or demo" >&2; exit 2 ;;
      esac
      ;;
    -h|--help)
      cat <<'EOF'
Usage: scripts/k8s/dev-up.sh [options]

Options:
  --cluster NAME       kind cluster name. Default: flash-mall.
  -n, --namespace NAME Kubernetes namespace. Default: flash-mall.
  --rebuild-images     Rebuild local flash-mall images before deployment.
  --existing-cluster   Use the current kube context; do not recreate kind.
  --skip-apply         Do not apply manifests.
  --skip-wait          Do not wait for jobs/deployments.
  --port-forward       Restore entry-api port-forward after startup. Default.
  --no-port-forward    Do not restore entry-api port-forward.
  --profile local|demo Apply local single-node overrides or keep demo replicas.
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

cd "$repo_root"

if [ "$existing_cluster" -eq 0 ]; then
  kind_args="--cluster $cluster_name"
  if [ "$rebuild_images" -eq 1 ]; then
    kind_args="$kind_args --rebuild-images"
  fi
  if [ "$skip_apply" -eq 1 ]; then
    kind_args="$kind_args --skip-apply"
  fi
  kind_args="$kind_args --profile $profile"
  # shellcheck disable=SC2086
  "$script_dir/kind-deploy.sh" $kind_args
elif [ "$skip_apply" -eq 0 ]; then
  if [ "$rebuild_images" -eq 1 ]; then
    "$script_dir/build-images.sh" dev
  fi
  "$script_dir/load-images.sh" --cluster "$cluster_name"
  "$script_dir/apply.sh" "$namespace"
fi

if [ "$skip_apply" -eq 0 ] && [ "$profile" = "local" ]; then
  "$script_dir/apply-local-profile.sh" --namespace "$namespace"
fi

if [ "$skip_apply" -eq 0 ] && [ "$skip_wait" -eq 0 ]; then
  "$script_dir/wait-ready.sh" --namespace "$namespace"
  "$script_dir/seed-redis-stock.sh" --namespace "$namespace"
fi

cat <<EOF

K8s local stack is prepared.
Entry-api port-forward is restored by default.
Open:
  http://127.0.0.1:8888
EOF

if [ "$port_forward" -eq 1 ]; then
  "$script_dir/restore-port-forward.sh" --namespace "$namespace"
fi
