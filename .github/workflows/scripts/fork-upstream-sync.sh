#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

# shellcheck source=/dev/null
source "$SCRIPT_DIR/fork-workflow-config.sh"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/fork-sync-state.sh"

TRACKING_ISSUE_LABEL="${TRACKING_ISSUE_LABEL:-fork-upstream-sync-blocked}"
SYNC_RELEASE_WORKFLOW="${SYNC_RELEASE_WORKFLOW:-fork-sync-transport-release.yml}"
DISPATCH_RELEASE_SYNC="${DISPATCH_RELEASE_SYNC:-true}"
DRY_RUN="${DRY_RUN:-false}"
REPO_SLUG="${GITHUB_REPOSITORY:-}"
RUN_URL=""

if [[ -n "${GITHUB_SERVER_URL:-}" && -n "${GITHUB_RUN_ID:-}" && -n "$REPO_SLUG" ]]; then
  RUN_URL="${GITHUB_SERVER_URL}/${REPO_SLUG}/actions/runs/${GITHUB_RUN_ID}"
fi

ensure_remote() {
  local remote_name="$1"
  local remote_url="$2"
  if git remote get-url "$remote_name" >/dev/null 2>&1; then
    git remote set-url "$remote_name" "$remote_url"
  else
    git remote add "$remote_name" "$remote_url"
  fi
}

latest_reachable_upstream_transport_tag() {
  local ref_name="$1"
  git tag --merged "$ref_name" --list 'transports/v*' \
    | grep -Ev -- '-[0-9]+$' \
    | sort -V \
    | tail -1 || true
}

ensure_tracking_label() {
  if [[ -z "${GH_TOKEN:-}" || -z "$REPO_SLUG" ]]; then
    return 0
  fi

  gh label create "$TRACKING_ISSUE_LABEL" \
    --repo "$REPO_SLUG" \
    --color "B60205" \
    --description "Automated fork upstream sync is blocked and needs maintainer attention." \
    2>/dev/null || true
}

close_tracking_issues() {
  if [[ -z "${GH_TOKEN:-}" || -z "$REPO_SLUG" ]]; then
    return 0
  fi

  local issue_numbers=""
  issue_numbers="$(gh issue list --repo "$REPO_SLUG" --state open --label "$TRACKING_ISSUE_LABEL" --json number --jq '.[].number' || true)"
  if [[ -z "$issue_numbers" ]]; then
    return 0
  fi

  while IFS= read -r issue_number; do
    [[ -z "$issue_number" ]] && continue
    gh issue close "$issue_number" \
      --repo "$REPO_SLUG" \
      --comment "Fork upstream sync is healthy again. Closing after a successful automated sync." || true
  done <<< "$issue_numbers"
}

update_tracking_issue() {
  local diagnostic_branch="$1"
  local upstream_head="$2"
  local conflict_files="$3"
  local log_tail="$4"
  local branch_url=""
  local issue_number=""
  local comment_body=""

  if [[ -z "${GH_TOKEN:-}" || -z "$REPO_SLUG" ]]; then
    return 0
  fi

  ensure_tracking_label
  branch_url="${GITHUB_SERVER_URL:-https://github.com}/${REPO_SLUG}/tree/${diagnostic_branch}"
  issue_number="$(gh issue list --repo "$REPO_SLUG" --state open --label "$TRACKING_ISSUE_LABEL" --json number --jq '.[0].number' || true)"

  comment_body=$(cat <<EOF
**New blocked fork sync attempt**

- Upstream repo: 
  
  `${UPSTREAM_REPO}`
- Upstream ref: 
  
  `${UPSTREAM_BRANCH}`
- Upstream head: 
  
  `${upstream_head}`
- Maintained branch: 
  
  `${PATCH_BRANCH}`
- Diagnostic branch: [${diagnostic_branch}](${branch_url})
- Workflow run: ${RUN_URL:-not available}

### Conflict Files

```
${conflict_files:-No unmerged file list captured.}
```

### Rebase Log Tail

```
${log_tail}
```
EOF
)

  if [[ -n "$issue_number" ]]; then
    gh issue comment "$issue_number" --repo "$REPO_SLUG" --body "$comment_body" >/dev/null
    return 0
  fi

  gh issue create \
    --repo "$REPO_SLUG" \
    --title "fork upstream sync blocked: maintainer action required" \
    --label "$TRACKING_ISSUE_LABEL" \
    --body "$comment_body" >/dev/null
}

record_sync_state() {
  local state_file="$1"
  local upstream_head="$2"
  local upstream_transport_tag="$3"
  local sync_message="$4"

  fork_set_sync_state_value UPSTREAM_REPO "$UPSTREAM_REPO"
  fork_set_sync_state_value UPSTREAM_BRANCH "$UPSTREAM_BRANCH"
  fork_set_sync_state_value UPSTREAM_COMMIT "$upstream_head"
  fork_set_sync_state_value UPSTREAM_TRANSPORT_TAG "$upstream_transport_tag"
  fork_write_sync_state "$state_file"

  git add "$state_file"
  if git diff --cached --quiet; then
    return 1
  fi

  git commit -m "$sync_message"
  return 0
}

push_branch() {
  local branch_name="$1"
  if [[ "$DRY_RUN" == "true" ]]; then
    echo "[dry-run] skipping push for ${branch_name}"
    return 0
  fi

  git push origin "$branch_name" --force-with-lease
}

dispatch_release_sync() {
  local patch_branch="$1"
  local upstream_tag="$2"

  if [[ "$DISPATCH_RELEASE_SYNC" != "true" || "$DRY_RUN" == "true" ]]; then
    return 0
  fi

  if [[ -z "${GH_TOKEN:-}" || -z "$REPO_SLUG" ]]; then
    return 0
  fi

  gh workflow run "$SYNC_RELEASE_WORKFLOW" \
    --repo "$REPO_SLUG" \
    --ref "$patch_branch" \
    -f patch_branch="$patch_branch" \
    -f upstream_tag="$upstream_tag" >/dev/null || true
}

main() {
  local state_file=""
  local current_head=""
  local upstream_ref=""
  local upstream_head=""
  local merge_base=""
  local upstream_transport_tag=""
  local rebase_log=""
  local log_tail=""
  local diagnostic_branch=""
  local diagnostics_dir=""
  local timestamp=""
  local conflict_files=""
  local sync_commit_message=""

  fork_require_command git
  fork_require_command gh
  fork_set_workflow_defaults

  cd "$PROJECT_ROOT"
  state_file="$(fork_sync_state_file)"
  fork_load_sync_state "$state_file"

  ensure_remote upstream "https://github.com/${UPSTREAM_REPO}.git"
  git fetch origin "$PATCH_BRANCH" --tags >/dev/null 2>&1 || true
  git fetch upstream "$UPSTREAM_BRANCH" --tags

  if git show-ref --verify --quiet "refs/remotes/origin/${PATCH_BRANCH}"; then
    git checkout -B "$PATCH_BRANCH" "origin/${PATCH_BRANCH}"
  else
    git checkout -B "$PATCH_BRANCH"
  fi

  current_head="$(git rev-parse HEAD)"
  upstream_ref="upstream/${UPSTREAM_BRANCH}"
  upstream_head="$(git rev-parse "$upstream_ref")"
  merge_base="$(git merge-base "$PATCH_BRANCH" "$upstream_ref")"
  upstream_transport_tag="$(latest_reachable_upstream_transport_tag "$PATCH_BRANCH")"
  sync_commit_message="chore(fork-upstream-sync): record ${UPSTREAM_BRANCH} at ${upstream_head:0:12} --skip-ci --skip-pipeline"

  if [[ "$upstream_head" == "$merge_base" ]]; then
    if record_sync_state "$state_file" "$upstream_head" "$upstream_transport_tag" "$sync_commit_message"; then
      push_branch "$PATCH_BRANCH"
    fi
    close_tracking_issues
    dispatch_release_sync "$PATCH_BRANCH" "$upstream_transport_tag"
    echo "Fork branch already contains upstream ${UPSTREAM_BRANCH}; no rebase needed."
    return 0
  fi

  rebase_log="$(mktemp)"
  set +e
  git rebase --rebase-merges "$upstream_ref" 2>&1 | tee "$rebase_log"
  local rebase_status=${PIPESTATUS[0]}
  set -e

  if [[ $rebase_status -ne 0 ]]; then
    timestamp="$(date -u +%Y%m%d-%H%M%S)"
    diagnostic_branch="fork-upstream-sync/${timestamp}"
    diagnostics_dir=".github/upstream-sync"
    conflict_files="$(git diff --name-only --diff-filter=U || true)"
    log_tail="$(tail -n 80 "$rebase_log")"

    mkdir -p "/tmp/fork-upstream-sync"
    git diff >"/tmp/fork-upstream-sync/rebase-${timestamp}.patch" || true
    git status --short >"/tmp/fork-upstream-sync/status-${timestamp}.txt" || true

    git rebase --abort || true
    git checkout -B "$diagnostic_branch" "$current_head"
    mkdir -p "$diagnostics_dir"
    cp "/tmp/fork-upstream-sync/rebase-${timestamp}.patch" "$diagnostics_dir/rebase-${timestamp}.patch"
    cp "/tmp/fork-upstream-sync/status-${timestamp}.txt" "$diagnostics_dir/status-${timestamp}.txt"
    cat >"$diagnostics_dir/summary-${timestamp}.md" <<EOF
# Fork Upstream Sync Failure Snapshot

- Upstream repo: `${UPSTREAM_REPO}`
- Upstream branch: `${UPSTREAM_BRANCH}`
- Upstream head: `${upstream_head}`
- Maintained branch: `${PATCH_BRANCH}`

## Conflict Files

```
${conflict_files:-No unmerged files were captured.}
```

## Workflow Run

${RUN_URL:-Not available}
EOF
    git add "$diagnostics_dir"
    git commit -m "chore(fork-upstream-sync): snapshot blocked rebase against ${UPSTREAM_BRANCH} --skip-ci --skip-pipeline"
    push_branch "$diagnostic_branch"
    update_tracking_issue "$diagnostic_branch" "$upstream_head" "$conflict_files" "$log_tail"
    echo "Fork upstream sync failed and requires maintainer action."
    exit 1
  fi

  upstream_transport_tag="$(latest_reachable_upstream_transport_tag HEAD)"
  if record_sync_state "$state_file" "$upstream_head" "$upstream_transport_tag" "$sync_commit_message"; then
    :
  fi

  push_branch "$PATCH_BRANCH"
  close_tracking_issues
  dispatch_release_sync "$PATCH_BRANCH" "$upstream_transport_tag"
  echo "Fork upstream sync completed successfully."
}

main "$@"
