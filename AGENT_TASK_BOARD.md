# Agent Coordination Archive

This file is no longer an active task board.

The multi-agent collaboration mechanism has been retired. Future work in this
repository is coordinated directly by Codex in the current thread. Local MCP
tools, including Local MCP Brain `delegate_code`, may be used only as auxiliary
drafting or analysis tools. They do not own tasks, impose board updates, or
create integration obligations.

## Current Rules

- No task claiming is required before editing code.
- No status table update is required before or after implementation.
- No automatic main-branch integration is triggered by this file.
- No external agent owns project work.
- Codex remains responsible for final implementation decisions, file edits,
  verification, and commits.
- Before starting a new plan after completed work, Codex should still check
  whether the current worktree is clean and ask whether the user wants cleanup
  if tracked changes remain.

## Historical Completed Work

These entries are retained only as project history.

| ID | Work | Result | Commit / Branch | Verification |
|---|---|---|---|---|
| A1 | Payment success callback and idempotency | Added `MarkOrderPaid` RPC, idempotent pay transition logic, `/api/order/pay` handler, route wiring, and payment-order schema compatibility fields. | `codex/trading-loop-v2 @ 64189fd` | `go test ./app/order/rpc/internal/logic ./app/order/api/internal/handler -run "MarkOrderPaid|PayOrderHandler" -count=1`; `go test ./app/order/rpc/... ./app/order/api/... -count=1` |
| A2 | Order detail read path | Added snapshot-backed order detail RPC, API handler, and protected route wiring. | `codex/trading-loop-v2 @ ed39829` | `go test ./app/order/rpc/internal/logic ./app/order/api/internal/handler -run "MarkOrderPaid|GetOrderDetail|PayOrderHandler|OrderDetailHandler" -count=1`; `go test ./app/order/rpc/... ./app/order/api/... -count=1` |
| A3 | Timeout close and stock release hardening | Added conditional close CAS to prevent pay-vs-close stock release races, allowed closed-order compensation replay, and fixed empty-order-id revert stock idempotency. | `codex/trading-loop-v2 @ 9efab05` | `go test ./app/order/api/job ./app/product/rpc/internal/logic -run "CloseOrder|RevertStock" -count=1`; `go test ./app/order/api/job ./app/product/rpc/internal/logic -count=1` |
| B1 | Storefront payment-state UX | Added pending-payment, paid, closed states and polling behavior in shop UI. | `codex/auth-service-baseline` | `go build ./app/order/api/`; `node web/build.js`; `pnpm run build:shop` |
| B2 | Admin dashboard and order/product/user pages | Added admin dashboard, order, product, and user pages. | `codex/auth-service-baseline` | `go test ./app/order/api/internal/handler/ -count=1` |
| B3 | Monitor and metrics UI | Added monitor page and Prometheus metrics display. | `codex/auth-service-baseline` | `go test ./app/order/api/internal/handler/ -count=1` |
| C0 | Mainline integration | Integrated the completed A/B round into `main`. | `main @ de289e0` | `go test ./app/order/rpc/... ./app/order/api/... ./app/product/rpc/... ./app/auth/api/... -count=1`; `node web/build.js`; `pnpm run build:shop`; `pnpm run build:admin` |
| D1 | Payment callback validation hardening | Added callback request fields, configured-secret HMAC validation, RPC order/payment/out_trade_no binding, payable amount validation, and `payment_callback_event` audit persistence. | `main @ bb5ad39` | `go test ./app/order/api/internal/...`; `go test ./app/order/rpc/internal/logic` |

## Historical Note

Earlier versions of this file enforced strict task ownership, claim protocol,
status transitions, and automatic integration. Those rules were useful while
two agents worked concurrently, but they are now intentionally inactive because
the project uses a single Codex owner with optional MCP assistance.
