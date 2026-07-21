# Public API Inventory For Enterprise Surface Mapping

## Scope

This inventory is limited to official public API docs discovered from:

- `https://docs.getbifrost.ai/llms.txt`
- `https://docs.getbifrost.ai/api-reference/**`

The goal here is not to restate every public API in Bifrost. It is to identify the public contracts that are directly relevant to enterprise surface reconstruction, and to identify enterprise surfaces with no matching public API documentation.

`oss/ui-restoration` is used only to map likely handler and RTK Query ownership after the official contract is identified.

## Family 1: Governance And Routing Contracts

Official coverage:

| Surface family | Official contract coverage | Representative method/path | Likely `oss/ui-restoration` ownership |
| --- | --- | --- | --- |
| Core governance collections | customers, model configs, pricing overrides, provider governance, routing rules, teams, virtual keys | `GET/POST /api/governance/routing-rules`, `GET/PUT/DELETE /api/governance/routing-rules/{rule_id}` | `transports/bifrost-http/handlers/governance.go`, `ui/lib/store/apis/governanceApi.ts`, `routingRulesApi.ts` |
| Adaptive routing tie-in | only routing-rule contracts are public; no public adaptive score/metric contract found | `GET /api/governance/routing-rules`, `POST /api/governance/routing-rules` | `transports/bifrost-http/handlers/adaptive_routing.go`, `ui/lib/store/apis/adaptiveRoutingApi.ts` |
| Team CRUD in governance namespace | governance docs use `/api/governance/teams` while general team docs also exist separately | `GET/POST /api/governance/teams`, `GET/PUT/DELETE /api/governance/teams/{team_id}` | `transports/bifrost-http/handlers/governance.go`, `ui/lib/store/apis/governanceApi.ts` |
| Virtual keys | full CRUD plus self-service quota | `GET/POST /api/governance/virtual-keys`, `GET /api/governance/virtual-keys/quota` | `transports/bifrost-http/handlers/governance.go`, `ui/lib/store/apis/governanceApi.ts` |

Enterprise-surface implications:

1. `adaptive-load-balancing.md` has only partial public API support. Routing rules are documented; adaptive weights, performance metrics, route quality scores, and load-balancer diagnostics are not.
2. `advanced-governance.md` is only partially backed by public contracts. Core governance entities are public; enterprise identity/compliance overlays are not.

## Family 2: Identity, Session, And User Management Contracts

Official coverage:

| Surface family | Official contract coverage | Representative method/path | Likely `oss/ui-restoration` ownership |
| --- | --- | --- | --- |
| Session auth | auth enabled, login, logout, websocket ticket | `GET /api/session/is-auth-enabled`, `POST /api/session/login`, `POST /api/session/logout` | `transports/bifrost-http/handlers/session.go`, `ui/lib/store/apis/sessionApi.ts` |
| Users | user CRUD, role assignment, team assignments, current-user permissions | `GET/POST /api/users`, `PUT /api/users/{id}/role`, `GET /api/users/me/permissions` | `transports/bifrost-http/handlers/rbac_handler.go`, `sso_handler.go`, `ui/lib/store/apis/userGovernanceApi.ts`, `rbacApi.ts` |
| Teams | standalone team CRUD and membership | `GET/POST /api/teams`, `POST /api/teams/{id}/members`, `DELETE /api/teams/{id}/members/{userId}` | `transports/bifrost-http/handlers/governance.go`, `ui/lib/store/apis/userGovernanceApi.ts` |
| OAuth and per-user OAuth | per-user OAuth authorize/register/token/callback plus upstream proxy and status | `GET /api/oauth/per-user/authorize`, `POST /api/oauth/per-user/register`, `POST /api/oauth/per-user/token` | `transports/bifrost-http/handlers/mcp.go`, `session.go`, `ui/lib/store/apis/mcpApi.ts`, `sessionApi.ts` |

Enterprise-surface implications:

1. `rbac.md` has partial public API support through `users/get-current-user-permissions` and role assignment, but no public `/api-reference/rbac/**` section for role CRUD or permission matrices.
2. `user-provisioning.md` and provider setup guides have no public `/api-reference/scim/**` or `/api-reference/sso/**` section despite extensive official UI/docs coverage.
3. `advanced-governance.md` claims OIDC and user-level governance capabilities whose dedicated config contracts are not public in `/api-reference`.

## Family 3: Logging, Audit, And Export-Adjacent Contracts

Official coverage:

| Surface family | Official contract coverage | Representative method/path | Likely `oss/ui-restoration` ownership |
| --- | --- | --- | --- |
| Request logs | log search, single entry fetch, statistics, histograms, delete, recalculation | `GET /api/logs`, `GET /api/logs/stats`, `GET /api/logs/histogram/latency`, `POST /api/logs/recalculate-cost` | `transports/bifrost-http/handlers/logging.go`, `ui/lib/store/apis/logsApi.ts` |
| MCP logs | MCP tool logs, filters, stats, delete | `GET /api/mcp-logs`, `GET /api/mcp-logs/stats`, `DELETE /api/mcp-logs` | `transports/bifrost-http/handlers/logging.go`, `ui/lib/store/apis/mcpLogsApi.ts` |

Enterprise-surface implications:

1. `audit-logs.md` is not backed by a public dedicated audit-log contract. The official public docs only expose request-log and MCP-log APIs.
2. `log-exports.md` has no public export scheduler or destination-management contract in `/api-reference`.
3. Datadog, SIEM, and export connectors are not exposed as dedicated public API reference groups.

## Family 4: MCP, Federated Auth, And Tooling Contracts

Official coverage:

| Surface family | Official contract coverage | Representative method/path | Likely `oss/ui-restoration` ownership |
| --- | --- | --- | --- |
| MCP client registry | client create/list/edit/remove/reconnect/complete OAuth | `POST /api/mcp/client`, `GET /api/mcp/clients`, `PUT /api/mcp/client/{id}` | `transports/bifrost-http/handlers/mcp.go`, `ui/lib/store/apis/mcpApi.ts` |
| MCP tool execution | runtime MCP tool call | `POST /v1/mcp/tool/execute` | `transports/bifrost-http/handlers/mcpinference.go`, `ui/lib/store/apis/mcpApi.ts` |
| Federated/per-user OAuth | documented under OAuth rather than MCP | `GET /api/oauth/per-user/upstream/authorize`, `GET /api/oauth/config/{id}/status` | `transports/bifrost-http/handlers/mcp.go`, `ui/app/workspace/mcp-registry/**` |

Enterprise-surface implications:

1. `mcp-with-fa.md` is only partially backed by public contracts. MCP client management and per-user OAuth are public, but imported API-to-tool transformation flows are described narratively, not as their own contract family.
2. No public `/api-reference/mcp-tool-groups/**` section was found.

## Family 5: Providers, Plugins, And Connector-Adjacent Contracts

Official coverage:

| Surface family | Official contract coverage | Representative method/path | Likely `oss/ui-restoration` ownership |
| --- | --- | --- | --- |
| Providers | provider CRUD, provider key CRUD, model discovery helpers | `GET/POST /api/providers`, `GET/PUT/DELETE /api/providers/{provider}`, `POST /api/providers/{provider}/keys` | `transports/bifrost-http/handlers/providers.go`, `ui/lib/store/apis/providersApi.ts` |
| Plugins | plugin CRUD and status | `GET/POST /api/plugins`, `GET/PUT/DELETE /api/plugins/{name}` | `transports/bifrost-http/handlers/plugins.go`, `ui/lib/store/apis/pluginsApi.ts` |
| Configuration | overall config and proxy config | `GET/PUT /api/config`, `GET/PUT /api/proxy-config`, `GET /api/version` | `transports/bifrost-http/handlers/config.go`, `ui/lib/store/apis/configApi.ts` |

Enterprise-surface implications:

1. `custom-plugins.md` does align to public plugin CRUD docs, but the official enterprise page is service-oriented and does not document a richer enterprise-only plugin contract.
2. `datadog-connector.md` has no public Datadog-specific contract group in `/api-reference`; the nearest public contracts are generic plugin/config/provider surfaces.
3. `log-exports.md` also lacks dedicated public contracts despite being operationally adjacent to observability connectors.

## Public API Gaps By Enterprise Surface

The following enterprise surfaces were found in official `/enterprise/**` docs but did **not** have a matching dedicated public `/api-reference/**` group:

| Enterprise surface | Public API gap status | Closest public contract family |
| --- | --- | --- |
| RBAC roles and permission matrix management | no dedicated public contract found | `users`, partial governance |
| SCIM / SSO provider configuration and provisioning preview/import | no dedicated public contract found | `session`, `users`, `oauth` |
| Immutable audit logs | no dedicated public contract found | `logging` request logs |
| Guardrails rules/providers/profiles | no dedicated public contract found | none |
| Adaptive load-balancing metrics and weight management | no dedicated public contract found | `governance` routing rules |
| Clustering admin/status | no dedicated public contract found | none |
| Datadog connector management | no dedicated public contract found | `plugins`, `configuration` |
| Log export scheduling and destinations | no dedicated public contract found | `logging`, `plugins`, `configuration` |
| MCP tool groups | no dedicated public contract found | `mcp`, `users`, `teams` |
| Access profiles | no dedicated public contract found | `users` |
| Business units | no dedicated public contract found | `users`, `teams`, governance |
| Alert channels | no dedicated public contract found | none |
| PII redactor | no dedicated public contract found | none |
| Vault config | no dedicated public contract found | `configuration` |
| Large payload settings | no dedicated public contract found | `configuration` |

## Practical Reading Of The Public Contract Surface

1. Official public API docs strongly cover general admin/gateway management.
2. Official enterprise docs strongly cover UI and operator workflows for several enterprise-only modules.
3. The gap between those two sources is itself a key fact for the next implementation goal: many enterprise screens must be reconstructed from screenshots and prose because their public API contracts are missing or only partially exposed.
