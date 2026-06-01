# Agent Task Board

This file is the single source of truth for multi-agent collaboration in this repo.

## Rules

1. Before starting a task, an agent must claim it in the task table.
2. No code changes are allowed before the task row is updated to `IN_PROGRESS`.
3. After finishing a task, the agent must update:
   - `Status`
   - `Last Update`
   - `Commit / Branch`
   - `Verification`
   - `Handoff Notes`
4. Do not silently edit shared files without recording it here first.
5. If a task expands beyond its original scope, split it into a new task row instead of stretching the old one.
6. If blocked, set `Status` to `BLOCKED` and describe the blocker clearly.
7. If a task changes API contracts, state the exact request/response change in `Handoff Notes`.
8. A task is not complete until `Verification` and `Commit / Branch` are filled in.
9. If an agent needs to touch a task owned by another agent, it must add a note in `Handoff Notes` first and wait for explicit coordination.
10. When a status sweep shows all tasks in the current round are `DONE`, the next agent action must be main-branch integration before any new feature planning.

## Strict Status Model

Only these statuses are allowed:

- `TODO`: not yet claimed
- `IN_PROGRESS`: claimed and actively being worked on
- `REVIEW`: implementation finished, waiting for inspection or integration
- `BLOCKED`: cannot proceed until a blocker is resolved
- `DONE`: completed, verified, and handed off

Invalid patterns:

- changing code while the row is still `TODO`
- marking `DONE` without a commit or verification command
- keeping a task in `IN_PROGRESS` after implementation is already finished
- starting a new feature round while the completed round has not been integrated into the main integration branch

## Main Integration Protocol

Main integration branch:
- `main`

Automatic trigger:
- Every time an agent reads this board and finds that all tasks in the current round are `DONE`, it must stop feature work and integrate the completed round into the main integration branch.
- If an integration task already exists, the agent must claim that task instead of creating a duplicate.
- If no integration task exists, the agent must add one with the next `C*` ID, set it to `IN_PROGRESS`, and record the scope.

Required integration steps:

1. Confirm all source worktrees are clean or contain only documented board updates.
2. Confirm every completed task has a commit, branch, and verification command.
3. Integrate task commits into the main integration branch using the safest available method:
   - prefer cherry-pick for isolated task commits
   - use merge only when the branch history is already known to be safe
4. Resolve conflicts explicitly; never choose one side wholesale without reviewing the files.
5. Run the integration verification command recorded on the integration task.
6. Commit the integration result on the main integration branch.
7. Update the integration task to `DONE` with commit, verification, and handoff notes.

Safety gates:

- If conflicts are large or ambiguous, set the integration task to `BLOCKED` and document the exact files.
- If verification fails, keep the integration task `IN_PROGRESS` or `BLOCKED`; do not mark it `DONE`.
- Do not start the next functional workstream until integration is `DONE`.

## Claim Protocol

To claim a task, an agent must update the row with all of the following:

1. keep the assigned `Owner` unless the task is explicitly reassigned
2. set `Status` to `IN_PROGRESS`
3. set `Last Update`
4. write one line in `Handoff Notes` starting with `Claimed by ...`

Example:

`Claimed by Product Agent. Scope: storefront payment-state UI only. No RPC contract changes.`

When implementation is done:

1. set `Status` to `REVIEW` or `DONE`
2. fill `Commit / Branch`
3. fill `Verification`
4. replace the claim note with a real handoff summary

## Ownership

### Core Agent

Primary focus:
- `app/order/rpc/**`
- `app/product/rpc/**`
- `app/auth/api/**`
- `scripts/k8s/init-db.sql`
- transaction consistency, pricing, payment, timeout close, stock recovery, auth/session semantics

### Product Agent

Primary focus:
- `frontend/**`
- `web/**`
- `app/order/api/internal/handler/web/**`
- `app/order/api/internal/handler/admin*`
- `app/order/api/internal/handler/monitor*`
- thin order-api handlers for UI flows
- dashboard, storefront, admin, monitor, visual polish

### Shared Files

These require explicit coordination before editing:
- `app/order/api/internal/handler/routes.go`
- `app/order/api/internal/types/types.go`
- `app/order/api/internal/svc/servicecontext.go`
- `app/order/api/internal/config/config.go`

## Current Claim Order

### Core Agent

Claim in this order:

1. `A1`
2. `A2`
3. `A3`

### Product Agent

Claim only these tasks, in this order:

1. `B1`
2. `B2`
3. `B3`

Product Agent should not claim `A*` tasks.
Core Agent should not offload `A*` transaction-truth work unless the board is explicitly updated first.

## Active Workstreams

### Workstream A: Trading Loop V2 Core

Owner: Core Agent

Goal:
- finish payment success callback
- add order detail read path
- harden timeout close and stock release
- complete idempotency and pay-vs-close race handling

### Workstream B: Storefront And Admin Productization

Owner: Product Agent

Goal:
- improve storefront product and order flows
- add payment status display and polling UI
- build admin pages for user/order/product/dashboard
- add monitor and metrics UI

### Workstream C: Mainline Integration

Owner: Core Agent

Goal:
- integrate completed A/B round into `main`
- resolve cross-branch API/UI conflicts
- prove the merged project can build and run through the core demo flow

## Task Board

| ID | Task | Owner | Status | Scope | Key Files | Verification | Commit / Branch | Last Update | Handoff Notes |
|---|---|---|---|---|---|---|---|---|---|
| A1 | Payment success callback and idempotency | Core Agent | DONE | Add mark-paid path and ensure repeated callbacks are safe | `app/order/rpc/**`, `app/order/api/internal/handler/payorderhandler.go` | `go test ./app/order/rpc/internal/logic ./app/order/api/internal/handler -run "MarkOrderPaid|PayOrderHandler" -count=1`; `go test ./app/order/rpc/... ./app/order/api/... -count=1` | `codex/trading-loop-v2 @ 64189fd` | 2026-06-01 14:50:00 | Added `MarkOrderPaid` RPC, idempotent pay transition logic, `/api/order/pay` handler, route wiring, and payment-order schema compatibility fields. |
| A2 | Order detail read path | Core Agent | DONE | Return snapshot-backed order/payment detail for UI and admin | `app/order/rpc/**`, `app/order/api/internal/handler/orderdetailhandler.go` | `go test ./app/order/rpc/internal/logic ./app/order/api/internal/handler -run "MarkOrderPaid|GetOrderDetail|PayOrderHandler|OrderDetailHandler" -count=1`; `go test ./app/order/rpc/... ./app/order/api/... -count=1` | `codex/trading-loop-v2 @ ed39829` | 2026-06-01 15:03:00 | Added `GetOrderDetail` RPC, snapshot-backed detail query, `/api/order/detail` handler, and protected route wiring. |
| A3 | Timeout close and stock release hardening | Core Agent | DONE | Close unpaid orders only and release reserved stock safely | `app/order/api/job/**`, `app/product/rpc/internal/logic/revertstocklogic.go` | `go test ./app/order/api/job ./app/product/rpc/internal/logic -run "CloseOrder|RevertStock" -count=1`; `go test ./app/order/api/job ./app/product/rpc/internal/logic -count=1` | `codex/trading-loop-v2 @ 9efab05` | 2026-06-01 17:37:25 | Added conditional close CAS to prevent pay-vs-close stock release races, allowed closed-order compensation replay, and fixed empty-order-id revert stock idempotency. |
| B1 | Storefront payment-state UX | Product Agent | DONE | Show pending-payment, paid, closed states and polling behavior in shop UI | `frontend/**`, `web/**`, `app/order/api/internal/handler/web/**` | `go build ./app/order/api/` + `node web/build.js` + `pnpm run build:shop` all pass | `codex/auth-service-baseline` | 2026-06-01 15:20:00 | Completed by Product Agent. Changes: web/js/order.js (polling+banner+goToOrders), web/js/bootstrap.js (use goToOrders), web/styles/shop.css (banner CSS), frontend shop App.tsx/HomePage.tsx/OrdersPage.tsx (polling+banner+auto-navigate). Both web/ and React frontends updated. |
| B2 | Admin dashboard and order/product/user pages | Product Agent | DONE | Build admin list/detail pages and handler wiring | `app/order/api/internal/handler/admin*`, `app/order/api/internal/handler/web/admin.html` | `go test ./app/order/api/internal/handler/ -count=1` passes | `codex/auth-service-baseline` | 2026-06-01 15:35:00 | Completed by Product Agent. Changes: web/admin.html (new), web/js/admin.js (new), web/styles/admin.css (new), web/build.js (added admin page), webuihandler_test.go (case-insensitive HTML check). Admin page has dashboard/orders/products/users tabs. |
| B3 | Monitor and metrics UI | Product Agent | DONE | Add monitor page and metrics display | `app/order/api/internal/handler/monitor*`, `web/**` | `go test ./app/order/api/internal/handler/ -count=1` passes (13/13) | `codex/auth-service-baseline` | 2026-06-01 15:45:00 | Completed by Product Agent. Changes: monitoruihandler.go (enhanced with Prometheus metrics parsing, summary cards, full metrics table), webuihandler_test.go (added TestMonitorUIReturnsHTML). Monitor page shows health + dependencies + business metrics. |
| C0 | Integrate completed A/B round into main | Core Agent | TODO | Bring `codex/auth-service-baseline` and `codex/trading-loop-v2` work into `main` safely | shared API contracts, generated proto files, route wiring, frontend/web assets | `go test ./app/order/rpc/... ./app/order/api/... ./app/product/rpc/... ./app/auth/api/... -count=1`; frontend/web build commands if package managers are available | | 2026-06-01 17:45:00 | Auto-created after A/B completion sweep. Next agent must claim this before starting new feature work. |

## Update Log

Append new entries at the top.

### 2026-06-01

- Time: 17:45:00
- Task ID: Board Rule
- Status: DONE
- Commit / Branch: `codex/auth-service-baseline`
- Verification:
  - `git diff -- AGENT_TASK_BOARD.md`
- Summary: added automatic main integration protocol and created `C0` as the required integration task after the completed A/B round.
- Follow-up / Risks: next agent action must claim `C0`; no new feature work should start before `C0` is integrated and verified.
- Time: 17:37:25
- Task ID: A3
- Status: DONE
- Commit / Branch: `codex/trading-loop-v2 @ 9efab05`
- Verification:
  - `go test ./app/order/api/job ./app/product/rpc/internal/logic -run "CloseOrder|RevertStock" -count=1`
  - `go test ./app/order/api/job ./app/product/rpc/internal/logic -count=1`
- Summary: hardened timeout close with conditional DB status transition before stock release, preserved idempotent compensation replay for closed orders, and fixed `RevertStock` fallback key consistency.
- Follow-up / Risks: A3 is ready for integration; branch still needs merge coordination with Product Agent work on `codex/auth-service-baseline`.
- Time: 15:03:00
- Task ID: A2
- Status: DONE
- Commit / Branch: `codex/trading-loop-v2 @ ed39829`
- Verification:
  - `go test ./app/order/rpc/internal/logic ./app/order/api/internal/handler -run "MarkOrderPaid|GetOrderDetail|PayOrderHandler|OrderDetailHandler" -count=1`
  - `go test ./app/order/rpc/... ./app/order/api/... -count=1`
- Summary: added snapshot-backed order detail RPC, API handler, and protected route wiring.
- Follow-up / Risks: Product Agent can now wire payment-state UI to `/api/order/pay` and `/api/order/detail`.
- Time: 14:50:00
- Task ID: A1
- Status: DONE
- Commit / Branch: `codex/trading-loop-v2 @ 64189fd`
- Verification:
  - `go test ./app/order/rpc/internal/logic ./app/order/api/internal/handler -run "MarkOrderPaid|PayOrderHandler" -count=1`
  - `go test ./app/order/rpc/... ./app/order/api/... -count=1`
- Summary: added payment callback idempotency RPC, pay handler, route wiring, and payment-order schema compatibility fields.
- Follow-up / Risks: storefront pay-now UX and order detail display remain for later tasks.
- Tightened the board into strict mode with required status flow and claim protocol.
- Product Agent claim order is now explicit: `B1 -> B2 -> B3`.
- Initialized task board.
- Current split:
  - Core Agent owns transaction truth and backend consistency.
  - Product Agent owns storefront/admin/monitor productization.
- Rule: every completed task must update the table above before handoff.

## Completion Template

Copy this block when updating a task:

```md
- Time:
- Task ID:
- Status:
- Commit / Branch:
- Verification:
- Summary:
- Follow-up / Risks:
```
