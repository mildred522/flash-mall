param(
  [string]$Namespace = "flash-mall"
)

$ErrorActionPreference = "Stop"

Write-Host "Forward order-api to localhost:8888"
kubectl -n $Namespace port-forward svc/order-api 8888:8888
