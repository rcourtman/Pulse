#!/usr/bin/env bash
#
# Smoke tests for the pulse-docker-agent image contract.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DOCKERFILE="${ROOT_DIR}/Dockerfile"

if [[ ! -f "${DOCKERFILE}" ]]; then
  echo "Dockerfile not found at ${DOCKERFILE}" >&2
  exit 1
fi

agent_runtime_block="$(
  awk '
    /^FROM .* AS agent_runtime$/ { in_block=1 }
    in_block { print }
    in_block && /^FROM .* AS runtime$/ { exit }
  ' "${DOCKERFILE}"
)"

failures=0

assert_contains() {
  local desc="$1"
  local needle="$2"
  if grep -Fq "${needle}" <<<"${agent_runtime_block}"; then
    echo "[PASS] ${desc}"
  else
    echo "[FAIL] ${desc}" >&2
    echo "Missing: ${needle}" >&2
    ((failures++))
  fi
}

assert_not_contains() {
  local desc="$1"
  local needle="$2"
  if grep -Fq "${needle}" <<<"${agent_runtime_block}"; then
    echo "[FAIL] ${desc}" >&2
    echo "Unexpected: ${needle}" >&2
    ((failures++))
  else
    echo "[PASS] ${desc}"
  fi
}

assert_contains "docker-agent image persists agent identity by default" "PULSE_AGENT_ID_FILE=/var/lib/pulse-agent/agent-id"
assert_contains "docker-agent image declares persistent state volume" 'VOLUME ["/var/lib/pulse-agent"]'
assert_contains "docker-agent image disables host metrics by default" "PULSE_ENABLE_HOST=false"
assert_contains "docker-agent image enables docker metrics by default" "PULSE_ENABLE_DOCKER=true"
assert_contains "docker-agent image disables binary self-update" "PULSE_DISABLE_AUTO_UPDATE=true"
assert_contains "legacy entrypoint delegates to pulse-agent" 'exec /usr/local/bin/pulse-agent "$@"'
assert_not_contains "legacy shim does not force docker flag over user args" 'exec /usr/local/bin/pulse-agent --enable-docker "$@"'

if (( failures > 0 )); then
  echo "docker-agent image contract failed (${failures})" >&2
  exit 1
fi

echo "All docker-agent image contract tests passed"
