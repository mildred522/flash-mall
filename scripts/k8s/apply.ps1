param(
  [string]$Namespace = "flash-mall"
)

$ErrorActionPreference = "Stop"

kubectl apply -f k8s/00-namespace.yaml

$runtimeSecretName = "flash-mall-runtime-secrets"
$runtimeSecretPath = "k8s/examples/runtime-secrets.yaml"
if (Test-Path $runtimeSecretPath) {
  kubectl apply -f $runtimeSecretPath
} else {
  kubectl -n $Namespace get secret $runtimeSecretName 2>$null
  if ($LASTEXITCODE -ne 0) {
    throw "missing Secret $runtimeSecretName. Create it from k8s/examples/runtime-secrets.example.yaml before applying k8s/apps/."
  }
}

kubectl apply -f k8s/deps/
kubectl -n $Namespace create configmap mysql-init-sql --from-file=schema.sql=scripts/k8s/schema.sql --dry-run=client -o yaml | kubectl apply -f -
kubectl -n $Namespace delete job mysql-init redis-seed --ignore-not-found
kubectl apply -f k8s/jobs/
kubectl -n $Namespace wait --for=condition=complete job/mysql-init --timeout=180s
kubectl apply -f k8s/apps/

kubectl -n $Namespace get pods -o wide
