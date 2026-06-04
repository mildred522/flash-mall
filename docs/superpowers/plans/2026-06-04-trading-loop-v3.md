# Trading Loop V3 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move order lifecycle changes from scattered order-api SQL into an order-rpc state machine with lifecycle Outbox events, cancellation/refund reliability, and frontend role cleanup.

**Architecture:** order-api remains the HTTP boundary and calls order-rpc for all lifecycle mutations. order-rpc owns state transitions, status logs, Outbox writes, and refund/cancel orchestration using existing idempotent stock rollback APIs and the existing CloseOrderJob retry path.

**Tech Stack:** Go 1.24, go-zero REST/RPC, gRPC/protobuf, MySQL via `database/sql`, Redis, RabbitMQ Outbox, React/Vite frontend.

---

## File Map

- Modify `app/order/rpc/order.proto`: add lifecycle messages and RPC methods.
- Regenerate `app/order/rpc/order/*.pb.go`, `app/order/rpc/orderclient/order.go`, and `app/order/rpc/internal/server/orderServer.go` with `goctl rpc protoc`.
- Create `app/order/rpc/internal/logic/orderstate.go`: status constants, status text, transition validation, shared event payload helpers.
- Modify `app/order/rpc/internal/job/outbox_publisher.go`: add generic `InsertOrderEventOutbox`.
- Create `app/order/rpc/internal/logic/lifecyclelogic.go`: shared transactional transition helpers.
- Create lifecycle RPC logic files under `app/order/rpc/internal/logic`: `payorderlogic.go`, `shiporderlogic.go`, `confirmreceiptlogic.go`, `cancelorderlogic.go`, `requestrefundlogic.go`, `approverefundlogic.go`.
- Add/extend tests in `app/order/rpc/internal/logic`: state machine, lifecycle transitions, Outbox rows, refund failure/retry.
- Modify order-api lifecycle logic files to delegate to order-rpc.
- Modify `app/order/api/internal/types/types.go`, `app/order/api/internal/handler/routes.go`, `app/order/api/internal/handler/adminorderhandler.go`, and `app/order/api/order.api`.
- Sync `app/order/api/desc/order.sql` to current order tables.
- Modify `frontend/packages/shop/src/pages/OrdersPage.tsx` and `frontend/packages/shop/src/components/OrderCard.tsx`.
- Modify `frontend/packages/admin/src/pages/OrdersPage.tsx` and shared status constants if needed.

---

### Task 1: Add State Machine Tests

**Files:**
- Create: `app/order/rpc/internal/logic/orderstate_test.go`
- Create later: `app/order/rpc/internal/logic/orderstate.go`

- [ ] **Step 1: Write failing transition tests**

Create tests that define the expected allowed actions before implementation:

```go
func TestOrderStateTransitions(t *testing.T) {
	tests := []struct {
		name string
		from int64
		to   int64
		want bool
	}{
		{"pending to paid", 0, 1, true},
		{"pending to closed", 0, 2, true},
		{"paid to shipped", 1, 3, true},
		{"paid to refund requested", 1, 5, true},
		{"shipped to completed", 3, 4, true},
		{"refund requested to refunded", 5, 6, true},
		{"refund requested to failed", 5, 7, true},
		{"refund failed to refunded", 7, 6, true},
		{"pending to refund requested", 0, 5, false},
		{"completed to refund requested", 4, 5, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := canTransition(tt.from, tt.to)
			if got != tt.want {
				t.Fatalf("canTransition(%d,%d)=%v want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run red test**

Run: `go test ./app/order/rpc/internal/logic -run TestOrderStateTransitions -count=1`

Expected: FAIL because `canTransition` is undefined.

- [ ] **Step 3: Implement minimal state helper**

Create `orderstate.go` with integer constants matching the database and `canTransition`.

- [ ] **Step 4: Run green test**

Run: `go test ./app/order/rpc/internal/logic -run TestOrderStateTransitions -count=1`

Expected: PASS.

---

### Task 2: Add Generic Outbox Helper

**Files:**
- Modify: `app/order/rpc/internal/job/outbox_publisher.go`
- Test: extend existing lifecycle tests instead of adding a separate package-level DB test.

- [ ] **Step 1: Write failing lifecycle test expectation**

In the first lifecycle test, assert that a successful transition inserts a row into `order_outbox` with the expected `event_type`.

- [ ] **Step 2: Run red test**

Run the specific lifecycle test. Expected: FAIL because no generic helper or lifecycle transition exists.

- [ ] **Step 3: Implement `InsertOrderEventOutbox`**

Add:

```go
func InsertOrderEventOutbox(tx *sql.Tx, eventType, orderID, payload string) error {
	eventID := fmt.Sprintf("%s:%s", eventType, orderID)
	_, err := tx.Exec(
		`INSERT IGNORE INTO order_outbox (event_id, event_type, aggregate_id, payload, status, next_retry_at)
VALUES (?, ?, ?, ?, 0, NOW())`,
		eventID, eventType, orderID, payload,
	)
	return err
}
```

Keep `InsertOrderCreatedOutbox` as a wrapper.

- [ ] **Step 4: Run green test**

Run the same lifecycle test after the lifecycle implementation in Task 4.

---

### Task 3: Extend order-rpc Protobuf Contract

**Files:**
- Modify: `app/order/rpc/order.proto`
- Regenerate: generated RPC files.

- [ ] **Step 1: Add lifecycle messages and methods**

Add request/response messages for `PayOrder`, `ShipOrder`, `ConfirmReceipt`, `CancelOrder`, `RequestRefund`, and `ApproveRefund`. Use fields `order_id`, `operator_id`, `operator_role`, `reason`, and `request_id`.

- [ ] **Step 2: Regenerate RPC code**

Run:

```powershell
goctl rpc protoc app/order/rpc/order.proto --go_out=app/order/rpc --go-grpc_out=app/order/rpc --zrpc_out=app/order/rpc
```

Expected: generated client/server stubs include the new methods.

- [ ] **Step 3: Run compile check**

Run: `go test ./app/order/rpc/... -run TestDoesNotExist -count=0`

Expected: compile fails until lifecycle logic methods are added, then passes.

---

### Task 4: Implement order-rpc Lifecycle Transitions

**Files:**
- Create: `app/order/rpc/internal/logic/lifecyclelogic.go`
- Create: lifecycle RPC logic files.
- Modify: `app/order/rpc/internal/server/orderServer.go` only through generation plus small imports if generation requires it.

- [ ] **Step 1: Write failing tests for one transition at a time**

Start with `CancelOrder`: seed order status `0`, call logic, assert status `2`, status log row, and `order.cancelled` outbox row.

- [ ] **Step 2: Run red test**

Run: `go test ./app/order/rpc/internal/logic -run TestCancelOrder -count=1`

Expected: FAIL because `NewCancelOrderLogic` or method is undefined.

- [ ] **Step 3: Implement minimal shared transition helper**

Use `RawDB`, transaction, compare-and-set update, `order_status_log`, and `job.InsertOrderEventOutbox`.

- [ ] **Step 4: Implement remaining lifecycle methods**

Proceed in this order, each with red then green tests:

1. `ShipOrder`: `1 -> 3`, event `order.shipped`.
2. `ConfirmReceipt`: `3 -> 4`, event `order.completed`.
3. `RequestRefund`: `1 -> 5`, event `order.refund.requested`.
4. `ApproveRefund`: `5 or 7 -> 6` after idempotent stock rollback, event `order.refunded`; failure path writes `7` and `order.refund.failed`.
5. `PayOrder`: wrap or replace API direct DB pay path with RPC-owned transition.

- [ ] **Step 5: Run package tests**

Run: `go test ./app/order/rpc/internal/logic -count=1`

Expected: PASS.

---

### Task 5: Switch order-api Lifecycle Logic to RPC

**Files:**
- Modify: `app/order/api/internal/logic/payorderlogic.go`
- Modify: `app/order/api/internal/logic/shiporderlogic.go`
- Modify: `app/order/api/internal/logic/confirmreceiptlogic.go`
- Modify: `app/order/api/internal/logic/refundorderlogic.go`
- Create: `app/order/api/internal/logic/cancelorderlogic.go`

- [ ] **Step 1: Write failing API logic/handler tests**

Update handler tests or add logic tests that assert no local SQL path is needed and responses come from mocked/fake order-rpc behavior where existing test patterns allow it.

- [ ] **Step 2: Run red tests**

Run: `go test ./app/order/api/internal/handler -run "PayOrder|ShipOrder|ConfirmReceipt|Refund|Cancel" -count=1`

Expected: FAIL for missing cancel route or old semantics.

- [ ] **Step 3: Replace direct DB mutations**

Each logic reads identity, builds `orderclient.LifecycleReq`, calls order-rpc, and maps `StatusText` into existing response types.

- [ ] **Step 4: Run green tests**

Run: `go test ./app/order/api/internal/handler -run "PayOrder|ShipOrder|ConfirmReceipt|Refund|Cancel" -count=1`

Expected: PASS.

---

### Task 6: Update Routes, Types, API Spec, and SQL Baseline

**Files:**
- Modify: `app/order/api/internal/types/types.go`
- Modify: `app/order/api/internal/handler/routes.go`
- Modify: `app/order/api/internal/handler/adminorderhandler.go`
- Modify: `app/order/api/order.api`
- Modify: `app/order/api/desc/order.sql`

- [ ] **Step 1: Add cancel and approve-refund types**

Add request/response structs using existing JSON naming: `CancelOrderReq`, `CancelOrderResp`, `ApproveRefundReq`, `ApproveRefundResp`.

- [ ] **Step 2: Wire routes**

Add user route `/api/order/cancel` and admin route `/api/admin/orders/refund/approve`.

- [ ] **Step 3: Sync `order.api` and SQL**

Document the actual routes and current order-related tables. Keep SQL idempotency in `scripts/k8s/init-db.sql`; `desc/order.sql` is the schema baseline.

- [ ] **Step 4: Compile**

Run: `go test ./app/order/api/... -run TestDoesNotExist -count=0`

Expected: PASS compile.

---

### Task 7: Frontend Role Boundary

**Files:**
- Modify: `frontend/packages/shop/src/components/OrderCard.tsx`
- Modify: `frontend/packages/shop/src/pages/OrdersPage.tsx`
- Modify: `frontend/packages/admin/src/pages/OrdersPage.tsx`
- Modify shared types/constants if statuses need labels.

- [ ] **Step 1: Update shop actions**

Remove user-side ship action. Add cancel for status `0` and request refund for status `1`.

- [ ] **Step 2: Update admin actions**

Keep ship for paid orders. Use approve refund for status `5` and retry approve refund for status `7`.

- [ ] **Step 3: Build frontend**

Run:

```powershell
npm run build:shop --prefix frontend
npm run build:admin --prefix frontend
```

Expected: both exit 0.

---

### Task 8: Full Verification and Review

**Files:** all changed files.

- [ ] **Step 1: Run backend verification**

Run:

```powershell
go test ./app/order/rpc/internal/logic -count=1
go test ./app/order/api/internal/handler -count=1
go test ./app/order/api/... ./app/order/rpc/... ./app/product/rpc/... -count=1
```

Expected: all exit 0.

- [ ] **Step 2: Review diff with MCP**

Call `local_mcp_brain.review_diff` on the final diff and address valid findings.

- [ ] **Step 3: Commit implementation**

Commit in coherent slices if the diff is large: RPC state machine first, API switch second, frontend/contracts third.

---

## Self-Review

Spec coverage:

- Contract and SQL sync: Task 6.
- RPC state machine: Tasks 1, 3, 4.
- API delegates to RPC: Task 5.
- Outbox lifecycle events: Tasks 2 and 4.
- Cancel versus refund split: Tasks 4, 5, 7.
- Frontend role cleanup: Task 7.
- Verification and MCP review: Task 8.

Placeholder scan: no placeholder steps remain; every task includes concrete files and commands.

Type consistency: status codes match the spec and existing database values: 0 pending, 1 paid, 2 closed, 3 shipped, 4 completed, 5 refund requested, 6 refunded, 7 refund failed.
