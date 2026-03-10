param(
  [string]$Namespace = "flash-mall",
  [int64]$ProductId = 100,
  [int]$Shards = 4,
  [switch]$Repair
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

# CHG 2026-02-26: 变更=新增库存对账与修复脚本; 之前=只能人工比对; 原因=形成可验证的一致性闭环。
$bucketExistsRaw = Invoke-Mysql "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='mall_product' AND table_name='product_stock_bucket';" -NoHeader
$bucketExists = [int]($bucketExistsRaw -split "`r?`n" | Select-Object -First 1)

$bucketSum = $null
if ($bucketExists -gt 0) {
  $bucketSumRaw = Invoke-Mysql "SELECT COALESCE(SUM(stock),0) FROM mall_product.product_stock_bucket WHERE product_id=$ProductId;" -NoHeader
  $bucketSum = [int64]($bucketSumRaw -split "`r?`n" | Select-Object -First 1)
}

$productStockRaw = Invoke-Mysql "SELECT stock FROM mall_product.product WHERE id=$ProductId;" -NoHeader
$productStockRaw = $productStockRaw -split "`r?`n" | Select-Object -First 1
if (-not $productStockRaw) {
  throw "product $ProductId not found in mall_product.product"
}
$productStock = [int64]$productStockRaw

$keys = 0..($Shards - 1) | ForEach-Object { "stock:${ProductId}:$_" }
$redisOut = kubectl -n $Namespace exec -i $redisPod -- redis-cli MGET @keys
$values = $redisOut -split "`r?`n" | Where-Object { $_ -ne "" }
$redisSum = 0
$missing = 0
foreach ($v in $values) {
  if ($v -eq "(nil)") {
    $missing++
    continue
  }
  $redisSum += [int64]$v
}

$bucketDisplay = if ($bucketSum -eq $null) { "NA" } else { "$bucketSum" }
Write-Host "bucket_sum=$bucketDisplay product_stock=$productStock redis_sum=$redisSum missing_keys=$missing"

if (-not $Repair) {
  return
}

if ($bucketExists -gt 0) {
  # CHG 2026-02-26: 变更=以分桶表为权威修复 product/redis; 之前=无自动修复; 原因=扣减逻辑以分桶为准。
  $truth = $bucketSum
  Invoke-Mysql "UPDATE mall_product.product SET stock=$truth, version=version+1 WHERE id=$ProductId;" | Out-Null
  & "$PSScriptRoot/seed-stock.ps1" -Namespace $Namespace -ProductId $ProductId -TotalStock $truth -Shards $Shards -Force | Out-Null
  Write-Host "repair done: source=bucket sum=$truth"
} else {
  # CHG 2026-02-26: 变更=分桶表缺失时以 product 为权威重建; 之前=需手工补表; 原因=保证可恢复。
  $truth = $productStock
  & "$PSScriptRoot/seed-stock-bucket.ps1" -Namespace $Namespace -ProductId $ProductId -TotalStock $truth -Buckets $Shards -Force | Out-Null
  & "$PSScriptRoot/seed-stock.ps1" -Namespace $Namespace -ProductId $ProductId -TotalStock $truth -Shards $Shards -Force | Out-Null
  Write-Host "repair done: source=product sum=$truth"
}
