#!/usr/bin/env bash
set -euo pipefail

if command -v go >/dev/null 2>&1; then
  gobin="$(go env GOPATH)/bin"
  export PATH="$gobin:$PATH"
elif [ -x "$HOME/.local/go/bin/go" ]; then
  gobin="$($HOME/.local/go/bin/go env GOPATH)/bin"
  export PATH="$HOME/.local/go/bin:$gobin:$PATH"
fi

missing=0
for tool in kitex thriftgo; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    echo "[MISSING] $tool is not installed or not in PATH" >&2
    missing=1
  else
    echo "[OK] $tool: $(command -v "$tool")"
  fi
done

if [ "$missing" -ne 0 ]; then
  cat >&2 <<MSG

Install the Kitex toolchain before generating service code:
  go install github.com/cloudwego/kitex/tool/cmd/kitex@latest
  go install github.com/cloudwego/thriftgo@latest

MSG
  exit 1
fi
