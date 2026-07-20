# Memory

## Intake Snapshot

- Repository root: `/home/evans/Projects/Public/bifrost`
- Intake date: `2026-05-07`
- Intake branch: `oss`
- Unrelated local modifications present: `.gitignore`, `AGENTS.md`

## Confirmed OSS Gating Pattern

- [ui/vite.config.mts](/home/evans/Projects/Public/bifrost/ui/vite.config.mts) aliases `@enterprise` to `ui/app/enterprise` when present, otherwise to `ui/app/_fallbacks/enterprise`.
- The real `ui/app/enterprise` tree is absent in this OSS checkout, so all enterprise imports resolve to fallback implementations.
- Shared fallback API hooks currently return no-op data for enterprise-only dependencies such as access profiles and SCIM auth type:
  - [ui/app/_fallbacks/enterprise/lib/store/apis/accessProfileApi.ts](/home/evans/Projects/Public/bifrost/ui/app/_fallbacks/enterprise/lib/store/apis/accessProfileApi.ts)
  - [ui/app/_fallbacks/enterprise/lib/store/apis/scimApi.ts](/home/evans/Projects/Public/bifrost/ui/app/_fallbacks/enterprise/lib/store/apis/scimApi.ts)

## Confirmed Placeholder Surfaces

Routes that directly import fallback enterprise components include:
- `adaptive-routing`
- `alert-channels`
- `audit-logs`
- `cluster`
- `config/api-keys`
- `governance/access-profiles`
- `governance/business-units`
- `governance/rbac`
- `governance/teams`
- `governance/users`
- `guardrails`
- `guardrails/providers`
- `guardrails/configuration`
- `mcp-auth-config`
- `mcp-tool-groups`
- `pii-redactor`
- `pii-redactor/providers`
- `pii-redactor/rules`
- `scim`
- `dashboard` user rankings tab
- `config/client-settings` large payload settings
- `observability` connector views

Shared workspace screens that already depend on `@enterprise/lib` stubs include RBAC gating, SCIM auth type, access profile usage, virtual key user assignment, model/provider governance, routing rules, plugin views, logging, observability, dashboard, and multiple config sections.

## Added Acceptance Constraint

- The fallback enterprise UI is part of the final acceptance oracle, not only a symptom of missing functionality.
- Execution must preserve a matrix that maps each demo-only placeholder or enterprise stub surface to the concrete feature gap it represented.
- Goal completion requires a final parity audit against that matrix so restored OSS functionality can be checked one surface at a time.

## PR 2565 Evidence

- Upstream PR URL: `https://github.com/maximhq/bifrost/pull/2565`
- Pull ref still fetches successfully as local branch `pr-2565`
- Current merge-base versus `upstream/main`: `827783e7786c5bc3e52ec68d10d6bb4f41be1d63`
- Head commit analyzed: `f5eadcd1622a45e009ce39d733227a2809640389` (`update enterprise codes`)

### What The Fetched PR Actually Contains

Portable or partially portable material:
- New backend configstore models and migrations for adaptive routing, alerting, audit, connectors, guardrails, MCP groups, PII, RBAC, SSO, and user groups
- New transport handlers for adaptive routing, alerting, audit, cluster, connectors, guardrails, license, MCP groups, payload, PII, RBAC, SSO, user groups, and Vault
- New backend support packages: `framework/cluster`, `framework/vault`, `framework/license`
- Tests under `tests/enterprise/` and selected Playwright coverage for RBAC, audit logs, and license
- Extensive design and requirements material under `specs/`

Gaps or adaptation hazards:
- The fetched PR does not include the real `ui/app/enterprise` implementation; only `ui/specs/` exists on the PR ref
- The PR encodes enterprise license enforcement as a first-class dependency, which conflicts with the OSS-restoration target
- New plugin packages described in PR specs are not actually present in the fetched code

## Historical Feature Buckets Seen In PR Specs

The PR's own design index groups work into these feature areas:
- RBAC
- Audit Logs
- Guardrails
- PII Redactor
- SSO / SCIM
- Adaptive Routing
- Clustering
- Vault
- Alert Channels
- Large Payload
- MCP Tool Groups
- User Groups
- Data Connectors
- License Enforcement

For OSS restoration, `License Enforcement` is a gating concern to neutralize, not a feature goal to preserve.

## 2026-05-07 Checkpoint

- `GK-001` is complete.
- Inventory artifact created: `.goalkeeper/goals/oss-enterprise-recovery/gk-001-fallback-parity-matrix.md`
- The frontend gap baseline now distinguishes:
  - full-page demo-only workspace fallbacks
  - embedded or hybrid enterprise CTAs inside otherwise functional OSS pages
  - shared `@enterprise/lib` stubs that silently suppress missing backend capabilities
- Important nuance captured for later work:
  - `TeamsView` is not a demo-only placeholder, but it still depends on always-allow fallback RBAC.
  - `APIKeysView` is hybrid: OSS basic auth works, but scoped API keys remain enterprise-gated.
  - large payload settings currently disappear because the fallback fragment returns `null` and the enterprise hooks are no-ops.

- `GK-002` is complete.
- Salvage artifact created: `.goalkeeper/goals/oss-enterprise-recovery/gk-002-pr-2565-salvage-map.md`
- Durable conclusions from PR classification:
  - `pr-2565` contains no `ui/app/enterprise` tree at all, so frontend restoration must be implemented fresh on current OSS.
  - strongest backend salvage candidates are RBAC, Audit Logs, Vault, MCP Tool Groups, and User Groups storage/API layers.
  - Guardrails, PII, Adaptive Routing, Alerting, SSO/SCIM, Clustering, Data Connectors, and Large Payload are only partially implemented in the PR and still need major runtime work.
  - `framework/license` and handler-level feature gating are directly at odds with the OSS restoration goal and must be treated as negative reference, not implementation target.
  - at least one concrete stale mismatch exists inside the PR: guardrails tests hit `/api/guardrails/*` while the handler registers `/api/enterprise/guardrails/*`.

- `GK-003` is complete.
- Execution-baseline artifact created: `.goalkeeper/goals/oss-enterprise-recovery/gk-003-execution-baseline.md`
- Durable execution decisions:
  - `oss` is already at the same commit as `upstream/main`, so no rebase or sync step is needed before coding.
  - first coding branch should be `oss/foundation`.
  - `oss/ui-restoration` is intentionally last because PR frontend code is unrecoverable.
  - salvage-first branches: foundation, RBAC, user groups, audit logs, MCP tool groups, Vault.
  - fresh-heavy branches: large payload and final UI restoration; mixed branches cover guardrails, PII, adaptive routing, alerting, connectors, cluster, and SSO/SCIM.

- `GK-004` is complete.
- Foundation artifact created: `.goalkeeper/goals/oss-enterprise-recovery/gk-004-foundation-port-map.md`
- Durable foundation decisions:
  - first storage/migration patch should cover only RBAC, SSO, user groups, audit logs, and MCP tool groups.
  - current `transports/config.schema.json` already contains many enterprise-shaped config blocks, so broad schema rewrites are not the first bottleneck.
  - runtime-heavy storage for guardrails, PII, adaptive routing, alerting, and connectors is intentionally deferred until their runtime contracts are clearer.

- `GK-005` is complete.
- License-neutral artifact created: `.goalkeeper/goals/oss-enterprise-recovery/gk-005-license-neutral-strategy.md`
- Durable license-neutral decisions:
  - restored OSS features must not depend on `framework/license`, `/api/license`, or `402 Payment Required`.
  - frontend `IS_ENTERPRISE` is only a build-tree signal and must not remain the availability truth for restored OSS features.
  - feature availability should be determined by implementation presence, auth/RBAC, config validity, and runtime readiness.

- Branch naming drift resolved:
  - local branch `oss` was removed at the user's request
  - current work branch is `oss/foundation`
  - original `oss/<module>` naming is restored for subsequent feature branches
  - Goal Keeper execution notes and `GK-020` wording should refer to final integration on the `oss/*` branch line, not a recreated top-level `oss` branch
  - branch check on `2026-05-07` reconfirmed there is still no top-level local `oss` branch; `git branch --list` shows `oss/foundation` as the only active `oss/*` branch

## 2026-05-07 GK-020 Verification Checkpoint

- `GK-020` is complete.
- Verification artifact created:
  - `.goalkeeper/goals/oss-enterprise-recovery/gk-020-verification-map.md`
- Durable conclusions from the verification sweep:
  - every direct demo-only route from `GK-001` now maps either to a real restored OSS implementation or to an explicitly narrowed OSS compatibility surface with live behavior
  - active workspace consumers no longer import `@enterprise/lib/store/apis` directly
  - the only remaining direct `@enterprise/components` imports in active workspace code are:
    - `ui/app/workspace/config/views/clientSettingsView.tsx`
    - `ui/app/workspace/governance/teams/page.tsx`
  - both remaining component-path imports are functional compatibility surfaces, not demo CTAs
  - widespread `@enterprise/lib` imports still remain for RBAC enums/hooks/types, but these are now backed by the live fallback RBAC context rather than an always-allow stub
- Verification commands that passed in this checkpoint:
  - `npx tsc --noEmit` in `ui`
  - `go test ./bifrost-http/handlers ./bifrost-http/server ./bifrost-http/integrations` in `transports`
  - `go test ./configstore/...` in `framework`
  - `go test ./...` in `plugins/governance`
  - `npm ci` and `npx playwright test features/placeholders/placeholders.spec.ts --list` in `tests/e2e`
- Important environment fact:
  - `tests/e2e` had a `package.json` and lockfile but no installed `node_modules` at the start of `GK-020`, so `npm ci` was required before Playwright discovery would work
- Remaining focus for final closure:
  - `GK-021` must turn the verification map into the explicit final parity verdict and decide how to classify the remaining `@enterprise/*` compatibility imports

## 2026-05-07 GK-021 And Build/Run Checkpoint

- `GK-021` is complete.
- Final parity artifact created:
  - `.goalkeeper/goals/oss-enterprise-recovery/gk-021-final-parity-audit.md`
- Durable conclusions from the final parity audit:
  - `0` original `GK-001` fallback surfaces remain `still missing`
  - `8` surfaces are classified as `fully restored`
  - `20` surfaces are classified as `intentionally narrowed`
  - remaining `@enterprise/*` imports are now compatibility debt, not fallback parity failures
- Build-system conclusion:
  - the original `make build` target was a false-positive risk because it forced `GOWORK=off` for local repo builds and did not stop on `go build` failure inside the shell block
  - `Makefile` now uses the local workspace for `make build` and has `set -e` in the build shell so `go build` errors fail the target
- End-state operational evidence:
  - `make build` now succeeds on `oss/foundation`
  - `tmp/bifrost-http` was launched successfully with `-app-dir /tmp/bifrost-oss-check -host 127.0.0.1 -port 18080`
  - `curl -sf http://127.0.0.1:18080/health` returned `{"components":{"db_pings":"ok"},"status":"ok"}`
  - the process shut down cleanly on `SIGINT`
- Remaining uncertainty before goal closure:
  - whether the user considers the `intentionally narrowed` OSS replacements acceptable completion for "enterprise recovery", or wants another pass toward closer enterprise-SKU parity in those areas

## 2026-05-07 Post-GK-021 UI Compatibility Cleanup

- Active UI code no longer imports `@enterprise/components` at all.
- Local replacements now exist for:
  - `ui/app/workspace/config/views/largePayloadSettingsFragment.tsx`
  - `ui/app/workspace/governance/teams/teamsView.tsx`
  - `ui/app/login/loginView.tsx`
- The OSS no-op store refresh/token helpers are now local:
  - `ui/lib/store/utils/baseQueryWithRefresh.ts`
  - `ui/lib/store/utils/tokenManager.ts`
- `ui/lib/store/apis/baseApi.ts` and `ui/components/sidebar.tsx` no longer import those helpers from `@enterprise/lib/store/utils/*`.
- Remaining compatibility debt is now concentrated in:
  - fallback duplicate files still living under `ui/app/_fallbacks/enterprise/lib/**`
  - intentionally narrowed OSS scope choices in the final parity audit

## 2026-05-07 Deep UI Decoupling Checkpoint

- Active UI/store code no longer imports:
  - `@enterprise/components`
  - `@enterprise/lib/store/utils/*`
  - `@enterprise/lib/store/slices`
  - `@enterprise/lib` or `@enterprise/lib/contexts/rbacContext` outside fallback-tree files
- New local OSS-owned UI plumbing now exists at:
  - `ui/lib/rbac.ts`
  - `ui/lib/contexts/rbacContext.tsx`
  - `ui/lib/store/slices/enterpriseSlices.ts`
  - `ui/lib/types/accessProfile.ts`
  - `ui/lib/types/user.ts`
  - local copies of restored API slices under `ui/lib/store/apis/*.ts`
- Build/verification status after this cleanup:
  - `npx tsc --noEmit` in `ui` passed
  - `make build` passed again on `oss/foundation`
- The remaining uncertainty is now primarily product-scope, not code-plumbing:
  - the final parity audit still classifies many surfaces as `intentionally narrowed`
  - the branch topology remains a single surviving `oss/foundation` integration branch rather than a preserved per-module `oss/*` branch stack

## 2026-05-07 Governance Identity Tightening Checkpoint

- The `users` and `access-profiles` compatibility layer is no longer purely `user_groups`-synthetic:
  - [transports/bifrost-http/handlers/user_groups.go](/home/evans/Projects/Public/bifrost/transports/bifrost-http/handlers/user_groups.go) now loads `sso_external_users` as a directory overlay and enriches `/api/users`, `/api/virtual-keys/{vk_id}/users`, and `/api/users/{user_id}/access-profiles` with real directory metadata where available.
- Durable compatibility rule:
  - keep `user.id` on these endpoints aligned with the membership lookup key expected by `/api/users/{user_id}/access-profiles`, even when a real directory user exists; expose the actual external directory record separately through `directory_user_id` and related metadata.
- New user metadata now exposed to active UI code:
  - `directory_user_id`
  - `external_id`
  - `provider_id`
  - `identity_source`
  - `active`
  - `last_login_at`
- New access-profile metadata now exposed to active UI code:
  - `user_name`
  - `user_email`
  - `user_provider_id`
  - `user_active`
  - `identity_source`
- Verification completed for this tightening step:
  - `go test ./bifrost-http/handlers -run 'TestUserGroupCompatRoute_'` in `transports`
  - `npx tsc --noEmit` in `ui`

## 2026-05-07 Business Units And Virtual Keys Tightening Checkpoint

- `GET /api/user-groups` is no longer just a raw `TableUserGroup` passthrough:
  - [transports/bifrost-http/handlers/user_groups.go](/home/evans/Projects/Public/bifrost/transports/bifrost-http/handlers/user_groups.go) now returns a compatibility response that preserves the original group identity fields while adding:
    - `member_count`
    - `external_member_count`
    - `local_member_count`
    - `inactive_member_count`
    - `provider_ids`
- Compatibility constraint preserved for downstream consumers:
  - `mcp-tool-groups` still only relies on `id`, `name`, and `description`, so enriching `/api/user-groups` was safe without endpoint splitting.
- The `business-units` page now exposes real directory-sync context instead of only raw group metadata:
  - [ui/app/workspace/governance/business-units/businessUnitsView.tsx](/home/evans/Projects/Public/bifrost/ui/app/workspace/governance/business-units/businessUnitsView.tsx)
- The `virtual-keys` ownership displays now consume the already-enriched assigned-user metadata:
  - [ui/app/workspace/virtual-keys/views/virtualKeySheet.tsx](/home/evans/Projects/Public/bifrost/ui/app/workspace/virtual-keys/views/virtualKeySheet.tsx)
  - [ui/app/workspace/virtual-keys/views/virtualKeyDetailsSheet.tsx](/home/evans/Projects/Public/bifrost/ui/app/workspace/virtual-keys/views/virtualKeyDetailsSheet.tsx)
- Verification completed for this tightening step:
  - `go test ./bifrost-http/handlers -run 'TestUserGroupCompatRoute_'` in `transports`
  - `go test ./bifrost-http/server -run '^$'` in `transports`
  - `npx tsc --noEmit` in `ui`

## 2026-05-07 Completion Audit Checkpoint

- Latest functional/build audit evidence on top of the governance-identity tightening:
  - `rg -n "@enterprise/components|ContactUsView" ui/app/workspace ui/components ui/lib` returned no matches in active UI code
  - `npx tsc --noEmit` in `ui` passed
  - `make build` passed and rebuilt `tmp/bifrost-http`
- Real remaining blocker is no longer a hidden feature gap:
  - `git branch --list 'oss*'` still returns only `oss/foundation`
  - the planned `oss/<module>` branch history has still not been materialized as real local branches or commits
- This means the goal is functionally close but still not complete under the documented success criteria until the user chooses a branch-restacking/commit approach consistent with the `oss/*` naming rule and the no-top-level-`oss` constraint.

## 2026-05-07 Restack Planning Checkpoint

- A concrete restack planning artifact now exists:
  - [gk-022-restack-options.md](/home/evans/Projects/Public/bifrost/.goalkeeper/goals/oss-enterprise-recovery/gk-022-restack-options.md)
- Durable planning conclusion:
  - the current worktree shape favors a coarse-grained `oss/*` restack more than a strict one-feature-per-branch replay
  - this recommendation is about operational safety, not a scope reduction in restored functionality
- Current recommendation if the user wants real `oss/*` history materialized:
  - `oss/foundation`
  - `oss/governance-identity`
  - `oss/runtime-policy`
  - `oss/platform`
  - `oss/ui-restoration`
- Still blocked on explicit user decision:
  - do not begin manufacturing branch history from the dirty worktree until the user picks strict restack, coarse restack, or a custom variant

## 2026-05-07 Restack Manifest Checkpoint

- A staging-ready manifest now exists for the recommended coarse-grained branch line:
  - [gk-023-restack-manifest.md](/home/evans/Projects/Public/bifrost/.goalkeeper/goals/oss-enterprise-recovery/gk-023-restack-manifest.md)
- Durable implementation guidance from this manifest:
  - exclude `.goalkeeper/**`, `.gitignore`, and `AGENTS.md` from feature-restack commits
  - prefer whole-file ownership for feature-local files
  - treat `transports/bifrost-http/server/server.go` and selected shared UI config/governance files as hunk-split files during the real restack
- Current state remains blocked on user approval, not on missing planning detail:
  - restack option is defined
  - file ownership is defined
  - the next missing input is user authorization to execute a specific restack variant

## 2026-05-07 Completion Checklist Checkpoint

- A final prompt-to-artifact checklist now exists:
  - [gk-024-completion-checklist.md](/home/evans/Projects/Public/bifrost/.goalkeeper/goals/oss-enterprise-recovery/gk-024-completion-checklist.md)
- Durable conclusion from that checklist:
  - restoration work, parity audit, build evidence, and prior run evidence are all already mapped to concrete artifacts
  - the only remaining unmet success criterion is real `oss/*` branch materialization
- This reduces remaining uncertainty substantially:
  - the blocker is now explicit user approval for restack execution, not ambiguity about what still counts as unfinished

## 2026-05-07 Restack Runbook Checkpoint

- A step-by-step execution playbook now exists for the recommended coarse-grained restack:
  - [gk-025-restack-runbook.md](/home/evans/Projects/Public/bifrost/.goalkeeper/goals/oss-enterprise-recovery/gk-025-restack-runbook.md)
- Durable conclusion:
  - planning for the remaining blocker is now complete enough that the next action can be actual restack execution, not more discovery
- The remaining blocker has narrowed again:
  - only explicit user authorization to execute the runbook is missing

## 2026-05-07 RBAC Foundation Checkpoint

- `GK-006` has started on `oss/foundation`.
- The first live code patch is in `framework/configstore`:
  - RBAC table models added for roles, permissions, role-permissions, and user-roles
  - RBAC migrations wired into configstore startup
  - `RDBConfigStore` gained RBAC CRUD and assignment/query methods
  - configstore tests were extended for migrations and RBAC persistence
- Verification status:
  - `go test ./configstore/...` passed in `/home/evans/Projects/Public/bifrost/framework`
- Next RBAC work should target:
  - current-main handler wiring
  - any auth/session integration needed for permission checks
  - the fallback-backed RBAC workspace contract so UI parity can eventually be checked against `GK-001`

## 2026-05-07 RBAC Page And Context Checkpoint

- The direct `governance/rbac` demo-only placeholder has been replaced with a real OSS page at:
  - [ui/app/workspace/governance/rbac/rbacView.tsx](/home/evans/Projects/Public/bifrost/ui/app/workspace/governance/rbac/rbacView.tsx)
- The OSS fallback `@enterprise/lib` RBAC layer is no longer a pure allow-all stub:
  - [ui/app/_fallbacks/enterprise/lib/contexts/rbacContext.tsx](/home/evans/Projects/Public/bifrost/ui/app/_fallbacks/enterprise/lib/contexts/rbacContext.tsx)
  - [ui/app/_fallbacks/enterprise/lib/store/apis/rbacApi.ts](/home/evans/Projects/Public/bifrost/ui/app/_fallbacks/enterprise/lib/store/apis/rbacApi.ts)
- Transport/server additions now provide:
  - default RBAC seeding including the stable `local-admin` subject
  - principal resolution for the existing built-in dashboard admin
  - `/api/rbac/context`, role CRUD, role-permission assignment, and user-role assignment endpoints
- Verification completed for this slice:
  - `go test ./configstore/...` in `framework`
  - `go test ./bifrost-http/handlers ./bifrost-http/integrations` in `transports`
  - `go test -run '^$' ./bifrost-http/lib ./bifrost-http/server ./bifrost-http/websocket` in `transports`
  - `npx tsc --noEmit` in `ui`
- Verification nuance:
  - full `npm run format:check` is currently noisy because the repository already contains many pre-existing formatting deltas outside the RBAC slice
  - the six RBAC-related UI files touched in this slice were individually normalized with `oxfmt`
- Remaining `GK-006` risk:
  - initial propagation needed to continue into the wider admin surface

## 2026-05-07 RBAC Propagation Checkpoint

- `GK-006` acceptance is now treated as satisfied.
- The OSS RBAC permission model now gates selected current-main routes in:
  - [transports/bifrost-http/handlers/governance.go](/home/evans/Projects/Public/bifrost/transports/bifrost-http/handlers/governance.go)
  - [transports/bifrost-http/handlers/providers.go](/home/evans/Projects/Public/bifrost/transports/bifrost-http/handlers/providers.go)
  - [transports/bifrost-http/handlers/config.go](/home/evans/Projects/Public/bifrost/transports/bifrost-http/handlers/config.go)
  - [transports/bifrost-http/handlers/logging.go](/home/evans/Projects/Public/bifrost/transports/bifrost-http/handlers/logging.go)
  - [transports/bifrost-http/handlers/plugins.go](/home/evans/Projects/Public/bifrost/transports/bifrost-http/handlers/plugins.go)
  - [transports/bifrost-http/handlers/mcp.go](/home/evans/Projects/Public/bifrost/transports/bifrost-http/handlers/mcp.go)
- Focused verification added:
  - [transports/bifrost-http/handlers/rbac_route_test.go](/home/evans/Projects/Public/bifrost/transports/bifrost-http/handlers/rbac_route_test.go)
  - This verifies both deny and allow behavior on registered provider/governance routes using the real middleware chain order.

## 2026-05-07 User-Groups Foundation Checkpoint

- `GK-007` has started on `oss/foundation`.
- Current-main storage groundwork added for user groups:
  - [framework/configstore/tables/user_groups.go](/home/evans/Projects/Public/bifrost/framework/configstore/tables/user_groups.go)
  - [framework/configstore/user_groups.go](/home/evans/Projects/Public/bifrost/framework/configstore/user_groups.go)
  - [framework/configstore/user_groups_test.go](/home/evans/Projects/Public/bifrost/framework/configstore/user_groups_test.go)
- Migration and interface wiring added:
  - `migrationAddUserGroupTables` in [framework/configstore/migrations.go](/home/evans/Projects/Public/bifrost/framework/configstore/migrations.go)
  - `ConfigStore` interface additions in [framework/configstore/store.go](/home/evans/Projects/Public/bifrost/framework/configstore/store.go)
- Durable conclusion for the next slice:
  - the most immediate `GK-007` leverage is not the full demo-only users/business-units pages yet, but the shared enterprise stub APIs already consumed by virtual-key screens: access-profile lookup and virtual-key user attachment data

## 2026-05-07 User-Groups Compatibility Checkpoint

- `UserGroupHandler` is now fully registered in [transports/bifrost-http/server/server.go](/home/evans/Projects/Public/bifrost/transports/bifrost-http/server/server.go) alongside the existing RBAC bootstrap; the first transport verification for this slice passed.
- New OSS compatibility endpoints exist in [transports/bifrost-http/handlers/user_groups.go](/home/evans/Projects/Public/bifrost/transports/bifrost-http/handlers/user_groups.go):
  - `GET /api/virtual-keys/{vk_id}/users`
  - `GET /api/users/{user_id}/access-profiles`
  - `GET /api/access-profiles`
- These endpoints intentionally synthesize access-profile shaped responses from `user_groups` plus linked virtual keys, rather than introducing a separate enterprise-only access-profile table yet.
- The fallback `@enterprise/lib` APIs for virtual-key user attachments and per-user access-profile lookups are no longer no-ops:
  - [ui/app/_fallbacks/enterprise/lib/store/apis/virtualKeyUsersApi.ts](/home/evans/Projects/Public/bifrost/ui/app/_fallbacks/enterprise/lib/store/apis/virtualKeyUsersApi.ts)
  - [ui/app/_fallbacks/enterprise/lib/store/apis/accessProfileApi.ts](/home/evans/Projects/Public/bifrost/ui/app/_fallbacks/enterprise/lib/store/apis/accessProfileApi.ts)
- Direct fallback parity improved for `governance/access-profiles`:
  - [ui/app/workspace/governance/access-profiles/page.tsx](/home/evans/Projects/Public/bifrost/ui/app/workspace/governance/access-profiles/page.tsx) now renders a real OSS page
  - [ui/app/workspace/governance/access-profiles/accessProfilesView.tsx](/home/evans/Projects/Public/bifrost/ui/app/workspace/governance/access-profiles/accessProfilesView.tsx) lists synthetic profiles instead of the Book a demo CTA

## 2026-05-07 Vault Checkpoint

- `GK-016` is complete at a narrowed OSS-safe scope:
  - KV secret resolution only
  - supported auth methods: `token`, `approle`
  - explicitly deferred from this slice: transit encryption, dynamic credentials, Kubernetes auth, rotation workflows
- New Vault storage and runtime pieces now exist at:
  - [framework/vault/config.go](/home/evans/Projects/Public/bifrost/framework/vault/config.go)
  - [framework/vault/client.go](/home/evans/Projects/Public/bifrost/framework/vault/client.go)
  - [framework/vault/manager.go](/home/evans/Projects/Public/bifrost/framework/vault/manager.go)
  - [framework/configstore/tables/vault.go](/home/evans/Projects/Public/bifrost/framework/configstore/tables/vault.go)
  - [framework/configstore/vault.go](/home/evans/Projects/Public/bifrost/framework/configstore/vault.go)
  - [transports/bifrost-http/handlers/vault.go](/home/evans/Projects/Public/bifrost/transports/bifrost-http/handlers/vault.go)
- `core/schemas.EnvVar` and `framework/envutils` no longer treat only `env.*` as indirect secret sources; `vault://...` now resolves through a pluggable resolver installed by the Vault manager.
- `transports/bifrost-http/lib/config.go` now activates Vault before client/provider/auth config loading so DB-backed config scans can resolve `vault://...` references during startup and sync.
- The Security workspace and provider key inputs now expose the restored OSS contract instead of demo-only behavior:
  - [ui/app/workspace/config/views/securityView.tsx](/home/evans/Projects/Public/bifrost/ui/app/workspace/config/views/securityView.tsx)
  - [ui/components/ui/envVarInput.tsx](/home/evans/Projects/Public/bifrost/ui/components/ui/envVarInput.tsx)
  - [ui/app/workspace/providers/fragments/apiKeysFormFragment.tsx](/home/evans/Projects/Public/bifrost/ui/app/workspace/providers/fragments/apiKeysFormFragment.tsx)
- Durable bug fix:
  - Vault handler HTTP reachability checks must not pass `*fasthttp.RequestCtx` into the standard library `http.Client`; this caused a nil dereference during tests and is now fixed by using standard timeout contexts for Vault auth/health/sync calls.
- Focused verification added for this slice:
  - [framework/vault/manager_test.go](/home/evans/Projects/Public/bifrost/framework/vault/manager_test.go)
  - [framework/configstore/vault_test.go](/home/evans/Projects/Public/bifrost/framework/configstore/vault_test.go)
  - [transports/bifrost-http/lib/config_vault_test.go](/home/evans/Projects/Public/bifrost/transports/bifrost-http/lib/config_vault_test.go)
  - [transports/bifrost-http/handlers/vault_test.go](/home/evans/Projects/Public/bifrost/transports/bifrost-http/handlers/vault_test.go)
- Direct fallback parity also improved for:
  - [ui/app/workspace/governance/users/page.tsx](/home/evans/Projects/Public/bifrost/ui/app/workspace/governance/users/page.tsx), now backed by [ui/app/workspace/governance/users/usersView.tsx](/home/evans/Projects/Public/bifrost/ui/app/workspace/governance/users/usersView.tsx)
  - [ui/app/workspace/governance/business-units/page.tsx](/home/evans/Projects/Public/bifrost/ui/app/workspace/governance/business-units/page.tsx), now backed by [ui/app/workspace/governance/business-units/businessUnitsView.tsx](/home/evans/Projects/Public/bifrost/ui/app/workspace/governance/business-units/businessUnitsView.tsx)
- The compatibility layer now exposes a synthetic users list via `GET /api/users` in addition to the earlier access-profile endpoints, and the fallback store gained:
  - [ui/app/_fallbacks/enterprise/lib/store/apis/userGovernanceApi.ts](/home/evans/Projects/Public/bifrost/ui/app/_fallbacks/enterprise/lib/store/apis/userGovernanceApi.ts)
- Current interpretation of `GK-007` surface mapping:
  - `business-units` currently maps to `user_groups` records directly
  - `users` currently maps to distinct member identities aggregated across `user_groups`
  - `access-profiles` currently maps to synthetic per-user per-virtual-key views derived from `user_groups` plus linked virtual keys
- `GK-007` acceptance is now treated as satisfied at the current OSS compatibility scope:
  - user-group storage and handler primitives exist on current main
  - virtual-key governance screens can resolve assigned users and synthetic access-profile ownership
  - direct governance placeholder pages for `users`, `business-units`, and `access-profiles` no longer render Book a demo CTAs
- `useVirtualKeyUsage` now treats the first assigned user as a compatibility lookup seed because group-backed synthetic profiles are identical for each assignee of the same managed virtual key.
- Verification completed for this checkpoint:
  - `go test ./bifrost-http/handlers ./bifrost-http/server ./bifrost-http/integrations` in `transports`
  - `go test -run '^$' ./bifrost-http/lib` in `transports`
  - `npx tsc --noEmit` in `ui`

## 2026-05-07 SCIM And SSO Checkpoint

- `GK-008` is now treated as satisfied at the current OSS compatibility scope.
- Current-main storage and migrations added for:
  - [framework/configstore/tables/sso.go](/home/evans/Projects/Public/bifrost/framework/configstore/tables/sso.go)
  - [framework/configstore/sso.go](/home/evans/Projects/Public/bifrost/framework/configstore/sso.go)
- Current-main transport slice added:
  - [transports/bifrost-http/handlers/sso_handler.go](/home/evans/Projects/Public/bifrost/transports/bifrost-http/handlers/sso_handler.go)
  - registered in [transports/bifrost-http/server/server.go](/home/evans/Projects/Public/bifrost/transports/bifrost-http/server/server.go)
- Practical OSS SSO/SCIM contract now includes:
  - `GET /api/scim/auth-type`
  - `GET/POST/PUT/DELETE /api/sso/providers`
  - `GET /api/sso/users`
  - `GET /api/sso/users/{user_id}`
  - `POST /api/sso/users/{user_id}/activate`
  - `POST /api/sso/users/{user_id}/deactivate`
- Shared frontend auth-type handling no longer relies on a stub or on `IS_ENTERPRISE` as the runtime truth:
  - [ui/app/_fallbacks/enterprise/lib/store/apis/scimApi.ts](/home/evans/Projects/Public/bifrost/ui/app/_fallbacks/enterprise/lib/store/apis/scimApi.ts) now provides real RTK Query endpoints
  - [ui/app/workspace/config/views/securityView.tsx](/home/evans/Projects/Public/bifrost/ui/app/workspace/config/views/securityView.tsx) now queries auth type directly and hides password settings when live SSO providers exist
- Direct fallback parity improved for `scim`:
  - [ui/app/workspace/scim/page.tsx](/home/evans/Projects/Public/bifrost/ui/app/workspace/scim/page.tsx) now renders [ui/app/workspace/scim/scimView.tsx](/home/evans/Projects/Public/bifrost/ui/app/workspace/scim/scimView.tsx) instead of the Book a demo CTA

## 2026-05-07 MCP Tool Groups Checkpoint

- `GK-013` is now treated as satisfied at the current OSS compatibility scope.
- Current-main storage and migrations added for MCP tool groups:
  - [framework/configstore/tables/mcp_groups.go](/home/evans/Projects/Public/bifrost/framework/configstore/tables/mcp_groups.go)
  - [framework/configstore/mcp_groups.go](/home/evans/Projects/Public/bifrost/framework/configstore/mcp_groups.go)
- Durable storage/runtime decision:
  - do not restore the PR's direct `virtual_key_mcp_groups` enterprise linkage
  - attach MCP tool groups to existing `user_groups`, then resolve them indirectly for a virtual key through `user_group_virtual_keys` plus `user_group_mcp_groups`
- Practical OSS MCP tool groups contract now includes:
  - `GET/POST /api/mcp-tool-groups`
  - `GET/PUT/DELETE /api/mcp-tool-groups/{id}`
  - `POST /api/mcp-tool-groups/{id}/members`
  - `DELETE /api/mcp-tool-groups/{id}/members/{member_id}`
  - `GET/PUT /api/mcp-tool-groups/{id}/user-groups`
- Runtime MCP filtering now incorporates inherited group attachments in both paths:
  - governance auto-injected `x-bf-mcp-include-tools` headers in [plugins/governance/main.go](/home/evans/Projects/Public/bifrost/plugins/governance/main.go)
  - MCP gateway server-side `tools/list` and execution filtering in [transports/bifrost-http/handlers/mcpserver.go](/home/evans/Projects/Public/bifrost/transports/bifrost-http/handlers/mcpserver.go)
- Direct fallback parity improved for `mcp-tool-groups`:
  - [ui/app/workspace/mcp-tool-groups/page.tsx](/home/evans/Projects/Public/bifrost/ui/app/workspace/mcp-tool-groups/page.tsx) now renders [ui/app/workspace/mcp-tool-groups/mcpToolGroupsView.tsx](/home/evans/Projects/Public/bifrost/ui/app/workspace/mcp-tool-groups/mcpToolGroupsView.tsx) instead of the Book a demo CTA
  - [ui/app/_fallbacks/enterprise/lib/store/apis/mcpToolGroupsApi.ts](/home/evans/Projects/Public/bifrost/ui/app/_fallbacks/enterprise/lib/store/apis/mcpToolGroupsApi.ts) provides the restored OSS RTK Query slice
- Verification completed for this checkpoint:
  - `go test ./configstore/...` in `framework`
  - `go test ./...` in `plugins/governance`
  - `go test ./bifrost-http/handlers ./bifrost-http/server ./bifrost-http/integrations` in `transports`
  - `npx tsc --noEmit` in `ui`

## 2026-05-07 Alert Channels And Connectors Checkpoint

- `GK-014` is now treated as satisfied at a practical OSS-first scope.
- Current-main storage and migrations added for alerting/connectors:
  - [framework/configstore/tables/alerting.go](/home/evans/Projects/Public/bifrost/framework/configstore/tables/alerting.go)
  - [framework/configstore/tables/connectors.go](/home/evans/Projects/Public/bifrost/framework/configstore/tables/connectors.go)
  - [framework/configstore/alerting.go](/home/evans/Projects/Public/bifrost/framework/configstore/alerting.go)
  - [framework/configstore/connectors.go](/home/evans/Projects/Public/bifrost/framework/configstore/connectors.go)
- Durable scope decision for this slice:
  - restore persisted alert channels plus a stable trigger-source catalog first
  - restore persisted BigQuery / Datadog connector config management first
  - do not claim a full alert-rule execution engine or live connector export pipeline yet
- Practical OSS contracts now include:
  - `GET/POST /api/alert-channels/channels`
  - `GET/PUT/DELETE /api/alert-channels/channels/{id}`
  - `GET /api/alert-channels/trigger-sources`
  - `GET/POST /api/connectors`
  - `GET/PUT/DELETE /api/connectors/{id}`
  - `POST /api/connectors/{id}/test`
- Direct fallback parity improved for the remaining demo-only observability surfaces:
  - [ui/app/workspace/alert-channels/page.tsx](/home/evans/Projects/Public/bifrost/ui/app/workspace/alert-channels/page.tsx) now renders [ui/app/workspace/alert-channels/alertChannelsView.tsx](/home/evans/Projects/Public/bifrost/ui/app/workspace/alert-channels/alertChannelsView.tsx)
  - [ui/app/workspace/observability/views/plugins/bigqueryView.tsx](/home/evans/Projects/Public/bifrost/ui/app/workspace/observability/views/plugins/bigqueryView.tsx) now renders the real [ui/app/workspace/observability/views/plugins/connectorConfigView.tsx](/home/evans/Projects/Public/bifrost/ui/app/workspace/observability/views/plugins/connectorConfigView.tsx) for `bigquery`
  - [ui/app/workspace/observability/views/plugins/datadogView.tsx](/home/evans/Projects/Public/bifrost/ui/app/workspace/observability/views/plugins/datadogView.tsx) now renders the same real connector view for `datadog`
- Restored fallback store APIs for this slice:
  - [ui/app/_fallbacks/enterprise/lib/store/apis/alertChannelsApi.ts](/home/evans/Projects/Public/bifrost/ui/app/_fallbacks/enterprise/lib/store/apis/alertChannelsApi.ts)
  - [ui/app/_fallbacks/enterprise/lib/store/apis/connectorsApi.ts](/home/evans/Projects/Public/bifrost/ui/app/_fallbacks/enterprise/lib/store/apis/connectorsApi.ts)
- Verification completed for this checkpoint:
  - `go test ./configstore/...` in `framework`
  - `go test ./bifrost-http/handlers ./bifrost-http/server ./bifrost-http/integrations` in `transports`
  - `npx tsc --noEmit` in `ui`

## 2026-05-07 Cluster Checkpoint

- `GK-015` is now treated as satisfied at a practical OSS-safe single-node scope.
- Current-main cluster package added:
  - [framework/cluster/cluster.go](/home/evans/Projects/Public/bifrost/framework/cluster/cluster.go)
- Durable scope decision for clustering:
  - expose a stable cluster status contract now
  - explicitly narrow the current OSS runtime to single-node health plus local invalidation strategy
  - do not claim Redis-backed multi-node gossip / pubsub is active yet
- Practical OSS cluster contract now includes:
  - `GET /api/cluster/status`
  - `POST /api/cluster/drain`
- Runtime/server wiring:
  - [transports/bifrost-http/handlers/cluster.go](/home/evans/Projects/Public/bifrost/transports/bifrost-http/handlers/cluster.go)
  - [transports/bifrost-http/server/server.go](/home/evans/Projects/Public/bifrost/transports/bifrost-http/server/server.go) now instantiates a local cluster manager with node id, address, version, and KV-store availability
- Direct fallback parity improved for `cluster`:
  - [ui/app/workspace/cluster/page.tsx](/home/evans/Projects/Public/bifrost/ui/app/workspace/cluster/page.tsx) now renders [ui/app/workspace/cluster/clusterView.tsx](/home/evans/Projects/Public/bifrost/ui/app/workspace/cluster/clusterView.tsx)
  - [ui/app/_fallbacks/enterprise/lib/store/apis/clusterApi.ts](/home/evans/Projects/Public/bifrost/ui/app/_fallbacks/enterprise/lib/store/apis/clusterApi.ts) replaces the earlier `clusterApi = null` stub
- Verification completed for this checkpoint:
  - `go test ./cluster/...` in `framework`
  - `go test ./bifrost-http/handlers ./bifrost-http/server ./bifrost-http/integrations` in `transports`
  - `npx tsc --noEmit` in `ui`

## 2026-05-07 Adaptive Routing Checkpoint

- `GK-012` is now treated as satisfied at the current OSS-safe first-pass scope.
- Current-main storage and migrations added for:
  - [framework/configstore/tables/adaptive_routing.go](/home/evans/Projects/Public/bifrost/framework/configstore/tables/adaptive_routing.go)
  - [framework/configstore/adaptive_routing.go](/home/evans/Projects/Public/bifrost/framework/configstore/adaptive_routing.go)
- Current-main transport slice added:
  - [transports/bifrost-http/handlers/adaptive_routing.go](/home/evans/Projects/Public/bifrost/transports/bifrost-http/handlers/adaptive_routing.go)
  - registered in [transports/bifrost-http/server/server.go](/home/evans/Projects/Public/bifrost/transports/bifrost-http/server/server.go)
- Practical OSS adaptive-routing contract now includes:
  - `GET/POST/PUT/DELETE /api/adaptive-routing/policies`
  - `GET /api/adaptive-routing/metrics`
  - `POST /api/adaptive-routing/metrics/refresh`
  - `GET/PUT/DELETE /api/adaptive-routing/quality-scores`
- Runtime integration is intentionally narrowed:
  - governance load balancing now consults the most specific enabled adaptive-routing policy (`virtual_key` first, then `global`) before falling back to the existing weighted selection path
  - runtime uses stored metric snapshots, preferring the 60-minute window and then falling back to 5-minute and 24-hour windows
  - supported runtime strategies in this first pass are `latency_optimized`, `cost_optimized`, `quality_optimized`, `availability_optimized`, and `balanced`
  - `canary`, `geo_affinity`, and dedicated live streaming metrics collectors from the older PR/spec are explicitly deferred
- Direct fallback parity improved for `adaptive-routing`:
  - [ui/app/workspace/adaptive-routing/page.tsx](/home/evans/Projects/Public/bifrost/ui/app/workspace/adaptive-routing/page.tsx) now renders [ui/app/workspace/adaptive-routing/adaptiveRoutingView.tsx](/home/evans/Projects/Public/bifrost/ui/app/workspace/adaptive-routing/adaptiveRoutingView.tsx) instead of the Book a demo CTA
- The adaptive-routing page now provides:
  - real policy CRUD
  - manual model quality score CRUD
  - log-derived provider/model metrics refresh and inspection
- Verification completed for this checkpoint:
  - `go test ./configstore/...` in `framework`
  - `go test ./...` in `plugins/governance`
  - `go test ./bifrost-http/handlers ./bifrost-http/server ./bifrost-http/integrations` in `transports`
  - `npx tsc --noEmit` in `ui`
  - the replacement page can create/delete SSO providers and inspect current auth mode / synced external-user count
- Verification completed for this checkpoint:
  - `go test ./configstore/...` in `framework`
  - `go test ./bifrost-http/handlers ./bifrost-http/server ./bifrost-http/integrations` in `transports`
  - `go test -run '^$' ./bifrost-http/lib` in `transports`
  - `npx tsc --noEmit` in `ui`

## 2026-05-07 Large Payload Checkpoint

- `GK-017` is now treated as satisfied at a narrowed OSS-safe first-pass scope.
- Current-main large-payload restoration added:
  - [core/schemas/large_payload_config.go](/home/evans/Projects/Public/bifrost/core/schemas/large_payload_config.go)
  - [framework/config.go](/home/evans/Projects/Public/bifrost/framework/config.go)
  - [framework/configstore/tables/framework.go](/home/evans/Projects/Public/bifrost/framework/configstore/tables/framework.go)
  - [transports/bifrost-http/lib/config.go](/home/evans/Projects/Public/bifrost/transports/bifrost-http/lib/config.go)
  - [transports/bifrost-http/handlers/payload.go](/home/evans/Projects/Public/bifrost/transports/bifrost-http/handlers/payload.go)
  - [transports/bifrost-http/handlers/middlewares.go](/home/evans/Projects/Public/bifrost/transports/bifrost-http/handlers/middlewares.go)
- Practical OSS contract now includes:
  - `GET /api/payload/config`
  - `PUT /api/payload/config`
  - live runtime application of `request_threshold_bytes`, `response_threshold_bytes`, `prefetch_size_bytes`, `max_payload_bytes`, and `truncated_log_bytes`
  - known-size request rejection with `413` when `Content-Length` exceeds `max_payload_bytes`
  - response preview truncation control for large-response logging / usage extraction
- Direct fallback parity improved for `config/client-settings`:
  - [ui/app/_fallbacks/enterprise/components/large-payload/largePayloadSettingsFragment.tsx](/home/evans/Projects/Public/bifrost/ui/app/_fallbacks/enterprise/components/large-payload/largePayloadSettingsFragment.tsx) is no longer a null fragment
  - [ui/app/_fallbacks/enterprise/lib/store/apis/largePayloadApi.ts](/home/evans/Projects/Public/bifrost/ui/app/_fallbacks/enterprise/lib/store/apis/largePayloadApi.ts) is no longer a no-op
  - [ui/app/workspace/config/views/clientSettingsView.tsx](/home/evans/Projects/Public/bifrost/ui/app/workspace/config/views/clientSettingsView.tsx) now saves and reports live OSS large-payload settings rather than restart-only demo behavior
- Durable narrowing for later audit:
  - direct `/v1/*` inference handlers still parse request JSON eagerly, so request-side large-payload passthrough is not yet generalized there
  - current `GK-017` scope intentionally restores the management surface plus threshold-driven runtime hooks already present on current main, rather than porting the stale `pr-2565` payload handler
- Verification completed for this checkpoint:
  - `go test ./schemas` in `core`
  - `go test ./... -run '^$'` in `core`
  - `go test ./configstore/...` in `framework`
  - `go test ./bifrost-http/lib ./bifrost-http/handlers ./bifrost-http/server ./bifrost-http/integrations` in `transports`
  - `npx tsc --noEmit` in `ui`

## 2026-05-07 Frontend Restoration Checkpoint

- `GK-018` has started.
- Direct placeholder parity improved for `mcp-auth-config`:
  - [ui/app/workspace/mcp-auth-config/page.tsx](/home/evans/Projects/Public/bifrost/ui/app/workspace/mcp-auth-config/page.tsx) no longer imports the fallback `ContactUsView` wrapper
  - new live page at [ui/app/workspace/mcp-auth-config/mcpAuthConfigView.tsx](/home/evans/Projects/Public/bifrost/ui/app/workspace/mcp-auth-config/mcpAuthConfigView.tsx) reads real MCP client data and summarizes auth-enabled clients by auth mode
- Durable scoping decision:
  - this page is restored as a shared-page integration over existing MCP Registry + OAuth endpoints, not as a separate enterprise-only auth management backend
  - the running `GK-018` shortlist after this slice moved next to `dashboard` user rankings, `config/api-keys` scoped-key CTA, and the prompt-deployments sidebar CTA
- Verification completed for this checkpoint:
  - `npx tsc --noEmit` in `ui`

- Direct placeholder parity improved for `dashboard` user rankings:
  - [ui/app/workspace/dashboard/page.tsx](/home/evans/Projects/Public/bifrost/ui/app/workspace/dashboard/page.tsx) no longer imports `@enterprise/components/user-rankings/userRankingsTab`
  - new live tab at [ui/app/workspace/dashboard/components/userRankingsTab.tsx](/home/evans/Projects/Public/bifrost/ui/app/workspace/dashboard/components/userRankingsTab.tsx) renders sortable user rankings, request/cost trends, and top-spender summaries
  - [ui/lib/store/apis/logsApi.ts](/home/evans/Projects/Public/bifrost/ui/lib/store/apis/logsApi.ts) now exposes `/logs/user-rankings`
  - [transports/bifrost-http/handlers/logging.go](/home/evans/Projects/Public/bifrost/transports/bifrost-http/handlers/logging.go) now exposes `GET /api/logs/user-rankings` over the existing logstore aggregation
- Durable scoping decision:
  - this page is restored as a shared dashboard integration over already-present `framework/logstore.GetUserRankings` data, not as a separate enterprise-only analytics subsystem
  - the remaining high-signal `GK-018` shortlist is now `config/api-keys` scoped-key CTA and the prompt-deployments sidebar CTA
- Verification completed for this checkpoint:
  - `go test ./bifrost-http/handlers ./bifrost-http/server ./bifrost-http/integrations` in `transports`
  - `npx tsc --noEmit` in `ui`

- Direct placeholder parity improved for `config/api-keys`:
  - [ui/app/workspace/config/api-keys/page.tsx](/home/evans/Projects/Public/bifrost/ui/app/workspace/config/api-keys/page.tsx) no longer imports the enterprise fallback API keys view
  - new live page at [ui/app/workspace/config/api-keys/apiKeysView.tsx](/home/evans/Projects/Public/bifrost/ui/app/workspace/config/api-keys/apiKeysView.tsx) summarizes admin auth state and repurposes existing Virtual Keys as the OSS-compatible scoped key management surface
- `config/api-keys` parity improved again after `GK-021`:
  - persistent admin API key model added at [framework/configstore/tables/admin_api_key.go](/home/evans/Projects/Public/bifrost/framework/configstore/tables/admin_api_key.go) with CRUD in [framework/configstore/admin_api_keys.go](/home/evans/Projects/Public/bifrost/framework/configstore/admin_api_keys.go)
  - OSS transport routes added at [transports/bifrost-http/handlers/admin_api_keys.go](/home/evans/Projects/Public/bifrost/transports/bifrost-http/handlers/admin_api_keys.go) and registered in [transports/bifrost-http/server/server.go](/home/evans/Projects/Public/bifrost/transports/bifrost-http/server/server.go)
  - admin Bearer token auth now resolves through [transports/bifrost-http/handlers/middlewares.go](/home/evans/Projects/Public/bifrost/transports/bifrost-http/handlers/middlewares.go) before session fallback
  - active UI now uses [ui/lib/store/apis/adminApiKeysApi.ts](/home/evans/Projects/Public/bifrost/ui/lib/store/apis/adminApiKeysApi.ts) and a rewritten [ui/app/workspace/config/api-keys/apiKeysView.tsx](/home/evans/Projects/Public/bifrost/ui/app/workspace/config/api-keys/apiKeysView.tsx) to create, toggle, delete, and copy Admin API Keys
  - browser login can now use Admin API Keys through [transports/bifrost-http/handlers/session.go](/home/evans/Projects/Public/bifrost/transports/bifrost-http/handlers/session.go), [ui/lib/store/apis/baseApi.ts](/home/evans/Projects/Public/bifrost/ui/lib/store/apis/baseApi.ts), [ui/lib/store/apis/sessionApi.ts](/home/evans/Projects/Public/bifrost/ui/lib/store/apis/sessionApi.ts), and [ui/app/login/loginView.tsx](/home/evans/Projects/Public/bifrost/ui/app/login/loginView.tsx)
- Branch-structure fact check after the post-`GK-021` work:
  - [oss/foundation] is still the only surviving local `oss/*` branch
  - the planned per-module refs such as `oss/adaptive-routing` and `oss/user` do not exist in git yet
  - because the restoration work remains mostly uncommitted in the dirty worktree, satisfying the original branch-topology success criterion now requires an explicit commit/branch-restacking decision rather than more feature coding alone
- Direct placeholder parity improved for prompt deployments:
  - [ui/components/prompts/fragments/settingsPanel.tsx](/home/evans/Projects/Public/bifrost/ui/components/prompts/fragments/settingsPanel.tsx) no longer imports the fallback deployments accordion item
  - new live sidebar fragment at [ui/components/prompts/fragments/promptDeploymentsAccordionItem.tsx](/home/evans/Projects/Public/bifrost/ui/components/prompts/fragments/promptDeploymentsAccordionItem.tsx) exposes prompt version/session summaries and commit actions over existing prompt-repo APIs
- `GK-018` acceptance is now treated as satisfied at the current OSS compatibility scope:
  - direct and hybrid demo-only workspace surfaces identified in `GK-001` no longer resolve to `ContactUsView` placeholders in active workspace screens
  - a parity sweep now finds only two remaining `@enterprise/components` imports under active workspace screens: `LargePayloadSettingsFragment` and `TeamsView`, both of which were already converted into functional fallback integrations rather than paywall CTAs
  - remaining OSS restoration work has shifted from placeholder pages to shared `@enterprise/lib` helpers and context behavior under `GK-019`
- Verification completed for this checkpoint:
  - `npx tsc --noEmit` in `ui`

## 2026-05-07 Shared Store Checkpoint

- `GK-019` is now treated as satisfied at the current OSS compatibility scope.
- Restored fallback APIs are now first-class exports from [ui/lib/store/apis/index.ts](/home/evans/Projects/Public/bifrost/ui/lib/store/apis/index.ts), and the store boot path now injects them through [ui/lib/store/store.ts](/home/evans/Projects/Public/bifrost/ui/lib/store/store.ts) rather than relying on the enterprise API barrel import.
- Active workspace consumers that previously imported restored hooks directly from `@enterprise/lib` or `@enterprise/lib/store/apis/*` now point at the main store entrypoints, including:
  - audit logs, SCIM, security, client settings, cluster, alert channels, adaptive routing, connectors
  - virtual-key usage helpers, access profiles, users, business units, MCP tool groups
  - guardrails, PII redaction, and RBAC management
- Current residual non-component alias state after this sweep:
  - [ui/app/workspace/governance/rbac/rbacView.tsx](/home/evans/Projects/Public/bifrost/ui/app/workspace/governance/rbac/rbacView.tsx) still imports RBAC enums/context from `@enterprise/lib`, but its data hooks now resolve through the main store
  - [ui/lib/store/apis/baseApi.ts](/home/evans/Projects/Public/bifrost/ui/lib/store/apis/baseApi.ts) still imports the shared `createBaseQueryWithRefresh` compatibility wrapper from the fallback enterprise tree
- Interpretation:
  - the `GK-001` shared-stub items for access profiles, virtual-key users, SCIM auth type, and large-payload config are no longer no-op hooks in active workspace usage
  - RBAC behavior is already live-backed through the restored `/api/rbac/*` endpoints and context query flow; the remaining alias references are now structural compatibility imports rather than placeholder behavior
- Verification completed for this checkpoint:
  - `npx tsc --noEmit` in `ui`

## 2026-05-07 Audit Logs Checkpoint

- `GK-009` is now treated as satisfied at the current OSS compatibility scope.
- Current-main audit persistence added:
  - [framework/configstore/tables/audit.go](/home/evans/Projects/Public/bifrost/framework/configstore/tables/audit.go)
  - [framework/configstore/audit.go](/home/evans/Projects/Public/bifrost/framework/configstore/audit.go)
- Current-main transport slice added:
  - [transports/bifrost-http/handlers/audit_handler.go](/home/evans/Projects/Public/bifrost/transports/bifrost-http/handlers/audit_handler.go)
  - registered in [transports/bifrost-http/server/server.go](/home/evans/Projects/Public/bifrost/transports/bifrost-http/server/server.go)
- Practical OSS audit contract now includes:
  - `GET /api/audit/logs`
  - `POST /api/audit/verify`
- The audit chain implementation was tightened relative to the PR salvage by explicitly assigning `Seq` inside the append transaction before hashing, so the chain covers the actual stored sequence number.
- SSO provider mutations now emit audit entries through [transports/bifrost-http/handlers/sso_handler.go](/home/evans/Projects/Public/bifrost/transports/bifrost-http/handlers/sso_handler.go), which gives the restored audit page a real write path immediately.
- Direct fallback parity improved for `audit-logs`:
  - [ui/app/workspace/audit-logs/page.tsx](/home/evans/Projects/Public/bifrost/ui/app/workspace/audit-logs/page.tsx) now renders [ui/app/workspace/audit-logs/auditLogsView.tsx](/home/evans/Projects/Public/bifrost/ui/app/workspace/audit-logs/auditLogsView.tsx) instead of the Book a demo CTA
  - [ui/app/_fallbacks/enterprise/lib/store/apis/auditLogsApi.ts](/home/evans/Projects/Public/bifrost/ui/app/_fallbacks/enterprise/lib/store/apis/auditLogsApi.ts) now provides real query and verify endpoints
- Verification completed for this checkpoint:
  - `go test ./configstore/...` in `framework`
  - `go test ./bifrost-http/handlers ./bifrost-http/server ./bifrost-http/integrations` in `transports`
  - `go test -run '^$' ./bifrost-http/lib` in `transports`
  - `npx tsc --noEmit` in `ui`

## 2026-05-07 Guardrails Checkpoint

- `GK-010` is now treated as satisfied at a narrowed OSS-safe first-pass scope.
- Current-main guardrails persistence and migrations added:
  - [framework/configstore/tables/guardrails.go](/home/evans/Projects/Public/bifrost/framework/configstore/tables/guardrails.go)
  - [framework/configstore/guardrails.go](/home/evans/Projects/Public/bifrost/framework/configstore/guardrails.go)
- Current-main transport slice added:
  - [transports/bifrost-http/handlers/guardrails.go](/home/evans/Projects/Public/bifrost/transports/bifrost-http/handlers/guardrails.go)
  - [transports/bifrost-http/handlers/guardrails_middleware.go](/home/evans/Projects/Public/bifrost/transports/bifrost-http/handlers/guardrails_middleware.go)
  - registered in [transports/bifrost-http/server/server.go](/home/evans/Projects/Public/bifrost/transports/bifrost-http/server/server.go)
- Practical OSS guardrails contract now includes:
  - `GET/POST/PUT/DELETE /api/guardrails/policies`
  - `GET/POST /api/guardrails/policies/{id}/rules`
  - `DELETE /api/guardrails/rules/{ruleId}`
  - `GET /api/guardrails/violations`
  - `POST /api/guardrails/test`
  - input-side runtime enforcement for `TextCompletionRequest`, `ChatCompletionRequest`, and `ResponsesRequest`
- Runtime scope is intentionally narrower than the enterprise product:
  - local keyword and regex rules only
  - input-side enforcement only
  - policy scope support is effectively `global`, plus direct virtual-key string matching when a scoped `scope_id` is explicitly set
  - provider-native managed guardrail profiles and output-side interception remain follow-up work rather than hidden demo-only pages
- Direct fallback parity improved for `guardrails`:
  - [ui/app/workspace/guardrails/page.tsx](/home/evans/Projects/Public/bifrost/ui/app/workspace/guardrails/page.tsx) now chooses between local OSS configuration/providers views instead of importing the Book a demo fallback
  - [ui/app/workspace/guardrails/configuration/page.tsx](/home/evans/Projects/Public/bifrost/ui/app/workspace/guardrails/configuration/page.tsx) now renders [ui/app/workspace/guardrails/configuration/guardrailsConfigurationView.tsx](/home/evans/Projects/Public/bifrost/ui/app/workspace/guardrails/configuration/guardrailsConfigurationView.tsx)
  - [ui/app/workspace/guardrails/providers/page.tsx](/home/evans/Projects/Public/bifrost/ui/app/workspace/guardrails/providers/page.tsx) now renders [ui/app/workspace/guardrails/providers/guardrailsProviderView.tsx](/home/evans/Projects/Public/bifrost/ui/app/workspace/guardrails/providers/guardrailsProviderView.tsx)
- The fallback `@enterprise/lib` API for guardrails is no longer a stub:
  - [ui/app/_fallbacks/enterprise/lib/store/apis/guardrailsApi.ts](/home/evans/Projects/Public/bifrost/ui/app/_fallbacks/enterprise/lib/store/apis/guardrailsApi.ts)
- Verification completed for this checkpoint:
  - `go test ./configstore/...` in `framework`
  - `go test ./bifrost-http/handlers ./bifrost-http/server ./bifrost-http/integrations` in `transports`
  - `go test -run '^$' ./bifrost-http/lib` in `transports`
  - `npx tsc --noEmit` in `ui`

## 2026-05-07 PII Redactor Checkpoint

- `GK-011` is now treated as satisfied at a narrowed OSS-safe first-pass scope.
- Current-main PII persistence and migrations added:
  - [framework/configstore/tables/pii.go](/home/evans/Projects/Public/bifrost/framework/configstore/tables/pii.go)
  - [framework/configstore/pii.go](/home/evans/Projects/Public/bifrost/framework/configstore/pii.go)
- Current-main transport slice added:
  - [transports/bifrost-http/handlers/pii.go](/home/evans/Projects/Public/bifrost/transports/bifrost-http/handlers/pii.go)
  - [transports/bifrost-http/handlers/pii_redaction.go](/home/evans/Projects/Public/bifrost/transports/bifrost-http/handlers/pii_redaction.go)
  - [transports/bifrost-http/handlers/pii_middleware.go](/home/evans/Projects/Public/bifrost/transports/bifrost-http/handlers/pii_middleware.go)
  - registered in [transports/bifrost-http/server/server.go](/home/evans/Projects/Public/bifrost/transports/bifrost-http/server/server.go)
- Practical OSS PII contract now includes:
  - `GET/POST/PUT/DELETE /api/pii/policies`
  - `GET/POST /api/pii/policies/{id}/rules`
  - `DELETE /api/pii/rules/{ruleId}`
  - `POST /api/pii/preview`
  - input-side runtime redaction for `TextCompletionRequest`, `ChatCompletionRequest`, and `ResponsesRequest`
- Runtime scope is intentionally narrower than the enterprise design:
  - built-in detectors currently cover `email`, `phone`, `ssn`, `credit_card`, `ip_address`, plus `custom` regex
  - request-side redaction is live; response-side interception remains follow-up work
  - provider-native override matrices are not restored; the providers page is now an OSS preview/status surface instead of a demo CTA
- Direct fallback parity improved for `pii-redactor`:
  - [ui/app/workspace/pii-redactor/page.tsx](/home/evans/Projects/Public/bifrost/ui/app/workspace/pii-redactor/page.tsx) now renders [ui/app/workspace/pii-redactor/rules/piiRedactorRulesView.tsx](/home/evans/Projects/Public/bifrost/ui/app/workspace/pii-redactor/rules/piiRedactorRulesView.tsx)
  - [ui/app/workspace/pii-redactor/rules/page.tsx](/home/evans/Projects/Public/bifrost/ui/app/workspace/pii-redactor/rules/page.tsx) now renders the same local rules view instead of the Book a demo fallback
  - [ui/app/workspace/pii-redactor/providers/page.tsx](/home/evans/Projects/Public/bifrost/ui/app/workspace/pii-redactor/providers/page.tsx) now renders [ui/app/workspace/pii-redactor/providers/piiRedactorProviderView.tsx](/home/evans/Projects/Public/bifrost/ui/app/workspace/pii-redactor/providers/piiRedactorProviderView.tsx)
- The fallback `@enterprise/lib` API for PII is no longer missing:
  - [ui/app/_fallbacks/enterprise/lib/store/apis/piiRedactorApi.ts](/home/evans/Projects/Public/bifrost/ui/app/_fallbacks/enterprise/lib/store/apis/piiRedactorApi.ts)
- Verification completed for this checkpoint:
  - `go test ./configstore/...` in `framework`
  - `go test ./bifrost-http/handlers ./bifrost-http/server ./bifrost-http/integrations` in `transports`
  - `go test -run '^$' ./bifrost-http/lib` in `transports`
  - `npx tsc --noEmit` in `ui`

## 2026-05-08 Restack Closure Checkpoint

- The planned `oss/*` branch line now exists in real git state:
  - `oss/foundation`
  - `oss/foundation-restack`
  - `oss/governance-identity`
  - `oss/runtime-policy`
  - `oss/platform`
  - `oss/ui-restoration`
- The coarse-grained restack was executed as a real commit chain rather than left as planning-only artifacts.
- `oss/foundation` was fast-forwarded to the tip of `oss/ui-restoration`, so the integrated OSS recovery now lives on the intended integration branch while preserving the intermediate `oss/*` history.
- Post-restack committed-state verification passed:
  - `go test ./configstore/...` in `framework`
  - `go test ./bifrost-http/handlers ./bifrost-http/server ./bifrost-http/integrations` in `transports`
  - `go test ./...` in `plugins/governance`
  - `npx tsc --noEmit` in `ui`
  - `make build LOCAL=1`
- Post-restack runtime verification also passed:
  - `./tmp/bifrost-http -app-dir /tmp/bifrost-oss-restack-check -host 127.0.0.1 -port 18081`
  - `curl -sf http://127.0.0.1:18081/health` returned `{"components":{"db_pings":"ok"},"status":"ok"}`
  - the process shut down cleanly on `SIGINT`
- A raw string search still matches `Book a demo` in [ui/components/sidebar.tsx](/home/evans/Projects/Public/bifrost/ui/components/sidebar.tsx), but that occurrence is a static production-support marketing card, not one of the tracked enterprise fallback surfaces from `GK-001`.
