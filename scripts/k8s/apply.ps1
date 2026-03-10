param(
  [string]$Namespace = "flash-mall"
)

$ErrorActionPreference = "Stop"

kubectl apply -f k8s/00-namespace.yaml
kubectl apply -f k8s/deps/
kubectl apply -f k8s/jobs/
kubectl apply -f k8s/apps/

kubectl -n $Namespace get pods -o wide
