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

# CHG 2026-02-24: 变更=用管道传 SQL; 之前=使用 < 重定向导致 PowerShell 报错; 原因=PS 不支持这种重定向语法。
Get-Content -Raw scripts/k8s/init-db.sql | kubectl -n $Namespace exec -i $MysqlPod -- mysql -uroot -p$RootPassword
