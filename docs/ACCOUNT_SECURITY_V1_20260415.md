# Account Security V1 Demo Note

## Demo Flow

1. Log in through the storefront and place a normal order.
2. Trigger repeated bad password attempts until login throttling appears.
3. Trigger repeated code sends until the code send limiter appears.
4. Open `/shop` and inspect the security visibility area.
5. Request `GET /api/auth/security/events/recent` through the BFF and confirm the newest event appears first.

## Visible Anchors

- `/api/auth/security/events/recent`
- `security-events`
- `login-risk-summary`
- `code-risk-summary`

## Scope Note

This is a lightweight visibility surface for the storefront, not a full admin panel.
