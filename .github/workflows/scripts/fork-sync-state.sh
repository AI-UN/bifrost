#!/usr/bin/env bash

fork_sync_state_file() {
  printf '%s\n' "${FORK_SYNC_STATE_FILE:-.fork-upstream-sync.env}"
}

fork_sync_state_keys() {
  cat <<'EOF'
UPSTREAM_REPO
UPSTREAM_BRANCH
UPSTREAM_COMMIT
UPSTREAM_TRANSPORT_TAG
LAST_RELEASED_UPSTREAM_TRANSPORT_TAG
FORK_TRANSPORT_TAG
EOF
}

fork_load_sync_state() {
  local state_file="${1:-$(fork_sync_state_file)}"
  if [[ ! -f "$state_file" ]]; then
    echo "fork sync state file not found: $state_file" >&2
    return 1
  fi

  while IFS= read -r key; do
    export "$key"=""
  done < <(fork_sync_state_keys)

  # shellcheck disable=SC1090
  source "$state_file"

  while IFS= read -r key; do
    export "$key"
  done < <(fork_sync_state_keys)
}

fork_write_sync_state() {
  local state_file="${1:-$(fork_sync_state_file)}"
  local tmp_file=""
  tmp_file="$(mktemp)"

  {
    printf '# Tracked fork upstream sync state.\n'
    printf '# This file is updated by fork maintenance workflows.\n'
    while IFS= read -r key; do
      printf '%s=%q\n' "$key" "${!key:-}"
    done < <(fork_sync_state_keys)
  } >"$tmp_file"

  mv "$tmp_file" "$state_file"
}

fork_set_sync_state_value() {
  local key="$1"
  local value="$2"
  export "$key=$value"
}

fork_transport_version_from_tag() {
  local tag="$1"
  tag="${tag#transports/}"
  tag="${tag#v}"
  printf '%s\n' "$tag"
}

fork_fork_tag_from_upstream_tag() {
  local upstream_tag="$1"
  local version=""
  version="$(fork_transport_version_from_tag "$upstream_tag")"
  printf 'v%s-oss\n' "$version"
}

fork_list_upstream_transport_tags() {
  git tag --list 'transports/v*' | grep -Ev -- '-[0-9]+$' || true
}

fork_latest_reachable_upstream_transport_tag() {
  local ref_name="${1:-HEAD}"
  git tag --merged "$ref_name" --list 'transports/v*' \
    | grep -Ev -- '-[0-9]+$' \
    | sort -V \
    | tail -1 || true
}

fork_upstream_tag_from_fork_tag() {
  local fork_tag="$1"
  local version=""
  version="$(fork_transport_version_from_tag "$fork_tag")"
  if [[ "$version" == "dev" ]]; then
    return 1
  fi
  version="${version%-oss}"
  printf 'transports/v%s\n' "$version"
}

fork_source_tag_is_stable() {
  local source_tag="$1"
  local version=""
  version="$(fork_transport_version_from_tag "$source_tag")"
  [[ "$version" != *-* ]]
}
