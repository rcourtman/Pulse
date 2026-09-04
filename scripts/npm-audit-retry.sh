#!/usr/bin/env bash
# Retry transient npm audit service failures without hiding vulnerability findings.

set -euo pipefail

readonly max_attempts=3
readonly fetch_timeout_ms=60000
attempt=1
output="$(mktemp)"
trap 'rm -f -- "${output}"' EXIT

while (( attempt <= max_attempts )); do
  : >"${output}"
  set +e
  npm audit --fetch-timeout="${fetch_timeout_ms}" "$@" 2>&1 | tee "${output}"
  status=${PIPESTATUS[0]}
  set -e

  if (( status == 0 )); then
    exit 0
  fi

  # An advisory result takes precedence if npm also emits a transport warning.
  # Never turn a real vulnerability finding into a retryable service failure.
  if grep -Fq '# npm audit report' "${output}"; then
    exit "${status}"
  fi

  # npm audit uses a registry POST. npm's fetch retries cover idempotent reads,
  # so a transient timeout or audit endpoint error otherwise consumes the full
  # default five-minute timeout and fails the check without another attempt.
  # A real advisory report is not an infrastructure error and must remain an
  # immediate, fail-closed result.
  if ! grep -Eiq \
    'npm (warn|error) audit (network|endpoint|429 |5[0-9]{2} )|npm error (code )?(EAI_AGAIN|ECONNRESET|ECONNREFUSED|ENETUNREACH|ETIMEDOUT|ERR_SOCKET_TIMEOUT|FETCH_ERROR)|npm error network|npm error request to .* failed' \
    "${output}"; then
    exit "${status}"
  fi

  if (( attempt == max_attempts )); then
    echo "npm audit registry request failed after ${max_attempts} attempts." >&2
    exit "${status}"
  fi

  echo "npm audit registry request failed on attempt ${attempt}; retrying." >&2
  sleep 5
  ((attempt += 1))
done
