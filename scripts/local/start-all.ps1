param(
  [switch]$SkipCompose,
  [switch]$SkipDbInit,
  [switch]$SkipSeedStock,
  [switch]$NoBrowser,
  [int]$PortWaitSeconds = 90
)

$ErrorActionPreference = "Stop"

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
$composeFile = Join-Path $repoRoot "deploy\docker-compose.yml"
$runtimeDir = Join-Path $repoRoot ".runtime"
$pidFile = Join-Path $runtimeDir "local-services.json"
$logDir = Join-Path $repoRoot "logs\local"
$runStamp = Get-Date -Format "yyyyMMdd-HHmmss"

function Test-TcpPort {
  param(
    [string]$TargetHost,
    [int]$Port,
    [int]$TimeoutMs = 1500
  )

  $client = New-Object System.Net.Sockets.TcpClient
  try {
    $iar = $client.BeginConnect($TargetHost, $Port, $null, $null)
    if (-not $iar.AsyncWaitHandle.WaitOne($TimeoutMs, $false)) {
      return $false
    }
    $null = $client.EndConnect($iar)
    return $true
  } catch {
    return $false
  } finally {
    $client.Close()
  }
}

function Wait-TcpPort {
  param(
    [string]$Name,
    [string]$TargetHost,
    [int]$Port,
    [int]$TimeoutSeconds = 90,
    [int]$ProcessId = 0,
    [string]$ErrorLog = ""
  )

  $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
  while ((Get-Date) -lt $deadline) {
    if (Test-TcpPort -TargetHost $TargetHost -Port $Port) {
      Write-Host "[OK] $Name is ready at $TargetHost`:$Port"
      return
    }
    if ($ProcessId -gt 0) {
      $proc = Get-Process -Id $ProcessId -ErrorAction SilentlyContinue
      if (-not $proc) {
        if ($ErrorLog -and (Test-Path $ErrorLog)) {
          $tail = (Get-Content $ErrorLog -Tail 8 -ErrorAction SilentlyContinue) -join [Environment]::NewLine
          throw "$Name exited before becoming ready. See $ErrorLog`n$tail"
        }
        throw "$Name exited before becoming ready on $TargetHost`:$Port"
      }
    }
    Start-Sleep -Seconds 1
  }
  throw "$Name is not ready on $TargetHost`:$Port within ${TimeoutSeconds}s"
}

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

function Ensure-DockerDaemon {
  $oldPref = $ErrorActionPreference
  try {
    $ErrorActionPreference = "Continue"
    & docker info *> $null
  } finally {
    $ErrorActionPreference = $oldPref
  }
  if ($LASTEXITCODE -ne 0) {
    throw "docker daemon is not running. Please start Docker Desktop first."
  }
}

function Wait-MySQLReady {
  param(
    [int]$TimeoutSeconds = 120
  )

  $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
  while ((Get-Date) -lt $deadline) {
    $oldPref = $ErrorActionPreference
    try {
      $ErrorActionPreference = "Continue"
      & docker exec mysql mysqladmin ping -uroot -p6494kj06 --silent *> $null
    } finally {
      $ErrorActionPreference = $oldPref
    }
    if ($LASTEXITCODE -eq 0) {
      Write-Host "[OK] mysql is accepting queries"
      return
    }
    Start-Sleep -Seconds 2
  }
  throw "mysql is not ready for queries within ${TimeoutSeconds}s"
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

function Stop-StaleServices {
  param([string]$PidFilePath)

  if (-not (Test-Path $PidFilePath)) {
    return
  }
  try {
    $stale = Get-Content -Raw $PidFilePath | ConvertFrom-Json
    foreach ($svc in $stale) {
      $proc = Get-Process -Id $svc.pid -ErrorAction SilentlyContinue
      if ($proc) {
        Write-Host "[CLEANUP] stop stale process $($svc.name) pid=$($svc.pid)"
        Stop-Process -Id $svc.pid -Force -ErrorAction SilentlyContinue
      }
    }
  } finally {
    Remove-Item $PidFilePath -Force -ErrorAction SilentlyContinue
  }
}

function Start-GoService {
  param(
    [string]$Name,
    [string]$Entry,
    [string]$Config
  )

  $outLog = Join-Path $logDir "$Name.$runStamp.out.log"
  $errLog = Join-Path $logDir "$Name.$runStamp.err.log"

  $proc = Start-Process `
    -FilePath "go" `
    -ArgumentList @("run", $Entry, "-f", $Config) `
    -WorkingDirectory $repoRoot `
    -RedirectStandardOutput $outLog `
    -RedirectStandardError $errLog `
    -PassThru

  Write-Host "[START] $Name pid=$($proc.Id)"
  return @{
    name = $Name
    pid = $proc.Id
    out = $outLog
    err = $errLog
  }
}

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
  throw "go not found in PATH"
}

New-Item -ItemType Directory -Path $runtimeDir -Force | Out-Null
New-Item -ItemType Directory -Path $logDir -Force | Out-Null
Set-Location $repoRoot

Stop-StaleServices -PidFilePath $pidFile

$composeCmd = $null
if (-not $SkipCompose) {
  $composeCmd = Get-ComposeCommand
  Ensure-DockerDaemon
  Write-Host "[STEP] starting dependencies with docker compose"
  Invoke-Compose -Command $composeCmd -ComposeArgs @("-f", $composeFile, "up", "-d", "etcd", "mysql", "redis", "dtm", "rabbitmq")

  Wait-TcpPort -Name "etcd" -TargetHost "127.0.0.1" -Port 2379 -TimeoutSeconds $PortWaitSeconds
  Wait-TcpPort -Name "mysql" -TargetHost "127.0.0.1" -Port 3306 -TimeoutSeconds $PortWaitSeconds
  Wait-TcpPort -Name "redis" -TargetHost "127.0.0.1" -Port 6379 -TimeoutSeconds $PortWaitSeconds
  Wait-TcpPort -Name "dtm" -TargetHost "127.0.0.1" -Port 36790 -TimeoutSeconds $PortWaitSeconds
  Wait-TcpPort -Name "rabbitmq" -TargetHost "127.0.0.1" -Port 5672 -TimeoutSeconds $PortWaitSeconds
}

if (-not $SkipDbInit) {
  $sqlFile = Join-Path $repoRoot "scripts\k8s\init-db.sql"
  Write-Host "[STEP] initialize mysql schema"
  Wait-MySQLReady -TimeoutSeconds $PortWaitSeconds
  Get-Content -Raw $sqlFile | & docker exec -i mysql mysql --force -uroot -p6494kj06
  if ($LASTEXITCODE -ne 0) {
    Write-Warning "mysql init returned non-zero. Existing schema may already exist; continue startup."
  }
}

if (-not $SkipSeedStock) {
  Write-Host "[STEP] seed redis stock shards"
  & go run ./app/order/api/scripts/seed/seed_stock.go -product 100 -stock 10000 -shards 4
  if ($LASTEXITCODE -ne 0) {
    throw "redis stock seed failed"
  }
}

$procs = @()
try {
  $svc = Start-GoService -Name "product-rpc" -Entry "./app/product/rpc/product.go" -Config "./app/product/rpc/etc/product.yaml"
  $procs += $svc
  Wait-TcpPort -Name "product-rpc" -TargetHost "127.0.0.1" -Port 8080 -TimeoutSeconds $PortWaitSeconds -ProcessId $svc.pid -ErrorLog $svc.err

  $svc = Start-GoService -Name "order-rpc" -Entry "./app/order/rpc/order.go" -Config "./app/order/rpc/etc/order.yaml"
  $procs += $svc
  Wait-TcpPort -Name "order-rpc" -TargetHost "127.0.0.1" -Port 8090 -TimeoutSeconds $PortWaitSeconds -ProcessId $svc.pid -ErrorLog $svc.err

  $svc = Start-GoService -Name "order-api" -Entry "./app/order/api/order.go" -Config "./app/order/api/etc/order-api.yaml"
  $procs += $svc
  Wait-TcpPort -Name "order-api" -TargetHost "127.0.0.1" -Port 8888 -TimeoutSeconds $PortWaitSeconds -ProcessId $svc.pid -ErrorLog $svc.err

  $procs | ConvertTo-Json | Set-Content -Encoding UTF8 $pidFile
} catch {
  foreach ($svc in $procs) {
    $proc = Get-Process -Id $svc.pid -ErrorAction SilentlyContinue
    if ($proc) {
      Stop-Process -Id $svc.pid -Force -ErrorAction SilentlyContinue
    }
  }
  throw
}

Write-Host ""
Write-Host "========== Flash Mall Ready =========="
Write-Host "UI:          http://127.0.0.1:8888/"
Write-Host "Health API:  http://127.0.0.1:8888/api/system/health"
Write-Host "Metrics:     http://127.0.0.1:9090/metrics"
Write-Host "PProf:       http://127.0.0.1:6060/debug/pprof/"
Write-Host "PID file:    $pidFile"
Write-Host "Logs:        $logDir"
Write-Host "Stop cmd:    powershell -ExecutionPolicy Bypass -File scripts/local/stop-all.ps1 -WithDeps"
Write-Host "======================================"

if (-not $NoBrowser) {
  Start-Process "http://127.0.0.1:8888/"
}
