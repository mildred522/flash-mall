# Hertz Payment Safety Port Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将支付审计、三类幂等对账和退款领域写入按当前 Hertz + Go-zero RPC + Kitex 架构落到 Hertz 主线。

**Architecture:** Hertz 只保留 HTTP 和鉴权；`order-rpc` 持有支付/退款事务；库存释放继续调用 Kitex。对账复用现有后台与 `reconciliation_issue`，不引入旧表和旧 API。

**Tech Stack:** Go 1.24、CloudWeGo Hertz、Go-zero zRPC、Kitex、MySQL、Outbox/RabbitMQ、Protocol Buffers。

---

### Task 1: 支付状态日志

**Files:**
- Modify: `app/order/rpc/internal/logic/markorderpaidlogic_test.go`
- Modify: `app/order/rpc/internal/logic/markorderpaidlogic.go`

- [x] **Step 1: Write the failing test**

在现有幂等测试中创建 `order_status_log`，首次和重复支付后都断言 `0 -> 1` 日志数量为 `1`。

- [x] **Step 2: Run test to verify it fails**

Run: `go test ./app/order/rpc/internal/logic -run TestMarkOrderPaidLogic_MarkPaid_IsIdempotent -count=1`
Expected: FAIL，日志数量为 0。

- [x] **Step 3: Write minimal implementation**

在支付事务更新成功后执行：

```go
_, err = tx.ExecContext(l.ctx,
    "INSERT INTO order_status_log (order_id, from_status, to_status, operator_id, remark) VALUES (?, ?, ?, 0, 'payment callback success')",
    in.OrderId, orderstatus.PendingPayment, orderstatus.Paid,
)
```

- [x] **Step 4: Run test to verify it passes**

Run: `go test ./app/order/rpc/internal/logic -run TestMarkOrderPaidLogic_MarkPaid_IsIdempotent -count=1`
Expected: PASS。

### Task 2: 对账扩展与幂等键

**Files:**
- Create: `app/gateway/hertz/internal/handler/admin_reconciliation_test.go`
- Modify: `app/gateway/hertz/internal/handler/admin_ops.go`
- Modify: `scripts/k8s/init-db.sql`

- [x] **Step 1: Write failing integration tests**

构造金额不一致、支付成功/订单待支付、活跃订单/支付未成功三种数据；连续扫描两次并断言每种 `issue_key` 只有一条开放记录。

- [x] **Step 2: Run tests to verify they fail**

Run: `go test ./app/gateway/hertz/internal/handler -run TestScanGatewayReconciliationIssues -count=1`
Expected: FAIL，当前只生成金额不一致问题。

- [x] **Step 3: Implement schema and scan**

为 `reconciliation_issue` 增加 `issue_key varchar(192)` 唯一索引；将三条 `INSERT ... SELECT` 使用稳定 key 和 `ON DUPLICATE KEY UPDATE status=0, detail=VALUES(detail)` 执行并汇总影响行数。

- [x] **Step 4: Run focused tests**

Run: `go test ./app/gateway/hertz/internal/handler -run TestScanGatewayReconciliationIssues -count=1`
Expected: PASS。

### Task 3: 退款领域 RPC

**Files:**
- Modify: `app/order/rpc/order.proto`
- Regenerate: `app/order/rpc/order/order.pb.go`
- Regenerate: `app/order/rpc/order/order_grpc.pb.go`
- Regenerate: `app/order/rpc/orderclient/order.go`
- Regenerate: `app/order/rpc/internal/server/orderServer.go`
- Create: `app/order/rpc/internal/logic/refundstate.go`
- Create: `app/order/rpc/internal/logic/requestrefundlogic.go`
- Create: `app/order/rpc/internal/logic/auditrefundlogic.go`
- Create: `app/order/rpc/internal/logic/refundlogic_test.go`

- [x] **Step 1: Define failing refund scenarios**

测试用户归属、重复申请返回同一退款单、批准成功调用一次 Kitex、重复批准安全、驳回恢复原状态、库存释放失败记录可重试状态。

- [x] **Step 2: Run tests to verify RED**

Run: `go test ./app/order/rpc/internal/logic -run 'Test(Request|Audit)Refund' -count=1`
Expected: FAIL，因为退款 RPC 逻辑尚不存在。

- [x] **Step 3: Add precise RPC contract**

新增 `RequestRefundReq/Resp` 和 `AuditRefundReq/Resp`，显式传递订单/退款 ID、操作人、角色、决策、原因和请求 ID；运行：

```bash
protoc --go_out=. --go_opt=module=flash-mall \
  --go-grpc_out=. --go-grpc_opt=module=flash-mall \
  app/order/rpc/order.proto
```

- [x] **Step 4: Implement transactional state machine**

`RequestRefund` 使用订单行锁和稳定 `rf:<order_id>`；`AuditRefund` 使用退款行锁，批准调用 `InventoryClient.ReleaseStock`，失败写 `refund_order.status=4`，成功/驳回写状态日志和稳定 Outbox。

- [x] **Step 5: Run focused tests**

Run: `go test ./app/order/rpc/internal/logic -run 'Test(Request|Audit)Refund' -count=1`
Expected: PASS。

### Task 4: Hertz 入口收口

**Files:**
- Modify: `app/gateway/hertz/internal/handler/order.go`
- Modify: `app/gateway/hertz/internal/handler/admin_refund.go`
- Modify: `app/gateway/hertz/internal/handler/admin_order.go`
- Create: `app/gateway/hertz/internal/handler/refund_rpc_test.go`

- [x] **Step 1: Write failing delegation tests**

使用订单 RPC stub 断言用户退款、管理员审核和管理员一键退款调用新 RPC，且 Handler 不直接写退款状态。

- [x] **Step 2: Run RED**

Run: `go test ./app/gateway/hertz/internal/handler -run 'Test.*Refund.*RPC' -count=1`
Expected: FAIL，当前 Handler 直接访问数据库。

- [x] **Step 3: Replace direct writes**

用户入口调用 `RequestRefund`；审核入口调用 `AuditRefund`；一键退款先 `RequestRefund(role=admin)` 再 `AuditRefund(approve=true)`。

- [x] **Step 4: Run GREEN**

Run: `go test ./app/gateway/hertz/internal/handler -run 'Test.*Refund.*RPC' -count=1`
Expected: PASS。

### Task 5: 完整验证与集成

- [x] 运行 `gofmt`、`git diff --check`。
- [x] 运行目标包测试和 `go test ./... -count=1`。
- [x] 运行 `go vet ./...`。
- [x] 构建 Hertz、Kitex、auth、order、product、entry 六个二进制。
- [ ] 提交并推送 `codex/payment-safety-hertz-port`。
- [ ] 创建中文 PR，目标为 `codex/arch-hertz-kitex`，等待 CI 成功后合并。
- [ ] 给旧提交 `e54cc1b` 创建归档标签，删除旧支付分支和已合并短期分支。
