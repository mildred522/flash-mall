param(
  [string]$Tag = "latest"
)

$ErrorActionPreference = "Stop"

Write-Host "[1/4] build auth-api:$Tag"
docker build -f build/docker/auth-api.Dockerfile -t flash-mall/auth-api:$Tag .

Write-Host "[2/4] build entry-api:$Tag"
docker build -f build/docker/entry-api.Dockerfile -t flash-mall/entry-api:$Tag .

Write-Host "[3/4] build order-rpc:$Tag"
docker build -f build/docker/order-rpc.Dockerfile -t flash-mall/order-rpc:$Tag .

Write-Host "[4/4] build product-rpc:$Tag"
docker build -f build/docker/product-rpc.Dockerfile -t flash-mall/product-rpc:$Tag .
