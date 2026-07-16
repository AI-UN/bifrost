#!/usr/bin/env bash
set -euo pipefail

# Validate input argument
if [ "${1:-}" = "" ]; then
  echo "Usage: $0 <version>" >&2
  exit 1
fi

VERSION="$1"
REGISTRY="docker.io"
ACCOUNT="maximhq"
IMAGE_NAME="bifrost"
IMAGE="${IMAGE_REF:-${REGISTRY}/${ACCOUNT}/${IMAGE_NAME}}"
SOURCE_TAG_FOR_LATEST="${SOURCE_TAG_FOR_LATEST:-}"
PUBLISH_LATEST_TAGS="${PUBLISH_LATEST_TAGS:-}"
IMAGE_TAG="${IMAGE_TAG:-v${VERSION}}"

if [[ -z "$PUBLISH_LATEST_TAGS" ]]; then
  if [[ "$VERSION" == "main" || "$IMAGE_TAG" == "main" ]]; then
    PUBLISH_LATEST_TAGS="false"
  elif [[ -n "$SOURCE_TAG_FOR_LATEST" ]]; then
    SOURCE_VERSION="${SOURCE_TAG_FOR_LATEST#transports/v}"
    if [[ "$SOURCE_VERSION" != *-* ]]; then
      PUBLISH_LATEST_TAGS="true"
    else
      PUBLISH_LATEST_TAGS="false"
    fi
  elif [[ "$VERSION" != *-* ]]; then
    PUBLISH_LATEST_TAGS="true"
  else
    PUBLISH_LATEST_TAGS="false"
  fi
fi

# Get the actual image digests from the platform-specific builds.
# Filter by platform.architecture rather than relying on positional [0]:
# `docker/build-push-action` with default provenance creates an OCI image
# index containing the platform image manifest AND a provenance attestation
# manifest, and the ordering is not guaranteed. Selecting by architecture
# is robust to buildx changing the layout.
AMD64_DIGEST=$(docker manifest inspect "${IMAGE}:${IMAGE_TAG}-amd64" | jq -er '.manifests[] | select(.platform.architecture == "amd64") | .digest')
ARM64_DIGEST=$(docker manifest inspect "${IMAGE}:${IMAGE_TAG}-arm64" | jq -er '.manifests[] | select(.platform.architecture == "arm64") | .digest')

echo "AMD64 digest: ${AMD64_DIGEST}"
echo "ARM64 digest: ${ARM64_DIGEST}"

# Create manifest for versioned tag using digests
docker manifest create \
    "${IMAGE}:${IMAGE_TAG}" \
    "${IMAGE}@${AMD64_DIGEST}" \
    "${IMAGE}@${ARM64_DIGEST}"

docker manifest push "${IMAGE}:${IMAGE_TAG}"

# Create latest manifest only for stable versions
if [[ "$PUBLISH_LATEST_TAGS" == "true" ]]; then
    docker manifest create \
        "${IMAGE}:latest" \
        "${IMAGE}@${AMD64_DIGEST}" \
        "${IMAGE}@${ARM64_DIGEST}"

    docker manifest push "${IMAGE}:latest"
fi

# Additionally mirror the multi-arch manifest to GitHub Container Registry (ghcr.io).
# This is purely additive and does not affect the Docker Hub tags above.
#
# NOTE on first run / package visibility: GHCR creates new container packages as
# PRIVATE by default. Until a maintainer flips visibility for the package
# (`https://github.com/orgs/<owner>/packages/container/<repo>/settings` →
# "Change visibility" → Public) anonymous pulls from `ghcr.io/<owner>/<repo>`
# return 403/404 even after a successful push here. This is one-time per
# package and not something this script can fix automatically — the GHCR
# REST API does not currently expose visibility-PATCH for container packages.
if [ -n "${GITHUB_REPOSITORY:-}" ]; then
  (
    set -e
    GHCR_IMAGE="ghcr.io/$(echo "${GITHUB_REPOSITORY}" | tr '[:upper:]' '[:lower:]')"

    GHCR_AMD64_DIGEST=$(docker manifest inspect "${GHCR_IMAGE}:${IMAGE_TAG}-amd64" | jq -er '.manifests[] | select(.platform.architecture == "amd64") | .digest')
    GHCR_ARM64_DIGEST=$(docker manifest inspect "${GHCR_IMAGE}:${IMAGE_TAG}-arm64" | jq -er '.manifests[] | select(.platform.architecture == "arm64") | .digest')

    echo "GHCR AMD64 digest: ${GHCR_AMD64_DIGEST}"
    echo "GHCR ARM64 digest: ${GHCR_ARM64_DIGEST}"

    docker manifest create \
        "${GHCR_IMAGE}:${IMAGE_TAG}" \
        "${GHCR_IMAGE}@${GHCR_AMD64_DIGEST}" \
        "${GHCR_IMAGE}@${GHCR_ARM64_DIGEST}"

    docker manifest push "${GHCR_IMAGE}:${IMAGE_TAG}"

    if [[ "$PUBLISH_LATEST_TAGS" == "true" ]]; then
        docker manifest create \
            "${GHCR_IMAGE}:latest" \
            "${GHCR_IMAGE}@${GHCR_AMD64_DIGEST}" \
            "${GHCR_IMAGE}@${GHCR_ARM64_DIGEST}"

        docker manifest push "${GHCR_IMAGE}:latest"
    fi
  ) || echo "::warning::GHCR mirroring failed; Docker Hub publish unaffected"
fi
