#!/usr/bin/env sh
set -eu

pid_file="${1:-/tmp/flash-mall-wsl-keepalive.pid}"

if [ -s "$pid_file" ]; then
  old_pid="$(cat "$pid_file" 2>/dev/null || true)"
  if [ -n "$old_pid" ] && kill -0 "$old_pid" 2>/dev/null; then
    exit 0
  fi
fi

printf '%s\n' "$$" > "$pid_file"

cleanup() {
  rm -f "$pid_file"
  exit 0
}

trap cleanup INT TERM

while :; do
  sleep 3600
done
