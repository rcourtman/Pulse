#!/usr/bin/env bash

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")"/.. && pwd)"

TARGETS=(
  "bin"
  "build"
  "dist"
  "downloads"
  "frontend-modern/dist"
  "frontend-modern/node_modules"
  "frontend-modern/release"
  "frontend-modern/.vite"
  "frontend-modern/.cache"
  "frontend-modern/.eslintcache"
  "frontend"
  "frontend-modern/public/download"
  "frontend-modern/public/pulse-host-agent-windows-amd64.exe"
  "frontend-modern/public/pulse-host-agent-darwin-arm64.tar.gz"
  "frontend-modern/public/pulse-host-agent-darwin-amd64.tar.gz"
  "frontend-modern/public/pulse-host-agent-linux-amd64"
  "frontend-modern/public/pulse-host-agent-linux-arm64"
  "frontend-modern/public/pulse-host-agent-linux-armv7"
  "data"
  "internal/api/frontend-modern"
  "node_modules"
  "pulse"
  "pulse.log"
  "pulse-docker-agent"
  "pulse-server"
  "pulse-test"
  "pulse-host-agent"
  "pulse-host-agent-linux-amd64"
  "pulse-host-agent-linux-arm64"
  "pulse-host-agent-linux-armv7"
  "pulse-host-agent-darwin-amd64"
  "pulse-host-agent-darwin-arm64"
  "pulse-host-agent-windows-amd64"
  "monitoring.test"
  "release"
  "scripts/macos/dist"
  "testing-tools/node_modules"
  "tmp"
  ".codex"
  ".claudecode-hooks"
  ".claudecode-settings.json"
  ".claude"
  ".mcp-servers"
  ".env"
  ".env.local"
  ".tmp"
  ".cache"
  ".parcel-cache"
  ".sass-cache"
  ".turbo"
  ".next"
  ".pytest_cache"
  ".gradle"
)

try_remove() {
  local path="$1"
  if [ -e "$path" ]; then
    local rel="${path#"${ROOT}/"}"
    if git -C "${ROOT}" ls-files --error-unmatch "$rel" >/dev/null 2>&1; then
      echo "Skipping tracked ${rel}"
      return
    fi
    if rm -rf "$path" 2>/dev/null; then
      echo "Removed ${path#"${ROOT}/"}"
    elif command -v sudo >/dev/null 2>&1; then
      sudo rm -rf "$path"
      echo "Removed ${path#"${ROOT}/"} (via sudo)"
    else
      echo "Warning: unable to remove ${path#"${ROOT}/"}" >&2
    fi
  fi
}

for target in "${TARGETS[@]}"; do
  try_remove "${ROOT}/${target}"
done

find "${ROOT}" -maxdepth 1 -type f -name "*.log" -print -delete
find "${ROOT}/frontend-modern" -maxdepth 1 -type f -name "*.log" -print -delete
find "${ROOT}" -maxdepth 2 -type d -name "coverage" -prune -exec rm -rf {} +
find "${ROOT}" -maxdepth 2 -type f \( -name "*.coverage" -o -name "coverage*.out" \) -delete
find "${ROOT}" -type d -name "__pycache__" -prune -exec rm -rf {} +
find "${ROOT}" -type f -name "*.pyc" -delete
find "${ROOT}" -type f \( -name "*.tmp" -o -name "*.bak" -o -name "*.backup" -o -name "*~" -o -name "*.orig" \) -delete
find "${ROOT}" -type f \( -name ".DS_Store" -o -name "Thumbs.db" \) -delete
find "${ROOT}" -maxdepth 2 -type d -empty -delete

echo "Cleanup complete."
