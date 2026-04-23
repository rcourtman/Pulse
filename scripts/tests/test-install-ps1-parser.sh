#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

if ! command -v pwsh >/dev/null 2>&1; then
  if [[ "${GITHUB_ACTIONS:-false}" == "true" || "${PULSE_REQUIRE_PWSH:-false}" == "true" ]]; then
    echo "pwsh is required for install.ps1 parser validation" >&2
    exit 1
  fi
  echo "SKIP: pwsh not installed"
  exit 0
fi

PULSE_INSTALL_PS1_PATH="${ROOT_DIR}/scripts/install.ps1" \
  pwsh -NoLogo -NoProfile -NonInteractive -Command \
  '$errors = $null; [System.Management.Automation.Language.Parser]::ParseFile($env:PULSE_INSTALL_PS1_PATH, [ref]$null, [ref]$errors) > $null; if ($errors.Count) { $errors | ForEach-Object { Write-Error $_.ToString() }; exit 1 }'

echo "install.ps1 parses successfully"
