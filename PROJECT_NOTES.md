# Flash-Mall Compact Project Notes

This is the lightweight project map. Historical long-form notes are archived at
`docs/archive/PROJECT_NOTES_FULL_20260612.md`.

## Interview Thesis

Flash-Mall is an e-commerce flash-sale system evolved from a simple order demo
into a defensible distributed-systems project. The strongest story is not the
UI; it is correctness under concurrency, recoverability, observability, and
operational demo readiness.

## Current High-Value Capabilities

- Trading loop: order creation, payment success callback, order detail, timeout
  close, stock release, and idempotent retry behavior.
- Distributed consistency: DTM SAGA for pre-deduct, create-order, deduct-stock,
  plus compensating branches.
- Inventory reliability: Redis pre-deduct, SQL stock buckets, timeout release,
  and reconciliation scripts.
- Payment hardening: callback HMAC validation, strict
  `order_id + payment_order_id + out_trade_no` binding, paid amount validation,
  and `payment_callback_event` audit persistence.
- Account security: auth-service split, session/version model, audit direction,
  and entry-api as BFF.
- Event architecture: RabbitMQ with Outbox Pattern, publisher/consumer
  idempotency, and metrics.
- Observability: pprof, Prometheus metrics, benchmark scripts, and p50/p95/p99
  evidence paths.
- Demo readiness: Docker-based local dependencies, one-click scripts, and web
  pages for shop/admin/monitor/debug flows.
- Windows local startup now rebuilds fixed `.runtime\bin` service executables
  and signs them before launch; firewall rules stay stable because the paths do
  not change. Use `scripts/local/start-all.ps1 -PrepareOnly` to rebuild and
  sign without starting the full stack.
- Windows users can launch `scripts/local/launcher.ps1` for a visual startup
  selector. Daily development should prefer
  `scripts/local/start-all.ps1 -Fast -StartDockerDesktop`; use the full startup
  only when database initialization, Redis stock seeding, or frontend rebuilds
  are needed. If Docker Desktop is running but `docker info` hangs, use the
  launcher's "启动前重启 Docker" option or add `-RestartDockerDesktop`.
- Current local development is being moved to the WSL native workspace
  `/home/mildred/code/flash-mall`. Prefer
  `scripts/local/start-compose-all.sh` inside WSL, or
  `scripts\local\start-wsl-compose.ps1` from Windows PowerShell. Windows should
  access the mall through `http://127.0.0.1:8888`, not the WSL `172.x.x.x`
  address.
- The Windows GUI launcher now has dedicated `WSL 启动` and `WSL 停止` actions.
  The legacy `快速启动` and `完整启动` actions still run the Windows exe path.
- K8s local deployment should also run inside WSL. Prefer
  `scripts/k8s/dev-up.sh`, `scripts/k8s/dev-down.sh`,
  `scripts/k8s/dev-restart.sh`, `scripts/k8s/port-forward.sh`, and
  `scripts/k8s/health.sh`; keep the PowerShell K8s scripts as compatibility
  wrappers for the older Windows route.
- K8s `dev-up.sh` defaults to the local single-node profile and scales Go
  services to 1 replica after apply. Use `--profile demo` to keep multi-replica
  demo behavior.

## Best Interview Talking Points

- "I changed payment success from trusting callback parameters to validating
  the payment row binding and amount inside the order transaction."
- "I use Outbox to decouple local DB commit from message delivery, so event
  publishing can retry without corrupting the order transaction."
- "Timeout close uses conditional state transition and idempotent stock release,
  so pay-vs-close races do not double-release stock."
- "I keep performance claims tied to scripts and p95/p99 reports instead of
  unsupported numbers."

## Next Work Selection Rule

Prefer one complete business slice per task:

1. Payment reconciliation and refund safety.
2. Order fulfillment and supplier/shipping state machine.
3. Account risk/audit dashboard.
4. One-click demo script and guided demo data.
5. Observability trace propagation across API, RPC, RabbitMQ, and jobs.

## Files To Inspect First

- `app/entry/api/internal/handler/payorderhandler.go`
- `app/order/rpc/internal/logic/markorderpaidlogic.go`
- `app/entry/api/internal/logic/createorderlogic.go`
- `app/entry/api/job/closeorder.go`
- `app/product/rpc/internal/logic/*stock*.go`
- `scripts/k8s/init-db.sql`
- `deploy/docker-compose.yml`
- `scripts/local/start-compose-all.sh`
- `scripts/local/health-compose.sh`
- `scripts/k8s/dev-up.sh`
- `scripts/k8s/dev-restart.sh`
- `scripts/k8s/dev-down.sh`
- `scripts/k8s/apply.sh`
