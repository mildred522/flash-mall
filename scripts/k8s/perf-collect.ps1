param(
  [string]$Namespace = "flash-mall",
  [int]$Concurrency = 20,
  [int]$DurationSeconds = 180,
  [int]$WarmupSeconds = 30,
  [string]$Scenario = "seckill",
  [int]$ProfileSeconds = 0,
  [string]$OutputDir = "",
  [string]$LogSince = "30m",
  [int64]$ProductId = 100,
  [int64]$TotalStock = 10000,
  [int]$Shards = 4,
  [int]$TargetRps = 0,
  [int]$TimeoutMs = 5000,
  [ValidateSet("port-forward", "in-cluster")] [string]$LoadMode = "port-forward"
)

$ErrorActionPreference = "Stop"

function Start-PortForwardJob {
  param(
    [string]$Namespace,
    [string]$Target,
    [int]$LocalPort,
    [int]$RemotePort
  )

  return Start-Job -ArgumentList $Namespace, $Target, $LocalPort, $RemotePort -ScriptBlock {
    param($ns, $target, $local, $remote)
    kubectl -n $ns port-forward $target "$local`:$remote"
  }
}

function Wait-HttpReady {
  param(
    [string]$Url,
    [int]$TimeoutSeconds = 30
  )

  $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
  while ((Get-Date) -lt $deadline) {
    try {
      & curl.exe -sS -f --max-time 2 $Url -o NUL 2>$null
      if ($LASTEXITCODE -eq 0) {
        return $true
      }
    } catch {
      # Keep retrying while endpoint is warming up.
    }
    Start-Sleep -Milliseconds 500
  }
  return $false
}

function Wait-PortOpen {
  param(
    [int]$Port,
    [int]$TimeoutSeconds = 30
  )

  $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
  while ((Get-Date) -lt $deadline) {
    try {
      $client = New-Object System.Net.Sockets.TcpClient
      $async = $client.BeginConnect("127.0.0.1", $Port, $null, $null)
      if ($async.AsyncWaitHandle.WaitOne(1000, $false) -and $client.Connected) {
        $client.Close()
        return $true
      }
      $client.Close()
    } catch {
      # Ignore transient errors while waiting for port-forward.
    }
    Start-Sleep -Milliseconds 500
  }
  return $false
}

function Complete-Job {
  param(
    [System.Management.Automation.Job]$Job,
    [string]$Name,
    [int]$TimeoutSeconds = 300
  )

  if (-not $Job) {
    return
  }

  $done = Wait-Job $Job -Timeout $TimeoutSeconds
  if (-not $done) {
    Write-Host "$Name job timeout after ${TimeoutSeconds}s"
    Stop-Job $Job -ErrorAction SilentlyContinue | Out-Null
  }
  $output = Receive-Job $Job -ErrorAction SilentlyContinue
  if ($Job.State -ne "Completed") {
    Write-Host "$Name job state: $($Job.State)"
  }
  if ($output) {
    $output | Out-File -Append -Encoding utf8 (Join-Path $OutputDir "$Name.job.log")
  }
}

if ([string]::IsNullOrWhiteSpace($OutputDir)) {
  $OutputDir = Join-Path "docs/perf" (Get-Date -Format "yyyyMMdd-HHmmss")
}
New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null
$OutputDir = (Resolve-Path $OutputDir).Path

$runStartedAt = Get-Date
$runStartedIso = $runStartedAt.ToString("o")

kubectl -n $Namespace wait --for=condition=Ready pod -l app=order-api --timeout=120s | Out-Null
kubectl -n $Namespace wait --for=condition=Ready pod -l app=order-rpc --timeout=120s | Out-Null
kubectl -n $Namespace wait --for=condition=Ready pod -l app=product-rpc --timeout=120s | Out-Null

$apiPod = kubectl -n $Namespace get pods -l app=order-api -o jsonpath='{.items[0].metadata.name}'
$rpcPod = kubectl -n $Namespace get pods -l app=order-rpc -o jsonpath='{.items[0].metadata.name}'
$productPod = kubectl -n $Namespace get pods -l app=product-rpc -o jsonpath='{.items[0].metadata.name}'
if (-not $apiPod) { throw "order-api pod not found" }
if (-not $rpcPod) { throw "order-rpc pod not found" }
if (-not $productPod) { throw "product-rpc pod not found" }

$apiPort = 18888
$apiPprofPort = 16060
$rpcPprofPort = 16061
$productPprofPort = 16062

$pfJobs = @(
  (Start-PortForwardJob -Namespace $Namespace -Target "pod/$apiPod" -LocalPort $apiPprofPort -RemotePort 6060)
  (Start-PortForwardJob -Namespace $Namespace -Target "pod/$rpcPod" -LocalPort $rpcPprofPort -RemotePort 6061)
  (Start-PortForwardJob -Namespace $Namespace -Target "pod/$productPod" -LocalPort $productPprofPort -RemotePort 6062)
)
if ($LoadMode -eq "port-forward") {
  $pfJobs += (Start-PortForwardJob -Namespace $Namespace -Target "svc/order-api" -LocalPort $apiPort -RemotePort 8888)
}

Start-Sleep -Seconds 2

try {
  if ($LoadMode -eq "port-forward" -and -not (Wait-PortOpen -Port $apiPort -TimeoutSeconds 30)) {
    throw "order-api port-forward not ready"
  }
  if (-not (Wait-HttpReady -Url "http://localhost:$apiPprofPort/debug/pprof/" -TimeoutSeconds 30)) {
    throw "order-api pprof endpoint not ready"
  }
  if (-not (Wait-HttpReady -Url "http://localhost:$rpcPprofPort/debug/pprof/" -TimeoutSeconds 30)) {
    throw "order-rpc pprof endpoint not ready"
  }
  $productPprofReady = Wait-HttpReady -Url "http://localhost:$productPprofPort/debug/pprof/" -TimeoutSeconds 10
  if (-not $productPprofReady) {
    Write-Host "warning: product-rpc pprof endpoint not ready, skip product-rpc profile collection"
  }

  # Save environment snapshot for reproducibility.
  kubectl config current-context | Out-File (Join-Path $OutputDir "kube-context.txt") -Encoding utf8
  kubectl -n $Namespace get deploy -o wide | Out-File (Join-Path $OutputDir "deployments.txt") -Encoding utf8
  kubectl -n $Namespace get pods -o wide | Out-File (Join-Path $OutputDir "pods.before.txt") -Encoding utf8
  kubectl get nodes -o wide | Out-File (Join-Path $OutputDir "nodes.txt") -Encoding utf8
  kubectl -n $Namespace get configmap order-api-config -o yaml | Out-File (Join-Path $OutputDir "order-api-configmap.yaml") -Encoding utf8

  $meta = [ordered]@{
    namespace = $Namespace
    scenario = $Scenario
    load_mode = $LoadMode
    started_at = $runStartedIso
    concurrency = $Concurrency
    warmup_seconds = $WarmupSeconds
    measurement_seconds = $DurationSeconds
    target_rps = $TargetRps
    timeout_ms = $TimeoutMs
    product_id = $ProductId
    total_stock = $TotalStock
    shards = $Shards
    pods = [ordered]@{
      order_api = $apiPod
      order_rpc = $rpcPod
      product_rpc = $productPod
    }
  }
  $meta | ConvertTo-Json -Depth 8 | Out-File (Join-Path $OutputDir "run_meta.json") -Encoding utf8

  $profileWindow = $ProfileSeconds
  if ($profileWindow -le 0) {
    $profileWindow = $WarmupSeconds + $DurationSeconds
    if ($profileWindow -le 0) {
      $profileWindow = 30
    }
  }

  # Start CPU profile capture before load starts so profile overlaps measured traffic.
  $apiCpu = Join-Path $OutputDir "order-api.cpu.pprof"
  $rpcCpu = Join-Path $OutputDir "order-rpc.cpu.pprof"
  $productCpu = Join-Path $OutputDir "product-rpc.cpu.pprof"

  $cpuJobs = @(
    (Start-Job -ArgumentList $apiPprofPort, $profileWindow, $apiCpu -ScriptBlock {
      param($port, $seconds, $outFile)
      & curl.exe -sS -f "http://localhost:$port/debug/pprof/profile?seconds=$seconds" -o $outFile
    })
    (Start-Job -ArgumentList $rpcPprofPort, $profileWindow, $rpcCpu -ScriptBlock {
      param($port, $seconds, $outFile)
      & curl.exe -sS -f "http://localhost:$port/debug/pprof/profile?seconds=$seconds" -o $outFile
    })
  )
  if ($productPprofReady) {
    $cpuJobs += (Start-Job -ArgumentList $productPprofPort, $profileWindow, $productCpu -ScriptBlock {
      param($port, $seconds, $outFile)
      & curl.exe -sS -f "http://localhost:$port/debug/pprof/profile?seconds=$seconds" -o $outFile
    })
  }

  Start-Sleep -Milliseconds 500

  $reportPath = Join-Path $OutputDir "bench_report.json"
  $benchStdout = Join-Path $OutputDir "benchmark.stdout.log"

  Write-Host "running benchmark..."
  if ($LoadMode -eq "in-cluster") {
    $benchParams = @{
      Namespace       = $Namespace
      Concurrency     = $Concurrency
      DurationSeconds = $DurationSeconds
      WarmupSeconds   = $WarmupSeconds
      Scenario        = $Scenario
      ReportPath      = $reportPath
      PrepareData     = $true
      ProductId       = $ProductId
      TotalStock      = $TotalStock
      Shards          = $Shards
      TimeoutMs       = $TimeoutMs
    }
    if ($TargetRps -gt 0) {
      $benchParams.TargetRps = $TargetRps
    }
    ./scripts/k8s/run-benchmark-incluster.ps1 @benchParams 2>&1 | Tee-Object -FilePath $benchStdout
  } else {
    $benchParams = @{
      Namespace       = $Namespace
      Url             = "http://localhost:$apiPort/api/order/create"
      Concurrency     = $Concurrency
      DurationSeconds = $DurationSeconds
      WarmupSeconds   = $WarmupSeconds
      Scenario        = $Scenario
      ReportPath      = $reportPath
      PrepareData     = $true
      ProductId       = $ProductId
      TotalStock      = $TotalStock
      Shards          = $Shards
      TimeoutMs       = $TimeoutMs
    }
    if ($TargetRps -gt 0) {
      $benchParams.TargetRps = $TargetRps
    }
    ./scripts/k8s/run-benchmark.ps1 @benchParams 2>&1 | Tee-Object -FilePath $benchStdout
  }

  foreach ($job in $cpuJobs) {
    Complete-Job -Job $job -Name "cpu-profile" -TimeoutSeconds ($profileWindow + 60)
  }
  foreach ($profilePath in @($apiCpu, $rpcCpu)) {
    if (-not (Test-Path $profilePath)) {
      Write-Host "warning: missing CPU profile $profilePath"
    }
  }
  if ($productPprofReady -and -not (Test-Path $productCpu)) {
    Write-Host "warning: missing CPU profile $productCpu"
  }

  Write-Host "collecting heap profiles..."
  try {
    & curl.exe -sS -f "http://localhost:$apiPprofPort/debug/pprof/heap" -o (Join-Path $OutputDir "order-api.heap.pprof")
    if ($LASTEXITCODE -ne 0) { throw "order-api heap profile fetch failed" }
    & curl.exe -sS -f "http://localhost:$rpcPprofPort/debug/pprof/heap" -o (Join-Path $OutputDir "order-rpc.heap.pprof")
    if ($LASTEXITCODE -ne 0) { throw "order-rpc heap profile fetch failed" }
    if ($productPprofReady) {
      & curl.exe -sS -f "http://localhost:$productPprofPort/debug/pprof/heap" -o (Join-Path $OutputDir "product-rpc.heap.pprof")
      if ($LASTEXITCODE -ne 0) { throw "product-rpc heap profile fetch failed" }
    }
  } catch {
    Write-Host "heap fetch failed: $($_.Exception.Message)"
  }

  Write-Host "collecting logs and cluster snapshots..."
  kubectl -n $Namespace logs -l app=order-api --all-containers=true --prefix --since-time=$runStartedIso | Out-File (Join-Path $OutputDir "order-api.log") -Encoding utf8
  kubectl -n $Namespace logs -l app=order-rpc --all-containers=true --prefix --since-time=$runStartedIso | Out-File (Join-Path $OutputDir "order-rpc.log") -Encoding utf8
  kubectl -n $Namespace logs -l app=product-rpc --all-containers=true --prefix --since-time=$runStartedIso | Out-File (Join-Path $OutputDir "product-rpc.log") -Encoding utf8
  kubectl -n $Namespace logs deploy/dtm --since-time=$runStartedIso | Out-File (Join-Path $OutputDir "dtm.log") -Encoding utf8
  kubectl -n $Namespace get pods -o wide | Out-File (Join-Path $OutputDir "pods.after.txt") -Encoding utf8
  kubectl -n $Namespace get events --sort-by=.metadata.creationTimestamp | Out-File (Join-Path $OutputDir "events.txt") -Encoding utf8

  try {
    kubectl -n $Namespace top pods | Out-File (Join-Path $OutputDir "pods.top.txt") -Encoding utf8
  } catch {
    "metrics-server unavailable" | Out-File (Join-Path $OutputDir "pods.top.txt") -Encoding utf8
  }

  if ($LogSince) {
    "LogSince parameter retained for compatibility: $LogSince" | Out-File (Join-Path $OutputDir "notes.txt") -Encoding utf8
  }
} finally {
  foreach ($job in $pfJobs) {
    if ($job -and ($job.State -eq "Running" -or $job.State -eq "NotStarted")) {
      Stop-Job $job | Out-Null
    }
    if ($job) {
      Remove-Job $job | Out-Null
    }
  }
}

Write-Host "output: $OutputDir"
