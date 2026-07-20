## Context

This fork already tracks `upstream` as `https://github.com/maximhq/bifrost`, but its maintenance and release flow are still source-repo-oriented. In the current repository, `release-pipeline.yml` publishes transport releases from pushes to `main` when `transports/version` changes, and the official transport tag namespace is `transports/v*` (plural), not `transport/v*`. Docker publication is also tied to the transport version, but it is driven by the repository's own version files and release scripts rather than by explicit observation of upstream release tags.

The reference implementation in `../CLIProxyAPI` uses three ideas that are directly useful here: a scheduled upstream sync workflow, a state file that records the upstream commit and release marker that the fork has incorporated, and a separate tag-sync workflow that explicitly dispatches downstream release and Docker workflows because `GITHUB_TOKEN` tag pushes do not cascade. The main difference is that Bifrost's release surface is larger and the upstream transport release is identified by `transports/vX.Y.Z` tags while the published Docker image uses plain `vX.Y.Z` tags.

## Goals / Non-Goals

**Goals:**
- Keep a configurable fork-maintained patch branch automatically rebased onto `upstream/main` on a schedule and on demand.
- Preserve fork-only commits during sync, and make blocked rebases visible through a single issue/branch trail instead of silent divergence.
- Detect new upstream `transports/v*` tags only after the tagged commit is reachable from the maintained fork branch.
- Create a fork release tag derived from the upstream transport version, then publish a fork GitHub release and Docker images exactly once for that upstream transport release.
- Reuse as much of the repository's existing transport build, upload, and release logic as practical instead of replacing the whole release system.

**Non-Goals:**
- Rewriting the full upstream `release-pipeline.yml` for source-repo releases.
- Automating releases for `core`, `framework`, plugins, Helm, or CLI in this change.
- Blindly auto-resolving source conflicts during rebase by always preferring upstream or fork content.
- Supporting any tag namespace other than the observed upstream transport namespace `transports/v*`.

## Decisions

### 1. Add fork-specific maintenance workflows instead of folding everything into `release-pipeline.yml`

The fork automation will be implemented as additive workflows under `.github/workflows/`, with small shared scripts under `.github/workflows/scripts/`. This keeps the current source-oriented release pipeline intact for normal repository development, while making fork maintenance behavior explicit and separately auditable.

Why this over extending `release-pipeline.yml` directly:
- `release-pipeline.yml` is a large multi-module release orchestrator keyed off version-file deltas, not upstream release events.
- The fork flow must wait for an upstream transport tag to exist and be reachable, which is a different trigger model from the current push-to-main logic.
- Isolating fork workflows reduces the risk of regressing official release behavior while still letting the new workflows call existing helper scripts where useful.

### 2. Use a true rebase flow for the maintained patch branch

The sync workflow will fetch `upstream/main`, identify fork-only commits on the maintained branch, and attempt to rebase that branch onto the latest upstream head. A successful run fast-forwards the maintained branch in the fork. A failed run pushes a timestamped diagnostic branch and updates a single tracking issue.

The maintained branch name will be configurable through a repository variable such as `PATCH_BRANCH`. If that variable is not set, the workflow will default to the repository's default branch and the implementation will add a fork-mode skip guard to the existing push-based release pipeline so automated sync commits do not trigger premature source-style releases.

Why this over the merge-based CLIProxyAPI flow:
- The user explicitly asked for automatic rebase of the patch branch.
- A rebase keeps fork-only patches linear and makes the maintained branch easier to inspect against upstream.
- Merge commits from periodic syncs would make it harder to tell which commits are fork-only patches versus upstream imports.

Alternatives considered:
- Periodic merge from upstream: simpler, but does not satisfy the required rebase behavior.
- Cherry-picking a curated patch queue: cleaner in theory, but much higher maintenance overhead and more assumptions about how the fork is organized.

### 3. Record upstream sync and release state in a committed env file

The fork will keep a small tracked file such as `.fork-upstream-sync.env` containing at least:
- `UPSTREAM_REPO`
- `UPSTREAM_BRANCH`
- `UPSTREAM_COMMIT`
- `UPSTREAM_TRANSPORT_TAG`
- `LAST_RELEASED_UPSTREAM_TRANSPORT_TAG`
- `FORK_TRANSPORT_TAG`

Why this over workflow artifacts or issue comments:
- The state is versioned, reviewable, and available to every workflow run from the checked-out branch.
- The tag-sync workflow can operate without querying previous run artifacts.
- Maintainers can see exactly which upstream commit and tag a given fork revision corresponds to.

### 4. Gate fork transport publication on upstream tag reachability, not version-file bumps

Release detection will look for the newest upstream tag matching `transports/v*` whose tagged commit is an ancestor of the maintained branch head. Only then will the fork consider that release eligible for publication. The workflow will compare that upstream tag against `LAST_RELEASED_UPSTREAM_TRANSPORT_TAG` in the state file and create a new fork tag only when the upstream tag is newer.

Why this over reusing `detect-all-changes.sh` as-is:
- `detect-all-changes.sh` treats a missing fork Docker image or transport tag as enough reason to release when `transports/version` increments.
- In a fork, upstream may bump `transports/version` before publishing the actual upstream tag, which would release the fork too early.
- Reachability-based tag detection matches the user's requirement to follow upstream releases rather than raw version-file churn.

### 5. Use fork transport tags in the form `transports/v<upstream-version>-0`

When the latest reachable upstream release is `transports/v1.5.2`, the fork will create `transports/v1.5.2-0`. This mirrors the CLIProxyAPI fork convention of appending `-0`, preserves the upstream version, and gives the fork a unique release identity.

Why this over reusing the exact upstream tag name:
- A suffix makes it obvious which release was published by the fork.
- It allows a clean path for future fork-only rebuilds such as `-1` without pretending they are identical to upstream artifacts.
- Existing Bifrost helper scripts already accept version strings with prerelease-style suffixes, so this is lower risk than inventing a new tag namespace.

Alternatives considered:
- Exact same tag as upstream: simpler, but loses fork provenance.
- A new namespace such as `fork/transports/v*`: clearer, but would require broader script and docs changes for a weaker payoff.

### 6. Explicitly dispatch fork release and Docker workflows after creating the fork tag

The tag-sync workflow will create the fork tag with `GITHUB_TOKEN`, then immediately dispatch a dedicated fork release workflow and a dedicated fork Docker workflow using `gh workflow run`. This mirrors the proven pattern in `CLIProxyAPI`.

Why this over relying on tag-push triggers alone:
- `GITHUB_TOKEN` tag pushes do not reliably trigger downstream workflows.
- Explicit dispatch lets the workflow repair missing downstream publication when the tag already exists but the release or Docker publish did not complete.
- It keeps the publication logic idempotent.

### 7. Reuse existing transport helper scripts, but add fork-aware entry points

The new fork release workflow should reuse existing build/upload/release helpers where practical, but it should do so through fork-specific wrappers or small script extensions. The main expected adjustments are:
- allow release creation for an already-existing tag instead of always creating the tag inside `release-bifrost-http-finalize.sh`
- allow fork release titles and notes to include both fork tag and upstream source tag
- allow Docker publication to build from an explicit fork tag/ref and publish configurable registry targets

Why this over a full reimplementation:
- The repository already contains the transport build, R2 upload, changelog, and Docker manifest logic.
- Reusing those paths lowers behavioral drift between manual and automated releases.
- Small wrapper scripts are easier to maintain than duplicating a large transport release process.

### 8. Publish GHCR by default and Docker Hub when credentials are configured

The fork Docker workflow will always publish to GHCR under a configurable image name derived from the fork repository owner, and it will publish to Docker Hub only when the required credentials are present.

Why this over Docker Hub only:
- GHCR works with the built-in `GITHUB_TOKEN`, which lowers the bootstrap cost for a fork.
- This mirrors the proven `CLIProxyAPI` fork pattern while preserving an optional Docker Hub path for maintainers who want public pull targets outside GitHub.
- It avoids making Docker Hub credentials a hard prerequisite for the automation to be useful.

## Risks / Trade-offs

- Rebase conflicts on fork-only patches could happen frequently if the fork diverges widely from upstream. → Mitigation: fail safely, push the failed rebase branch, and update a single tracking issue with branch name, upstream commit, and log tail.
- Running upstream sync directly on `main` could accidentally trigger the existing push-based release pipeline before upstream tags are checked. → Mitigation: either use a configurable dedicated maintained branch or add a fork-mode skip guard in `release-pipeline.yml` for automated upstream-sync commits.
- Existing helper scripts are optimized for source-repo releases and may assume they own tag creation. → Mitigation: introduce narrow script flags or wrappers instead of editing unrelated release logic.
- Upstream transport tags are plural `transports/v*`, while the user description used singular `transport/`. → Mitigation: hard-code detection to the observed upstream namespace and document that research result in maintainer docs and release notes.
- Docker publication targets differ between the current repo (`docker.io/maximhq/bifrost`, optional GHCR build workflow) and a fork's likely registries. → Mitigation: make registry owner/repository configurable through repository variables and keep GHCR mandatory with Docker Hub optional.

## Migration Plan

1. Add fork-maintenance workflows and helper scripts behind repository variables such as `UPSTREAM_REPO`, `UPSTREAM_BRANCH`, `PATCH_BRANCH`, and optional Docker publication settings.
2. Seed the tracked sync-state file with the current maintained branch's upstream merge-base, current reachable upstream transport tag, and an empty or current `LAST_RELEASED_UPSTREAM_TRANSPORT_TAG` depending on how the fork is being adopted.
3. Run the sync workflow manually in dry-run or no-push mode to verify branch discovery, rebase behavior, and issue creation.
4. Run the tag-sync workflow manually against an already-known upstream transport tag to verify fork tag creation and downstream workflow dispatch.
5. Enable the schedules only after both manual paths succeed.
6. If a rollback is needed, disable the schedules, delete the new workflows, and leave the repository on its last manually maintained branch state. Fork publication tags and releases can remain because they are immutable historical artifacts.

## Open Questions

- None at this stage.
