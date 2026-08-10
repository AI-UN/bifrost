# GK-021 Final Parity Audit

Date: `2026-05-07`

Purpose:
- Perform the final missing-feature vs fallback-UI parity audit required by the user.
- Use `GK-001` as the original oracle and `GK-020` as the implementation/verification map.
- Classify every originally demo-only or stub-backed surface as:
  - `fully restored`
  - `intentionally narrowed`
  - `still missing`

Audit inputs:
- `.goalkeeper/goals/oss-enterprise-recovery/gk-001-fallback-parity-matrix.md`
- `.goalkeeper/goals/oss-enterprise-recovery/gk-020-verification-map.md`

## Summary Verdict

| Verdict | Count | Meaning |
|---|---:|---|
| `fully restored` | 8 | The original fallback surface is now backed by a live OSS implementation without relying on the old demo CTA behavior. |
| `intentionally narrowed` | 20 | The fallback surface is replaced with a live OSS-compatible implementation, but the exact enterprise SKU shape was intentionally not recreated. |
| `still missing` | 0 | No `GK-001` fallback surface remains unresolved at the parity-audit level. |

## A. Former Full-Page Demo-Only Routes

| Surface | Final verdict | Audit rationale |
|---|---|---|
| `/workspace/adaptive-routing` | `intentionally narrowed` | Live adaptive-routing policy/metric/quality-score management exists, but the OSS scope is intentionally pragmatic rather than enterprise-SKU identical. |
| `/workspace/alert-channels` | `intentionally narrowed` | Real alert-channel CRUD and trigger-source management exist, but the restored scope is the documented OSS-safe first pass. |
| `/workspace/audit-logs` | `fully restored` | The demo page is gone and the route is backed by persisted audit-log storage plus live query/verification APIs. |
| `/workspace/cluster` | `intentionally narrowed` | Real cluster status/drain APIs exist, but the implementation is intentionally single-node/local-manager rather than full enterprise clustering. |
| `/workspace/governance/access-profiles` | `intentionally narrowed` | Access profiles are restored as synthetic compatibility views derived from `user_groups` and virtual-key assignments rather than a separate enterprise model. |
| `/workspace/governance/business-units` | `intentionally narrowed` | Business units are restored through `user_groups` compatibility rather than a distinct enterprise-only governance object. |
| `/workspace/governance/rbac` | `fully restored` | Real roles, permissions, context resolution, and route-level enforcement replaced the CTA page. |
| `/workspace/governance/users` | `intentionally narrowed` | Users are restored through aggregated `user_groups` membership rather than a separate identity catalog. |
| `/workspace/guardrails` | `intentionally narrowed` | Live CRUD and runtime enforcement exist for the documented OSS-safe subset, not full enterprise parity. |
| `/workspace/guardrails/configuration` | `intentionally narrowed` | Same rationale as the primary guardrails surface. |
| `/workspace/guardrails/providers` | `intentionally narrowed` | Provider overrides are live, but the feature scope remains the OSS first pass. |
| `/workspace/mcp-auth-config` | `intentionally narrowed` | The CTA is replaced with a real OSS management surface based on the existing MCP registry/client config rather than a separate enterprise auth product. |
| `/workspace/mcp-tool-groups` | `fully restored` | Live MCP tool-group CRUD, membership, and governance attachment flows are present. |
| `/workspace/pii-redactor` | `intentionally narrowed` | Live rules and request-side redaction exist, but the feature scope is explicitly the OSS-safe first pass. |
| `/workspace/pii-redactor/rules` | `intentionally narrowed` | Same rationale as the primary PII surface. |
| `/workspace/pii-redactor/providers` | `intentionally narrowed` | Provider override management is live, but still within the narrowed OSS scope. |
| `/workspace/scim` | `intentionally narrowed` | The route is live and no longer demo-only, but the OSS surface is a practical SCIM/SSO configuration slice rather than full enterprise provisioning parity. |

## B. Former Embedded Or Hybrid Demo-Only Surfaces

| Surface | Final verdict | Audit rationale |
|---|---|---|
| `/workspace/config/api-keys` | `intentionally narrowed` | The route now has real OSS Admin API Key CRUD, Bearer-token auth for admin APIs, and browser login support for Admin API Keys, but its scope-based key story still relies on Virtual Keys rather than recreating the enterprise SKU's dedicated scoped-key model. |
| `/workspace/config/client-settings` large payload fragment | `intentionally narrowed` | The fragment is now functional and live-backed, but large-payload support is restored at a documented OSS first-pass scope. |
| `/workspace/dashboard` user rankings tab | `fully restored` | The CTA tab was replaced with a live rankings tab backed by `GET /api/logs/user-rankings`. |
| Prompt settings sidebar deployments accordion | `intentionally narrowed` | The CTA was replaced with a live versions/sessions/commit workflow rather than recreating a dedicated enterprise deployment subsystem. |
| Observability BigQuery plugin view | `intentionally narrowed` | Real connector CRUD/test UI exists, but the connector model is the narrowed OSS first pass. |
| Observability Datadog plugin view | `intentionally narrowed` | Same rationale as BigQuery. |

## C. Former Functional Fallback Components Backed By Stubs

| Surface | Final verdict | Audit rationale |
|---|---|---|
| `/workspace/governance/teams` | `intentionally narrowed` | The page still comes from the fallback component tree, but it now runs against live RBAC-aware OSS behavior instead of a CTA or always-allow stub. |
| Governance/providers/plugins/logs/routing/config/sidebar RBAC gating | `fully restored` | The compatibility import path remains, but the RBAC behavior is live-backed and no longer an always-allow fake. |
| `/workspace/virtual-keys` usage display | `intentionally narrowed` | The page now gets live compatibility data, but the access-profile/user model is still synthesized from `user_groups`. |
| `/workspace/config/security` auth-type awareness | `fully restored` | The SCIM/SSO auth-type stub is replaced by a real OSS-backed contract. |
| `/workspace/config/client-settings` save flow | `fully restored` | Large-payload query/mutation hooks are now live instead of undefined/no-op. |

## Residual Compatibility Debt

These items do not count as parity failures, but they remain explicit follow-up debt:

- Fallback duplicates still exist under `ui/app/_fallbacks/enterprise/lib/**`, even though active UI/store code now uses local OSS paths for RBAC, user/access-profile types, login, large-payload UI, teams UI, and restored API slices.
- The final branch structure is still centered on `oss/foundation`; the originally imagined per-module local branches such as `oss/adaptive-routing` and `oss/user` were documented in the plan but not materialized as separate surviving local branches during execution.

## Final GK-021 Verdict

Audit conclusion:
- The final parity check against the original fallback UI matrix finds no `still missing` surfaces.
- All originally demo-only or stub-backed surfaces are now either:
  - fully restored with live OSS behavior, or
  - intentionally narrowed to an OSS-compatible implementation that replaces the old demo-only fallback.
- Remaining work after this audit is about compatibility cleanup and full end-to-end operational verification, not about unresolved demo-only enterprise gaps from `GK-001`.
