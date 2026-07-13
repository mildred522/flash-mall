# Tiered CI/CD Design

## Goal

Keep pull-request feedback fast during active development while preserving a deliberate, reproducible Hertz + Kitex system validation path for milestones and releases.

## Current Problem

The existing `CI` workflow couples four different concerns on every pull-request update:

- Go and frontend feedback;
- a full dependency stack and end-to-end smoke test;
- five Docker image builds;
- legacy `entry-api` validation.

This makes routine commits pay the cost of release-level validation. It also treats the legacy entry API as the primary gateway even though the active architecture uses Hertz and Inventory Kitex.

## Design

### Fast CI

`CI` remains the stable pull-request gate. It detects changed paths and runs only relevant jobs:

- Go vet, tests, and binary builds for Go or workflow changes, backed by one ephemeral MySQL service required by the existing repository tests;
- shop/admin frontend builds only for `frontend/**` changes;
- embedded web build only for `web/**` changes;
- API contract parsing only when the corresponding contract changes;
- shell and workflow syntax validation for CI changes.

An always-present aggregate job exposes one stable `Fast CI` result for branch protection. It accepts successful or intentionally skipped component jobs and rejects cancellations or failures. Fast CI does not start the full Etcd, DTM, Redis, RabbitMQ, or application-service stack.

### Full Integration CI

A separate `Full Integration CI` workflow runs when:

- a pull request has the `full-ci` label;
- a labeled pull request receives a new commit;
- it is manually dispatched;
- the nightly schedule runs.

The primary smoke target is `hertz-gateway`. The smoke stack starts Product RPC, Order RPC, Inventory Kitex, Auth API, DTM, MySQL, Redis, Etcd, and RabbitMQ, then verifies registration and order creation through Hertz. Order RPC is forced to use Kitex inventory reservation.

Legacy `entry-api` smoke is non-blocking for normal development and runs only through an explicit manual option or a `legacy-ci` pull-request label.

### Docker Validation

Docker image builds happen only in Full Integration CI after the primary smoke passes. Pull-request runs build only affected service images. Manual runs can request all images. Nightly runs skip image builds to avoid repeating expensive work without a release intent.

The active image set includes `hertz-gateway`. `entry-api` remains available as a legacy image but is not the primary gateway.

### Release and Deployment

Release image publication includes Hertz. Demo deployment applies, updates, and waits for Hertz alongside the other active services. Existing release triggers remain limited to `main`, `develop`, tags, and explicit manual dispatch; this branch will not be merged or deployed by this work.

## Failure and Safety Rules

- Fast CI must fail only for checks relevant to the changed files.
- Full smoke owns and cleans only its isolated runner containers and processes.
- The smoke script defaults to Hertz; legacy mode must be explicitly selected.
- Release and deploy workflows keep their current permissions and do not gain automatic feature-branch deployment.
- Docker builds use BuildKit GitHub Actions cache and never push from validation workflows.

## Success Criteria

- A documentation-only pull-request update completes Fast CI without Go, Node, smoke, or Docker work.
- A Go change runs Go checks but not full integration by default.
- A `full-ci` labeled pull request validates the Hertz + Kitex order path.
- A `legacy-ci` label or manual option can validate `entry-api` independently.
- Release and deploy definitions include Hertz without changing their protected triggers.
