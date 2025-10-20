#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

if [[ ${1-} ]]; then
  export HARNESS_SOAK_MINUTES="$1"
elif [[ -z "${HARNESS_SOAK_MINUTES-}" ]]; then
  export HARNESS_SOAK_MINUTES=15
fi

SOAK_MINUTES="${HARNESS_SOAK_MINUTES}"
if ! [[ "$SOAK_MINUTES" =~ ^[0-9]+$ ]]; then
  echo "HARNESS_SOAK_MINUTES must be an integer number of minutes" >&2
  exit 1
fi

TIMEOUT_MINUTES=$(( SOAK_MINUTES + 5 ))
LOG_DIR="${REPO_ROOT}/tmp"
mkdir -p "${LOG_DIR}"
LOG_FILE="${LOG_DIR}/adaptive_soak_$(date +%Y%m%d_%H%M%S).log"

echo "Running adaptive polling soak test for ${SOAK_MINUTES} minute(s)..."
set -o pipefail
go test -tags=integration ./internal/monitoring -run TestAdaptiveSchedulerSoak -soak -timeout "${TIMEOUT_MINUTES}m" 2>&1 | tee "${LOG_FILE}"
STATUS=${PIPESTATUS[0]}
echo "Soak test log saved to ${LOG_FILE}"
exit ${STATUS}
