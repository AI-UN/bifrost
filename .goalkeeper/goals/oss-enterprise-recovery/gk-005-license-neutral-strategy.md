# GK-005 License-Neutral Strategy

Date: `2026-05-07`

Scope:
- Define how restored OSS features must replace the enterprise license assumptions found in `pr-2565`.
- Prevent `framework/license`, `402 Payment Required`, and frontend `IS_ENTERPRISE` checks from becoming the recovered feature gate.

## Verified Baseline

| Evidence | Observation | Consequence |
|---|---|---|
| Current OSS code search | No current backend `framework/license` package or `license.IsFeatureEnabled(...)` usage exists in live source | License gating is not part of current OSS runtime behavior |
| `ui/vite.config.mts` | `process.env.BIFROST_IS_ENTERPRISE` is derived only from whether `ui/app/enterprise` exists | This flag is a build-tree selector, not a valid restored-feature entitlement system |
| `ui/app/workspace/config/views/securityView.tsx` | `IS_ENTERPRISE` suppresses enterprise auth-type requests in OSS builds | Restored OSS functionality must not continue depending on this build flag |
| `pr-2565` `framework/license/license.go` | PR introduces feature-level commercial gating | Direct conflict with OSS restoration goal |
| `pr-2565` handler patterns | Many handlers use `requireFeature(...)` and return `402 Payment Required` | Must not be carried forward as the feature-availability model |

## Replacement Rules

### 1. No commercial feature gate in restored OSS

Do not port:
- `framework/license/**`
- `/api/license` endpoints
- handler-level `requireFeature(...)` guards
- `402 Payment Required` responses for restored features

OSS rule:
- If a feature is restored, it is available in OSS.
- If a feature is not yet restored on the current branch, the route may remain absent, the UI may remain placeholder, or the code may explicitly return `501 Not Implemented` during development.

### 2. Feature availability is implementation-based, not entitlement-based

Use these truth sources instead:

| Feature type | Availability signal |
|---|---|
| Admin CRUD feature | Handler is registered and backed by working store methods |
| Runtime plugin feature | Plugin or middleware is registered and config is valid |
| Config-driven feature | Config section exists and validation passes |
| Optional infrastructure integration | Dependency is configured and health check succeeds |

### 3. Frontend must stop treating `IS_ENTERPRISE` as capability truth

`IS_ENTERPRISE` may remain as a temporary build alias detail while placeholders still exist, but restored OSS features must not depend on it for behavior.

Required migration pattern:
- remove `IS_ENTERPRISE` checks where they suppress restored API calls
- replace with one of:
  - normal data fetching against restored OSS endpoints
  - route-level presence of the feature page
  - explicit API response handling such as `404` or `501` during transitional development

Immediate example:
- `SecurityView` currently skips `useGetAuthTypeQuery` when `!IS_ENTERPRISE`
- after SCIM/SSO restoration, that query should run based on endpoint availability, not enterprise build status

### 4. Preserve ordinary auth and validation

Removing license gates does not mean removing real safeguards.

Keep or add:
- session auth
- RBAC permission checks once RBAC is restored
- config validation
- dependency availability checks
- normal `400`, `401`, `403`, `404`, `409`, `500`, `501` semantics as appropriate

### 5. Tests must be rewritten away from commercial assumptions

Do not port PR assertions that prove:
- a feature requires an enterprise license
- `/api/license` reports feature entitlements
- `402 Payment Required` is the expected missing-feature response

Prefer tests that prove:
- restored feature works in OSS
- missing configuration fails with ordinary validation errors
- protected routes require auth and RBAC where applicable

## Migration Pattern For Reused PR Code

When copying or adapting a PR handler or store layer:

1. Remove imports of `github.com/maximhq/bifrost/framework/license`
2. Delete `requireFeature(...)` helpers
3. Keep auth/RBAC/validation checks
4. If implementation is still partial, return an explicit non-commercial transitional error such as `501 Not Implemented`
5. Once implementation is real, expose it unconditionally in OSS

## Practical Rule For The Next Coding Branches

| Branch | License-neutral instruction |
|---|---|
| `oss/foundation` | Never add `framework/license`; port storage and migrations only |
| `oss/rbac` | Enforce permissions through restored RBAC, not feature entitlements |
| `oss/scim-sso` | Replace `IS_ENTERPRISE` UI gating with normal OSS endpoint usage |
| `oss/audit-logs` | Expose audit APIs directly in OSS once implemented |
| Runtime branches | Guard by config and runtime readiness, not commercial license state |

## GK-005 Acceptance Verdict

`complete`

Evidence:
- Current OSS code search completed
- PR license package and handler patterns inspected
- Frontend `IS_ENTERPRISE` gating inspected in current UI

Next task:
- Begin the first real implementation branch with `GK-006`, using the foundation and license-neutral rules above.
