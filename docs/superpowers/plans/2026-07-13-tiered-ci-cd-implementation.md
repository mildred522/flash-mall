# Tiered CI/CD Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace per-commit release-level validation with path-aware Fast CI and explicitly triggered Hertz + Kitex integration validation.

**Architecture:** Keep `.github/workflows/ci.yml` as the stable lightweight PR gate and move service-stack smoke plus Docker builds into `.github/workflows/full-ci.yml`. Parameterize the existing smoke runner so Hertz is the default target and legacy entry-api validation is opt-in. Keep release/deploy triggers protected while correcting their active service set.

**Tech Stack:** GitHub Actions, Bash, Go, Node.js, Docker Buildx, Hertz, Kitex, go-zero.

---

### Task 1: Build the path-aware Fast CI gate

**Files:**
- Modify: `.github/workflows/ci.yml`

- [ ] Replace unconditional smoke and Docker jobs with a `changes` job using `dorny/paths-filter@v3`.
- [ ] Run Go checks only for Go, API contract, Dockerfile, script, or workflow changes.
- [ ] Run `frontend` and `web` builds only when their directories change.
- [ ] Add an always-running `Fast CI` aggregate job that rejects failed or cancelled dependencies.
- [ ] Validate workflow YAML and confirm the old `smoke-e2e` and `docker-build` jobs are absent.

### Task 2: Make Hertz the primary smoke target

**Files:**
- Modify: `scripts/ci/smoke-e2e.sh`

- [ ] Add `FLASH_MALL_SMOKE_GATEWAY`, defaulting to `hertz` and accepting only `hertz` or `entry`.
- [ ] Keep the shared dependency and Kitex-enabled Order RPC startup path unchanged.
- [ ] Generate and start a Hertz smoke config for the default mode on port `8889`.
- [ ] Retain the existing entry-api config path only for explicit legacy mode on port `8888`.
- [ ] Route health, registration, and order requests through the selected gateway base URL.
- [ ] Run `bash -n scripts/ci/smoke-e2e.sh` and `git diff --check`.

### Task 3: Add explicitly triggered Full Integration CI

**Files:**
- Create: `.github/workflows/full-ci.yml`

- [ ] Trigger on manual dispatch, nightly schedule, and relevant labeled PR activity.
- [ ] Gate PR jobs on the `full-ci` label while allowing manual and scheduled runs.
- [ ] Run the default Hertz smoke test as the primary integration job.
- [ ] Run legacy entry smoke only for `legacy-ci` or the manual `run_legacy` input.
- [ ] Detect changed service paths and emit a JSON Docker matrix.
- [ ] Build selected images after primary smoke with GitHub Actions cache and `push: false`.

### Task 4: Align release and deployment with Hertz

**Files:**
- Modify: `.github/workflows/release-images.yml`
- Modify: `.github/workflows/deploy-k8s.yml`

- [ ] Add `hertz-gateway` to the release image matrix.
- [ ] Apply `k8s/apps/09-hertz-gateway.yaml` during deployment.
- [ ] Update the Hertz image and wait for its rollout.
- [ ] Preserve the existing protected release and deployment triggers.

### Task 5: Verify and publish

**Files:**
- Verify: `.github/workflows/*.yml`
- Verify: `scripts/ci/smoke-e2e.sh`

- [ ] Parse all workflow files as YAML using the available Python YAML runtime.
- [ ] Run shell syntax checks and repository diff checks.
- [ ] Run `go vet ./...` and `go test ./... -count=1` because CI topology and smoke behavior touch all Go services.
- [ ] Commit the implementation and push `codex/arch-hertz-kitex`.
- [ ] Observe Fast CI to completion; manually dispatch Full Integration CI on this feature branch and manage failures until it completes.
- [ ] Do not merge or deploy to `main`.
