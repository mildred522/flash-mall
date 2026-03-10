param(
  [string]$ClusterName = "flash-mall",
  [string]$ConfigPath = "k8s/kind/cluster-multi.yaml",
  [switch]$RebuildImages,
  [switch]$SkipApply
)

$ErrorActionPreference = "Stop"

function Resolve-KindPath {
  $cmd = Get-Command kind -ErrorAction SilentlyContinue
  if ($cmd) {
    return $cmd.Source
  }

  $default = Join-Path $env:LOCALAPPDATA "Microsoft\WinGet\Packages\Kubernetes.kind_Microsoft.Winget.Source_8wekyb3d8bbwe\kind.exe"
  if (Test-Path $default) {
    return $default
  }

  throw "kind.exe not found. Please install kind or add it to PATH."
}

$kind = Resolve-KindPath

# CHG 2026-02-24: 变更=新增一键多节点部署脚本; 之前=手动删除/创建集群; 原因=可重复构建多节点环境。
$prevErrorAction = $ErrorActionPreference
$ErrorActionPreference = "Continue"
$deleteOut = & $kind delete cluster --name $ClusterName 2>&1
$deleteCode = $LASTEXITCODE
$ErrorActionPreference = $prevErrorAction
if ($deleteCode -ne 0) {
  if ($deleteOut -notmatch "(?i)no nodes|not found") {
    throw "failed to delete kind cluster: $ClusterName"
  }
}

$ErrorActionPreference = "Continue"
$createOut = & $kind create cluster --name $ClusterName --config $ConfigPath 2>&1
$createCode = $LASTEXITCODE
$ErrorActionPreference = $prevErrorAction
if ($createCode -ne 0) {
  Write-Host $createOut
  throw "failed to create kind cluster: $ClusterName"
}

if ($RebuildImages) {
  & "$PSScriptRoot/build-images.ps1" -Tag dev
}

$images = @(
  "flash-mall/order-api:dev",
  "flash-mall/order-rpc:dev",
  "flash-mall/product-rpc:dev",
  "mysql:8.0",
  "redis:7",
  "bitnamilegacy/etcd:3.5",
  "yedf/dtm:latest"
)

foreach ($image in $images) {
  & $kind load docker-image $image --name $ClusterName
}

if (-not $SkipApply) {
  & "$PSScriptRoot/apply.ps1" -Namespace "flash-mall"
}
