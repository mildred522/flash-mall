# Auth Service Business Auth Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build an independent `auth-service` for business-grade registration and login, then integrate it behind `order-api` as the unified BFF entry while preserving the existing storefront and order flow.

**Architecture:** Introduce a standalone `auth-service` that owns account, identity, credential, session, verification-code, and auth-audit responsibilities. Keep `order-api` as the browser-facing entry and forward `/api/auth/*` requests to `auth-service`, while business endpoints continue local JWT verification plus strong session-state checks via Redis.

**Tech Stack:** Go, go-zero, MySQL, Redis, JWT, bcrypt, HttpOnly Cookie, server-rendered storefront

---

## File Structure

### New Service

- Create: `app/auth/api/auth.go`
- Create: `app/auth/api/auth.api`
- Create: `app/auth/api/etc/auth-api.yaml`
- Create: `app/auth/api/desc/auth.sql`
- Create: `app/auth/api/internal/config/config.go`
- Create: `app/auth/api/internal/svc/servicecontext.go`
- Create: `app/auth/api/internal/types/types.go`
- Create: `app/auth/api/internal/handler/routes.go`
- Create: `app/auth/api/internal/handler/*`
- Create: `app/auth/api/internal/logic/auth/*`
- Create: `app/auth/api/internal/model/*`
- Create: `app/auth/api/internal/middleware/*`
- Create: `app/auth/api/internal/provider/sms/*`
- Create: `app/auth/api/internal/token/*`
- Create: `app/auth/api/internal/session/*`

### BFF Integration

- Modify: `app/order/api/internal/handler/routes.go`
- Modify: `app/order/api/internal/config/config.go`
- Modify: `app/order/api/etc/order-api.yaml`
- Create: `app/order/api/internal/handler/authproxyhandler.go`
- Create: `app/order/api/internal/logic/sessionstate/*` if needed for strong-consistency checks

### Shared Runtime And Startup

- Modify: `scripts/local/start-all.ps1`
- Modify: `scripts/local/stop-all.ps1`
- Modify: `deploy/docker-compose.yml` only if runtime dependency changes are required

### Frontend

- Modify: `app/order/api/internal/handler/web/shop.html`
- Modify: `app/order/api/internal/handler/web/home.html`
- Modify: `app/order/api/internal/handler/webuihandler_test.go`

### Docs And Verification

- Modify: `docs/JWT_EXECUTION_20260307.md`
- Create: `docs/AUTH_SERVICE_EXECUTION_20260410.md`

## Execution Order

### Task 1: Scaffold Auth Service Skeleton

**Files:**
- Create: `app/auth/api/auth.go`
- Create: `app/auth/api/auth.api`
- Create: `app/auth/api/etc/auth-api.yaml`
- Create: `app/auth/api/internal/config/config.go`
- Create: `app/auth/api/internal/svc/servicecontext.go`
- Create: `app/auth/api/internal/types/types.go`
- Create: `app/auth/api/internal/handler/routes.go`

- [ ] Add service directory structure following `order-api` conventions.
- [ ] Define auth-facing routes for register, password login, code login, refresh, logout, logout-all, me, send-code, forgot-password, reset-password.
- [ ] Add service config for MySQL, Redis, JWT secret, token TTLs, cookie config, sms provider mode, and session policy.
- [ ] Add minimal service bootstrap so `go run ./app/auth/api/auth.go -f ./app/auth/api/etc/auth-api.yaml` can start cleanly.
- [ ] Run a focused build command for the new package.

### Task 2: Add Auth Schema And Models

**Files:**
- Create: `app/auth/api/desc/auth.sql`
- Create: `app/auth/api/internal/model/usersmodel.go`
- Create: `app/auth/api/internal/model/useridentitiesmodel.go`
- Create: `app/auth/api/internal/model/usercredentialsmodel.go`
- Create: `app/auth/api/internal/model/usersessionsmodel.go`
- Create: `app/auth/api/internal/model/verifycodesmodel.go`
- Create: `app/auth/api/internal/model/authauditlogsmodel.go`

- [ ] Add the auth schema for users, identities, credentials, sessions, verification codes, and audit logs.
- [ ] Implement focused model files instead of a single oversized auth repository.
- [ ] Keep model responsibilities split by aggregate so account/session/code logic stay readable.
- [ ] Add model-level tests for uniqueness, session lookup, and code lifecycle where practical.
- [ ] Verify auth schema can be initialized in local MySQL.

### Task 3: Implement Token And Session Core

**Files:**
- Create: `app/auth/api/internal/token/jwt.go`
- Create: `app/auth/api/internal/session/sessionmanager.go`
- Create: `app/auth/api/internal/session/sessionstate.go`

- [ ] Implement JWT access-token issuance using `sub`, `sid`, `session_version`, `iat`, and `exp`.
- [ ] Implement refresh-token generation, hashing, persistence, rotation, and revocation.
- [ ] Encode session policy: same-device-type mutual exclusion, cross-device coexistence.
- [ ] Write tests for session creation, same-device takeover, refresh rotation, and logout invalidation.
- [ ] Verify Redis-backed strong session state can be read independently from JWT signature verification.

### Task 4: Implement Verification Code Provider Layer

**Files:**
- Create: `app/auth/api/internal/provider/sms/provider.go`
- Create: `app/auth/api/internal/provider/sms/mock.go`
- Create: `app/auth/api/internal/provider/sms/real.go`
- Create: `app/auth/api/internal/logic/auth/sendcodelogic.go`

- [ ] Add a provider abstraction so dev can use mock codes and production can switch to real SMS.
- [ ] Add rate limiting and single-scene code validation rules.
- [ ] Store only verification-code hashes and expiry metadata.
- [ ] Add tests for send frequency, wrong code rejection, expired code rejection, and single-use semantics.
- [ ] Verify the dev mode can issue a predictable code without real SMS infrastructure.

### Task 5: Implement Registration And Password Reset

**Files:**
- Create: `app/auth/api/internal/logic/auth/registerlogic.go`
- Create: `app/auth/api/internal/logic/auth/forgotpasswordlogic.go`
- Create: `app/auth/api/internal/logic/auth/resetpasswordlogic.go`
- Create: matching handlers

- [ ] Implement register flow: verify phone code first, then create user, identity, credential, session, and audit log.
- [ ] Implement forgot-password flow: send reset code through the same provider abstraction.
- [ ] Implement reset-password flow: verify code, update credential hash, invalidate old sessions, and audit.
- [ ] Add tests for duplicate identity rejection, registration success, reset success, and reset-triggered session invalidation.
- [ ] Verify reset-password does not leave old refresh tokens usable.

### Task 6: Implement Login, Refresh, Logout, And Me

**Files:**
- Create: `app/auth/api/internal/logic/auth/loginpasswordlogic.go`
- Create: `app/auth/api/internal/logic/auth/logincodelogic.go`
- Create: `app/auth/api/internal/logic/auth/refreshlogic.go`
- Create: `app/auth/api/internal/logic/auth/logoutlogic.go`
- Create: `app/auth/api/internal/logic/auth/logoutalllogic.go`
- Create: `app/auth/api/internal/logic/auth/melogic.go`
- Create: matching handlers

- [ ] Implement password login with bcrypt verification and audit logging.
- [ ] Implement code login using verified phone codes.
- [ ] Implement refresh using `HttpOnly Cookie` semantics and refresh-token rotation.
- [ ] Implement logout and logout-all by invalidating session state in both MySQL and Redis.
- [ ] Implement `me` for storefront identity display and session-aware auth UI.

### Task 7: Integrate Auth Service Behind Order API BFF

**Files:**
- Modify: `app/order/api/internal/handler/routes.go`
- Create: `app/order/api/internal/handler/authproxyhandler.go`
- Modify: `app/order/api/internal/config/config.go`
- Modify: `app/order/api/etc/order-api.yaml`

- [ ] Add `/api/auth/*` proxy routes in `order-api` that forward requests to `auth-service`.
- [ ] Preserve a single browser-facing origin so the storefront does not need to know service topology.
- [ ] Ensure `HttpOnly Cookie` handling survives through the BFF path.
- [ ] Remove or deprecate the old demo login path once the proxy path is verified.
- [ ] Verify storefront auth calls no longer depend on in-process demo login logic.

### Task 8: Upgrade JWT Consumption In Business Services

**Files:**
- Modify: `app/order/api/internal/handler/createorderhandler.go`
- Modify: `app/order/api/internal/logic/createorderlogic.go`
- Add: `app/order/api/internal/logic/sessionstate/*` or equivalent

- [ ] Update claim parsing from demo `user_id` semantics to `sub + sid + session_version`.
- [ ] Keep request body user identity untrusted and override from verified auth context.
- [ ] Add strong-consistency session validation using Redis-backed session-state checks.
- [ ] Add tests for revoked sessions, same-device takeover invalidation, and valid-session order submission.
- [ ] Verify order flow still works when auth is externalized.

### Task 9: Upgrade Storefront Auth UX

**Files:**
- Modify: `app/order/api/internal/handler/web/shop.html`
- Modify: `app/order/api/internal/handler/web/home.html`
- Modify: `app/order/api/internal/handler/webuihandler_test.go`

- [ ] Replace the single demo login modal with login/register/recover flows wired to `/api/auth/*`.
- [ ] Keep consumer-facing copy in the storefront and restrict technical state to the floating developer console.
- [ ] Display current user info via `/api/auth/me`.
- [ ] Add refresh-aware client behavior so expired access tokens can recover quietly where appropriate.
- [ ] Extend UI tests to assert new auth sections exist without leaking developer wording into storefront body copy.

### Task 10: Local Runtime, Migration, And Verification

**Files:**
- Modify: `scripts/local/start-all.ps1`
- Modify: `scripts/local/stop-all.ps1`
- Create: `docs/AUTH_SERVICE_EXECUTION_20260410.md`
- Modify: `docs/JWT_EXECUTION_20260307.md`

- [ ] Add `auth-service` to local startup and shutdown scripts.
- [ ] Add auth-schema initialization to the local bootstrap path.
- [ ] Document local env behavior for mock SMS mode and `HttpOnly Cookie` testing.
- [ ] Verify the full chain: register -> login -> me -> order -> refresh -> logout -> invalidated order attempt.
- [ ] Run targeted tests and record exact verification commands in docs.

## Acceptance Checklist

- Independent `auth-service` exists and can run locally
- Storefront still uses a single browser-facing origin
- Registration requires SMS verification first
- Password login and SMS code login both work
- Refresh uses `HttpOnly Cookie`
- Same-device login invalidates the old same-device session
- Different device types can stay online concurrently
- Business services locally verify JWT and session state
- Old demo login flow is removed or fully retired from runtime path
- Storefront copy remains consumer-facing, developer state stays in the floating console
