param(
  [string]$Tag = "latest"
)

$ErrorActionPreference = "Stop"

Write-Host "[1/6] build auth-api:$Tag"
docker build -f build/docker/auth-api.Dockerfile -t flash-mall/auth-api:$Tag .

Write-Host "[2/6] build entry-api:$Tag"
docker build -f build/docker/entry-api.Dockerfile -t flash-mall/entry-api:$Tag .

Write-Host "[3/6] build order-rpc:$Tag"
docker build -f build/docker/order-rpc.Dockerfile -t flash-mall/order-rpc:$Tag .

Write-Host "[4/6] build product-rpc:$Tag"
docker build -f build/docker/product-rpc.Dockerfile -t flash-mall/product-rpc:$Tag .

Write-Host "[5/6] build inventory-kitex:$Tag"
docker build -f build/docker/inventory-kitex.Dockerfile -t flash-mall/inventory-kitex:$Tag .

Write-Host "[6/6] build hertz-gateway:$Tag"
docker build -f build/docker/hertz-gateway.Dockerfile -t flash-mall/hertz-gateway:$Tag .
