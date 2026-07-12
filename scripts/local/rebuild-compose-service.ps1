[CmdletBinding()]
param(
  [Parameter(Mandatory = $true)]
  [string]$Service,
  [string]$Tag = "dev",
  [string]$Distro = "Ubuntu",
  [string]$Workspace = "/home/mildred/code/flash-mall"
)

$ErrorActionPreference = "Stop"
$supportedServices = @(
  "auth-api",
  "product-rpc",
  "order-rpc",
  "inventory-kitex",
  "entry-api",
  "hertz-gateway"
)
if ($Service -notin $supportedServices) {
  throw "unknown service: $Service"
}

function ConvertTo-BashSingleQuoted {
  param([string]$Value)
  return "'" + ($Value -replace "'", "'\''") + "'"
}

$quotedWorkspace = ConvertTo-BashSingleQuoted $Workspace
$quotedTag = ConvertTo-BashSingleQuoted $Tag
$quotedService = ConvertTo-BashSingleQuoted $Service
$bashCommand = "cd $quotedWorkspace && scripts/local/rebuild-compose-service.sh --tag $quotedTag $quotedService"

Write-Host "[WSL] $Distro $bashCommand"
& wsl.exe -d $Distro -- bash -lc $bashCommand
exit $LASTEXITCODE
