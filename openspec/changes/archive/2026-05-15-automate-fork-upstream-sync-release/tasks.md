## 1. Fork Sync Foundations

- [x] 1.1 Add a tracked sync-state file format and helper script(s) for reading/writing upstream repo, upstream branch, upstream commit, reachable upstream transport tag, last released upstream transport tag, and fork transport tag.
- [x] 1.2 Add repository-variable and secret handling for `UPSTREAM_REPO`, `UPSTREAM_BRANCH`, `PATCH_BRANCH`, GHCR image naming, and optional Docker Hub credentials.
- [x] 1.3 Add a fork-mode skip guard to the existing push-based release pipeline so automated upstream-sync commits do not trigger premature source-style transport or Docker releases.

## 2. Upstream Rebase Automation

- [x] 2.1 Add `.github/workflows/fork-upstream-sync.yml` with scheduled and manual triggers, concurrency protection, and checkout/fetch steps for origin plus upstream.
- [x] 2.2 Implement the rebase flow that detects fork-only commits on the maintained branch, rebases them onto the latest upstream head, and exits cleanly when no upstream delta exists.
- [x] 2.3 Implement success handling that updates the sync-state file, pushes the maintained branch, and closes any stale blocked-sync issue.
- [x] 2.4 Implement failure handling that pushes a timestamped diagnostic branch and creates or updates a single blocked-sync tracking issue with upstream commit, branch name, and log context.

## 3. Upstream Transport Tag Sync

- [x] 3.1 Add `.github/workflows/fork-sync-transport-release.yml` with push, schedule, and manual triggers plus upstream tag fetch logic.
- [x] 3.2 Implement reachable-tag detection for the observed upstream namespace `transports/v*`, ensuring only tags whose commits are ancestors of the maintained branch are eligible.
- [x] 3.3 Implement fork tag creation in the form `transports/v<upstream-version>-0`, plus state-file updates for the source upstream tag and created fork tag.
- [x] 3.4 Implement idempotent behavior for re-runs so an existing fork tag dispatches downstream publication again only when release or Docker output is missing.

## 4. Fork Release And Docker Publication

- [x] 4.1 Add a fork transport release workflow that runs from an explicit fork tag/ref, reuses existing transport build and R2 upload helpers where possible, and creates a GitHub release for an already-existing fork tag.
- [x] 4.2 Add or extend release helper scripts so release notes include both the fork tag and the source upstream `transports/v*` tag without trying to recreate the tag.
- [x] 4.3 Add a fork Docker publication workflow that publishes multi-arch images to GHCR by default and to Docker Hub when credentials are configured.
- [x] 4.4 Reuse or wrap existing Docker build and manifest helpers so fork Docker tags align with the fork transport version suffix and remain idempotent on re-runs.

## 5. Verification And Documentation

- [x] 5.1 Add maintainer documentation covering branch strategy, required repository variables/secrets, manual dry-run steps, and blocked-sync recovery.
- [x] 5.2 Validate workflow YAML and shell scripts locally where feasible, including tag parsing for `transports/v*` and fork tag derivation with `-0` suffixes.
- [ ] 5.3 Run targeted manual GitHub Actions dispatch tests for sync, tag-sync, release, and Docker workflows in a safe order before enabling schedules.
