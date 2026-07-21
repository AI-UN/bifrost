# Implementation Matrix

Generated for GK-001. Source of truth: `.goalkeeper/goals/enterprise-docs-ui-api-parity/`.

## Status Legend

- **confidence**: `high` = screenshot + doc text + partial API | `medium` = doc text + inference | `partial-api` = public API exists but UI layout inferred
- **contract-class**: `exact-public` = direct public `/api-reference/**` match | `partial-public` = some public, some private | `private-only` = no public contract family found

---

## Module 1: RBAC — Roles & Permissions

| Field | Value |
|---|---|
| **Implementation order** | 1 |
| **Confidence** | high (screenshot + doc text + partial API) |
| **Status in fallback-parity.md** | `covered` |
| **Fallback surface to replace** | `ui/app/_fallbacks/enterprise/components/rbac/rbacView.tsx` |
| **Current workspace route** | `ui/app/workspace/governance/rbac/rbacView.tsx` (9-line page.tsx delegate), also `ui/app/workspace/rbac/page.tsx` |
| **Backend handler** | `transports/bifrost-http/handlers/rbac_handler.go`, `rbac.go` |
| **Frontend API slice** | `ui/lib/store/apis/rbacApi.ts` |
| **Contract classification** | `private-only` — no public `/api-reference/rbac/**` family |
| **Private contract routes** | `/api/rbac/context`, `/api/rbac/roles`, `/api/rbac/permissions`, role-permission binding, user-role assignment |
| **Public contract anchors** | `/api/users/me/permissions` (closest partial public equivalent) |
| **Official layout target** | Wide role table (Name, Description, Type, Permissions columns) with green "Add Role" CTA top-right. Edit opens right-side slide-over with two-column permission editor: Resources list left, operations right. |
| **Known drift (mismatch-notes.md)** | Current page is a mixed admin workbench (role list + creation + permission editing + user-role assignment in one card page). Official is table + editor split. |
| **Primary research artifacts** | `assets/rbac-list.png`, `assets/rbac-edit-role.png`, `reference-specs/governance-identity.md` Surface A, `design-notes.md` RBAC sections, `api-parity.md` Family 2 |
| **Risk** | `/api/rbac/context` previously caused 401 auth-loop — must not reintroduce |

---

## Module 2: SCIM / User Provisioning

| Field | Value |
|---|---|
| **Implementation order** | 2 |
| **Confidence** | high (screenshot + doc text + inference) |
| **Status in fallback-parity.md** | `covered` |
| **Fallback surface to replace** | `ui/app/_fallbacks/enterprise/components/scim/scimView.tsx` |
| **Current workspace route** | `ui/app/workspace/scim/scimView.tsx` (231 lines) |
| **Backend handler** | `transports/bifrost-http/handlers/sso_handler.go` |
| **Frontend API slice** | `ui/lib/store/apis/scimApi.ts` |
| **Contract classification** | `private-only` |
| **Private contract routes** | `/api/scim/auth-type`, `/api/sso/providers/**`, `/api/sso/users/**` |
| **Public contract anchors** | `/api/session/**`, `/api/oauth/**` (session + OAuth substrate) |
| **Official layout target** | Left provider rail with logos/active status. Right pane: large provider-specific long-form config. Attribute mapping cards for roles, teams, BUs. Sync Users modal with filter checkboxes. |
| **Known drift** | Current page is "OSS compatibility view" with simple provider CRUD table. Missing: screenshot-matching attribute-mapping composer, import preview wizard. |
| **Primary research artifacts** | `assets/official/scim-overview.jpg`, `scim-attribute-mapping.jpg`, `scim-import-preview.jpg`, `reference-specs/governance-identity.md` Surfaces B/C/D, `api-parity.md` Family 2 |
| **Risk** | Provider-specific setup guides (Okta, Entra, GWS, Keycloak, Zitadel) show different field sets — the shared surface must accommodate all |

---

## Module 3: Guardrails — Configuration

| Field | Value |
|---|---|
| **Implementation order** | 3a |
| **Confidence** | high (screenshot + doc text + inference) |
| **Status in fallback-parity.md** | `covered` |
| **Fallback surface to replace** | `ui/app/_fallbacks/enterprise/components/guardrails/guardrailsConfigurationView.tsx` |
| **Current workspace route** | `ui/app/workspace/guardrails/configuration/guardrailsConfigurationView.tsx` (372 lines) |
| **Backend handler** | `transports/bifrost-http/handlers/guardrails.go`, `guardrails_middleware.go` |
| **Frontend API slice** | `ui/lib/store/apis/guardrailsApi.ts` |
| **Contract classification** | `private-only` |
| **Private contract routes** | `/api/guardrails/**` |
| **Public contract anchors** | none dedicated |
| **Official layout target** | Rules table landing (Rule Name, Description, Apply To, Sampling Rate, Status columns). Green "Add New Rule" CTA. Full-height right slide-over drawer with CEL rule builder (mode toggle, Add Rule/Group toolbar, subject/operator/operand rows, expression preview). |
| **Known drift** | Current page is "First-pass OSS scope" with local keyword/regex rule handling and dry-run tooling emphasis instead of official enterprise table + CEL builder. |
| **Primary research artifacts** | `assets/official/guardrails-overview.jpg`, `query-creation.jpg`, `cel-rule-builder.jpg`, `reference-specs/policy-routing.md` Surfaces A/B/C, `design-notes.md` |
| **Risk** | Guardrails middleware exists alongside the handler — must preserve middleware behavior during UI rebuild |

---

## Module 4: Guardrails — Providers

| Field | Value |
|---|---|
| **Implementation order** | 3b |
| **Confidence** | high (screenshot + doc text + inference) |
| **Status in fallback-parity.md** | `covered` |
| **Fallback surface to replace** | `ui/app/_fallbacks/enterprise/components/guardrails/guardrailsProviderView.tsx` |
| **Current workspace route** | `ui/app/workspace/guardrails/providers/guardrailsProviderView.tsx` |
| **Backend handler** | same as Guardrails Configuration |
| **Frontend API slice** | same as Guardrails Configuration |
| **Contract classification** | `private-only` |
| **Official layout target** | Provider/profile management panel — evidence is less detailed than Configuration but the docs explicitly name "Providers" as a separate dashboard page. |
| **Known drift** | Provider nuances still partly inferred. Current page uses generic card layout. |
| **Primary research artifacts** | `assets/official/provider-aws-create.jpg`, `reference-specs/policy-routing.md`, `enterprise/guardrails.md` |
| **Risk** | Lower evidence density than Configuration — keep conservative |

---

## Module 5: Adaptive Routing

| Field | Value |
|---|---|
| **Implementation order** | 4 |
| **Confidence** | high (screenshot + doc text + inference) |
| **Status in fallback-parity.md** | `covered` |
| **Fallback surface to replace** | `ui/app/_fallbacks/enterprise/components/adaptive-routing/adaptiveRoutingView.tsx` |
| **Current workspace route** | `ui/app/workspace/adaptive-routing/adaptiveRoutingView.tsx` (751 lines) |
| **Backend handler** | `transports/bifrost-http/handlers/adaptive_routing.go` |
| **Frontend API slice** | `ui/lib/store/apis/adaptiveRoutingApi.ts` |
| **Contract classification** | `private-only` — no public `/api/adaptive-routing/**` |
| **Private contract routes** | `/api/adaptive-routing/**`, `/api/adaptive-routing/metrics`, `/api/adaptive-routing/metrics/refresh`, quality scores |
| **Public contract anchors** | `/api/governance/routing-rules` (closest anchor) |
| **Official layout target** | Metrics-first dashboard: summary metrics (total requests, success rate), traffic-distribution table, direction-weight and route-weight performance tables. Provider rows with icons. Repeated provider/model filter controls per section. |
| **Known drift** | Current page is policy CRUD + quality-score editing central, with "First-pass OSS scope" copy. Official is operator console first, not CRUD form. |
| **Primary research artifacts** | `assets/ui-load-balancing.png`, `reference-specs/policy-routing.md` Surface D, `design-notes.md`, `enterprise/adaptive-load-balancing.md` |
| **Risk** | Metrics endpoints are private-only — must ensure `/api/adaptive-routing/metrics` and `/refresh` are stable before UI rebuild |

---

## Module 6: Datadog Connector

| Field | Value |
|---|---|
| **Implementation order** | 5 |
| **Confidence** | high (screenshot + doc text + inference) |
| **Status in fallback-parity.md** | `covered` |
| **Fallback surface to replace** | `ui/app/_fallbacks/enterprise/components/data-connectors/datadog/datadogConnectorView.tsx` |
| **Current workspace route** | `ui/app/workspace/observability/views/plugins/datadogView.tsx` (24 lines — thin wrapper) |
| **Backend handler** | `transports/bifrost-http/handlers/connectors.go` |
| **Frontend API slice** | `ui/lib/store/apis/connectorsApi.ts` |
| **Contract classification** | `private-only` |
| **Private contract routes** | `/api/connectors`, `/api/connectors/{id}`, `/api/connectors/{id}/test` |
| **Public contract anchors** | closest: `/api/plugins/**`, `/api/providers/**` |
| **Official layout target** | Left provider rail (Open Telemetry, Maxim, Datadog, New Relic "COMING SOON"). Right pane: Datadog-specific form with service name, LLM Observability toggle, ML app name, connection mode selector, agent/transport address, environment + version two-column row, custom tags repeatable table. |
| **Known drift** | Current page is generic `ConnectorConfigView` wrapper with "first-pass OSS Datadog connector" copy. Official has dedicated Datadog form. |
| **Primary research artifacts** | `assets/official/dd-config-page.jpg`, `dd-mode.jpg`, `dd-llmobs.jpg`, `dd-trace.jpg`, `reference-specs/connectors-observability.md`, `enterprise/datadog-connector.md` |
| **Risk** | API is private generic connectors — the Datadog-specific fields may need new connector schema fields or adapter logic |

---

## Module 7: Audit Logs

| Field | Value |
|---|---|
| **Implementation order** | 6 |
| **Confidence** | medium (doc text + inference, no screenshots) |
| **Status in fallback-parity.md** | `partial` |
| **Fallback surface to replace** | `ui/app/_fallbacks/enterprise/components/audit-logs/auditLogsView.tsx` |
| **Current workspace route** | `ui/app/workspace/audit-logs/auditLogsView.tsx` |
| **Backend handler** | `transports/bifrost-http/handlers/audit_handler.go` |
| **Frontend API slice** | `ui/lib/store/apis/auditLogsApi.ts` |
| **Contract classification** | `private-only` |
| **Private contract routes** | `/api/audit/logs`, `/api/audit/verify` |
| **Public contract anchors** | `/api/logs/**` (request logs — distinct from audit) |
| **Official layout target** | Append-only log table emphasizing actor, action, target, timestamp, verification status. Integrity/chain-verification actions near page header. |
| **Known drift** | Current page explicitly describes narrowed OSS scope. |
| **Primary research artifacts** | `reference-specs/audit-mcp.md` Surface A, `enterprise/audit-logs.md`, `api-parity.md` Family 4 |
| **Risk** | No screenshot evidence — layout must stay conservative. Do not overfit to inferred layout. |

---

## Module 8: Cluster

| Field | Value |
|---|---|
| **Implementation order** | 7 |
| **Confidence** | medium (doc text + inference, no screenshots) |
| **Status in fallback-parity.md** | `partial` |
| **Fallback surface to replace** | `ui/app/_fallbacks/enterprise/components/cluster/clusterView.tsx` |
| **Current workspace route** | `ui/app/workspace/cluster/clusterView.tsx` |
| **Backend handler** | `transports/bifrost-http/handlers/cluster.go` |
| **Frontend API slice** | `ui/lib/store/apis/clusterApi.ts` |
| **Contract classification** | `private-only` |
| **Private contract routes** | `/api/cluster/**` |
| **Public contract anchors** | none |
| **Official layout target** | Architecture-heavy surface. Likely diagnostics-first: cluster status, drain controls, node health. |
| **Known drift** | Current page is narrowed OSS restoration with explicit scope framing. |
| **Primary research artifacts** | `enterprise/clustering.md`, `api-parity.md` Family 5 |
| **Risk** | Lowest evidence density of all in-scope modules — keep very conservative |

---

## Module 9: MCP Auth Config

| Field | Value |
|---|---|
| **Implementation order** | 8 |
| **Confidence** | medium (doc text + public API + inference) |
| **Status in fallback-parity.md** | `partial` |
| **Fallback surface to replace** | `ui/app/_fallbacks/enterprise/components/mcp-auth-config/mcpAuthConfigView.tsx` |
| **Current workspace route** | `ui/app/workspace/mcp-auth-config/mcpAuthConfigView.tsx` |
| **Backend handler** | assembled from `mcp.go`, `oauth2.go`, `oauth2_per_user.go` |
| **Frontend API slice** | `ui/lib/store/apis/mcpApi.ts` (shared with MCP registry) |
| **Contract classification** | `partial-public` — MCP + OAuth public contracts exist but no dedicated mcp-auth-config page |
| **Public contract anchors** | `/api/mcp/**`, `/api/oauth/**`, `/api/oauth/per-user/**` |
| **Private contract routes** | `/api/mcp-tool-groups/**` (if used) |
| **Official layout target** | Admin surface over client auth modes. Counts, status, and entry points into OAuth completion flows. |
| **Known drift** | Current page lists counts of auth-enabled clients and pushes editing back into MCP Registry. Is a summary/redirect surface, not a first-class editor. |
| **Primary research artifacts** | `reference-specs/audit-mcp.md` Surface B, `enterprise/mcp-with-fa.md`, `api-parity.md` Family 3 |
| **Risk** | Don't assume a dedicated config editor unless new screenshot evidence appears |

---

## Module 10: Access Profiles

| Field | Value |
|---|---|
| **Implementation order** | 9a |
| **Confidence** | medium (doc text + inference) |
| **Status in fallback-parity.md** | `partial` |
| **Fallback surface to replace** | `ui/app/_fallbacks/enterprise/components/access-profiles/accessProfilesIndexView.tsx` |
| **Current workspace route** | `ui/app/workspace/governance/access-profiles/accessProfilesView.tsx` |
| **Backend handler** | `transports/bifrost-http/handlers/user_groups.go` |
| **Frontend API slice** | `ui/lib/store/apis/accessProfileApi.ts` |
| **Contract classification** | `private-only` |
| **Private contract routes** | `/api/access-profiles`, `/api/users/{user_id}/access-profiles` |
| **Official layout target** | Mentioned through governance/provisioning outcomes. Cleanup alignment to updated identity/RBAC flows. |
| **Primary research artifacts** | `reference-specs/governance-identity.md`, `enterprise/advanced-governance.md`, `api-parity.md` Family 2 |
| **Risk** | Not screenshot-backed — keep conservative |

---

## Module 11: Users

| Field | Value |
|---|---|
| **Implementation order** | 9b |
| **Confidence** | medium (doc text + inference) |
| **Status in fallback-parity.md** | `partial` |
| **Fallback surface to replace** | `ui/app/_fallbacks/enterprise/components/user-groups/usersView.tsx` |
| **Current workspace route** | `ui/app/workspace/governance/users/usersView.tsx` |
| **Backend handler** | `transports/bifrost-http/handlers/user_groups.go` |
| **Frontend API slice** | `ui/lib/store/apis/userGovernanceApi.ts` |
| **Contract classification** | `partial-public` |
| **Public contract anchors** | `/api/users` |
| **Private contract routes** | `/api/user-groups/**` |
| **Official layout target** | Users table evidenced indirectly via provisioning screenshots. Cleanup alignment. |
| **Primary research artifacts** | `reference-specs/governance-identity.md`, `enterprise/user-provisioning.md`, `api-parity.md` Family 2 |
| **Risk** | Restored governance surface combines private group/access-profile behavior with public users |

---

## Module 12: Teams

| Field | Value |
|---|---|
| **Implementation order** | 9c |
| **Confidence** | medium (doc text + inference) |
| **Status in fallback-parity.md** | `partial` |
| **Fallback surface to replace** | `ui/app/_fallbacks/enterprise/components/user-groups/teamsView.tsx` |
| **Current workspace route** | `ui/app/workspace/governance/teams/teamsView.tsx` |
| **Backend handler** | `transports/bifrost-http/handlers/governance.go` + `user_groups.go` |
| **Frontend API slice** | `ui/lib/store/apis/governanceApi.ts` (governance teams) |
| **Contract classification** | `partial-public` |
| **Public contract anchors** | `/api/governance/teams`, `/api/teams` |
| **Official layout target** | Team concepts are fact-backed; dedicated team page layout is not screenshot-backed. Cleanup alignment. |
| **Primary research artifacts** | `reference-specs/governance-identity.md`, `api-parity.md` Family 1+2 |
| **Risk** | Dual contract paths (governance teams vs standalone teams) — must reconcile |

---

## Module 13: Business Units

| Field | Value |
|---|---|
| **Implementation order** | 9d |
| **Confidence** | medium (doc text + inference) |
| **Status in fallback-parity.md** | `partial` |
| **Fallback surface to replace** | `ui/app/_fallbacks/enterprise/components/user-groups/businessUnitsView.tsx` |
| **Current workspace route** | `ui/app/workspace/governance/business-units/businessUnitsView.tsx` |
| **Backend handler** | `transports/bifrost-http/handlers/user_groups.go` |
| **Frontend API slice** | `ui/lib/store/apis/userGovernanceApi.ts` |
| **Contract classification** | `private-only` |
| **Private contract routes** | `/api/user-groups/**` |
| **Official layout target** | BU mappings visible in SCIM mapping screenshots, but dedicated BU page is inferred. Cleanup alignment. |
| **Primary research artifacts** | `reference-specs/governance-identity.md`, `enterprise/user-provisioning.md` |
| **Risk** | Must not create a layout that conflicts with the SCIM attribute mapping structure |

---

## Module 14: API Keys (governance context)

| Field | Value |
|---|---|
| **Implementation order** | 9e |
| **Confidence** | medium (doc text + inference) |
| **Status in fallback-parity.md** | `partial` |
| **Fallback surface to replace** | `ui/app/_fallbacks/enterprise/components/api-keys/apiKeysIndexView.tsx` |
| **Current workspace route** | `ui/app/workspace/config/api-keys/` (config area, not governance) |
| **Backend handler** | `transports/bifrost-http/handlers/providers.go` (provider keys) |
| **Frontend API slice** | `ui/lib/store/apis/providersApi.ts` |
| **Contract classification** | `partial-public` |
| **Public contract anchors** | `/api/keys`, `/api/providers/**` |
| **Official layout target** | Governance/API key concepts exist. Cleanup alignment. |
| **Primary research artifacts** | `api-parity.md` Family 5, `enterprise/advanced-governance.md` |
| **Risk** | Lowest priority in this group — only touch if governance flow requires it |

---

## Summary: Contract Strategy

### Public contracts to reuse directly (no private alternatives needed)
- `/api/session/**` — session auth, login, logout
- `/api/governance/**` — routing rules, virtual keys, teams, customers, model configs, pricing overrides
- `/api/logs/**`, `/api/mcp-logs/**` — request/MCP logs, stats, histograms
- `/api/mcp/**` — client registry CRUD, reconnect, OAuth
- `/api/oauth/**`, `/api/oauth/per-user/**` — OAuth lifecycle
- `/api/providers/**`, `/api/keys`, `/api/models/**` — provider/key management
- `/api/plugins/**` — plugin CRUD

### Private contracts to carry forward (no public equivalent found)
- `/api/rbac/**` — roles, permissions, context, binding
- `/api/scim/auth-type`, `/api/sso/**` — SSO providers, SCIM sync
- `/api/guardrails/**` — guardrail rules and profiles
- `/api/adaptive-routing/**` — metrics, scores, policies
- `/api/connectors/**` — generic connector CRUD (Datadog surface)
- `/api/audit/**` — audit logs, verification
- `/api/cluster/**` — cluster status/drain
- `/api/user-groups/**` — compatibility substrate for users/teams/BUs
- `/api/access-profiles` — access profile management

### Dual-path contracts (public + private coexist)
- Users: `/api/users` (public) + `/api/user-groups/**` (private)
- Teams: `/api/governance/teams` + `/api/teams` (public) + `/api/user-groups/**` (private)
- Permissions: `/api/users/me/permissions` (public) + `/api/rbac/context` (private)

---

## Out-of-Scope Modules (deferred — `unresolved` in fallback-parity.md)

These modules are explicitly excluded from this implementation pass:

1. Alert Channels — no official enterprise page found
2. BigQuery Connector — not supported by gathered official docs
3. Large Payload Settings — no official enterprise page found
4. Login enterprise template — no dedicated enterprise product page found
5. MCP Tool Groups — private restored module only
6. PII Redactor — private restored module only
7. Prompt Deployments — private restored module only
8. User Rankings — not in official enterprise docs set
