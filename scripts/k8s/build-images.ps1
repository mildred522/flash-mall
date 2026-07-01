param(
  [string]$Tag = "latest"
)

$ErrorActionPreference = "Stop"

Write-Host "[1/5] build auth-api:$Tag"
docker build -f build/docker/auth-api.Dockerfile -t flash-mall/auth-api:$Tag .

Write-Host "[2/5] build entry-api:$Tag"
docker build -f build/docker/entry-api.Dockerfile -t flash-mall/entry-api:$Tag .

Write-Host "[3/5] build order-rpc:$Tag"
docker build -f build/docker/order-rpc.Dockerfile -t flash-mall/order-rpc:$Tag .

Write-Host "[4/5] build product-rpc:$Tag"
docker build -f build/docker/product-rpc.Dockerfile -t flash-mall/product-rpc:$Tag .

Write-Host "[5/5] build inventory-kitex:$Tag"
docker build -f build/docker/inventory-kitex.Dockerfile -t flash-mall/inventory-kitex:$Tag .
