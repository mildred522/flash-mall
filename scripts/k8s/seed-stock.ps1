param(
  [string]$Namespace = "flash-mall",
  [int64]$ProductId = 100,
  [int64]$TotalStock = 10000,
  [int]$Shards = 4,
  [switch]$Force
)

$ErrorActionPreference = "Stop"

if ($Shards -le 0) {
  $Shards = 1
}

kubectl -n $Namespace wait --for=condition=Ready pod -l app=redis --timeout=60s | Out-Null
$redisPod = kubectl -n $Namespace get pods -l app=redis -o jsonpath='{.items[0].metadata.name}'
if (-not $redisPod) {
  throw "Redis pod not found in namespace $Namespace"
}

$per = [math]::Floor($TotalStock / $Shards)
$remain = $TotalStock % $Shards

for ($i = 0; $i -lt $Shards; $i++) {
  $value = $per
  if ($i -eq 0) {
    $value = $per + $remain
  }
  $key = "stock:${ProductId}:$i"

  if ($Force) {
    # CHG 2026-02-24: 变更=支持强制覆盖库存; 之前=仅 SETNX 不覆盖; 原因=便于演示/重置库存。
    kubectl -n $Namespace exec -i $redisPod -- redis-cli SET $key $value | Out-Null
  } else {
    # CHG 2026-02-24: 变更=默认仅在 key 不存在时写入; 之前=无脚本需手动 MSET; 原因=避免误覆盖已有库存。
    kubectl -n $Namespace exec -i $redisPod -- redis-cli SETNX $key $value | Out-Null
  }
}

$keys = 0..($Shards - 1) | ForEach-Object { "stock:${ProductId}:$_" }
kubectl -n $Namespace exec -i $redisPod -- redis-cli MGET @keys
