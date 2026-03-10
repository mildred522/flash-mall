param(
  [string]$Namespace = "flash-mall",
  [int64]$ProductId = 100,
  [int]$Shards = 4
)

$ErrorActionPreference = "Stop"

function Invoke-Mysql {
  param([string]$Sql)
  $cmd = "MYSQL_PWD=flashmall mysql -uroot -h 127.0.0.1 -P 3306 -N -s"
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

# CHG 2026-02-24: 变更=新增库存一致性校验脚本; 之前=只能手动对比; 原因=给面试提供可验证证据链。
# CHG 2026-02-24: 变更=优先校验分桶表汇总; 之前=读取 product 单行库存; 原因=分桶扣减后单行库存不再准确。
$bucketExistsRaw = Invoke-Mysql "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='mall_product' AND table_name='product_stock_bucket';"
$bucketExists = [int]($bucketExistsRaw -split "`r?`n" | Select-Object -First 1)
if ($bucketExists -gt 0) {
  $dbStockRaw = Invoke-Mysql "SELECT COALESCE(SUM(stock),0) FROM mall_product.product_stock_bucket WHERE product_id=$ProductId;"
} else {
  $dbStockRaw = Invoke-Mysql "SELECT stock FROM mall_product.product WHERE id=$ProductId;"
}
$dbStock = [int64]($dbStockRaw -split "`r?`n" | Select-Object -First 1)

$keys = 0..($Shards - 1) | ForEach-Object { "stock:${ProductId}:$_" }
$redisOut = kubectl -n $Namespace exec -i $redisPod -- redis-cli MGET @keys
$values = $redisOut -split "`r?`n" | Where-Object { $_ -ne "" }

$missing = 0
$redisSum = 0
foreach ($v in $values) {
  if ($v -eq "(nil)") {
    $missing++
    continue
  }
  $redisSum += [int64]$v
}

Write-Host "db_stock=$dbStock redis_sum=$redisSum missing_keys=$missing"

if ($missing -gt 0) {
  throw "consistency failed: redis keys missing=$missing"
}
if ($dbStock -ne $redisSum) {
  throw "consistency failed: db_stock=$dbStock redis_sum=$redisSum"
}

Write-Host "consistency ok"
