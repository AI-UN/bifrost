# Progress

## Phase 1: Inventory And Salvage Map

- [x] GK-001 Inventory current enterprise-gated UI surfaces and fallback behavior
  Depends on: none
  Acceptance: a route-to-placeholder matrix exists, covering direct fallback pages, shared `@enterprise/lib` consumers, and any E2E placeholder coverage already in the repo; each placeholder is linked to the concrete missing capability it represents.
  Areas: `ui/app/workspace`, `ui/app/_fallbacks/enterprise`, `ui/vite.config.mts`, `tests/e2e/features/placeholders`
  Result: `.goalkeeper/goals/oss-enterprise-recovery/gk-001-fallback-parity-matrix.md`

- [x] GK-002 Classify `PR #2565` by module and portability
  Depends on: `GK-001`
  Acceptance: each feature bucket is tagged as salvageable backend code, useful specs/tests, stale implementation, or missing UI; license-enforcement-only pieces are called out separately.
  Areas: `pr-2565`, `framework/configstore`, `framework/cluster`, `framework/vault`, `transports/bifrost-http/handlers`, `specs/changes`, `tests/enterprise`
  Result: `.goalkeeper/goals/oss-enterprise-recovery/gk-002-pr-2565-salvage-map.md`

- [x] GK-003 Establish the execution baseline and feature branch map
  Depends on: `GK-002`
  Acceptance: the integration baseline strategy for `oss` and the naming/order for `oss/<module>` branches are written down and consistent with the dependency graph.
  Areas: `.goalkeeper/`, git branch strategy, merge sequencing notes
  Result: `.goalkeeper/goals/oss-enterprise-recovery/gk-003-execution-baseline.md`

## Phase 2: Shared Backend Foundation

- [x] GK-004 Port or redesign shared schema, migration, and configstore primitives
  Depends on: `GK-003`
  Acceptance: the common storage and migration approach for restored enterprise features is defined against current main, with explicit notes on which PR tables and models can be reused safely.
  Areas: `framework/configstore`, `framework/configstore/tables`, `transports/config.schema.json`
  Result: `.goalkeeper/goals/oss-enterprise-recovery/gk-004-foundation-port-map.md`

- [x] GK-005 Remove or replace enterprise license gating assumptions
  Depends on: `GK-004`
  Acceptance: any reused code path that expects `framework/license` or feature entitlement checks is either removed, made unconditional for OSS, or replaced with feature-specific config.
  Areas: `framework/license`, `transports/bifrost-http/handlers/license.go`, reused handler/plugin wiring
  Result: `.goalkeeper/goals/oss-enterprise-recovery/gk-005-license-neutral-strategy.md`

## Phase 3: Governance And Identity Surface

- [x] GK-006 Restore RBAC backend and admin surface
  Depends on: `GK-005`
  Acceptance: current-main-compatible RBAC models, APIs, and shared permission checks exist, and the placeholder RBAC workspace can be backed by real data.
  Areas: `framework/configstore/rbac.go`, `framework/configstore/tables/rbac.go`, `transports/bifrost-http/handlers/rbac*.go`, `ui/app/workspace/governance/rbac`, shared layout gating

- [x] GK-007 Restore users, teams, business units, and access profiles
  Depends on: `GK-006`
  Acceptance: user-group-related APIs and data models exist, virtual key and governance screens can resolve assigned users/profiles, and no-op fallback access-profile APIs are replaced.
  Areas: `framework/configstore/user_groups.go`, `framework/configstore/tables/user_groups.go`, governance handlers, `ui/app/workspace/governance/*`, `ui/app/workspace/virtual-keys`

- [x] GK-008 Restore SCIM and SSO integration surfaces
  Depends on: `GK-007`
  Acceptance: SCIM or SSO configuration APIs are present at a practical OSS scope, shared pages stop relying on stubbed auth-type responses, and the SCIM workspace has a real backend contract.
  Areas: `framework/configstore/sso.go`, `framework/configstore/tables/sso.go`, `transports/bifrost-http/handlers/sso_handler.go`, `ui/app/workspace/scim`, `ui/app/workspace/config/views/securityView.tsx`

## Phase 4: Runtime Policy, Routing, And Observability

- [x] GK-009 Restore immutable audit log persistence and query APIs
  Depends on: `GK-005`
  Acceptance: admin-side audit storage and retrieval are implemented on current main, and the audit logs page can render actual entries instead of a CTA.
  Areas: `framework/configstore/audit*.go`, `transports/bifrost-http/handlers/audit_handler.go`, `ui/app/workspace/audit-logs`, tests

- [x] GK-010 Restore guardrails management and request/response enforcement
  Depends on: `GK-005`
  Acceptance: guardrail configuration APIs and runtime enforcement hooks are implemented or stubbed to a documented first-pass OSS scope, with working UI surfaces for configuration and provider overrides.
  Areas: `framework/configstore/guardrails.go`, `transports/bifrost-http/handlers/guardrails.go`, plugins or middleware integration points, `ui/app/workspace/guardrails`

- [x] GK-011 Restore PII redactor rules and provider overrides
  Depends on: `GK-010`
  Acceptance: PII rule APIs exist, request/response or logging redaction behavior is defined, and both PII redactor workspace routes are backed by real data.
  Areas: `framework/configstore/pii.go`, `transports/bifrost-http/handlers/pii.go`, logging integration, `ui/app/workspace/pii-redactor`

- [x] GK-012 Restore adaptive routing configuration and runtime metrics
  Depends on: `GK-005`
  Acceptance: adaptive routing config and stats APIs exist, runtime decision logic is integrated with provider routing, and the adaptive-routing page can configure and inspect real state.
  Areas: `framework/configstore/adaptive_routing.go`, `transports/bifrost-http/handlers/adaptive_routing.go`, governance or routing engine integration, `ui/app/workspace/adaptive-routing`

- [x] GK-013 Restore MCP tool groups and governance attachments
  Depends on: `GK-007`
  Acceptance: MCP group models and APIs exist, governance or request filters can reference them, and the MCP tool groups page no longer resolves to a placeholder.
  Areas: `framework/configstore/mcp_groups.go`, `transports/bifrost-http/handlers/mcp_groups.go`, `core/mcp`, `ui/app/workspace/mcp-tool-groups`

- [x] GK-014 Restore alert channels and connector-backed observability hooks
  Depends on: `GK-009`, `GK-010`, `GK-012`
  Acceptance: alert channel APIs exist, trigger sources are defined, and the alert channel and connector screens can manage real resources.
  Areas: `framework/configstore/alerting.go`, `framework/configstore/connectors.go`, `transports/bifrost-http/handlers/alerting.go`, `transports/bifrost-http/handlers/connectors.go`, `ui/app/workspace/alert-channels`, `ui/app/workspace/observability/views/plugins`

## Phase 5: Platform And Performance Features

- [x] GK-015 Restore clustering primitives at a practical OSS scope
  Depends on: `GK-004`
  Acceptance: cluster state, health, and invalidation strategy are implemented or explicitly narrowed for OSS, and the cluster workspace has a real backend surface.
  Areas: `framework/cluster/cluster.go`, `transports/bifrost-http/handlers/cluster.go`, related config and runtime wiring, `ui/app/workspace/cluster`

- [x] GK-016 Restore Vault-backed secret resolution
  Depends on: `GK-004`
  Acceptance: Vault config and resolver behavior are either fully implemented or replaced with a documented OSS-safe variant, and provider/config UIs can target the resulting backend contract.
  Areas: `framework/vault`, `transports/bifrost-http/handlers/vault.go`, config views, provider views

- [x] GK-017 Restore large payload support and related config UI
  Depends on: `GK-004`
  Acceptance: large payload behavior and settings are defined against current transport/provider code, and the client settings screen stops relying on the fallback large-payload fragment.
  Areas: request streaming and upload paths, `ui/app/workspace/config/views/clientSettingsView.tsx`, large-payload fallback fragments

## Phase 6: Frontend Restoration, Verification, And Merge

- [x] GK-018 Replace placeholder enterprise pages with real workspace implementations
  Depends on: `GK-006`, `GK-007`, `GK-008`, `GK-009`, `GK-010`, `GK-011`, `GK-012`, `GK-013`, `GK-014`, `GK-015`, `GK-016`, `GK-017`
  Acceptance: direct enterprise placeholder pages are replaced by functioning implementations or shared-page integrations, without requiring the missing private `ui/app/enterprise` tree.
  Areas: `ui/app/workspace/*`, `ui/app/_fallbacks/enterprise/components`, shared route layouts and views

- [x] GK-019 Replace shared `@enterprise/lib` stubs with real OSS implementations
  Depends on: `GK-007`, `GK-008`, `GK-017`
  Acceptance: no-op fallback hooks for access profiles, SCIM auth type, large payload config, virtual key users, and RBAC context are replaced or materially upgraded for restored OSS behavior.
  Areas: `ui/app/_fallbacks/enterprise/lib/**`, shared workspace consumers, store integration

- [x] GK-020 Add verification coverage and prepare final integration on the `oss/*` branch line
  Depends on: `GK-018`, `GK-019`
  Acceptance: builds pass for the integrated branch, targeted Go and UI verification is recorded, the fallback-UI parity matrix is checked item by item, and merge order plus unresolved follow-ups are documented.
  Areas: `Makefile`, `tests/e2e`, `tests/enterprise` or replacement tests, module build/test commands, integration notes
  Result: `.goalkeeper/goals/oss-enterprise-recovery/gk-020-verification-map.md`

- [x] GK-021 Run final missing-feature vs fallback-UI parity audit
  Depends on: `GK-020`
  Acceptance: every originally demo-only fallback surface is reviewed against the restored OSS implementation, with each item marked as fully restored, intentionally narrowed, or still missing; any residual gaps are explicitly documented before closing the goal.
  Areas: `.goalkeeper/`, `ui/app/_fallbacks/enterprise`, `ui/app/workspace`, final verification notes
  Result: `.goalkeeper/goals/oss-enterprise-recovery/gk-021-final-parity-audit.md`

## Work Log

- `2026-05-07`: Completed `GK-001` by producing `gk-001-fallback-parity-matrix.md`, covering full-page placeholder routes, embedded demo-only surfaces, shared `@enterprise/lib` stubs, and current E2E placeholder coverage.
- `2026-05-07`: Completed `GK-002` by producing `gk-002-pr-2565-salvage-map.md`, tagging each PR feature bucket by salvageability, missing UI, stub level, and OSS-conflicting license assumptions.
- `2026-05-07`: Completed `GK-003` by producing `gk-003-execution-baseline.md`, confirming `oss` already matches `upstream/main` and defining the concrete `oss/<module>` branch order and dependency map.
- `2026-05-07`: Completed `GK-004` by producing `gk-004-foundation-port-map.md`, narrowing the first foundation patch to RBAC, SSO, user groups, audit logs, and MCP tool groups storage/migrations, while deferring runtime-heavy tables.
- `2026-05-07`: Completed `GK-005` by producing `gk-005-license-neutral-strategy.md`, replacing PR license-gating assumptions with OSS-native availability rules based on implementation, auth, config, and runtime readiness.
- `2026-05-07`: Branch naming drift temporarily occurred because a top-level local branch named `oss` blocked `oss/<module>` refs.
- `2026-05-07`: User removed the local `oss` branch, current work branch is now `oss/foundation`, and the execution plan has been restored to the original `oss/<module>` naming scheme.
- `2026-05-07`: Started `GK-006` implementation by adding RBAC tables, migrations, and `RDBConfigStore` role/permission assignment methods in `framework/configstore`; verified with `go test ./configstore/...` from the `framework` module on `oss/foundation`.
- `2026-05-07`: Extended `GK-006` with OSS-native RBAC seeding, permission middleware, transport handlers, a real fallback-backed `rbacApi`, live `RbacProvider` context, and a non-demo `ui/app/workspace/governance/rbac/rbacView.tsx`; verified with `go test ./configstore/...`, `go test -run '^$' ./bifrost-http/lib ./bifrost-http/server ./bifrost-http/websocket`, `go test ./bifrost-http/handlers ./bifrost-http/integrations`, and `npx tsc --noEmit`.
- `2026-05-07`: Completed `GK-006` by extending the new RBAC middleware into selected current-main governance, provider, config, logging, plugin, and MCP handlers, plus focused route tests proving permission denial/allowance on registered routes.
- `2026-05-07`: Started `GK-007` implementation by porting `user_groups` storage primitives into current main: new tables, `RDBConfigStore` CRUD/assignment methods, migrations, and configstore tests; verified with `go test ./configstore/...` and `go test -run '^$' ./bifrost-http/lib`.
- `2026-05-07`: Verified the first `GK-007` transport slice by registering `UserGroupHandler` in `bifrost-http/server`, then passing `go test ./bifrost-http/handlers ./bifrost-http/server ./bifrost-http/integrations` plus `go test -run '^$' ./bifrost-http/lib`.
- `2026-05-07`: Extended `GK-007` with OSS compatibility endpoints for synthetic access-profile and virtual-key user lookups on top of `user_groups`, replaced the fallback no-op `accessProfileApi` and `virtualKeyUsersApi`, and added focused transport tests for those contracts.
- `2026-05-07`: Replaced the direct `governance/access-profiles` demo CTA with a real OSS page backed by the new compatibility endpoint; verified with the same transport test set and `npx tsc --noEmit` in `ui`.
- `2026-05-07`: Replaced the direct `governance/users` and `governance/business-units` demo CTAs with OSS-native read-only pages backed by the same `user_groups` compatibility layer, including a new `GET /api/users` aggregate endpoint and UI hooks for users/business units; verified with the same transport test set and `npx tsc --noEmit` in `ui`.
- `2026-05-07`: Completed `GK-008` by adding current-main-compatible SSO/SCIM tables, configstore methods, migrations, and an OSS-safe `SSOHandler`; replaced the fallback `scimApi` no-op with real auth-type/provider/user endpoints, updated `securityView` to stop keying off `IS_ENTERPRISE` for auth-type detection, and replaced the SCIM CTA with a live provider-backed workspace; verified with `go test ./configstore/...`, `go test ./bifrost-http/handlers ./bifrost-http/server ./bifrost-http/integrations`, `go test -run '^$' ./bifrost-http/lib`, and `npx tsc --noEmit`.
- `2026-05-07`: Completed `GK-009` by adding current-main-compatible audit-log storage, migrations, query and hash-chain verification APIs, an OSS-safe `AuditHandler`, and a live audit-logs workspace page; additionally wired SSO provider create/update/delete to emit audit entries so the new page can show real data; verified with `go test ./configstore/...`, `go test ./bifrost-http/handlers ./bifrost-http/server ./bifrost-http/integrations`, `go test -run '^$' ./bifrost-http/lib`, and `npx tsc --noEmit`.
- `2026-05-07`: Completed `GK-010` at a documented OSS-safe first-pass scope by adding guardrail policy/rule/violation storage and APIs, replacing the direct `guardrails`, `guardrails/configuration`, and `guardrails/providers` demo CTAs with live OSS pages, fixing guardrails route-level RBAC behavior, and wiring a real inference middleware that evaluates enabled global/local keyword and regex rules on text/chat/responses input requests with block/warn/log-only actions plus violation capture; verified with `go test ./configstore/...`, `go test ./bifrost-http/handlers ./bifrost-http/server ./bifrost-http/integrations`, `go test -run '^$' ./bifrost-http/lib`, and `npx tsc --noEmit`.
- `2026-05-07`: Completed `GK-011` at a documented OSS-safe first-pass scope by adding PII policy/rule/token persistence, OSS-safe `/api/pii/*` CRUD and preview endpoints, request-side redaction middleware for text/chat/responses inference payloads, and live `pii-redactor` / `pii-redactor/providers` workspace pages plus fallback RTK Query integration; verified with `go test ./configstore/...`, `go test ./bifrost-http/handlers ./bifrost-http/server ./bifrost-http/integrations`, `go test -run '^$' ./bifrost-http/lib`, and `npx tsc --noEmit`.
- `2026-05-07`: Completed `GK-012` at a documented OSS-safe first-pass scope by adding adaptive-routing policy / metric / quality-score tables, CRUD and log-refresh APIs, a runtime governance selection hook that consults stored adaptive metrics before falling back to existing weighted routing, and a live `adaptive-routing` workspace page for policies, quality scores, and metrics inspection; verified with `go test ./configstore/...`, `go test ./...` in `plugins/governance`, `go test ./bifrost-http/handlers ./bifrost-http/server ./bifrost-http/integrations`, and `npx tsc --noEmit`.
- `2026-05-07`: Completed `GK-013` at a current-main-compatible OSS scope by adding MCP tool group tables, migrations, CRUD and membership APIs, user-group attachment management, virtual-key resolution of inherited MCP tool groups, governance and MCP server include-tool filtering for those inherited groups, and a live `mcp-tool-groups` workspace page backed by real RTK Query hooks; verified with `go test ./configstore/...`, `go test ./...` in `plugins/governance`, `go test ./bifrost-http/handlers ./bifrost-http/server ./bifrost-http/integrations`, and `npx tsc --noEmit`.
- `2026-05-07`: Completed `GK-014` at a documented OSS-safe first-pass scope by adding alert-channel and connector tables, CRUD/configstore methods, OSS-safe alert-channel and connector handlers, a defined trigger-source catalog, and live `alert-channels`, `observability/bigquery`, and `observability/datadog` screens backed by restored fallback RTK Query APIs; verified with `go test ./configstore/...`, `go test ./bifrost-http/handlers ./bifrost-http/server ./bifrost-http/integrations`, and `npx tsc --noEmit`.
- `2026-05-07`: Completed `GK-015` at a documented OSS-safe single-node scope by adding a new `framework/cluster` local-manager contract, OSS-safe `/api/cluster/status` and `/api/cluster/drain` endpoints, server wiring for the local cluster manager, and a live `/workspace/cluster` page that reports node health, drain state, and the narrowed invalidation strategy instead of the Book-a-demo CTA; verified with `go test ./cluster/...`, `go test ./bifrost-http/handlers ./bifrost-http/server ./bifrost-http/integrations`, and `npx tsc --noEmit`.
- `2026-05-07`: Completed `GK-016` at a documented OSS-safe first-pass scope by extending `EnvVar` and `envutils` to resolve `vault://...` references, adding a new `framework/vault` KV resolver package with token + approle auth, persisting Vault config in `framework/configstore`, wiring startup/load-time Vault activation into `transports/bifrost-http/lib/config.go`, adding OSS-safe `/api/vault/*` handler routes and a live Vault section in `ui/app/workspace/config/views/securityView.tsx`, and updating provider key inputs to accept Vault references; additionally fixed a real transport bug by switching Vault HTTP calls off `*fasthttp.RequestCtx` onto standard timeout contexts so handler-side health/auth checks do not panic under `net/http`; verified with `go test ./schemas` in `core`, `go test ./vault -v` plus `go test ./configstore -run Vault -v` in `framework`, `go test ./bifrost-http/lib ./bifrost-http/handlers -run Vault -v` plus `go test -run '^$' ./bifrost-http/server` in `transports`, and `npx tsc --noEmit` in `ui`.
- `2026-05-07`: Completed `GK-017` at a documented OSS-safe first-pass scope by adding a current-main large-payload config model and framework-config persistence, backfilling the `framework_configs` table with large-payload columns, loading and applying the resolved config at startup and live updates, wiring a new OSS `/api/payload/config` transport handler plus request middleware that injects payload thresholds/max-size guards into the Bifrost context, replacing the fallback `largePayloadApi` no-op and `largePayloadSettingsFragment` null render with a real client-settings form, and switching the client-settings success copy to reflect live activation; verified with `go test ./schemas` in `core`, `go test ./... -run '^$'` in `core`, `go test ./configstore/...` in `framework`, `go test ./bifrost-http/lib ./bifrost-http/handlers ./bifrost-http/server ./bifrost-http/integrations` in `transports`, and `npx tsc --noEmit` in `ui`.
- `2026-05-07`: Started `GK-018` by replacing `/workspace/mcp-auth-config`'s direct enterprise CTA import with a live OSS page that reads configured MCP clients, summarizes header/OAuth/per-user-OAuth auth usage, and links operators into the existing MCP Registry management flows; verified with `npx tsc --noEmit` in `ui`.
- `2026-05-07`: Continued `GK-018` by restoring `dashboard` user rankings end to end: exposed `GET /api/logs/user-rankings` from the logging transport over the existing `framework/logstore.GetUserRankings` backend, added the matching RTK query in `ui/lib/store/apis/logsApi.ts`, replaced the fallback enterprise tab import in `ui/app/workspace/dashboard/page.tsx`, and introduced a live `ui/app/workspace/dashboard/components/userRankingsTab.tsx` with sortable rankings, trend badges, and summary cards; verified with `go test ./bifrost-http/handlers ./bifrost-http/server ./bifrost-http/integrations` in `transports` and `npx tsc --noEmit` in `ui`.
- `2026-05-07`: Continued `GK-018` by replacing the `config/api-keys` scoped-key CTA with a live OSS compatibility page that reads security config and recent virtual keys, and by replacing the prompt sidebar deployments CTA with a real summary/commit panel backed by existing prompt versions and sessions; verified with `npx tsc --noEmit` in `ui`.
- `2026-05-07`: Closed `GK-018` after a workspace-wide parity sweep showed no remaining demo-only `@enterprise/components` imports in active workspace screens; the only surviving component-path imports are the now-functional `LargePayloadSettingsFragment` and `TeamsView` fallback integrations rather than paywall CTAs.
- `2026-05-07`: Completed `GK-019` by promoting restored fallback APIs into the main `ui/lib/store` barrel and switching active workspace consumers away from direct `@enterprise/lib/store/apis` entrypoints across audit logs, SCIM, security, client settings, virtual-key usage, access profiles, users, business units, MCP tool groups, guardrails, PII redaction, cluster, alert channels, adaptive routing, connectors, and RBAC management; verified with `npx tsc --noEmit` in `ui`, and a follow-up alias sweep now shows only the intentional RBAC/context import path plus the shared `baseQueryWithRefresh` compatibility wrapper outside of `@enterprise/components`.
- `2026-05-07`: Synced Goal Keeper branch terminology after confirming the user removed the top-level local `oss` branch; `GK-020` and the active-goal focus now point at final integration along the `oss/*` branch line, with `oss/foundation` still serving as the current integration branch.
- `2026-05-07`: Completed `GK-020` by creating `gk-020-verification-map.md`, checking every `GK-001` fallback surface against the current implementation state, recording residual compatibility imports, updating `tests/e2e/features/placeholders/placeholders.spec.ts` from CTA assertions to restored-route assertions, and re-running targeted verification with `go test ./configstore/...` in `framework`, `go test ./...` in `plugins/governance`, `go test ./bifrost-http/handlers ./bifrost-http/server ./bifrost-http/integrations` in `transports`, `npx tsc --noEmit` in `ui`, plus `npm ci` and `npx playwright test features/placeholders/placeholders.spec.ts --list` in `tests/e2e`.
- `2026-05-07`: Completed `GK-021` by creating `gk-021-final-parity-audit.md`, classifying every original `GK-001` fallback surface as `fully restored`, `intentionally narrowed`, or `still missing`; the final audit found `0` `still missing` surfaces. Added end-state operational evidence by fixing `Makefile` so `make build` uses the local `go.work` and fails hard on `go build` errors, then verifying `make build` succeeds and `./tmp/bifrost-http` starts successfully on `127.0.0.1:18080` with `/health` returning `{\"components\":{\"db_pings\":\"ok\"},\"status\":\"ok\"}`.
- `2026-05-07`: Reduced remaining UI compatibility debt after `GK-021` by localizing the last `@enterprise/components` consumers (`LargePayloadSettingsFragment`, `TeamsView`, `LoginView`) and the OSS no-op store refresh/token helpers into local `ui/lib/store/utils`; verification now shows `rg -n "@enterprise/components" ui` returns no matches and `npx tsc --noEmit` in `ui` still passes.
- `2026-05-07`: Continued the post-`GK-021` UI compatibility cleanup by localizing active RBAC imports to `ui/lib/rbac.ts`, moving the live RBAC context implementation into `ui/lib/contexts/rbacContext.tsx`, localizing the empty enterprise slice placeholders into `ui/lib/store/slices/enterpriseSlices.ts`, and migrating restored active API slices/types for access profiles, users, RBAC, large payload, and related governance paths into local `ui/lib/store/apis` and `ui/lib/types`; verification now shows `rg -n '@enterprise/components|@enterprise/lib/store/utils|@enterprise/lib/store/slices|from "@enterprise/lib"|from "@enterprise/lib/contexts/rbacContext"' ui` returns no active-code matches, `npx tsc --noEmit` in `ui` passes, and `make build` still succeeds.
- `2026-05-07`: Tightened the previously narrowed `config/api-keys` restoration by adding persisted `admin_api_keys` storage + migration support, OSS `/api/admin-api-keys` CRUD routes with `Settings` RBAC, Bearer-token admin API key auth inside the main auth middleware, a live `adminApiKeysApi` RTK Query slice, and a real `/workspace/config/api-keys` management surface that creates, activates, deactivates, deletes, and previews Admin API Keys while keeping Virtual Keys as the scoped inference-key compatibility layer; then extended the login/session flow so Admin API Keys can be validated via `/api/session/api-key-login`, recognized by `/api/session/is-auth-enabled`, and reused by the browser through restored local token storage for Bearer-authenticated admin UI access. Verified with `go test ./configstore/...` in `framework`, `go test ./bifrost-http/handlers ./bifrost-http/server` in `transports`, `npx tsc --noEmit` in `ui`, and `make build` at repo root.
- `2026-05-07`: Re-checked the branch-topology success criterion against real git state: `git branch --list 'oss/*'` still returns only `oss/foundation`, `git log --all --branches='oss/*'` shows no dedicated `oss/<module>` history, and the restored enterprise work is still primarily present as an uncommitted dirty worktree on `oss/foundation`; this means the remaining branch-structure requirement is now an explicit process/commit decision, not a hidden code gap.
- `2026-05-07`: Tightened the previously narrowed `users` and `access-profiles` compatibility layer by changing `UserGroupHandler` to seed `/api/users`, `/api/virtual-keys/{vk_id}/users`, and `/api/users/{user_id}/access-profiles` from real `sso_external_users` directory data when available, then overlay `user_groups` membership as the business-unit assignment source; added enriched user metadata (`directory_user_id`, `external_id`, `provider_id`, `identity_source`, `active`, `last_login_at`) plus access-profile user fields, updated the governance `users` and `access-profiles` pages to display that directory context, and extended compatibility tests to cover external-user enrichment and external-only users. Verified with `go test ./bifrost-http/handlers -run 'TestUserGroupCompatRoute_'` in `transports` and `npx tsc --noEmit` in `ui`.
- `2026-05-07`: Tightened the previously narrowed `business-units` compatibility layer by enriching `GET /api/user-groups` with member counts, external/local/inactive splits, and provider summaries derived from the restored external-user directory overlay while preserving the existing `id/name/description` contract needed by `mcp-tool-groups`; updated the business-units UI to surface those member and directory-sync summaries, and extended compatibility tests with a dedicated `ListGroups` assertion. Verified with `go test ./bifrost-http/handlers -run 'TestUserGroupCompatRoute_'`, `go test ./bifrost-http/server -run '^$'` in `transports`, and `npx tsc --noEmit` in `ui`.
- `2026-05-07`: Tightened the previously narrowed `virtual-keys` ownership display by replacing the plain assigned-user name list in both `virtualKeySheet` and `virtualKeyDetailsSheet` with directory-aware summaries that show identity source, provider, and inactive state for assigned users; this reused the enriched user metadata already flowing through `useVirtualKeyUsage`. Verified with `npx tsc --noEmit` in `ui`.
- `2026-05-07`: Re-ran the end-state audit on the latest `oss/foundation` worktree: `rg -n "@enterprise/components|ContactUsView" ui/app/workspace ui/components ui/lib` returned no matches in active UI code, `npx tsc --noEmit` in `ui` passed again, and `make build` completed successfully after the latest governance identity tightening. The audit still found only one unmet success criterion in real repo state: `git branch --list 'oss*'` still shows only `oss/foundation`, so the planned `oss/<module>` branch trail has not been materialized yet.
- `2026-05-07`: Converted the remaining branch-topology blocker into a concrete execution artifact by creating `gk-022-restack-options.md`. The new document maps the current dirty `oss/foundation` worktree onto both a strict module restack and a coarse-grained `oss/*` restack, recommends the coarse-grained variant as lower risk for the already-interleaved worktree, and explicitly defers any actual branch-history rewrite until the user chooses a restack option.
- `2026-05-07`: Made the recommended coarse-grained restack staging-ready by creating `gk-023-restack-manifest.md`. The new manifest turns the current dirty worktree into branch-owned file buckets for `oss/foundation`, `oss/governance-identity`, `oss/runtime-policy`, `oss/platform`, and `oss/ui-restoration`, explicitly excludes unrelated local files and `.goalkeeper/**`, and marks the shared transport/UI files that will require hunk-level splitting during the eventual restack.
- `2026-05-07`: Formalized the end-state audit into `gk-024-completion-checklist.md`, mapping the original user objective and success criteria to current artifacts and command evidence. That checklist concludes that feature restoration, parity audit, buildability, and runtime evidence are all present, and that the only remaining unmet criterion in real git state is the absence of actual `oss/*` branch history beyond `oss/foundation`.
- `2026-05-07`: Added `gk-025-restack-runbook.md` to turn the recommended coarse-grained restack into an execution playbook. The runbook defines the branch creation order, baseline patch-capture method, branch-by-branch verification checkpoints, merge-back order, and the final verification gate that must pass before the goal can be marked complete after restack execution.
- `2026-05-08`: Executed the coarse-grained `oss/*` restack on real git history by creating committed branches `oss/foundation-restack`, `oss/governance-identity`, `oss/runtime-policy`, `oss/platform`, and `oss/ui-restoration`, then fast-forwarding the integrated result back onto `oss/foundation` without recreating a top-level `oss` branch.
- `2026-05-08`: Re-ran the committed-state verification set after the restack on `oss/foundation`: `git branch --list 'oss*'`, `go test ./configstore/...` in `framework`, `go test ./bifrost-http/handlers ./bifrost-http/server ./bifrost-http/integrations` in `transports`, `go test ./...` in `plugins/governance`, `npx tsc --noEmit` in `ui`, `make build LOCAL=1`, and a live `./tmp/bifrost-http -app-dir /tmp/bifrost-oss-restack-check -host 127.0.0.1 -port 18081` boot with `/health` returning `{"components":{"db_pings":"ok"},"status":"ok"}` before graceful shutdown. A follow-up UI text scan still found one sidebar marketing card that says `Book a demo`, but it is a static support CTA in `ui/components/sidebar.tsx`, not a remaining enterprise fallback surface.
