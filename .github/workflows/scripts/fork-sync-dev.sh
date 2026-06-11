#!/usr/bin/env bash
set -euo pipefail

source ./.github/workflows/scripts/fork-workflow-config.sh
source ./.github/workflows/scripts/fork-sync-state.sh

fork_set_workflow_defaults

DRY_RUN="${DRY_RUN:-false}"
UPSTREAM_TAG="${UPSTREAM_TAG:-}"
SYNC_CONFLICT_TITLE="[fork-sync-conflict] ${PATCH_BRANCH} cannot rebase onto upstream/${UPSTREAM_BRANCH}"
TAG_CONFLICT_TITLE_PREFIX="[fork-tag-conflict]"

run_url() {
  if [[ -n "${GITHUB_SERVER_URL:-}" && -n "${GITHUB_REPOSITORY:-}" && -n "${GITHUB_RUN_ID:-}" ]]; then
    printf '%s/%s/actions/runs/%s\n' "$GITHUB_SERVER_URL" "$GITHUB_REPOSITORY" "$GITHUB_RUN_ID"
  fi
}

issue_number_by_title() {
  local title="$1"
  gh issue list --state open --json number,title \
    | python3 -c 'import json, sys; title = sys.argv[1]; print(next((str(issue["number"]) for issue in json.load(sys.stdin) if issue["title"] == title), ""))' "$title"
}

create_or_update_issue() {
  local title="$1"
  local body="$2"
  local issue_number=""

  issue_number="$(issue_number_by_title "$title" 2>/dev/null || true)"
  if [[ -n "$issue_number" ]]; then
    gh issue comment "$issue_number" --body "$body" >/dev/null || true
    echo "Updated existing issue #${issue_number}: ${title}"
    return 0
  fi

  gh issue create --title "$title" --body "$body" --label automated --label conflict >/dev/null \
    || gh issue create --title "$title" --body "$body" >/dev/null \
    || echo "Failed to create issue: ${title}" >&2
}

close_issue_if_open() {
  local title="$1"
  local body="$2"
  local issue_number=""

  issue_number="$(issue_number_by_title "$title" 2>/dev/null || true)"
  if [[ -n "$issue_number" ]]; then
    gh issue close "$issue_number" --comment "$body" >/dev/null || true
    echo "Closed resolved issue #${issue_number}: ${title}"
  fi
}

upstream_remote_url() {
  if [[ "$UPSTREAM_REPO" == http://* || "$UPSTREAM_REPO" == https://* || "$UPSTREAM_REPO" == git@* ]]; then
    printf '%s\n' "$UPSTREAM_REPO"
  else
    printf 'https://github.com/%s.git\n' "$UPSTREAM_REPO"
  fi
}

ensure_remotes() {
  local upstream_url=""
  upstream_url="$(upstream_remote_url)"
  if git remote get-url upstream >/dev/null 2>&1; then
    git remote set-url upstream "$upstream_url"
  else
    git remote add upstream "$upstream_url"
  fi

  git fetch origin --prune --tags --force
  git fetch upstream "$UPSTREAM_BRANCH" --tags --force
}

rebase_patch_source() {
  local target_ref="$1"
  local work_branch="$2"

  git checkout -B "$PATCH_BRANCH" "origin/${PATCH_BRANCH}"
  git checkout -B "$work_branch" "$PATCH_BRANCH"
  git rebase --rebase-merges "$target_ref"
}

sync_generated_branch() {
  local conflict_body=""
  local status_output=""
  local run=""

  echo "Rebasing ${PATCH_BRANCH} onto upstream/${UPSTREAM_BRANCH} for ${GENERATED_BRANCH}"
  if rebase_patch_source "upstream/${UPSTREAM_BRANCH}" "$GENERATED_BRANCH"; then
    close_issue_if_open "$SYNC_CONFLICT_TITLE" "Resolved by successful run: $(run_url)"
    if [[ "$DRY_RUN" == "true" ]]; then
      echo "Dry run: skipping push of ${GENERATED_BRANCH}"
    else
      git push origin "HEAD:refs/heads/${GENERATED_BRANCH}" --force-with-lease
    fi
    return 0
  fi

  status_output="$(git status --short || true)"
  run="$(run_url)"
  conflict_body="$(cat <<EOF
## patched-dev sync conflict

The fork sync workflow could not rebase \\`${PATCH_BRANCH}\\` onto \\`upstream/${UPSTREAM_BRANCH}\\`.

- Patch source: \\`${PATCH_BRANCH}\\`
- Generated branch: \\`${GENERATED_BRANCH}\\`
- Upstream target: \\`upstream/${UPSTREAM_BRANCH}\\`
- Run: ${run:-unavailable}

### Git status

\\`\\`\\`text
${status_output}
\\`\\`\\`
EOF
)"
  git rebase --abort || true
  create_or_update_issue "$SYNC_CONFLICT_TITLE" "$conflict_body"
  return 1
}

resolve_upstream_tag() {
  if [[ -n "$UPSTREAM_TAG" ]]; then
    printf '%s\n' "$UPSTREAM_TAG"
    return 0
  fi

  fork_latest_reachable_upstream_transport_tag "upstream/${UPSTREAM_BRANCH}"
}

fork_tag_exists() {
  local fork_tag="$1"
  git rev-parse -q --verify "refs/tags/${fork_tag}" >/dev/null \
    || git ls-remote --exit-code --tags origin "refs/tags/${fork_tag}" >/dev/null 2>&1
}

sync_release_tag() {
  local upstream_tag="$1"
  local fork_tag=""
  local release_branch=""
  local title=""
  local body=""
  local status_output=""
  local run=""

  if [[ -z "$upstream_tag" ]]; then
    echo "No upstream transport tag found; skipping release tag sync."
    return 0
  fi

  if ! git rev-parse -q --verify "refs/tags/${upstream_tag}" >/dev/null; then
    echo "Upstream tag not found locally: ${upstream_tag}" >&2
    return 1
  fi

  fork_tag="$(fork_fork_tag_from_upstream_tag "$upstream_tag")"
  title="${TAG_CONFLICT_TITLE_PREFIX} ${fork_tag} cannot be created from ${upstream_tag}"
  if fork_tag_exists "$fork_tag"; then
    echo "Fork tag already exists: ${fork_tag}"
    close_issue_if_open "$title" "Resolved because ${fork_tag} now exists. Run: $(run_url)"
    return 0
  fi

  release_branch="fork-release/${fork_tag}"
  echo "Rebasing ${PATCH_BRANCH} onto ${upstream_tag} for ${fork_tag}"
  if rebase_patch_source "$upstream_tag" "$release_branch"; then
    git tag -a "$fork_tag" -m "Fork release ${fork_tag} from ${upstream_tag}"
    close_issue_if_open "$title" "Resolved by successful tag creation: $(run_url)"
    if [[ "$DRY_RUN" == "true" ]]; then
      echo "Dry run: skipping push and release workflow dispatch for ${fork_tag}"
    else
      git push origin "refs/tags/${fork_tag}"
      gh workflow run fork-release-cli.yml --ref "$GENERATED_BRANCH" -f "tag=${fork_tag}" -f "upstream_tag=${upstream_tag}"
      gh workflow run fork-release-docker.yml --ref "$GENERATED_BRANCH" -f "tag=${fork_tag}" -f "upstream_tag=${upstream_tag}"
    fi
    return 0
  fi

  status_output="$(git status --short || true)"
  run="$(run_url)"
  body="$(cat <<EOF
## Fork release tag conflict

The fork sync workflow could not rebase \\`${PATCH_BRANCH}\\` onto upstream tag \\`${upstream_tag}\\` to create \\`${fork_tag}\\`.

- Patch source: \\`${PATCH_BRANCH}\\`
- Upstream tag: \\`${upstream_tag}\\`
- Fork tag: \\`${fork_tag}\\`
- Run: ${run:-unavailable}

### Git status

\\`\\`\\`text
${status_output}
\\`\\`\\`
EOF
)"
  git rebase --abort || true
  create_or_update_issue "$title" "$body"
  return 1
}

main() {
  fork_print_workflow_config
  ensure_remotes
  sync_generated_branch
  sync_release_tag "$(resolve_upstream_tag)"
}

main "$@"
