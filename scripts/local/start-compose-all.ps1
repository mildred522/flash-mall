param(
  [switch]$NoBuild,
  [switch]$ComposeBuild,
  [switch]$Foreground
)

$ErrorActionPreference = "Stop"

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
$composeFile = Join-Path $repoRoot "deploy\docker-compose.yml"

function Set-DefaultEnv {
  param(
    [string]$Name,
    [string]$Value
  )

  if ([string]::IsNullOrWhiteSpace([Environment]::GetEnvironmentVariable($Name, "Process"))) {
    [Environment]::SetEnvironmentVariable($Name, $Value, "Process")
  }
}

Set-DefaultEnv -Name "FLASH_MALL_MYSQL_ROOT_PASSWORD" -Value "6494kj06"
Set-DefaultEnv -Name "FLASH_MALL_JWT_AUTH_SECRET" -Value "flash-mall-local-jwt-secret"
Set-DefaultEnv -Name "FLASH_MALL_PAYMENT_CALLBACK_SECRET" -Value "flash-mall-local-payment-secret"
Set-DefaultEnv -Name "FLASH_MALL_DEMO_PASSWORD" -Value "flashmall123"
Set-DefaultEnv -Name "FLASH_MALL_RABBITMQ_USER" -Value "flashmall"
Set-DefaultEnv -Name "FLASH_MALL_RABBITMQ_PASSWORD" -Value "flashmall-local"
Set-DefaultEnv -Name "FLASH_MALL_IMAGE_TAG" -Value "dev"

if (-not $NoBuild) {
  if ($ComposeBuild) {
    & docker compose -f $composeFile build auth-api product-rpc order-rpc inventory-kitex entry-api
    if ($LASTEXITCODE -ne 0) {
      throw "docker compose image build failed"
    }
  } else {
    $buildScript = Join-Path $PSScriptRoot "build-compose-images.ps1"
    & $buildScript -Tag $env:FLASH_MALL_IMAGE_TAG
    if ($LASTEXITCODE -ne 0) {
      throw "local compose image build failed"
    }
  }
}
$args = @("compose", "-f", $composeFile, "up", "--no-build")
if (-not $Foreground) {
  $args += "-d"
}

Write-Host "[COMPOSE] docker $($args -join ' ')"
& docker @args
if ($LASTEXITCODE -ne 0) {
  throw "docker compose startup failed"
}

Write-Host ""
Write-Host "entry-api: http://127.0.0.1:8888"
Write-Host "auth-api:  http://127.0.0.1:8890"
Write-Host "rabbitmq:  http://127.0.0.1:15672"
