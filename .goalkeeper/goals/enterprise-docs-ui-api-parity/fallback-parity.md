# Fallback UI Parity Audit

## Scope

This audit compares the current demo-only fallback surfaces under `ui/app/_fallbacks/enterprise/components/**` against the research package assembled from official docs.

Status meanings:

- `covered`: official docs package contains enough evidence to target the surface directly
- `partial`: official docs mention the capability, but the final UI/API shape still depends on inference
- `unresolved`: the current fallback exists, but the gathered official enterprise docs do not confirm it as an enterprise surface

## Matrix

| Fallback component | Official evidence status | Closest research artifact | Audit result | Notes |
| --- | --- | --- | --- | --- |
| `rbac/rbacView.tsx` | screenshot + doc text + partial API | `inventory.md`, `design-notes.md`, `api-parity.md`, `reference-specs/governance-identity.md` | `covered` | High-confidence surface. |
| `scim/scimView.tsx` | screenshot + doc text + inference | `inventory.md`, `design-notes.md`, `api-parity.md`, `reference-specs/governance-identity.md` | `covered` | High-confidence UI, private contract uncertainty remains. |
| `guardrails/guardrailsConfigurationView.tsx` | screenshot + doc text + inference | `inventory.md`, `design-notes.md`, `api-parity.md`, `reference-specs/policy-routing.md` | `covered` | Rule table and drawer are fact-backed. |
| `guardrails/guardrailsProviderView.tsx` | screenshot + doc text + inference | `inventory.md`, `design-notes.md`, `api-parity.md`, `reference-specs/policy-routing.md` | `covered` | Provider/profile nuances still partly inferred. |
| `adaptive-routing/adaptiveRoutingView.tsx` | screenshot + doc text + inference | `inventory.md`, `design-notes.md`, `api-parity.md`, `reference-specs/policy-routing.md` | `covered` | Metrics dashboard hierarchy is fact-backed. |
| `data-connectors/datadog/datadogConnectorView.tsx` | screenshot + doc text + inference | `inventory.md`, `design-notes.md`, `api-parity.md`, `reference-specs/connectors-observability.md` | `covered` | API remains private-only. |
| `audit-logs/auditLogsView.tsx` | doc text + inference | `inventory.md`, `inference-notes.md`, `api-parity.md`, `reference-specs/audit-mcp.md` | `partial` | No screenshot-backed layout evidence. |
| `cluster/clusterView.tsx` | doc text + inference | `inventory.md`, `inference-notes.md`, `api-parity.md` | `partial` | No screenshot-backed layout evidence. |
| `mcp-auth-config/mcpAuthConfigView.tsx` | doc text + public API + inference | `inventory.md`, `api-inventory.md`, `api-parity.md`, `reference-specs/audit-mcp.md` | `partial` | Narrative and API evidence only. |
| `access-profiles/accessProfilesIndexView.tsx` | doc text + inference | `inventory.md`, `inference-notes.md`, `api-parity.md` | `partial` | Mentioned through governance/provisioning outcomes, not screenshot-backed. |
| `user-groups/usersView.tsx` | doc text + inference | `inventory.md`, `inference-notes.md`, `reference-specs/governance-identity.md` | `partial` | Official users table is only indirectly evidenced via provisioning screenshots. |
| `user-groups/teamsView.tsx` | doc text + inference | `inventory.md`, `inference-notes.md`, `reference-specs/governance-identity.md` | `partial` | Team concepts are fact-backed; dedicated team page layout is not. |
| `user-groups/businessUnitsView.tsx` | doc text + inference | `inventory.md`, `inference-notes.md`, `reference-specs/governance-identity.md` | `partial` | BU mappings are visible in SCIM mapping screenshots, but dedicated BU page is inferred. |
| `api-keys/apiKeysIndexView.tsx` | doc text + inference | `inventory.md`, `inference-notes.md`, `api-parity.md` | `partial` | Governance/API key concepts exist, but this exact fallback page is not screenshot-backed. |
| `alert-channels/alertChannelsView.tsx` | no official enterprise page found | none | `unresolved` | Present in restored/fallback OSS surface, but not in the official enterprise inventory. |
| `data-connectors/bigquery/bigqueryConnectorView.tsx` | no official enterprise page found | none | `unresolved` | Not supported by the gathered official docs. |
| `large-payload/largePayloadSettingsFragment.tsx` | no official enterprise page found | none | `unresolved` | Not supported by the gathered official docs. |
| `login/loginView.tsx` | no dedicated enterprise product page found | none | `unresolved` | Provider setup guides show external IdP consoles, not a Bifrost enterprise login page template. |
| `mcp-tool-groups/mcpToolGroups.tsx` | no official enterprise page found | none | `unresolved` | Private restored module only. |
| `pii-redactor/piiRedactorProviderView.tsx` | no official enterprise page found | none | `unresolved` | Private restored module only. |
| `pii-redactor/piiRedactorRulesView.tsx` | no official enterprise page found | none | `unresolved` | Private restored module only. |
| `prompt-deployments/promptDeploymentView.tsx` | no official enterprise page found | none | `unresolved` | Private restored module only. |
| `user-rankings/userRankingsTab.tsx` | no official enterprise page found | none | `unresolved` | Logging-adjacent surface exists in branch, but not in official enterprise docs set. |

## Audit Result

Covered or partially covered by the official docs package:

- RBAC
- SCIM / User Provisioning
- Guardrails
- Adaptive Routing
- Datadog connector
- Audit Logs
- Cluster
- MCP Auth Config
- Access Profiles
- Users / Teams / Business Units
- API Keys

Still unresolved against the official docs package:

- alert channels
- BigQuery connector
- large payload settings
- login enterprise template
- MCP tool groups
- PII redactor
- prompt deployments
- user rankings

## Practical Instruction For The Next Goal

Only the `covered` and `partial` rows should be treated as fact-backed parity targets on the first implementation pass. The `unresolved` rows need fresh official evidence before they can be promoted to enterprise-parity work rather than branch-only restoration work.
