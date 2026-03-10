param(
  [string]$Namespace = "flash-mall",
  [string]$Url = "http://localhost:8888/api/order/create",
  [int]$Concurrency = 12,
  [int]$DurationSeconds = 120,
  [int64]$ProductId = 100,
  [int64]$TotalStock = 10000,
  [int]$Shards = 4,
  [switch]$PrepareData
)

$ErrorActionPreference = "Stop"

function Invoke-Mysql {
  param([string]$Sql)
  $cmd = "MYSQL_PWD=flashmall mysql -uroot -h 127.0.0.1 -P 3306"
  $out = $Sql | kubectl -n $Namespace exec -i $mysqlPod -- /bin/sh -c $cmd
  if ($LASTEXITCODE -ne 0) {
    throw "mysql exec failed"
  }
  return $out
}

if ($PrepareData) {
  # CHG 2026-02-24: 变更=压测前自动准备商品库存; 之前=手动执行 SQL + MSET; 原因=一键可重复演示 HPA。
  kubectl -n $Namespace wait --for=condition=Ready pod -l app=mysql --timeout=60s | Out-Null
  $mysqlPod = kubectl -n $Namespace get pods -l app=mysql -o jsonpath='{.items[0].metadata.name}'
  if (-not $mysqlPod) {
    throw "MySQL pod not found in namespace $Namespace"
  }

  # CHG 2026-02-26: 变更=同步初始化分桶库存; 之前=仅初始化 product 单行库存; 原因=分桶扣减需要对应数据。
  & "$PSScriptRoot/seed-stock-bucket.ps1" -Namespace $Namespace -ProductId $ProductId -TotalStock $TotalStock -Buckets $Shards -Force | Out-Null
  & "$PSScriptRoot/seed-stock.ps1" -Namespace $Namespace -ProductId $ProductId -TotalStock $TotalStock -Shards $Shards -Force | Out-Null
}

$stopAt = (Get-Date).AddSeconds($DurationSeconds)
$jobs = @()

for ($i = 0; $i -lt $Concurrency; $i++) {
  $jobs += Start-Job -ArgumentList $Url, $ProductId, $stopAt, $i -ScriptBlock {
    param($Url, $ProductId, $StopAt, $WorkerId)

    $success = 0
    $fail = 0
    # CHG 2026-02-26: 变更=显式加载 System.Net.Http; 之前=后台作业找不到类型; 原因=兼容 PowerShell 作业上下文。
    Add-Type -AssemblyName System.Net.Http | Out-Null
    $client = New-Object System.Net.Http.HttpClient
    $client.Timeout = [TimeSpan]::FromSeconds(3)

    while ((Get-Date) -lt $StopAt) {
      $reqId = "hpa-$WorkerId-" + [guid]::NewGuid().ToString("N")
      $body = @{user_id=1; product_id=$ProductId; amount=1; request_id=$reqId} | ConvertTo-Json -Compress
      $content = New-Object System.Net.Http.StringContent($body, [System.Text.Encoding]::UTF8, "application/json")
      try {
        $resp = $client.PostAsync($Url, $content).GetAwaiter().GetResult()
        if ($resp.IsSuccessStatusCode) {
          $success++
        } else {
          $fail++
        }
      } catch {
        $fail++
      }
    }

    $client.Dispose()
    [pscustomobject]@{
      worker  = $WorkerId
      success = $success
      fail    = $fail
    }
  }
}

Write-Host "HPA demo running... concurrency=$Concurrency duration=${DurationSeconds}s"
Wait-Job -Job $jobs | Out-Null
$results = Receive-Job -Job $jobs
Remove-Job -Job $jobs | Out-Null

$totalSuccess = ($results | Measure-Object -Property success -Sum).Sum
$totalFail = ($results | Measure-Object -Property fail -Sum).Sum
Write-Host "done. success=$totalSuccess fail=$totalFail"
