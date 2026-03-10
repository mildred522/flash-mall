param(
  [string]$Namespace = "flash-mall",
  [string]$Url = "http://localhost:8888/api/order/create",
  [int]$Concurrency = 20,
  [int]$DurationSeconds = 120,
  [int]$ObserveIntervalSeconds = 5,
  [int64]$ProductId = 100,
  [int64]$TotalStock = 10000,
  [int]$Shards = 4,
  [switch]$PrepareData
)

$ErrorActionPreference = "Stop"

# CHG 2026-02-26: 变更=新增 HPA 端到端演练脚本; 之前=手动端口转发与监控; 原因=一键验证扩缩容。
$pf = Start-Job -ArgumentList $Namespace, "svc/order-api", 8888, 8888 -ScriptBlock {
  param($ns, $target, $local, $remote)
  kubectl -n $ns port-forward $target "$local`:$remote"
}

Start-Sleep -Seconds 2

$watch = Start-Job -ArgumentList $Namespace, $DurationSeconds, $ObserveIntervalSeconds -ScriptBlock {
  param($ns, $duration, $interval)
  $end = (Get-Date).AddSeconds($duration)
  while ((Get-Date) -lt $end) {
    Write-Output "`n--- HPA snapshot $(Get-Date -Format HH:mm:ss) ---"
    kubectl -n $ns get hpa | Out-String | Write-Output
    kubectl -n $ns get deploy order-api | Out-String | Write-Output
    Start-Sleep -Seconds $interval
  }
}

try {
  & "$PSScriptRoot/hpa-demo.ps1" -Namespace $Namespace -Url $Url -Concurrency $Concurrency -DurationSeconds $DurationSeconds -ProductId $ProductId -TotalStock $TotalStock -Shards $Shards -PrepareData:$PrepareData
} finally {
  foreach ($job in @($watch, $pf)) {
    if ($job -eq $watch) {
      Receive-Job -Job $watch -ErrorAction SilentlyContinue | Write-Host
    }
    if ($job -and ($job.State -eq "Running" -or $job.State -eq "NotStarted")) {
      Stop-Job $job | Out-Null
    }
    if ($job) { Remove-Job $job | Out-Null }
  }
}
