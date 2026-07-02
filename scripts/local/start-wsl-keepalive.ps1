[CmdletBinding()]
param(
  [string]$Distro = "Ubuntu",
  [string]$Workspace = "/home/mildred/code/flash-mall",
  [string]$PidFile = "/tmp/flash-mall-wsl-keepalive.pid",
  [switch]$Help
)

$ErrorActionPreference = "Stop"

if ($Help) {
  @"
Usage: scripts\local\start-wsl-keepalive.ps1 [options]

Options:
  -Distro NAME       WSL distro name. Default: Ubuntu.
  -Workspace PATH    WSL workspace path. Default: /home/mildred/code/flash-mall.
  -PidFile PATH      Pid file inside WSL. Default: /tmp/flash-mall-wsl-keepalive.pid.
"@ | Write-Host
  exit 0
}

function ConvertTo-BashSingleQuoted {
  param([string]$Value)
  return "'" + ($Value -replace "'", "'\''") + "'"
}

$quotedPidFile = ConvertTo-BashSingleQuoted $PidFile
$checkCommand = "if [ -s $quotedPidFile ] && kill -0 `$(cat $quotedPidFile) 2>/dev/null; then exit 0; fi; exit 1"
& wsl.exe -d $Distro -- bash -lc $checkCommand
if ($LASTEXITCODE -eq 0) {
  Write-Host "[WSL] keepalive already running"
  exit 0
}

$quotedWorkspace = ConvertTo-BashSingleQuoted $Workspace
$startCommand = "cd $quotedWorkspace && chmod +x scripts/local/wsl-keepalive.sh && exec scripts/local/wsl-keepalive.sh $quotedPidFile"
$quotedStartCommand = ConvertTo-BashSingleQuoted $startCommand
Write-Host "[WSL] starting keepalive for $Distro"
Start-Process -WindowStyle Hidden -FilePath "wsl.exe" -ArgumentList @("-d", $Distro, "--", "bash", "-lc", $quotedStartCommand)

Start-Sleep -Seconds 1
& wsl.exe -d $Distro -- bash -lc $checkCommand
if ($LASTEXITCODE -ne 0) {
  throw "failed to start WSL keepalive"
}

Write-Host "[WSL] keepalive started"
