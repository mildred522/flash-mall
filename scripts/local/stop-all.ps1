param(
  [switch]$WithDeps
)

$ErrorActionPreference = "Stop"

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
$runtimeDir = Join-Path $repoRoot ".runtime"
$pidFile = Join-Path $runtimeDir "local-services.json"
$composeFile = Join-Path $repoRoot "deploy\docker-compose.yml"

function Get-ComposeCommand {
  if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
    throw "docker not found in PATH"
  }
  & docker compose version *> $null
  if ($LASTEXITCODE -eq 0) {
    return @("docker", "compose")
  }
  if (Get-Command docker-compose -ErrorAction SilentlyContinue) {
    return @("docker-compose")
  }
  throw "docker compose not available (need docker compose plugin or docker-compose)"
}

function Invoke-Compose {
  param(
    [string[]]$Command,
    [string[]]$ComposeArgs
  )
  if ($Command.Count -eq 2 -and $Command[0] -eq "docker" -and $Command[1] -eq "compose") {
    & docker compose @ComposeArgs
  } else {
    & $Command[0] @ComposeArgs
  }
  if ($LASTEXITCODE -ne 0) {
    throw "docker compose command failed: $($ComposeArgs -join ' ')"
  }
}

if (Test-Path $pidFile) {
  $items = Get-Content -Raw $pidFile | ConvertFrom-Json
  foreach ($svc in $items) {
    $proc = Get-Process -Id $svc.pid -ErrorAction SilentlyContinue
    if ($proc) {
      Write-Host "[STOP] $($svc.name) pid=$($svc.pid)"
      Stop-Process -Id $svc.pid -Force -ErrorAction SilentlyContinue
    } else {
      Write-Host "[SKIP] $($svc.name) pid=$($svc.pid) not running"
    }
  }
  Remove-Item $pidFile -Force -ErrorAction SilentlyContinue
} else {
  Write-Host "No local pid file found: $pidFile"
}

if ($WithDeps) {
  $composeCmd = Get-ComposeCommand
  Write-Host "[STOP] docker compose dependencies"
  Invoke-Compose -Command $composeCmd -ComposeArgs @("-f", $composeFile, "down")
}

Write-Host "Done."
