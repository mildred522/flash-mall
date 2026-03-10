param(
  [string]$Namespace = "flash-mall",
  [string]$NodeName = "flash-mall-worker",
  [string[]]$Apps = @("order-api", "order-rpc", "product-rpc"),
  [switch]$Restore
)

$ErrorActionPreference = "Stop"

if (-not $Apps -or $Apps.Count -eq 0) {
  throw "Apps cannot be empty"
}

$selector = ""
if ($Apps.Count -eq 1) {
  $selector = "app=$($Apps[0])"
} else {
  $selector = "app in (" + ($Apps -join ",") + ")"
}

# CHG 2026-02-26: 变更=新增节点维护演练脚本; 之前=手工 cordon/drain; 原因=可复现实验并留痕。
Write-Host "cordon node: $NodeName"
kubectl cordon $NodeName | Out-Null

Write-Host "drain node: $NodeName selector=[$selector]"
kubectl drain $NodeName --ignore-daemonsets --pod-selector=$selector | Out-Null

Write-Host "pods after drain:"
kubectl -n $Namespace get pods -o wide | Out-String | Write-Host

if ($Restore) {
  Write-Host "uncordon node: $NodeName"
  kubectl uncordon $NodeName | Out-Null
}
