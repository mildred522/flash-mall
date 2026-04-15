# Auth Persistence Execution (2026-04-13)

## Scope

This round completes the unfinished persistence work for the business auth system.

Implemented:

- `auth-service` now switches to a SQL-backed `AuthStore` when `DataSource` is configured.
- The SQL store covers verification codes, user registration, password login, refresh token rotation, logout, logout-all, password reset invalidation, and device-aware session replacement.
- Session snapshots and user session versions continue to sync to Redis when Redis is available.
- Redis state writes now fail fast with a bounded timeout, so local environments without Redis no longer block `auth-service` request threads for the full API timeout window.

## Runtime Switch

Files:

- `app/auth/api/internal/config/config.go`
- `app/auth/api/internal/svc/servicecontext.go`
- `app/auth/api/etc/auth-api.yaml`

Behavior:

- `DataSource` configured: use `authstore.SQLStore`
- `DataSource` empty: keep using the in-memory store

This preserves existing unit tests and demo behavior while allowing the real auth service to move onto MySQL.

## SQL Store Behavior

Files:

- `app/auth/api/internal/authstore/sqlstore.go`
- `app/auth/api/internal/authstore/sqlstore_user.go`
- `app/auth/api/internal/authstore/sqlstore_code.go`
- `app/auth/api/internal/authstore/sqlstore_session.go`

Business behavior kept consistent with the in-memory store:

- fixed debug verification code for local/demo flows
- register creates user, identity, and password credential records
- same `device_type` login invalidates prior active sessions on that device
- different device types can coexist
- refresh rotates the refresh token hash in storage
- logout revokes one session
- logout-all bumps `session_version` and revokes every active session
- password reset also bumps `session_version` and revokes every active session

The SQL store lazily seeds the demo account (`user_id=1001`, `phone=13800000001`) so the service-context type switch test does not require a live database connection during construction.

## Redis Degrade Strategy

Files:

- `app/auth/api/internal/sessionstate/state.go`
- `app/auth/api/internal/sessionstate/state_test.go`

Observed issue:

- Redis errors were logically ignored, but each write still retried long enough to consume the auth API's 2-second request timeout.

Fix:

- every Redis state write now runs inside a short bounded context timeout
- when Redis is absent, requests return quickly and auth continues using MySQL persistence

## Verification

Automated:

- `go test ./app/auth/api/...`
- `go test ./app/order/api/...`

Smoke test executed against local MySQL:

1. initialize schema from `scripts/k8s/init-db.sql`
2. start `auth-api`
3. call `/api/auth/code/send`
4. call `/api/auth/register`
5. call `/api/auth/login`
6. call `/api/auth/refresh`
7. call `/api/auth/me`

Observed result:

- full register/login/refresh/me path succeeded with a newly created user persisted in MySQL

## Remaining Environment Gap

The full `order-api` runtime smoke was not executed in this round because the current local environment still does not have Redis on `127.0.0.1:6379`, and the full mall stack was not started.

Code-wise, the persistence path required for the auth system is now complete.
