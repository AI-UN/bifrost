#!/usr/bin/env bash
set -euo pipefail

# Finalize bifrost-http release: changelog, tagging, GitHub release, optional release assets, optional R2 latest copy
# Usage: ./release-bifrost-http-finalize.sh <version>

# Validate input argument
if [ "${1:-}" = "" ]; then
  echo "Usage: $0 <version>" >&2
  exit 1
fi

VERSION="$1"
TAG_NAME="${TRANSPORT_TAG_NAME:-transports/v${VERSION}}"
UPSTREAM_SOURCE_TAG="${UPSTREAM_SOURCE_TAG:-$TAG_NAME}"
UPSTREAM_REPO_SLUG="${UPSTREAM_REPO_SLUG:-}"
SKIP_GIT_TAG_CREATE="${SKIP_GIT_TAG_CREATE:-false}"
ALLOW_SAME_CHANGELOG="${ALLOW_SAME_CHANGELOG:-false}"
DOCKER_IMAGE_REFERENCE="${DOCKER_IMAGE_REFERENCE:-maximhq/bifrost}"
RELEASE_ASSET_DIR="${RELEASE_ASSET_DIR:-}"
RELEASE_TITLE_OVERRIDE="${RELEASE_TITLE_OVERRIDE:-}"

echo "🏷️ Finalizing bifrost-http v$VERSION release..."

release_assets=()
if [[ -n "$RELEASE_ASSET_DIR" ]]; then
  if [[ ! -d "$RELEASE_ASSET_DIR" ]]; then
    echo "❌ Release asset directory not found: $RELEASE_ASSET_DIR" >&2
    exit 1
  fi

  shopt -s nullglob
  for asset_path in "$RELEASE_ASSET_DIR"/*; do
    if [[ -f "$asset_path" ]]; then
      release_assets+=("$asset_path")
    fi
  done
  shopt -u nullglob

  if [[ ${#release_assets[@]} -eq 0 ]]; then
    echo "❌ Release asset directory is empty: $RELEASE_ASSET_DIR" >&2
    exit 1
  fi
fi

# Get core and framework versions from version files
CORE_VERSION="v$(tr -d '\n\r' < core/version)"
FRAMEWORK_VERSION="v$(tr -d '\n\r' < framework/version)"

# Re-compute plugin versions from version files and transports/go.mod
declare -A PLUGIN_VERSIONS
PLUGINS_USED=()

for plugin_dir in plugins/*/; do
  if [ -d "$plugin_dir" ]; then
    plugin_name=$(basename "$plugin_dir")
    PLUGIN_VERSION="v$(tr -d '\n\r' < "${plugin_dir}version")"
    PLUGIN_VERSIONS["$plugin_name"]="$PLUGIN_VERSION"
  fi
done

# Check which plugins are actually used by the transport
while IFS= read -r plugin_line; do
  plugin_name=$(echo "$plugin_line" | awk -F'/' '{print $NF}' | awk '{print $1}')
  plugin_version=$(echo "$plugin_line" | awk '{print $NF}')

  # Use version file version if available, otherwise use go.mod version
  if [[ -n "${PLUGIN_VERSIONS[$plugin_name]:-}" ]]; then
    PLUGINS_USED+=("$plugin_name:${PLUGIN_VERSIONS[$plugin_name]}")
  else
    PLUGIN_VERSIONS["$plugin_name"]="$plugin_version"
    PLUGINS_USED+=("$plugin_name:$plugin_version")
  fi
done < <(grep "github.com/maximhq/bifrost/plugins/" transports/go.mod)

echo "🔧 Versions:"
echo "   Core: $CORE_VERSION"
echo "   Framework: $FRAMEWORK_VERSION"
echo "   Plugins:"
for plugin_name in "${!PLUGIN_VERSIONS[@]}"; do
  echo "     - $plugin_name: ${PLUGIN_VERSIONS[$plugin_name]}"
done

# Remove Markdown comments before validating or publishing changelog content.
strip_changelog_comments() {
  sed '/<!--/,/-->/d'
}

changelog_is_empty() {
  [[ -z "${1//[[:space:]]/}" ]]
}

# The fork tag may replay the upstream post-release changelog reset while
# rebasing fork patches onto the transport tag. Prefer the local changelog, but
# recover the immutable upstream-tag changelog when that local file is empty.
CHANGELOG_BODY="$(strip_changelog_comments < transports/changelog.md)"
if changelog_is_empty "$CHANGELOG_BODY" &&
  [[ -n "$UPSTREAM_REPO_SLUG" && "$UPSTREAM_SOURCE_TAG" != "$TAG_NAME" ]]; then
  echo "ℹ️ Local changelog is empty; loading transports/changelog.md from ${UPSTREAM_REPO_SLUG}@${UPSTREAM_SOURCE_TAG}"
  if UPSTREAM_CHANGELOG_BODY="$(
    gh api --method GET \
      -H "Accept: application/vnd.github.raw+json" \
      "repos/${UPSTREAM_REPO_SLUG}/contents/transports/changelog.md" \
      -f "ref=${UPSTREAM_SOURCE_TAG}"
  )"; then
    CHANGELOG_BODY="$(printf '%s\n' "$UPSTREAM_CHANGELOG_BODY" | strip_changelog_comments)"
  else
    echo "❌ Failed to load upstream changelog from ${UPSTREAM_REPO_SLUG}@${UPSTREAM_SOURCE_TAG}" >&2
    exit 1
  fi
fi
if changelog_is_empty "$CHANGELOG_BODY"; then
  echo "❌ Changelog is empty"
  exit 1
fi
echo "📝 New changelog: $CHANGELOG_BODY"

# Finding previous tag
echo "🔍 Finding previous tag..."
if [[ "$TAG_NAME" == v*-oss ]]; then
  PREV_TAG=$(git tag -l "v*-oss" | sort -V | tail -1)
else
  PREV_TAG=$(git tag -l "transports/v*" | sort -V | tail -1)
fi
if [[ "$PREV_TAG" == "$TAG_NAME" ]]; then
  if [[ "$TAG_NAME" == v*-oss ]]; then
    PREV_TAG=$(git tag -l "v*-oss" | sort -V | tail -2 | head -1)
  else
    PREV_TAG=$(git tag -l "transports/v*" | sort -V | tail -2 | head -1)
  fi
fi
echo "🔍 Previous tag: $PREV_TAG"

# Get message of the tag
echo "🔍 Getting previous tag message..."
PREV_CHANGELOG=$(git tag -l --format='%(contents)' "$PREV_TAG")
echo "📝 Previous changelog body: $PREV_CHANGELOG"

# Checking if tag message is the same as the changelog
if [[ "$ALLOW_SAME_CHANGELOG" != "true" && "$PREV_CHANGELOG" == "$CHANGELOG_BODY" ]]; then
  echo "❌ Changelog is the same as the previous changelog"
  exit 1
fi

# Create and push tag unless it was created by the fork sync workflow.
git config user.name "github-actions[bot]"
git config user.email "41898282+github-actions[bot]@users.noreply.github.com"
if [[ "$SKIP_GIT_TAG_CREATE" == "true" ]]; then
  if ! git rev-parse "$TAG_NAME" >/dev/null 2>&1; then
    echo "❌ Expected existing tag not found: $TAG_NAME"
    exit 1
  fi
  echo "🏷️ Reusing existing tag: $TAG_NAME"
else
  echo "🏷️ Creating tag: $TAG_NAME"
  git tag "$TAG_NAME" -m "Release transports v$VERSION" -m "$CHANGELOG_BODY"
  git push origin "$TAG_NAME"
fi

# Create GitHub release
TITLE="${RELEASE_TITLE_OVERRIDE:-Bifrost HTTP v$VERSION}"

SOURCE_VERSION="${UPSTREAM_SOURCE_TAG#transports/v}"
if [[ "$SOURCE_VERSION" == "$UPSTREAM_SOURCE_TAG" ]]; then
  SOURCE_VERSION="${UPSTREAM_SOURCE_TAG#v}"
fi
if [[ "$SOURCE_VERSION" == "$UPSTREAM_SOURCE_TAG" ]]; then
  SOURCE_VERSION="${VERSION%-oss}"
fi

# Mark prereleases when version contains a hyphen
PRERELEASE_FLAG=""
if [[ "$SOURCE_VERSION" == *-* ]]; then
  PRERELEASE_FLAG="--prerelease"
fi

LATEST_FLAG=""
if [[ "$SOURCE_VERSION" != *-* ]]; then
  LATEST_FLAG="--latest"
fi

# Generate plugin version summary
PLUGIN_UPDATES=""
if [ ${#PLUGINS_USED[@]} -gt 0 ]; then
  PLUGIN_UPDATES="

### 🔌 Plugin Versions
This release includes the following plugin versions:
"
  for plugin_info in "${PLUGINS_USED[@]}"; do
    plugin_name="${plugin_info%%:*}"
    plugin_version="${plugin_info##*:}"
    PLUGIN_UPDATES="$PLUGIN_UPDATES- **$plugin_name**: \`$plugin_version\`
"
  done
else
  # Show all available plugin versions even if not directly used
  PLUGIN_UPDATES="

### 🔌 Available Plugin Versions
The following plugin versions are compatible with this release:
"
  for plugin_name in "${!PLUGIN_VERSIONS[@]}"; do
    plugin_version="${PLUGIN_VERSIONS[$plugin_name]}"
    PLUGIN_UPDATES="$PLUGIN_UPDATES- **$plugin_name**: \`$plugin_version\`
"
  done
fi

BODY="## Bifrost HTTP Transport Release v$VERSION

### Source Upstream Release
- Source tag: \`$UPSTREAM_SOURCE_TAG\`

$CHANGELOG_BODY

### Installation

#### Docker
\`\`\`bash
docker run -p 8080:8080 ${DOCKER_IMAGE_REFERENCE}:v$VERSION
\`\`\`

#### Binary Download
\`\`\`bash
npx @maximhq/bifrost --transport-version v$VERSION
\`\`\`

### Docker Images
- **\`${DOCKER_IMAGE_REFERENCE}:v$VERSION\`** - This specific version
- **\`${DOCKER_IMAGE_REFERENCE}:latest\`** - Latest stable version when the source upstream tag is stable

---
_This release was automatically created with dependencies: core \`$CORE_VERSION\`, framework \`$FRAMEWORK_VERSION\`. All plugins have been validated and updated._"

if [ -z "${GH_TOKEN:-}" ] && [ -z "${GITHUB_TOKEN:-}" ]; then
  echo "Error: GH_TOKEN or GITHUB_TOKEN is not set. Please export one to authenticate the GitHub CLI."
  exit 1
fi

echo "🎉 Creating GitHub release for $TITLE..."
if gh release view "$TAG_NAME" >/dev/null 2>&1; then
  echo "ℹ️ GitHub release already exists for $TAG_NAME"
else
  release_command=(gh release create "$TAG_NAME" --title "$TITLE" --notes "$BODY")
  if [[ -n "$PRERELEASE_FLAG" ]]; then
    release_command+=("$PRERELEASE_FLAG")
  fi
  if [[ -n "$LATEST_FLAG" ]]; then
    release_command+=("$LATEST_FLAG")
  fi
  "${release_command[@]}"
fi

if [[ ${#release_assets[@]} -gt 0 ]]; then
  echo "📦 Uploading ${#release_assets[@]} release assets..."
  gh release upload "$TAG_NAME" --clobber "${release_assets[@]}"
  echo "✅ Release assets uploaded"
fi

echo "✅ Bifrost HTTP released successfully"

# Copy versioned R2 path to latest/ for stable releases
if [[ "$SOURCE_VERSION" != *-* ]]; then
  if [ -n "${R2_ENDPOINT:-}" ] && [ -n "${R2_BUCKET:-}" ]; then
    echo "📤 Copying versioned binaries to latest/ on R2..."
    R2_ENDPOINT="$(echo "$R2_ENDPOINT" | tr -d '[:space:]')"
    aws s3 sync "s3://$R2_BUCKET/bifrost/v$VERSION/" "s3://$R2_BUCKET/bifrost/latest/" \
      --endpoint-url "$R2_ENDPOINT" \
      --profile "${R2_AWS_PROFILE:-R2}" \
      --no-progress \
      --delete
    echo "✅ Latest binaries updated on R2"
  fi
fi

# Print summary
echo ""
echo "📋 Release Summary:"
echo "   🏷️  Tag: $TAG_NAME"
echo "   🧭 Source tag: $UPSTREAM_SOURCE_TAG"
echo "   🔧 Core version: $CORE_VERSION"
echo "   🔧 Framework version: $FRAMEWORK_VERSION"
echo "   📦 Transport: Updated"
if [ ${#PLUGINS_USED[@]} -gt 0 ]; then
  echo "   🔌 Plugins used: ${PLUGINS_USED[*]}"
else
  echo "   🔌 Available plugins: $(printf "%s " "${!PLUGIN_VERSIONS[@]}")"
fi
echo "   🎉 GitHub release: Created"

if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
  echo "success=true" >> "$GITHUB_OUTPUT"
fi
