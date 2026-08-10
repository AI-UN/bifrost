# Memory

## Intake Snapshot

- Repository root: `/home/evans/Projects/Public/bifrost`
- Intake date: `2026-05-09`
- Current checked-out branch: `main`
- Available restoration reference branches: `oss/foundation`, `oss/foundation-restack`, `oss/governance-identity`, `oss/runtime-policy`, `oss/platform`, `oss/ui-restoration`
- Official docs index: `https://docs.getbifrost.ai/llms.txt`

## Durable Constraints

- Official enterprise demo is unavailable.
- Official `/enterprise` docs and `/api-reference` docs are the primary evidence source.
- Outputs must help a later coding AI without visual perception.
- Missing evidence must be marked as inference, not fact.

## 2026-05-09

- User manually supplied official enterprise screenshots into `.goalkeeper/goals/enterprise-docs-ui-api-parity/assets/official`.
- Supplied assets arrived as `.avif`; local `.jpg` copies were generated with ImageMagick for downstream visual inspection while preserving the originals.
- Available converted surface families now include: Guardrails, Datadog connector, and User Provisioning.
- The goal has now been expanded from `goal-ready` to `plan-draft` with a concrete phased execution plan and atomic tasks `GK-001` through `GK-010`.
- The previous enterprise-restoration implementation goal is already complete; this research goal should treat the committed `oss/*` branches as comparison inputs while remaining fact-bound to official docs.
- The user selected `oss/ui-restoration` as the primary implementation-reference branch for this research effort.
- This goal is limited to facts, template/design reconstruction, and API collection/parity mapping. Actual backend interface changes and UI wiring are deferred to the next goal.
- `inventory.md` now holds the authoritative enterprise page inventory.
- `api-inventory.md` now holds the public API grouping and identified enterprise contract gaps.
- `design-notes.md` now holds screenshot-backed direct observations and cross-surface UI patterns.
- `inference-notes.md` now holds text-derived and partial-evidence reconstruction notes for non-screenshot or non-product-console pages.
- Runtime drift observed during execution: the actual checked-out branch moved to `oss/ui-restoration`; the docs package was updated to reflect this while preserving the rule that official docs remain the primary evidence source.
- `api-parity.md` now records which enterprise surfaces align to public contracts versus private restored contracts on `oss/ui-restoration`.
- `mismatch-notes.md` now records confirmed layout and contract divergence between official enterprise evidence and the current restored OSS branch.
- `reference-specs/` now contains implementation-ready layout contracts for governance/identity, policy/routing, connectors/observability, audit/MCP, plus structured example templates for non-multimodal coding AI.
- `fallback-parity.md` now classifies all current demo-only fallback surfaces as covered, partial, or unresolved against the official docs package.
- `handoff.md` now provides the execution rules and recommended module order for the next implementation goal.
