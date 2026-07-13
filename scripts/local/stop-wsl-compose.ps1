[CmdletBinding()]
param(
  [string]$Distro = "Ubuntu",
  [string]$Workspace = "/home/mildred/code/flash-mall",
  [switch]$Volumes,
  [switch]$KeepAlive,
  [switch]$Help
)

$ErrorActionPreference = "Stop"

if ($Help) {
  @"
Usage: scripts\local\stop-wsl-compose.ps1 [options]

Options:
  -Distro NAME       WSL distro name. Default: Ubuntu.
  -Workspace PATH    WSL workspace path. Default: /home/mildred/code/flash-mall.
  -Volumes           Also remove compose volumes, including local MySQL data.
  -KeepAlive         Leave the WSL keepalive process running after compose down.
"@ | Write-Host
  exit 0
}

function ConvertTo-BashSingleQuoted {
  param([string]$Value)
  return "'" + ($Value -replace "'", "'\''") + "'"
}

$scriptArgs = @()
if ($Volumes) { $scriptArgs += "--volumes" }

$quotedWorkspace = ConvertTo-BashSingleQuoted $Workspace
$quotedArgs = $scriptArgs | ForEach-Object { ConvertTo-BashSingleQuoted $_ }
$bashCommand = "cd $quotedWorkspace && scripts/local/stop-compose-all.sh $($quotedArgs -join ' ')"

Write-Host "[WSL] $Distro $bashCommand"
& wsl.exe -d $Distro -- bash -lc $bashCommand
$exitCode = $LASTEXITCODE

if ($exitCode -eq 0 -and -not $KeepAlive) {
  & (Join-Path $PSScriptRoot "stop-wsl-keepalive.ps1") -Distro $Distro
  $exitCode = $LASTEXITCODE
}

exit $exitCode
