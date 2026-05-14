#!/usr/bin/env bash
set -euo pipefail

# Cross-compile Go binaries for multiple platforms
# Usage: ./build-executables.sh <version> [platforms]
# Examples:
#   ./build-executables.sh 1.4.15                                          # Build all platforms
#   ./build-executables.sh 1.4.15 "darwin/amd64 darwin/arm64 linux/amd64 windows/amd64 windows/arm64"  # Build specific platforms
#   ./build-executables.sh 1.4.15 "linux/arm64"                            # Build single platform (native on ARM)

# Require version argument (matches usage)
if [[ -z "${1:-}" ]]; then
  echo "Usage: $0 <version> [platforms]" >&2
  exit 1
fi
VERSION="$1"
PLATFORM_FILTER="${2:-}"
LOCAL_WORKSPACE_BUILD="${LOCAL_WORKSPACE_BUILD:-false}"

echo "🔨 Building Go executables with version: $VERSION"

# Get the script directory and project root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

if [[ "$LOCAL_WORKSPACE_BUILD" == "true" ]]; then
  echo "🔧 Enabling local Go workspace build"
  cd "$PROJECT_ROOT"
  # Tags may point at commits before go.work was tracked; create one on demand.
  source "$SCRIPT_DIR/setup-go-workspace.sh"
fi

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
  "windows/arm64"
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

# Detect host OS/architecture for native build detection
HOST_OS_RAW=$(uname -s)
case "${HOST_OS_RAW,,}" in
  linux*)
    HOST_GOOS="linux"
    ;;
  darwin*)
    HOST_GOOS="darwin"
    ;;
  msys*|mingw*|cygwin*)
    HOST_GOOS="windows"
    ;;
  *)
    HOST_GOOS="unknown"
    ;;
esac

HOST_ARCH_RAW=$(uname -m)
case "$HOST_ARCH_RAW" in
  x86_64|amd64)
    HOST_GOARCH="amd64"
    ;;
  aarch64|arm64)
    HOST_GOARCH="arm64"
    ;;
  i386|i686)
    HOST_GOARCH="386"
    ;;
  *)
    HOST_GOARCH="$HOST_ARCH_RAW"
    ;;
esac

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

  workspace_env=()
  if [[ "$LOCAL_WORKSPACE_BUILD" != "true" ]]; then
    workspace_env+=(GOWORK=off)
  fi

  # Change to the module directory for building
  cd "$MODULE_PATH"

  if [[ "$GOOS" = "linux" ]]; then
    # Detect native build: if target arch matches host, use system compiler
    if [[ "$GOOS" == "$HOST_GOOS" && "$GOARCH" == "$HOST_GOARCH" ]]; then
      echo "  🏠 Native Linux ${GOARCH} build detected — using system compiler"
      CC_COMPILER="${CC:-gcc}"
      CXX_COMPILER="${CXX:-g++}"
    elif [[ "$GOARCH" = "amd64" ]]; then
      CC_COMPILER="x86_64-linux-musl-gcc"
      CXX_COMPILER="x86_64-linux-musl-g++"
    elif [[ "$GOARCH" = "arm64" ]]; then
      CC_COMPILER="aarch64-linux-musl-gcc"
      CXX_COMPILER="aarch64-linux-musl-g++"
    else
      echo "Unsupported Linux architecture: $GOARCH" >&2
      exit 1
    fi

    env "${workspace_env[@]}" CGO_ENABLED=1 GOOS="$GOOS" GOARCH="$GOARCH" CC="$CC_COMPILER" CXX="$CXX_COMPILER" \
      go build -trimpath -tags "netgo,osusergo,sqlite_static" \
      -ldflags "-s -w -buildid= -extldflags '-static' -X main.Version=v${VERSION}" \
      -o "$PROJECT_ROOT/dist/$PLATFORM_DIR/$GOARCH/$output_name" .

  elif [[ "$GOOS" = "windows" ]]; then
    if [[ "$GOOS" == "$HOST_GOOS" && "$GOARCH" == "$HOST_GOARCH" ]]; then
      echo "  🏠 Native Windows ${GOARCH} build detected — using system compiler"
      CC_COMPILER="${CC:-gcc}"
      CXX_COMPILER="${CXX:-g++}"
    elif [[ "$GOARCH" = "amd64" ]]; then
      CC_COMPILER="x86_64-w64-mingw32-gcc"
      CXX_COMPILER="x86_64-w64-mingw32-g++"
    elif [[ "$GOARCH" = "arm64" ]]; then
      CC_COMPILER="aarch64-w64-mingw32-gcc"
      CXX_COMPILER="aarch64-w64-mingw32-g++"
    else
      echo "Unsupported Windows architecture: $GOARCH" >&2
      exit 1
    fi

    env "${workspace_env[@]}" CGO_ENABLED=1 GOOS="$GOOS" GOARCH="$GOARCH" CC="$CC_COMPILER" CXX="$CXX_COMPILER" \
      go build -trimpath -ldflags "-s -w -buildid= -X main.Version=v${VERSION}" \
      -o "$PROJECT_ROOT/dist/$PLATFORM_DIR/$GOARCH/$output_name" .

   else # Darwin (macOS)
    if [[ "$GOOS" == "$HOST_GOOS" && "$GOARCH" == "$HOST_GOARCH" ]]; then
      echo "  🏠 Native Darwin ${GOARCH} build detected — using system compiler"
      CC_COMPILER="${CC:-clang}"
      CXX_COMPILER="${CXX:-clang++}"
    elif [[ "$GOARCH" = "amd64" ]]; then
      CC_COMPILER="o64-clang"
      CXX_COMPILER="o64-clang++"
    elif [[ "$GOARCH" = "arm64" ]]; then
      CC_COMPILER="oa64-clang"
      CXX_COMPILER="oa64-clang++"
    else
      echo "Unsupported Darwin architecture: $GOARCH" >&2
      exit 1
    fi

    env "${workspace_env[@]}" CGO_ENABLED=1 GOOS="$GOOS" GOARCH="$GOARCH" CC="$CC_COMPILER" CXX="$CXX_COMPILER" \
      go build -trimpath -ldflags "-s -w -buildid= -X main.Version=v${VERSION}" \
      -o "$PROJECT_ROOT/dist/$PLATFORM_DIR/$GOARCH/$output_name" .
  fi

  # Change back to project root
  cd "$PROJECT_ROOT"
done

echo "✅ All binaries built successfully"
