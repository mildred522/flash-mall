param(
  [string]$Namespace = "flash-mall",
  [int64]$ProductId = 100,
  [int64]$TotalStock = 10000,
  [int]$Buckets = 4,
  [switch]$Force
)

$ErrorActionPreference = "Stop"

function Invoke-Mysql {
  param([string]$Sql)
  $cmd = "MYSQL_PWD=flashmall mysql -uroot -h 127.0.0.1 -P 3306"
  $out = $Sql | kubectl -n $Namespace exec -i $mysqlPod -- /bin/sh -c $cmd
  if ($LASTEXITCODE -ne 0) {
    throw "mysql exec failed"
  }
  return $out
}

if ($Buckets -le 0) {
  $Buckets = 1
}

kubectl -n $Namespace wait --for=condition=Ready pod -l app=mysql --timeout=60s | Out-Null
$mysqlPod = kubectl -n $Namespace get pods -l app=mysql -o jsonpath='{.items[0].metadata.name}'
if (-not $mysqlPod) {
  throw "MySQL pod not found in namespace $Namespace"
}

# CHG 2026-02-24: 变更=新增库存分桶种子脚本; 之前=仅有单行库存初始化; 原因=分桶扣减需要可重复初始化数据。
$per = [math]::Floor($TotalStock / $Buckets)
$remain = $TotalStock % $Buckets

$sqlLines = @()
$sqlLines += "USE mall_product;"
$sqlLines += "CREATE TABLE IF NOT EXISTS product_stock_bucket (product_id bigint NOT NULL, bucket_idx int NOT NULL, stock int NOT NULL DEFAULT 0, version bigint NOT NULL DEFAULT 0, PRIMARY KEY (product_id, bucket_idx)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;"

if ($Force) {
  $sqlLines += ("INSERT INTO product (id, name, stock, version) VALUES ({0}, 'demo-product', {1}, 0) ON DUPLICATE KEY UPDATE stock={1}, version=0;" -f $ProductId, $TotalStock)
} else {
  $sqlLines += ("INSERT INTO product (id, name, stock, version) VALUES ({0}, 'demo-product', {1}, 0) ON DUPLICATE KEY UPDATE name=VALUES(name);" -f $ProductId, $TotalStock)
}

for ($i = 0; $i -lt $Buckets; $i++) {
  $stock = $per
  if ($i -eq 0) {
    $stock = $per + $remain
  }
  if ($Force) {
    $sqlLines += ("INSERT INTO product_stock_bucket (product_id, bucket_idx, stock, version) VALUES ({0}, {1}, {2}, 0) ON DUPLICATE KEY UPDATE stock={2}, version=0;" -f $ProductId, $i, $stock)
  } else {
    $sqlLines += ("INSERT INTO product_stock_bucket (product_id, bucket_idx, stock, version) VALUES ({0}, {1}, {2}, 0) ON DUPLICATE KEY UPDATE stock=stock, version=version;" -f $ProductId, $i, $stock)
  }
}

Invoke-Mysql ($sqlLines -join "`n") | Out-Null
