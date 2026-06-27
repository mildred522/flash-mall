#!/usr/bin/env sh
set -eu

required_go="go1.24.11"
required_node_major="20"
repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)

check_cmd() {
  name="$1"
  if command -v "$name" >/dev/null 2>&1; then
    printf '[OK] %-22s %s\n' "$name" "$(command -v "$name")"
  else
    printf '[MISS] %-20s not found\n' "$name"
    return 1
  fi
}

fail=0

check_cmd go || fail=1
if command -v go >/dev/null 2>&1; then
  go_version=$(go version | awk '{print $3}')
  if [ "$go_version" = "$required_go" ]; then
    echo "[OK] go version $go_version"
  else
    echo "[WARN] go version $go_version, expected $required_go"
    fail=1
  fi
fi

check_cmd node || fail=1
if command -v node >/dev/null 2>&1; then
  node_major=$(node --version | sed 's/^v//' | cut -d. -f1)
  if [ "$node_major" = "$required_node_major" ]; then
    echo "[OK] node $(node --version)"
  else
    echo "[WARN] node $(node --version), expected major $required_node_major"
    fail=1
  fi
fi

check_cmd npm || fail=1
check_cmd docker || fail=1
if command -v docker >/dev/null 2>&1; then
  if timeout 10 docker info >/dev/null 2>&1; then
    echo "[OK] docker daemon reachable"
  else
    echo "[MISS] docker daemon is not reachable"
    fail=1
  fi
  if docker compose version >/dev/null 2>&1; then
    echo "[OK] docker compose $(docker compose version --short 2>/dev/null || docker compose version)"
  else
    echo "[MISS] docker compose plugin not available"
    fail=1
  fi
  if docker buildx version >/dev/null 2>&1; then
    echo "[OK] docker buildx $(docker buildx version | awk '{print $2}')"
  else
    echo "[MISS] docker buildx plugin not available"
    fail=1
  fi
fi

check_cmd git || fail=1
check_cmd curl || fail=1
check_cmd tar || fail=1
check_cmd gzip || fail=1
check_cmd unzip || fail=1
check_cmd goctl || fail=1
check_cmd protoc || fail=1
check_cmd protoc-gen-go || fail=1
check_cmd protoc-gen-go-grpc || fail=1

if [ -d "$repo_root/frontend/node_modules" ]; then
  echo "[OK] frontend node_modules"
else
  echo "[MISS] frontend node_modules"
  fail=1
fi

if [ -d "$repo_root/web/node_modules" ]; then
  echo "[OK] web node_modules"
else
  echo "[MISS] web node_modules"
  fail=1
fi

if [ "$fail" -ne 0 ]; then
  echo
  echo "Missing or mismatched prerequisites. Run: scripts/local/bootstrap-wsl.sh"
  exit 1
fi

echo
echo "All build prerequisites are present."
