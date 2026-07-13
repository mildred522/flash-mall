# Demo Images, Admin Login, and Sandbox Payment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restore deterministic product images, add one-click demo administrator login, and replace direct mock payment with a QR sandbox flow that proves end-to-end idempotency.

**Architecture:** Hertz remains the external gateway, Go-zero order-rpc owns payment/order state, and Kitex owns inventory commands. Hertz issues signed payment capability tokens and sends sandbox confirmation through the same `MarkOrderPaid` path as provider callbacks; React renders QR codes and polls status.

**Tech Stack:** Go 1.24, Hertz, Go-zero zRPC, Kitex, MySQL 5.7, React 19, Vite 6, Vitest, Testing Library, `@rc-component/qrcode`, Docker Compose.

---

### Task 1: Frontend test foundation

**Files:** Modify `frontend/package.json`, both workspace `package.json` and `vite.config.ts`, `frontend/package-lock.json`; create `frontend/vitest.setup.ts` and one smoke test per UI workspace.

- [ ] Write shop/admin jsdom smoke tests importing `describe`, `expect`, and `it` from Vitest and asserting `document.createElement('div')` is an `HTMLDivElement`.
- [ ] Run `cd frontend && npm test`; expect failure because the script and dependencies do not exist.
- [ ] Add root dev dependencies `vitest`, `jsdom`, `@testing-library/react`, `@testing-library/jest-dom`, `@testing-library/user-event`; add root/workspace test scripts and `test: { environment: 'jsdom', setupFiles: '../../vitest.setup.ts' }` to both Vite configs.
- [ ] Put `import '@testing-library/jest-dom/vitest';` in `vitest.setup.ts`.
- [ ] Run `cd frontend && npm test -- --run`; expect both smoke tests to pass.
- [ ] Commit only these files as `test: add frontend component test foundation`.

### Task 2: Deterministic product images

**Files:** Create five SVGs in `frontend/packages/shop/public/products`; create `frontend/packages/shared/src/product-image.ts` and its test; modify shared constants/types/exports, `ProductCard.tsx`, `OrderCard.tsx`, shop CSS, `frontend/build.js`, Hertz `static_page.go`, routes and tests, and `scripts/k8s/init-db.sql`.

- [ ] Write failing tests proving an explicit upload URL wins, product 100 falls back to `/products/100.svg`, unknown products return an empty URL, `ProductCard` renders an image, and image error exposes emoji.
- [ ] Run `cd frontend && npm run test -w packages/shop -- --run`; expect missing resolver/image behavior failures.
- [ ] Implement:

```ts
export function resolveProductImage(productId: number, imageURL?: string): string {
  const explicit = imageURL?.trim();
  return explicit || PRODUCT_META[productId]?.image || '';
}
```

- [ ] Add `image_url` to the frontend product type, set `PRODUCT_META.image` for 100–104, render images in product/order cards, and switch to emoji on `onError`.
- [ ] Create five accessible product-specific SVG illustrations. Update `frontend/build.js` to recursively copy `dist/products` into the Hertz Web directory.
- [ ] Write a failing Go test for safe asset path resolution; implement `StaticAssetHandler("products")`, reject traversal, and register `GET /products/*any`.
- [ ] Add `/products/<id>.svg` to demo product INSERT and duplicate-key UPDATE statements so repeated initialization repairs empty image URLs.
- [ ] Run shop tests, `go test ./app/gateway/hertz/internal/handler`, and `npm run build`; assert five SVGs exist in the generated Web directory.
- [ ] Commit scoped files as `fix: restore durable product images`.

### Task 3: One-click demo administrator login

**Files:** Create `frontend/packages/admin/src/pages/admin-login.ts` and test; modify `LoginPage.tsx` and add its component test.

- [ ] Write failing tests that a login helper calls `/api/auth/login` with POST credentials and persists both tokens, and that clicking “演示管理员一键登录” submits `13800000002` / `flashmall123` and invokes `onLogin`.
- [ ] Run `cd frontend && npm run test -w packages/admin -- --run`; expect missing service/button failures.
- [ ] Export immutable `DEMO_ADMIN_CREDENTIALS`, share one login-and-token-persistence helper between form and shortcut, keep independent loading text, and show a specific shortcut failure message.
- [ ] Run admin tests and `npm run build:admin`; expect pass.
- [ ] Commit scoped sources and regenerated admin HTML as `feat: add demo admin quick login`.

### Task 4: Hertz sandbox payment backend

**Files:** Create `payment_token.go/_test.go` and `sandbox_payment.go/_test.go`; modify payment callback, order handler, types, routes/tests, auth middleware/tests, gateway config, deploy config, and K8s ConfigMap.

- [ ] Write failing token tests for HMAC round trip, one-byte tampering, expired confirmation, and correctly signed expired status decoding.
- [ ] Run `go test ./app/gateway/hertz/internal/handler -run TestPaymentToken -count=1`; expect missing token symbols.
- [ ] Implement `base64.RawURLEncoding(JSON claims) + "." + hex(HMAC-SHA256)` for order ID, payment ID, trade number, amount, and expiry; constant-time compare signatures and distinguish invalid from expired.
- [ ] Write failing MySQL-backed tests proving same user/order returns stable identifiers, another user cannot access them, closed orders are not payable, and token fields must exactly match the payment row.
- [ ] Add failing route tests for `/pay`, `/api/payment/status`, and `/api/payment/sandbox/confirm`; add middleware tests showing absent Authorization continues anonymously, valid JWT injects identity, and invalid supplied JWT returns 401.
- [ ] Change `PayOrderHandler` to return payment identifiers, amount, pending/paid state, absolute QR URL and fifteen-minute expiry without calling `MarkOrderPaid`.
- [ ] Add `OptionalIdentity`. Status accepts exactly one of authenticated `payment_order_id` or signed `token`, enforces owner/binding, and returns pending, paid, or derived expired state.
- [ ] Extract a common `markPaymentPaid` helper from provider callback. Sandbox confirmation validates token and database binding, then uses provider `sandbox` and stable event ID `sandbox:<payment_order_id>` through the same helper.
- [ ] Add `SandboxPaymentTokenTTLSeconds: 900` to Hertz configuration files.
- [ ] Run `gofmt` on modified Go files, focused gateway/middleware tests, and `go test ./app/order/rpc/internal/logic -run TestMarkOrderPaidLogic -count=1`; expect pass.
- [ ] Commit scoped files as `feat: add idempotent sandbox payment flow`.

### Task 5: QR payer experience

**Files:** Add direct shop dependency `@rc-component/qrcode`; modify lockfile/shared types; create `PaymentModal.tsx` and `PaymentPage.tsx` with tests; modify OrdersPage, App, shop CSS and Hertz `/pay` route.

- [ ] Write failing tests that the modal renders exact QR URL, amount, expiry and local-open link; payer page reads `token`, shows status, posts confirmation, and retains a second-confirm action after paid.
- [ ] Run `cd frontend && npm run test -w packages/shop -- --run`; expect missing component failures.
- [ ] Make `OrdersPage.handlePay` consume `PaymentIntentResp`, open the modal, and poll authenticated `/api/payment/status?payment_order_id=...`; only reload as paid after the backend says paid.
- [ ] Render public `PaymentPage` when `window.location.pathname === '/pay'`; query status by token, confirm it, and expose “再次提交确认（验证幂等）” after success.
- [ ] Register `/pay` to serve the shop HTML. Run shop tests, full frontend build and gateway route tests; expect pass.
- [ ] Commit scoped files and regenerated shop HTML as `feat: add QR sandbox payment experience`.

### Task 6: Real chain verification and documentation

**Files:** Modify `docs/CURRENT_PROJECT.md`.

- [ ] Run `go test ./...`, `cd frontend && npm test -- --run && npm run build`, and scoped `git diff --check` for Task 1–5 sources.
- [ ] Reapply the idempotent MySQL initialization, rebuild only `hertz-gateway`, and start the default Compose topology without pruning named volumes.
- [ ] Verify `/products/100.svg` is 200, catalog returns five non-empty image URLs, cards render images, and one-click admin login reaches the dashboard with an admin JWT.
- [ ] Log in as user `13800000001`, create an order using a unique request ID, open its QR URL in a separate browser context, and submit confirmation three times.
- [ ] Query MySQL and assert one paid order, one successful payment order, one unique sandbox callback row, one logical `order.paid` outbox event, and one inventory deduction.
- [ ] Tamper one token character and assert status/confirmation rejects it while order and stock stay unchanged.
- [ ] Record endpoints, demo flow, idempotency evidence and remaining real-provider adapter scope in `docs/CURRENT_PROJECT.md`; commit as `docs: record sandbox payment verification`.
