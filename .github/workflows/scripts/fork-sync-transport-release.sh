#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

# shellcheck source=/dev/null
source "$SCRIPT_DIR/fork-workflow-config.sh"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/fork-sync-state.sh"

DRY_RUN="${DRY_RUN:-false}"
REPO_SLUG="${GITHUB_REPOSITORY:-}"
UPSTREAM_TAG_INPUT="${UPSTREAM_TAG:-}"
RELEASE_WORKFLOW_NAME="${RELEASE_WORKFLOW_NAME:-fork-transport-release.yml}"
DOCKER_WORKFLOW_NAME="${DOCKER_WORKFLOW_NAME:-fork-docker-publish.yml}"

ensure_remote() {
  local remote_name="$1"
  local remote_url="$2"
  if git remote get-url "$remote_name" >/dev/null 2>&1; then
    git remote set-url "$remote_name" "$remote_url"
  else
    git remote add "$remote_name" "$remote_url"
  fi
}

release_exists() {
  local release_tag="$1"
  [[ -n "${GH_TOKEN:-}" && -n "$REPO_SLUG" ]] || return 1
  gh release view "$release_tag" --repo "$REPO_SLUG" >/dev/null 2>&1
}

docker_output_exists() {
  local fork_tag="$1"
  local version=""
  local image_ref=""

  version="$(fork_transport_version_from_tag "$fork_tag")"
  image_ref="${GHCR_REGISTRY}/${GHCR_IMAGE_NAME}:v${version}"

  if command -v docker >/dev/null 2>&1 && [[ -n "${GH_TOKEN:-}" && -n "${GITHUB_ACTOR:-}" ]]; then
    printf '%s' "$GH_TOKEN" | docker login "$GHCR_REGISTRY" -u "$GITHUB_ACTOR" --password-stdin >/dev/null 2>&1 || true
  fi

  docker manifest inspect "$image_ref" >/dev/null 2>&1
}

remote_tag_exists() {
  local tag_name="$1"
  git ls-remote --exit-code --tags origin "refs/tags/${tag_name}" >/dev/null 2>&1
}

dispatch_downstream_workflows() {
  local fork_tag="$1"
  local upstream_tag="$2"
  local dispatch_release="${3:-true}"
  local dispatch_docker="${4:-true}"

  if [[ "$DRY_RUN" == "true" || -z "${GH_TOKEN:-}" || -z "$REPO_SLUG" ]]; then
    return 0
  fi

  if [[ "$dispatch_release" == "true" ]]; then
    gh workflow run "$RELEASE_WORKFLOW_NAME" \
      --repo "$REPO_SLUG" \
      --ref "$PATCH_BRANCH" \
      -f tag="$fork_tag" \
      -f upstream_tag="$upstream_tag" >/dev/null
  fi

  if [[ "$dispatch_docker" == "true" ]]; then
    gh workflow run "$DOCKER_WORKFLOW_NAME" \
      --repo "$REPO_SLUG" \
      --ref "$PATCH_BRANCH" \
      -f tag="$fork_tag" \
      -f upstream_tag="$upstream_tag" >/dev/null
  fi
}

record_release_sync_state() {
  local state_file="$1"
  local upstream_tag="$2"
  local fork_tag="$3"
  local commit_message="$4"

  fork_set_sync_state_value UPSTREAM_REPO "$UPSTREAM_REPO"
  fork_set_sync_state_value UPSTREAM_BRANCH "$UPSTREAM_BRANCH"
  fork_set_sync_state_value UPSTREAM_TRANSPORT_TAG "$upstream_tag"
  fork_set_sync_state_value LAST_RELEASED_UPSTREAM_TRANSPORT_TAG "$upstream_tag"
  fork_set_sync_state_value FORK_TRANSPORT_TAG "$fork_tag"
  fork_write_sync_state "$state_file"

  git add "$state_file"
  if git diff --cached --quiet; then
    return 1
  fi

  git commit -m "$commit_message"
  return 0
}

main() {
  local state_file=""
  local latest_upstream_tag=""
  local fork_tag=""
  local tag_created="false"
  local release_missing="false"
  local docker_missing="false"
  local state_changed="false"
  local commit_message=""
  local dispatch_release="false"
  local dispatch_docker="false"

  fork_require_command git
  fork_require_command gh
  fork_set_workflow_defaults

  if [[ "${GITHUB_EVENT_NAME:-}" == "push" && -n "${GITHUB_REF_NAME:-}" && "${GITHUB_REF_NAME}" != "$PATCH_BRANCH" ]]; then
    echo "Push event is for ${GITHUB_REF_NAME}, not maintained branch ${PATCH_BRANCH}; skipping transport release sync."
    return 0
  fi

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

  if [[ -n "$UPSTREAM_TAG_INPUT" ]]; then
    if ! git rev-parse "$UPSTREAM_TAG_INPUT" >/dev/null 2>&1; then
      echo "Input upstream tag ${UPSTREAM_TAG_INPUT} does not exist locally after fetch; skipping."
      return 0
    fi

    if git merge-base --is-ancestor "$(git rev-list -n 1 "$UPSTREAM_TAG_INPUT")" HEAD; then
      latest_upstream_tag="$UPSTREAM_TAG_INPUT"
    else
      echo "Input upstream tag ${UPSTREAM_TAG_INPUT} is not reachable from ${PATCH_BRANCH}; skipping."
      return 0
    fi
  else
    latest_upstream_tag="$(fork_latest_reachable_upstream_transport_tag HEAD)"
  fi

  if [[ -z "$latest_upstream_tag" ]]; then
    echo "No reachable upstream transport tag found on ${PATCH_BRANCH}."
    return 0
  fi

  fork_tag="$(fork_fork_tag_from_upstream_tag "$latest_upstream_tag")"
  commit_message="chore(fork-upstream-release-sync): record ${fork_tag} from ${latest_upstream_tag} --skip-ci --skip-pipeline"

  if ! remote_tag_exists "$fork_tag"; then
    if [[ "$DRY_RUN" == "true" ]]; then
      echo "[dry-run] would create tag ${fork_tag}"
    else
      if ! git rev-parse "$fork_tag" >/dev/null 2>&1; then
        git tag -a "$fork_tag" -m "Fork release ${fork_tag} (source ${latest_upstream_tag})"
      fi
      git push origin "refs/tags/${fork_tag}"
    fi
    tag_created="true"
  fi

  if ! release_exists "$fork_tag"; then
    release_missing="true"
  fi

  if ! docker_output_exists "$fork_tag"; then
    docker_missing="true"
  fi

  if record_release_sync_state "$state_file" "$latest_upstream_tag" "$fork_tag" "$commit_message"; then
    state_changed="true"
  fi

  if [[ "$state_changed" == "true" && "$DRY_RUN" != "true" ]]; then
    git push origin "$PATCH_BRANCH" --force-with-lease
  fi

  if [[ "$tag_created" == "true" ]]; then
    dispatch_downstream_workflows "$fork_tag" "$latest_upstream_tag" "true" "true"
    echo "Fork tag ${fork_tag} was created; dispatched downstream release workflows explicitly."
    return 0
  fi

  if [[ "$release_missing" == "true" ]]; then
    dispatch_release="true"
  fi

  if [[ "$docker_missing" == "true" ]]; then
    dispatch_docker="true"
  fi

  if [[ "$dispatch_release" == "true" || "$dispatch_docker" == "true" ]]; then
    dispatch_downstream_workflows "$fork_tag" "$latest_upstream_tag" "$dispatch_release" "$dispatch_docker"
    echo "Fork transport release sync dispatched downstream workflows for ${fork_tag}."
    return 0
  fi

  echo "Fork transport release state already matches reachable upstream tag ${latest_upstream_tag}."
}

main "$@"
