# Trading Loop V4: Observability and Regression Governance Design

## Background

Flash-Mall already has a strong distributed trading demo foundation: authentication, JWT/session validation, DTM SAGA order creation, Redis pre-deduct stock, product stock buckets, reliable delayed close, idempotent payment callback handling, RabbitMQ Outbox, Kubernetes deployment, HPA/PDB topology controls, and repeatable performance scripts.

Trading Loop V3 moves order lifecycle mutations into an order-rpc state machine and adds lifecycle Outbox events. V4 should not add more shop pages or unrelated business features. The next valuable step is to make the system easier to debug, prove, and protect from regressions.

V4 focuses on two production-grade capabilities:

1. Full-chain observability across HTTP, gRPC, MySQL, Redis, RabbitMQ, and business events.
2. Regression governance through fixed CI paths, smoke checks, and optional performance gates.

V4 is intentionally split into independent milestones. Phase A and Phase B are the minimum useful iteration. Phase C and Phase D can ship afterward without blocking the foundation.

## Current State

Relevant existing assets:

1. `app/entry/api/entry.go` and `app/order/rpc/order.go` expose pprof and Prometheus endpoints.
2. `app/entry/api/internal/metrics/metrics.go` already defines business counters and histograms.
3. `scripts/k8s/perf-collect.ps1`, `scripts/k8s/perf-reliable.ps1`, and `docs/PERF_RELIABILITY_PLAYBOOK.md` provide repeatable benchmark evidence.
4. `.github/workflows/ci.yml` runs Go checks, smoke tests, Docker builds, and a frontend build.
5. `scripts/ci/smoke-e2e.sh` exists as an end-to-end smoke entry point.
6. V3 branch `codex/trading-loop-v3` is the expected prerequisite because lifecycle status changes and lifecycle Outbox events are the observability targets.

Known gaps:

1. Trace correlation is not yet a first-class contract. `request_id`, `order_id`, and logs are useful, but they are not consistently tied to a trace id across service boundaries.
2. Metrics endpoints exist, but business metrics are not tied to traces or a central operational workflow. They do not make it easy to jump from a failed order to the exact cross-service trace.
3. RabbitMQ Outbox publishing and consumption do not expose enough per-event lifecycle telemetry.
4. CI currently references `web/package-lock.json` and `web` build paths, while the active frontend workspace is under `frontend/packages/shop`, `frontend/packages/admin`, and `frontend/packages/shared`.
5. Performance scripts are mature, but performance regression checks are not wired into a controlled CI/manual gate.

## Goals

1. Introduce OpenTelemetry tracing across entry-api, order-rpc, product-rpc, RabbitMQ publishing, and RabbitMQ consumption.
2. Define a stable correlation contract: `trace_id`, `request_id`, `order_id`, `payment_order_id`, and `event_id`.
3. Make key logs include correlation fields without rewriting all logging.
4. Add targeted metrics for lifecycle transitions, Outbox publishing, consumer results, and compensation failures where current metrics are too coarse.
5. Fix CI frontend paths so the actual Vite workspace builds are checked.
6. Add a lightweight smoke gate for the trading path.
7. Add a manual or scheduled performance regression workflow using existing benchmark scripts, with conservative thresholds and artifact upload.
8. Produce documentation that explains how to debug one failed order from API response to trace, logs, outbox rows, and benchmark evidence.

## Non-Goals

1. Do not split `payment-rpc` in V4. Payment service extraction is a later architecture iteration.
2. Do not introduce a full logging stack such as Loki unless a minimal local collector is already required by OpenTelemetry work.
3. Do not replace go-zero, DTM, RabbitMQ, Redis, or MySQL.
4. Do not make performance regression blocking on every pull request. CI-hosted performance results are noisy; V4 should start with a manual gate and documented thresholds.
5. Do not add new customer-facing frontend features beyond health/debug visibility if needed.
6. Do not build a full centralized log search platform such as ELK or Loki in this iteration.
7. Do not implement dynamic sampling control. Use static sampling config first.
8. Do not add spans to every internal function. Instrument service, component, transaction, and async boundaries first.

## Milestones

### Phase A: Baseline Governance

Scope:

1. Merge Trading Loop V3 into `main`.
2. Fix CI frontend workspace paths from `web` to `frontend`.
3. Ensure CI runs Go package tests, API contract validation, shop build, admin build, and Docker image builds.

Exit criteria:

1. `main` contains V3 lifecycle state machine and lifecycle Outbox changes.
2. GitHub Actions no longer references `web/package-lock.json`.
3. A CI run builds both `frontend/packages/shop` and `frontend/packages/admin`.

### Phase B: Observability Foundation

Scope:

1. Add OpenTelemetry config with tracing disabled by default in non-demo configs.
2. Add HTTP middleware and gRPC interceptors.
3. Add trace propagation through entry-api, order-rpc, and product-rpc.
4. Add a local Jaeger or OpenTelemetry collector path for demo use.

Exit criteria:

1. A create-order flow produces one trace containing entry-api, order-rpc, and product-rpc spans.
2. Logs on the same path include `trace_id` and `order_id` where available.
3. Tracing can be disabled by config without changing business behavior.

### Phase C: Async and Business Telemetry

Scope:

1. Add trace context to RabbitMQ message headers.
2. Add Outbox publish and consume spans.
3. Add low-cardinality Outbox and lifecycle metrics.
4. Add focused correlation logging on compensation and retry paths.

Exit criteria:

1. An `order.created` event can be traced from DB outbox row to publish attempt and consumer handling.
2. Consumers tolerate missing trace headers from old messages.
3. Outbox backlog and publish result metrics are visible on `/metrics`.

### Phase D: Regression Workflow and Playbooks

Scope:

1. Add deterministic smoke assertions for the trading path.
2. Add a manual `workflow_dispatch` performance regression workflow.
3. Upload benchmark, pprof, logs, K8s snapshots, and summary artifacts.
4. Write observability and CI regression playbooks.

Exit criteria:

1. Manual performance workflow produces a pass/warn/unstable summary artifact.
2. Smoke test verifies `orders`, `payment_order`, `order_status_log`, and `order_outbox` evidence.
3. A playbook shows how to debug one failed order from `order_id` to trace, logs, DB rows, and metrics.

## Architecture

V4 adds an observability layer around the existing services:

```text
client
  -> entry-api
      -> DTM SAGA
      -> order-rpc
          -> MySQL mall_order
          -> Redis pre-deduct shards
          -> RabbitMQ Outbox publisher
      -> product-rpc
          -> MySQL mall_product / product_stock_bucket
      -> RabbitMQ consumer
```

Trace context should move through:

1. HTTP headers into entry-api.
2. gRPC metadata from entry-api to order-rpc and product-rpc.
3. Outbox payload and RabbitMQ message headers from publisher to consumers.
4. Log fields and business metrics as exemplars or labels where cardinality is safe.

The observability layer must be additive. Business logic should not depend on tracing being available.

## Correlation Contract

Use these fields consistently:

| Field | Source | Purpose |
| --- | --- | --- |
| `trace_id` | OpenTelemetry runtime | Cross-service request correlation |
| `span_id` | OpenTelemetry runtime | Local operation correlation |
| `request_id` | order create request | Idempotency and user retry correlation |
| `order_id` | order domain | Business entity correlation |
| `payment_order_id` | payment_order row | Payment callback correlation |
| `event_id` | Outbox event | Message publish and consume correlation |
| `event_type` | Outbox event | Routing and event lifecycle analysis |
| `user_id` | JWT/session identity | Ownership and authorization analysis |

Rules:

1. Never put high-cardinality IDs into Prometheus labels except where the metric is explicitly debug-only and disabled by default.
2. Put high-cardinality IDs in traces and logs.
3. Outbox payload should include `trace_id` when available, but event idempotency must not depend on trace id.
4. If an incoming request lacks trace headers, entry-api creates a new trace.
5. If an async consumer receives an event without trace metadata, it starts a new trace and links the event id.

Default sampling:

1. Local demo: 100% sampling for short manual flows.
2. CI smoke: 100% sampling only for the smoke job.
3. Load/performance run: start with 1% sampling or disabled tracing, then run a second comparison with tracing enabled to measure overhead.
4. Production-like demo config: configurable static ratio, default 1%.

## Tracing Design

### entry-api

Add middleware around HTTP routes:

1. Extract incoming W3C trace context.
2. Start spans named by route pattern, not raw URL.
3. Add attributes: `http.method`, `http.route`, `user_id` when authenticated, `request_id` for order creation, and `order_id` when known.
4. Inject trace context into outbound gRPC calls.

### order-rpc

Add gRPC interceptors:

1. Extract trace metadata from incoming gRPC requests.
2. Start spans for each RPC method.
3. Add attributes for `order_id`, `product_id`, `amount`, lifecycle `from_status`, `to_status`, and `event_type` where available.
4. Trace MySQL transactions at the operation level without recording full SQL parameter values.
5. Trace Redis pre-deduct and rollback operations at script-call granularity.

### product-rpc

Add gRPC interceptors and stock-operation spans:

1. Trace `Deduct`, `DeductRollback`, and `RevertStock`.
2. Add attributes for `product_id`, `amount`, `bucket_idx`, `order_id`, and idempotent stock log result.
3. Expose pprof consistently with order services if not already enabled in runtime.

### RabbitMQ Outbox

Add spans for:

1. Claiming outbox batches.
2. Publishing each event.
3. Marking published or retry/dead status.

RabbitMQ message headers should include trace context and event metadata. Consumers should extract this context when handling deliveries.

Required RabbitMQ headers:

1. W3C `traceparent`
2. W3C `tracestate` when present
3. `x-event-id`
4. `x-event-type`
5. `x-order-id`

Validation must cover both new messages with headers and legacy messages without headers.

## Metrics Design

Keep current metrics and add narrow, low-cardinality metrics:

1. `flashmall_order_outbox_publish_total{event_type,result}`
2. `flashmall_order_outbox_backlog{status}`
3. `flashmall_order_outbox_publish_duration_seconds{event_type}`
4. `flashmall_order_lifecycle_transition_total{from_status,to_status,result}`
5. `flashmall_order_compensation_failure_total{type}`
6. `flashmall_order_event_consume_total{event_type,result}`

Avoid labels such as `order_id`, `request_id`, `trace_id`, and `user_id`.

## Logging Design

Add a small helper for structured correlation fields:

1. `TraceFields(ctx)` returns `trace_id` and `span_id` when present.
2. `OrderFields(ctx, orderID, requestID)` combines trace fields with business fields.
3. Existing log calls in critical paths should be updated gradually:
   - create order
   - payment callback
   - lifecycle transition
   - stock rollback
   - outbox publish retry/dead
   - RabbitMQ consume retry/duplicate/invalid

V4 should avoid large log refactors. Only critical trading paths need correlation fields.

## CI and Regression Governance

### CI Fixes

Update `.github/workflows/ci.yml`:

1. Use `frontend/package-lock.json` for Node cache.
2. Run `npm ci` in `frontend`.
3. Run `npm run build:shop` and `npm run build:admin`.
4. Run `go test ./app/entry/api/... ./app/order/rpc/... ./app/product/rpc/... ./app/auth/api/...`.
5. Run `goctl api validate -api app/entry/api/entry.api` if goctl is available or install it explicitly.
6. Keep Docker build checks for entry-api, order-rpc, and product-rpc.

This is Phase A work. It should be completed before deeper V4 instrumentation so every later patch gets the correct baseline checks.

### Smoke Gate

Keep smoke checks deterministic and short:

1. Start required dependencies.
2. Initialize schema and seed stock.
3. Run one order create path.
4. Run payment callback path.
5. After V3 is merged, run lifecycle path: pay, ship, confirm, request refund, approve refund.
6. Verify key rows: `orders`, `payment_order`, `order_status_log`, and `order_outbox`.

### Performance Gate

Add a manual workflow first:

1. Use GitHub Actions `workflow_dispatch` first. A scheduled run can be added only after manual results are stable.
2. Accept input: namespace, scenario, duration, warmup, target RPS, concurrency, baseline artifact.
3. Run existing `perf-reliable.ps1` or Linux-compatible script equivalent.
4. Upload raw benchmark JSON, pprof, pod snapshots, and summary report.
5. Compare candidate to baseline using conservative thresholds:
   - success rate must not drop below 99.9%
   - p95 regression over 20% is a warning
   - p99 regression over 30% is a warning
   - QPS regression over 10% is a warning
   - CV over 10% marks result as unstable, not failed

Blocking PRs on performance should remain a later decision after the manual workflow has stable data.

## Documentation Deliverables

Add or update:

1. `docs/OBSERVABILITY_PLAYBOOK.md`
2. `docs/CI_REGRESSION_GOVERNANCE.md`
3. `docs/PERF_RELIABILITY_PLAYBOOK.md` with CI/manual workflow references.

The observability playbook should answer:

1. How to find a trace from an `order_id`.
2. How to trace a failed refund approval.
3. How to connect an Outbox row to RabbitMQ publish and consume logs.
4. How to distinguish business failure, infrastructure failure, and retryable compensation failure.

## Rollout Plan

### Phase 1: Foundation

1. Merge Trading Loop V3 first.
2. Add OpenTelemetry configuration structs and default disabled config.
3. Add HTTP and gRPC tracing middleware/interceptors.
4. Add local collector or Jaeger service to `deploy/docker-compose.yml` and optional K8s manifests.

### Phase 2: Critical Path Instrumentation

1. Instrument create order, payment callback, lifecycle transition, stock operations, and Outbox publishing.
2. Add RabbitMQ trace propagation.
3. Add log correlation helpers and update critical logs.
4. Add low-cardinality metrics for Outbox and lifecycle results.

### Phase 3: Governance

1. Fix CI frontend workspace paths.
2. Add API validation and focused Go package checks.
3. Add smoke assertions for database and Outbox evidence.
4. Add manual performance workflow and artifact upload.

### Phase 4: Playbooks and Demo

1. Write observability and regression governance docs.
2. Add a scripted failure demo: stock rollback failure or Outbox retry.
3. Show one debug path from API response to trace to log to database evidence.

## Risks and Mitigations

| Risk | Mitigation |
| --- | --- |
| Tracing adds too much overhead | Keep static sampling config; compare benchmark runs with tracing disabled and enabled; target less than 10% p95 regression for demo load |
| Instrumentation pollutes business logic | Use middleware, interceptors, and small helper functions; avoid tracing decisions inside domain branches |
| Prometheus cardinality explosion | Keep IDs out of metric labels; put them in traces/logs |
| CI performance gate is flaky | Start as manual workflow with warnings and artifact upload, not a required PR blocker |
| RabbitMQ trace propagation breaks old messages | Consumers must tolerate missing trace headers and start a new trace; tests must cover both header-present and header-missing deliveries |
| RabbitMQ headers are not propagated correctly | Publisher and consumer tests must assert `traceparent`, `x-event-id`, `x-event-type`, and `x-order-id` behavior |
| V4 implemented before V3 merge causes drift | Treat V3 merge as a hard prerequisite for lifecycle-specific instrumentation; Phase A cannot pass until V3 is merged |

## Validation Plan

Backend checks:

```powershell
go test ./app/entry/api/... ./app/order/rpc/... ./app/product/rpc/... ./app/auth/api/... -count=1
goctl api validate -api app/entry/api/entry.api
```

Frontend checks:

```powershell
npm run build:shop --prefix frontend
npm run build:admin --prefix frontend
```

Observability checks:

1. Start local dependencies and collector.
2. Create an order with a known `request_id`.
3. Verify trace spans across entry-api, order-rpc, product-rpc, MySQL, Redis, and Outbox publish.
4. Verify logs contain the same `trace_id` and `order_id` on critical path entries.
5. Verify metrics expose Outbox backlog and lifecycle result counters.

Governance checks:

1. CI builds actual `frontend` workspace, not obsolete `web` path.
2. Smoke test proves at least one complete trading flow.
3. Manual performance workflow uploads raw evidence and summary report.

## Success Criteria

1. Given a known `order_id`, an engineer can find the related `trace_id` and critical logs within five minutes using the V4 playbook.
2. A create-order test flow produces a continuous trace containing at least entry-api HTTP span, order-rpc create span, product-rpc deduct span, MySQL operation span, Redis pre-deduct span, and Outbox publish span.
3. After V3 merge, one lifecycle flow produces trace/log evidence for pay, ship, confirm receipt, request refund, and approve refund.
4. Outbox publish retry and consumer duplicate paths increment visible metrics and emit logs with `event_id`, `event_type`, and `trace_id` when available.
5. CI validates Go packages, API contract, shop build, admin build, and Docker images without referencing obsolete `web` frontend paths.
6. Manual performance workflow produces archived raw artifacts and a pass/warn/unstable summary including QPS, p95, p99, success rate, and CV.
7. Benchmark comparison with tracing enabled shows less than 10% p95 regression under the selected demo load, or the report explicitly marks the overhead as a known tradeoff.
8. V4 documentation explains the operational debug path clearly enough for a live demo.

## Resume Value

Implemented full-chain observability and regression governance for a distributed flash-sale trading system, correlating HTTP, gRPC, database, Redis, RabbitMQ Outbox, metrics, logs, smoke tests, and performance evidence into a reproducible production-debug workflow.
