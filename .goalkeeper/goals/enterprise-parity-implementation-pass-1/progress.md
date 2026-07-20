# Progress

## Phase 1: Contract Baseline And Scope Freeze

- [x] GK-001 Build the in-scope implementation matrix from the completed research package
  Depends on: none
  Acceptance: a concrete list exists for every `covered` or `partial` module, including route target, fallback surface to replace, expected confidence level, and primary research artifacts to follow.
  Areas: `.goalkeeper/goals/enterprise-docs-ui-api-parity/**`, `ui/app/_fallbacks/enterprise`, `ui/app/workspace`

- [x] GK-002 Audit current `oss/ui-restoration` endpoints and route ownership for the in-scope modules
  Depends on: `GK-001`
  Acceptance: each in-scope module has a backend/frontend ownership map covering handlers, store slices, active workspace routes, and known auth/session dependencies.
  Areas: `transports/bifrost-http/handlers`, `ui/lib/store/apis`, `ui/app/workspace`, `oss/ui-restoration`

- [x] GK-003 Freeze the out-of-scope unresolved modules
  Depends on: `GK-001`
  Acceptance: unresolved modules are explicitly listed as deferred and excluded from active implementation work so they are not accidentally pulled into parity claims.
  Areas: `.goalkeeper/goals/enterprise-parity-implementation-pass-1/`, `.goalkeeper/goals/enterprise-docs-ui-api-parity/fallback-parity.md`

## Phase 2: Backend Parity Alignment

- [x] GK-004 Fix auth/session prerequisites for governance and RBAC surfaces
  Depends on: `GK-002`
  Acceptance: the touched governance and RBAC routes can authenticate and load without triggering the known redirect-loop behavior.
  Areas: `transports/bifrost-http/handlers/session.go`, `rbac_handler.go`, related middleware/store/auth utilities

- [x] GK-005 Align RBAC and governance membership contracts to the in-scope parity target
  Depends on: `GK-004`
  Acceptance: RBAC, access profile, user, team, and business-unit backend contracts are coherent enough for the planned UI structure and no longer depend on fallback-only stubs.
  Areas: `transports/bifrost-http/handlers/rbac_handler.go`, `user_groups.go`, related stores/types

- [x] GK-006 Align SCIM / SSO backend contracts for provider config, mapping, and sync workflows
  Depends on: `GK-004`
  Acceptance: SCIM and SSO routes can support provider selection, long-form configuration, mapping sections, and sync/import flows required by the research package.
  Areas: `transports/bifrost-http/handlers/sso_handler.go`, related store/types

- [x] GK-007 Align runtime-policy backend contracts for Guardrails and Adaptive Routing
  Depends on: `GK-002`
  Acceptance: guardrail rules/providers and adaptive routing metrics/policies expose the data shape needed by the screenshot-backed layouts.
  Areas: `transports/bifrost-http/handlers/guardrails.go`, `adaptive_routing.go`, related stores/types

- [x] GK-008 Align connector, audit, cluster, and MCP-auth backend contracts for the remaining in-scope modules
  Depends on: `GK-002`, `GK-004`
  Acceptance: Datadog connector, audit logs, cluster status, and MCP auth surfaces have working backend shapes and route behavior suitable for the planned UI pass.
  Areas: `transports/bifrost-http/handlers/connectors.go`, `audit_handler.go`, `cluster.go`, `mcp.go`, `oauth2*.go`

## Phase 3: High-Confidence UI Restoration

- [x] GK-009 Replace the RBAC fallback-era workbench with the official table + editor structure
  Depends on: `GK-005`
  Acceptance: RBAC matches the research-backed roles list and right-side permissions editor pattern, and no in-scope RBAC path resolves to `ContactUsView`.
  Areas: `ui/app/workspace/governance/rbac/**`, `ui/lib/store/apis/rbacApi.ts`

- [x] GK-010 Replace the SCIM compatibility CRUD view with the provider-selector and mapping-driven configuration flow
  Depends on: `GK-006`
  Acceptance: SCIM follows the provider rail, long-form config, mapping cards, and sync-modal structure described in the reference specs.
  Areas: `ui/app/workspace/scim/**`, `ui/app/workspace/governance/users/**`, related store/types

- [x] GK-011 Rebuild Guardrails to the official rules-index and slide-over builder pattern
  Depends on: `GK-007`
  Acceptance: Guardrails land on a rules table with a rule drawer/builder structure consistent with the docs package, replacing the current OSS-first compatibility layout.
  Areas: `ui/app/workspace/guardrails/**`, `ui/lib/store/apis/guardrailsApi.ts`

- [x] GK-012 Rebuild Adaptive Routing as a metrics-first dashboard
  Depends on: `GK-007`
  Acceptance: Adaptive Routing prioritizes live metrics, traffic distribution, and weighted performance tables rather than the current policy-heavy form layout.
  Areas: `ui/app/workspace/adaptive-routing/**`, `ui/lib/store/apis/adaptiveRoutingApi.ts`

- [x] GK-013 Replace the generic Datadog connector wrapper with the dedicated official configuration structure
  Depends on: `GK-008`
  Acceptance: Datadog uses the dedicated selector + detail form layout and exposes the documented observability controls and metadata fields.
  Areas: `ui/app/workspace/observability/**`, `ui/lib/store/apis/connectorsApi.ts`

## Phase 4: Medium-Confidence UI And Governance Cleanup

- [x] GK-014 Implement Audit Logs, Cluster, and MCP Auth Config to the bounded parity target
  Depends on: `GK-008`
  Acceptance: these medium-confidence modules are working, no longer demo-only, and remain explicitly conservative where the research package marks inference.
  Areas: `ui/app/workspace/audit-logs/**`, `cluster/**`, `mcp-auth-config/**`

- [x] GK-015 Align access profiles, users, teams, business units, and related governance navigation to the updated parity flow
  Depends on: `GK-005`, `GK-006`, `GK-009`, `GK-010`, `GK-014`
  Acceptance: governance surfaces that were only partially evidenced are consistent with the updated identity and RBAC flows, and do not route users back into demo-only placeholders.
  Areas: `ui/app/workspace/governance/**`, related store slices and nav wiring

## Phase 5: Verification And Final Parity Audit

- [x] GK-016 Run build and targeted smoke verification for all touched in-scope modules
  Depends on: `GK-009` through `GK-015`
  Acceptance: `make build LOCAL=1` succeeds and targeted smoke checks cover auth/session load, in-scope route rendering, and removal of active fallback/demo paths.
  Areas: `Makefile`, touched backend/frontend modules, local runtime checks

- [x] GK-017 Produce the final implementation audit and deferred follow-up list
  Depends on: `GK-016`
  Acceptance: the goal folder records what was aligned exactly, what remains inference-bounded, what unresolved modules stayed deferred, and what should happen in the next goal.
  Areas: `.goalkeeper/goals/enterprise-parity-implementation-pass-1/`
