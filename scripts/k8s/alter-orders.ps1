param(
  [string]$Namespace = "flash-mall",
  [string]$MysqlPod = "",
  [string]$RootPassword = "flashmall"
)

$ErrorActionPreference = "Stop"

if ($MysqlPod -eq "") {
  $MysqlPod = kubectl -n $Namespace get pods -l app=mysql -o jsonpath='{.items[0].metadata.name}'
}

Write-Host "Using MySQL pod: $MysqlPod"

$mysqlPasswordArg = "-p$RootPassword"
Get-Content -Raw scripts/k8s/alter-orders.sql | kubectl -n $Namespace exec -i $MysqlPod -- mysql -uroot $mysqlPasswordArg

if ($LASTEXITCODE -ne 0) {
  throw "failed to apply scripts/k8s/alter-orders.sql"
}
