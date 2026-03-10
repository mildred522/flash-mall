param(
  [string]$Namespace = "flash-mall",
  [string]$Url = "http://localhost:8888/api/order/create",
  [int64]$ProductId = 100,
  [int]$Shards = 4,
  [int64]$RedisStock = 10,
  [int64]$DbStock = 0,
  [int64]$Amount = 1
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

if ($Shards -le 0) {
  $Shards = 1
}

kubectl -n $Namespace wait --for=condition=Ready pod -l app=mysql --timeout=60s | Out-Null
$mysqlPod = kubectl -n $Namespace get pods -l app=mysql -o jsonpath='{.items[0].metadata.name}'
if (-not $mysqlPod) {
  throw "MySQL pod not found in namespace $Namespace"
}

kubectl -n $Namespace wait --for=condition=Ready pod -l app=redis --timeout=60s | Out-Null
$redisPod = kubectl -n $Namespace get pods -l app=redis -o jsonpath='{.items[0].metadata.name}'
if (-not $redisPod) {
  throw "Redis pod not found in namespace $Namespace"
}

# CHG 2026-02-24: 变更=新增 SAGA 失败演练脚本; 之前=只能口述; 原因=可复现实验并展示补偿链路。
# CHG 2026-02-24: 变更=同步初始化分桶库存; 之前=仅初始化 product 单行库存; 原因=分桶扣减需要对应数据。
& "$PSScriptRoot/seed-stock-bucket.ps1" -Namespace $Namespace -ProductId $ProductId -TotalStock $DbStock -Buckets $Shards -Force | Out-Null
& "$PSScriptRoot/seed-stock.ps1" -Namespace $Namespace -ProductId $ProductId -TotalStock $RedisStock -Shards $Shards -Force | Out-Null

$requestId = "fail-" + [guid]::NewGuid().ToString("N")
$body = @{user_id=1; product_id=$ProductId; amount=$Amount; request_id=$requestId} | ConvertTo-Json -Compress

try {
  Invoke-RestMethod -Uri $Url -Method Post -ContentType "application/json" -Body $body | Out-Null
  Write-Host "unexpected success"
} catch {
  Write-Host "expected failure: $($_.Exception.Message)"
}

Start-Sleep -Seconds 2

$orderSql = "USE mall_order; SELECT id,status,request_id FROM orders WHERE request_id='$requestId';"
Write-Host "id` tstatus` trequest_id".Replace("`t", "	")
Invoke-Mysql $orderSql -NoHeader

$keys = 0..($Shards - 1) | ForEach-Object { "stock:${ProductId}:$_" }
$redisOut = kubectl -n $Namespace exec -i $redisPod -- redis-cli MGET @keys
$values = $redisOut -split "`r?`n" | Where-Object { $_ -ne "" }
$redisSum = 0
foreach ($v in $values) {
  if ($v -eq "(nil)") { continue }
  $redisSum += [int64]$v
}
Write-Host "redis_sum=$redisSum (expect=$RedisStock) db_stock=$DbStock"
