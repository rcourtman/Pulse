#!/usr/bin/env bash
# Integration test for install-docker-agent-v2.sh

set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
CONTAINER_IMAGE="${INTEGRATION_DOCKER_IMAGE:-ubuntu:22.04}"
log() {
  printf '[integration] %s\n' "$*"
}

if ! command -v docker >/dev/null 2>&1; then
  log "Docker not available. Skipping docker-agent integration tests."
  exit 0
fi

log "Running docker-agent installer integration tests in ${CONTAINER_IMAGE}"

container_script="$(mktemp -t pulse-docker-agent-integ-XXXXXX.sh)"
trap 'rm -f "${container_script}"' EXIT

cat <<'EOS' >"${container_script}"
#!/usr/bin/env bash
set -euo pipefail

INSTALLER_SCRIPT="/workspace/scripts/install-docker-agent-v2.sh"
STUB_DIR=/tmp/installer-stubs
mkdir -p "${STUB_DIR}"

create_systemctl_stub() {
  cat <<'EOF' >"${STUB_DIR}/systemctl"
#!/usr/bin/env bash
set -eo pipefail

cmd=""
for arg in "$@"; do
  case "$arg" in
    list-unit-files|is-active|daemon-reload|enable|start|stop|disable|restart)
      cmd="$arg"
      shift
      break
      ;;
  esac
  shift || true
done

case "$cmd" in
  list-unit-files)
    exit 1
    ;;
  is-active)
    unit=""
    for arg in "$@"; do
      case "$arg" in
        --*) ;;
        *) unit="$arg"; break ;;
      esac
    done
    if [[ -n "$unit" && -f "/etc/systemd/system/$unit" ]]; then
      exit 0
    fi
    exit 3
    ;;
  *)
    exit 0
    ;;
esac
EOF
  chmod +x "${STUB_DIR}/systemctl"
}

create_curl_stub() {
  cat <<'EOF' >"${STUB_DIR}/curl"
#!/usr/bin/env bash
set -eo pipefail

outfile=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    -o|--output)
      outfile="$2"; shift 2;;
    -H|-d|-X|--header|--data|--request|--url)
      shift 2;;
    -f|-s|-S|-L|-k|--progress-bar|--connect-timeout|--retry|--retry-delay|--silent)
      shift;;
    *)
      shift;;
  esac
done

if [[ -n "${outfile:-}" ]]; then
  cat <<'SCRIPT' >"$outfile"
#!/usr/bin/env bash
if [[ "$1" == "--help" ]]; then
  echo "--no-auto-update"
fi
exit 0
SCRIPT
  chmod +x "$outfile"
else
  printf '{"ok":true}\n'
fi
EOF
  chmod +x "${STUB_DIR}/curl"
}

create_wget_stub() {
  cat <<'EOF' >"${STUB_DIR}/wget"
#!/usr/bin/env bash
set -eo pipefail

outfile=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    -O|--output-document)
      outfile="$2"; shift 2;;
    --header|--method|--body-data)
      shift 2;;
    -q|--quiet|--show-progress|--no-check-certificate)
      shift;;
    *)
      shift;;
  esac
done

if [[ -n "${outfile:-}" ]]; then
  cat <<'SCRIPT' >"$outfile"
#!/usr/bin/env bash
if [[ "$1" == "--help" ]]; then
  echo "--no-auto-update"
fi
exit 0
SCRIPT
  chmod +x "$outfile"
else
  printf '{"ok":true}\n'
fi
EOF
  chmod +x "${STUB_DIR}/wget"
}

create_docker_stub() {
  cat <<'EOF' >"${STUB_DIR}/docker"
#!/usr/bin/env bash
set -eo pipefail

if [[ "${1:-}" == "info" ]]; then
  if [[ "${2:-}" == "--format" ]]; then
    printf 'stub-host-id\n'
  else
    printf '{}\n'
  fi
  exit 0
fi
exit 0
EOF
  chmod +x "${STUB_DIR}/docker"
}

create_stubs() {
  create_systemctl_stub
  create_curl_stub
  create_wget_stub
  create_docker_stub
  export PATH="${STUB_DIR}:$PATH"
}

assert_file_contains() {
  local file=$1
  local expected=$2
  if ! grep -Fq "$expected" "$file"; then
    echo "Expected to find \"$expected\" in $file" >&2
    exit 1
  fi
}

test_dry_run() {
  echo "== Dry-run scenario =="
  set +e
  output="$(PULSE_FORCE_INTERACTIVE=1 PATH="${STUB_DIR}:$PATH" bash "${INSTALLER_SCRIPT}" --dry-run --url http://primary.local --token token123 2>&1)"
  status=$?
  set -e
  if [[ $status -ne 0 ]]; then
    echo "$output"
    echo "dry-run failed"
    exit 1
  fi
  if [[ "$output" != *"[dry-run]"* ]]; then
    echo "$output"
    echo "dry-run output missing markers"
    exit 1
  fi
}

test_install_basic() {
  echo "== Basic installation =="
  rm -f /usr/local/bin/pulse-docker-agent /etc/systemd/system/pulse-docker-agent.service
  PULSE_FORCE_INTERACTIVE=1 PATH="${STUB_DIR}:$PATH" bash "${INSTALLER_SCRIPT}" --url http://primary.local --token token123 --interval 15s >/tmp/install.log 2>&1
  test -f /usr/local/bin/pulse-docker-agent
  test -f /etc/systemd/system/pulse-docker-agent.service
  assert_file_contains /etc/systemd/system/pulse-docker-agent.service "Environment=\"PULSE_URL=http://primary.local\""
  assert_file_contains /etc/systemd/system/pulse-docker-agent.service "ExecStart=/usr/local/bin/pulse-docker-agent --url \"http://primary.local\" --interval \"15s\""
}

test_install_without_docker() {
  echo "== Installation without docker binary =="
  mv "${STUB_DIR}/docker" "${STUB_DIR}/docker.disabled"
  set +e
  output="$(printf 'y\n' | PULSE_FORCE_INTERACTIVE=1 PATH="${STUB_DIR}:$PATH" bash "${INSTALLER_SCRIPT}" --dry-run --url http://nodocker.local --token token456 2>&1)"
  status=${PIPESTATUS[1]:-0}
  set -e
  mv "${STUB_DIR}/docker.disabled" "${STUB_DIR}/docker"
  if [[ $status -ne 0 ]]; then
    echo "$output"
    echo "dry-run without docker failed"
    exit 1
  fi
  if [[ "$output" != *"Docker not found"* ]]; then
    echo "$output"
    echo "expected docker warning"
    exit 1
  fi
}

test_multi_target() {
  echo "== Multi-target installation =="
  rm -f /usr/local/bin/pulse-docker-agent /etc/systemd/system/pulse-docker-agent.service
  PULSE_FORCE_INTERACTIVE=1 PATH="${STUB_DIR}:$PATH" bash "${INSTALLER_SCRIPT}" \
    --url http://primary.local \
    --token token789 \
    --target 'https://target.one|tok1' \
    --target 'https://target.two|tok2' \
    >/tmp/install-multi.log 2>&1
  if [[ ! -f /etc/systemd/system/pulse-docker-agent.service ]]; then
    echo "ERROR: Service file doesn't exist!" >&2
    exit 1
  fi

  echo "Service file contents:"
  cat /etc/systemd/system/pulse-docker-agent.service

  assert_file_contains /etc/systemd/system/pulse-docker-agent.service 'PULSE_TARGETS='
}

test_uninstall() {
  echo "== Uninstall =="
  PATH="${STUB_DIR}:$PATH" bash "${INSTALLER_SCRIPT}" --uninstall >/tmp/uninstall.log 2>&1
  if [[ -f /etc/systemd/system/pulse-docker-agent.service ]]; then
    echo "service file still present after uninstall" >&2
    exit 1
  fi
  if [[ -f /usr/local/bin/pulse-docker-agent ]]; then
    echo "agent binary still present after uninstall" >&2
    exit 1
  fi
}

create_stubs
test_dry_run
test_install_basic
test_install_without_docker
test_multi_target
test_uninstall

echo "All docker-agent installer integration scenarios passed."
EOS

chmod +x "${container_script}"

docker run --rm \
  -v "${PROJECT_ROOT}:/workspace:ro" \
  -v "${container_script}:/tmp/integration-test.sh:ro" \
  -w /workspace \
  "${CONTAINER_IMAGE}" \
  bash /tmp/integration-test.sh

log "Integration testing complete."
