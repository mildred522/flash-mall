[CmdletBinding()]
param(
  [string]$Distro = "Ubuntu",
  [string]$Workspace = "/home/mildred/code/flash-mall",
  [switch]$Volumes,
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
exit $LASTEXITCODE
