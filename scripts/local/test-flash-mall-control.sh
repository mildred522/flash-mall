#!/usr/bin/env sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
control="$root/scripts/local/flash-mall-control.sh"
output="${TMPDIR:-/tmp}/flash-mall-control-test.$$"
trap 'rm -f "$output"' EXIT

help=$(sh "$control" help)
printf '%s' "$help" | grep -q 'rebuild-service SERVICE'
printf '%s' "$help" | grep -q 'verify-demo'
printf '%s' "$help" | grep -q 'reset-demo --confirm-reset'
grep -q 'umask 077' "$control"
grep -q 'chmod 600 "$backup"' "$control"

if sh "$control" rebuild-service unknown >"$output" 2>&1; then
  echo "unknown service unexpectedly accepted" >&2
  exit 1
fi
grep -q 'invalid_service' "$output"

if sh "$control" status --wait-timeout nope >"$output" 2>&1; then
  echo "invalid timeout unexpectedly accepted" >&2
  exit 1
fi
grep -q 'invalid_timeout' "$output"

if sh "$control" reset-demo >"$output" 2>&1; then
  echo "unconfirmed demo reset unexpectedly accepted" >&2
  exit 1
fi
grep -q 'reset_confirmation_required' "$output"

events=$(FLASH_MALL_CONTROL_SOURCE_ONLY=1 sh -c '. "$1"; emit progress preflight info "quote: \"safe\""' sh "$control")
printf '%s' "$events" | grep -q '"type":"progress"'
printf '%s' "$events" | grep -q 'quote: \\"safe\\"'

tool_path=$(env PATH=/usr/bin:/bin FLASH_MALL_CONTROL_SOURCE_ONLY=1 sh -c '. "$1"; printf "%s" "$PATH"' sh "$control")
printf '%s' "$tool_path" | grep -Fq "$HOME/.local/go/bin"

echo "flash-mall-control protocol tests passed"
