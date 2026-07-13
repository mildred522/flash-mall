#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo_root"

"$repo_root/scripts/kitex/check-tools.sh"

mkdir -p app/generated/kitex

for idl in idl/product.thrift idl/inventory.thrift idl/order.thrift idl/merchant.thrift; do
  name="$(basename "$idl" .thrift)"
  out="app/generated/kitex/$name"
  mkdir -p "$out"
  echo "[GEN] $idl -> $out"
  (cd "$out" && kitex -module flash-mall -service "flash-mall-$name" -I "$repo_root/idl" -gen-path kitex_gen "$repo_root/$idl")
done
