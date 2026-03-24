#!/usr/bin/env bash
#
# Smoke test for scripts/toggle-mock.sh managed runtime helpers.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TOGGLE_MOCK="${ROOT_DIR}/scripts/toggle-mock.sh"

if [[ ! -f "${TOGGLE_MOCK}" ]]; then
  echo "toggle-mock.sh not found at ${TOGGLE_MOCK}" >&2
  exit 1
fi

failures=0
temp_dirs=()

cleanup() {
  local dir
  for dir in "${temp_dirs[@]:-}"; do
    rm -rf "${dir}" 2>/dev/null || true
  done
}
trap cleanup EXIT

assert_contains() {
  local desc="$1"
  local haystack="$2"
  local needle="$3"

  if [[ "${haystack}" == *"${needle}"* ]]; then
    echo "[PASS] ${desc}"
  else
    echo "[FAIL] ${desc}" >&2
    echo "Expected to find: ${needle}" >&2
    ((failures++))
  fi
}

make_stub_hot_dev_bg() {
  local dir
  dir="$(mktemp -d)"
  temp_dirs+=("${dir}")
  cat > "${dir}/hot-dev-bg.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

COMMAND="${1:-}"
shift || true

printf "%s %s\n" "${COMMAND}" "$*" >> "${HOT_DEV_BG_STUB_LOG}"

case "${COMMAND}" in
  status)
    if [[ "${HOT_DEV_BG_STUB_RUNNING:-false}" == "true" ]]; then
      echo "[hot-dev-bg] Running (pid: 12345)"
      echo "[hot-dev-bg] Frontend shell HTTP: 200"
      echo "[hot-dev-bg] Frontend proxy /api/health: 200"
      echo "[hot-dev-bg] Backend /api/health: 200"
    else
      echo "[hot-dev-bg] Not running"
    fi
    ;;
  start)
    echo "[hot-dev-bg] Started"
    ;;
  stop)
    echo "[hot-dev-bg] Stopped"
    ;;
  *)
    echo "[hot-dev-bg] ${COMMAND}"
    ;;
esac
EOF
  chmod +x "${dir}/hot-dev-bg.sh"
  printf "%s\n" "${dir}/hot-dev-bg.sh"
}

test_detects_managed_hot_dev_runtime() {
  local stub state output
  state="$(mktemp -d)"
  temp_dirs+=("${state}")
  stub="$(make_stub_hot_dev_bg)"

  output="$(
    HOT_DEV_BG_PATH="${stub}" \
    HOT_DEV_BG_STUB_LOG="${state}/calls.log" \
    HOT_DEV_BG_STUB_RUNNING=true \
    TOGGLE_MOCK_PATH="${TOGGLE_MOCK}" \
    bash -lc '
      source "${TOGGLE_MOCK_PATH}"
      printf "mode=%s\n" "$(detect_runtime_mode)"
    '
  )"

  assert_contains "detect_runtime_mode prefers managed hot-dev" "${output}" "mode=managed-hot-dev"
}

test_managed_hot_dev_start_uses_takeover() {
  local stub state output calls
  state="$(mktemp -d)"
  temp_dirs+=("${state}")
  stub="$(make_stub_hot_dev_bg)"

  output="$(
    HOT_DEV_BG_PATH="${stub}" \
    HOT_DEV_BG_STUB_LOG="${state}/calls.log" \
    HOT_DEV_BG_STUB_RUNNING=false \
    TOGGLE_MOCK_PATH="${TOGGLE_MOCK}" \
    bash -lc '
      source "${TOGGLE_MOCK_PATH}"
      start_hot_dev_runtime
    '
  )"
  calls="$(cat "${state}/calls.log")"

  assert_contains "start_hot_dev_runtime announces managed startup" "${output}" "Starting managed hot-dev runtime"
  assert_contains "start_hot_dev_runtime uses managed takeover start" "${calls}" "start --takeover"
}

test_managed_hot_dev_stop_uses_control_plane() {
  local stub state output calls
  state="$(mktemp -d)"
  temp_dirs+=("${state}")
  stub="$(make_stub_hot_dev_bg)"

  output="$(
    HOT_DEV_BG_PATH="${stub}" \
    HOT_DEV_BG_STUB_LOG="${state}/calls.log" \
    HOT_DEV_BG_STUB_RUNNING=true \
    TOGGLE_MOCK_PATH="${TOGGLE_MOCK}" \
    bash -lc '
      source "${TOGGLE_MOCK_PATH}"
      stop_hot_dev_runtime managed-hot-dev
    '
  )"
  calls="$(cat "${state}/calls.log")"

  assert_contains "stop_hot_dev_runtime announces managed stop" "${output}" "Stopping managed hot-dev runtime"
  assert_contains "stop_hot_dev_runtime uses managed stop command" "${calls}" "stop "
}

main() {
  test_detects_managed_hot_dev_runtime
  test_managed_hot_dev_start_uses_takeover
  test_managed_hot_dev_stop_uses_control_plane

  if (( failures > 0 )); then
    echo "Total failures: ${failures}" >&2
    exit 1
  fi

  echo "toggle-mock managed runtime smoke tests passed."
}

main "$@"
