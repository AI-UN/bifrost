# Plan

## Status

`plan-accepted`

## Working Strategy

- Treat official Bifrost docs as the only primary evidence source.
- Rank evidence by confidence: screenshot pixels, then explicit doc text, then API reference contracts, then clearly labeled inference.
- Use `main` as the current checked-out branch context, but treat `oss/ui-restoration` as the primary implementation-reference branch whenever current OSS behavior or UI/API drift needs to be traced to prior restoration work.
- Use the other `oss/*` branches only as secondary historical references when a divergence needs finer provenance.
- Produce research artifacts inside `.goalkeeper/goals/enterprise-docs-ui-api-parity/` so later coding work can consume them without needing multimodal capabilities.
- Do not perform production backend or frontend implementation in this goal.
- Keep artifacts limited to research documents, parity matrices, and optional reference specs/templates that exist only to communicate facts and intended structure.
- Compare reconstructed enterprise functionality against the current OSS fallback UI so completion can be audited against the actual demo-only surfaces.

## Phase 1: Evidence Intake And Inventory

Purpose:
Normalize the official evidence set and build the authoritative page inventory.

Output:
- Enterprise page inventory with screenshot presence, local asset path, and evidence confidence
- API-reference inventory grouped by feature family
- Current OSS route and fallback-surface mapping for the same families, with `oss/ui-restoration` pointers where prior restoration work already exists

## Phase 2: Visual Reconstruction Notes

Purpose:
Turn screenshots and adjacent docs text into reusable design specifications.

Output:
- Page-by-page visual notes for all screenshot-backed enterprise surfaces
- Structured descriptions of layout, information hierarchy, controls, states, and navigation placement
- Explicit separation of direct observation versus inference

## Phase 3: API Contract And Parity Mapping

Purpose:
Map official public API contracts against the current OSS code and identify drift.

Output:
- Feature-family API matrix: documented endpoints, request/response shapes, current OSS implementation, and mismatch notes
- References to any relevant prior implementation already present on `oss/ui-restoration`
- Missing-contract and divergent-contract lists for follow-up implementation
- Inference notes where docs imply a UI or backend behavior not exposed in API reference

## Phase 4: Reference Surface Generation

Purpose:
Create handoff-ready example surfaces that later coding AI can follow without visual input.

Output:
- At least one reference template or structured component spec per major surface family
- Reusable style and interaction guidelines derived from official screenshots
- Notes on how each surface should align with existing Bifrost OSS design patterns

## Phase 5: Final Audit And Handoff Package

Purpose:
Package the research into an execution-ready artifact set and verify it against fallback UI coverage.

Output:
- Final parity audit comparing reconstructed features against current fallback/demo-only surfaces
- Handoff summary listing confirmed facts, inferred behaviors, and unresolved gaps
- Clear next-step instructions for a later `/goal` implementation pass

## Sequencing Rationale

- The intake phase comes first because the screenshot corpus and docs index are now available but not yet normalized into an authoritative inventory.
- Visual notes come before API parity conclusions so design claims can rely on direct screenshot evidence where available.
- API mapping comes before reference page generation so example surfaces reflect the documented contract rather than only the screenshots.
- The fallback-UI comparison is reserved for the end so every reconstructed feature can be checked against the original demo-only surface list in one pass.
- Real implementation is intentionally deferred to the next goal so this goal can stay fact-bound and avoid contaminating the standard with speculative code decisions.

## Proposed `/goal` Handoff

When this plan is accepted, execute `.goalkeeper/goals/enterprise-docs-ui-api-parity/progress.md` in order, starting at `GK-001`, under these rules:
- use only official Bifrost docs as primary evidence
- preserve the distinction between observed facts and inference
- treat `oss/ui-restoration` as the primary implementation reference, not as primary factual evidence about enterprise behavior
- prefer `.goalkeeper` research artifacts and template/reference files over product code changes
- compare every reconstructed surface against the current OSS fallback UI before declaring the research package complete
- do not adjust backend handlers, UI wiring, or runtime logic in this goal; record those changes for the next implementation goal instead
