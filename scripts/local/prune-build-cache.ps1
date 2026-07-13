[CmdletBinding()]
param(
  [switch]$Milestone,
  [string]$Distro = "Ubuntu",
  [string]$Workspace = "/home/mildred/code/flash-mall"
)

$ErrorActionPreference = "Stop"

function ConvertTo-BashSingleQuoted {
  param([string]$Value)
  return "'" + ($Value -replace "'", "'\''") + "'"
}

$quotedWorkspace = ConvertTo-BashSingleQuoted $Workspace
$mode = if ($Milestone) { " --milestone" } else { "" }
$bashCommand = "cd $quotedWorkspace && scripts/local/prune-build-cache.sh$mode"

Write-Host "[WSL] $Distro $bashCommand"
& wsl.exe -d $Distro -- bash -lc $bashCommand
exit $LASTEXITCODE
