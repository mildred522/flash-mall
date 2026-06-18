param(
  [string]$RepoRoot = ""
)

$ErrorActionPreference = "Stop"

if ($RepoRoot -eq "") {
  $RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
}

$startScript = Join-Path $RepoRoot "scripts\local\start-all.ps1"
$launcherScript = Join-Path $RepoRoot "scripts\local\launcher.ps1"

function Assert-True {
  param(
    [bool]$Condition,
    [string]$Message
  )
  if (-not $Condition) {
    throw $Message
  }
}

function Assert-Contains {
  param(
    [object[]]$Items,
    [string]$Expected,
    [string]$Message
  )
  Assert-True -Condition ($Items -contains $Expected) -Message $Message
}

function Assert-ScriptParses {
  param([string]$Path)

  Assert-True -Condition (Test-Path $Path) -Message "Script not found: $Path"
  $tokens = $null
  $errors = $null
  [System.Management.Automation.Language.Parser]::ParseFile($Path, [ref]$tokens, [ref]$errors) | Out-Null
  Assert-True -Condition ($errors.Count -eq 0) -Message "Parse errors in $Path`: $($errors | Out-String)"
}

function Get-PwshPath {
  $pwsh = Get-Command pwsh -ErrorAction SilentlyContinue
  if ($pwsh) {
    return $pwsh.Source
  }
  return (Get-Process -Id $PID).Path
}

function Test-DockerDaemon {
  param([int]$TimeoutSeconds = 5)

  $docker = Get-Command docker -ErrorAction SilentlyContinue
  if (-not $docker) {
    return $false
  }

  $process = $null
  try {
    $startInfo = [System.Diagnostics.ProcessStartInfo]::new()
    $startInfo.FileName = $docker.Source
    $startInfo.Arguments = "info"
    $startInfo.UseShellExecute = $false
    $startInfo.CreateNoWindow = $true
    $startInfo.RedirectStandardOutput = $true
    $startInfo.RedirectStandardError = $true
    $process = [System.Diagnostics.Process]::Start($startInfo)
    if (-not $process.WaitForExit($TimeoutSeconds * 1000)) {
      $process.Kill()
      return $false
    }
    return ($process.ExitCode -eq 0)
  } catch {
    if ($process -and -not $process.HasExited) {
      $process.Kill()
    }
    return $false
  }
}

Assert-ScriptParses -Path $startScript
Assert-ScriptParses -Path $launcherScript

$pwshPath = Get-PwshPath

$fastOutput = & $pwshPath -NoProfile -ExecutionPolicy Bypass -File $startScript -Fast -PrepareOnly -SkipLocalExeBuild -NoBrowser 2>&1
Assert-True -Condition ($LASTEXITCODE -eq 0) -Message "start-all fast prepare failed: $($fastOutput | Out-String)"
Assert-True -Condition (($fastOutput | Out-String) -match "\[MODE\] fast startup") -Message "Fast mode output missing"
Assert-True -Condition (($fastOutput | Out-String) -match "\[SKIP\] local executable build") -Message "Fast prepare should skip executable build when requested"

$dryRunFast = (& $pwshPath -NoProfile -ExecutionPolicy Bypass -File $launcherScript -DryRunPreset Fast 2>&1 | Out-String) | ConvertFrom-Json
Assert-True -Condition ($LASTEXITCODE -eq 0) -Message "launcher fast dry run failed"
Assert-True -Condition ($dryRunFast.Script.EndsWith("scripts\local\start-all.ps1")) -Message "Fast dry run should call start-all.ps1"
Assert-Contains -Items $dryRunFast.Arguments -Expected "-Fast" -Message "Fast dry run should include -Fast"
Assert-Contains -Items $dryRunFast.Arguments -Expected "-NoBrowser" -Message "Fast dry run should include -NoBrowser"
Assert-Contains -Items $dryRunFast.Arguments -Expected "-StartDockerDesktop" -Message "Fast dry run should auto-start Docker Desktop"

$dryRunFastRestart = (& $pwshPath -NoProfile -ExecutionPolicy Bypass -File $launcherScript -DryRunPreset FastRestartDocker 2>&1 | Out-String) | ConvertFrom-Json
Assert-True -Condition ($LASTEXITCODE -eq 0) -Message "launcher fast restart dry run failed"
Assert-Contains -Items $dryRunFastRestart.Arguments -Expected "-RestartDockerDesktop" -Message "Fast restart dry run should include -RestartDockerDesktop"

$dryRunFull = (& $pwshPath -NoProfile -ExecutionPolicy Bypass -File $launcherScript -DryRunPreset Full 2>&1 | Out-String) | ConvertFrom-Json
Assert-True -Condition ($LASTEXITCODE -eq 0) -Message "launcher full dry run failed"
Assert-True -Condition ($dryRunFull.Script.EndsWith("scripts\local\start-all.ps1")) -Message "Full dry run should call start-all.ps1"
Assert-Contains -Items $dryRunFull.Arguments -Expected "-StartDockerDesktop" -Message "Full dry run should auto-start Docker Desktop"

$dryRunPrepare = (& $pwshPath -NoProfile -ExecutionPolicy Bypass -File $launcherScript -DryRunPreset PrepareOnly 2>&1 | Out-String) | ConvertFrom-Json
Assert-True -Condition ($LASTEXITCODE -eq 0) -Message "launcher prepare dry run failed"
Assert-Contains -Items $dryRunPrepare.Arguments -Expected "-PrepareOnly" -Message "Prepare dry run should include -PrepareOnly"
Assert-True -Condition (-not ($dryRunPrepare.Arguments -contains "-RebuildLocalExes")) -Message "Prepare dry run should not force rebuild unless requested"

$dryRunStop = (& $pwshPath -NoProfile -ExecutionPolicy Bypass -File $launcherScript -DryRunPreset StopWithDeps 2>&1 | Out-String) | ConvertFrom-Json
Assert-True -Condition ($LASTEXITCODE -eq 0) -Message "launcher stop dry run failed"
Assert-True -Condition ($dryRunStop.Script.EndsWith("scripts\local\stop-all.ps1")) -Message "Stop dry run should call stop-all.ps1"
Assert-Contains -Items $dryRunStop.Arguments -Expected "-WithDeps" -Message "Stop dry run should include -WithDeps"

if (-not (Test-DockerDaemon)) {
  $stopOutput = & $pwshPath -NoProfile -ExecutionPolicy Bypass -File (Join-Path $RepoRoot "scripts\local\stop-all.ps1") -WithDeps 2>&1
  Assert-True -Condition ($LASTEXITCODE -eq 0) -Message "stop-all -WithDeps should tolerate Docker daemon being stopped: $($stopOutput | Out-String)"
} else {
  Write-Host "[SKIP] docker daemon is running; skip live stop-all -WithDeps check"
}

Write-Host "[OK] local launcher checks passed"
