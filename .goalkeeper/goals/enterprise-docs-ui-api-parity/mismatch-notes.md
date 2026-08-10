# UI And API Mismatch Notes

## Reading Rules

- `confirmed divergence`: visible mismatch between official evidence and the current restored OSS surface
- `confirmed contract drift`: current restored API shape differs from or exceeds what is publicly documented
- `inference boundary`: the official docs do not provide enough evidence to lock the design or contract down fully

## Confirmed Surface Divergence

### 1. RBAC is officially a list-plus-editor surface, but the restored page is a mixed admin workbench

- Official screenshot evidence:
  - `assets/rbac-list.png`
  - `assets/rbac-edit-role.png`
- Official pattern:
  - role table first
  - green primary CTA
  - edit happens in a side drawer / side editor
- Restored page behavior:
  - `ui/app/workspace/governance/rbac/rbacView.tsx`
  - mixes role list, role creation, permission editing, and user-role assignment in one card-based page
  - includes OSS-specific seeded-admin copy that does not appear in the official screenshots
- Conclusion:
  - `confirmed divergence`

### 2. SCIM/User Provisioning officially uses a provider selector plus deep mapping/import workflows, but the restored page is a simplified provider CRUD view

- Official screenshot evidence:
  - `assets/official/scim-overview.jpg`
  - `assets/official/scim-attribute-mapping.jpg`
  - `assets/official/scim-import-preview.jpg`
- Official pattern:
  - left provider rail
  - large right-side provider form
  - dedicated mapping cards for roles, teams, and business units
  - modal-based import/sync flow with filters
- Restored page behavior:
  - `ui/app/workspace/scim/scimView.tsx`
  - top card declares itself an `OSS compatibility view`
  - create-provider form and configured-provider table are the primary UI
  - no screenshot-matching attribute-mapping composer
  - no screenshot-matching import preview wizard
- Conclusion:
  - `confirmed divergence`

### 3. Guardrails officially centers on a rules table and a rich slide-over rule builder, but the restored UI is split into generic configuration and provider cards

- Official screenshot evidence:
  - `assets/official/guardrails-overview.jpg`
  - `assets/official/query-creation.jpg`
  - `assets/official/cel-rule-builder.jpg`
- Official pattern:
  - rule list as the landing surface
  - `Add New Rule` CTA in the page header
  - full-height right slide-over with CEL-focused rule builder
  - structured fields for apply direction, profiles, sampling, timeout, builder clauses, and CEL preview
- Restored page behavior:
  - `ui/app/workspace/guardrails/configuration/guardrailsConfigurationView.tsx`
  - `ui/app/workspace/guardrails/providers/guardrailsProviderView.tsx`
  - explicit `First-pass OSS scope` copy
  - local keyword/regex rule handling and dry-run tooling are emphasized more than the official enterprise visual language
- Conclusion:
  - `confirmed divergence`

### 4. Adaptive Routing officially looks like an operations dashboard, but the restored UI is primarily a policy-management form

- Official screenshot evidence:
  - `assets/ui-load-balancing.png`
- Official pattern:
  - live metrics summary
  - traffic-distribution table
  - direction-weight and route-weight performance tables
  - provider/model filter controls repeated per section
- Restored page behavior:
  - `ui/app/workspace/adaptive-routing/adaptiveRoutingView.tsx`
  - policy CRUD and quality-score editing are central
  - page copy explicitly says `First-pass OSS scope`
  - dashboard-style distribution and weight views are not the dominant information hierarchy
- Conclusion:
  - `confirmed divergence`

### 5. Datadog officially has a dedicated connector form, but the restored page is a generic connector wrapper

- Official screenshot evidence:
  - `assets/official/dd-config-page.jpg`
  - `assets/official/dd-mode.jpg`
  - `assets/official/dd-llmobs.jpg`
  - `assets/official/dd-trace.jpg`
- Official pattern:
  - provider selector on the left
  - Datadog-specific right-side form
  - dedicated toggles and fields for LLM Observability, connection mode, service metadata, agent/http transport, and tag rows
- Restored page behavior:
  - `ui/app/workspace/observability/views/plugins/datadogView.tsx`
  - wraps a generic `ConnectorConfigView`
  - explicit `first-pass OSS Datadog connector` copy
- Conclusion:
  - `confirmed divergence`

### 6. MCP Auth Config is only weakly evidenced officially, and the restored page is a summary surface rather than a dedicated enterprise editor

- Official evidence:
  - `enterprise/mcp-with-fa.md` prose
  - public MCP and OAuth API contracts
- Restored page behavior:
  - `ui/app/workspace/mcp-auth-config/mcpAuthConfigView.tsx`
  - lists counts of auth-enabled clients and pushes editing back into MCP Registry
  - explicit OSS-branch framing
- Conclusion:
  - `confirmed divergence`
  - confidence is lower than RBAC/SCIM/Guardrails/Adaptive Routing because the official docs do not expose a screenshot-backed MCP auth admin page

### 7. Audit Logs and Cluster remain narrowed OSS restorations rather than screenshot-matched enterprise pages

- Official evidence:
  - `enterprise/audit-logs.md`
  - `enterprise/clustering.md`
- Restored page behavior:
  - `ui/app/workspace/audit-logs/auditLogsView.tsx`
  - `ui/app/workspace/cluster/clusterView.tsx`
  - both pages explicitly describe narrowed OSS scope
- Conclusion:
  - `confirmed divergence`
  - mostly contract and scope drift, because the official docs do not provide in-product screenshots for either surface

## Confirmed Contract Drift

### 1. Public docs cover the shared admin substrate, but enterprise pages depend on undocumented private route families

Undocumented route families present on `oss/ui-restoration`:

- `/api/rbac/**`
- `/api/scim/auth-type`
- `/api/sso/**`
- `/api/guardrails/**`
- `/api/adaptive-routing/**`
- `/api/cluster/**`
- `/api/connectors/**`
- `/api/audit/**`
- `/api/mcp-tool-groups/**`
- `/api/access-profiles`
- `/api/user-groups/**`

Conclusion:

- `confirmed contract drift`

### 2. The restored governance-membership model uses `user_groups` as a compatibility substrate

- Official docs discuss users, teams, business units, access profiles, and SCIM outcomes as user-facing concepts.
- Restored implementation routes several of those surfaces through `/api/user-groups/**` and `/api/access-profiles`.
- That is a practical OSS restoration choice, but it is not an officially documented enterprise contract family.

Conclusion:

- `confirmed contract drift`

### 3. Datadog and other connectors are normalized into a generic `/api/connectors/**` contract on the restored branch

- Official Datadog screenshots present a dedicated Datadog configuration surface.
- The restored branch maps that surface to a generic connector CRUD family.

Conclusion:

- `confirmed contract drift`

## Inference Boundaries

### 1. No official public contract exists for several screenshot-backed surfaces

This affects:

- RBAC role CRUD and permission binding
- SCIM provider config and provisioning sync
- Guardrails policies, rules, and test tooling
- Adaptive routing metrics and quality scores
- Datadog connector management
- Audit log verification

Implication:

- The next implementation goal must preserve the distinction between:
  - public contracts that can be matched exactly
  - private restored contracts that are only branch evidence

### 2. Several current OSS fallback surfaces have no official enterprise doc support in the gathered source set

No official enterprise page was found for:

- alert channels
- BigQuery connector
- large payload settings
- MCP tool groups
- PII redactor
- prompt deployments
- user rankings
- vault config

Implication:

- These surfaces should not be treated as confirmed enterprise parity targets unless new official evidence is discovered.

## Practical Instruction For The Next Goal

1. Rebuild screenshot-backed surfaces to match official structure first, not the current restored OSS card layouts.
2. Keep public contract reuse where it already aligns cleanly:
   - governance
   - logs
   - MCP
   - OAuth
   - session
   - providers
   - plugins
3. Treat the private `oss/ui-restoration` route families as temporary implementation scaffolding until each module is reconciled against the fact standard defined by the docs package.
