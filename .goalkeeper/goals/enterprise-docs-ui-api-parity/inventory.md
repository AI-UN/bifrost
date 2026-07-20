# Enterprise Surface Inventory

## Scope

This inventory covers every enterprise page listed in the official docs index:

- Source index: `https://docs.getbifrost.ai/llms.txt`
- Enterprise scope: `https://docs.getbifrost.ai/enterprise/**`

Evidence tags are limited to:

- `screenshot`: page includes a UI screenshot or screen-like image
- `doc text`: page contains descriptive prose that can inform the surface
- `api contract`: a relevant public `/api-reference/**` contract exists
- `inference`: OSS mapping requires interpretation beyond directly stated official docs

`oss/ui-restoration` is used only as an implementation-reference branch for current OSS surface comparison.

## Governance And Identity

| Enterprise doc | Evidence tags | Official media on page | Local asset status | Current OSS surface / fallback surface | `oss/ui-restoration` reference | Notes |
| --- | --- | --- | --- | --- | --- | --- |
| `advanced-governance.md` | `doc text`, `api contract`, `inference` | none | none | Primary OSS surfaces: `/workspace/governance`, `/workspace/governance/virtual-keys`, `/workspace/governance/teams`, `/workspace/governance/customers`, `/workspace/config/api-keys`; fallback surfaces: `ui/app/_fallbacks/enterprise/components/access-profiles/*`, `user-groups/*`, `api-keys/*` | `ui/app/workspace/governance/**`, `ui/app/workspace/config/api-keys/**` | Official page describes enterprise extensions on top of core governance, not a single dedicated UI page. OSS mapping is therefore partly inferred from named capabilities. |
| `rbac.md` | `screenshot`, `doc text`, `api contract` | `media/rbac/rbac-list.png`, `media/rbac/rbac-edit-role.png` | local: `assets/rbac-list.png`, `assets/rbac-edit-role.png` | `/workspace/governance/rbac`; fallback surface: `ui/app/_fallbacks/enterprise/components/rbac/rbacView.tsx` | `ui/app/workspace/governance/rbac/rbacView.tsx` | Official page explicitly says navigation path is `Governance -> Roles & Permissions`. |
| `user-provisioning.md` | `screenshot`, `doc text`, `inference` | `media/user-provisioning/scim-overview.png`, `scim-flow.png`, `scim-provider-select.png`, `scim-attribute-mapping.png`, `scim-import-preview.png` | local: matching `.jpg`/`.avif` assets in `assets/official/` | `/workspace/scim`; adjacent OSS governance surfaces: `/workspace/governance/users`, `/workspace/governance/teams`, `/workspace/governance/business-units`, `/workspace/governance/access-profiles`; fallback surfaces: `ui/app/_fallbacks/enterprise/components/scim/*`, `user-groups/*`, `access-profiles/*` | `ui/app/workspace/scim/scimView.tsx`, `ui/app/workspace/governance/users/usersView.tsx`, `ui/app/workspace/governance/business-units/businessUnitsView.tsx`, `ui/app/workspace/governance/access-profiles/accessProfilesView.tsx` | Official page states all providers share one Bifrost configuration surface; user/team/BU effects extend beyond the SCIM page itself. |
| `setting-up-okta.md` | `screenshot`, `doc text`, `inference` | multiple `media/user-provisioning/okta-*.png` images plus shared mapping screenshots | no local copies in repo | `/workspace/scim` provider-selection and provider-form surface | `ui/app/workspace/scim/scimView.tsx` | Official setup guide is provider-specific evidence for the Okta variant of the shared SCIM surface. |
| `setting-up-entra.md` | `screenshot`, `doc text`, `inference` | multiple `media/user-provisioning/entra-*.png` images plus shared mapping screenshots | no local copies in repo | `/workspace/scim` provider-selection and provider-form surface | `ui/app/workspace/scim/scimView.tsx` | Same shared surface as user provisioning; evidence is provider-specific field and setup flow. |
| `setting-up-google-workspace.md` | `screenshot`, `doc text`, `inference` | multiple `media/user-provisioning/gws-*.png` images plus shared mapping screenshots | no local copies in repo | `/workspace/scim` provider-selection and provider-form surface | `ui/app/workspace/scim/scimView.tsx` | Provider-specific evidence only. |
| `setting-up-keycloak.md` | `screenshot`, `doc text`, `inference` | multiple `media/user-provisioning/keycloak-*.png` images | no local copies in repo | `/workspace/scim` provider-selection and provider-form surface | `ui/app/workspace/scim/scimView.tsx` | Provider-specific evidence only. |
| `setting-up-zitadel.md` | `screenshot`, `doc text`, `inference` | multiple `media/user-provisioning/zitadel-*.png` images plus shared mapping screenshots | no local copies in repo | `/workspace/scim` provider-selection and provider-form surface | `ui/app/workspace/scim/scimView.tsx` | Provider-specific evidence only. |
| `audit-logs.md` | `doc text`, `inference` | none | none | `/workspace/audit-logs`; fallback surface: `ui/app/_fallbacks/enterprise/components/audit-logs/auditLogsView.tsx` | `ui/app/workspace/audit-logs/auditLogsView.tsx` | Official page describes immutable security/compliance audit trails. Public API docs expose request logs, but no dedicated public audit-log contract was found. |

## Routing, Policy, And Observability

| Enterprise doc | Evidence tags | Official media on page | Local asset status | Current OSS surface / fallback surface | `oss/ui-restoration` reference | Notes |
| --- | --- | --- | --- | --- | --- | --- |
| `adaptive-load-balancing.md` | `screenshot`, `doc text`, `api contract`, `inference` | `media/ui-load-balancing.png` | local: `assets/ui-load-balancing.png` | `/workspace/adaptive-routing`; fallback surface: `ui/app/_fallbacks/enterprise/components/adaptive-routing/adaptiveRoutingView.tsx` | `ui/app/workspace/adaptive-routing/adaptiveRoutingView.tsx` | Official text is about adaptive load balancing at provider/key-selection level. Public API docs only confirm governance routing-rule contracts, not the full adaptive metrics surface, so part of the OSS mapping remains inference. |
| `guardrails.md` | `screenshot`, `doc text`, `inference` | `media/guardrails/guardrails-overview.png`, `query-creation.png`, `cel-rule-builder.png`, `guardrails-rule-list-2.png`, `provider-aws-create.png` | local: matching `.jpg`/`.avif` assets in `assets/official/` | `/workspace/guardrails`, `/workspace/guardrails/configuration`, `/workspace/guardrails/providers`; fallback surfaces: `ui/app/_fallbacks/enterprise/components/guardrails/*` | `ui/app/workspace/guardrails/**` | Official page explicitly names `Configuration` and `Providers` as separate dashboard pages. No matching public `/api-reference/guardrails/**` section was found. |
| `datadog-connector.md` | `screenshot`, `doc text`, `inference` | `media/dd-config-page.png`, `dd-llmobs.png`, `dd-mode.png`, `dd-trace.png` | local: matching `.jpg`/`.avif` assets in `assets/official/` | Datadog surface under `/workspace/observability`; fallback surface: `ui/app/_fallbacks/enterprise/components/data-connectors/datadog/datadogConnectorView.tsx` | `ui/app/workspace/observability/views/plugins/datadogView.tsx` | Public API docs cover generic plugin/config/provider surfaces, but no Datadog-specific `/api-reference` section was found. |
| `log-exports.md` | `doc text`, `inference` | none | none | Closest OSS surfaces are `/workspace/observability` connector/export tabs; fallback surface likely overlaps data connectors, not a single dedicated page | `ui/app/workspace/observability/views/plugins/connectorConfigView.tsx`, `bigqueryView.tsx`, `datadogView.tsx` | Official page is destination-oriented and appears broader than the existing OSS observability connector tabs. Mapping is inferred. |

## MCP, Plugins, And Related Admin Surfaces

| Enterprise doc | Evidence tags | Official media on page | Local asset status | Current OSS surface / fallback surface | `oss/ui-restoration` reference | Notes |
| --- | --- | --- | --- | --- | --- | --- |
| `mcp-with-fa.md` | `doc text`, `api contract`, `inference` | none | none | `/workspace/mcp-auth-config`, plus `/workspace/mcp-registry`; fallback surface: `ui/app/_fallbacks/enterprise/components/mcp-auth-config/mcpAuthConfigView.tsx` | `ui/app/workspace/mcp-auth-config/mcpAuthConfigView.tsx`, `ui/app/workspace/mcp-registry/**` | Official page is about turning private APIs into MCP tools with federated auth. Public API docs cover MCP client management and OAuth/per-user OAuth, but no single page mirrors the entire enterprise narrative. |
| `custom-plugins.md` | `doc text`, `api contract`, `inference` | none | none | `/workspace/plugins` | `ui/app/workspace/plugins/views/pluginsView.tsx` | Official page is service-oriented rather than UI-oriented. Public plugin CRUD contracts exist, but no official screenshot evidence was found. |
| `overview.md` | `doc text`, `inference` | `media/architecture.png` | none | Cross-cutting; spans governance, observability, SCIM, guardrails, clustering, and MCP surfaces | multiple `ui/app/workspace/**` routes | Official image is architecture-oriented rather than a dashboard screenshot. Treat as scope/context evidence, not layout evidence. |
| `migration-guides/v1.4.0.md` | `doc text` | none | none | no direct OSS UI surface | none | Version migration guidance only; not a surface reconstruction source. |

## Platform, Infra, And Commercial Context Pages

| Enterprise doc | Evidence tags | Official media on page | Local asset status | Current OSS surface / fallback surface | `oss/ui-restoration` reference | Notes |
| --- | --- | --- | --- | --- | --- | --- |
| `clustering.md` | `doc text`, `inference` | none | none | `/workspace/cluster`; fallback surface: `ui/app/_fallbacks/enterprise/components/cluster/clusterView.tsx` | `ui/app/workspace/cluster/clusterView.tsx` | Official page is architecture-heavy and does not include a dashboard screenshot. No public `/api-reference/cluster/**` section was found. |
| `invpc-deployments.md` | `doc text` | none | none | no direct OSS workspace surface found | none | Commercial deployment/offering page, not an in-product dashboard surface. |
| `release-cadence.md` | `doc text` | none | none | no direct OSS workspace surface found | none | Commercial/support policy page, not a product surface. |

## Inventory Findings

1. Screenshot-backed, high-value UI surfaces are concentrated in `rbac`, `user-provisioning`, `adaptive-load-balancing`, `guardrails`, `datadog-connector`, and the provider-setup guides.
2. Several enterprise pages are descriptive but not UI-specific: `custom-plugins`, `clustering`, `audit-logs`, `log-exports`, `invpc-deployments`, `release-cadence`.
3. `overview.md` contains architecture evidence, not dashboard-layout evidence.
4. Official public API coverage exists for governance, logging, MCP, OAuth, session, providers, plugins, teams, and users, but not for many enterprise-only management surfaces. Those absences are recorded in `api-inventory.md`.
