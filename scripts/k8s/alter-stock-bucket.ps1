param(
  [string]$Namespace = "flash-mall",
  [int64]$ProductId = 100,
  [int]$Buckets = 4
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

kubectl -n $Namespace wait --for=condition=Ready pod -l app=mysql --timeout=60s | Out-Null
$mysqlPod = kubectl -n $Namespace get pods -l app=mysql -o jsonpath='{.items[0].metadata.name}'
if (-not $mysqlPod) {
  throw "MySQL pod not found in namespace $Namespace"
}

# CHG 2026-02-24: 变更=新增分桶迁移脚本; 之前=存量库需手工建表与分桶; 原因=便于平滑切换。
$stockRaw = Invoke-Mysql "SELECT stock FROM mall_product.product WHERE id=$ProductId;"
if (-not $stockRaw) {
  throw "product $ProductId not found in mall_product.product"
}
$stock = [int64]($stockRaw -split "`r?`n" | Select-Object -First 1)

& "$PSScriptRoot/seed-stock-bucket.ps1" -Namespace $Namespace -ProductId $ProductId -TotalStock $stock -Buckets $Buckets -Force | Out-Null
