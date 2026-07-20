# GK-020 Verification Map

Date: `2026-05-07`

Purpose:
- Re-check the `GK-001` fallback parity matrix against the current `oss/foundation` implementation state.
- Record which former demo-only or no-op enterprise surfaces are now backed by real OSS code.
- Separate fully restored surfaces from intentionally narrowed OSS compatibility surfaces.
- Capture residual compatibility imports that still need explicit audit notes before final goal closure.

## Snapshot

- Current integration branch: `oss/foundation`
- Top-level local branch `oss` has been removed at the user's request.
- Final integration wording must stay on the `oss/*` branch line, not a recreated top-level `oss` branch.

Command evidence gathered during this checkpoint:
- `rg -n "@enterprise/components" ui/app/workspace ui/components ui/lib`
  - active workspace import sites were reduced to two functional compatibility surfaces during the first `GK-020` sweep
- `rg -n "@enterprise/lib/store/apis" ui/app ui/components ui/lib`
  - no active workspace consumers remain
- `rg -n "ContactUsView" ui/app/workspace ui/components ui/lib`
  - no active workspace route imports remain

## A. Former Full-Page Demo-Only Routes

| Workspace surface | GK-001 gap bucket | Current status | Evidence | Notes |
|---|---|---|---|---|
| `/workspace/adaptive-routing` | Adaptive Routing | Restored at narrowed OSS scope | `ui/app/workspace/adaptive-routing/page.tsx`, `ui/app/workspace/adaptive-routing/adaptiveRoutingView.tsx`, `transports/bifrost-http/handlers/adaptive_routing.go`, `plugins/governance/adaptive_routing.go` | Uses live policy, metrics, and quality-score data; scope is pragmatic OSS, not enterprise SKU parity. |
| `/workspace/alert-channels` | Alert Channels | Restored at narrowed OSS scope | `ui/app/workspace/alert-channels/page.tsx`, `ui/app/workspace/alert-channels/alertChannelsView.tsx`, `transports/bifrost-http/handlers/alert_channels.go` | Backed by live alert-channel CRUD and trigger-source catalog. |
| `/workspace/audit-logs` | Audit Logs | Restored | `ui/app/workspace/audit-logs/page.tsx`, `ui/app/workspace/audit-logs/auditLogsView.tsx`, `transports/bifrost-http/handlers/audit_handler.go` | No longer a CTA route; backed by persisted audit log APIs. |
| `/workspace/cluster` | Clustering | Restored at narrowed OSS scope | `ui/app/workspace/cluster/page.tsx`, `ui/app/workspace/cluster/clusterView.tsx`, `transports/bifrost-http/handlers/cluster.go`, `framework/cluster/cluster.go` | Single-node/local-manager implementation rather than full enterprise cluster fabric. |
| `/workspace/governance/access-profiles` | Access Profiles | Restored at narrowed OSS compatibility scope | `ui/app/workspace/governance/access-profiles/page.tsx`, `ui/app/workspace/governance/access-profiles/accessProfilesView.tsx`, `transports/bifrost-http/handlers/user_groups.go` | Synthetic access-profile views are derived from `user_groups` plus virtual key assignments. |
| `/workspace/governance/business-units` | Business Units / User Groups | Restored at narrowed OSS compatibility scope | `ui/app/workspace/governance/business-units/page.tsx`, `ui/app/workspace/governance/business-units/businessUnitsView.tsx`, `transports/bifrost-http/handlers/user_groups.go` | Current UI maps business units directly onto `user_groups`. |
| `/workspace/governance/rbac` | RBAC | Restored | `ui/app/workspace/governance/rbac/page.tsx`, `ui/app/workspace/governance/rbac/rbacView.tsx`, `transports/bifrost-http/handlers/rbac.go`, `transports/bifrost-http/handlers/rbac_handler.go` | Real roles, permissions, context, and handler gating exist. |
| `/workspace/governance/users` | Users / User Groups | Restored at narrowed OSS compatibility scope | `ui/app/workspace/governance/users/page.tsx`, `ui/app/workspace/governance/users/usersView.tsx`, `transports/bifrost-http/handlers/user_groups.go` | User list is aggregated from `user_groups` membership rather than a separate enterprise identity catalog. |
| `/workspace/guardrails` | Guardrails | Restored at narrowed OSS first-pass scope | `ui/app/workspace/guardrails/page.tsx`, `ui/app/workspace/guardrails/configuration/guardrailsConfigurationView.tsx`, `transports/bifrost-http/handlers/guardrails.go`, `transports/bifrost-http/handlers/guardrails_middleware.go` | Live CRUD and runtime enforcement exist for the documented OSS-safe subset. |
| `/workspace/guardrails/configuration` | Guardrails | Restored at narrowed OSS first-pass scope | `ui/app/workspace/guardrails/configuration/page.tsx`, `ui/app/workspace/guardrails/configuration/guardrailsConfigurationView.tsx`, `transports/bifrost-http/handlers/guardrails.go` | Shares the same restored configuration surface. |
| `/workspace/guardrails/providers` | Guardrails provider overrides | Restored at narrowed OSS first-pass scope | `ui/app/workspace/guardrails/providers/page.tsx`, `ui/app/workspace/guardrails/providers/guardrailsProviderView.tsx`, `transports/bifrost-http/handlers/guardrails.go` | Live provider override management replaces the CTA page. |
| `/workspace/mcp-auth-config` | MCP auth configuration | Restored at narrowed OSS compatibility scope | `ui/app/workspace/mcp-auth-config/page.tsx`, `ui/app/workspace/mcp-auth-config/mcpAuthConfigView.tsx` | Uses the existing MCP registry/client configuration as the OSS management surface rather than a separate enterprise auth product. |
| `/workspace/mcp-tool-groups` | MCP Tool Groups | Restored | `ui/app/workspace/mcp-tool-groups/page.tsx`, `ui/app/workspace/mcp-tool-groups/mcpToolGroupsView.tsx`, `transports/bifrost-http/handlers/mcp_groups.go` | Live group CRUD and governance attachments are implemented. |
| `/workspace/pii-redactor` | PII Redactor rules | Restored at narrowed OSS first-pass scope | `ui/app/workspace/pii-redactor/page.tsx`, `ui/app/workspace/pii-redactor/rules/piiRedactorRulesView.tsx`, `transports/bifrost-http/handlers/pii.go`, `transports/bifrost-http/handlers/pii_middleware.go` | Backed by live rules, preview APIs, and request-side redaction subset. |
| `/workspace/pii-redactor/rules` | PII Redactor rules | Restored at narrowed OSS first-pass scope | `ui/app/workspace/pii-redactor/rules/page.tsx`, `ui/app/workspace/pii-redactor/rules/piiRedactorRulesView.tsx`, `transports/bifrost-http/handlers/pii.go` | Shares the same restored rules surface. |
| `/workspace/pii-redactor/providers` | PII Redactor provider overrides | Restored at narrowed OSS first-pass scope | `ui/app/workspace/pii-redactor/providers/page.tsx`, `ui/app/workspace/pii-redactor/providers/piiRedactorProviderView.tsx`, `transports/bifrost-http/handlers/pii.go` | Provider override management is live. |
| `/workspace/scim` | SCIM / user provisioning | Restored at narrowed OSS scope | `ui/app/workspace/scim/page.tsx`, `ui/app/workspace/scim/scimView.tsx`, `transports/bifrost-http/handlers/sso_handler.go` | Practical OSS surface for SCIM/SSO config replaces the provisioning CTA. |

## B. Former Embedded Or Hybrid Demo-Only Surfaces

| Surface | GK-001 gap bucket | Current status | Evidence | Notes |
|---|---|---|---|---|
| `/workspace/config/api-keys` | Scoped multi-key admin API key management | Intentionally narrowed OSS compatibility replacement | `framework/configstore/tables/admin_api_key.go`, `framework/configstore/admin_api_keys.go`, `transports/bifrost-http/handlers/admin_api_keys.go`, `transports/bifrost-http/handlers/middlewares.go`, `transports/bifrost-http/handlers/session.go`, `ui/lib/store/apis/adminApiKeysApi.ts`, `ui/lib/store/apis/baseApi.ts`, `ui/lib/store/apis/sessionApi.ts`, `ui/app/workspace/config/api-keys/apiKeysView.tsx`, `ui/app/login/loginView.tsx` | Now restores persisted Admin API Key issuance, Bearer-token auth for admin APIs, and browser login support for Admin API Keys, while still using Virtual Keys as the OSS scoped-key compatibility layer instead of recreating the exact enterprise scope-based key product. |
| `/workspace/config/client-settings` large payload fragment | Large Payload Optimization | Restored at narrowed OSS first-pass scope | `ui/app/workspace/config/views/clientSettingsView.tsx`, `ui/app/workspace/config/views/largePayloadSettingsFragment.tsx`, `transports/bifrost-http/handlers/payload.go` | The fragment is now local to the OSS workspace and backed by live payload config APIs. |
| `/workspace/dashboard` user rankings tab | User-level observability / rankings | Restored | `ui/app/workspace/dashboard/page.tsx`, `ui/app/workspace/dashboard/components/userRankingsTab.tsx`, `transports/bifrost-http/handlers/logging.go`, `ui/lib/store/apis/logsApi.ts` | Live `GET /api/logs/user-rankings` endpoint and RTK query replace the CTA tab. |
| Prompt settings sidebar deployments accordion | Prompt deployments / rollout strategy | Intentionally narrowed OSS compatibility replacement | `ui/components/prompts/fragments/settingsPanel.tsx`, `ui/components/prompts/fragments/promptDeploymentsAccordionItem.tsx` | Uses saved sessions and committed prompt versions as the rollout artifact rather than a separate enterprise deployment subsystem. |
| Observability BigQuery plugin view | Data Connectors: BigQuery | Restored at narrowed OSS first-pass scope | `ui/app/workspace/observability/views/plugins/bigqueryView.tsx`, `ui/app/workspace/observability/views/plugins/connectorConfigView.tsx`, `transports/bifrost-http/handlers/connectors.go` | Connector CRUD/test UI is live. |
| Observability Datadog plugin view | Data Connectors: Datadog | Restored at narrowed OSS first-pass scope | `ui/app/workspace/observability/views/plugins/datadogView.tsx`, `ui/app/workspace/observability/views/plugins/connectorConfigView.tsx`, `transports/bifrost-http/handlers/connectors.go` | Connector CRUD/test UI is live. |

## C. Former Functional Fallback Components Backed By Stubs

| Surface | GK-001 hidden gap | Current status | Evidence | Notes |
|---|---|---|---|---|
| `/workspace/governance/teams` | Real RBAC enforcement for a functional page | Functionally restored at narrowed OSS compatibility scope | `ui/app/workspace/governance/teams/page.tsx`, `ui/app/workspace/governance/teams/teamsView.tsx`, `ui/app/_fallbacks/enterprise/lib/contexts/rbacContext.tsx` | The page is now local to the OSS workspace and runs against the live RBAC context instead of the original always-allow stub. |
| Governance, providers, plugins, logs, routing rules, MCP registry, config sections, sidebar | Permission-aware UI actions | Functionally restored through live RBAC compatibility layer | `ui/app/_fallbacks/enterprise/lib/contexts/rbacContext.tsx`, `transports/bifrost-http/handlers/rbac.go`, `transports/bifrost-http/handlers/rbac_route_test.go` | Many pages still import RBAC enums/hooks from `@enterprise/lib`, but the context is no longer a pure allow-all stub. |
| `/workspace/virtual-keys` usage display | User assignment / AP-managed usage context | Restored at narrowed OSS compatibility scope | `ui/app/workspace/virtual-keys/hooks/useVirtualKeyUsage.ts`, `ui/app/_fallbacks/enterprise/lib/store/apis/virtualKeyUsersApi.ts`, `ui/app/_fallbacks/enterprise/lib/store/apis/accessProfileApi.ts`, `transports/bifrost-http/handlers/user_groups.go` | Still uses synthetic compatibility responses built on top of `user_groups`. |
| `/workspace/config/security` auth-type awareness | SCIM / SSO auth-mode awareness | Restored | `ui/app/workspace/config/views/securityView.tsx`, `ui/app/_fallbacks/enterprise/lib/store/apis/scimApi.ts`, `transports/bifrost-http/handlers/sso_handler.go` | No longer depends on an undefined auth-type stub. |
| `/workspace/config/client-settings` save flow | Large payload config API | Restored | `ui/app/workspace/config/views/clientSettingsView.tsx`, `ui/app/_fallbacks/enterprise/lib/store/apis/largePayloadApi.ts`, `transports/bifrost-http/handlers/payload.go` | Query and mutation hooks are live instead of no-op. |

## D. Shared Stub Inventory Re-Check

| Stub or compatibility path | Current state | Evidence | Audit note |
|---|---|---|---|
| `ui/app/_fallbacks/enterprise/lib/contexts/rbacContext.tsx` | Upgraded from always-allow to live RBAC context | `ui/app/_fallbacks/enterprise/lib/contexts/rbacContext.tsx` | Still a compatibility layer, but no longer a fake permission model. |
| `ui/app/_fallbacks/enterprise/lib/store/apis/accessProfileApi.ts` | Live compatibility API | `ui/app/_fallbacks/enterprise/lib/store/apis/accessProfileApi.ts`, `transports/bifrost-http/handlers/user_groups.go` | Synthetic AP model remains an explicit narrowed scope. |
| `ui/app/_fallbacks/enterprise/lib/store/apis/virtualKeyUsersApi.ts` | Live compatibility API | `ui/app/_fallbacks/enterprise/lib/store/apis/virtualKeyUsersApi.ts`, `transports/bifrost-http/handlers/user_groups.go` | Backed by `user_groups` compatibility endpoints. |
| `ui/app/_fallbacks/enterprise/lib/store/apis/scimApi.ts` | Live API | `ui/app/_fallbacks/enterprise/lib/store/apis/scimApi.ts`, `transports/bifrost-http/handlers/sso_handler.go` | No longer returns undefined auth-type data. |
| `ui/app/_fallbacks/enterprise/lib/store/apis/largePayloadApi.ts` | Live API | `ui/app/_fallbacks/enterprise/lib/store/apis/largePayloadApi.ts`, `transports/bifrost-http/handlers/payload.go` | No longer a no-op query/mutation pair. |

## E. Residual Compatibility Imports That Still Need Explicit Final Audit Notes

These are not direct evidence of missing functionality, but they remain important to call out before goal closure.

| Residual import or pattern | Current state | Evidence | Why it matters |
|---|---|---|---|
| Direct `@enterprise/components` imports in active workspace code | Cleared | follow-up localizations moved `LargePayloadSettingsFragment`, `TeamsView`, and `LoginView` onto local `ui/app/**` paths; `rg -n "@enterprise/components" ui` now returns no matches | This compatibility debt is now removed from the UI tree. |
| Direct `@enterprise/lib/store/apis` consumers in active workspace code | Cleared | `rg -n "@enterprise/lib/store/apis" ui/app ui/components ui/lib` | Important `GK-019` milestone: active consumers no longer bypass the local OSS store barrel. |
| `ui/lib/store/apis/baseApi.ts` imports enterprise refresh/token helpers | Cleared | local `ui/lib/store/utils/baseQueryWithRefresh.ts` and `ui/lib/store/utils/tokenManager.ts`; `ui/lib/store/apis/baseApi.ts` now imports from those local paths | OSS no-op refresh/token behavior is now local rather than borrowed from fallback enterprise utils. |
| `ui/lib/store/store.ts` and `ui/lib/store/slices/index.ts` still import enterprise reducers/slices | Cleared | local `ui/lib/store/slices/enterpriseSlices.ts`; `ui/lib/store/store.ts` and `ui/lib/store/slices/index.ts` now import from local paths | The shared store no longer depends on fallback enterprise slice exports. |
| Widespread `@enterprise/lib` RBAC enums/hooks/types imports | Cleared in active UI | local `ui/lib/rbac.ts` plus `ui/lib/contexts/rbacContext.tsx`; `rg -n '@enterprise/lib' ui/app ui/components ui/lib` now only returns fallback-tree files | Active UI now uses a local RBAC barrel/context even though fallback duplicates still exist on disk. |

## F. Verification Results

Command evidence completed during `GK-020`:

| Command | Result | Notes |
|---|---|---|
| `cd ui && npx tsc --noEmit` | passed | Confirms current UI changes and store wiring type-check on the integration branch. |
| `cd transports && go test ./bifrost-http/handlers ./bifrost-http/server ./bifrost-http/integrations` | passed | Confirms transport handlers, server registration, and SDK integration compilation/tests for the restored surfaces. |
| `cd framework && go test ./configstore/...` | passed | Confirms configstore tables and persistence layer for restored enterprise-backed features. |
| `cd plugins/governance && go test ./...` | passed | Confirms adaptive-routing/governance plugin layer remains green. |
| `cd tests/e2e && npm ci` | passed | Required because local E2E dependencies were not installed in this workspace snapshot. |
| `cd tests/e2e && npx playwright test features/placeholders/placeholders.spec.ts --list` | passed | Confirms the updated parity-oriented Playwright spec loads and enumerates 20 tests. |

Not completed in this checkpoint:
- Full Playwright execution was not run because no dev server/browser session was started as part of this checkpoint.

## G. Integration Notes And Follow-Ups

Integration notes:
- The current long-lived integration branch remains `oss/foundation`.
- Because the user removed the top-level local `oss` branch mid-plan, the effective integration target for this restoration effort is the `oss/*` branch line rather than a recreated `oss` branch.
- At this checkpoint, the restored feature work already exists on the current integration branch, so `GK-020` merge documentation is primarily about preserving the `oss/*` branch-line strategy and recording residual compatibility debt, not replaying a pending branch stack.

Explicit follow-ups before final goal closure:
- Run the final `GK-021` missing-feature vs fallback-UI parity audit using this document and the original `GK-001` matrix.
- Decide whether the remaining compatibility imports under `@enterprise/*` are acceptable to keep or should be localized further.
- If full E2E execution is required for closure, run it with the expected dev server/browser environment rather than relying only on Playwright test discovery.

## Initial GK-020 Verdict

Current conclusion:
- Every `GK-001` direct demo-only route is now mapped to either:
  - a real restored OSS implementation, or
  - an explicitly narrowed OSS compatibility replacement with live behavior.
- The major remaining work for `GK-020` is verification and documentation, not broad missing-feature implementation.
- Residual risk is concentrated in:
  - compatibility imports still living under `@enterprise/*`
  - final acceptance of the remaining compatibility imports
  - final command-based verification on the integrated branch

Next `GK-020` actions:
- Sync Goal Keeper progress and memory with the verification results and residual compatibility notes.
- Move focus to `GK-021` for the explicit final parity audit.
