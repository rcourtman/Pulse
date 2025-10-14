#!/usr/bin/env bash
# Identify files that will bloat Claude Code's context window.

set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT_DIR"

echo "Scanning modified files that exceed size thresholds..."
echo

git status --short | awk '{print $2}' | while read -r file; do
  [ -f "$file" ] || continue
  bytes=$(wc -c < "$file")
  if [ "$bytes" -ge 65536 ]; then
    lines=$(wc -l < "$file")
    printf "%8d bytes  %7d lines  %s\n" "$bytes" "$lines" "$file"
  fi
done

echo
echo "Tip: stash or split these files when you do not need Claude to inspect them directly."
