# GK-023 Restack Manifest

Date: `2026-05-07`

Purpose:
- Translate the recommended coarse-grained `oss/*` restack into a staging-ready manifest.
- Identify which paths can be moved branch-by-branch as whole files and which shared files require hunk-level splitting.
- Preserve the current verified `oss/foundation` behavior while making the eventual branch restack operationally concrete.

Related artifacts:
- [gk-003-execution-baseline.md](/home/evans/Projects/Public/bifrost/.goalkeeper/goals/oss-enterprise-recovery/gk-003-execution-baseline.md)
- [gk-022-restack-options.md](/home/evans/Projects/Public/bifrost/.goalkeeper/goals/oss-enterprise-recovery/gk-022-restack-options.md)

## Restack Target

Recommended branch line:

1. `oss/foundation`
2. `oss/governance-identity`
3. `oss/runtime-policy`
4. `oss/platform`
5. `oss/ui-restoration`

## Exclusions

Do not include these in feature-restack commits:

- `.goalkeeper/**`
- `.gitignore`
- `AGENTS.md`

Reason:
- `.goalkeeper/**` is planning state, not product code
- `.gitignore` and `AGENTS.md` are known unrelated local user changes

## Ownership Rules

- Prefer whole-file ownership when a path is clearly feature-local.
- Use hunk-splitting only for files that are already known to be shared across several restored feature buckets.
- When a shared UI or server-registration file mixes several restored features, prefer landing the broad visual/local-store cleanup in `oss/ui-restoration` and keep earlier branches backend-heavy.

## Branch Manifests

### `oss/foundation`

Whole-file paths or globs:

- `Makefile`
- `core/schemas/bifrost.go`
- `core/schemas/envvar.go`
- `core/schemas/large_payload_config.go`
- `framework/config.go`
- `framework/envutils/utils.go`
- `framework/configstore/migrations.go`
- `framework/configstore/migrations_test.go`
- `framework/configstore/rdb_test.go`
- `framework/configstore/store.go`
- `framework/configstore/tables/framework.go`
- `transports/bifrost-http/lib/config.go`
- `transports/bifrost-http/lib/config_test.go`

Shared files that likely need hunk-splitting later:

- `transports/bifrost-http/server/server.go`
  - foundation owns only the baseline wiring needed before feature registrations diverge

Why this bucket is safe:
- these files are mostly shared scaffolding or build/runtime plumbing, not feature-local UI

### `oss/governance-identity`

Whole-file paths or globs:

- `framework/configstore/admin_api_keys.go`
- `framework/configstore/admin_api_keys_test.go`
- `framework/configstore/rbac.go`
- `framework/configstore/rbac_seed.go`
- `framework/configstore/rbac_seed_test.go`
- `framework/configstore/rbac_test.go`
- `framework/configstore/sso.go`
- `framework/configstore/sso_test.go`
- `framework/configstore/user_groups.go`
- `framework/configstore/user_groups_test.go`
- `framework/configstore/tables/admin_api_key.go`
- `framework/configstore/tables/rbac.go`
- `framework/configstore/tables/sso.go`
- `framework/configstore/tables/user_groups.go`
- `transports/bifrost-http/handlers/admin_api_keys.go`
- `transports/bifrost-http/handlers/admin_api_keys_test.go`
- `transports/bifrost-http/handlers/rbac.go`
- `transports/bifrost-http/handlers/rbac_handler.go`
- `transports/bifrost-http/handlers/rbac_route_test.go`
- `transports/bifrost-http/handlers/session_test.go`
- `transports/bifrost-http/handlers/sso_handler.go`
- `transports/bifrost-http/handlers/sso_handler_test.go`
- `transports/bifrost-http/handlers/user_groups.go`
- `transports/bifrost-http/handlers/user_groups_compat_test.go`
- `ui/app/workspace/governance/access-profiles/accessProfilesView.tsx`
- `ui/app/workspace/governance/business-units/businessUnitsView.tsx`
- `ui/app/workspace/governance/rbac/rbacView.tsx`
- `ui/app/workspace/governance/teams/teamsView.tsx`
- `ui/app/workspace/governance/users/usersView.tsx`
- `ui/app/workspace/scim/scimView.tsx`
- `ui/app/workspace/config/api-keys/apiKeysView.tsx`
- `ui/app/login/loginView.tsx`
- `ui/lib/contexts/rbacContext.tsx`
- `ui/lib/rbac.ts`
- `ui/lib/store/apis/accessProfileApi.ts`
- `ui/lib/store/apis/adminApiKeysApi.ts`
- `ui/lib/store/apis/rbacApi.ts`
- `ui/lib/store/apis/scimApi.ts`
- `ui/lib/store/apis/sessionApi.ts`
- `ui/lib/store/apis/userGovernanceApi.ts`
- `ui/lib/store/apis/virtualKeyUsersApi.ts`
- `ui/lib/types/accessProfile.ts`
- `ui/lib/types/rbac.ts`
- `ui/lib/types/user.ts`

Shared files that likely need hunk-splitting later:

- `transports/bifrost-http/handlers/middlewares.go`
- `transports/bifrost-http/handlers/middlewares_test.go`
- `transports/bifrost-http/handlers/session.go`
- `transports/bifrost-http/handlers/governance.go`
- `transports/bifrost-http/server/server.go`
- `ui/app/workspace/governance/views/teamDialog.tsx`
- `ui/app/workspace/governance/views/teamsTable.tsx`
- `ui/app/workspace/governance/views/customerDialog.tsx`
- `ui/app/workspace/governance/views/customerTable.tsx`
- `ui/app/workspace/governance/virtual-keys/page.tsx`
- `ui/app/workspace/virtual-keys/hooks/useVirtualKeyUsage.ts`
- `ui/app/workspace/virtual-keys/views/virtualKeyDetailsSheet.tsx`
- `ui/app/workspace/virtual-keys/views/virtualKeySheet.tsx`
- `ui/app/workspace/virtual-keys/views/virtualKeysTable.tsx`
- `ui/app/workspace/config/views/securityView.tsx`

Ownership note:
- if the restack aims to minimize hunk-splitting, the `virtual-keys` display refinements can move to `oss/ui-restoration`; if it aims to keep governance compatibility together, keep them here

### `oss/runtime-policy`

Whole-file paths or globs:

- `framework/configstore/adaptive_routing.go`
- `framework/configstore/adaptive_routing_test.go`
- `framework/configstore/alerting.go`
- `framework/configstore/alerting_test.go`
- `framework/configstore/audit.go`
- `framework/configstore/audit_test.go`
- `framework/configstore/connectors.go`
- `framework/configstore/connectors_test.go`
- `framework/configstore/guardrails.go`
- `framework/configstore/guardrails_test.go`
- `framework/configstore/mcp_groups.go`
- `framework/configstore/mcp_groups_test.go`
- `framework/configstore/pii.go`
- `framework/configstore/pii_test.go`
- `framework/configstore/tables/adaptive_routing.go`
- `framework/configstore/tables/alerting.go`
- `framework/configstore/tables/audit.go`
- `framework/configstore/tables/connectors.go`
- `framework/configstore/tables/guardrails.go`
- `framework/configstore/tables/mcp_groups.go`
- `framework/configstore/tables/pii.go`
- `plugins/governance/adaptive_routing.go`
- `plugins/governance/adaptive_routing_test.go`
- `transports/bifrost-http/handlers/adaptive_routing.go`
- `transports/bifrost-http/handlers/adaptive_routing_test.go`
- `transports/bifrost-http/handlers/alert_channels.go`
- `transports/bifrost-http/handlers/alert_channels_test.go`
- `transports/bifrost-http/handlers/audit_handler.go`
- `transports/bifrost-http/handlers/audit_handler_test.go`
- `transports/bifrost-http/handlers/connectors.go`
- `transports/bifrost-http/handlers/connectors_test.go`
- `transports/bifrost-http/handlers/guardrails.go`
- `transports/bifrost-http/handlers/guardrails_middleware.go`
- `transports/bifrost-http/handlers/guardrails_test.go`
- `transports/bifrost-http/handlers/logging_rankings_test.go`
- `transports/bifrost-http/handlers/mcp_groups.go`
- `transports/bifrost-http/handlers/mcp_groups_test.go`
- `transports/bifrost-http/handlers/pii.go`
- `transports/bifrost-http/handlers/pii_handler_test.go`
- `transports/bifrost-http/handlers/pii_middleware.go`
- `transports/bifrost-http/handlers/pii_redaction.go`
- `ui/app/workspace/adaptive-routing/adaptiveRoutingView.tsx`
- `ui/app/workspace/alert-channels/alertChannelsView.tsx`
- `ui/app/workspace/audit-logs/auditLogsView.tsx`
- `ui/app/workspace/guardrails/configuration/guardrailsConfigurationView.tsx`
- `ui/app/workspace/guardrails/providers/guardrailsProviderView.tsx`
- `ui/app/workspace/mcp-auth-config/mcpAuthConfigView.tsx`
- `ui/app/workspace/mcp-tool-groups/mcpToolGroupsView.tsx`
- `ui/app/workspace/observability/views/plugins/connectorConfigView.tsx`
- `ui/app/workspace/pii-redactor/providers/piiRedactorProviderView.tsx`
- `ui/app/workspace/pii-redactor/rules/piiRedactorRulesView.tsx`
- `ui/app/workspace/dashboard/components/userRankingsTab.tsx`
- `ui/lib/store/apis/adaptiveRoutingApi.ts`
- `ui/lib/store/apis/alertChannelsApi.ts`
- `ui/lib/store/apis/auditLogsApi.ts`
- `ui/lib/store/apis/connectorsApi.ts`
- `ui/lib/store/apis/guardrailsApi.ts`
- `ui/lib/store/apis/mcpToolGroupsApi.ts`
- `ui/lib/store/apis/piiRedactorApi.ts`

Shared files that likely need hunk-splitting later:

- `plugins/governance/main.go`
- `plugins/governance/allowonallvirtualkeys_test.go`
- `plugins/logging/operations.go`
- `plugins/logging/utils.go`
- `transports/bifrost-http/handlers/logging.go`
- `transports/bifrost-http/handlers/mcp.go`
- `transports/bifrost-http/handlers/mcpserver.go`
- `transports/bifrost-http/handlers/plugins.go`
- `transports/bifrost-http/server/server.go`
- `ui/app/workspace/observability/views/plugins/bigqueryView.tsx`
- `ui/app/workspace/observability/views/plugins/datadogView.tsx`

### `oss/platform`

Whole-file paths or globs:

- `core/providers/utils/large_response.go`
- `framework/cluster/**`
- `framework/configstore/vault.go`
- `framework/configstore/vault_test.go`
- `framework/configstore/tables/vault.go`
- `framework/vault/**`
- `transports/bifrost-http/handlers/cluster.go`
- `transports/bifrost-http/handlers/cluster_test.go`
- `transports/bifrost-http/handlers/payload.go`
- `transports/bifrost-http/handlers/payload_test.go`
- `transports/bifrost-http/handlers/vault.go`
- `transports/bifrost-http/handlers/vault_test.go`
- `transports/bifrost-http/lib/config_vault_test.go`
- `ui/app/workspace/cluster/clusterView.tsx`
- `ui/app/workspace/config/views/largePayloadSettingsFragment.tsx`
- `ui/lib/store/apis/clusterApi.ts`
- `ui/lib/store/apis/largePayloadApi.ts`
- `ui/lib/store/apis/vaultApi.ts`
- `ui/lib/types/largePayload.ts`

Shared files that likely need hunk-splitting later:

- `transports/bifrost-http/handlers/config.go`
- `transports/bifrost-http/handlers/providers.go`
- `transports/bifrost-http/server/server.go`
- `ui/app/workspace/config/views/clientSettingsView.tsx`
- `ui/app/workspace/config/views/securityView.tsx`
- `ui/app/workspace/providers/fragments/apiKeysFormFragment.tsx`
- `ui/app/workspace/providers/fragments/networkFormFragment.tsx`
- `ui/app/workspace/providers/fragments/proxyFormFragment.tsx`
- `ui/app/workspace/providers/views/providerKeyForm.tsx`

### `oss/ui-restoration`

Whole-file paths or globs:

- `tests/e2e/features/placeholders/placeholders.spec.ts`
- `ui/app/_fallbacks/enterprise/**`
- `ui/components/prompts/context.tsx`
- `ui/components/prompts/fragments/promptDeploymentsAccordionItem.tsx`
- `ui/components/prompts/fragments/settingsPanel.tsx`
- `ui/components/sidebar.tsx`
- `ui/lib/store/apis/baseApi.ts`
- `ui/lib/store/apis/index.ts`
- `ui/lib/store/slices/enterpriseSlices.ts`
- `ui/lib/store/slices/index.ts`
- `ui/lib/store/store.ts`
- `ui/lib/store/utils/**`

Shared files that likely need hunk-splitting later:

- `ui/app/clientLayout.tsx`
- `ui/app/workspace/config/layout.tsx`
- `ui/app/workspace/config/page.tsx`
- `ui/app/workspace/config/views/compatibilityView.tsx`
- `ui/app/workspace/config/views/loggingView.tsx`
- `ui/app/workspace/config/views/mcpView.tsx`
- `ui/app/workspace/config/views/modelSettingsView.tsx`
- `ui/app/workspace/config/views/observabilityView.tsx`
- `ui/app/workspace/config/views/performanceTuningView.tsx`
- `ui/app/workspace/config/views/pricingConfigView.tsx`
- `ui/app/workspace/config/views/proxyView.tsx`
- `ui/app/workspace/dashboard/page.tsx`
- `ui/app/workspace/logs/page.tsx`
- `ui/app/workspace/mcp-registry/views/mcpClientForm.tsx`
- `ui/app/workspace/mcp-registry/views/mcpClientSheet.tsx`
- `ui/app/workspace/mcp-registry/views/mcpClientsTable.tsx`
- `ui/app/workspace/model-catalog/views/modelCatalogView.tsx`
- `ui/app/workspace/model-limits/views/modelLimitSheet.tsx`
- `ui/app/workspace/model-limits/views/modelLimitsTable.tsx`
- `ui/app/workspace/model-limits/views/modelLimitsView.tsx`
- `ui/app/workspace/plugins/sheets/addNewPluginSheet.tsx`
- `ui/app/workspace/plugins/views/pluginsView.tsx`
- `ui/app/workspace/providers/dialogs/addNewCustomProviderSheet.tsx`
- `ui/app/workspace/providers/dialogs/confirmDeleteProviderDialog.tsx`
- `ui/app/workspace/providers/dialogs/providerConfigSheet.tsx`
- `ui/app/workspace/providers/fragments/apiStructureFormFragment.tsx`
- `ui/app/workspace/providers/fragments/betaHeadersFormFragment.tsx`
- `ui/app/workspace/providers/fragments/debuggingFormFragment.tsx`
- `ui/app/workspace/providers/fragments/governanceFormFragment.tsx`
- `ui/app/workspace/providers/fragments/openaiConfigFormFragment.tsx`
- `ui/app/workspace/providers/fragments/performanceFormFragment.tsx`
- `ui/app/workspace/providers/views/modelProviderConfig.tsx`
- `ui/app/workspace/providers/views/modelProviderKeysTableView.tsx`
- `ui/app/workspace/providers/views/providerGovernanceTable.tsx`
- `ui/app/workspace/routing-rules/views/routingRuleSheet.tsx`
- `ui/app/workspace/routing-rules/views/routingRulesView.tsx`
- `ui/components/ui/envVarInput.tsx`
- `ui/lib/types/config.ts`

## Shared Transport Registration File

Primary shared file:

- `transports/bifrost-http/server/server.go`

Recommended handling:

1. `oss/foundation`
   - only baseline/common bootstrap adjustments
2. `oss/governance-identity`
   - RBAC, user-groups, SSO, admin API keys registration hunks
3. `oss/runtime-policy`
   - audit, adaptive routing, guardrails, PII, MCP groups, alerts, connectors registration hunks
4. `oss/platform`
   - cluster, vault, payload registration hunks

## Shared UI Config File

Highest-risk shared UI file:

- `ui/app/workspace/config/views/securityView.tsx`

Recommended handling:

- keep auth-type and admin-auth related changes in `oss/governance-identity`
- keep vault-related changes in `oss/platform`
- if hunk-splitting becomes too brittle, defer the whole file to `oss/ui-restoration` after backend branches land

## Staging Discipline

Before any real restack starts:

1. Save this manifest as the branch-file source of truth.
2. Restack one target branch at a time.
3. After staging a branch, run a quick ownership audit with `git diff --cached --name-only`.
4. If a shared file shows unrelated hunks, unstage and re-add by hunk.
5. Re-run targeted verification after each branch or after each merge batch.

## Manifest Verdict

This manifest is sufficient to start a coarse-grained `oss/*` restack once the user explicitly approves that option.
