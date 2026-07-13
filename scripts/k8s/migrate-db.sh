#!/usr/bin/env sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(CDPATH= cd -- "$script_dir/../.." && pwd)
namespace="${FLASH_MALL_K8S_NAMESPACE:-flash-mall}"
password="${FLASH_MALL_MYSQL_ROOT_PASSWORD:-flashmall}"

while [ "$#" -gt 0 ]; do
  case "$1" in
    --namespace|-n)
      shift
      namespace="${1:-}"
      [ -n "$namespace" ] || { echo "--namespace requires name" >&2; exit 2; }
      ;;
    --password)
      shift
      password="${1:-}"
      [ -n "$password" ] || { echo "--password requires value" >&2; exit 2; }
      ;;
    -h|--help)
      cat <<'EOF'
Usage: scripts/k8s/migrate-db.sh [options]

Options:
  -n, --namespace NAME  Kubernetes namespace. Default: flash-mall.
  --password PASSWORD   MySQL root password. Default: flashmall.
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

mysql_pod=$(kubectl -n "$namespace" get pod -l app=mysql -o jsonpath='{.items[0].metadata.name}')
if [ -z "$mysql_pod" ]; then
  echo "mysql pod not found in namespace $namespace" >&2
  exit 1
fi

echo "[MYSQL] apply scripts/k8s/init-db.sql through pod/$mysql_pod"
kubectl -n "$namespace" exec -i "$mysql_pod" -- mysql --default-character-set=utf8mb4 -uroot -p"$password" < "$repo_root/scripts/k8s/init-db.sql"
