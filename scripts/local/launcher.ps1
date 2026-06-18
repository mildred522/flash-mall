param(
  [ValidateSet("", "Fast", "Full", "PrepareOnly", "StopWithDeps")]
  [string]$DryRunPreset = ""
)

$ErrorActionPreference = "Stop"

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
$startScript = Join-Path $repoRoot "scripts\local\start-all.ps1"
$stopScript = Join-Path $repoRoot "scripts\local\stop-all.ps1"

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
      return New-CommandSpec -Script $startScript -Arguments @("-Fast", "-NoBrowser")
    }
    "Full" {
      return New-CommandSpec -Script $startScript -Arguments @()
    }
    "PrepareOnly" {
      return New-CommandSpec -Script $startScript -Arguments @("-PrepareOnly", "-RebuildLocalExes")
    }
    "StopWithDeps" {
      return New-CommandSpec -Script $stopScript -Arguments @("-WithDeps")
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

function Invoke-CommandSpec {
  param([object]$CommandSpec)

  $pwshPath = Get-PwshPath
  $argumentLine = ConvertTo-CommandLine -CommandSpec $CommandSpec
  Start-Process -FilePath $pwshPath -ArgumentList $argumentLine -WorkingDirectory $repoRoot
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
$form.Size = New-Object System.Drawing.Size(660, 560)
$form.MinimumSize = New-Object System.Drawing.Size(640, 520)
$form.Font = New-Object System.Drawing.Font("Microsoft YaHei UI", 9)

$title = New-Object System.Windows.Forms.Label
$title.Text = "Flash Mall 本地启动选择"
$title.Font = New-Object System.Drawing.Font("Microsoft YaHei UI", 14, [System.Drawing.FontStyle]::Bold)
$title.Location = New-Object System.Drawing.Point(18, 16)
$title.Size = New-Object System.Drawing.Size(600, 30)
$form.Controls.Add($title)

$hint = New-Object System.Windows.Forms.Label
$hint.Text = "快速启动适合日常开发；完整启动会重新执行数据库初始化、库存 seed 和前端构建。"
$hint.Location = New-Object System.Drawing.Point(20, 52)
$hint.Size = New-Object System.Drawing.Size(600, 26)
$form.Controls.Add($hint)

$actionGroup = New-Object System.Windows.Forms.GroupBox
$actionGroup.Text = "启动动作"
$actionGroup.Location = New-Object System.Drawing.Point(20, 86)
$actionGroup.Size = New-Object System.Drawing.Size(600, 104)
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
$actionGroup.Controls.AddRange(@($fastButton, $fullButton, $prepareButton, $stopButton))

$optionsGroup = New-Object System.Windows.Forms.GroupBox
$optionsGroup.Text = "高级选项"
$optionsGroup.Location = New-Object System.Drawing.Point(20, 204)
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
$optionsGroup.Controls.AddRange(@(
    $skipCompose, $skipDbInit, $skipSeed,
    $skipFrontend, $rebuildExe, $noBrowser,
    $trustCert, $trustRoot, $firewall
  ))

$commandLabel = New-Object System.Windows.Forms.Label
$commandLabel.Text = "将执行的命令"
$commandLabel.Location = New-Object System.Drawing.Point(20, 366)
$commandLabel.Size = New-Object System.Drawing.Size(200, 22)
$form.Controls.Add($commandLabel)

$commandBox = New-Object System.Windows.Forms.TextBox
$commandBox.Location = New-Object System.Drawing.Point(20, 392)
$commandBox.Size = New-Object System.Drawing.Size(600, 70)
$commandBox.Multiline = $true
$commandBox.ReadOnly = $true
$commandBox.ScrollBars = "Vertical"
$form.Controls.Add($commandBox)

$status = New-Object System.Windows.Forms.Label
$status.Location = New-Object System.Drawing.Point(20, 476)
$status.Size = New-Object System.Drawing.Size(600, 24)
$status.Text = "选择一个动作，启动器会打开新的 PowerShell 窗口执行。"
$form.Controls.Add($status)

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
  return $args
}

function Show-Command {
  param([object]$CommandSpec)

  $commandBox.Text = "$(Get-PwshPath) $(ConvertTo-CommandLine -CommandSpec $CommandSpec)"
}

function Start-Selected {
  param([object]$CommandSpec)

  Show-Command -CommandSpec $CommandSpec
  Invoke-CommandSpec -CommandSpec $CommandSpec
  $status.Text = "已打开新的 PowerShell 窗口执行，请在该窗口查看实时输出。"
}

$fastButton.Add_Click({
    $args = @("-Fast", "-NoBrowser") + (Get-AdvancedArguments)
    Start-Selected -CommandSpec (New-CommandSpec -Script $startScript -Arguments $args)
  })

$fullButton.Add_Click({
    Start-Selected -CommandSpec (New-CommandSpec -Script $startScript -Arguments (Get-AdvancedArguments))
  })

$prepareButton.Add_Click({
    $args = @("-PrepareOnly", "-RebuildLocalExes") + (Get-AdvancedArguments)
    Start-Selected -CommandSpec (New-CommandSpec -Script $startScript -Arguments $args)
  })

$stopButton.Add_Click({
    Start-Selected -CommandSpec (New-CommandSpec -Script $stopScript -Arguments @("-WithDeps"))
  })

Show-Command -CommandSpec (New-PresetCommand -Preset "Fast")
$null = $form.ShowDialog()
