# Brief

## Objective

Restore the enterprise-gated functionality that is intentionally absent from the current OSS Bifrost codebase, covering both backend services and frontend workspace surfaces, by reimplementing the missing capabilities on top of the latest `upstream/main` code and integrating the finished work into the long-lived `oss` branch.

## Success Criteria

- A current inventory exists for all enterprise-gated UI routes, placeholder views, and shared `@enterprise/lib` stubs that affect OSS behavior.
- That inventory explicitly maps each factually missing enterprise capability to the current fallback UI surface that only asks the user to request a demo, so final validation can prove those surfaces were fully replaced or backed by working functionality.
- `PR #2565` is classified by module, with salvageable backend code, tests, and specs separated from outdated or non-portable pieces.
- Missing features are reimplemented as latest-main-compatible workstreams, not by replaying the old PR blindly.
- Work is split into module branches such as `oss/adaptive-routing`, `oss/user`, and similar focused branches, then merged back into `oss`.
- The final integrated `oss` branch builds successfully and can run the gateway and UI without enterprise placeholder breakage for the restored features.
- Verification covers both backend and frontend paths, with targeted tests or explicit coverage notes per restored module.

## Constraints

- Base all reimplementation on the latest `upstream/main`, not on the historical PR base.
- Use the current OSS repository, current fallback UI, and `PR #2565` as reference material only.
- Treat the current fallback UI as a verification oracle for missing enterprise functionality: every demo-only placeholder surface must be part of the final parity audit.
- Do not depend on the missing private `ui/app/enterprise` source tree.
- Preserve unrelated local changes already present in the working tree.
- Favor OSS-native behavior; commercial licensing gates and enterprise packaging logic are not the target outcome.
- End state must remain buildable and operable inside this repository layout and workspace.

## Non-Goals

- Bit-for-bit parity with Maxim's private enterprise repository.
- Shipping commercial licensing, trial enforcement, or account-entitlement flows as a user-facing requirement.
- Reproducing private deployment artifacts, sales flows, or hosted-service integrations that cannot be inferred from the OSS tree and surviving PR evidence.
- Porting every line from `PR #2565`; portability and correctness on current main matter more than historical fidelity.

## Risks And Open Questions

- `PR #2565` exposes a large backend-oriented patch set and design docs, but the fetched ref does not contain the real `ui/app/enterprise` implementation, so frontend restoration will require fresh work from OSS placeholders and shared-page usage.
- Current `main` has drifted from the PR merge-base, so direct cherry-picks are likely to conflict or encode stale assumptions.
- Several capabilities are cross-cutting rather than isolated pages: RBAC, SCIM auth type, access profiles, large payload settings, user rankings, connector views, and virtual-key user assignments already leak into shared OSS pages through `@enterprise/lib`.
- Some features in the PR are enterprise-license oriented rather than OSS-restoration oriented, especially `framework/license`; these must be audited and likely replaced by unconditional OSS enablement or feature-specific config.
- Cluster, Vault, SSO/SCIM, and large payload support may need narrowed first-pass implementations to keep the integrated branch shippable.

## Readiness Verdict

`goal-ready`

Reason:
- The target outcome is explicit.
- The integration baseline, reference sources, and branch strategy direction are explicit.
- Constraints, non-goals, and major risks are concrete enough to support phased execution.

## Next Step

Review the draft execution plan in `plan.md` and `progress.md`. If accepted, mark the goal `plan-accepted` and begin `/goal` execution from `GK-001`.
