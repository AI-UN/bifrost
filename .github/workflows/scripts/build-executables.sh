#!/usr/bin/env bash
set -euo pipefail

# Cross-compile Go binaries for multiple platforms
# Usage: ./build-executables.sh <version> [platforms]
# Examples:
#   ./build-executables.sh 1.4.15                                          # Build all platforms
#   ./build-executables.sh 1.4.15 "darwin/amd64 darwin/arm64 linux/amd64 windows/amd64"  # Build specific platforms
#   ./build-executables.sh 1.4.15 "linux/arm64"                            # Build single platform (native on ARM)

# Require version argument (matches usage)
if [[ -z "${1:-}" ]]; then
  echo "Usage: $0 <version> [platforms]" >&2
  exit 1
fi
VERSION="$1"
PLATFORM_FILTER="${2:-}"

echo "🔨 Building Go executables with version: $VERSION"

# Get the script directory and project root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

# Clean and create dist directory
rm -rf "$PROJECT_ROOT/dist"
mkdir -p "$PROJECT_ROOT/dist"

# Define platforms — use filter if provided, otherwise build all
all_platforms=(
  "darwin/amd64"
  "darwin/arm64"
  "linux/amd64"
  "linux/arm64"
  "windows/amd64"
)

if [[ -n "$PLATFORM_FILTER" ]]; then
  platforms=()
  for p in $PLATFORM_FILTER; do
    platforms+=("$p")
  done
  echo "📋 Building filtered platforms: ${platforms[*]}"
else
  platforms=("${all_platforms[@]}")
  echo "📋 Building all platforms: ${platforms[*]}"
fi

# Detect the host platform for native build detection
HOST_ARCH=$(uname -m)
HOST_OS=$(uname -s)

# Resolve Go workspace mode.
# When LOCAL_WORKSPACE_BUILD=true, build against a freshly generated go.work so
# that local module sources (e.g. core, framework) are used instead of the
# published module versions pinned in transports/go.mod. This mirrors the Docker
# build (transports/Dockerfile.local), which compiles against the local
# workspace. go.work is gitignored and absent from release checkouts, so it must
# be generated on the fly. Otherwise, disable the workspace and resolve modules
# from go.mod as before.
if [[ "${LOCAL_WORKSPACE_BUILD:-}" == "true" ]]; then
  GOWORK_MODE="$PROJECT_ROOT/go.fork-ci.work"
  echo "🧩 Local workspace build enabled — generating workspace at $GOWORK_MODE"

  rm -f "$GOWORK_MODE" "${GOWORK_MODE}.sum"
  GOWORK="$GOWORK_MODE" go work init

  # Local modules to include, mirroring transports/Dockerfile.local. Only
  # directories that actually contain a go.mod are added, so the list degrades
  # gracefully if the module layout changes.
  workspace_modules=(
    core
    framework
    plugins/compat
    plugins/governance
    plugins/jsonparser
    plugins/logging
    plugins/maxim
    plugins/mocker
    plugins/otel
    plugins/prompts
    plugins/semanticcache
    plugins/telemetry
    transports
  )
  for mod in "${workspace_modules[@]}"; do
    if [[ -f "$PROJECT_ROOT/$mod/go.mod" ]]; then
      GOWORK="$GOWORK_MODE" go work use "$PROJECT_ROOT/$mod"
    fi
  done

  # Align the workspace go directive with the transports module requirement so
  # the toolchain does not reject modules requiring a newer Go version.
  GO_DIRECTIVE="$(awk '/^go /{print $2; exit}' "$PROJECT_ROOT/transports/go.mod")"
  if [[ -n "$GO_DIRECTIVE" ]]; then
    GOWORK="$GOWORK_MODE" go work edit -go="$GO_DIRECTIVE"
  fi
else
  GOWORK_MODE="off"
fi

MODULE_PATH="$PROJECT_ROOT/transports/bifrost-http"


for platform in "${platforms[@]}"; do
  IFS='/' read -r PLATFORM_DIR GOARCH <<< "$platform"

  case "$PLATFORM_DIR" in
    "windows") GOOS="windows" ;;
    "darwin")  GOOS="darwin" ;;
    "linux")   GOOS="linux" ;;
    *) echo "Unsupported platform: $PLATFORM_DIR"; exit 1 ;;
  esac

  output_name="bifrost-http"
  [[ "$GOOS" = "windows" ]] && output_name+='.exe'

  echo "Building bifrost-http for $PLATFORM_DIR/$GOARCH..."
  mkdir -p "$PROJECT_ROOT/dist/$PLATFORM_DIR/$GOARCH"

  # Change to the module directory for building
  cd "$MODULE_PATH"

  if [[ "$GOOS" = "linux" ]]; then
    # Detect native build: if target arch matches host, use system compiler
    if [[ "$GOARCH" = "arm64" ]] && [[ "$HOST_ARCH" = "aarch64" || "$HOST_ARCH" = "arm64" ]]; then
      echo "  🏠 Native ARM64 build detected — using system compiler"
      CC_COMPILER="${CC:-gcc}"
      CXX_COMPILER="${CXX:-g++}"
    elif [[ "$GOARCH" = "amd64" ]] && [[ "$HOST_ARCH" = "x86_64" ]]; then
      echo "  🏠 Native AMD64 build detected — using system compiler"
      CC_COMPILER="${CC:-gcc}"
      CXX_COMPILER="${CXX:-g++}"
    elif [[ "$GOARCH" = "amd64" ]]; then
      CC_COMPILER="x86_64-linux-musl-gcc"
      CXX_COMPILER="x86_64-linux-musl-g++"
    elif [[ "$GOARCH" = "arm64" ]]; then
      CC_COMPILER="aarch64-linux-musl-gcc"
      CXX_COMPILER="aarch64-linux-musl-g++"
    fi

    env GOWORK="$GOWORK_MODE" CGO_ENABLED=1 GOOS="$GOOS" GOARCH="$GOARCH" CC="$CC_COMPILER" CXX="$CXX_COMPILER" \
      go build -trimpath -tags "netgo,osusergo,sqlite_static" \
      -ldflags "-s -w -buildid= -extldflags '-static' -X main.Version=v${VERSION}" \
      -o "$PROJECT_ROOT/dist/$PLATFORM_DIR/$GOARCH/$output_name" .

  elif [[ "$GOOS" = "windows" ]]; then
    if [[ "$GOARCH" = "amd64" ]]; then
      CC_COMPILER="x86_64-w64-mingw32-gcc"
      CXX_COMPILER="x86_64-w64-mingw32-g++"
    fi

    env GOWORK="$GOWORK_MODE" CGO_ENABLED=1 GOOS="$GOOS" GOARCH="$GOARCH" CC="$CC_COMPILER" CXX="$CXX_COMPILER" \
      go build -trimpath -ldflags "-s -w -buildid= -X main.Version=v${VERSION}" \
      -o "$PROJECT_ROOT/dist/$PLATFORM_DIR/$GOARCH/$output_name" .

  else # Darwin (macOS)
    if [[ "$HOST_OS" = "Darwin" ]] && \
       { [[ "$GOARCH" = "amd64" && "$HOST_ARCH" = "x86_64" ]] || \
         [[ "$GOARCH" = "arm64" && "$HOST_ARCH" = "arm64" ]]; }; then
      echo "  🏠 Native Darwin build detected — using system compiler"
      CC_COMPILER="${CC:-clang}"
      CXX_COMPILER="${CXX:-clang++}"
    elif [[ "$GOARCH" = "amd64" ]]; then
      CC_COMPILER="o64-clang"
      CXX_COMPILER="o64-clang++"
    elif [[ "$GOARCH" = "arm64" ]]; then
      CC_COMPILER="oa64-clang"
      CXX_COMPILER="oa64-clang++"
    fi

    env GOWORK="$GOWORK_MODE" CGO_ENABLED=1 GOOS="$GOOS" GOARCH="$GOARCH" CC="$CC_COMPILER" CXX="$CXX_COMPILER" \
      go build -trimpath -ldflags "-s -w -buildid= -X main.Version=v${VERSION}" \
      -o "$PROJECT_ROOT/dist/$PLATFORM_DIR/$GOARCH/$output_name" .
  fi

  # Change back to project root
  cd "$PROJECT_ROOT"
done

echo "✅ All binaries built successfully"
