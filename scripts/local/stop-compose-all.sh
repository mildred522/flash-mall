#!/usr/bin/env sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(CDPATH= cd -- "$script_dir/../.." && pwd)
deploy_dir="$repo_root/deploy"
compose_file="docker-compose.yml"
volumes=0

while [ "$#" -gt 0 ]; do
  case "$1" in
    --volumes) volumes=1 ;;
    -h|--help)
      cat <<'EOF'
Usage: scripts/local/stop-compose-all.sh [options]

Options:
  --volumes  Remove compose volumes, including local MySQL data.
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

args="-f $compose_file down"
if [ "$volumes" -eq 1 ]; then
  args="$args --volumes"
fi

cd "$deploy_dir"
echo "[COMPOSE] docker compose $args"
# shellcheck disable=SC2086
docker compose $args
