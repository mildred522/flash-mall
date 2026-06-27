param(
  [string]$Namespace = "flash-mall"
)

$ErrorActionPreference = "Stop"

Write-Host "Forward entry-api to localhost:8888"
kubectl -n $Namespace port-forward svc/entry-api 8888:8888
