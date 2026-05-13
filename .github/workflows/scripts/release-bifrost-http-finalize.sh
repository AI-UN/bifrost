#!/usr/bin/env bash
set -euo pipefail

# Finalize bifrost-http release: changelog, tagging, GitHub release, R2 latest copy
# Usage: ./release-bifrost-http-finalize.sh <version>

if [[ -z "${1:-}" ]]; then
  echo "Usage: $0 <version>" >&2
  exit 1
fi

VERSION="$1"
TAG_NAME="${TRANSPORT_TAG_NAME:-transports/v${VERSION}}"
UPSTREAM_SOURCE_TAG="${UPSTREAM_SOURCE_TAG:-$TAG_NAME}"
SKIP_GIT_TAG_CREATE="${SKIP_GIT_TAG_CREATE:-false}"
ALLOW_SAME_CHANGELOG="${ALLOW_SAME_CHANGELOG:-false}"
DOCKER_IMAGE_REFERENCE="${DOCKER_IMAGE_REFERENCE:-maximhq/bifrost}"
TITLE="${RELEASE_TITLE_OVERRIDE:-Bifrost HTTP v$VERSION}"

echo "🏷️ Finalizing bifrost-http v$VERSION release..."

SOURCE_VERSION="${UPSTREAM_SOURCE_TAG#transports/v}"
if [[ "$SOURCE_VERSION" == "$UPSTREAM_SOURCE_TAG" ]]; then
  SOURCE_VERSION="$VERSION"
fi

CORE_VERSION="v$(tr -d '\n\r' < core/version)"
FRAMEWORK_VERSION="v$(tr -d '\n\r' < framework/version)"

declare -A PLUGIN_VERSIONS
PLUGINS_USED=()

for plugin_dir in plugins/*/; do
  if [[ -d "$plugin_dir" ]]; then
    plugin_name="$(basename "$plugin_dir")"
    plugin_version="v$(tr -d '\n\r' < "${plugin_dir}version")"
    PLUGIN_VERSIONS["$plugin_name"]="$plugin_version"
  fi
done

while IFS= read -r plugin_line; do
  plugin_name="$(echo "$plugin_line" | awk -F'/' '{print $NF}' | awk '{print $1}')"
  plugin_version="$(echo "$plugin_line" | awk '{print $NF}')"
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

CHANGELOG_BODY="$(grep -v '^<!--' transports/changelog.md | grep -v '^-->' || true)"
if [[ -z "$CHANGELOG_BODY" ]]; then
  echo "❌ Changelog is empty"
  exit 1
fi

echo "🔍 Finding previous tag..."
PREV_TAG="$(git tag -l 'transports/v*' | sort -V | tail -1)"
if [[ "$PREV_TAG" == "$TAG_NAME" ]]; then
  PREV_TAG="$(git tag -l 'transports/v*' | sort -V | tail -2 | head -1)"
fi
echo "🔍 Previous tag: ${PREV_TAG:-<none>}"

PREV_CHANGELOG=""
if [[ -n "$PREV_TAG" ]]; then
  PREV_CHANGELOG="$(git tag -l --format='%(contents)' "$PREV_TAG")"
fi

if [[ "$ALLOW_SAME_CHANGELOG" != "true" && "$PREV_CHANGELOG" == "$CHANGELOG_BODY" ]]; then
  echo "❌ Changelog is the same as the previous changelog"
  exit 1
fi

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

if [[ -z "${GH_TOKEN:-}" && -z "${GITHUB_TOKEN:-}" ]]; then
  echo "Error: GH_TOKEN or GITHUB_TOKEN is not set. Please export one to authenticate the GitHub CLI."
  exit 1
fi

if gh release view "$TAG_NAME" >/dev/null 2>&1; then
  echo "ℹ️ GitHub release already exists for $TAG_NAME"
  if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
    echo "success=true" >> "$GITHUB_OUTPUT"
  fi
  exit 0
fi

PRERELEASE_FLAG=""
if [[ "$SOURCE_VERSION" == *-* ]]; then
  PRERELEASE_FLAG="--prerelease"
fi

LATEST_FLAG=""
if [[ "$SOURCE_VERSION" != *-* ]]; then
  LATEST_FLAG="--latest"
fi

PLUGIN_UPDATES=""
if [[ ${#PLUGINS_USED[@]} -gt 0 ]]; then
  PLUGIN_UPDATES="

### Plugin Versions
This release includes the following plugin versions:
"
  for plugin_info in "${PLUGINS_USED[@]}"; do
    plugin_name="${plugin_info%%:*}"
    plugin_version="${plugin_info##*:}"
    PLUGIN_UPDATES+="- **${plugin_name}**: \`${plugin_version}\`
"
  done
fi

BODY="## Bifrost HTTP Transport Release v$VERSION

### Source Upstream Release
- Source tag: \`${UPSTREAM_SOURCE_TAG}\`

${CHANGELOG_BODY}${PLUGIN_UPDATES}

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
_This release was automatically created with dependencies: core \`${CORE_VERSION}\`, framework \`${FRAMEWORK_VERSION}\`. All plugins have been validated and updated._"

echo "🎉 Creating GitHub release for $TITLE..."
gh release create "$TAG_NAME" \
  --title "$TITLE" \
  --notes "$BODY" \
  ${PRERELEASE_FLAG} ${LATEST_FLAG}

echo "✅ Bifrost HTTP released successfully"

if [[ "$SOURCE_VERSION" != *-* ]]; then
  if [[ -n "${R2_ENDPOINT:-}" && -n "${R2_BUCKET:-}" ]]; then
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

echo ""
echo "📋 Release Summary:"
echo "   🏷️  Tag: $TAG_NAME"
echo "   🧭 Source tag: $UPSTREAM_SOURCE_TAG"
echo "   🔧 Core version: $CORE_VERSION"
echo "   🔧 Framework version: $FRAMEWORK_VERSION"
echo "   📦 Transport: Updated"
if [[ ${#PLUGINS_USED[@]} -gt 0 ]]; then
  echo "   🔌 Plugins used: ${PLUGINS_USED[*]}"
else
  echo "   🔌 Available plugins: $(printf '%s ' "${!PLUGIN_VERSIONS[@]}")"
fi
echo "   🎉 GitHub release: Created"

if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
  echo "success=true" >> "$GITHUB_OUTPUT"
fi
