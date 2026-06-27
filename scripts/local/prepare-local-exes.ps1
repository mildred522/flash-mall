param(
  [string]$OutputDir = "",
  [string]$CertSubject = "CN=Flash Mall Local Dev Code Signing",
  [switch]$SkipSigning,
  [switch]$TrustCert,
  [switch]$TrustRootCert,
  [switch]$SkipFirewall,
  [switch]$ForceCert
)

$ErrorActionPreference = "Stop"

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
if ($OutputDir -eq "") {
  $OutputDir = Join-Path $repoRoot ".runtime\bin"
}
$OutputDir = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($OutputDir)

$services = @(
  @{ Name = "auth-api";    Package = "./app/auth/api";    Exe = "auth-api.exe";    Ports = @(8890) },
  @{ Name = "product-rpc"; Package = "./app/product/rpc"; Exe = "product-rpc.exe"; Ports = @(8080) },
  @{ Name = "order-rpc";   Package = "./app/order/rpc";   Exe = "order-rpc.exe";   Ports = @(8090) },
  @{ Name = "entry-api";   Package = "./app/entry/api";   Exe = "entry-api.exe";   Ports = @(8888, 6060, 9090) }
)

function Test-IsAdministrator {
  $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
  $principal = [Security.Principal.WindowsPrincipal]::new($identity)
  return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Get-CodeSigningCertificate {
  param(
    [string]$Subject,
    [switch]$ForceNew
  )

  if (-not $ForceNew) {
    $existing = Get-ChildItem Cert:\CurrentUser\My -CodeSigningCert |
      Where-Object { $_.Subject -eq $Subject -and $_.NotAfter -gt (Get-Date).AddDays(30) } |
      Sort-Object NotAfter -Descending |
      Select-Object -First 1
    if ($existing) {
      return $existing
    }
  }

  return New-SelfSignedCertificate `
    -Type CodeSigningCert `
    -Subject $Subject `
    -CertStoreLocation Cert:\CurrentUser\My `
    -KeyExportPolicy Exportable `
    -KeyUsage DigitalSignature `
    -NotAfter (Get-Date).AddYears(5)
}

function Trust-CodeSigningCertificate {
  param(
    [System.Security.Cryptography.X509Certificates.X509Certificate2]$Certificate,
    [switch]$Root
  )

  $tmp = Join-Path $env:TEMP "flash-mall-local-dev-code-signing.cer"
  Export-Certificate -Cert $Certificate -FilePath $tmp -Force | Out-Null
  & certutil.exe -user -addstore TrustedPublisher $tmp | Out-Null
  if ($LASTEXITCODE -ne 0) {
    throw "certutil failed to import certificate into CurrentUser\TrustedPublisher"
  }
  if ($Root) {
    & certutil.exe -user -addstore Root $tmp | Out-Null
    if ($LASTEXITCODE -ne 0) {
      throw "certutil failed to import certificate into CurrentUser\Root"
    }
  }
  Remove-Item $tmp -Force -ErrorAction SilentlyContinue

  $publisher = Get-ChildItem Cert:\CurrentUser\TrustedPublisher |
    Where-Object { $_.Thumbprint -eq $Certificate.Thumbprint } |
    Select-Object -First 1
  if (-not $publisher) {
    Write-Warning "Certificate was not found in CurrentUser\TrustedPublisher after import."
  }

  if ($Root) {
    $rootCert = Get-ChildItem Cert:\CurrentUser\Root |
      Where-Object { $_.Thumbprint -eq $Certificate.Thumbprint } |
      Select-Object -First 1
    if (-not $rootCert) {
      Write-Warning "Certificate was not found in CurrentUser\Root after import."
    }
  }
}

function Set-FirewallRuleForExe {
  param(
    [string]$Name,
    [string]$ExePath,
    [int[]]$Ports
  )

  $ruleName = "Flash Mall Local Dev - $Name"
  Get-NetFirewallRule -DisplayName $ruleName -ErrorAction SilentlyContinue | Remove-NetFirewallRule

  New-NetFirewallRule `
    -DisplayName $ruleName `
    -Direction Inbound `
    -Action Allow `
    -Program $ExePath `
    -Protocol TCP `
    -LocalPort $Ports `
    -Profile Private,Domain `
    -Description "Allow stable Flash Mall local development executable $Name" | Out-Null
}

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
  throw "go not found in PATH"
}

New-Item -ItemType Directory -Path $OutputDir -Force | Out-Null
Set-Location $repoRoot

$cert = $null
if (-not $SkipSigning) {
  $cert = Get-CodeSigningCertificate -Subject $CertSubject -ForceNew:$ForceCert
  if ($TrustCert -or $TrustRootCert) {
    Trust-CodeSigningCertificate -Certificate $cert -Root:$TrustRootCert
  }
}

foreach ($svc in $services) {
  $exePath = Join-Path $OutputDir $svc.Exe
  Write-Host "[BUILD] $($svc.Name) -> $exePath"
  & go build -o $exePath $svc.Package
  if ($LASTEXITCODE -ne 0) {
    throw "go build failed for $($svc.Name)"
  }

  if ($cert) {
    Write-Host "[SIGN] $($svc.Name)"
    $signature = Set-AuthenticodeSignature -FilePath $exePath -Certificate $cert -HashAlgorithm SHA256
    if ($signature.Status -notin @("Valid", "UnknownError")) {
      throw "signing failed for $($svc.Name): $($signature.Status) $($signature.StatusMessage)"
    }
  }
}

$isAdmin = Test-IsAdministrator
if (-not $SkipFirewall) {
  if ($isAdmin) {
    foreach ($svc in $services) {
      Set-FirewallRuleForExe -Name $svc.Name -ExePath (Join-Path $OutputDir $svc.Exe) -Ports $svc.Ports
      Write-Host "[FIREWALL] allow $($svc.Name) ports=$($svc.Ports -join ',')"
    }
  } else {
    Write-Warning "Not running as Administrator; firewall rules were not changed. Re-run this script in an elevated PowerShell to suppress Windows Defender Firewall prompts."
  }
}

Write-Host ""
Write-Host "Prepared stable local executables in: $OutputDir"
Write-Host "Start script will prefer these executables over go run when they exist."
