[CmdletBinding()]
param(
  [string]$Distro = "Ubuntu",
  [string]$Workspace = "/home/mildred/code/flash-mall",
  [switch]$NoBuild,
  [switch]$ComposeBuild,
  [switch]$Foreground,
  [switch]$PullDeps,
  [switch]$NoWait,
  [int]$WaitTimeout = 180,
  [switch]$Help
)

$ErrorActionPreference = "Stop"

if ($Help) {
  @"
Usage: scripts\local\start-wsl-compose.ps1 [options]

Options:
  -Distro NAME       WSL distro name. Default: Ubuntu.
  -Workspace PATH    WSL workspace path. Default: /home/mildred/code/flash-mall.
  -NoBuild           Start existing images without rebuilding Go services.
  -ComposeBuild      Build service images through docker compose build.
  -Foreground        Run docker compose up in the foreground.
  -PullDeps          Pull dependency images before startup.
  -NoWait            Do not wait for entry-api health after detached startup.
  -WaitTimeout N     Wait up to N seconds for health. Default: 180.
"@ | Write-Host
  exit 0
}

function ConvertTo-BashSingleQuoted {
  param([string]$Value)
  return "'" + ($Value -replace "'", "'\''") + "'"
}

$scriptArgs = @()
if ($NoBuild) { $scriptArgs += "--no-build" }
if ($ComposeBuild) { $scriptArgs += "--compose-build" }
if ($Foreground) { $scriptArgs += "--foreground" }
if ($PullDeps) { $scriptArgs += "--pull-deps" }
if ($NoWait) { $scriptArgs += "--no-wait" }
if ($WaitTimeout -gt 0) {
  $scriptArgs += "--wait-timeout"
  $scriptArgs += [string]$WaitTimeout
}

$quotedWorkspace = ConvertTo-BashSingleQuoted $Workspace
$quotedArgs = $scriptArgs | ForEach-Object { ConvertTo-BashSingleQuoted $_ }
$bashCommand = "cd $quotedWorkspace && scripts/local/start-compose-all.sh $($quotedArgs -join ' ')"

Write-Host "[WSL] $Distro $bashCommand"
& wsl.exe -d $Distro -- bash -lc $bashCommand
exit $LASTEXITCODE
