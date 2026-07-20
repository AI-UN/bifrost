# Review

## Review Gates

- Confirm the scope really is "restore enterprise-gated functionality into OSS", not "mirror the enterprise SKU exactly".
- Confirm the implementation baseline is latest `upstream/main`.
- Confirm `PR #2565` is treated as a reference and salvage source, not as a patch to replay.
- Confirm license enforcement code will be audited for removal or unconditional enablement, not carried forward as a shipping requirement.
- Confirm the branch strategy is acceptable: focused `oss/<module>` branches feeding an `oss/*` integration branch rather than the deleted top-level `oss` branch.
- Confirm final completion requires a parity audit between the original demo-only fallback UI surfaces and the restored OSS feature set.

## Decision Log

- Decided to mark the goal `goal-ready` because objective, scope, constraints, and reference sources are explicit enough for planning.
- Decided to keep the plan at `plan-draft` until the user explicitly approves it.
- Decided to separate backend salvage from frontend reimplementation because the PR ref lacks the actual enterprise UI tree.
- Decided to treat fallback enterprise UI as both discovery input and final acceptance evidence, not just as a temporary placeholder implementation detail.
- Recorded plan drift after the user removed the top-level `oss` branch: the integration baseline now stays under `oss/*`, with `oss/foundation` as the current long-lived integration branch.

## Acceptance Notes

- The user accepted the plan after adding the fallback-UI parity constraint; status has been updated to `plan-accepted`.
- `/goal` should start from `GK-001` unless later review promotes a different first task.

## Final Audit

Date: `2026-05-08`

Concrete deliverables audited against real evidence:

1. Inventory the enterprise-gated and fallback/demo-only OSS surfaces.
2. Classify `PR #2565` by module and portability.
3. Reimplement the missing functionality on top of latest `upstream/main`.
4. Re-audit the restored functionality against the fallback/demo-only UI surfaces.
5. Materialize the intended `oss/*` branch line and keep the integrated result on `oss/foundation` without recreating a top-level `oss` branch.
6. Reconfirm that the result builds and runs from the committed post-restack state.

Evidence checked:

- Discovery and planning artifacts:
  - `gk-001-fallback-parity-matrix.md`
  - `gk-002-pr-2565-salvage-map.md`
  - `gk-003-execution-baseline.md`
  - `gk-020-verification-map.md`
  - `gk-021-final-parity-audit.md`
  - `gk-024-completion-checklist.md`
- Real branch state:
  - `git branch --list 'oss*'` shows `oss/foundation`, `oss/foundation-restack`, `oss/governance-identity`, `oss/runtime-policy`, `oss/platform`, `oss/ui-restoration`
  - `git log --oneline --decorate --max-count=6 --branches='oss/*'` shows the committed recovery chain ending at `ddd6fe1fd`
- Committed post-restack verification:
  - `cd framework && go test ./configstore/...`
  - `cd transports && go test ./bifrost-http/handlers ./bifrost-http/server ./bifrost-http/integrations`
  - `cd plugins/governance && go test ./...`
  - `cd ui && npx tsc --noEmit`
  - `make build LOCAL=1`
- Committed post-restack runtime proof:
  - `./tmp/bifrost-http -app-dir /tmp/bifrost-oss-restack-check -host 127.0.0.1 -port 18081`
  - `curl -sf http://127.0.0.1:18081/health` returned `{"components":{"db_pings":"ok"},"status":"ok"}`
  - graceful shutdown completed on `SIGINT`

Residual risks checked:

- The string `Book a demo` still appears in `ui/components/sidebar.tsx`, but the surrounding code is a generic production-support marketing card, not a tracked enterprise fallback surface.
- Unrelated local files remain outside the product restack:
  - `.gitignore`
  - `AGENTS.md`
  - `.goalkeeper/**`
  These were intentionally preserved and excluded from feature commits.

Verdict:

- The fallback-surface inventory and PR salvage classification are present.
- The recovered OSS implementation is present on committed `oss/*` history rooted in latest `upstream/main`.
- The final parity audit still reports `0` `still missing` fallback surfaces.
- The previously open branch-topology blocker is now closed in real git state.
- Build and runtime evidence have been reconfirmed after the restack.

Final decision: **complete**
