param(
  [string]$Namespace = "flash-mall",
  [string]$Url = "http://localhost:8888/api/order/create",
  [int64]$ProductId = 100,
  [int64]$Amount = 1,
  [string]$RequestId = "",
  [switch]$PrepareData
)

$ErrorActionPreference = "Stop"

function Invoke-Mysql {
  param([string]$Sql, [switch]$NoHeader)
  $cmd = "MYSQL_PWD=flashmall mysql -uroot -h 127.0.0.1 -P 3306"
  if ($NoHeader) {
    $cmd += " -N -s"
  }
  $out = $Sql | kubectl -n $Namespace exec -i $mysqlPod -- /bin/sh -c $cmd
  if ($LASTEXITCODE -ne 0) {
    throw "mysql exec failed"
  }
  return $out
}

if (-not $RequestId) {
  $RequestId = "idem-" + [guid]::NewGuid().ToString("N")
}

kubectl -n $Namespace wait --for=condition=Ready pod -l app=mysql --timeout=60s | Out-Null
$mysqlPod = kubectl -n $Namespace get pods -l app=mysql -o jsonpath='{.items[0].metadata.name}'
if (-not $mysqlPod) {
  throw "MySQL pod not found in namespace $Namespace"
}

if ($PrepareData) {
  # CHG 2026-02-24: 变更=幂等验证前自动准备商品库存; 之前=手动执行 SQL + MSET; 原因=一键可重复验证。
  & "$PSScriptRoot/seed-stock-bucket.ps1" -Namespace $Namespace -ProductId $ProductId -TotalStock 10000 -Buckets 4 -Force | Out-Null
  & "$PSScriptRoot/seed-stock.ps1" -Namespace $Namespace -ProductId $ProductId -TotalStock 10000 -Shards 4 -Force | Out-Null
}

$body = @{user_id=1; product_id=$ProductId; amount=$Amount; request_id=$RequestId} | ConvertTo-Json -Compress

try {
  $resp1 = Invoke-RestMethod -Uri $Url -Method Post -ContentType "application/json" -Body $body
  $resp2 = Invoke-RestMethod -Uri $Url -Method Post -ContentType "application/json" -Body $body
} catch {
  throw "request failed: $($_.Exception.Message)"
}

if ($resp1.order_id -ne $resp2.order_id) {
  throw "idempotency failed: first=$($resp1.order_id) second=$($resp2.order_id)"
}

Write-Host "idempotency ok: order_id=$($resp1.order_id)"

$sql = "USE mall_order; SELECT id,status,request_id FROM orders WHERE request_id='$RequestId';"
Write-Host "id` tstatus` trequest_id".Replace("`t", "	")
Invoke-Mysql $sql -NoHeader
