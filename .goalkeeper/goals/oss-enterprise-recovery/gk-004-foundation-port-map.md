# GK-004 Foundation Port Map

Date: `2026-05-07`

Scope:
- Decide which shared schema, migration, and configstore primitives belong in the first foundation patch.
- Compare current-main `framework/configstore` against the `pr-2565` additions.
- Separate "port now", "defer", and "do not port" decisions.

## Current-Main Baseline

Important facts from the current OSS tree:

| Area | Current state | Implication |
|---|---|---|
| `framework/configstore/store.go` | Large monolithic interface already exists for providers, governance, MCP, sessions, prompts, pricing, and auth | Foundation work should extend the existing store incrementally, not replace it |
| `framework/configstore/migrations.go` | Mature migration stack already exists with many feature-specific steps | New migrations should be appended surgically, not replayed from the PR wholesale |
| `framework/configstore/tables/` | Current main already has core governance tables such as teams, customers, virtual keys, MCP clients, routing rules, sessions | Early enterprise restoration should build on existing governance tables where possible |
| `transports/config.schema.json` | Already contains enterprise-shaped blocks such as `guardrails_config`, `audit_logs`, `cluster_config`, `large_payload_optimization`, `access_profiles`, and `mcp_tool_groups` | Foundation does not need a broad schema-first rewrite; many config surfaces already exist |

## PR Additions Relevant To Foundation

From `pr-2565`, the shared configstore additions fall into three groups:

| Group | PR artifacts |
|---|---|
| Governance and identity tables | `rbac.go`, `rbac_seed.go`, `sso.go`, `user_groups.go`, `tables/rbac.go`, `tables/sso.go`, `tables/user_groups.go` |
| Shared admin/audit/MCP storage | `audit.go`, `audit_writer.go`, `mcp_groups.go`, `tables/audit.go`, `tables/mcp_groups.go` |
| Later-phase runtime storage | `guardrails.go`, `pii.go`, `adaptive_routing.go`, `alerting.go`, `connectors.go` and matching table files |

## Port-Now Set For `oss/foundation`

These additions should be the first shared storage baseline because they unblock the earliest branches and are not tightly coupled to missing runtime plugins.

| Priority | Files or concepts | Why foundation should own it |
|---|---|---|
| P0 | RBAC tables and store methods | Needed before real permission-aware UI or API enforcement can exist |
| P0 | SSO provider and external-user tables | Needed to replace auth-type stubs and prepare SCIM/SSO work |
| P0 | User-group tables | Needed for users, business units, access profiles, and VK-user relationships |
| P0 | Audit log table and append/query store methods | Needed early because it is mostly storage/API work and does not depend on runtime plugins |
| P1 | MCP tool group tables and store methods | Useful shared storage for later MCP governance work, low coupling to runtime engines |

Foundation implementation target:
- add new `framework/configstore/tables/*.go` files only for the port-now set
- add store interface methods only for the port-now set
- add migration steps only for the port-now set
- keep PR naming and shapes only where they fit current main cleanly

## Defer From Foundation

These features should not be included in the first shared-storage patch even if PR files exist.

| Feature area | Why defer |
|---|---|
| Guardrails tables | Runtime enforcement model is unresolved because the PR plugin layer is missing |
| PII tables | Same issue as guardrails; storage alone does not define the real runtime contract |
| Adaptive routing tables | Runtime routing engine is absent from the PR, so table design needs validation against current main routing internals |
| Alerting tables | Dispatch/runtime behavior is missing, so pure storage can wait until trigger sources are defined |
| Connector tables | Connector CRUD without actual connector integration is low-value in the first foundation patch |

## Exclude From Foundation Entirely

| Area | Reason |
|---|---|
| `framework/license/**` | Conflicts with OSS restoration goal |
| handler-level `requireFeature(...)` logic | Same conflict; should not become foundation behavior |
| `framework/cluster/**` | Not a configstore-first concern |
| `framework/vault/**` | Separate platform feature branch |
| `transports/bifrost-http/handlers/payload.go` | Stub-heavy and not shared-storage-first |

## Recommended Foundation Patch Shape

Patch 1 inside `oss/foundation` should aim for this minimum coherent slice:

1. Add table models for:
   - RBAC
   - SSO provider / external user / SSO session
   - user groups and VK/group join tables
   - audit logs
   - MCP tool groups
2. Extend `ConfigStore` with just the methods required by:
   - `oss/rbac`
   - `oss/user`
   - `oss/scim-sso`
   - `oss/audit-logs`
   - `oss/mcp-tool-groups`
3. Implement matching migrations in current-main style inside `framework/configstore/migrations.go`
4. Avoid changing `transports/config.schema.json` unless a follow-on feature branch proves a missing field is truly blocking

## Specific Reuse Guidance

| PR area | Reuse mode | Guidance |
|---|---|---|
| `framework/configstore/tables/rbac.go` | adapt | Likely portable with naming and index review |
| `framework/configstore/tables/sso.go` | adapt | Portable, but SCIM-specific fields should be reviewed against current auth/session model |
| `framework/configstore/tables/user_groups.go` | adapt | Portable, but relation to existing team/customer/VK tables must be checked |
| `framework/configstore/tables/audit.go` | adapt | Portable candidate |
| `framework/configstore/tables/mcp_groups.go` | adapt | Portable candidate |
| `framework/configstore/rbac.go` | adapt | Good source for CRUD/store methods |
| `framework/configstore/sso.go` | adapt carefully | Needs review against current OAuth/session flow |
| `framework/configstore/user_groups.go` | adapt carefully | Must align with existing governance tables and naming |
| `framework/configstore/audit.go` and `audit_writer.go` | adapt | Good source for storage and write-path concepts |
| `framework/configstore/mcp_groups.go` | adapt | Good source for storage/API layer |
| PR migration additions | re-author | Use as checklist only; do not paste wholesale into current `migrations.go` |

## Foundation Acceptance Boundary

`GK-004` does not require shipping code yet. It requires a concrete decision on:
- which shared tables to add first
- which store methods to add first
- which migrations to add first
- which PR pieces must be deferred or excluded

This document now fixes that boundary.

## GK-004 Acceptance Verdict

`complete`

Evidence:
- Current-main `framework/configstore/store.go` and `migrations.go` inspected
- PR configstore and migration diffs inspected
- Current `transports/config.schema.json` searched for already-present enterprise-shaped config blocks

Next task:
- `GK-005` define the license-neutral replacement strategy for reused PR code paths before the first foundation code patch.
