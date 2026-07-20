# Memory

## Intake Snapshot

- Repository root: `/home/evans/Projects/Public/bifrost`
- Intake date: `2026-05-09`
- Working branch expected for this goal: `oss/ui-restoration`
- Predecessor research package: `.goalkeeper/goals/enterprise-docs-ui-api-parity/`

## Durable Constraints

- Use the completed research package as the fact standard.
- Only modules marked `covered` or `partial` in `fallback-parity.md` are in scope.
- `unresolved` modules are explicitly out of scope unless new official evidence is added in a separate goal.
- Official docs override the current restored OSS implementation when they conflict.

## In-Scope Module Order

1. RBAC
2. SCIM / User Provisioning
3. Guardrails
4. Adaptive Routing
5. Datadog connector
6. Audit Logs
7. Cluster
8. MCP Auth Config
9. Access Profiles / Business Units / Users parity cleanup

## GK-004 Auth Analysis

- Auth-loop fix already in place: `middlewares.go:848` sets `IsLocalAdminContextKey = true` when auth is disabled
- Local admin (`local-admin`) is assigned `super_admin` role via `rbac_seed.go:154`
- All in-scope handlers use `RBACMiddleware.MiddlewareFor()` with correct resource/operation pairs
- RBAC resource names used: RBAC, UserProvisioning, Settings, GuardrailsConfig, GuardrailsProviders, AdaptiveRouter, Observability, AuditLogs, Cluster
- Session validation: cookie-based + Bearer token + admin API key — all paths set `IsLocalAdminContextKey`
- No backend auth changes needed for GK-004 through GK-008

## Backend Contract Status (GK-005 through GK-008)

All in-scope backend handlers already expose complete CRUD + query contracts:
- RBAC: roles, permissions, role-permission binding, user-role assignment (10 endpoints)
- SSO/SCIM: provider CRUD, user list, activate/deactivate, auth-type (12 endpoints)
- Guardrails: policies, rules, violations, test (10 endpoints)
- Adaptive Routing: policies, metrics, metrics/refresh, quality scores (10 endpoints)
- Connectors: CRUD + test (6 endpoints)
- Audit: logs query + verify chain (2 endpoints)
- Cluster: status + drain (2 endpoints)
- User Groups: CRUD + members + VK + MCP groups + access profiles + users (16 endpoints)

The primary work for this goal is **UI structure/parity alignment**, not backend contract changes.
