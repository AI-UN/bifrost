# Plan

## Status

`plan-accepted`

## Working Strategy

- Keep an `oss/*` integration branch as the long-lived baseline for this restoration effort; the current integration branch is `oss/foundation` after the user explicitly removed the top-level `oss` branch.
- Before execution, sync the integration baseline to the latest `upstream/main`.
- Create focused feature branches from that current integration baseline, for example:
  - `oss/foundation`
  - `oss/rbac`
  - `oss/user`
  - `oss/scim-sso`
  - `oss/audit-logs`
  - `oss/guardrails`
  - `oss/pii-redactor`
  - `oss/adaptive-routing`
  - `oss/mcp-tool-groups`
  - `oss/alert-channels`
  - `oss/cluster`
  - `oss/vault`
  - `oss/large-payload`
  - `oss/connectors`
  - `oss/ui-restoration`
- Use `PR #2565` for salvageable backend code, tests, and design intent, but do not inherit its enterprise licensing model as-is.

## Phase 1: Inventory And Salvage Map

Purpose:
Build the authoritative gap map for current OSS gating and the authoritative salvage map for `PR #2565`.

Output:
- Route and component inventory for enterprise placeholders and shared `@enterprise/lib` stubs
- A missing-feature to fallback-UI parity matrix that will be reused as the final completion checklist
- Module-by-module PR classification: portable, stale, missing, or incompatible
- Initial feature matrix that drives branch creation order

## Phase 2: Shared Backend Foundation

Purpose:
Establish the common primitives needed by multiple restored features before feature branches diverge too far.

Output:
- Current-main-compatible configstore and migration plan
- Decision on how OSS will enable restored features without enterprise license checks
- Shared schemas, feature toggles, storage tables, and handler wiring patterns

## Phase 3: Governance And Identity Surface

Purpose:
Restore the admin and identity features that already bleed into shared pages and are likely to block the rest of the UI.

Modules:
- RBAC
- Users
- Teams
- Business Units
- Access Profiles
- SCIM / SSO

Output:
- Data model and API surface for user and role management
- Shared auth and permission hooks for backend handlers and frontend screens
- Replacement of no-op access profile and SCIM fallback APIs

## Phase 4: Runtime Policy, Routing, And Observability

Purpose:
Restore the runtime features that act inside request flow, governance, or observability pipelines.

Modules:
- Audit Logs
- Guardrails
- PII Redactor
- Adaptive Routing
- MCP Tool Groups
- Alert Channels
- Data Connectors

Output:
- Handler and plugin implementations restored on current main
- Integration points defined clearly with existing plugin ordering and governance flow
- Frontend management surfaces backed by working APIs

## Phase 5: Platform And Performance Features

Purpose:
Restore the lower-level or infrastructural features that are broader in blast radius and may need pragmatic OSS-first scope decisions.

Modules:
- Clustering
- Vault
- Large Payload Optimization

Output:
- Feasible first-pass OSS implementation boundaries
- Storage, config, and runtime changes that do not destabilize the main request path
- Clear fallback behavior where exact enterprise parity is not practical

## Phase 6: Frontend Restoration, Verification, And Merge

Purpose:
Replace enterprise placeholder and fallback behavior across the workspace, then integrate and validate the full OSS branch.

Output:
- Real page implementations or integrated shared-page fragments replacing placeholder CTAs
- A final parity pass that compares the original demo-only fallback surfaces against the restored OSS feature set and records any remaining gaps explicitly
- Updated E2E and targeted backend verification
- Final merge into the agreed `oss/*` integration branch with successful build and runnable application

## Sequencing Rationale

- Discovery comes first because current OSS gating is spread across both direct placeholder pages and shared page fragments.
- The fallback UI itself is treated as a hard verification source, so the first inventory must be precise enough to support a final one-to-one parity audit.
- Shared backend primitives come before feature branches because the PR's configstore and handler patterns touch multiple modules.
- Governance and identity restore early because they unblock shared UI code paths already consuming fallback enterprise APIs.
- Runtime policy and observability work next because those features mostly sit behind explicit handlers or plugin boundaries.
- Platform features are intentionally later because they carry the highest architectural risk and may need narrowed first-pass scope.
- UI restoration is delayed until enough APIs and schemas exist to replace placeholders with functioning screens rather than new mocks.

## Proposed `/goal` Handoff

When unblocked, execute `.goalkeeper/goals/oss-enterprise-recovery/progress.md` in order, starting at `GK-001`, using the branch strategy above and preserving these rules:
- latest `upstream/main` baseline
- no destructive reset of local user changes
- no dependency on the missing private enterprise UI tree
- do not ship enterprise license enforcement as an OSS requirement
