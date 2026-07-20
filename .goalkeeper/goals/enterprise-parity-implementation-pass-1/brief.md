# Brief

## Objective

Implement the first official-parity restoration pass for Bifrost OSS using the completed research package in `.goalkeeper/goals/enterprise-docs-ui-api-parity/` as the factual standard.

This goal should align backend API behavior and frontend page structure for the enterprise surfaces that were marked `covered` or `partial` in the completed research audit, while keeping `oss/ui-restoration` as the working implementation branch and comparison base.

## Success Criteria

- in-scope surfaces from `fallback-parity.md` marked `covered` or `partial` are implemented or revised on `oss/ui-restoration`
- current demo-only fallback surfaces for those in-scope modules are removed from the active user flow and replaced by working OSS implementations
- screenshot-backed modules are restructured to match the official information architecture and interaction patterns captured in `reference-specs/*.md`
- public API contracts identified in `api-parity.md` are reused directly where possible
- private restored contracts are reconciled module-by-module and kept clearly bounded to the in-scope enterprise surfaces
- the app builds successfully with `make build LOCAL=1`
- the implemented UI can load and navigate the touched in-scope surfaces without regressing into known auth-loop or demo-fallback behavior
- final implementation notes explain what was aligned exactly, what remains inferred, and what was intentionally deferred

## Constraints

- treat `.goalkeeper/goals/enterprise-docs-ui-api-parity/` as the primary fact standard
- official Bifrost docs remain the authoritative source whenever they conflict with current restored OSS code
- work on `oss/ui-restoration` and preserve the existing `oss/*` branch naming scheme
- prefer public contracts first:
  - `/api/session/**`
  - `/api/governance/**`
  - `/api/logs/**`
  - `/api/mcp/**`
  - `/api/oauth/**`
  - `/api/providers/**`
  - `/api/plugins/**`
- use private restored contracts only where the research package says no public equivalent exists yet
- keep the distinction between direct fact and implementation inference visible in commit/patch reasoning and final notes

## Non-Goals

- do not implement `unresolved` surfaces from `fallback-parity.md` as official-parity work
- do not restart the research/intake process already completed in `enterprise-docs-ui-api-parity`
- do not rewrite unrelated OSS routes or design systems outside the touched parity modules
- do not claim exact upstream enterprise parity for medium-confidence modules where the docs package still labels important behavior as inference

## Risks And Open Questions

- the in-scope modules vary in confidence; `RBAC`, `SCIM`, `Guardrails`, `Adaptive Routing`, and `Datadog` are stronger than `Audit Logs`, `Cluster`, and `MCP Auth Config`
- some current restored APIs may need backend contract changes before the UI can be made screenshot-aligned
- the login/session layer has previously shown `/api/rbac/context` auth-loop issues, so parity work must not silently reintroduce that regression
- medium-confidence modules may need conservative UX choices when the docs package does not fully specify the official layout

## Readiness Verdict

This goal is `goal-ready`.

The evidence package, in-scope module set, implementation constraints, and branch strategy are explicit enough to begin phased planning without reopening research.

## Next Step

Expand this brief into a phased implementation plan with backend contract alignment first, then UI parity restructuring, then verification and final parity audit.
