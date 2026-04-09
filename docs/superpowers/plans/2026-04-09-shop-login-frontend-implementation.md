# Shop And Login Frontend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn `/shop` into a presentation-ready ecommerce homepage with an in-page login modal and a floating developer console while keeping existing backend flows intact.

**Architecture:** Keep the current server-rendered embedded HTML approach. Rebuild `shop.html` around a responsive storefront shell, add a Figma-inspired login modal that still posts to the existing auth API, and surface auth, health, quick actions, and logs in a floating developer console. Lightly refresh `home.html` so it matches the new product direction while keeping `/debug` as the full engineering page.

**Tech Stack:** Go embedded HTML, inline CSS, inline browser JavaScript, go test, Chrome DevTools MCP, Figma MCP

---

## File Map

- Modify: `app/order/api/internal/handler/web/shop.html`
  - Replace the current form-heavy storefront with the redesigned ecommerce homepage, login modal, and floating developer console.
- Modify: `app/order/api/internal/handler/web/home.html`
  - Refresh the landing page visual language so it aligns with the new storefront.
- Create: `app/order/api/internal/handler/webuihandler_test.go`
  - Add regression tests that assert embedded pages expose the new UI anchors.

### Task 1: Add Failing UI Regression Tests

**Files:**
- Create: `app/order/api/internal/handler/webuihandler_test.go`
- Test: `app/order/api/internal/handler/webuihandler_test.go`

- [ ] **Step 1: Write the failing test**

```go
package handler

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestShopUIIncludesStorefrontAnchors(t *testing.T) {
	req := httptest.NewRequest("GET", "/shop", nil)
	rec := httptest.NewRecorder()

	ShopUIHandler().ServeHTTP(rec, req)

	body := rec.Body.String()
	for _, needle := range []string{
		"Flash Mall",
		"developer-console",
		"login-modal",
		"featured-products",
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("expected shop UI to contain %q", needle)
		}
	}
}

func TestHomeUIIncludesNewEntryAnchors(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	HomeUIHandler().ServeHTTP(rec, req)

	body := rec.Body.String()
	for _, needle := range []string{
		"Flash Mall",
		"/shop",
		"/debug",
		"new storefront",
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("expected home UI to contain %q", needle)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./app/order/api/internal/handler -run "TestShopUIIncludesStorefrontAnchors|TestHomeUIIncludesNewEntryAnchors" -count=1`
Expected: FAIL because the current HTML does not yet contain the new structural anchors.

- [ ] **Step 3: Write minimal implementation**

Add the new anchor ids and text while rebuilding `shop.html` and `home.html` in later tasks.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./app/order/api/internal/handler -run "TestShopUIIncludesStorefrontAnchors|TestHomeUIIncludesNewEntryAnchors" -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add app/order/api/internal/handler/webuihandler_test.go app/order/api/internal/handler/web/shop.html app/order/api/internal/handler/web/home.html
git commit -m "test: cover storefront ui anchors"
```

### Task 2: Rebuild `/shop` As The Main Storefront

**Files:**
- Modify: `app/order/api/internal/handler/web/shop.html`
- Test: `app/order/api/internal/handler/webuihandler_test.go`

- [ ] **Step 1: Write the failing test**

Extend the `TestShopUIIncludesStorefrontAnchors` test to assert additional ids:

```go
for _, needle := range []string{
	"hero-section",
	"campaign-strip",
	"featured-products",
	"open-login",
} {
	if !strings.Contains(body, needle) {
		t.Fatalf("expected shop UI to contain %q", needle)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./app/order/api/internal/handler -run TestShopUIIncludesStorefrontAnchors -count=1`
Expected: FAIL with missing hero and campaign anchors.

- [ ] **Step 3: Write minimal implementation**

Rebuild `shop.html` with:

- a product-first top navigation
- a responsive hero section
- campaign and discovery content
- a featured product area that still supports quantity selection and order creation
- retained links to `/debug`

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./app/order/api/internal/handler -run TestShopUIIncludesStorefrontAnchors -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add app/order/api/internal/handler/web/shop.html app/order/api/internal/handler/webuihandler_test.go
git commit -m "feat: redesign shop storefront shell"
```

### Task 3: Add The Figma-Inspired Login Modal

**Files:**
- Modify: `app/order/api/internal/handler/web/shop.html`
- Test: `app/order/api/internal/handler/webuihandler_test.go`

- [ ] **Step 1: Write the failing test**

Extend `TestShopUIIncludesStorefrontAnchors` with:

```go
for _, needle := range []string{
	"login-modal",
	"login-user-id",
	"login-password",
	"login-submit",
} {
	if !strings.Contains(body, needle) {
		t.Fatalf("expected shop UI to contain %q", needle)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./app/order/api/internal/handler -run TestShopUIIncludesStorefrontAnchors -count=1`
Expected: FAIL because the modal ids are not present yet.

- [ ] **Step 3: Write minimal implementation**

Add an in-page modal to `shop.html` that:

- opens from a visible login CTA
- uses `user_id` and `password`
- posts to `/api/auth/login`
- updates the storefront auth state
- closes on success
- shows inline errors on failure

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./app/order/api/internal/handler -run TestShopUIIncludesStorefrontAnchors -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add app/order/api/internal/handler/web/shop.html app/order/api/internal/handler/webuihandler_test.go
git commit -m "feat: add storefront login modal"
```

### Task 4: Add The Floating Developer Console

**Files:**
- Modify: `app/order/api/internal/handler/web/shop.html`
- Test: `app/order/api/internal/handler/webuihandler_test.go`

- [ ] **Step 1: Write the failing test**

Extend `TestShopUIIncludesStorefrontAnchors` with:

```go
for _, needle := range []string{
	"developer-console",
	"console-auth",
	"console-health",
	"console-actions",
	"console-logs",
} {
	if !strings.Contains(body, needle) {
		t.Fatalf("expected shop UI to contain %q", needle)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./app/order/api/internal/handler -run TestShopUIIncludesStorefrontAnchors -count=1`
Expected: FAIL because the floating console structure is missing.

- [ ] **Step 3: Write minimal implementation**

Add a floating developer console to `shop.html` that:

- defaults open on desktop
- defaults collapsed on mobile
- shows auth summary
- runs health checks through `/api/system/health`
- exposes quick buy and related order actions
- streams recent request and response logs

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./app/order/api/internal/handler -run TestShopUIIncludesStorefrontAnchors -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add app/order/api/internal/handler/web/shop.html app/order/api/internal/handler/webuihandler_test.go
git commit -m "feat: add floating developer console"
```

### Task 5: Refresh `/` To Match The New Direction

**Files:**
- Modify: `app/order/api/internal/handler/web/home.html`
- Test: `app/order/api/internal/handler/webuihandler_test.go`

- [ ] **Step 1: Write the failing test**

Extend `TestHomeUIIncludesNewEntryAnchors` with:

```go
for _, needle := range []string{
	"new storefront",
	"entry-shop",
	"entry-debug",
} {
	if !strings.Contains(body, needle) {
		t.Fatalf("expected home UI to contain %q", needle)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./app/order/api/internal/handler -run TestHomeUIIncludesNewEntryAnchors -count=1`
Expected: FAIL because the refreshed landing structure is not present yet.

- [ ] **Step 3: Write minimal implementation**

Refresh `home.html` so it:

- matches the new color and typography direction
- clearly frames `/shop` as the consumer storefront
- clearly frames `/debug` as the engineering console
- remains lightweight and fast

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./app/order/api/internal/handler -run TestHomeUIIncludesNewEntryAnchors -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add app/order/api/internal/handler/web/home.html app/order/api/internal/handler/webuihandler_test.go
git commit -m "feat: refresh frontend entry page"
```

### Task 6: Verify End-To-End In Browser

**Files:**
- Modify: `app/order/api/internal/handler/web/shop.html` if browser validation reveals issues
- Modify: `app/order/api/internal/handler/web/home.html` if browser validation reveals issues
- Test: `app/order/api/internal/handler/webuihandler_test.go`

- [ ] **Step 1: Run handler tests**

Run: `go test ./app/order/api/internal/handler -count=1`
Expected: PASS

- [ ] **Step 2: Start the service stack or the local order-api flow**

Run the project’s existing local startup flow so `/` and `/shop` are reachable.

- [ ] **Step 3: Validate with Chrome DevTools MCP**

Verify:

- `/` renders the new entry page
- `/shop` renders the storefront shell
- login modal opens and logs in successfully
- quick buy and order flow still work
- developer console updates auth, health, actions, and logs
- desktop and mobile layouts both work

- [ ] **Step 4: Fix any browser-found issues and re-run tests**

Run: `go test ./app/order/api/internal/handler -count=1`
Expected: PASS after any final fixes

- [ ] **Step 5: Commit**

```bash
git add app/order/api/internal/handler/web/home.html app/order/api/internal/handler/web/shop.html app/order/api/internal/handler/webuihandler_test.go
git commit -m "feat: finalize storefront frontend redesign"
```
