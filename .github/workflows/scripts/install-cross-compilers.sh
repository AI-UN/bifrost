#!/usr/bin/env bash
set -euo pipefail

# Install cross-compilation toolchains for Go + CGO
# Usage: ./install-cross-compilers.sh

echo "📦 Installing cross-compilation toolchains for Go + CGO..."

LLVM_MINGW_VERSION="${LLVM_MINGW_VERSION:-20260505}"
LLVM_MINGW_ARCHIVE="llvm-mingw-${LLVM_MINGW_VERSION}-msvcrt-ubuntu-22.04-x86_64.tar.xz"
LLVM_MINGW_URL="https://github.com/mstorsjo/llvm-mingw/releases/download/${LLVM_MINGW_VERSION}/${LLVM_MINGW_ARCHIVE}"
LLVM_MINGW_DIR="/opt/llvm-mingw-${LLVM_MINGW_VERSION}"

# Install all required packages
sudo apt-get update
sudo apt-get install -y \
  gcc-x86-64-linux-gnu \
  gcc-aarch64-linux-gnu \
  gcc-mingw-w64-x86-64 \
  musl-tools \
  clang \
  lld \
  xz-utils \
  curl

# Create symbolic links for musl compilers
sudo ln -sf /usr/bin/x86_64-linux-gnu-gcc /usr/local/bin/x86_64-linux-musl-gcc
sudo ln -sf /usr/bin/x86_64-linux-gnu-g++ /usr/local/bin/x86_64-linux-musl-g++
sudo ln -sf /usr/bin/aarch64-linux-gnu-gcc /usr/local/bin/aarch64-linux-musl-gcc
sudo ln -sf /usr/bin/aarch64-linux-gnu-g++ /usr/local/bin/aarch64-linux-musl-g++

echo "🪟 Setting up Windows ARM64 cross-compilation..."

if [ ! -d "$LLVM_MINGW_DIR" ]; then
  echo "📦 Downloading llvm-mingw ${LLVM_MINGW_VERSION}..."
  curl -fL "$LLVM_MINGW_URL" -o "/tmp/${LLVM_MINGW_ARCHIVE}"
  sudo mkdir -p /opt
  sudo tar -xf "/tmp/${LLVM_MINGW_ARCHIVE}" -C /opt
  sudo mv "/opt/llvm-mingw-${LLVM_MINGW_VERSION}-msvcrt-ubuntu-22.04-x86_64" "$LLVM_MINGW_DIR"
  rm -f "/tmp/${LLVM_MINGW_ARCHIVE}"
fi

sudo ln -sf "$LLVM_MINGW_DIR/bin/aarch64-w64-mingw32-gcc" /usr/local/bin/aarch64-w64-mingw32-gcc
sudo ln -sf "$LLVM_MINGW_DIR/bin/aarch64-w64-mingw32-g++" /usr/local/bin/aarch64-w64-mingw32-g++

echo "🍎 Setting up Darwin cross-compilation..."

# Where to install SDK
SDK_DIR="/opt/MacOSX12.3.sdk"
SDK_URL="https://github.com/phracker/MacOSX-SDKs/releases/download/12.3/MacOSX12.3.sdk.tar.xz"

# Download and extract macOS SDK if not already installed
if [ ! -d "$SDK_DIR" ]; then
  echo "📦 Downloading macOS SDK..."
  # Use -f to fail on HTTP errors, -L to follow redirects
  if ! curl -fL "$SDK_URL" -o /tmp/MacOSX12.3.sdk.tar.xz; then
    echo "❌ Failed to download macOS SDK from primary URL, trying alternative..."
    SDK_URL_ALT="https://github.com/joseluisq/macosx-sdks/releases/download/12.3/MacOSX12.3.sdk.tar.xz"
    curl -fL "$SDK_URL_ALT" -o /tmp/MacOSX12.3.sdk.tar.xz
  fi
  sudo mkdir -p /opt
  sudo tar -xf /tmp/MacOSX12.3.sdk.tar.xz -C /opt
  rm -f /tmp/MacOSX12.3.sdk.tar.xz
fi

# Create wrapper scripts with proper shebang and linker configuration
sudo tee /usr/local/bin/o64-clang > /dev/null << 'WRAPPER_EOF'
#!/bin/bash
exec clang -target x86_64-apple-darwin --sysroot=/opt/MacOSX12.3.sdk -fuse-ld=lld -Wno-unused-command-line-argument "$@"
WRAPPER_EOF

sudo tee /usr/local/bin/o64-clang++ > /dev/null << 'WRAPPER_EOF'
#!/bin/bash
exec clang++ -target x86_64-apple-darwin --sysroot=/opt/MacOSX12.3.sdk -fuse-ld=lld -Wno-unused-command-line-argument "$@"
WRAPPER_EOF

sudo tee /usr/local/bin/oa64-clang > /dev/null << 'WRAPPER_EOF'
#!/bin/bash
exec clang -target arm64-apple-darwin --sysroot=/opt/MacOSX12.3.sdk -fuse-ld=lld -Wno-unused-command-line-argument "$@"
WRAPPER_EOF

sudo tee /usr/local/bin/oa64-clang++ > /dev/null << 'WRAPPER_EOF'
#!/bin/bash
exec clang++ -target arm64-apple-darwin --sysroot=/opt/MacOSX12.3.sdk -fuse-ld=lld -Wno-unused-command-line-argument "$@"
WRAPPER_EOF

sudo chmod +x /usr/local/bin/o64-clang /usr/local/bin/o64-clang++ \
               /usr/local/bin/oa64-clang /usr/local/bin/oa64-clang++

echo "✅ Darwin cross-compilation environment ready!"

echo "✅ Cross-compilation toolchains installed"
echo ""
echo "Available cross-compilers:"
echo "  Linux amd64:   x86_64-linux-musl-gcc, x86_64-linux-musl-g++"
echo "  Linux arm64:   aarch64-linux-musl-gcc, aarch64-linux-musl-g++"
echo "  Windows amd64: x86_64-w64-mingw32-gcc, x86_64-w64-mingw32-g++"
echo "  Windows arm64: aarch64-w64-mingw32-gcc, aarch64-w64-mingw32-g++"
echo "  Darwin amd64:  o64-clang, o64-clang++"
echo "  Darwin arm64:  oa64-clang, oa64-clang++"
