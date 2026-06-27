param(
  [ValidateSet("", "Fast", "FastRestartDocker", "Full", "PrepareOnly", "StopWithDeps", "WslCompose", "WslStop")]
  [string]$DryRunPreset = ""
)

$ErrorActionPreference = "Stop"

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).ProviderPath
$startScript = Join-Path $repoRoot "scripts\local\start-all.ps1"
$stopScript = Join-Path $repoRoot "scripts\local\stop-all.ps1"
$startWslComposeScript = Join-Path $repoRoot "scripts\local\start-wsl-compose.ps1"
$stopWslComposeScript = Join-Path $repoRoot "scripts\local\stop-wsl-compose.ps1"
$launcherLogDir = Join-Path $repoRoot ".runtime\launcher-logs"
$serviceLogDir = Join-Path $repoRoot "logs\local"

$script:activeProcess = $null
$script:activeCommand = $null
$script:activeStdout = ""
$script:activeStderr = ""
$script:activeStartedAt = $null

function New-CommandSpec {
  param(
    [string]$Script,
    [string[]]$Arguments = @()
  )

  [PSCustomObject]@{
    Script = $Script
    Arguments = @($Arguments)
    WorkingDirectory = $repoRoot
  }
}

function New-PresetCommand {
  param([string]$Preset)

  switch ($Preset) {
    "Fast" {
      return New-CommandSpec -Script $startScript -Arguments @("-Fast", "-StartDockerDesktop", "-NoBrowser")
    }
    "FastRestartDocker" {
      return New-CommandSpec -Script $startScript -Arguments @("-Fast", "-StartDockerDesktop", "-RestartDockerDesktop", "-NoBrowser")
    }
    "Full" {
      return New-CommandSpec -Script $startScript -Arguments @("-StartDockerDesktop")
    }
    "PrepareOnly" {
      return New-CommandSpec -Script $startScript -Arguments @("-PrepareOnly")
    }
    "StopWithDeps" {
      return New-CommandSpec -Script $stopScript -Arguments @("-WithDeps")
    }
    "WslCompose" {
      return New-CommandSpec -Script $startWslComposeScript -Arguments @("-NoBuild")
    }
    "WslStop" {
      return New-CommandSpec -Script $stopWslComposeScript -Arguments @()
    }
    default {
      throw "Unknown preset: $Preset"
    }
  }
}

function Get-PwshPath {
  $pwsh = Get-Command pwsh -ErrorAction SilentlyContinue
  if ($pwsh) {
    return $pwsh.Source
  }
  return (Get-Process -Id $PID).Path
}

function ConvertTo-CommandLine {
  param([object]$CommandSpec)

  $parts = @("-NoProfile", "-ExecutionPolicy", "Bypass", "-File", $CommandSpec.Script) + @($CommandSpec.Arguments)
  return ($parts | ForEach-Object {
      $value = [string]$_
      if ($value -match '[\s"]') {
        '"' + $value.Replace('"', '\"') + '"'
      } else {
        $value
      }
    }) -join " "
}

function Get-FileTailText {
  param(
    [string]$Path,
    [int]$Tail = 120
  )

  if (-not (Test-Path -LiteralPath $Path)) {
    return ""
  }

  $lines = Get-Content -LiteralPath $Path -Tail $Tail -ErrorAction SilentlyContinue
  if (-not $lines) {
    return ""
  }
  return ($lines -join [Environment]::NewLine)
}

function Add-LogSection {
  param(
    [System.Collections.ArrayList]$Sections,
    [string]$Title,
    [string]$Body
  )

  if ([string]::IsNullOrWhiteSpace($Body)) {
    return
  }

  [void]$Sections.Add("========== $Title ==========")
  [void]$Sections.Add($Body.Trim())
  [void]$Sections.Add("")
}

if ($DryRunPreset -ne "") {
  New-PresetCommand -Preset $DryRunPreset | ConvertTo-Json -Depth 4
  return
}

if ([System.Threading.Thread]::CurrentThread.GetApartmentState() -ne "STA") {
  $argumentLine = ConvertTo-CommandLine -CommandSpec (New-CommandSpec -Script $PSCommandPath -Arguments @())
  Start-Process -FilePath (Get-PwshPath) -ArgumentList "-STA $argumentLine" -WorkingDirectory $repoRoot
  return
}

Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing

[System.Windows.Forms.Application]::EnableVisualStyles()

$form = New-Object System.Windows.Forms.Form
$form.Text = "Flash Mall Local Launcher"
$form.StartPosition = "CenterScreen"
$form.Size = New-Object System.Drawing.Size(1120, 600)
$form.MinimumSize = New-Object System.Drawing.Size(1040, 560)
$form.Font = New-Object System.Drawing.Font("Microsoft YaHei UI", 9)

$title = New-Object System.Windows.Forms.Label
$title.Text = "Flash Mall 本地启动选择"
$title.Font = New-Object System.Drawing.Font("Microsoft YaHei UI", 14, [System.Drawing.FontStyle]::Bold)
$title.Location = New-Object System.Drawing.Point(18, 16)
$title.Size = New-Object System.Drawing.Size(600, 30)
$form.Controls.Add($title)

$hint = New-Object System.Windows.Forms.Label
$hint.Text = "推荐使用 WSL Compose 启动；Windows exe 启动保留用于兼容旧流程。"
$hint.Location = New-Object System.Drawing.Point(20, 52)
$hint.Size = New-Object System.Drawing.Size(600, 26)
$form.Controls.Add($hint)

$actionGroup = New-Object System.Windows.Forms.GroupBox
$actionGroup.Text = "启动动作"
$actionGroup.Location = New-Object System.Drawing.Point(20, 86)
$actionGroup.Size = New-Object System.Drawing.Size(600, 146)
$form.Controls.Add($actionGroup)

function New-Button {
  param(
    [string]$Text,
    [int]$X,
    [int]$Y,
    [int]$Width = 135
  )

  $button = New-Object System.Windows.Forms.Button
  $button.Text = $Text
  $button.Location = New-Object System.Drawing.Point($X, $Y)
  $button.Size = New-Object System.Drawing.Size($Width, 34)
  return $button
}

$fastButton = New-Button -Text "快速启动" -X 16 -Y 28
$fullButton = New-Button -Text "完整启动" -X 160 -Y 28
$prepareButton = New-Button -Text "仅准备 exe" -X 304 -Y 28
$stopButton = New-Button -Text "停止项目和依赖" -X 448 -Y 28
$wslStartButton = New-Button -Text "WSL 启动" -X 16 -Y 72
$wslStopButton = New-Button -Text "WSL 停止" -X 160 -Y 72
$actionGroup.Controls.AddRange(@($fastButton, $fullButton, $prepareButton, $stopButton, $wslStartButton, $wslStopButton))

$optionsGroup = New-Object System.Windows.Forms.GroupBox
$optionsGroup.Text = "Windows exe 高级选项"
$optionsGroup.Location = New-Object System.Drawing.Point(20, 246)
$optionsGroup.Size = New-Object System.Drawing.Size(600, 148)
$form.Controls.Add($optionsGroup)

function New-CheckBox {
  param(
    [string]$Text,
    [int]$X,
    [int]$Y,
    [bool]$Checked = $false
  )

  $check = New-Object System.Windows.Forms.CheckBox
  $check.Text = $Text
  $check.Location = New-Object System.Drawing.Point($X, $Y)
  $check.Size = New-Object System.Drawing.Size(180, 24)
  $check.Checked = $Checked
  return $check
}

$skipCompose = New-CheckBox -Text "跳过 Docker 依赖" -X 16 -Y 28
$skipDbInit = New-CheckBox -Text "跳过数据库初始化" -X 210 -Y 28
$skipSeed = New-CheckBox -Text "跳过 Redis 库存 seed" -X 404 -Y 28
$skipFrontend = New-CheckBox -Text "跳过前端构建" -X 16 -Y 60
$rebuildExe = New-CheckBox -Text "强制重建 exe" -X 210 -Y 60
$noBrowser = New-CheckBox -Text "启动后不打开浏览器" -X 404 -Y 60
$trustCert = New-CheckBox -Text "信任签名发布者" -X 16 -Y 92
$trustRoot = New-CheckBox -Text "信任签名根证书" -X 210 -Y 92
$firewall = New-CheckBox -Text "更新防火墙规则" -X 404 -Y 92
$startDocker = New-CheckBox -Text "自动启动 Docker" -X 16 -Y 118 -Checked $true
$restartDocker = New-CheckBox -Text "启动前重启 Docker" -X 210 -Y 118
$optionsGroup.Controls.AddRange(@(
    $skipCompose, $skipDbInit, $skipSeed,
    $skipFrontend, $rebuildExe, $noBrowser,
    $trustCert, $trustRoot, $firewall, $startDocker, $restartDocker
  ))

$commandLabel = New-Object System.Windows.Forms.Label
$commandLabel.Text = "将执行的命令"
$commandLabel.Location = New-Object System.Drawing.Point(20, 408)
$commandLabel.Size = New-Object System.Drawing.Size(200, 22)
$form.Controls.Add($commandLabel)

$commandBox = New-Object System.Windows.Forms.TextBox
$commandBox.Location = New-Object System.Drawing.Point(20, 434)
$commandBox.Size = New-Object System.Drawing.Size(600, 46)
$commandBox.Multiline = $true
$commandBox.ReadOnly = $true
$commandBox.ScrollBars = "Vertical"
$form.Controls.Add($commandBox)

$status = New-Object System.Windows.Forms.Label
$status.Location = New-Object System.Drawing.Point(20, 496)
$status.Size = New-Object System.Drawing.Size(600, 24)
$status.Text = "选择一个动作，启动器会在后台执行，并在右侧显示失败日志。WSL 启动访问 http://127.0.0.1:8888。"
$form.Controls.Add($status)

$logGroup = New-Object System.Windows.Forms.GroupBox
$logGroup.Text = "错误日志"
$logGroup.Location = New-Object System.Drawing.Point(640, 86)
$logGroup.Size = New-Object System.Drawing.Size(440, 414)
$form.Controls.Add($logGroup)

$logBox = New-Object System.Windows.Forms.RichTextBox
$logBox.Location = New-Object System.Drawing.Point(12, 24)
$logBox.Size = New-Object System.Drawing.Size(416, 334)
$logBox.ReadOnly = $true
$logBox.WordWrap = $false
$logBox.ScrollBars = "Both"
$logBox.Font = New-Object System.Drawing.Font("Consolas", 9)
$logBox.Text = "暂无错误。命令失败后，这里会显示 stderr、stdout 尾部和服务错误日志。"
$logGroup.Controls.Add($logBox)

$clearLogButton = New-Button -Text "清空日志" -X 12 -Y 368 -Width 100
$openLogDirButton = New-Button -Text "打开日志目录" -X 122 -Y 368 -Width 120
$logGroup.Controls.AddRange(@($clearLogButton, $openLogDirButton))

function Get-AdvancedArguments {
  $args = @()
  if ($skipCompose.Checked) { $args += "-SkipCompose" }
  if ($skipDbInit.Checked) { $args += "-SkipDbInit" }
  if ($skipSeed.Checked) { $args += "-SkipSeedStock" }
  if ($skipFrontend.Checked) { $args += "-SkipFrontend" }
  if ($rebuildExe.Checked) { $args += "-RebuildLocalExes" }
  if ($noBrowser.Checked) { $args += "-NoBrowser" }
  if ($trustCert.Checked) { $args += "-TrustLocalCodeSigningCert" }
  if ($trustRoot.Checked) { $args += "-TrustLocalCodeSigningRoot" }
  if ($firewall.Checked) { $args += "-UpdateLocalFirewall" }
  if ($startDocker.Checked) { $args += "-StartDockerDesktop" }
  if ($restartDocker.Checked) { $args += "-RestartDockerDesktop" }
  return $args
}

function Show-Command {
  param([object]$CommandSpec)

  $commandBox.Text = "$(Get-PwshPath) $(ConvertTo-CommandLine -CommandSpec $CommandSpec)"
}

function Set-ActionButtonsEnabled {
  param([bool]$Enabled)

  $fastButton.Enabled = $Enabled
  $fullButton.Enabled = $Enabled
  $prepareButton.Enabled = $Enabled
  $stopButton.Enabled = $Enabled
  $wslStartButton.Enabled = $Enabled
  $wslStopButton.Enabled = $Enabled
}

function New-RunLogPaths {
  param([object]$CommandSpec)

  New-Item -ItemType Directory -Path $launcherLogDir -Force | Out-Null
  $stamp = Get-Date -Format "yyyyMMdd-HHmmss"
  $scriptName = [System.IO.Path]::GetFileNameWithoutExtension($CommandSpec.Script)
  $base = "$stamp-$scriptName"

  [PSCustomObject]@{
    Stdout = Join-Path $launcherLogDir "$base.out.log"
    Stderr = Join-Path $launcherLogDir "$base.err.log"
  }
}

function Get-RecentServiceErrorLogs {
  if (-not (Test-Path -LiteralPath $serviceLogDir)) {
    return @()
  }

  $since = $script:activeStartedAt
  if (-not $since) {
    $since = (Get-Date).AddMinutes(-10)
  }

  Get-ChildItem -LiteralPath $serviceLogDir -Filter "*.err.log" -File -ErrorAction SilentlyContinue |
    Where-Object { $_.LastWriteTime -ge $since.AddSeconds(-5) -and $_.Length -gt 0 } |
    Sort-Object LastWriteTime -Descending |
    Select-Object -First 8
}

function Show-RunFailureLog {
  param([int]$ExitCode)

  $sections = New-Object System.Collections.ArrayList
  Add-LogSection -Sections $sections -Title "命令" -Body $script:activeCommand
  Add-LogSection -Sections $sections -Title "退出码" -Body ([string]$ExitCode)
  Add-LogSection -Sections $sections -Title "stderr" -Body (Get-FileTailText -Path $script:activeStderr -Tail 160)
  Add-LogSection -Sections $sections -Title "stdout tail" -Body (Get-FileTailText -Path $script:activeStdout -Tail 120)

  foreach ($file in (Get-RecentServiceErrorLogs)) {
    Add-LogSection -Sections $sections -Title "service log: $($file.Name)" -Body (Get-FileTailText -Path $file.FullName -Tail 100)
  }

  $logFiles = @(
    "stdout: $script:activeStdout"
    "stderr: $script:activeStderr"
    "service logs: $serviceLogDir"
  ) -join [Environment]::NewLine
  Add-LogSection -Sections $sections -Title "日志文件" -Body $logFiles

  if ($sections.Count -eq 0) {
    $logBox.Text = "命令失败，但没有捕获到错误输出。`r`nstdout: $script:activeStdout`r`nstderr: $script:activeStderr"
  } else {
    $logBox.Text = ($sections -join [Environment]::NewLine)
  }
  $logBox.SelectionStart = 0
  $logBox.ScrollToCaret()
}

function Show-RunSuccessLog {
  param([int]$ExitCode)

  $stdout = Get-FileTailText -Path $script:activeStdout -Tail 60
  if ([string]::IsNullOrWhiteSpace($stdout)) {
    $logBox.Text = "命令执行成功，退出码 $ExitCode。"
  } else {
    $logBox.Text = "命令执行成功，退出码 $ExitCode。`r`n`r`n========== stdout tail ==========`r`n$stdout"
  }
  $logBox.SelectionStart = 0
  $logBox.ScrollToCaret()
}

function Start-Selected {
  param([object]$CommandSpec)

  if ($script:activeProcess -and -not $script:activeProcess.HasExited) {
    $status.Text = "已有命令正在运行，请等待完成。"
    return
  }

  Show-Command -CommandSpec $CommandSpec
  $paths = New-RunLogPaths -CommandSpec $CommandSpec
  $pwshPath = Get-PwshPath
  $argumentLine = ConvertTo-CommandLine -CommandSpec $CommandSpec

  $script:activeCommand = "$pwshPath $argumentLine"
  $script:activeStdout = $paths.Stdout
  $script:activeStderr = $paths.Stderr
  $script:activeStartedAt = Get-Date

  $logBox.Text = "命令运行中...`r`n`r`n$script:activeCommand`r`n`r`nstdout: $script:activeStdout`r`nstderr: $script:activeStderr"
  Set-ActionButtonsEnabled -Enabled $false

  try {
    $script:activeProcess = Start-Process `
      -FilePath $pwshPath `
      -ArgumentList $argumentLine `
      -WorkingDirectory $repoRoot `
      -RedirectStandardOutput $script:activeStdout `
      -RedirectStandardError $script:activeStderr `
      -WindowStyle Hidden `
      -PassThru
  } catch {
    Set-ActionButtonsEnabled -Enabled $true
    $logBox.Text = "无法启动命令。`r`n`r`n$($_.Exception.Message)"
    $status.Text = "启动失败。"
    return
  }

  $status.Text = "命令运行中，pid=$($script:activeProcess.Id)。失败后会在右侧显示错误日志。"
  $processTimer.Start()
}

$processTimer = New-Object System.Windows.Forms.Timer
$processTimer.Interval = 1000
$processTimer.Add_Tick({
    if (-not $script:activeProcess) {
      return
    }

    $script:activeProcess.Refresh()
    if (-not $script:activeProcess.HasExited) {
      $elapsed = [int]((Get-Date) - $script:activeStartedAt).TotalSeconds
      $status.Text = "命令运行中，pid=$($script:activeProcess.Id)，已运行 ${elapsed}s。"
      return
    }

    $processTimer.Stop()
    $exitCode = $script:activeProcess.ExitCode
    if ($exitCode -eq 0) {
      Show-RunSuccessLog -ExitCode $exitCode
      $status.Text = "命令执行成功。"
    } else {
      Show-RunFailureLog -ExitCode $exitCode
      $status.Text = "命令执行失败，错误日志已显示在右侧。"
    }

    $script:activeProcess = $null
    Set-ActionButtonsEnabled -Enabled $true
  })

$clearLogButton.Add_Click({
    $logBox.Text = "暂无错误。命令失败后，这里会显示 stderr、stdout 尾部和服务错误日志。"
  })

$openLogDirButton.Add_Click({
    New-Item -ItemType Directory -Path $launcherLogDir -Force | Out-Null
    Start-Process explorer.exe -ArgumentList $launcherLogDir
  })

$fastButton.Add_Click({
    $args = @("-Fast", "-NoBrowser") + (Get-AdvancedArguments)
    Start-Selected -CommandSpec (New-CommandSpec -Script $startScript -Arguments $args)
  })

$fullButton.Add_Click({
    Start-Selected -CommandSpec (New-CommandSpec -Script $startScript -Arguments (Get-AdvancedArguments))
  })

$prepareButton.Add_Click({
    $args = @("-PrepareOnly") + (Get-AdvancedArguments)
    Start-Selected -CommandSpec (New-CommandSpec -Script $startScript -Arguments $args)
  })

$stopButton.Add_Click({
    Start-Selected -CommandSpec (New-CommandSpec -Script $stopScript -Arguments @("-WithDeps"))
  })

$wslStartButton.Add_Click({
    Start-Selected -CommandSpec (New-PresetCommand -Preset "WslCompose")
  })

$wslStopButton.Add_Click({
    Start-Selected -CommandSpec (New-PresetCommand -Preset "WslStop")
  })

Show-Command -CommandSpec (New-PresetCommand -Preset "WslCompose")
$null = $form.ShowDialog()
