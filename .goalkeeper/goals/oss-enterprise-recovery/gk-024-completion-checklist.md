# GK-024 Completion Checklist

Date: `2026-05-07`

Purpose:
- Restate the user objective as concrete deliverables.
- Map every explicit requirement to real artifacts and command evidence.
- Identify which requirements are complete, which are narrowed-but-accepted, and which are still blocked.

## Objective Restatement

The user asked for a phased OSS recovery of enterprise-gated Bifrost features, with these concrete deliverables:

1. Inspect the current codebase for enterprise functionality hidden behind fallback UI or Book-a-demo surfaces.
2. Inspect `PR #2565`, classify what it restores, and group it by module.
3. Reimplement the missing functionality on top of the latest `main`, module by module, using the PR as reference only.
4. Ensure final acceptance compares factually missing features against the current fallback/demo-only UI surfaces to confirm they were fully restored or otherwise backed by working OSS behavior.
5. Produce an end state where the missing features are restored, changes are integrated into the intended `oss/*` branch line, and the result builds and runs successfully.

## Requirement Checklist

| Requirement | Evidence | Current status | Notes |
|---|---|---|---|
| Inventory hidden enterprise/fallback UI surfaces | `gk-001-fallback-parity-matrix.md` | complete | Covers full-page fallbacks, hybrid CTA surfaces, shared `@enterprise/lib` stubs, and E2E placeholder coverage. |
| Map missing capability to fallback/demo-only UI | `gk-001-fallback-parity-matrix.md` | complete | User-added parity constraint is reflected directly in the matrix. |
| Inspect and classify `PR #2565` by module | `gk-002-pr-2565-salvage-map.md` | complete | Salvageable backend code, stale code, and incompatible license-gated pieces are separated. |
| Base work on latest `upstream/main`, not PR base | `brief.md`, `gk-003-execution-baseline.md`, current branch state | complete | Plan and execution artifacts explicitly treat current main as the baseline. |
| Reimplement features module by module on current main | `progress.md`, restored code under `framework/`, `transports/`, `ui/`, `plugins/` | functionally complete | Actual restored modules exist across governance, routing, observability, platform, and UI layers. |
| Do not depend on missing private `ui/app/enterprise` tree | `brief.md`, restored local UI paths under `ui/app/workspace/**`, `ui/lib/**` | complete | Active OSS UI now uses local paths; missing private tree was not required. |
| Remove license-gating as shipping requirement | `gk-005-license-neutral-strategy.md`, absence of `framework/license` in restored path | complete | OSS enablement strategy is license-neutral. |
| Compare restored functionality against fallback/demo UI at the end | `gk-020-verification-map.md`, `gk-021-final-parity-audit.md` | complete | Final parity audit exists and uses fallback surfaces as the oracle. |
| Eliminate unresolved fallback/demo-only surfaces | `gk-021-final-parity-audit.md` | complete at parity level | `0` surfaces remain `still missing`; some are intentionally narrowed. |
| Keep `oss/*` naming and avoid recreating top-level `oss` | `active-goal.md`, `memory.md`, real branch state | complete as constraint | Current branch is `oss/foundation`; no top-level `oss` exists. |
| Materialize real `oss/*` branch history for restored work | `git branch --list 'oss*'` => `oss/foundation`, `oss/foundation-restack`, `oss/governance-identity`, `oss/runtime-policy`, `oss/platform`, `oss/ui-restoration`; commit chain rooted at `5a0f6a3fc`..`ddd6fe1fd` | complete | The coarse-grained restack was executed on real committed history and then integrated back onto `oss/foundation` via fast-forward. |
| Build successfully after restoration | `make build` evidence in `progress.md` and later audit entries | complete | Reconfirmed after latest governance tightening. |
| Run successfully after restoration | `gk-021-final-parity-audit.md` plus `2026-05-08` runtime evidence in `progress.md` / `memory.md` | complete | Reconfirmed after the restack by booting `./tmp/bifrost-http` on `127.0.0.1:18081` and checking `/health`. |
| Verification covers backend and frontend | `progress.md`, `gk-020-verification-map.md`, repeated `go test`, `tsc`, `make build` evidence | complete with noted gaps | Full Playwright execution was not rerun in the latest checkpoint, but targeted backend/UI/build checks are documented. |

## Real Remaining Gap

No uncovered success criterion remains in real repo state based on the current audit evidence.

## Closure Path Executed

The previously planned blocker has now been executed and re-audited:

1. The coarse-grained `oss/*` restack was materialized as real committed branch history.
2. The resulting line was integrated back onto `oss/foundation` without recreating a top-level `oss` branch.
3. The high-signal verification set was rerun on the committed post-restack state.
4. Runtime boot and `/health` checks were repeated after the restack.

## Checklist Verdict

Current verdict:
- feature restoration and parity evidence: **complete**
- branch-topology evidence: **complete**
- overall goal status: **complete**
