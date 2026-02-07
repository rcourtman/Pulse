#!/usr/bin/env bash
# dev-launchd-wrapper.sh - Wrapper for running hot-dev.sh under launchd
#
# launchd doesn't source login profiles, so PATH won't include go, npm,
# node, fswatch, etc. This wrapper sources the user's shell profile to
# set up the full environment before exec'ing hot-dev.sh.

set -euo pipefail

# Source shell profiles for PATH (Homebrew, nvm, goenv, etc.)
# shellcheck disable=SC1090,SC1091
if [[ -f "$HOME/.zprofile" ]]; then source "$HOME/.zprofile"; fi
# shellcheck disable=SC1090,SC1091
if [[ -f "$HOME/.zshrc" ]]; then source "$HOME/.zshrc"; fi

# Resolve the real script directory (handles symlinks)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
cd "${SCRIPT_DIR}/.." || exit 1

exec bash scripts/hot-dev.sh
