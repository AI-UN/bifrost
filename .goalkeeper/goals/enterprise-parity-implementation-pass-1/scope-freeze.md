# Scope Freeze — GK-003

## Frozen Modules (explicitly deferred)

The following modules are marked `unresolved` in `fallback-parity.md` and are **excluded from this implementation pass**. They must not be modified, restructured, or claimed as enterprise-parity work unless new official evidence is discovered in a future goal.

| # | Module | Fallback surface | Reason for deferral |
|---|--------|-----------------|---------------------|
| 1 | Alert Channels | `_fallbacks/enterprise/components/alert-channels/alertChannelsView.tsx` | No official enterprise page found in gathered docs. |
| 2 | BigQuery Connector | `_fallbacks/enterprise/components/data-connectors/bigquery/bigqueryConnectorView.tsx` | Not supported by gathered official docs. |
| 3 | Large Payload Settings | `_fallbacks/enterprise/components/large-payload/largePayloadSettingsFragment.tsx` | No official enterprise page found. |
| 4 | Login Enterprise Template | `_fallbacks/enterprise/components/login/loginView.tsx` | Provider setup guides show external IdP consoles, not a Bifrost enterprise login page. |
| 5 | MCP Tool Groups | `_fallbacks/enterprise/components/mcp-tool-groups/mcpToolGroups.tsx` | Private restored module only — not in official enterprise inventory. |
| 6 | PII Redactor (Providers) | `_fallbacks/enterprise/components/pii-redactor/piiRedactorProviderView.tsx` | Private restored module only. |
| 7 | PII Redactor (Rules) | `_fallbacks/enterprise/components/pii-redactor/piiRedactorRulesView.tsx` | Private restored module only. |
| 8 | Prompt Deployments | `_fallbacks/enterprise/components/prompt-deployments/promptDeploymentView.tsx` | Private restored module only. |
| 9 | User Rankings | `_fallbacks/enterprise/components/user-rankings/userRankingsTab.tsx` | Not in official enterprise docs set. |

## Workspace routes for frozen modules

These workspace routes exist on `oss/ui-restoration` and will be preserved as-is but **not touched** by this implementation pass:

- `ui/app/workspace/alert-channels/`
- `ui/app/workspace/mcp-tool-groups/`
- `ui/app/workspace/pii-redactor/`
- `ui/app/workspace/prompt-repo/` (may overlap with prompt deployments)

## Guardrails

1. No UI restructuring of frozen modules.
2. No new backend handler changes for frozen modules.
3. No new API slice changes for frozen modules.
4. If a frozen module's route is accidentally touched by a dependency change, document it in memory.md.
5. These modules may receive incidental type imports or shared component updates, but should not have their primary views rewritten.

## Promotion Criteria

A frozen module can be promoted to in-scope in a future goal if and only if:
- New official enterprise documentation is discovered (screenshot or dedicated page)
- The evidence is added to the research package under `.goalkeeper/goals/enterprise-docs-ui-api-parity/`
- The `fallback-parity.md` status is updated from `unresolved` to `partial` or `covered`
