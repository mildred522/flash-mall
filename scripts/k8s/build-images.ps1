param(
  [string]$Tag = "latest"
)

$ErrorActionPreference = "Stop"

Write-Host "[1/3] build order-api:$Tag"
docker build -f build/docker/order-api.Dockerfile -t flash-mall/order-api:$Tag .

Write-Host "[2/3] build order-rpc:$Tag"
docker build -f build/docker/order-rpc.Dockerfile -t flash-mall/order-rpc:$Tag .

Write-Host "[3/3] build product-rpc:$Tag"
docker build -f build/docker/product-rpc.Dockerfile -t flash-mall/product-rpc:$Tag .
