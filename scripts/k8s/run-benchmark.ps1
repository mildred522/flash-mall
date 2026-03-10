param(
  [string]$Namespace = "flash-mall",
  [string]$Url = "http://localhost:8888/api/order/create",
  [int]$Concurrency = 20,
  [int]$DurationSeconds = 180,
  [int]$WarmupSeconds = 30,
  [int]$Requests = 0,
  [int]$TargetRps = 0,
  [int]$TimeoutMs = 5000,
  [int64]$MaxErrorSamples = 5,
  [string]$Scenario = "seckill",
  [string]$ReportPath = "bench_report.json",
  [int64]$ProductId = 100,
  [int64]$TotalStock = 10000,
  [int]$Shards = 4,
  [int64]$Amount = 1,
  [switch]$PrepareData
)

$ErrorActionPreference = "Stop"

if ($PrepareData) {
  # CHG 2026-02-24: 变更=压测前自动准备商品库存; 之前=需手动初始化; 原因=可重复生成性能基线数据。
  kubectl -n $Namespace wait --for=condition=Ready pod -l app=mysql --timeout=60s | Out-Null
  $mysqlPod = kubectl -n $Namespace get pods -l app=mysql -o jsonpath='{.items[0].metadata.name}'
  if (-not $mysqlPod) {
    throw "MySQL pod not found in namespace $Namespace"
  }

  # CHG 2026-02-24: 变更=同步初始化分桶库存; 之前=仅初始化 product 单行库存; 原因=分桶扣减需要对应数据。
  & "$PSScriptRoot/seed-stock-bucket.ps1" -Namespace $Namespace -ProductId $ProductId -TotalStock $TotalStock -Buckets $Shards -Force | Out-Null
  & "$PSScriptRoot/seed-stock.ps1" -Namespace $Namespace -ProductId $ProductId -TotalStock $TotalStock -Shards $Shards -Force | Out-Null
}

$toolPath = Resolve-Path (Join-Path $PSScriptRoot "..\\..\\app\\order\\api\\scripts\\benchmark\\benchmark_tool.go")

$args = @(
  "run",
  $toolPath,
  "-url", $Url,
  "-c", $Concurrency,
  "-scenario", $Scenario,
  "-product", $ProductId,
  "-amount", $Amount,
  "-warmup", $WarmupSeconds,
  "-timeout-ms", $TimeoutMs,
  "-max-error-samples", $MaxErrorSamples,
  "-report", $ReportPath
)

if ($TargetRps -gt 0) {
  $args += @("-rps", $TargetRps)
}

if ($Requests -gt 0) {
  $args += @("-n", $Requests)
} else {
  $args += @("-d", $DurationSeconds)
}

Write-Host "running benchmark..."
& go @args
