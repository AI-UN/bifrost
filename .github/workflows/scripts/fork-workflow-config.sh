#!/usr/bin/env bash

fork_detect_default_branch() {
  if [[ -n "${REPOSITORY_DEFAULT_BRANCH:-}" ]]; then
    printf '%s\n' "$REPOSITORY_DEFAULT_BRANCH"
    return 0
  fi

  local remote_head=""
  remote_head="$(git symbolic-ref refs/remotes/origin/HEAD 2>/dev/null || true)"
  if [[ -n "$remote_head" ]]; then
    printf '%s\n' "${remote_head##refs/remotes/origin/}"
    return 0
  fi

  printf 'main\n'
}

fork_set_workflow_defaults() {
  local detected_default_branch=""
  detected_default_branch="$(fork_detect_default_branch)"

  UPSTREAM_REPO="${UPSTREAM_REPO:-maximhq/bifrost}"
  UPSTREAM_BRANCH="${UPSTREAM_BRANCH:-main}"
  PATCH_BRANCH="${PATCH_BRANCH:-$detected_default_branch}"
  GHCR_REGISTRY="${GHCR_REGISTRY:-ghcr.io}"

  local default_ghcr_image="${GITHUB_REPOSITORY:-bifrost}"
  default_ghcr_image="${default_ghcr_image,,}"
  GHCR_IMAGE_NAME="${GHCR_IMAGE_NAME:-$default_ghcr_image}"
  GHCR_IMAGE_NAME="${GHCR_IMAGE_NAME,,}"

  DOCKERHUB_IMAGE_NAME="${DOCKERHUB_IMAGE_NAME:-}"
  DOCKERHUB_IMAGE_NAME="${DOCKERHUB_IMAGE_NAME,,}"
  DOCKERHUB_USERNAME="${DOCKERHUB_USERNAME:-}"
  DOCKERHUB_TOKEN="${DOCKERHUB_TOKEN:-}"

  export UPSTREAM_REPO UPSTREAM_BRANCH PATCH_BRANCH GHCR_REGISTRY GHCR_IMAGE_NAME
  export DOCKERHUB_IMAGE_NAME DOCKERHUB_USERNAME DOCKERHUB_TOKEN
}

fork_print_workflow_config() {
  cat <<EOF
UPSTREAM_REPO=${UPSTREAM_REPO}
UPSTREAM_BRANCH=${UPSTREAM_BRANCH}
PATCH_BRANCH=${PATCH_BRANCH}
GHCR_REGISTRY=${GHCR_REGISTRY}
GHCR_IMAGE_NAME=${GHCR_IMAGE_NAME}
DOCKERHUB_IMAGE_NAME=${DOCKERHUB_IMAGE_NAME}
EOF
}

fork_require_command() {
  local cmd="$1"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "required command not found: $cmd" >&2
    return 1
  fi
}

fork_has_dockerhub_publish_config() {
  [[ -n "${DOCKERHUB_IMAGE_NAME:-}" && -n "${DOCKERHUB_TOKEN:-}" ]]
}
