# Plan

## Status

`plan-accepted`

## Working Strategy

- Use `.goalkeeper/goals/enterprise-docs-ui-api-parity/` as the factual standard for scope, UI structure, API parity, and confidence level.
- Work on `oss/ui-restoration`.
- Implement only the modules marked `covered` or `partial` in `fallback-parity.md`.
- Prioritize backend/API contract alignment before deep UI restructuring so the redesigned surfaces can bind to stable routes.
- Use the strongest-evidence modules to establish the restored enterprise shell patterns:
  - wide operator tables
  - selector rail + detail pane composition
  - right-side slide-over editors
  - metrics-first dashboards where officially evidenced
- Treat medium-confidence modules conservatively and document any remaining inference at handoff.

## Phase 1: Contract Baseline And Scope Freeze

Purpose:
Lock the implementation scope to the research package and identify which current restored endpoints and fallback routes are authoritative, replaceable, or invalid.

Output:

- in-scope module checklist
- exact route and fallback replacement map
- public-contract reuse map versus private-contract carry-forward map

## Phase 2: Backend Parity Alignment

Purpose:
Bring the server-side contracts and data-shape behavior into line with the research-backed module expectations before reshaping the UI.

Output:

- reconciled handlers for in-scope modules
- updated request/response shapes where current restored contracts drift from the intended parity target
- preserved auth/session behavior for RBAC and related governance routes

## Phase 3: High-Confidence UI Restoration

Purpose:
Rebuild the strongest-evidence modules to follow the official structure rather than the current OSS-compatibility layouts.

Output:

- RBAC table + editor pattern
- SCIM provider selector + long-form configuration pattern
- Guardrails rules index + right-side builder editor
- Adaptive Routing metrics-first dashboard
- Datadog dedicated connector form

## Phase 4: Medium-Confidence UI And Governance Cleanup

Purpose:
Bring the partially inferred modules into a consistent, working OSS implementation without overstating certainty.

Output:

- Audit Logs implementation aligned to docs semantics
- Cluster implementation aligned to diagnostics-first expectations
- MCP Auth Config implementation aligned to MCP/OAuth surface reality
- Access Profiles / Users / Teams / Business Units cleanup aligned to the updated governance flow

## Phase 5: Verification And Final Parity Audit

Purpose:
Verify the implementation works, that in-scope fallbacks are no longer active, and that remaining gaps are explicitly documented.

Output:

- successful build verification
- smoke-tested in-scope routes
- final audit mapping implemented surfaces back to the research package
- explicit deferred list for unresolved modules

## Sequencing Rationale

- Contract stabilization comes first because the current restored branch mixes public and private shapes that can block later UI rewrites.
- High-confidence modules come before medium-confidence modules so the official visual/system patterns are established from the strongest evidence.
- Verification comes last so fallback removal, auth behavior, and route wiring are checked against the actual integrated result rather than partial patches.

## Proposed `/goal` Handoff

When this plan is accepted, execute `.goalkeeper/goals/enterprise-parity-implementation-pass-1/progress.md` in order, starting at `GK-001`, under these rules:

- use `.goalkeeper/goals/enterprise-docs-ui-api-parity/` as the factual standard
- work only on `covered` or `partial` modules from `fallback-parity.md`
- reuse public API contracts where `api-parity.md` marks them as aligned
- where no public enterprise contract exists, use the restored private contracts conservatively and revise them only when needed to satisfy the research-backed parity target
- replace current OSS-compatibility layouts when `mismatch-notes.md` says they diverge from official screenshot-backed structure
- do not promote `unresolved` modules to official-parity implementation work
- verify with real build and route smoke checks before closing the goal
