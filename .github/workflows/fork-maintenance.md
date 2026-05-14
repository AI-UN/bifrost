# Fork Upstream Maintenance

This repository now includes a fork-maintenance workflow set for keeping a maintained fork branch rebased on top of `maximhq/bifrost` and mirroring upstream transport releases.

## Branch Strategy

- Maintained branch: controlled by the `PATCH_BRANCH` repository variable.
- Default behavior: if `PATCH_BRANCH` is unset, workflows fall back to the repository default branch.
- Tracked state file: `.fork-upstream-sync.env`.
- Synthetic sync commits include `--skip-ci --skip-pipeline` so they do not trigger the source-oriented release pipeline.

## Repository Variables

- `UPSTREAM_REPO`: upstream repository slug, default `maximhq/bifrost`
- `UPSTREAM_BRANCH`: upstream branch to track, default `main`
- `PATCH_BRANCH`: maintained fork branch to rebase and publish from
- `GHCR_REGISTRY`: optional container registry hostname, default `ghcr.io`
- `GHCR_IMAGE_NAME`: optional GHCR image path, default `<owner>/<repo>` lowercased
- `DOCKERHUB_IMAGE_NAME`: optional Docker Hub image path for public publication

## Repository Secrets

- Built-in `GITHUB_TOKEN`: used for pushing sync commits, creating tags, creating releases, and dispatching downstream workflows
- `DOCKERHUB_USERNAME` (optional, only needed when Docker Hub publication is enabled)
- `DOCKERHUB_TOKEN` (optional, only needed when Docker Hub publication is enabled)

## Workflows

- `fork-upstream-sync.yml`: scheduled and manual upstream rebase sync for the maintained branch
- `fork-sync-transport-release.yml`: detects the newest reachable upstream `transports/v*` tag on its schedule or via manual dispatch and creates or reuses the matching fork tag `vX.Y.Z-oss`
- `fork-transport-release.yml`: builds transport artifacts from an explicit fork tag such as `v1.5.2-oss`, packages multi-platform archives plus checksums, and attaches them to the fork GitHub release, while always overlaying the latest fork maintenance scripts from the maintained branch
- `fork-docker-publish.yml`: publishes fork multi-arch Docker images to GHCR by default and Docker Hub when configured, while always overlaying the latest fork maintenance scripts from the maintained branch; manual runs on `patched` with no tag publish `:dev`

## Manual Dry-Run Order

1. Run `Fork Upstream Sync` with `dry_run=true`.
2. Run `Fork Sync Transport Release` with `patch_branch=<PATCH_BRANCH>` and `dry_run=true`.
3. If a fork tag already exists, run `Fork Transport Release` with:
   - `tag=vX.Y.Z-oss`
   - `upstream_tag=transports/vX.Y.Z`
4. Run `Fork Docker Publish` with the same `tag` and `upstream_tag` values.
5. Enable workflow schedules only after all four manual runs succeed.

## Docker Tagging

- Mirrored upstream releases publish image tags such as `ghcr.io/<owner>/<repo>:v1.5.2-oss`.
- Manual `Fork Docker Publish` runs on the maintained `patched` branch with no `tag` input publish `ghcr.io/<owner>/<repo>:dev`.
- Only mirrored stable upstream releases publish `:latest`.

## Blocked Sync Recovery

1. Open the single tracking issue labeled `fork-upstream-sync-blocked`.
2. Check the linked diagnostic branch `fork-upstream-sync/<timestamp>`.
3. Inspect `.github/upstream-sync/summary-<timestamp>.md`, the saved patch, and the saved `git status` snapshot.
4. Fix the maintained branch locally, push the repair, and rerun `Fork Upstream Sync`.
5. A successful rerun closes the tracking issue automatically.
