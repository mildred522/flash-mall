# Flash Mall: Current Code State

This is the only active project document. It is derived from the current source
tree and deployment manifests, not from archived plans or logs.

## Runtime topology

- External HTTP service: `app/gateway/hertz`, default port `8889`.
- Kubernetes Ingress points to `hertz-gateway:8889`.
- Docker Compose keeps `entry-api` behind the `legacy-entry` profile.
- Core services: `auth-api`, `product-rpc`, `order-rpc`, `inventory-kitex`,
  MySQL, Redis, RabbitMQ, Etcd, and DTM.

## Ownership boundaries

- Hertz owns canonical external HTTP routes, request identity, role checks,
  response shaping, and static home/shop/admin pages.
- auth-api owns credentials, sessions, verification codes, and security audit.
- order-rpc owns order creation, payment state, refund state, outbox events,
  and SAGA branches.
- inventory-kitex owns explicit stock reserve/release/confirm/adjust commands.
- product-rpc and product tables own catalog metadata and product read models.

## Implemented Hertz route groups

- Shop: catalog, product list/detail, authenticated user addresses.
- Auth: login, registration, refresh, logout, verification-code and password
  flows proxied to auth-api.
- Orders: create, mock payment, signed payment callback, status, list/detail,
  cancel, refund request, and confirm receipt.
- Inventory: administrator and merchant stock audit, stock adjustment, and
  stock/card snapshot rebuild. Reserve/release/confirm remain internal
  `order-rpc -> inventory-kitex` commands rather than public Hertz routes.
- Admin: products, suppliers, promotions, campaigns, orders, refunds,
  reconciliation, events, dashboard, and auth-admin proxy operations.
- Merchant: application creation, dashboard, products, stock adjustment,
  stock audit, orders, shipping, and refunds.

## Functional migration status

All normal customer, admin, and merchant business route groups now have
canonical Hertz handlers. Merchant application administration is included:

- `GET /api/admin/merchants/applications`
- `POST /api/admin/merchants/applications/audit`

`entry-api` remains as a compatibility profile and as the Go-zero baseline for
the project's architecture-evolution narrative. Removing it is not required
for the Hertz/Kitex branch to be considered complete.

## Deliberately local or internal surfaces

- `/debug` and `/monitor` are local operational pages, not production API
  migration blockers.
- `/metrics` needs an internal-only exposure policy before being mounted on
  Hertz.
- Legacy static `/js/*` and `/styles/*` routes are not required by the current
  inlined shop/admin build artifacts.

## Verification already performed

- `go test ./...` and both front-end production builds have completed in the
  Hertz worktree.
- The default Compose topology has been started and the rebuilt Hertz image
  returned HTTP 200 for `/`, `/shop`, `/admin`, `/api/system/health`, and
  `/api/shop/catalog`.
- The catalog UI response mismatch was fixed in the shared front-end client:
  it now unwraps the Hertz response envelope without changing the direct
  auth-api response contract.

## Go-zero vs Hertz comparison plan

**Goal:** Produce a repeatable, evidence-backed comparison between the
unmodified `origin/main` Go-zero baseline and the current Hertz/Kitex
migration. The comparison must distinguish functional compatibility from
performance, operational, and architectural effects.

### Rules for a valid comparison

1. Pin the baseline to `origin/main` commit `344ebf9`; record the exact current
   Hertz commit plus its working-tree diff before execution.
2. Run the two variants sequentially, never concurrently. Both Compose files
   define the same fixed container names (`mysql`, `auth-api`, etc.), so a
   concurrent run would invalidate results.
3. Recreate an isolated database/Redis state for each run from the same
   initialization scripts and record image digests, configuration, host port,
   machine load, and start/end time.
4. Do not compare debug, monitor, or metrics pages as customer-facing
   compatibility targets. Compare public shop, authenticated user, merchant,
   and administrator routes.
5. Treat a response-envelope difference as compatible only when the front-end
   adapter and documented consumer contract produce the same rendered result.

### Phase 1 — Baseline preparation

- Use a fresh detached worktree at `origin/main` for the baseline; do not
  modify its application code. The existing
  `/home/mildred/.config/superpowers/worktrees/flash-mall/main-baseline`
  worktree has generated `web/*.html` changes from an earlier build and is not
  a clean comparison source.
- Build `origin/main` and start its Compose stack at `http://127.0.0.1:8888`.
  Capture `docker compose config --services`, image IDs, container health, and
  the startup time from `docker compose up` until the health endpoint answers.
- Export the initialized MySQL state and record Redis keys required by the
  seeded catalog. Preserve the export as the baseline fixture for the Hertz
  run; do not use a database previously modified by manual UI testing.

### Phase 2 — Functional contract matrix

Run each request once against the Go-zero baseline and once against Hertz,
using the same fixture and a fresh user where a write is needed. Record HTTP
status, normalized response body, latency, and rendered browser result.

| Area | Required checks |
| --- | --- |
| Public | `/`, `/shop`, `/admin`, health, catalog, product list/detail |
| Identity | verification-code send, register, login, `/api/auth/me`, logout, password reset |
| Customer | address upsert/list, create order, mock pay/callback, status, list/detail, cancel, refund, confirm receipt |
| Merchant | application, dashboard, product list/create, order list/ship, refunds, stock adjustment/audit |
| Administrator | login, dashboard, products, suppliers, promotions, campaigns, merchant-application audit, orders, refunds, reconciliation, events, users/security |
| Inventory-specific | Internal Kitex reserve/release/confirm commands plus Hertz administrator/merchant adjust, audit, and snapshot routes; mark these as new capability, not a baseline regression |

For every common route, compare semantic fields rather than volatile fields
such as request IDs, timestamps, generated order IDs, tokens, and trace IDs.
An endpoint is a failure if its operation, authorization rule, error meaning,
or rendered user-visible outcome differs.

### Phase 3 — End-to-end browser chains

Execute and capture screenshots/network traces for four flows in each variant:

1. Anonymous visitor opens the shop and sees the seeded products.
2. New customer registers, signs in, creates an address, places and pays an
   order, then sees it in the order list.
3. Merchant creates/updates a product or adjusts stock, ships an order, and
   reviews a refund.
4. Administrator signs in, audits a merchant application, changes catalog or
   promotion data, and verifies the storefront result.

The acceptance artifact for each chain is a short request trace plus before/
after screenshots. A raw HTTP 200 alone is not sufficient.

### Phase 4 — Measured runtime comparison

Use a fixed request corpus (catalog read, authenticated order detail, and
order create) with the same concurrency, duration, warm-up, database fixture,
and host conditions. Run three repetitions per variant and report median and
range for:

- startup-to-ready time;
- p50/p95/p99 latency and error rate;
- requests/second;
- gateway/entry CPU and memory;
- downstream RPC latency/error counts;
- MySQL, Redis, and RabbitMQ container resource use.

Read-heavy catalog traffic and write-heavy order creation must be reported
separately. The result is descriptive, not a claim that Hertz is faster,
unless the repeated measurements agree and no dependency bottleneck dominates.

### Phase 5 — Architecture and operational comparison

Document the actual deltas from source and manifests:

- Go-zero `entry-api` is a combined static host and BFF; Hertz becomes the
  canonical HTTP edge, with `entry-api` retained only behind `legacy-entry`.
- `origin/main` has `product-rpc` and `order-rpc`; the migration adds an
  explicit Kitex inventory owner and moves stock commands out of implicit
  product/order coupling.
- Compare route ownership, error/response normalization, tracing propagation,
  service discovery, deployment units, rollback procedure, and failure modes.
- Classify each delta as an improvement, neutral trade-off, or unresolved risk
  using the Phase 2–4 evidence rather than framework preference.

### Phase 6 — Decision report and cleanup gate

Deliver one comparison report containing the version table, environment,
contract matrix, browser evidence, metrics, architecture assessment, and an
explicit migration verdict:

- **Hertz branch validated** when all common critical chains pass, no
  unapproved response incompatibility remains, and rollback is documented;
- **retain compatibility profile with explicit gaps** when functionality is
  correct but runtime or operational evidence is incomplete;
- **block cutover** when an authorization, order, payment, inventory, or
  storefront regression remains.

The legacy entry service and Go-zero baseline remain available for comparison
and interview narration regardless of the Hertz branch verdict. Cleanup is a
separate optional decision, not a migration acceptance requirement.

## Docker build design (approved 2026-07-12)

### Problem statement

Repeated container-native builds produced 78.03 GB of reclaimable BuildKit
cache. All six Go images use the repository root as their build context and
run `COPY . .`; a change in any included file therefore invalidates every
selected service's source layer and the following `go build` step. The large
private cache records came primarily from Go compiler caches captured inside
those invalidated build steps.

The default local startup script already builds binaries on the WSL host, but
it always builds all six services. Its generic scratch image also omits the
external `/app/web` files required by Hertz. The optimized design must fix
both paths instead of replacing one incomplete path with another.

### Goals and constraints

- Rebuild and restart one changed service during normal local development.
- Keep CI and full validation builds reproducible inside Docker.
- Share Go module and compiler caches without committing them to ordinary
  image layers.
- Preserve the explicit source dependency boundary of each service.
- Keep BuildKit cache near 8 GB during rapid iteration.
- Never include MySQL or Redis volumes in automatic cleanup.
- Preserve Hertz static pages and the legacy Entry embedded web resources.
- Avoid introducing Air or Compose Watch until the simpler host-build path is
  measured and shown to be insufficient.

### Local development path

`scripts/local/build-compose-images.sh` and its PowerShell counterpart become
service-selective. With no service arguments they keep the current full-build
behavior; with service arguments they validate names and build only those
images. A single-service rebuild command then recreates only that Compose
service without rebuilding or restarting its dependencies.

The local runtime image remains separate from the reproducible builder image.
It contains the static Go binary, CA certificates, and timezone data. Every
generated local image context contains an `app` binary and an optional `web`
directory; the Hertz build populates `web` from the canonical Entry web asset
directory, while other services use an empty directory. This keeps one small
runtime Dockerfile without losing Hertz pages.

### Reproducible Docker and CI path

Service-specific Dockerfiles remain separate. A single generic Dockerfile is
not used because Order, Hertz, and Entry depend on different generated RPC and
Kitex packages, while Hertz also owns external runtime web files.

Every Go builder uses Dockerfile syntax with two shared cache mounts:

```dockerfile
RUN --mount=type=cache,id=flash-mall-go-mod,target=/go/pkg/mod \
    go mod download
RUN --mount=type=cache,id=flash-mall-go-build,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ...
```

Source copies follow the local dependency graph reported by `go list -deps`:

- Auth copies only `app/auth`.
- Product copies `app/common` and `app/product`.
- Inventory copies `app/common` and `app/inventory`.
- Order copies `app/common`, `app/order`, Product RPC generated clients, and
  Inventory Kitex generated clients.
- Hertz copies `app/common`, `app/gateway`, the required Product, Order, and
  Inventory generated clients, plus the canonical web assets.
- Entry copies `app/common`, `app/entry`, and the same generated clients.

The root `.dockerignore` also excludes documentation, Kubernetes manifests,
local runtime output, unrelated frontend workspaces, test output, and editor
state when those files are not required by an image build. Build failures from
a newly introduced local dependency are treated as a dependency-list update,
not worked around by restoring a repository-wide `COPY . .`.

### Cache lifecycle

After a successful iteration, cache cleanup runs only when inspection shows
that the configured budget is exceeded. The normal policy retains recent hot
cache while removing older records:

```bash
docker buildx prune --force \
  --filter 'until=48h' \
  --max-used-space 8gb \
  --reserved-space 2gb
```

Failed builds do not trigger automatic cleanup. At a milestone or major
branch transition, `docker buildx prune --all --force` may remove the complete
builder cache. Automatic scripts may remove dangling images, but they do not
run `docker system prune -a --volumes`. The Compose MySQL and Redis volumes
receive an explicit `keep=true` label and remain outside every cleanup path.

### Verification and acceptance

The implementation is accepted only when all of the following are observed:

1. A cold full Compose build succeeds for the default Hertz topology.
2. A no-change rebuild reuses dependency and compiler caches.
3. An Order-only source change rebuilds and recreates only `order-rpc`.
4. Ten representative single-service rebuilds leave at most about 8 GB of
   BuildKit cache after the iteration cleanup policy runs.
5. The local fast path serves Hertz `/`, `/shop`, and `/admin` successfully.
6. The full customer order and payment path still reaches Inventory Kitex.
7. `deploy_mysql-data` and `deploy_redis-data` retain their contents before
   and after every cache-cleanup verification.
