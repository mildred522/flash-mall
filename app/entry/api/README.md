# Go-zero Entry API comparison baseline

This service is intentionally retained as the pre-Hertz comparison baseline.
It is not the default external gateway on `codex/arch-hertz-kitex`.

Rules:

- New customer, merchant, and administrator features are implemented in
  `app/gateway/hertz`, not copied into this service.
- Changes here are limited to security fixes, build compatibility, and fixes
  required to keep an existing comparison flow runnable.
- Business contracts and persisted data remain compatible with the baseline;
  framework-specific Hertz or Kitex packages must not be imported here.
- The frontend is not owned by this service. Both gateways consume the shared
  build output in `artifacts/web`, generated only from `frontend/packages`.
- The default local launcher does not start this service. Use the explicit
  legacy comparison profile or legacy CI workflow when collecting evidence.

Git history preserves the original implementation. Do not add historical
plans or migration diaries to this directory.
