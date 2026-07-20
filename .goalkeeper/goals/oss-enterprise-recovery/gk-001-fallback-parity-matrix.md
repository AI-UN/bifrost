# GK-001 Fallback Parity Matrix

Date: `2026-05-07`

Purpose:
- Inventory current enterprise-gated UI surfaces and fallback behavior.
- Map each demo-only fallback surface to the concrete missing OSS capability it represents.
- Record shared `@enterprise/lib` stubs that silently change behavior outside dedicated placeholder pages.

## Summary

| Category | Count | Notes |
|---|---:|---|
| Full-page demo-only route fallbacks | 17 | Dedicated workspace routes that resolve directly to `ContactUsView` paywalls |
| Embedded or hybrid demo-only surfaces | 6 | Enterprise CTAs embedded inside otherwise usable OSS pages |
| Shared `@enterprise/lib` stub behaviors | 5 | No-op hooks or always-allow RBAC paths that hide missing backend support |
| Existing E2E placeholder coverage entries | 9 | Current tests cover only a subset of placeholder routes |

## A. Full-Page Demo-Only Route Fallbacks

| Workspace surface | Current import target | Current fallback UI | Missing capability bucket | Later plan hook |
|---|---|---|---|---|
| `/workspace/adaptive-routing` | `@enterprise/components/adaptive-routing/adaptiveRoutingView` | CTA: "Unlock adaptive routing for better performance" | Adaptive Routing | `GK-012` |
| `/workspace/alert-channels` | `@enterprise/components/alert-channels/alertChannelsView` | CTA: "Unlock alert channels for better observability" | Alert Channels | `GK-014` |
| `/workspace/audit-logs` | `@enterprise/components/audit-logs/auditLogsView` | CTA: "Unlock audit logs for better compliance" | Audit Logs | `GK-009` |
| `/workspace/cluster` | `@enterprise/components/cluster/clusterView` | CTA: "Unlock cluster mode to scale reliably" | Clustering | `GK-015` |
| `/workspace/governance/access-profiles` | `@enterprise/components/access-profiles/accessProfilesIndexView` | CTA: "Unlock access profiles for better performance" | Access Profiles | `GK-007` |
| `/workspace/governance/business-units` | `@enterprise/components/user-groups/businessUnitsView` | CTA: "Unlock business units & advanced governance" | Business Units / User Groups | `GK-007` |
| `/workspace/governance/rbac` | `@enterprise/components/rbac/rbacView` | CTA: "Unlock roles and permissions for better security" | RBAC | `GK-006` |
| `/workspace/governance/users` | `@enterprise/components/user-groups/usersView` | CTA: "Unlock users & user governance" | Users / User Groups | `GK-007` |
| `/workspace/guardrails` | `@enterprise/components/guardrails/guardrailsConfigurationView` | CTA: "Unlock guardrails for better security" | Guardrails | `GK-010` |
| `/workspace/guardrails/configuration` | `@enterprise/components/guardrails/guardrailsConfigurationView` | CTA: "Unlock guardrails for better security" | Guardrails | `GK-010` |
| `/workspace/guardrails/providers` | `@enterprise/components/guardrails/guardrailsProviderView` | CTA: "Unlock guardrails for better security" | Guardrails provider overrides | `GK-010` |
| `/workspace/mcp-auth-config` | `@enterprise/components/mcp-auth-config/mcpAuthConfigView` | CTA: "Unlock MCP Auth Config" | MCP auth configuration | `GK-013` |
| `/workspace/mcp-tool-groups` | `@enterprise/components/mcp-tool-groups/mcpToolGroups` | CTA: "Unlock MCP Tool Groups" | MCP Tool Groups | `GK-013` |
| `/workspace/pii-redactor` | `@enterprise/components/pii-redactor/piiRedactorRulesView` | CTA: "Unlock PII Redaction for better privacy" | PII Redactor rules | `GK-011` |
| `/workspace/pii-redactor/rules` | `@enterprise/components/pii-redactor/piiRedactorRulesView` | CTA: "Unlock PII Redaction for better privacy" | PII Redactor rules | `GK-011` |
| `/workspace/pii-redactor/providers` | `@enterprise/components/pii-redactor/piiRedactorProviderView` | CTA: "Unlock PII Redaction for better privacy" | PII Redactor provider overrides | `GK-011` |
| `/workspace/scim` | `@enterprise/components/scim/scimView` | CTA: "Unlock SCIM based access management for user provisioning" | SCIM / user provisioning | `GK-008` |

## B. Embedded Or Hybrid Demo-Only Surfaces

These are not pure placeholder routes, but they still represent missing enterprise capability and must be checked in the final parity audit.

| Surface | Current behavior | Missing capability bucket | Evidence |
|---|---|---|---|
| `/workspace/config/api-keys` | Page is usable for OSS basic auth, but embeds a CTA card for "Scope Based API Keys" | Scoped multi-key admin API key management | `ui/app/_fallbacks/enterprise/components/api-keys/apiKeysIndexView.tsx` |
| `/workspace/config/client-settings` | Imports `LargePayloadSettingsFragment`, which currently returns `null`; related API hooks are no-ops | Large Payload Optimization settings UI and persistence | `ui/app/_fallbacks/enterprise/components/large-payload/largePayloadSettingsFragment.tsx` |
| `/workspace/dashboard` | Imports `UserRankingsTab`, which is a demo-only CTA | User Rankings / user-level observability | `ui/app/_fallbacks/enterprise/components/user-rankings/userRankingsTab.tsx` |
| Prompt settings sidebar | Imports `PromptDeploymentsAccordionItem`, which resolves to a demo-only CTA | Prompt Deployments / version rollout strategy | `ui/app/_fallbacks/enterprise/components/prompt-deployments/promptDeploymentView.tsx` |
| Observability BigQuery plugin view | Embedded CTA for native BigQuery ingestion | Data Connectors: BigQuery | `ui/app/_fallbacks/enterprise/components/data-connectors/bigquery/bigqueryConnectorView.tsx` |
| Observability Datadog plugin view | Embedded CTA for native Datadog ingestion | Data Connectors: Datadog | `ui/app/_fallbacks/enterprise/components/data-connectors/datadog/datadogConnectorView.tsx` |

## C. Functional Enterprise Fallback Components That Still Depend On Missing Backends

These surfaces are important because they are not obviously paywalled, but their behavior depends on fallback enterprise stubs.

| Surface | Current fallback behavior | Hidden missing capability |
|---|---|---|
| `/workspace/governance/teams` | `TeamsView` renders a real table workflow, but all permission checks go through always-allow fallback RBAC | Real RBAC enforcement and team-governance permissions |
| Governance, providers, plugins, logs, routing rules, model limits, MCP registry, config sections, sidebar | Many pages call `useRbac(...)`, but fallback RBAC returns `true` for everything | Real role-based UI restrictions and permission-aware actions |
| `/workspace/virtual-keys` usage display | `useVirtualKeyUsage` queries fallback VK-user and access-profile APIs that return `undefined`, so UI silently falls back to raw VK-owned usage | User assignment, access profiles, AP-managed budget/rate-limit display |
| `/workspace/config/security` | `useGetAuthTypeQuery` fallback returns `undefined`; enterprise-specific auth-type handling is absent | SSO/SCIM auth-mode awareness |
| `/workspace/config/client-settings` save flow | Large payload query and mutation hooks are no-ops, so enterprise payload settings cannot be loaded or saved | Large Payload config API |

## D. Shared `@enterprise/lib` Stub Inventory

| Stub file | Current fallback behavior | Affected capability |
|---|---|---|
| `ui/app/_fallbacks/enterprise/lib/contexts/rbacContext.tsx` | `useRbac()` always returns `true`; provider exposes empty permissions | RBAC |
| `ui/app/_fallbacks/enterprise/lib/store/apis/accessProfileApi.ts` | Returns `undefined` data | Access Profiles |
| `ui/app/_fallbacks/enterprise/lib/store/apis/virtualKeyUsersApi.ts` | Returns `undefined` data | Virtual key user assignment |
| `ui/app/_fallbacks/enterprise/lib/store/apis/scimApi.ts` | Returns `undefined` auth type | SCIM / SSO |
| `ui/app/_fallbacks/enterprise/lib/store/apis/largePayloadApi.ts` | Query returns `undefined`; mutation resolves no-op | Large Payload Optimization |

## E. Existing E2E Coverage Against Placeholder Surfaces

File: `tests/e2e/features/placeholders/placeholders.spec.ts`

| Covered surface | Assertion style |
|---|---|
| `/workspace/alert-channels` | Checks CTA text and docs popup URL |
| `/workspace/guardrails` | Checks route loads |
| `/workspace/audit-logs` | Checks route loads |
| `/workspace/cluster` | Checks route loads |
| `/workspace/rbac` | Checks redirect target route loads |
| `/workspace/scim` | Checks route loads |
| `/workspace/adaptive-routing` | Checks CTA text and docs popup URL |
| `/workspace/guardrails/configuration` | Checks route loads |
| `/workspace/guardrails/providers` | Checks route loads |

Not currently covered here:
- Access Profiles
- Users
- Business Units
- MCP Auth Config
- MCP Tool Groups
- PII Redactor routes
- API Keys scoped-key CTA
- User Rankings
- Prompt Deployments
- Connector CTAs
- Large payload hidden fragment

## GK-001 Acceptance Verdict

`complete`

Evidence:
- Direct `@enterprise/components/*` route imports were inventoried from `ui/app/workspace/**`.
- Demo-only fallback components were reviewed from `ui/app/_fallbacks/enterprise/components/**`.
- Shared `@enterprise/lib` stubs were reviewed from `ui/app/_fallbacks/enterprise/lib/**`.
- Existing placeholder E2E coverage was mapped from `tests/e2e/features/placeholders/placeholders.spec.ts`.

Next task:
- `GK-002` classify `PR #2565` by module and portability using this matrix as the frontend gap baseline.
