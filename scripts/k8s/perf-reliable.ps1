param(
  [string]$Namespace = "flash-mall",
  [int]$Runs = 5,
  [int]$Concurrency = 100,
  [int]$DurationSeconds = 180,
  [int]$WarmupSeconds = 30,
  [int]$CooldownSeconds = 20,
  [string]$Scenario = "interview-baseline",
  [int64]$ProductId = 100,
  [int64]$TotalStock = 10000,
  [int]$Shards = 4,
  [int]$TargetRps = 0,
  [int]$TimeoutMs = 5000,
  [ValidateSet("port-forward", "in-cluster")] [string]$LoadMode = "port-forward",
  [string]$OutputRoot = ""
)

$ErrorActionPreference = "Stop"

function Get-Percentile {
  param(
    [double[]]$Values,
    [double]$P
  )

  if (-not $Values -or $Values.Count -eq 0) {
    return 0.0
  }

  $sorted = $Values | Sort-Object
  if ($P -le 0) {
    return [double]$sorted[0]
  }
  if ($P -ge 1) {
    return [double]$sorted[-1]
  }

  $index = [math]::Ceiling($P * $sorted.Count) - 1
  if ($index -lt 0) { $index = 0 }
  if ($index -ge $sorted.Count) { $index = $sorted.Count - 1 }
  return [double]$sorted[$index]
}

function Get-Mean {
  param([double[]]$Values)
  if (-not $Values -or $Values.Count -eq 0) {
    return 0.0
  }
  return ($Values | Measure-Object -Average).Average
}

function Get-StdDev {
  param([double[]]$Values)
  if (-not $Values -or $Values.Count -le 1) {
    return 0.0
  }
  $mean = Get-Mean -Values $Values
  $variance = 0.0
  foreach ($v in $Values) {
    $variance += [math]::Pow(($v - $mean), 2)
  }
  $variance = $variance / ($Values.Count - 1)
  return [math]::Sqrt($variance)
}

function Get-StatusCount {
  param(
    [object]$Report,
    [string]$Code
  )

  if (-not $Report.status_codes) {
    return 0
  }
  $prop = $Report.status_codes.PSObject.Properties | Where-Object { $_.Name -eq $Code } | Select-Object -First 1
  if (-not $prop) {
    return 0
  }
  return [int64]$prop.Value
}

if ($Runs -lt 1) {
  throw "Runs must be >= 1"
}

$timestamp = Get-Date -Format "yyyyMMdd-HHmmss"
if ([string]::IsNullOrWhiteSpace($OutputRoot)) {
  $OutputRoot = Join-Path "docs/perf" "reliable-$timestamp"
}
New-Item -ItemType Directory -Force -Path $OutputRoot | Out-Null

$results = @()
for ($i = 1; $i -le $Runs; $i++) {
  $runName = "run-{0:d2}" -f $i
  $runDir = Join-Path $OutputRoot $runName
  New-Item -ItemType Directory -Force -Path $runDir | Out-Null

  $runScenario = "$Scenario-$runName"
  Write-Host "[$i/$Runs] Running $runScenario ..."

  $collectParams = @{
    Namespace      = $Namespace
    Concurrency    = $Concurrency
    DurationSeconds = $DurationSeconds
    WarmupSeconds  = $WarmupSeconds
    Scenario       = $runScenario
    OutputDir      = $runDir
    ProductId      = $ProductId
    TotalStock     = $TotalStock
    Shards         = $Shards
    TargetRps      = $TargetRps
    TimeoutMs      = $TimeoutMs
    LoadMode       = $LoadMode
  }

  ./scripts/k8s/perf-collect.ps1 @collectParams

  $reportPath = Join-Path $runDir "bench_report.json"
  if (-not (Test-Path $reportPath)) {
    throw "Missing bench_report.json at $reportPath"
  }
  $report = Get-Content -Raw $reportPath | ConvertFrom-Json

  $results += [pscustomobject]@{
    run          = $runName
    scenario     = $runScenario
    qps          = [double]$report.qps
    avg_ms       = [double]$report.avg_ms
    p95_ms       = [double]$report.p95_ms
    p99_ms       = [double]$report.p99_ms
    success      = [int64]$report.success
    failed       = [int64]$report.failed
    success_rate = [double]$report.success_rate
    status_429   = Get-StatusCount -Report $report -Code "429"
    status_503   = Get-StatusCount -Report $report -Code "503"
    output       = $runDir
  }

  if ($i -lt $Runs -and $CooldownSeconds -gt 0) {
    Start-Sleep -Seconds $CooldownSeconds
  }
}

$qpsValues = @($results | ForEach-Object { [double]$_.qps })
$p95Values = @($results | ForEach-Object { [double]$_.p95_ms })
$p99Values = @($results | ForEach-Object { [double]$_.p99_ms })
$successValues = @($results | ForEach-Object { [double]$_.success_rate })

$summary = [ordered]@{
  timestamp = $timestamp
  namespace = $Namespace
  runs = $Runs
  concurrency = $Concurrency
  warmup_seconds = $WarmupSeconds
  measurement_seconds = $DurationSeconds
  target_rps = $TargetRps
  timeout_ms = $TimeoutMs
  load_mode = $LoadMode
  product_id = $ProductId
  total_stock = $TotalStock
  shards = $Shards
  qps_median = [math]::Round((Get-Percentile -Values $qpsValues -P 0.5), 2)
  qps_p25 = [math]::Round((Get-Percentile -Values $qpsValues -P 0.25), 2)
  qps_p75 = [math]::Round((Get-Percentile -Values $qpsValues -P 0.75), 2)
  qps_cv = [math]::Round((Get-StdDev -Values $qpsValues) / [math]::Max((Get-Mean -Values $qpsValues), 0.000001), 4)
  p95_median_ms = [math]::Round((Get-Percentile -Values $p95Values -P 0.5), 2)
  p95_p25_ms = [math]::Round((Get-Percentile -Values $p95Values -P 0.25), 2)
  p95_p75_ms = [math]::Round((Get-Percentile -Values $p95Values -P 0.75), 2)
  p95_cv = [math]::Round((Get-StdDev -Values $p95Values) / [math]::Max((Get-Mean -Values $p95Values), 0.000001), 4)
  p99_median_ms = [math]::Round((Get-Percentile -Values $p99Values -P 0.5), 2)
  success_rate_median = [math]::Round((Get-Percentile -Values $successValues -P 0.5), 6)
  status_429_total = ($results | Measure-Object -Property status_429 -Sum).Sum
  status_503_total = ($results | Measure-Object -Property status_503 -Sum).Sum
  recommendation = if (((Get-StdDev -Values $p95Values) / [math]::Max((Get-Mean -Values $p95Values), 0.000001)) -le 0.1 -and ((Get-StdDev -Values $qpsValues) / [math]::Max((Get-Mean -Values $qpsValues), 0.000001)) -le 0.1) { "stable" } else { "unstable_retest" }
}

$summaryPath = Join-Path $OutputRoot "summary.json"
$summary | ConvertTo-Json -Depth 6 | Out-File $summaryPath -Encoding utf8

$reportFile = Join-Path "docs" "PERF_RELIABLE_$timestamp.md"
$lines = @()
$lines += "# 可靠性能报告（$timestamp）"
$lines += ""
$lines += "参数：runs=$Runs, concurrency=$Concurrency, warmup=${WarmupSeconds}s, measure=${DurationSeconds}s, targetRps=$TargetRps, loadMode=$LoadMode"
$lines += ""
$lines += "## 单次结果"
$lines += ""
$lines += "| run | qps | avg_ms | p95_ms | p99_ms | success | failed | 429 | 503 | output |"
$lines += "|---|---:|---:|---:|---:|---:|---:|---:|---:|---|"
foreach ($r in $results) {
  $lines += "| $($r.run) | $([math]::Round($r.qps,2)) | $([math]::Round($r.avg_ms,2)) | $([math]::Round($r.p95_ms,2)) | $([math]::Round($r.p99_ms,2)) | $($r.success) | $($r.failed) | $($r.status_429) | $($r.status_503) | $($r.output) |"
}
$lines += ""
$lines += "## 统计摘要（用于面试答辩）"
$lines += ""
$lines += "- QPS 中位数: $($summary.qps_median)（IQR: $($summary.qps_p25) ~ $($summary.qps_p75), CV=$($summary.qps_cv)）"
$lines += "- P95 中位数: $($summary.p95_median_ms)ms（IQR: $($summary.p95_p25_ms) ~ $($summary.p95_p75_ms), CV=$($summary.p95_cv)）"
$lines += "- P99 中位数: $($summary.p99_median_ms)ms"
$lines += "- 成功率中位数: $([math]::Round($summary.success_rate_median * 100, 2))%"
$lines += "- 429 总数: $($summary.status_429_total), 503 总数: $($summary.status_503_total)"
$lines += "- 稳定性结论: $($summary.recommendation)"
$lines += ""
$lines += "原始数据目录：$OutputRoot"

$lines -join "`n" | Set-Content -Encoding utf8 $reportFile

Write-Host "summary json: $summaryPath"
Write-Host "markdown report: $reportFile"
Write-Host "raw output: $OutputRoot"
