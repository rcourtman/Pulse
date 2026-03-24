#!/usr/bin/env bash
# dev-launchd-wrapper.sh - Wrapper for running hot-dev-bg launchd-session
#
# launchd doesn't source login profiles, so PATH won't include go, npm,
# node, fswatch, etc. Hand off to an interactive login zsh so the user's
# normal shell environment is loaded before exec'ing the managed launchd
# supervisor path.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
export PULSE_DEV_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd -P)"

exec /bin/zsh -ilc 'cd "$PULSE_DEV_ROOT" && exec bash scripts/hot-dev-bg.sh launchd-session --takeover'
