param(
  [string]$Namespace = "flash-mall",
  [int[]]$Buckets = @(4, 8),
  [int[]]$Concurrencies = @(20, 50, 100),
  [int]$DurationSeconds = 15,
  [int]$ProfileSeconds = 10,
  [int64]$ProductId = 100,
  [int64]$TotalStock = 10000
)

$ErrorActionPreference = "Stop"

function Apply-ConfigMap {
  param([int]$BucketCount)
  $raw = Get-Content -Raw -Path "k8s/apps/01-configmaps.yaml"
  $raw = $raw -replace "(?m)^(\s*)StockShardCount:\s*\d+", "`$1StockShardCount: $BucketCount"
  $raw = $raw -replace "(?m)^(\s*)StockBucketCount:\s*\d+", "`$1StockBucketCount: $BucketCount"
  $raw | kubectl apply -f - | Out-Null
}

function Wait-DeploymentsReady {
  param([string[]]$Deployments)
  foreach ($d in $Deployments) {
    kubectl -n $Namespace rollout status "deploy/$d" --timeout=120s | Out-Null
  }
}

# CHG 2026-02-26: 变更=新增性能矩阵脚本; 之前=单点压测; 原因=形成容量模型与分桶对比证据链。
$timestamp = Get-Date -Format "yyyyMMdd-HHmmss"
$rootDir = Join-Path "docs/perf" "matrix-$timestamp"
New-Item -ItemType Directory -Force -Path $rootDir | Out-Null

$results = @()

foreach ($b in $Buckets) {
  Write-Host "apply config: bucket=$b"
  Apply-ConfigMap -BucketCount $b
  kubectl -n $Namespace rollout restart deploy/order-rpc deploy/product-rpc | Out-Null
  Wait-DeploymentsReady -Deployments @("order-rpc", "product-rpc")

  foreach ($c in $Concurrencies) {
    $scenario = "b${b}-c${c}"
    $outDir = Join-Path $rootDir $scenario
    Write-Host "run: bucket=$b concurrency=$c output=$outDir"

    ./scripts/k8s/perf-collect.ps1 -Namespace $Namespace -Concurrency $c -DurationSeconds $DurationSeconds -Scenario $scenario -ProfileSeconds $ProfileSeconds -OutputDir $outDir -ProductId $ProductId -TotalStock $TotalStock -Shards $b

    $reportPath = Join-Path $outDir "bench_report.json"
    $report = Get-Content -Raw $reportPath | ConvertFrom-Json
    $results += [pscustomobject]@{
      bucket      = $b
      concurrency = $c
      qps         = [math]::Round($report.qps, 2)
      avg_ms      = [math]::Round($report.avg_ms, 2)
      p95_ms      = [math]::Round($report.p95_ms, 2)
      p99_ms      = [math]::Round($report.p99_ms, 2)
      success     = $report.success
      failed      = $report.failed
      output      = $outDir
    }
  }
}

$resetBucket = $Buckets | Select-Object -First 1
Write-Host "reset config: bucket=$resetBucket"
Apply-ConfigMap -BucketCount $resetBucket
kubectl -n $Namespace rollout restart deploy/order-rpc deploy/product-rpc | Out-Null
Wait-DeploymentsReady -Deployments @("order-rpc", "product-rpc")

$reportFile = "docs/PERF_MATRIX_$timestamp.md"
$lines = @()
$lines += "# 性能矩阵报告（$timestamp）"
$lines += ""
$lines += "参数：Duration=${DurationSeconds}s, Profile=${ProfileSeconds}s, ProductId=$ProductId, TotalStock=$TotalStock"
$lines += ""
$lines += "| buckets | concurrency | qps | avg_ms | p95_ms | p99_ms | success | failed | output |"
$lines += "|---|---:|---:|---:|---:|---:|---:|---:|---|"
foreach ($r in $results) {
  $lines += "| $($r.bucket) | $($r.concurrency) | $($r.qps) | $($r.avg_ms) | $($r.p95_ms) | $($r.p99_ms) | $($r.success) | $($r.failed) | $($r.output) |"
}
$lines -join "`n" | Set-Content -Encoding utf8 $reportFile

Write-Host "matrix report: $reportFile"
