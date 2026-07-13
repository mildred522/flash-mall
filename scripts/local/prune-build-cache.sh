#!/usr/bin/env sh
set -eu

mode="normal"
if [ "$#" -gt 1 ]; then
  echo "Usage: scripts/local/prune-build-cache.sh [--milestone]" >&2
  exit 2
fi
if [ "$#" -eq 1 ]; then
  if [ "$1" != "--milestone" ]; then
    echo "unknown option: $1" >&2
    exit 2
  fi
  mode="milestone"
fi

command -v docker >/dev/null 2>&1 || {
  echo "docker not found in PATH" >&2
  exit 1
}

echo "[BUILD CACHE BEFORE]"
docker buildx du

if [ "$mode" = "milestone" ]; then
  docker buildx prune --all --force
else
  docker buildx prune --force \
    --filter "until=48h" \
    --max-used-space 8gb \
    --reserved-space 2gb
fi

echo "[BUILD CACHE AFTER]"
docker buildx du
