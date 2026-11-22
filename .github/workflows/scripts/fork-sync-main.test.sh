#!/usr/bin/env bash
# Regression test for fork-sync-main.sh: the release-source guard, and carrying
# upstream changes onto a patch source whose merges needed hand resolutions.
#
# Builds a throwaway upstream and fork in a temp dir, where upstream/main has
# already moved to 2.5.0 past transports/v2.4.0 and the 2.4.0 release is kept
# on release/v2.4.0-oss, then runs the real script in dry-run mode and checks
# which fork tag it would cut, and from what.
#
# Usage: ./fork-sync-main.test.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORKFLOWS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
ROOT="$(mktemp -d)"
trap 'rm -rf "$ROOT"' EXIT

export GIT_AUTHOR_NAME=test GIT_AUTHOR_EMAIL=test@example.com
export GIT_COMMITTER_NAME=test GIT_COMMITTER_EMAIL=test@example.com
export GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null

failures=0
fail() {
  echo "FAIL: $*" >&2
  failures=$((failures + 1))
}

# Upstream: transports/v2.4.0, then main bumps transports/version to 2.5.0.
git init -q -b main "$ROOT/up"
(
  cd "$ROOT/up"
  mkdir -p transports
  echo 2.4.0 > transports/version
  git add . && git commit -qm "v2.4.0"
  git tag transports/v2.4.0
)

# Fork: automation + one fork patch on v2.4.0, kept as release/v2.4.0-oss.
git init -q --bare "$ROOT/origin.git"
git clone -q "$ROOT/up" "$ROOT/work"
(
  cd "$ROOT/work"
  git remote rename origin upstream
  git remote add origin "$ROOT/origin.git"
  mkdir -p .github/workflows/scripts
  cp "$SCRIPT_DIR"/*.sh .github/workflows/scripts/
  cp "$WORKFLOWS_DIR"/fork-sync-main.yml "$WORKFLOWS_DIR"/fork-release-cli.yml "$WORKFLOWS_DIR"/fork-release-docker.yml .github/workflows/
  touch .fork-upstream-sync.env
  git add . && git commit -qm "ci: fork automation"
  echo fork > fork.txt && git add . && git commit -qm "fork patch"
  git push -q origin HEAD:refs/heads/release/v2.4.0-oss
)
(
  cd "$ROOT/up"
  echo 2.5.0 > transports/version && git commit -qam "v2.5.0"
)
(
  cd "$ROOT/work"
  git fetch -q upstream
  git rebase -q upstream/main
  git push -q origin HEAD:refs/heads/patched HEAD:refs/heads/patched-main
)

mkdir -p "$ROOT/bin"
printf '#!/bin/sh\nexit 0\n' > "$ROOT/bin/gh"
chmod +x "$ROOT/bin/gh"
export PATH="$ROOT/bin:$PATH"
export UPSTREAM_REPO=fork-sync-test/upstream UPSTREAM_BRANCH=main PATCH_BRANCH=patched GENERATED_BRANCH=patched-main DRY_RUN=true
export GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0="url.$ROOT/up.insteadOf" GIT_CONFIG_VALUE_0="https://github.com/fork-sync-test/upstream.git"

# run_sync <env assignments...>: runs the script in a fresh clone; sets RC and RUN_DIR.
run_sync() {
  RUN_DIR="$ROOT/run-$RANDOM"
  git clone -q "$ROOT/origin.git" "$RUN_DIR" 2>/dev/null
  git -C "$RUN_DIR" checkout -q patched
  set +e
  (cd "$RUN_DIR" && env "$@" bash ./.github/workflows/scripts/fork-sync-main.sh > "$RUN_DIR.log" 2>&1)
  RC=$?
  set -e
}

tag_version() {
  git -C "$RUN_DIR" show "$1:transports/version" 2>/dev/null || echo none
}

# The nightly run must not cut v2.4.0-oss from a patch source already on 2.5.0.
run_sync UPSTREAM_TAG=
[[ "$RC" == 0 ]] || fail "nightly run exited $RC: $(cat "$RUN_DIR.log")"
[[ "$(tag_version v2.4.0-oss)" == none ]] || fail "nightly run cut v2.4.0-oss from 2.5.0 code"

# An explicit tag with a mismatched source fails instead of mislabelling the release.
run_sync UPSTREAM_TAG=transports/v2.4.0
[[ "$RC" != 0 ]] || fail "explicit v2.4.0 release from patched succeeded"
[[ "$(tag_version v2.4.0-oss)" == none ]] || fail "explicit run cut v2.4.0-oss from 2.5.0 code"

# The preserved stable source releases the older tag.
run_sync UPSTREAM_TAG=transports/v2.4.0 RELEASE_SOURCE_BRANCH=release/v2.4.0-oss
[[ "$RC" == 0 ]] || fail "stable-source release exited $RC: $(cat "$RUN_DIR.log")"
[[ "$(tag_version v2.4.0-oss)" == 2.4.0 ]] || fail "v2.4.0-oss does not carry 2.4.0"
git -C "$RUN_DIR" show v2.4.0-oss:fork.txt >/dev/null 2>&1 || fail "v2.4.0-oss lost the fork patch"

# Once upstream tags 2.5.0, the nightly run releases the patch source as v2.5.0-oss.
git -C "$ROOT/up" tag transports/v2.5.0
run_sync UPSTREAM_TAG=
[[ "$RC" == 0 ]] || fail "nightly run after v2.5.0 exited $RC: $(cat "$RUN_DIR.log")"
[[ "$(tag_version v2.5.0-oss)" == 2.5.0 ]] || fail "nightly run did not cut v2.5.0-oss from 2.5.0 code"

# Two patch branches append to the same file, so merging the second into patched
# needed a hand resolution that `git rebase --rebase-merges` cannot replay.
(
  cd "$ROOT/work"
  git checkout -q -B patched
  printf 'a\n' > list.txt && git add list.txt && git commit -qm "fork: list"
  git checkout -q -b b1 && echo b1 >> list.txt && git commit -qam b1
  git checkout -q -b b2 patched && echo b2 >> list.txt && git commit -qam b2
  git checkout -q patched
  git merge -q --no-ff -m "Merge b1" b1
  git merge -q --no-ff -m "Merge b2" b2 >/dev/null 2>&1 \
    || { printf 'a\nb1\nb2\n' > list.txt && git add list.txt && git commit -q --no-edit; }
  git push -q -f origin patched:refs/heads/patched
)
# Upstream branches: one that misses the fork's lines, one that collides with
# them, and one that already contains the fork's own patch.
(
  cd "$ROOT/up"
  git checkout -q -b docs main && echo docs > docs.txt && git add docs.txt && git commit -qm docs
  git checkout -q -b conflict main && printf 'a\nupstream\n' > list.txt && git add list.txt && git commit -qm conflict
  git checkout -q -b absorbed main && echo fork > fork.txt && git add fork.txt && git commit -qm absorbed
  git checkout -q main
)

# An upstream change the fork does not touch is carried without replaying merges.
run_sync UPSTREAM_TAG= UPSTREAM_BRANCH=docs
[[ "$RC" == 0 ]] || fail "docs-only upstream advance exited $RC: $(cat "$RUN_DIR.log")"
(
  cd "$RUN_DIR"
  git checkout -q -B probe origin/patched
  if git rebase -q --rebase-merges upstream/docs >/dev/null 2>&1; then
    echo "FAIL: fixture: rebase --rebase-merges did not conflict, so the carry path is unproven" >&2
    exit 1
  fi
  git rebase --abort
) || failures=$((failures + 1))
[[ "$(git -C "$RUN_DIR" show patched-main:docs.txt 2>/dev/null)" == docs ]] || fail "patched-main lacks the upstream change"
[[ "$(git -C "$RUN_DIR" show patched-main:list.txt 2>/dev/null)" == $'a\nb1\nb2' ]] || fail "patched-main lost the hand-resolved merge"
git -C "$RUN_DIR" merge-base --is-ancestor origin/patched^2 patched-main || fail "patched-main dropped the patch-branch parent"
grep -q '^Carry: rebuilt' "$RUN_DIR.log" || fail "docs-only advance did not take the carry path"

# Upstream editing the fork's lines still fails and opens the conflict issue.
run_sync UPSTREAM_TAG= UPSTREAM_BRANCH=conflict
[[ "$RC" != 0 ]] || fail "conflicting upstream advance succeeded"
grep -q '^Carry: upstream changes conflict' "$RUN_DIR.log" || fail "conflict was not reported by the carry path"

# A clean merge that changes the fork's own delta is refused, not published.
run_sync UPSTREAM_TAG= UPSTREAM_BRANCH=absorbed
[[ "$RC" != 0 ]] || fail "upstream absorbing a fork patch was published silently"
grep -q 'Fork changes differ in `fork.txt`' "$RUN_DIR.log" || fail "absorbed fork patch not named: $(cat "$RUN_DIR.log")"
if grep -q 'Falling back to rebase' "$RUN_DIR.log"; then fail "fork delta mismatch fell back to rebase"; fi

if (( failures > 0 )); then
  echo "$failures check(s) failed" >&2
  exit 1
fi
echo "fork-sync-main: all checks passed"
