$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$installer = Join-Path $PSScriptRoot 'install-desktop-launcher.ps1'
$scriptRootPath = $PSScriptRoot -replace '^Microsoft\.PowerShell\.Core\\FileSystem::', ''
$repoRoot = [IO.Path]::GetFullPath((Join-Path $scriptRootPath '..\..'))
$publishDirectory = Join-Path $repoRoot '.runtime\launcher'
$existedBefore = Test-Path -LiteralPath $publishDirectory
$raw = & $installer -DryRun -Distro Ubuntu -Workspace /home/mildred/code/flash-mall
$result = $raw | ConvertFrom-Json

if ($result.distro -ne 'Ubuntu') { throw 'unexpected distro' }
if ($result.workspace -ne '/home/mildred/code/flash-mall') { throw 'unexpected workspace' }
if ($result.executable -notlike '*\.runtime\launcher\FlashMall.Launcher.exe') { throw 'unexpected executable' }
if ($result.shortcut -notlike '*\Desktop\Flash Mall 控制中心.lnk') { throw 'unexpected shortcut' }
if ((Test-Path -LiteralPath $result.publishDirectory) -ne $existedBefore) {
    throw 'dry-run unexpectedly changed publish directory existence'
}

Write-Output 'desktop launcher installer tests passed'
