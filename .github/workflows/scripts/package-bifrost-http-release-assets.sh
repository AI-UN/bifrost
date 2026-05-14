#!/usr/bin/env bash
set -euo pipefail

# Package bifrost-http binaries into GitHub Release assets.
# Usage: ./package-bifrost-http-release-assets.sh <version> [dist-dir] [output-dir]

if [[ -z "${1:-}" ]]; then
  echo "Usage: $0 <version> [dist-dir] [output-dir]" >&2
  exit 1
fi

VERSION="$1"
DIST_DIR="${2:-dist}"
OUTPUT_DIR="${3:-release-assets}"

if [[ ! -d "$DIST_DIR" ]]; then
  echo "❌ Dist directory not found: $DIST_DIR" >&2
  exit 1
fi

if ! command -v zip >/dev/null 2>&1; then
  echo "❌ zip is required to package Windows release assets" >&2
  exit 1
fi

checksum_cmd=()
if command -v sha256sum >/dev/null 2>&1; then
  checksum_cmd=(sha256sum)
elif command -v shasum >/dev/null 2>&1; then
  checksum_cmd=(shasum -a 256)
else
  echo "❌ sha256sum or shasum is required to generate release checksums" >&2
  exit 1
fi

rm -rf "$OUTPUT_DIR"
mkdir -p "$OUTPUT_DIR"
OUTPUT_DIR_ABS="$(cd "$OUTPUT_DIR" && pwd)"

package_binary() {
  local goos="$1"
  local goarch="$2"
  local binary_name="$3"
  local source_path="$DIST_DIR/$goos/$goarch/$binary_name"

  if [[ ! -f "$source_path" ]]; then
    echo "❌ Missing built binary: $source_path" >&2
    exit 1
  fi

  local asset_base="bifrost-http_${VERSION}_${goos}_${goarch}"
  local staging_dir
  staging_dir="$(mktemp -d)"
  cp "$source_path" "$staging_dir/$binary_name"

  if [[ "$goos" == "windows" ]]; then
    (cd "$staging_dir" && zip -q "$OUTPUT_DIR_ABS/${asset_base}.zip" "$binary_name")
  else
    tar -C "$staging_dir" -czf "$OUTPUT_DIR_ABS/${asset_base}.tar.gz" "$binary_name"
  fi

  rm -rf "$staging_dir"
}

package_binary darwin amd64 bifrost-http
package_binary darwin arm64 bifrost-http
package_binary linux amd64 bifrost-http
package_binary linux arm64 bifrost-http
package_binary windows amd64 bifrost-http.exe

shopt -s nullglob
assets=("$OUTPUT_DIR"/*.tar.gz "$OUTPUT_DIR"/*.zip)
shopt -u nullglob

if [[ ${#assets[@]} -eq 0 ]]; then
  echo "❌ No packaged release assets were created" >&2
  exit 1
fi

checksum_file="$OUTPUT_DIR/bifrost-http_${VERSION}_checksums.txt"
: > "$checksum_file"

for asset_path in "${assets[@]}"; do
  asset_name="$(basename "$asset_path")"
  (
    cd "$OUTPUT_DIR"
    "${checksum_cmd[@]}" "$asset_name"
  ) >> "$checksum_file"
done

echo "✅ Packaged bifrost-http release assets:"
for asset_path in "${assets[@]}" "$checksum_file"; do
  echo "   - $(basename "$asset_path")"
done
