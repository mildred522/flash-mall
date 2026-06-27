#!/usr/bin/env sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(CDPATH= cd -- "$script_dir/../.." && pwd)
namespace="${1:-${FLASH_MALL_K8S_NAMESPACE:-flash-mall}}"
runtime_secret_name="flash-mall-runtime-secrets"
runtime_secret_path="$repo_root/k8s/examples/runtime-secrets.yaml"

cd "$repo_root"

command -v kubectl >/dev/null 2>&1 || {
  echo "kubectl not found in PATH" >&2
  exit 1
}

kubectl apply -f k8s/00-namespace.yaml

if [ -f "$runtime_secret_path" ]; then
  kubectl apply -f "$runtime_secret_path"
elif ! kubectl -n "$namespace" get secret "$runtime_secret_name" >/dev/null 2>&1; then
  mysql_root_password="${FLASH_MALL_MYSQL_ROOT_PASSWORD:-flashmall}"
  rabbitmq_user="${FLASH_MALL_RABBITMQ_USER:-flashmall}"
  rabbitmq_password="${FLASH_MALL_RABBITMQ_PASSWORD:-flashmall-local}"
  jwt_secret="${FLASH_MALL_JWT_AUTH_SECRET:-flash-mall-local-jwt-secret}"
  payment_secret="${FLASH_MALL_PAYMENT_CALLBACK_SECRET:-flash-mall-local-payment-secret}"
  demo_password="${FLASH_MALL_DEMO_PASSWORD:-flashmall123}"

  echo "[SECRET] create local development $runtime_secret_name"
  kubectl -n "$namespace" create secret generic "$runtime_secret_name" \
    --from-literal=mysql-root-password="$mysql_root_password" \
    --from-literal=order-datasource="root:${mysql_root_password}@tcp(mysql:3306)/mall_order?charset=utf8mb4&parseTime=true&loc=Asia%2FShanghai" \
    --from-literal=auth-datasource="root:${mysql_root_password}@tcp(mysql:3306)/mall_auth?charset=utf8mb4&parseTime=true&loc=Asia%2FShanghai" \
    --from-literal=product-datasource="root:${mysql_root_password}@tcp(mysql:3306)/mall_product?charset=utf8mb4&parseTime=true&loc=Asia%2FShanghai" \
    --from-literal=rabbitmq-user="$rabbitmq_user" \
    --from-literal=rabbitmq-password="$rabbitmq_password" \
    --from-literal=rabbitmq-url="amqp://${rabbitmq_user}:${rabbitmq_password}@rabbitmq.flash-mall.svc.cluster.local:5672/" \
    --from-literal=jwt-auth-secret="$jwt_secret" \
    --from-literal=payment-callback-secret="$payment_secret" \
    --from-literal=demo-password="$demo_password"
fi

kubectl apply -f k8s/deps/
kubectl -n "$namespace" create configmap mysql-init-sql \
  --from-file=init-db.sql=scripts/k8s/init-db.sql \
  --dry-run=client -o yaml | kubectl apply -f -
kubectl apply -f k8s/jobs/
kubectl apply -f k8s/apps/

kubectl -n "$namespace" get pods -o wide
