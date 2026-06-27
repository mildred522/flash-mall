#!/usr/bin/env sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(CDPATH= cd -- "$script_dir/../.." && pwd)
cluster_name="${FLASH_MALL_KIND_CLUSTER:-flash-mall}"
config_path="${FLASH_MALL_KIND_CONFIG:-k8s/kind/cluster-local.yaml}"
rebuild_images=0
skip_apply=0
profile="local"

while [ "$#" -gt 0 ]; do
  case "$1" in
    --cluster)
      shift
      cluster_name="${1:-}"
      [ -n "$cluster_name" ] || { echo "--cluster requires name" >&2; exit 2; }
      ;;
    --config)
      shift
      config_path="${1:-}"
      [ -n "$config_path" ] || { echo "--config requires path" >&2; exit 2; }
      ;;
    --rebuild-images) rebuild_images=1 ;;
    --skip-apply) skip_apply=1 ;;
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
Usage: scripts/k8s/kind-deploy.sh [options]

Options:
  --cluster NAME     kind cluster name. Default: flash-mall.
  --config PATH      kind config path. Default: k8s/kind/cluster-local.yaml.
  --rebuild-images   Rebuild local flash-mall images before loading them.
  --skip-apply       Create/load the kind cluster but do not apply manifests.
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

command -v kind >/dev/null 2>&1 || {
  echo "kind not found in PATH" >&2
  exit 1
}
command -v docker >/dev/null 2>&1 || {
  echo "docker not found in PATH" >&2
  exit 1
}

kind delete cluster --name "$cluster_name" >/dev/null 2>&1 || true
kind create cluster --name "$cluster_name" --config "$config_path"

if [ "$rebuild_images" -eq 1 ]; then
  "$script_dir/build-images.sh" dev
fi

"$script_dir/load-images.sh" --cluster "$cluster_name"

if [ "$skip_apply" -eq 0 ]; then
  "$script_dir/apply.sh" flash-mall
  if [ "$profile" = "local" ]; then
    "$script_dir/apply-local-profile.sh" --namespace flash-mall
  fi
fi
