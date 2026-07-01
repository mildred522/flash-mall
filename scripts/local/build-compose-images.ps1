param(
  [string]$Tag = "dev"
)

$ErrorActionPreference = "Stop"

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
$contextRoot = Join-Path $repoRoot ".runtime\docker-context"
$dockerfile = Join-Path $repoRoot "build\docker\local-binary.Dockerfile"

$services = @(
  @{ Name = "auth-api"; Package = "./app/auth/api" },
  @{ Name = "product-rpc"; Package = "./app/product/rpc" },
  @{ Name = "order-rpc"; Package = "./app/order/rpc" },
  @{ Name = "inventory-kitex"; Package = "./app/inventory/kitex" },
  @{ Name = "entry-api"; Package = "./app/entry/api" }
)

New-Item -ItemType Directory -Force -Path $contextRoot | Out-Null

$oldGOOS = $env:GOOS
$oldGOARCH = $env:GOARCH
$oldCGO = $env:CGO_ENABLED
try {
  $env:GOOS = "linux"
  $env:GOARCH = "amd64"
  $env:CGO_ENABLED = "0"

  foreach ($svc in $services) {
    $svcContext = Join-Path $contextRoot $svc.Name
    New-Item -ItemType Directory -Force -Path $svcContext | Out-Null
    $binaryPath = Join-Path $svcContext "app"

    Write-Host "[GO BUILD] $($svc.Name)"
    & go build -trimpath -tags timetzdata -o $binaryPath $svc.Package
    if ($LASTEXITCODE -ne 0) {
      throw "go build failed for $($svc.Name)"
    }

    Write-Host "[DOCKER BUILD] flash-mall/$($svc.Name):$Tag"
    & docker buildx version *> $null
    if ($LASTEXITCODE -eq 0) {
      & docker buildx build --load -f $dockerfile -t "flash-mall/$($svc.Name):$Tag" $svcContext
    } else {
      & docker build -f $dockerfile -t "flash-mall/$($svc.Name):$Tag" $svcContext
    }
    if ($LASTEXITCODE -ne 0) {
      throw "docker build failed for $($svc.Name)"
    }
  }
} finally {
  $env:GOOS = $oldGOOS
  $env:GOARCH = $oldGOARCH
  $env:CGO_ENABLED = $oldCGO
}
