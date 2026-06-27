#!/usr/bin/env sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
namespace="${1:-${FLASH_MALL_K8S_NAMESPACE:-flash-mall}}"

"$script_dir/restore-port-forward.sh" --namespace "$namespace"
