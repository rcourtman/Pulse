#!/usr/bin/env bash
# Integration test for install-docker-agent.sh (deprecated wrapper).

set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
CONTAINER_IMAGE="${INTEGRATION_DOCKER_IMAGE:-ubuntu:22.04}"

log() {
  printf '[integration] %s\n' "$*"
}

if ! command -v docker >/dev/null 2>&1; then
  log "Docker not available. Skipping docker-agent integration test."
  exit 0
fi

log "Running docker-agent wrapper integration test in ${CONTAINER_IMAGE}"

container_script="$(mktemp -t pulse-docker-agent-wrapper-integ-XXXXXX.sh)"
trap 'rm -f "${container_script}"' EXIT

cat <<'EOS' >"${container_script}"
#!/usr/bin/env bash
set -euo pipefail

INSTALLER_SCRIPT="/workspace/scripts/install-docker-agent.sh"
STUB_DIR="/tmp/installer-stubs"
DELEGATED_ARGS_FILE="/tmp/delegated-args.txt"
mkdir -p "${STUB_DIR}"

if [[ ! -f "${INSTALLER_SCRIPT}" ]]; then
  echo "Installer script missing: ${INSTALLER_SCRIPT}" >&2
  exit 1
fi

cat <<'EOF' >"${STUB_DIR}/curl"
#!/usr/bin/env bash
set -euo pipefail
cat <<'SCRIPT'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$@" > "${PULSE_DELEGATED_ARGS_FILE:?}"
SCRIPT
EOF
chmod +x "${STUB_DIR}/curl"

test_missing_url() {
  set +e
  output="$(PATH="${STUB_DIR}:$PATH" bash "${INSTALLER_SCRIPT}" 2>&1)"
  status=$?
  set -e
  [[ ${status} -ne 0 ]] || { echo "expected missing-url failure"; echo "${output}"; exit 1; }
  [[ "${output}" == *"--url is required"* ]] || { echo "missing-url output mismatch"; echo "${output}"; exit 1; }
}

test_missing_token() {
  set +e
  output="$(PATH="${STUB_DIR}:$PATH" bash "${INSTALLER_SCRIPT}" --url http://primary.local:7655 2>&1)"
  status=$?
  set -e
  [[ ${status} -ne 0 ]] || { echo "expected missing-token failure"; echo "${output}"; exit 1; }
  [[ "${output}" == *"--token is required"* ]] || { echo "missing-token output mismatch"; echo "${output}"; exit 1; }
}

test_delegation() {
  rm -f "${DELEGATED_ARGS_FILE}"
  PATH="${STUB_DIR}:$PATH" \
  PULSE_DELEGATED_ARGS_FILE="${DELEGATED_ARGS_FILE}" \
  bash "${INSTALLER_SCRIPT}" --url http://primary.local:7655 --token tok_integration >/tmp/install.log 2>&1

  [[ -f "${DELEGATED_ARGS_FILE}" ]] || { echo "delegated args file missing"; cat /tmp/install.log; exit 1; }
  grep -Fx -- "--url" "${DELEGATED_ARGS_FILE}" >/dev/null
  grep -Fx -- "http://primary.local:7655" "${DELEGATED_ARGS_FILE}" >/dev/null
  grep -Fx -- "--token" "${DELEGATED_ARGS_FILE}" >/dev/null
  grep -Fx -- "tok_integration" "${DELEGATED_ARGS_FILE}" >/dev/null
  grep -Fx -- "--enable-docker" "${DELEGATED_ARGS_FILE}" >/dev/null
}

test_missing_url
test_missing_token
test_delegation

echo "docker-agent wrapper integration checks passed"
EOS

chmod +x "${container_script}"

docker run --rm \
  -v "${PROJECT_ROOT}:/workspace" \
  -v "${container_script}:/tmp/test.sh:ro" \
  "${CONTAINER_IMAGE}" \
  bash /tmp/test.sh

log "Docker-agent wrapper integration test passed."
