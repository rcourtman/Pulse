#!/usr/bin/env bash
#
# Smoke test: install-sensor-proxy HTTP mode (uninstall → install → health check)

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ARTIFACT_DIR="${ROOT_DIR}/tmp/sensor-proxy-test"
LOCAL_BINARY="${ARTIFACT_DIR}/pulse-sensor-proxy-linux-amd64"
DOCKER_IMAGE="${SENSOR_PROXY_TEST_IMAGE:-debian:12}"

log() {
  printf '[sensor-proxy-test] %s\n' "$*"
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    log "Missing required command: $1"
    return 1
  fi
}

cleanup() {
  [[ -n "${CONTAINER_SCRIPT:-}" && -f "${CONTAINER_SCRIPT}" ]] && rm -f "${CONTAINER_SCRIPT}"
}
trap cleanup EXIT

main() {
  if ! require_cmd go; then
    log "Go toolchain is required for this test. Skipping."
    return 0
  fi
  if ! require_cmd docker; then
    log "Docker not available. Skipping sensor-proxy installer test."
    return 0
  fi

  mkdir -p "${ARTIFACT_DIR}"
  log "Building pulse-sensor-proxy binary for test harness..."
  (
    cd "${ROOT_DIR}"
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o "${LOCAL_BINARY}" ./cmd/pulse-sensor-proxy
  )

  CONTAINER_SCRIPT="$(mktemp -t sensor-proxy-http-XXXXXX.sh)"
  cat <<'EOS' >"${CONTAINER_SCRIPT}"
#!/usr/bin/env bash
set -euo pipefail

export DEBIAN_FRONTEND=noninteractive
apt-get update >/dev/null
apt-get install -y --no-install-recommends ca-certificates curl openssl >/dev/null

INSTALLER="/workspace/scripts/install-sensor-proxy.sh"
LOCAL_BIN="${SENSOR_PROXY_LOCAL_BINARY:-/artifacts/pulse-sensor-proxy-linux-amd64}"
STUB_DIR=/tmp/sensor-proxy-stubs
mkdir -p "${STUB_DIR}"

create_curl_stub() {
  cat <<'EOF' >"${STUB_DIR}/curl"
#!/usr/bin/env bash
set -eo pipefail

args="$*"
if [[ "${args}" == *"/api/temperature-proxy/register"* ]]; then
  printf '{"success":true,"token":"INTEGRATION-TOKEN","pve_instance":"integration"}\n200\n'
  exit 0
fi

if [[ "${args}" == *"/api/health"* ]]; then
  printf '{"status":"ok"}\n'
  exit 0
fi

exec /usr/bin/curl "$@"
EOF
  chmod +x "${STUB_DIR}/curl"
}

create_systemctl_stub() {
  cat <<'EOF' >"${STUB_DIR}/systemctl"
#!/usr/bin/env bash
set -eo pipefail

PID_FILE=/run/pulse-sensor-proxy-test.pid
LOG_FILE=/var/log/pulse/sensor-proxy/integration.log

cmd=""
declare -a units=()
for arg in "$@"; do
  case "$arg" in
    start|stop|restart|status|is-active|enable|disable|daemon-reload)
      cmd="$arg"
      ;;
    --*) ;;
    *)
      units+=("$arg")
      ;;
  esac
done

if [[ -z "${cmd}" ]]; then
  exit 0
fi
if [[ ${#units[@]} -eq 0 ]]; then
  units=("pulse-sensor-proxy.service")
fi

is_proxy_unit() {
  local unit="$1"
  [[ "$unit" == "pulse-sensor-proxy" || "$unit" == "pulse-sensor-proxy.service" ]]
}

start_proxy() {
  mkdir -p "$(dirname "${LOG_FILE}")"
  if [[ -f "${PID_FILE}" ]]; then
    local old_pid
    old_pid="$(cat "${PID_FILE}")"
    kill "${old_pid}" 2>/dev/null || true
    rm -f "${PID_FILE}"
  fi
  /usr/local/bin/pulse-sensor-proxy --config /etc/pulse-sensor-proxy/config.yaml >>"${LOG_FILE}" 2>&1 &
  echo $! >"${PID_FILE}"
}

stop_proxy() {
  if [[ -f "${PID_FILE}" ]]; then
    local pid
    pid="$(cat "${PID_FILE}")"
    kill "${pid}" 2>/dev/null || true
    rm -f "${PID_FILE}"
  fi
}

case "${cmd}" in
  start)
    for unit in "${units[@]}"; do
      if is_proxy_unit "${unit}"; then
        start_proxy
      fi
    done
    ;;
  stop)
    for unit in "${units[@]}"; do
      if is_proxy_unit "${unit}"; then
        stop_proxy
      fi
    done
    ;;
  restart)
    stop_proxy
    start_proxy
    ;;
  status)
    if [[ -f "${PID_FILE}" && -d "/proc/$(cat "${PID_FILE}")" ]]; then
      echo "pulse-sensor-proxy.service active"
      exit 0
    fi
    echo "pulse-sensor-proxy.service inactive"
    exit 3
    ;;
  is-active)
    if [[ -f "${PID_FILE}" && -d "/proc/$(cat "${PID_FILE}")" ]]; then
      exit 0
    fi
    exit 3
    ;;
  *)
    ;;
esac

exit 0
EOF
  chmod +x "${STUB_DIR}/systemctl"
}

assert_file_contains() {
  local file="$1"
  local text="$2"
  if ! grep -Fq "$text" "$file"; then
    echo "Assertion failed: \"$text\" not found in $file" >&2
    exit 1
  fi
}

assert_not_exists() {
  local target="$1"
  if [[ -e "$target" ]]; then
    echo "Assertion failed: expected $target to be absent" >&2
    exit 1
  fi
}

create_curl_stub
create_systemctl_stub
export PATH="${STUB_DIR}:$PATH"

if [[ ! -f "${LOCAL_BIN}" ]]; then
  echo "Local binary not found at ${LOCAL_BIN}" >&2
  exit 1
fi

PULSE_FORCE_INTERACTIVE=1 bash "${INSTALLER}" \
  --standalone \
  --http-mode \
  --pulse-server http://pulse.local:7655 \
  --local-binary "${LOCAL_BIN}" >/tmp/install.log 2>&1

CONFIG_FILE="/etc/pulse-sensor-proxy/config.yaml"
assert_file_contains "${CONFIG_FILE}" "http_enabled: true"
assert_file_contains "${CONFIG_FILE}" "http_auth_token: \"INTEGRATION-TOKEN\""
assert_file_contains "${CONFIG_FILE}" "127.0.0.1/32"

/usr/bin/curl -k -s \
  -H "Authorization: Bearer INTEGRATION-TOKEN" \
  https://127.0.0.1:8443/health >/tmp/health.json
assert_file_contains /tmp/health.json '"status":"ok"'

PULSE_FORCE_INTERACTIVE=1 bash "${INSTALLER}" --uninstall --purge >/tmp/uninstall.log 2>&1
assert_not_exists /usr/local/bin/pulse-sensor-proxy
assert_not_exists /etc/systemd/system/pulse-sensor-proxy.service

echo "Sensor proxy HTTP installation smoke test passed."
EOS
  chmod +x "${CONTAINER_SCRIPT}"

  log "Running sensor-proxy installer test in ${DOCKER_IMAGE}"
  docker run --rm \
    -v "${ROOT_DIR}:/workspace:ro" \
    -v "${ARTIFACT_DIR}:/artifacts:ro" \
    -e SENSOR_PROXY_LOCAL_BINARY="/artifacts/pulse-sensor-proxy-linux-amd64" \
    "${DOCKER_IMAGE}" bash -s <"${CONTAINER_SCRIPT}"

  log "Test completed successfully."
}

main "$@"
