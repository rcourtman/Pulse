#!/usr/bin/env bash
set -euo pipefail

if ! command -v node >/dev/null 2>&1; then
  echo "node is required for demo browser smoke" >&2
  exit 1
fi

if ! command -v npx >/dev/null 2>&1; then
  echo "npx is required for demo browser smoke" >&2
  exit 1
fi

if [ -z "${PULSE_PUBLIC_SITE_URL:-}" ]; then
  echo "PULSE_PUBLIC_SITE_URL is required for demo browser smoke" >&2
  exit 1
fi

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
BROWSERS_PATH="${PLAYWRIGHT_BROWSERS_PATH:-${RUNNER_TEMP:-/tmp}/pw-browsers}"

mkdir -p "$BROWSERS_PATH"

export PLAYWRIGHT_BROWSERS_PATH="$BROWSERS_PATH"
npx --yes -p playwright playwright install --with-deps chromium

playwright_bin="$(npx --yes -p playwright -c 'which playwright')"
playwright_node_path="$(cd "$(dirname "$playwright_bin")/.." && pwd)"

NODE_PATH="$playwright_node_path${NODE_PATH:+:$NODE_PATH}" \
  node "$SCRIPT_DIR/demo_public_browser_smoke.cjs"
