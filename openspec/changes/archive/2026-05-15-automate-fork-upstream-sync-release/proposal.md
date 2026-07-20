## Why

Maintaining this fork currently depends on manual monitoring of `upstream/main`, manual rebases of fork-only patches, and manual reaction to upstream transport releases. That is slow, easy to miss, and especially risky here because Bifrost's transport release artifacts are keyed off `transports/v*` tags while the fork also needs its own release and Docker publication flow.

## What Changes

- Add automated GitHub Actions workflows to keep the fork aligned with `maximhq/bifrost` by fetching upstream changes on a schedule and on demand.
- Add an automated sync flow that rebases or reapplies the fork's patch branch on top of the latest upstream base, pushes the result to the fork, and surfaces failures in a single maintainer-facing place instead of silently drifting.
- Add automated release detection that watches for new upstream transport tags in the `transports/vX.Y.Z` format after the upstream release commit has landed in the fork's maintained branch.
- Add fork-side release tagging that maps upstream transport tags to fork tags without losing the upstream version provenance.
- Add automatic dispatch of the fork's existing release and Docker publication workflows after a fork release tag is created or when a tag exists but downstream publication is missing.
- Add supporting scripts, state recording, and documentation for required secrets, branch conventions, and recovery steps.

## Capabilities

### New Capabilities
- `fork-upstream-sync`: Keep a maintained fork branch synchronized with `upstream/main`, reapply fork-only patches automatically, and surface blocked sync attempts with actionable diagnostics.
- `fork-upstream-release-sync`: Detect newly synced upstream `transports/v*` releases, create matching fork release tags, and trigger fork release plus Docker publication exactly once per upstream transport release.

### Modified Capabilities
- None.

## Impact

- Affected code: `.github/workflows/`, release helper scripts under `.github/workflows/scripts/`, and fork-maintainer documentation.
- Affected systems: GitHub Actions, GitHub Releases, git tags, Docker registry publication, and fork/upstream branch management.
- External dependencies: `gh` CLI usage inside Actions, repository secrets for authenticated pushes and optional Docker publication, and reliable access to upstream tags.
