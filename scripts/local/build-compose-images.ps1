[CmdletBinding()]
param(
  [string]$Tag = "dev",
  [string[]]$Services = @(),
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

foreach ($service in $Services) {
  if ($service -notin $supportedServices) {
    throw "unknown service: $service"
  }
}

function ConvertTo-BashSingleQuoted {
  param([string]$Value)
  return "'" + ($Value -replace "'", "'\''") + "'"
}

$scriptArgs = @("--tag", $Tag) + $Services
$quotedWorkspace = ConvertTo-BashSingleQuoted $Workspace
$quotedArgs = $scriptArgs | ForEach-Object { ConvertTo-BashSingleQuoted $_ }
$bashCommand = "cd $quotedWorkspace && scripts/local/build-compose-images.sh $($quotedArgs -join ' ')"

Write-Host "[WSL] $Distro $bashCommand"
& wsl.exe -d $Distro -- bash -lc $bashCommand
exit $LASTEXITCODE
