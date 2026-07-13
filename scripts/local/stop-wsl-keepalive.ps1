[CmdletBinding()]
param(
  [string]$Distro = "Ubuntu",
  [string]$PidFile = "/tmp/flash-mall-wsl-keepalive.pid",
  [switch]$Help
)

$ErrorActionPreference = "Stop"

if ($Help) {
  @"
Usage: scripts\local\stop-wsl-keepalive.ps1 [options]

Options:
  -Distro NAME       WSL distro name. Default: Ubuntu.
  -PidFile PATH      Pid file inside WSL. Default: /tmp/flash-mall-wsl-keepalive.pid.
"@ | Write-Host
  exit 0
}

function ConvertTo-BashSingleQuoted {
  param([string]$Value)
  return "'" + ($Value -replace "'", "'\''") + "'"
}

$quotedPidFile = ConvertTo-BashSingleQuoted $PidFile
$stopCommand = @"
if [ -s $quotedPidFile ]; then
  pid=`$(cat $quotedPidFile 2>/dev/null || true)
  if [ -n "`$pid" ] && kill -0 "`$pid" 2>/dev/null; then
    kill "`$pid" 2>/dev/null || true
  fi
  rm -f $quotedPidFile
fi
"@

Write-Host "[WSL] stopping keepalive for $Distro"
& wsl.exe -d $Distro -- bash -lc $stopCommand
exit $LASTEXITCODE
