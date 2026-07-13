[CmdletBinding()]
param(
    [string]$Distro = 'Ubuntu',
    [string]$Workspace = '/home/mildred/code/flash-mall',
    [switch]$DryRun,
    [switch]$NoShortcut
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

if ($Distro -notmatch '^[A-Za-z0-9._-]+$') {
    throw 'Distro must be a simple WSL distribution name.'
}
if (-not $Workspace.StartsWith('/') -or $Workspace.IndexOfAny([char[]]@('"', "`r", "`n")) -ge 0) {
    throw 'Workspace must be an absolute WSL path without quotes or line breaks.'
}

$scriptRootPath = $PSScriptRoot -replace '^Microsoft\.PowerShell\.Core\\FileSystem::', ''
$repoRoot = [IO.Path]::GetFullPath((Join-Path $scriptRootPath '..\..'))
$project = Join-Path $repoRoot 'tools\FlashMall.Launcher\FlashMall.Launcher.csproj'
$publishDirectory = Join-Path $repoRoot '.runtime\launcher'
$executable = Join-Path $publishDirectory 'FlashMall.Launcher.exe'
$desktop = [Environment]::GetFolderPath([Environment+SpecialFolder]::DesktopDirectory)
$shortcut = Join-Path $desktop 'Flash Mall 控制中心.lnk'

$result = [ordered]@{
    distro = $Distro
    workspace = $Workspace
    publishDirectory = $publishDirectory
    executable = $executable
    shortcut = $shortcut
}

if ($DryRun) {
    $result | ConvertTo-Json -Compress
    return
}

$runtimes = & dotnet --list-runtimes
if ($LASTEXITCODE -ne 0) {
    throw 'Unable to inspect installed .NET runtimes.'
}
if (-not ($runtimes | Select-String -Pattern '^Microsoft\.WindowsDesktop\.App 8\.')) {
    throw '.NET 8 Windows Desktop Runtime is required. Install it before creating the launcher.'
}

New-Item -ItemType Directory -Path $publishDirectory -Force | Out-Null
& dotnet publish $project `
    -c Release `
    -r win-x64 `
    --self-contained false `
    -p:PublishSingleFile=true `
    -o $publishDirectory
if ($LASTEXITCODE -ne 0 -or -not (Test-Path -LiteralPath $executable -PathType Leaf)) {
    throw 'Desktop launcher publish failed.'
}

if (-not $NoShortcut) {
    $wsh = New-Object -ComObject WScript.Shell
    $link = $wsh.CreateShortcut($shortcut)
    $link.TargetPath = $executable
    $link.Arguments = '--distro "{0}" --workspace "{1}"' -f $Distro, $Workspace
    $link.WorkingDirectory = $repoRoot
    $link.Description = '启动并管理 Flash Mall WSL 本地环境'
    $link.IconLocation = "$executable,0"
    $link.Save()
}

$result | ConvertTo-Json -Compress
