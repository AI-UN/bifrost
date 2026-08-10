# API Parity Matrix

## Scope

This matrix maps the official public Bifrost API documentation to the implementation-reference code on `oss/ui-restoration`.

Primary evidence order:

1. Official `/api-reference/**` contracts
2. Official `/enterprise/**` screenshots and prose
3. `oss/ui-restoration` handlers and RTK Query slices as implementation-reference only

`ui/lib/store/apis/baseApi.ts` adds the `/api` prefix, so frontend slice URLs like `/rbac/context` resolve to `/api/rbac/context`.

## Status Legend

- `exact public`: the official public API contract is directly represented in `oss/ui-restoration`
- `partial public`: some of the workflow is public, but the enterprise surface depends on extra private contracts or undocumented states
- `private-only`: the restored branch has working contracts, but no dedicated official public `/api-reference/**` group was found
- `no public contract`: the official enterprise docs describe the capability, but no dedicated contract family was found and no stable public equivalent exists

## Family 1: Governance And Routing

| Capability | Official contract status | Public contract anchor | `oss/ui-restoration` backend | `oss/ui-restoration` frontend | Notes |
| --- | --- | --- | --- | --- | --- |
| Routing rules CRUD | `exact public` | `/api/governance/routing-rules` | `transports/bifrost-http/handlers/governance.go` | `ui/lib/store/apis/routingRulesApi.ts` | This is the strongest documented tie-in for enterprise routing behavior. |
| Virtual keys CRUD + quota | `exact public` | `/api/governance/virtual-keys`, `/api/governance/virtual-keys/quota` | `transports/bifrost-http/handlers/governance.go` | `ui/lib/store/apis/governanceApi.ts` | Public docs and restored code align cleanly. |
| Governance teams CRUD | `exact public` | `/api/governance/teams` | `transports/bifrost-http/handlers/governance.go` | `ui/lib/store/apis/governanceApi.ts` | Separate public `/api/teams` docs also exist, but the restored admin UI leans on governance-scoped team management. |
| Customers CRUD | `exact public` | `/api/governance/customers` | `transports/bifrost-http/handlers/governance.go` | `ui/lib/store/apis/governanceApi.ts` | Public governance docs align. |
| Model configs CRUD | `exact public` | `/api/governance/model-configs` | `transports/bifrost-http/handlers/governance.go` | `ui/lib/store/apis/governanceApi.ts` | Public governance docs align. |
| Pricing overrides CRUD | `exact public` | `/api/governance/pricing-overrides` | `transports/bifrost-http/handlers/governance.go` | `ui/lib/store/apis/governanceApi.ts` | Public governance docs align. |
| Provider governance overrides | `exact public` | `/api/governance/providers` | `transports/bifrost-http/handlers/governance.go` | `ui/lib/store/apis/governanceApi.ts` | Public governance docs align. |
| Budgets and rate limits | `partial public` | governance family in `/api-reference/governance/**` | `transports/bifrost-http/handlers/governance.go` | `ui/lib/store/apis/governanceApi.ts` | Restored UI and handlers expose budgets/rate-limits, but the enterprise surface evidence is more narrative than screenshot-backed. |
| Adaptive routing policies | `private-only` | closest public anchor: routing rules | `transports/bifrost-http/handlers/adaptive_routing.go` | `ui/lib/store/apis/adaptiveRoutingApi.ts` | Official public docs do not expose `/api/adaptive-routing/**`. |
| Adaptive routing metrics refresh | `private-only` | none | `transports/bifrost-http/handlers/adaptive_routing.go` | `ui/lib/store/apis/adaptiveRoutingApi.ts` | Restored contract adds `/metrics`, `/metrics/refresh`, and score management endpoints not present in public API docs. |
| Adaptive routing quality scores | `private-only` | none | `transports/bifrost-http/handlers/adaptive_routing.go` | `ui/lib/store/apis/adaptiveRoutingApi.ts` | This is enterprise-surface critical but undocumented publicly. |

## Family 2: Identity, Session, And Governance Membership

| Capability | Official contract status | Public contract anchor | `oss/ui-restoration` backend | `oss/ui-restoration` frontend | Notes |
| --- | --- | --- | --- | --- | --- |
| Session auth enabled/login/logout | `exact public` | `/api/session/is-auth-enabled`, `/api/session/login`, `/api/session/logout` | `transports/bifrost-http/handlers/session.go` | `ui/lib/store/apis/sessionApi.ts` | Clean public alignment. |
| API-key-backed session login | `partial public` | closest public anchor: session login | `transports/bifrost-http/handlers/session.go` | `ui/lib/store/apis/sessionApi.ts` | Restored branch adds `/api/session/api-key-login`, which is broader than the minimum public session docs used in enterprise pages. |
| Users list | `partial public` | `/api/users` | `transports/bifrost-http/handlers/user_groups.go` | `ui/lib/store/apis/userGovernanceApi.ts` | Public docs cover users, but the restored user-governance surface is combined with private group/access-profile behavior. |
| Teams standalone CRUD | `partial public` | `/api/teams` | closest code path is governance teams plus private user-group compatibility | no dedicated standalone RTK slice found for `/api/teams` | Public docs expose standalone teams, but the restored governance UI primarily uses `/api/governance/teams` and `/api/user-groups`. |
| Current user permissions | `partial public` | `/api/users/me/permissions` | restored RBAC context is served from `/api/rbac/context` | `ui/lib/store/apis/rbacApi.ts` | Public docs expose permissions, but the restored UI uses a private RBAC context endpoint instead of only the public `/users/me/permissions` contract. |
| RBAC roles CRUD | `private-only` | none | `transports/bifrost-http/handlers/rbac_handler.go` | `ui/lib/store/apis/rbacApi.ts` | Includes `/api/rbac/roles`, `/api/rbac/permissions`, role-permission binding, and user-role assignment. |
| RBAC permission matrix | `private-only` | none | `transports/bifrost-http/handlers/rbac_handler.go` | `ui/lib/store/apis/rbacApi.ts` | Core enterprise workflow has no dedicated public contract group. |
| SCIM auth type + SSO provider CRUD | `private-only` | none | `transports/bifrost-http/handlers/sso_handler.go` | `ui/lib/store/apis/scimApi.ts` | Includes `/api/scim/auth-type` and `/api/sso/providers/**`. |
| SCIM user sync/admin actions | `private-only` | none | `transports/bifrost-http/handlers/sso_handler.go` | `ui/lib/store/apis/scimApi.ts` | Includes `/api/sso/users/**` plus activate/deactivate flows. |
| Business units compatibility view | `private-only` | none | `transports/bifrost-http/handlers/user_groups.go` | `ui/lib/store/apis/userGovernanceApi.ts` | Restored via `/api/user-groups`, not via a public business-unit contract. |
| Access profiles compatibility view | `private-only` | none | `transports/bifrost-http/handlers/user_groups.go` | `ui/lib/store/apis/accessProfileApi.ts` | Exposed as `/api/access-profiles` and `/api/users/{user_id}/access-profiles`. |
| User groups | `private-only` | none | `transports/bifrost-http/handlers/user_groups.go` | no dedicated official-public slice family | Restored via `/api/user-groups/**` and used as the compatibility substrate for several governance surfaces. |

## Family 3: OAuth, MCP, And Federated Auth

| Capability | Official contract status | Public contract anchor | `oss/ui-restoration` backend | `oss/ui-restoration` frontend | Notes |
| --- | --- | --- | --- | --- | --- |
| MCP client registry CRUD | `exact public` | `/api/mcp/clients`, `/api/mcp/client`, `/api/mcp/client/{id}` | `transports/bifrost-http/handlers/mcp.go` | `ui/lib/store/apis/mcpApi.ts` | Strong alignment. |
| MCP client reconnect / complete OAuth | `exact public` | `/api/mcp/client/{id}/reconnect`, `/api/mcp/client/{id}/complete-oauth` | `transports/bifrost-http/handlers/mcp.go` | `ui/lib/store/apis/mcpApi.ts` | Public docs align. |
| OAuth callback + config status | `exact public` | `/api/oauth/callback`, `/api/oauth/config/{id}/status` | `transports/bifrost-http/handlers/oauth2.go` | `ui/lib/store/apis/mcpApi.ts` | Used directly by MCP/OAuth flows. |
| Per-user OAuth authorize/register/token | `exact public` | `/api/oauth/per-user/authorize`, `/api/oauth/per-user/register`, `/api/oauth/per-user/token` | `transports/bifrost-http/handlers/oauth2_per_user.go` | `ui/lib/store/apis/mcpApi.ts` | Strong public alignment. |
| Per-user upstream authorize | `exact public` | `/api/oauth/per-user/upstream/authorize` | `transports/bifrost-http/handlers/oauth2_per_user.go` | `ui/lib/store/apis/mcpApi.ts` | Public docs align. |
| MCP auth configuration page | `partial public` | public MCP + OAuth families, but no dedicated `mcp-auth-config` group | assembled from `mcp.go`, `oauth2.go`, `oauth2_per_user.go` | `ui/app/workspace/mcp-auth-config/mcpAuthConfigView.tsx` | The restored page is a summary/redirect surface, not a first-class official public contract family. |
| MCP tool groups | `private-only` | none | `transports/bifrost-http/handlers/mcp_groups.go` | `ui/lib/store/apis/mcpToolGroupsApi.ts` | Includes membership and user-group binding flows under `/api/mcp-tool-groups/**`. |

## Family 4: Logging, Audit, And Export-Adjacent Surfaces

| Capability | Official contract status | Public contract anchor | `oss/ui-restoration` backend | `oss/ui-restoration` frontend | Notes |
| --- | --- | --- | --- | --- | --- |
| Request logs, stats, histograms | `exact public` | `/api/logs`, `/api/logs/stats`, `/api/logs/histogram/**` | `transports/bifrost-http/handlers/logging.go` | `ui/lib/store/apis/logsApi.ts` | Strong public alignment. |
| MCP logs, stats, histograms | `exact public` | `/api/mcp-logs`, `/api/mcp-logs/stats`, `/api/mcp-logs/histogram/**` | `transports/bifrost-http/handlers/logging.go` | `ui/lib/store/apis/mcpLogsApi.ts` | Strong public alignment. |
| Immutable audit log query + verify chain | `private-only` | none | `transports/bifrost-http/handlers/audit_handler.go` | `ui/lib/store/apis/auditLogsApi.ts` | Restored via `/api/audit/logs` and `/api/audit/verify`; no dedicated public audit contract exists. |
| Log exports | `no public contract` | none | no dedicated route family found in the restored branch beyond generic connectors | no dedicated slice family found | Official enterprise docs describe the capability, but neither the public docs nor the restored branch expose a stable dedicated export contract family. |

## Family 5: Providers, Plugins, Connectors, And Platform Admin

| Capability | Official contract status | Public contract anchor | `oss/ui-restoration` backend | `oss/ui-restoration` frontend | Notes |
| --- | --- | --- | --- | --- | --- |
| Providers CRUD + keys | `exact public` | `/api/providers/**`, `/api/keys`, `/api/models/**` | `transports/bifrost-http/handlers/providers.go` | `ui/lib/store/apis/providersApi.ts` | Strong public alignment. |
| Plugins CRUD | `exact public` | `/api/plugins/**` | `transports/bifrost-http/handlers/plugins.go` | `ui/lib/store/apis/pluginsApi.ts` | Strong public alignment. |
| Generic connector CRUD | `private-only` | none | `transports/bifrost-http/handlers/connectors.go` | `ui/lib/store/apis/connectorsApi.ts` | Datadog and other connector UIs on the restored branch are built on this private generic connector family. |
| Datadog connector surface | `private-only` | closest public anchors: plugins/configuration | `transports/bifrost-http/handlers/connectors.go` | `ui/lib/store/apis/connectorsApi.ts` and `ui/app/workspace/observability/views/plugins/datadogView.tsx` | Official enterprise docs show a dedicated Datadog page, but public API docs do not. |
| Cluster status/drain | `private-only` | none | `transports/bifrost-http/handlers/cluster.go` | `ui/lib/store/apis/clusterApi.ts` | Enterprise docs describe clustering, but no public cluster contract exists. |
| Alert channels | `private-only` | none | `transports/bifrost-http/handlers/alert_channels.go` | `ui/lib/store/apis/alertChannelsApi.ts` | No official enterprise docs page or public contract group was found. |
| Vault config | `private-only` | none | `transports/bifrost-http/handlers/vault.go` | `ui/lib/store/apis/vaultApi.ts` | No official enterprise docs page or public contract group was found. |
| Large payload config | `private-only` | none | `transports/bifrost-http/handlers/payload.go` | `ui/lib/store/apis/largePayloadApi.ts` | No official enterprise docs page or public contract group was found. |
| Admin API keys | `private-only` | none | `transports/bifrost-http/handlers/admin_api_keys.go` | `ui/lib/store/apis/adminApiKeysApi.ts` | The public docs cover session auth and provider keys, but not a dedicated admin API key contract family. |
| PII redactor | `private-only` | none | `transports/bifrost-http/handlers/pii.go` | `ui/lib/store/apis/piiRedactorApi.ts` | Present on the restored branch, but not in the official enterprise page inventory gathered from `llms.txt`. |

## Practical Reading

1. Official public API docs are sufficient to rebuild the shared admin substrate: session, governance, logging, MCP, OAuth, providers, and plugins.
2. The most important enterprise-visible surfaces remain undocumented publicly and therefore rely on screenshot and prose reconstruction:
   - RBAC
   - SCIM / SSO
   - Guardrails
   - Adaptive Routing
   - Datadog connector
   - Audit logs
   - Cluster admin
3. `oss/ui-restoration` already demonstrates workable private route families for those modules, but those route families must be treated as implementation-reference only, not as evidence of the upstream official contract.
