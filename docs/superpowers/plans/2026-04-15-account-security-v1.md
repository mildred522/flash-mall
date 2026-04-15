# Account Security V1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first business-grade account-security slice for `flash-mall` by extending `auth-service` with throttling, session hardening, audit visibility, and BFF exposure through `order-api`.

**Architecture:** Keep `auth-service` as the owner of auth, session, risk, and audit behavior. Keep `order-api` as the only browser-facing BFF and continue to proxy `/api/auth/*`. Implement the work in four focused slices: security foundation, anti-abuse rules, session hardening, and storefront visibility plus verification.

**Tech Stack:** Go, go-zero, MySQL, Redis, JWT, bcrypt, HttpOnly Cookie, server-rendered HTML

---

## File Structure

### Security Foundation

- Create: `app/auth/api/internal/risk/limiter.go`
- Create: `app/auth/api/internal/risk/limiter_test.go`
- Create: `app/auth/api/internal/audit/events.go`
- Create: `app/auth/api/internal/audit/events_test.go`
- Create: `app/auth/api/internal/handler/requestmeta.go`
- Create: `app/auth/api/internal/handler/requestmeta_test.go`
- Modify: `app/auth/api/internal/config/config.go`
- Modify: `app/auth/api/internal/svc/servicecontext.go`
- Modify: `app/auth/api/etc/auth-api.yaml`

### Anti-Abuse And Session Logic

- Modify: `app/auth/api/internal/authstore/contracts.go`
- Modify: `app/auth/api/internal/authstore/store.go`
- Modify: `app/auth/api/internal/authstore/store_test.go`
- Modify: `app/auth/api/internal/authstore/sqlstore_code.go`
- Modify: `app/auth/api/internal/authstore/sqlstore_session.go`
- Modify: `app/auth/api/internal/sessionstate/state.go`
- Modify: `app/auth/api/internal/sessionstate/state_test.go`
- Modify: `app/auth/api/internal/logic/auth/loginpasswordlogic.go`
- Modify: `app/auth/api/internal/logic/auth/loginpasswordlogic_test.go`
- Modify: `app/auth/api/internal/logic/auth/sendcodelogic.go`
- Modify: `app/auth/api/internal/logic/auth/sendcodelogic_test.go`
- Modify: `app/auth/api/internal/logic/auth/refreshlogic.go`
- Modify: `app/auth/api/internal/logic/auth/refreshlogic_test.go`
- Modify: `app/auth/api/internal/logic/auth/logoutlogic.go`
- Modify: `app/auth/api/internal/logic/auth/logoutlogic_test.go`
- Modify: `app/auth/api/internal/logic/auth/logoutalllogic.go`
- Modify: `app/auth/api/internal/logic/auth/resetpasswordlogic.go`
- Modify: `app/auth/api/internal/logic/auth/resetpasswordlogic_test.go`
- Modify: `app/auth/api/internal/types/types.go`
- Modify: `scripts/k8s/init-db.sql`

### BFF And UI

- Create: `app/auth/api/internal/handler/securityeventsrecenthandler.go`
- Create: `app/auth/api/internal/logic/auth/securityeventslogic.go`
- Create: `app/auth/api/internal/logic/auth/securityeventslogic_test.go`
- Modify: `app/auth/api/internal/handler/routes.go`
- Modify: `app/auth/api/internal/types/types.go`
- Modify: `app/order/api/internal/handler/routes.go`
- Modify: `app/order/api/internal/handler/authproxyhandler_test.go`
- Modify: `app/order/api/internal/handler/web/shop.html`
- Modify: `app/order/api/internal/handler/webuihandler_test.go`

### Docs

- Create: `docs/ACCOUNT_SECURITY_V1_20260415.md`

### Task 1: Build Security Foundation

**Files:**
- Create: `app/auth/api/internal/risk/limiter.go`
- Create: `app/auth/api/internal/risk/limiter_test.go`
- Create: `app/auth/api/internal/audit/events.go`
- Create: `app/auth/api/internal/audit/events_test.go`
- Create: `app/auth/api/internal/handler/requestmeta.go`
- Create: `app/auth/api/internal/handler/requestmeta_test.go`
- Modify: `app/auth/api/internal/config/config.go`
- Modify: `app/auth/api/internal/svc/servicecontext.go`
- Modify: `app/auth/api/etc/auth-api.yaml`

- [ ] **Step 1: Write failing tests for the limiter, recorder, and request metadata helper**

```go
func TestMemoryLimiter_BlocksAtThreshold(t *testing.T) {
	limiter := NewMemoryLimiter()
	ctx := context.Background()
	key := "auth:risk:login:phone:13800000001"
	for i := 0; i < 5; i++ {
		if err := limiter.Incr(ctx, key, time.Minute); err != nil {
			t.Fatalf("incr failed: %v", err)
		}
	}
	blocked, count, err := limiter.Blocked(ctx, key, 5)
	if err != nil || !blocked || count != 5 {
		t.Fatalf("expected blocked at count 5, got blocked=%v count=%d err=%v", blocked, count, err)
	}
}

func TestMemoryRecorder_ListRecentNewestFirst(t *testing.T) {
	recorder := NewMemoryRecorder(8)
	_ = recorder.Record(context.Background(), Event{EventType: "login_password_fail", Result: "fail"})
	_ = recorder.Record(context.Background(), Event{EventType: "login_password_success", Result: "success"})
	items, err := recorder.ListRecent(context.Background(), 2)
	if err != nil || len(items) != 2 || items[0].EventType != "login_password_success" {
		t.Fatalf("unexpected recent items: %#v err=%v", items, err)
	}
}

func TestClientIPPrefersForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.7, 10.0.0.1")
	if got := clientIP(req); got != "203.0.113.7" {
		t.Fatalf("unexpected ip: %s", got)
	}
}
```

- [ ] **Step 2: Run the focused tests and verify red**

Run: `go test ./app/auth/api/internal/risk ./app/auth/api/internal/audit ./app/auth/api/internal/handler -run "Limiter|Recorder|ClientIP" -count=1`

Expected: FAIL because the new packages and helper do not exist yet.

- [ ] **Step 3: Add minimal production types and service wiring**

```go
type Limiter interface {
	Incr(ctx context.Context, key string, ttl time.Duration) error
	Blocked(ctx context.Context, key string, max int64) (bool, int64, error)
	Reset(ctx context.Context, key string) error
}

type Recorder interface {
	Record(ctx context.Context, event Event) error
	ListRecent(ctx context.Context, limit int) ([]Event, error)
}
```

```go
type Event struct {
	EventType     string
	Result        string
	UserID        int64
	IdentityValue string
	IP            string
	UserAgent     string
	CreatedAt     time.Time
}
```

```go
type ServiceContext struct {
	Config        config.Config
	Store         authstore.AuthStore
	RiskLimiter   risk.Limiter
	AuditRecorder audit.Recorder
}
```

- [ ] **Step 4: Add config defaults and request metadata helper**

```go
type Config struct {
	rest.RestConf
	JwtAuthSecret               string
	JwtExpireSeconds            int64
	DemoPassword                string
	DataSource                  string
	RedisConf                   redis.RedisConf
	RefreshTokenTTLSeconds      int64
	CodeTTLSeconds              int64
	RefreshCookieName           string
	LoginFailWindowSeconds      int64
	LoginFailPhoneMaxAttempts   int64
	LoginFailIPMaxAttempts      int64
	CodeSendCooldownSeconds     int64
	CodeSendPhoneWindowSeconds  int64
	CodeSendPhoneMaxAttempts    int64
	CodeSendIPWindowSeconds     int64
	CodeSendIPMaxAttempts       int64
	VerificationCodeMaxAttempts int64
	SecurityAuditRecentLimit    int64
}
```

```go
func clientIP(r *http.Request) string {
	if xff := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0]); xff != "" {
		return xff
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}
```

- [ ] **Step 5: Run the focused tests and commit**

Run: `go test ./app/auth/api/internal/risk ./app/auth/api/internal/audit ./app/auth/api/internal/handler -run "Limiter|Recorder|ClientIP" -count=1`

Expected: PASS

Run:

```powershell
git add app/auth/api/internal/risk app/auth/api/internal/audit app/auth/api/internal/handler/requestmeta.go app/auth/api/internal/handler/requestmeta_test.go app/auth/api/internal/config/config.go app/auth/api/internal/svc/servicecontext.go app/auth/api/etc/auth-api.yaml
git commit -m "feat: add auth security foundation"
```

### Task 2: Add Login And Verification-Code Anti-Abuse

**Files:**
- Modify: `app/auth/api/internal/authstore/contracts.go`
- Modify: `app/auth/api/internal/authstore/store.go`
- Modify: `app/auth/api/internal/authstore/store_test.go`
- Modify: `app/auth/api/internal/authstore/sqlstore_code.go`
- Modify: `app/auth/api/internal/handler/loginhandler.go`
- Modify: `app/auth/api/internal/handler/sendcodehandler.go`
- Modify: `app/auth/api/internal/handler/resetpasswordhandler.go`
- Modify: `app/auth/api/internal/logic/auth/loginpasswordlogic.go`
- Modify: `app/auth/api/internal/logic/auth/loginpasswordlogic_test.go`
- Modify: `app/auth/api/internal/logic/auth/sendcodelogic.go`
- Modify: `app/auth/api/internal/logic/auth/sendcodelogic_test.go`
- Modify: `app/auth/api/internal/logic/auth/resetpasswordlogic.go`
- Modify: `app/auth/api/internal/logic/auth/resetpasswordlogic_test.go`
- Modify: `app/auth/api/internal/types/types.go`
- Modify: `scripts/k8s/init-db.sql`

- [ ] **Step 1: Write failing tests for login throttling, code throttling, and bounded code attempts**

```go
func TestLoginPasswordLogic_Login_BlocksAfterPhoneThreshold(t *testing.T) {
	svcCtx := newSecuritySvcCtx(t)
	l := NewLoginPasswordLogic(context.Background(), svcCtx)
	for i := 0; i < 5; i++ {
		_, err := l.Login(&types.LoginReq{Phone: "13800000001", Password: "wrong", ClientIP: "203.0.113.5"})
		if err == nil {
			t.Fatalf("expected failure on attempt %d", i+1)
		}
	}
	_, err := l.Login(&types.LoginReq{Phone: "13800000001", Password: "wrong", ClientIP: "203.0.113.5"})
	if err == nil || !strings.Contains(err.Error(), "too many login failures") {
		t.Fatalf("expected throttling error, got %v", err)
	}
}

func TestSendCodeLogic_Send_BlocksAfterPhoneWindow(t *testing.T) {
	svcCtx := newSecuritySvcCtx(t)
	l := NewSendCodeLogic(context.Background(), svcCtx)
	for i := 0; i < 3; i++ {
		if _, err := l.Send(&types.SendCodeReq{Phone: "13800138000", Scene: "register", ClientIP: "203.0.113.7"}); err != nil {
			t.Fatalf("unexpected send error: %v", err)
		}
	}
	if _, err := l.Send(&types.SendCodeReq{Phone: "13800138000", Scene: "register", ClientIP: "203.0.113.7"}); err == nil {
		t.Fatalf("expected phone throttle")
	}
}

func TestStore_ConsumeCode_FailsAfterAttemptLimit(t *testing.T) {
	store := authstore.NewStore("pwd")
	code, _, _ := store.IssueCode("13800138000", "login", 300)
	for i := 0; i < 5; i++ {
		_ = store.ConsumeCode("13800138000", "login", "wrong", 5)
	}
	if err := store.ConsumeCode("13800138000", "login", code, 5); err == nil {
		t.Fatalf("expected code to be invalid after max attempts")
	}
}
```

- [ ] **Step 2: Run the focused tests and verify red**

Run: `go test ./app/auth/api/internal/authstore ./app/auth/api/internal/logic/auth -run "LoginPassword|SendCode|ConsumeCode" -count=1`

Expected: FAIL because the current logic does not use counters and the store does not track attempt limits.

- [ ] **Step 3: Extend the store contract and persistence model**

```go
type AuthStore interface {
	IssueCode(phone, scene string, ttlSeconds int64) (string, time.Time, error)
	ConsumeCode(phone, scene, code string, maxAttempts int64) error
	ResetCode(phone, scene string) error
	Authenticate(userID int64, phone, password string) (*User, error)
}
```

```go
func newSecuritySvcCtx(t *testing.T) *svc.ServiceContext {
	t.Helper()
	return svc.NewServiceContext(config.Config{
		JwtAuthSecret:               "test-auth-jwt-secret",
		JwtExpireSeconds:            900,
		DemoPassword:                "pwd",
		RefreshTokenTTLSeconds:      3600,
		CodeTTLSeconds:              300,
		LoginFailWindowSeconds:      900,
		LoginFailPhoneMaxAttempts:   5,
		LoginFailIPMaxAttempts:      20,
		CodeSendCooldownSeconds:     60,
		CodeSendPhoneWindowSeconds:  600,
		CodeSendPhoneMaxAttempts:    3,
		CodeSendIPWindowSeconds:     600,
		CodeSendIPMaxAttempts:       10,
		VerificationCodeMaxAttempts: 5,
		SecurityAuditRecentLimit:    10,
	})
}
```

```go
type LoginReq struct {
	UserId     int64  `json:"user_id,optional"`
	Phone      string `json:"phone,optional"`
	Password   string `json:"password"`
	DeviceType string `json:"device_type,optional"`
	ClientIP   string `json:"-"`
	UserAgent  string `json:"-"`
}

type SendCodeReq struct {
	Phone     string `json:"phone"`
	Scene     string `json:"scene"`
	ClientIP  string `json:"-"`
	UserAgent string `json:"-"`
}

type ResetPasswordReq struct {
	Phone       string `json:"phone"`
	Code        string `json:"code"`
	NewPassword string `json:"new_password"`
	ClientIP    string `json:"-"`
	UserAgent   string `json:"-"`
}
```

```sql
ALTER TABLE verification_codes
  ADD COLUMN attempt_count int NOT NULL DEFAULT 0 COMMENT 'verification attempts';
```

- [ ] **Step 4: Implement phone/IP throttling and reset-on-success**

```go
phoneKey := "auth:risk:login:phone:" + req.Phone
ipKey := "auth:risk:login:ip:" + req.ClientIP
blocked, _, _ := l.svcCtx.RiskLimiter.Blocked(l.ctx, phoneKey, l.svcCtx.Config.LoginFailPhoneMaxAttempts)
if blocked {
	return nil, status.Error(codes.ResourceExhausted, "too many login failures")
}
```

```go
cooldownKey := fmt.Sprintf("auth:risk:code:phone:%s:%s:cooldown", req.Scene, req.Phone)
windowKey := fmt.Sprintf("auth:risk:code:phone:%s:%s:window", req.Scene, req.Phone)
ipKey := fmt.Sprintf("auth:risk:code:ip:%s:%s", req.Scene, req.ClientIP)
```

```go
req.ClientIP = clientIP(r)
req.UserAgent = r.UserAgent()
```

- [ ] **Step 5: Run focused tests and commit**

Run: `go test ./app/auth/api/internal/authstore ./app/auth/api/internal/logic/auth -run "LoginPassword|SendCode|ConsumeCode|ResetPassword" -count=1`

Expected: PASS

Run:

```powershell
git add app/auth/api/internal/authstore/contracts.go app/auth/api/internal/authstore/store.go app/auth/api/internal/authstore/store_test.go app/auth/api/internal/authstore/sqlstore_code.go app/auth/api/internal/handler/loginhandler.go app/auth/api/internal/handler/sendcodehandler.go app/auth/api/internal/handler/resetpasswordhandler.go app/auth/api/internal/logic/auth/loginpasswordlogic.go app/auth/api/internal/logic/auth/loginpasswordlogic_test.go app/auth/api/internal/logic/auth/sendcodelogic.go app/auth/api/internal/logic/auth/sendcodelogic_test.go app/auth/api/internal/logic/auth/resetpasswordlogic.go app/auth/api/internal/logic/auth/resetpasswordlogic_test.go scripts/k8s/init-db.sql
git commit -m "feat: add auth anti-abuse rules"
```

### Task 3: Harden Sessions With Rotation, Replay Kill-Switch, And Global Invalidation

**Files:**
- Modify: `app/auth/api/internal/authstore/contracts.go`
- Modify: `app/auth/api/internal/authstore/store.go`
- Modify: `app/auth/api/internal/authstore/store_test.go`
- Modify: `app/auth/api/internal/authstore/sqlstore_session.go`
- Modify: `app/auth/api/internal/sessionstate/state.go`
- Modify: `app/auth/api/internal/sessionstate/state_test.go`
- Modify: `app/auth/api/internal/logic/auth/refreshlogic.go`
- Modify: `app/auth/api/internal/logic/auth/refreshlogic_test.go`
- Modify: `app/auth/api/internal/logic/auth/logoutlogic.go`
- Modify: `app/auth/api/internal/logic/auth/logoutlogic_test.go`
- Modify: `app/auth/api/internal/logic/auth/logoutalllogic.go`
- Modify: `app/auth/api/internal/logic/auth/resetpasswordlogic.go`
- Modify: `scripts/k8s/init-db.sql`

- [ ] **Step 1: Write failing tests for rotation, replay detection, and reset-password invalidation**

```go
func TestRefreshLogic_Refresh_ReplayedOldTokenKillsSession(t *testing.T) {
	svcCtx := newSecuritySvcCtx(t)
	login := NewLoginPasswordLogic(context.Background(), svcCtx)
	first, _ := login.Login(&types.LoginReq{Phone: "13800000001", Password: "pwd", DeviceType: "web", ClientIP: "203.0.113.9"})
	refresh := NewRefreshLogic(context.Background(), svcCtx)
	second, err := refresh.Refresh(first.RefreshToken)
	if err != nil {
		t.Fatalf("refresh failed: %v", err)
	}
	if _, err := refresh.Refresh(first.RefreshToken); err == nil {
		t.Fatalf("expected replayed token to fail")
	}
	if _, err := refresh.Refresh(second.RefreshToken); err == nil {
		t.Fatalf("expected latest token to be revoked after replay kill-switch")
	}
}

func TestResetPasswordLogic_Reset_RevokesOldRefreshToken(t *testing.T) {
	svcCtx := newSecuritySvcCtx(t)
	login := NewLoginPasswordLogic(context.Background(), svcCtx)
	resp, _ := login.Login(&types.LoginReq{Phone: "13800000001", Password: "pwd", ClientIP: "203.0.113.10"})
	reset := NewResetPasswordLogic(context.Background(), svcCtx)
	_ = reset.Reset(&types.ResetPasswordReq{Phone: "13800000001", Code: "246810", NewPassword: "new-pass-456", ClientIP: "203.0.113.10"})
	refresh := NewRefreshLogic(context.Background(), svcCtx)
	if _, err := refresh.Refresh(resp.RefreshToken); err == nil {
		t.Fatalf("expected old refresh token to be dead after password reset")
	}
}
```

- [ ] **Step 2: Run the focused tests and verify red**

Run: `go test ./app/auth/api/internal/authstore ./app/auth/api/internal/logic/auth -run "Refresh|Logout|ResetPassword" -count=1`

Expected: FAIL because replay detection and full invalidation are incomplete.

- [ ] **Step 3: Extend session persistence for replay detection**

```go
var ErrRefreshTokenReplayed = errors.New("refresh token replayed")

type Session struct {
	ID                       string
	UserID                   int64
	DeviceType               string
	SessionVersion           int64
	RefreshTokenHash         string
	PreviousRefreshTokenHash string
	ExpiresAt                time.Time
	Revoked                  bool
}
```

```sql
ALTER TABLE user_sessions
  ADD COLUMN previous_refresh_token_hash char(64) NOT NULL DEFAULT '' COMMENT 'previous refresh hash';
```

- [ ] **Step 4: Implement rotation and invalidation semantics**

```go
session.PreviousRefreshTokenHash = session.RefreshTokenHash
session.RefreshTokenHash = hashToken(newRefreshToken)
```

```go
if session.PreviousRefreshTokenHash == hashToken(refreshToken) {
	session.Revoked = true
	s.deleteSessionState(session.ID)
	return nil, "", ErrRefreshTokenReplayed
}
```

```go
updatedUser, err := l.svcCtx.Store.UpdatePassword(req.Phone, req.NewPassword)
if err == nil {
	l.svcCtx.Store.LogoutAll(updatedUser.ID)
}
```

- [ ] **Step 5: Run focused tests and commit**

Run: `go test ./app/auth/api/internal/authstore ./app/auth/api/internal/sessionstate ./app/auth/api/internal/logic/auth -run "Refresh|Logout|ResetPassword" -count=1`

Expected: PASS

Run:

```powershell
git add app/auth/api/internal/authstore/contracts.go app/auth/api/internal/authstore/store.go app/auth/api/internal/authstore/store_test.go app/auth/api/internal/authstore/sqlstore_session.go app/auth/api/internal/sessionstate/state.go app/auth/api/internal/sessionstate/state_test.go app/auth/api/internal/logic/auth/refreshlogic.go app/auth/api/internal/logic/auth/refreshlogic_test.go app/auth/api/internal/logic/auth/logoutlogic.go app/auth/api/internal/logic/auth/logoutlogic_test.go app/auth/api/internal/logic/auth/logoutalllogic.go app/auth/api/internal/logic/auth/resetpasswordlogic.go scripts/k8s/init-db.sql
git commit -m "feat: harden auth sessions"
```

### Task 4: Expose Security Events Through BFF, Update Storefront, And Verify End To End

**Files:**
- Create: `app/auth/api/internal/handler/securityeventsrecenthandler.go`
- Create: `app/auth/api/internal/logic/auth/securityeventslogic.go`
- Create: `app/auth/api/internal/logic/auth/securityeventslogic_test.go`
- Modify: `app/auth/api/internal/handler/routes.go`
- Modify: `app/auth/api/internal/types/types.go`
- Modify: `app/order/api/internal/handler/routes.go`
- Modify: `app/order/api/internal/handler/authproxyhandler_test.go`
- Modify: `app/order/api/internal/handler/web/shop.html`
- Modify: `app/order/api/internal/handler/webuihandler_test.go`
- Create: `docs/ACCOUNT_SECURITY_V1_20260415.md`

- [ ] **Step 1: Write failing tests for recent event listing and storefront anchors**

```go
func TestSecurityEventsLogic_ListRecent(t *testing.T) {
	svcCtx := newSecuritySvcCtx(t)
	_ = svcCtx.AuditRecorder.Record(context.Background(), audit.Event{EventType: "login_password_fail", Result: "fail"})
	l := NewSecurityEventsLogic(context.Background(), svcCtx)
	resp, err := l.ListRecent(5)
	if err != nil || len(resp.Items) != 1 || resp.Items[0].EventType != "login_password_fail" {
		t.Fatalf("unexpected resp=%#v err=%v", resp, err)
	}
}
```

```go
for _, needle := range []string{
	"/api/auth/security/events/recent",
	"security-events",
	"login-risk-summary",
	"code-risk-summary",
} {
	if !strings.Contains(body, needle) {
		t.Fatalf("expected shop UI to contain %q", needle)
	}
}
```

- [ ] **Step 2: Run the focused tests and verify red**

Run: `go test ./app/auth/api/internal/logic/auth ./app/order/api/internal/handler -run "SecurityEvents|ShopUI" -count=1`

Expected: FAIL because the endpoint and UI hooks do not exist yet.

- [ ] **Step 3: Add the auth endpoint, BFF proxy route, and storefront console section**

```go
type SecurityEventItem struct {
	EventType string `json:"event_type"`
	Result    string `json:"result"`
	UserId    int64  `json:"user_id"`
	Subject   string `json:"subject"`
	IP        string `json:"ip"`
	CreatedAt int64  `json:"created_at"`
}
```

```go
{
	Method:  http.MethodGet,
	Path:    "/api/auth/security/events/recent",
	Handler: SecurityEventsRecentHandler(serverCtx),
}
```

```html
<section class="console-summary">
  <div>Login risk: <strong id="login-risk-summary">normal</strong></div>
  <div>Code risk: <strong id="code-risk-summary">normal</strong></div>
</section>
<section class="console-card">
  <h4>Recent Security Events</h4>
  <pre id="security-events" class="console-log">No events yet</pre>
</section>
```

- [ ] **Step 4: Add execution notes and run package verification**

Run: `go test ./app/auth/api/... -count=1`

Expected: PASS

Run: `go test ./app/order/api/... -count=1`

Expected: PASS

```md
## Demo Flow
1. Fail password login until blocked
2. Trigger send-code throttling
3. Refresh once, then replay the old refresh token
4. Reset password and confirm the old session is dead
5. Inspect recent security events in /shop
```

- [ ] **Step 5: Run local E2E verification and commit**

Run: `powershell -ExecutionPolicy Bypass -File scripts/local/start-all.ps1`

Expected: auth-api on `8890` and order-api on `8888` start cleanly.

Run: `powershell -ExecutionPolicy Bypass -File scripts/local/stop-all.ps1 -WithDeps`

Expected: no leftover listeners remain on the validation ports.

Run:

```powershell
git add app/auth/api/internal/handler/securityeventsrecenthandler.go app/auth/api/internal/logic/auth/securityeventslogic.go app/auth/api/internal/logic/auth/securityeventslogic_test.go app/auth/api/internal/handler/routes.go app/auth/api/internal/types/types.go app/order/api/internal/handler/routes.go app/order/api/internal/handler/authproxyhandler_test.go app/order/api/internal/handler/web/shop.html app/order/api/internal/handler/webuihandler_test.go docs/ACCOUNT_SECURITY_V1_20260415.md
git commit -m "feat: expose account security visibility"
```

## Acceptance Checklist

- `auth-service` owns login, code, refresh, logout, logout-all, reset-password, risk, and audit behavior
- login failures are throttled by phone and IP
- send-code is throttled by phone and IP
- verification codes are scene-bound, time-bound, single-use, and attempt-limited
- refresh rotates the token each time and replaying an old token kills the session
- logout-all and reset-password invalidate prior sessions immediately
- `/api/auth/security/events/recent` is available through `order-api`
- `/shop` shows recent security state without turning into an admin console
- `go test ./app/auth/api/... ./app/order/api/... -count=1` passes
