#!/usr/bin/env bash
set -euo pipefail

export GOTOOLCHAIN=auto

# If go.work exists, skip
if [ -f "go.work" ]; then
  echo "🔍 Go workspace already exists, skipping initialization"
  return
fi


# Setup Go workspace for CI
# Usage: source setup-go-workspace.sh
echo "🔧 Setting up Go workspace..."
if [ -f "go.work" ]; then
  echo "✅ Go workspace already exists, skipping init"
  return 0 2>/dev/null || exit 0
fi

go work init

modules=(
  ./core
  ./framework
  ./plugins/compat
  ./plugins/governance
  ./plugins/jsonparser
  ./plugins/logging
  ./plugins/maxim
  ./plugins/mocker
  ./plugins/otel
  ./plugins/prompts
  ./plugins/semanticcache
  ./plugins/telemetry
  ./transports
  ./cli
)

for module_path in "${modules[@]}"; do
  if [ -d "$module_path" ]; then
    go work use "$module_path"
  fi
done

echo "✅ Go workspace initialized"
