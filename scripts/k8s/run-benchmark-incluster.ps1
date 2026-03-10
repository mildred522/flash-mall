param(
  [string]$Namespace = "flash-mall",
  [string]$Scenario = "seckill",
  [int]$Concurrency = 20,
  [int]$DurationSeconds = 180,
  [int]$WarmupSeconds = 30,
  [int]$TargetRps = 0,
  [int]$TimeoutMs = 5000,
  [string]$ServiceUrl = "http://order-api.flash-mall.svc.cluster.local:8888/api/order/create",
  [string]$ReportPath = "bench_report.json",
  [int64]$ProductId = 100,
  [int64]$Amount = 1,
  [int64]$TotalStock = 10000,
  [int]$Shards = 4,
  [switch]$PrepareData
)

$ErrorActionPreference = "Stop"

if ($PrepareData) {
  kubectl -n $Namespace wait --for=condition=Ready pod -l app=mysql --timeout=60s | Out-Null
  $mysqlPod = kubectl -n $Namespace get pods -l app=mysql -o jsonpath='{.items[0].metadata.name}'
  if (-not $mysqlPod) {
    throw "MySQL pod not found in namespace $Namespace"
  }

  & "$PSScriptRoot/seed-stock-bucket.ps1" -Namespace $Namespace -ProductId $ProductId -TotalStock $TotalStock -Buckets $Shards -Force | Out-Null
  & "$PSScriptRoot/seed-stock.ps1" -Namespace $Namespace -ProductId $ProductId -TotalStock $TotalStock -Shards $Shards -Force | Out-Null
}

$ts = Get-Date -Format "MMddHHmmss"
$jobName = "bench-k6-$ts"
$cmName = "$jobName-script"
$totalSeconds = [Math]::Max(1, $WarmupSeconds + $DurationSeconds)
$jobTimeout = [Math]::Max(300, $totalSeconds + 300)

$k6Script = @'
import http from 'k6/http';
import exec from 'k6/execution';
import { check } from 'k6';
import { Counter, Trend } from 'k6/metrics';

const targetUrl = __ENV.TARGET_URL;
const userId = Number(__ENV.USER_ID || '1');
const productId = Number(__ENV.PRODUCT_ID || '100');
const amount = Number(__ENV.AMOUNT || '1');
const timeoutMs = Number(__ENV.TIMEOUT_MS || '5000');
const warmupSeconds = Number(__ENV.WARMUP_SECONDS || '0');
const measureSeconds = Number(__ENV.MEASURE_SECONDS || '60');
const totalSeconds = Math.max(1, warmupSeconds + measureSeconds);
const concurrency = Number(__ENV.CONCURRENCY || '20');
const targetRps = Number(__ENV.TARGET_RPS || '0');

const appSuccess = new Counter('app_success');
const appFailure = new Counter('app_failure');
const appStatus429 = new Counter('app_status_429');
const appStatus503 = new Counter('app_status_503');
const appReqDuration = new Trend('app_req_duration', true);

export const options = targetRps > 0
  ? {
      summaryTrendStats: ['avg', 'med', 'p(95)', 'p(99)', 'min', 'max'],
      scenarios: {
        main: {
          executor: 'constant-arrival-rate',
          rate: targetRps,
          timeUnit: '1s',
          duration: `${totalSeconds}s`,
          preAllocatedVUs: Math.max(concurrency, targetRps),
          maxVUs: Math.max(concurrency * 2, targetRps * 2),
        },
      },
      discardResponseBodies: true,
    }
  : {
      summaryTrendStats: ['avg', 'med', 'p(95)', 'p(99)', 'min', 'max'],
      vus: concurrency,
      duration: `${totalSeconds}s`,
      discardResponseBodies: true,
    };

function inMeasuredWindow() {
  return (exec.instance.currentTestRunDuration / 1000) >= warmupSeconds;
}

export default function () {
  const requestId = `${Date.now()}-${exec.vu.idInTest}-${exec.vu.iterationInScenario}`;
  const payload = JSON.stringify({
    request_id: requestId,
    user_id: userId,
    product_id: productId,
    amount: amount,
  });

  const res = http.post(targetUrl, payload, {
    headers: { 'Content-Type': 'application/json' },
    timeout: `${timeoutMs}ms`,
  });

  const measured = inMeasuredWindow();
  const ok = check(res, { 'status is 200': (r) => r.status === 200 });

  if (!measured) {
    return;
  }

  appReqDuration.add(res.timings.duration);

  if (ok) {
    appSuccess.add(1);
    return;
  }

  appFailure.add(1);
  if (res.status === 429) {
    appStatus429.add(1);
  }
  if (res.status === 503) {
    appStatus503.add(1);
  }
}
'@

$resourceYaml = @"
apiVersion: v1
kind: ConfigMap
metadata:
  name: $cmName
  namespace: $Namespace
data:
  load.js: |
$($k6Script -split "`n" | ForEach-Object { "    $_" } | Out-String)
---
apiVersion: batch/v1
kind: Job
metadata:
  name: $jobName
  namespace: $Namespace
spec:
  backoffLimit: 0
  template:
    metadata:
      labels:
        app: k6-bench
        bench-scenario: $Scenario
    spec:
      restartPolicy: Never
      containers:
        - name: k6
          image: grafana/k6:0.49.0
          imagePullPolicy: IfNotPresent
          env:
            - name: TARGET_URL
              value: "$ServiceUrl"
            - name: CONCURRENCY
              value: "$Concurrency"
            - name: TARGET_RPS
              value: "$TargetRps"
            - name: WARMUP_SECONDS
              value: "$WarmupSeconds"
            - name: MEASURE_SECONDS
              value: "$DurationSeconds"
            - name: PRODUCT_ID
              value: "$ProductId"
            - name: AMOUNT
              value: "$Amount"
            - name: TIMEOUT_MS
              value: "$TimeoutMs"
            - name: USER_ID
              value: "1"
          command:
            - /bin/sh
            - -c
            - |
              set -e
              k6 run --summary-export=/tmp/summary.json /scripts/load.js
              echo __K6_SUMMARY_BEGIN__
              cat /tmp/summary.json
              echo __K6_SUMMARY_END__
          volumeMounts:
            - name: script
              mountPath: /scripts
      volumes:
        - name: script
          configMap:
            name: $cmName
"@

$reportDir = Split-Path -Parent $ReportPath
if ($reportDir) {
  New-Item -ItemType Directory -Force -Path $reportDir | Out-Null
}

$logPath = if ($reportDir) { Join-Path $reportDir "k6-job.log" } else { "k6-job.log" }

try {
  $resourceYaml | kubectl apply -f - | Out-Null

  try {
    kubectl -n $Namespace wait --for=condition=complete "job/$jobName" --timeout="${jobTimeout}s" | Out-Null
  } catch {
    $podOnFail = kubectl -n $Namespace get pods -l job-name=$jobName -o jsonpath='{.items[0].metadata.name}'
    if ($podOnFail) {
      kubectl -n $Namespace logs $podOnFail | Out-File $logPath -Encoding utf8
    }
    kubectl -n $Namespace describe "job/$jobName" | Out-File -Append $logPath -Encoding utf8
    throw "k6 job did not complete within ${jobTimeout}s; see $logPath"
  }

  $pod = kubectl -n $Namespace get pods -l job-name=$jobName -o jsonpath='{.items[0].metadata.name}'
  if (-not $pod) {
    throw "k6 job pod not found"
  }

  $logs = kubectl -n $Namespace logs $pod
  $logText = ($logs -join "`n")
  $logText | Out-File $logPath -Encoding utf8

  $begin = $logText.IndexOf("__K6_SUMMARY_BEGIN__")
  $end = $logText.IndexOf("__K6_SUMMARY_END__")
  if ($begin -lt 0 -or $end -lt 0 -or $end -le $begin) {
    throw "failed to parse k6 summary from logs"
  }

  $summaryJson = $logText.Substring($begin + "__K6_SUMMARY_BEGIN__".Length, $end - ($begin + "__K6_SUMMARY_BEGIN__".Length)).Trim()
  $summary = $summaryJson | ConvertFrom-Json

  function Get-MetricValue {
    param(
      [object]$Obj,
      [string]$Metric,
      [string]$Field,
      [double]$Default = 0
    )
    $metricObj = $Obj.metrics.$Metric
    if (-not $metricObj) {
      return $Default
    }
    $valueSource = $metricObj
    if ($metricObj.PSObject.Properties.Name -contains "values") {
      $valueSource = $metricObj.values
    }
    $prop = $valueSource.PSObject.Properties | Where-Object { $_.Name -eq $Field } | Select-Object -First 1
    if (-not $prop) {
      return $Default
    }
    return [double]$prop.Value
  }

  $success = [int64](Get-MetricValue -Obj $summary -Metric "app_success" -Field "count" -Default 0)
  $failed = [int64](Get-MetricValue -Obj $summary -Metric "app_failure" -Field "count" -Default 0)
  $status429 = [int64](Get-MetricValue -Obj $summary -Metric "app_status_429" -Field "count" -Default 0)
  $status503 = [int64](Get-MetricValue -Obj $summary -Metric "app_status_503" -Field "count" -Default 0)
  $measured = $success + $failed

  $avgMs = Get-MetricValue -Obj $summary -Metric "app_req_duration" -Field "avg" -Default 0
  $p50Ms = Get-MetricValue -Obj $summary -Metric "app_req_duration" -Field "med" -Default 0
  if ($p50Ms -eq 0) {
    $p50Ms = Get-MetricValue -Obj $summary -Metric "app_req_duration" -Field "p(50)" -Default 0
  }
  $p95Ms = Get-MetricValue -Obj $summary -Metric "app_req_duration" -Field "p(95)" -Default 0
  $p99Ms = Get-MetricValue -Obj $summary -Metric "app_req_duration" -Field "p(99)" -Default 0

  $qps = 0.0
  if ($DurationSeconds -gt 0) {
    $qps = [double]$measured / [double]$DurationSeconds
  }

  $report = [ordered]@{
    scenario = $Scenario
    url = $ServiceUrl
    load_mode = "in-cluster"
    concurrency = $Concurrency
    requests = 0
    warmup_seconds = $WarmupSeconds
    measurement_seconds = $DurationSeconds
    duration = $DurationSeconds
    target_rps = $TargetRps
    timeout_ms = $TimeoutMs
    success = $success
    failed = $failed
    measured_requests = $measured
    success_rate = if ($measured -gt 0) { [double]$success / [double]$measured } else { 0.0 }
    error_rate = if ($measured -gt 0) { [double]$failed / [double]$measured } else { 0.0 }
    qps = $qps
    avg_ms = $avgMs
    p50_ms = $p50Ms
    p95_ms = $p95Ms
    p99_ms = $p99Ms
    status_codes = [ordered]@{
      "200" = $success
      "429" = $status429
      "503" = $status503
    }
    fixed_request = $false
  }

  $report | ConvertTo-Json -Depth 8 | Out-File $ReportPath -Encoding utf8
  Write-Host "Report written: $ReportPath"
} finally {
  kubectl -n $Namespace delete job $jobName --ignore-not-found | Out-Null
  kubectl -n $Namespace delete configmap $cmName --ignore-not-found | Out-Null
}
