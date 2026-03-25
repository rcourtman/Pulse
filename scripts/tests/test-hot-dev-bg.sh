#!/usr/bin/env bash
#
# Smoke test for scripts/hot-dev-bg.sh managed ownership diagnostics.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
HOT_DEV_BG="${ROOT_DIR}/scripts/hot-dev-bg.sh"
HOT_DEV="${ROOT_DIR}/scripts/hot-dev.sh"
CLEAN_MOCK_ALERTS="${ROOT_DIR}/scripts/clean-mock-alerts.sh"
DEV_CHECK="${ROOT_DIR}/scripts/dev-check.sh"
PACKAGE_JSON="${ROOT_DIR}/package.json"
FRONTEND_PACKAGE_JSON="${ROOT_DIR}/frontend-modern/package.json"
DEV_LAUNCHD_WRAPPER="${ROOT_DIR}/scripts/dev-launchd-wrapper.sh"
DEV_LAUNCHD_SETUP="${ROOT_DIR}/scripts/dev-launchd-setup.sh"
INTEGRATION_README="${ROOT_DIR}/tests/integration/README.md"

if [[ ! -x "${HOT_DEV_BG}" ]]; then
  echo "hot-dev-bg.sh not found or not executable at ${HOT_DEV_BG}" >&2
  exit 1
fi

failures=0
server_pids=()
temp_dirs=()

cleanup() {
  local pid
  for pid in "${server_pids[@]:-}"; do
    kill "${pid}" 2>/dev/null || true
  done
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

assert_not_contains() {
  local desc="$1"
  local haystack="$2"
  local needle="$3"

  if [[ "${haystack}" == *"${needle}"* ]]; then
    echo "[FAIL] ${desc}" >&2
    echo "Did not expect to find: ${needle}" >&2
    ((failures++))
  else
    echo "[PASS] ${desc}"
  fi
}

pick_free_port() {
  python3 - <<'PY'
import socket

sock = socket.socket()
sock.bind(("127.0.0.1", 0))
print(sock.getsockname()[1])
sock.close()
PY
}

start_http_server() {
  local port="$1"
  python3 -m http.server "${port}" --bind 127.0.0.1 >/dev/null 2>&1 &
  local pid=$!
  server_pids+=("${pid}")
  sleep 1
}

make_isolated_hot_dev_bg_state() {
  local dir
  dir="$(mktemp -d)"
  temp_dirs+=("${dir}")
  printf "%s\n" "${dir}"
}

test_status_without_runtime() {
  local frontend_port backend_port output
  local state_dir
  frontend_port="$(pick_free_port)"
  backend_port="$(pick_free_port)"
  if [[ "${backend_port}" == "${frontend_port}" ]]; then
    backend_port="$(pick_free_port)"
  fi
  state_dir="$(make_isolated_hot_dev_bg_state)"

  output="$(
    HOT_DEV_BG_PID_FILE="${state_dir}/hot-dev-bg.pid" \
    HOT_DEV_BG_LOG_FILE="${state_dir}/hot-dev-bg.log" \
    FRONTEND_DEV_HOST=127.0.0.1 \
    FRONTEND_DEV_PORT="${frontend_port}" \
    PULSE_DEV_API_HOST=127.0.0.1 \
    PULSE_DEV_API_PORT="${backend_port}" \
    "${HOT_DEV_BG}" status
  )"

  assert_contains "status reports no managed runtime" "${output}" "[hot-dev-bg] Not running"
  assert_contains "status reports the browser entrypoint" "${output}" "[hot-dev-bg] Browser entrypoint: http://127.0.0.1:${frontend_port}"
  assert_contains "status reports idle frontend port" "${output}" "[hot-dev-bg] Frontend port ${frontend_port} is not listening"
  assert_contains "status reports idle backend port" "${output}" "[hot-dev-bg] Backend port ${backend_port} is not listening"
  assert_contains "status reports proxy health probe" "${output}" "[hot-dev-bg] Frontend proxy /api/health: 000"
}

test_cli_parses_takeover_flag() {
  local output
  output="$(
    HOT_DEV_BG_PATH="${HOT_DEV_BG}" \
    bash -lc '
      source "${HOT_DEV_BG_PATH}"
      printf "start=%s\n" "$(parse_takeover_flag start --takeover)"
      printf "restart=%s\n" "$(parse_takeover_flag restart --takeover)"
      printf "verify=%s\n" "$(parse_takeover_flag verify --takeover)"
      printf "launchd=%s\n" "$(parse_takeover_flag launchd-session --takeover)"
      printf "backend_restart=%s\n" "$(parse_takeover_flag backend-restart)"
      printf "plain=%s\n" "$(parse_takeover_flag start)"
      if parse_takeover_flag status --takeover >/tmp/hot-dev-bg.invalid 2>&1; then
        printf "invalid=accepted\n"
      else
        printf "invalid=rejected\n"
      fi
    '
  )"

  assert_contains "takeover parsing enables start" "${output}" "start=true"
  assert_contains "takeover parsing enables restart" "${output}" "restart=true"
  assert_contains "takeover parsing enables verify" "${output}" "verify=true"
  assert_contains "takeover parsing enables launchd-session" "${output}" "launchd=true"
  assert_contains "backend restart remains flagless" "${output}" "backend_restart=false"
  assert_contains "start without flag stays false" "${output}" "plain=false"
  assert_contains "unexpected status flag is rejected" "${output}" "invalid=rejected"
}

test_verify_command_injects_managed_runtime_env() {
  local output
  output="$(
    HOT_DEV_BG_PATH="${HOT_DEV_BG}" \
    bash -lc '
      source "${HOT_DEV_BG_PATH}"
      HOT_DEV_BG_VERIFY_COMMAND="python3 - <<'\''PY'\''
import os
print(f\"mode={os.environ.get('\''PULSE_E2E_USE_HOT_DEV'\'')}\")
print(f\"username={os.environ.get('\''PULSE_E2E_USERNAME'\'')}\")
print(f\"password={os.environ.get('\''PULSE_E2E_PASSWORD'\'')}\")
PY"
      run_verify_proof_command
    '
  )"

  assert_contains "verify proof forces managed hot-dev mode" "${output}" "mode=1"
  assert_contains "verify proof defaults username" "${output}" "username=admin"
  assert_contains "verify proof defaults password" "${output}" "password=admin"
}

test_default_verify_command_runs_runtime_and_layout_proofs() {
  local test_dir fake_bin output
  test_dir="$(mktemp -d)"
  temp_dirs+=("${test_dir}")
  fake_bin="${test_dir}/bin"
  mkdir -p "${fake_bin}"

  cat > "${fake_bin}/npm" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf 'cwd=%s\n' "$PWD"
printf 'args=%s\n' "$*"
EOF
  chmod +x "${fake_bin}/npm"

  output="$(
    FAKE_BIN_PATH="${fake_bin}" \
    HOT_DEV_BG_PATH="${HOT_DEV_BG}" \
    bash -lc '
      export PATH="${FAKE_BIN_PATH}:$PATH"
      source "${HOT_DEV_BG_PATH}"
      run_verify_proof_command
    '
  )"

  assert_contains "default verify proof runs from integration harness" "${output}" "cwd=${ROOT_DIR}/tests/integration"
  assert_contains "default verify proof includes dev runtime recovery spec" "${output}" "tests/16-dev-runtime-recovery.spec.ts"
  assert_contains "default verify proof includes recovery layout spec" "${output}" "tests/17-recovery-layout.spec.ts"
  assert_contains "default verify proof keeps chromium project pin" "${output}" "--project=chromium"
}

test_takeover_avoids_killing_current_shell_lineage() {
  local output
  output="$(
    HOT_DEV_BG_PATH="${HOT_DEV_BG}" \
    bash -lc '
      source "${HOT_DEV_BG_PATH}"
      process_group_id() {
        case "${1:-}" in
          "$$"|900)
            printf "4242\n"
            ;;
          *)
            printf "9999\n"
            ;;
        esac
      }
      process_parent_id() {
        case "${1:-}" in
          "$$")
            printf "900\n"
            ;;
          *)
            printf "1\n"
            ;;
        esac
      }
      hot_dev_root_pids() { printf "900\n"; }
      listener_pids() { printf "901\n"; }
      find_hot_dev_ancestor() { printf "900\n"; }
      process_command() { printf "cmd-%s\n" "${1:-unknown}"; }
      kill() { printf "kill %s\n" "$*"; }

      stop_hot_dev_sessions TERM
      stop_takeover_targets TERM
    '
  )"

  assert_not_contains "takeover does not signal the current process group" "${output}" "kill -TERM -4242"
  assert_not_contains "takeover does not signal the current shell ancestor" "${output}" "kill -TERM 900"
  assert_contains "takeover falls back to killing the occupied listener pid" "${output}" "kill -TERM 901"
}

test_launchd_session_supervises_managed_runtime() {
  local output
  output="$(
    HOT_DEV_BG_PATH="${HOT_DEV_BG}" \
    bash -lc '
      source "${HOT_DEV_BG_PATH}"
      set +e
      test_dir="$(mktemp -d)"
      trap "rm -rf \"$test_dir\"" EXIT
      PID_FILE="${test_dir}/hot-dev-bg.pid"
      LOG_FILE="${test_dir}/hot-dev-bg.log"
      require_python(){ :; }
      start_bg(){ printf "%s\n" "999999" > "${PID_FILE}"; }
      launchd_session_bg true
      status=$?
      printf "status=%s\n" "${status}"
    '
  )"

  assert_contains "launchd session starts supervised runtime" "${output}" "Managed runtime is not running. Starting supervised session..."
  assert_contains "launchd session announces supervision" "${output}" "Supervising managed runtime for launchd"
  assert_contains "launchd session exits nonzero on child crash" "${output}" "status=1"
}

test_start_bg_reports_browser_entrypoint() {
  local test_dir output
  test_dir="$(mktemp -d)"
  temp_dirs+=("${test_dir}")

  output="$(
    HOT_DEV_BG_PATH="${HOT_DEV_BG}" \
    bash -lc '
      source "${HOT_DEV_BG_PATH}"
      PID_FILE="'"${test_dir}"'/hot-dev-bg.pid"
      LOG_FILE="'"${test_dir}"'/hot-dev-bg.log"
      FRONTEND_DEV_PORT=5173
      PULSE_DEV_API_PORT=7655
      has_unmanaged_listeners(){ return 1; }
      is_running(){ return 1; }
      wait_for_managed_listener(){ return 0; }
      spawn_detached_supervisor(){ printf "4242\n"; }
      start_bg false
    '
  )"

  assert_contains "start reports managed runtime supervisor pid" "${output}" "Started managed runtime supervisor (pid: 4242)"
  assert_contains "start reports browser entrypoint" "${output}" "Browser entrypoint: http://127.0.0.1:5173"
  assert_contains "start reports managed backend" "${output}" "Managed backend:  http://127.0.0.1:7655"
  assert_not_contains "start no longer reports generic frontend url" "${output}" "Frontend: http://127.0.0.1:5173"
}

test_supervisor_restarts_unexpected_child_exit() {
  local output
  output="$(
    HOT_DEV_BG_PATH="${HOT_DEV_BG}" \
    bash -lc '
      source "${HOT_DEV_BG_PATH}"
      set +e
      test_dir="$(mktemp -d)"
      trap "rm -rf \"$test_dir\"" EXIT
      count_file="${test_dir}/starts"
      printf "0\n" > "${count_file}"
      start_hot_dev_child() {
        count="$(cat "${count_file}")"
        count=$((count + 1))
        printf "%s\n" "${count}" > "${count_file}"
        if [[ "${count}" -eq 1 ]]; then
          (exit 7) &
        else
          (sleep 30) &
        fi
        HOT_DEV_CHILD_PID="$!"
      }
      stop_hot_dev_child() {
        kill -TERM "${1}" 2>/dev/null || true
      }
      HOT_DEV_BG_SUPERVISOR_RESTART_DELAY=0.1
      run_supervised_session &
      supervisor_pid=$!
      sleep 0.5
      kill -TERM "${supervisor_pid}"
      wait "${supervisor_pid}"
      printf "starts=%s\n" "$(cat "${count_file}")"
    '
  )"

  assert_contains "supervisor reports unexpected child exit" "${output}" "Managed runtime child exited unexpectedly"
  assert_contains "supervisor restarts hot-dev child after exit" "${output}" "starts=2"
}

test_launchd_wrapper_uses_managed_supervisor() {
  local output
  output="$(sed -n '1,80p' "${DEV_LAUNCHD_WRAPPER}")"

  assert_contains "launchd wrapper uses managed launchd-session" "${output}" "scripts/hot-dev-bg.sh launchd-session --takeover"
}

test_launchd_setup_advertises_managed_runtime_controls() {
  local test_dir fake_bin output
  test_dir="$(mktemp -d)"
  temp_dirs+=("${test_dir}")
  fake_bin="${test_dir}/bin"
  mkdir -p "${fake_bin}"

  cat > "${fake_bin}/launchctl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
case "${1:-}" in
  bootout)
    exit 0
    ;;
  bootstrap)
    exit 0
    ;;
  print)
    echo "pid = 4242"
    exit 0
    ;;
  *)
    echo "unexpected launchctl command: $*" >&2
    exit 1
    ;;
esac
EOF
  chmod +x "${fake_bin}/launchctl"

  output="$(
    PATH="${fake_bin}:$PATH" \
    HOME="${test_dir}/home" \
    "${DEV_LAUNCHD_SETUP}" install
  )"

  assert_contains "launchd setup shows browser entrypoint" "${output}" "http://127.0.0.1:5173"
  assert_contains "launchd setup shows managed start command" "${output}" "npm run dev"
  assert_contains "launchd setup shows managed restart command" "${output}" "npm run dev:restart"
  assert_contains "launchd setup shows managed status command" "${output}" "npm run dev:status"
  assert_contains "launchd setup shows managed logs command" "${output}" "npm run dev:logs"
  assert_contains "launchd setup keeps launchctl maintenance commands" "${output}" "launchctl kickstart -k"
}

test_root_package_exposes_managed_runtime_entrypoints() {
  local output
  output="$(
    PACKAGE_JSON_PATH="${PACKAGE_JSON}" \
    python3 - <<'PY'
import json
import os

with open(os.environ["PACKAGE_JSON_PATH"], "r", encoding="utf-8") as fh:
    scripts = json.load(fh)["scripts"]

for key in [
    "dev",
    "dev:status",
    "dev:logs",
    "dev:stop",
    "dev:restart",
    "dev:backend-restart",
    "dev:verify",
    "dev:foreground",
]:
    print(f"{key}={scripts.get(key, '')}")
PY
  )"

  assert_contains "root package exposes managed dev start" "${output}" "dev=./scripts/hot-dev-bg.sh start --takeover"
  assert_contains "root package exposes managed dev status" "${output}" "dev:status=./scripts/hot-dev-bg.sh status"
  assert_contains "root package exposes managed dev logs" "${output}" "dev:logs=./scripts/hot-dev-bg.sh logs"
  assert_contains "root package exposes managed dev stop" "${output}" "dev:stop=./scripts/hot-dev-bg.sh stop"
  assert_contains "root package exposes managed dev restart" "${output}" "dev:restart=./scripts/hot-dev-bg.sh restart --takeover"
  assert_contains "root package exposes managed backend restart" "${output}" "dev:backend-restart=./scripts/hot-dev-bg.sh backend-restart"
  assert_contains "root package exposes managed dev verify" "${output}" "dev:verify=./scripts/hot-dev-bg.sh verify --takeover"
  assert_contains "root package keeps foreground escape hatch" "${output}" "dev:foreground=./scripts/hot-dev.sh"
}

test_frontend_package_exposes_managed_runtime_entrypoints() {
  local output
  output="$(
    PACKAGE_JSON_PATH="${FRONTEND_PACKAGE_JSON}" \
    python3 - <<'PY'
import json
import os

with open(os.environ["PACKAGE_JSON_PATH"], "r", encoding="utf-8") as fh:
    scripts = json.load(fh)["scripts"]

for key in [
    "dev",
    "dev:logs",
    "dev:restart",
    "dev:backend-restart",
    "dev:status",
    "dev:stop",
    "dev:verify",
    "dev:foreground",
    "dev:frontend-only",
]:
    print(f"{key}={scripts.get(key, '')}")
PY
  )"

  assert_contains "frontend package exposes managed dev start" "${output}" "dev=../scripts/hot-dev-bg.sh start --takeover"
  assert_contains "frontend package exposes managed dev logs" "${output}" "dev:logs=../scripts/hot-dev-bg.sh logs"
  assert_contains "frontend package exposes managed dev restart" "${output}" "dev:restart=../scripts/hot-dev-bg.sh restart --takeover"
  assert_contains "frontend package exposes managed backend restart" "${output}" "dev:backend-restart=../scripts/hot-dev-bg.sh backend-restart"
  assert_contains "frontend package exposes managed dev status" "${output}" "dev:status=../scripts/hot-dev-bg.sh status"
  assert_contains "frontend package exposes managed dev stop" "${output}" "dev:stop=../scripts/hot-dev-bg.sh stop"
  assert_contains "frontend package exposes managed dev verify" "${output}" "dev:verify=../scripts/hot-dev-bg.sh verify --takeover"
  assert_contains "frontend package keeps foreground managed launcher" "${output}" "dev:foreground=../scripts/hot-dev.sh"
  assert_contains "frontend package keeps explicit frontend-only escape hatch" "${output}" "dev:frontend-only=vite"
}

test_makefile_routes_managed_runtime_through_npm() {
  local output
  output="$(cat "${ROOT_DIR}/Makefile")"

  assert_contains "make dev routes through npm wrapper" "${output}" $'dev:\n\tnpm run dev'
  assert_contains "make dev-status routes through npm wrapper" "${output}" $'dev-status:\n\tnpm run dev:status'
  assert_contains "make dev-logs routes through npm wrapper" "${output}" $'dev-logs:\n\tnpm run dev:logs'
  assert_contains "make dev-stop routes through npm wrapper" "${output}" $'dev-stop:\n\tnpm run dev:stop'
  assert_contains "make dev-restart routes through npm wrapper" "${output}" $'dev-restart:\n\tnpm run dev:restart'
  assert_contains "make dev-backend-restart routes through npm wrapper" "${output}" $'dev-backend-restart:\n\tnpm run dev:backend-restart'
  assert_contains "make dev-verify routes through npm wrapper" "${output}" $'dev-verify:\n\tnpm run dev:verify'
  assert_contains "make dev-foreground routes through npm wrapper" "${output}" $'dev-foreground:\n\tnpm run dev:foreground'
  assert_contains "make dev-hot routes through foreground wrapper" "${output}" $'dev-hot:\n\tnpm run dev:foreground'
}

test_hot_dev_script_advertises_foreground_escape_hatch() {
  local output
  output="$(sed -n '1,30p' "${HOT_DEV}")"

  assert_contains "hot-dev header identifies foreground escape hatch" "${output}" "hot-dev.sh - Foreground Pulse dev runtime escape hatch"
  assert_contains "hot-dev usage points to managed runtime first" "${output}" "npm run dev                             # Canonical managed dev runtime"
  assert_contains "hot-dev usage reserves direct script for manual troubleshooting" "${output}" "./scripts/hot-dev.sh                    # Foreground/manual runtime troubleshooting"
  assert_not_contains "hot-dev usage no longer claims standard dev mode" "${output}" "Standard dev mode"
}

test_hot_dev_script_ignores_test_only_backend_churn() {
  local output
  output="$(cat "${HOT_DEV}")"

  assert_contains "hot-dev watcher ignores Go test files" "${output}" '[[ "${changed_file}" == *_test.go ]] && return 1'
  assert_contains "hot-dev watcher routes rebuild decisions through shared helper" "${output}" 'should_rebuild_backend_for_change "$changed_file"'
  assert_contains "hot-dev watcher suppresses self-build binary restart loops" "${output}" 'if manual_build_event_suppressed; then'
  assert_contains "hot-dev watcher suppresses startup self-build pulse events" "${output}" 'SELF_BUILD_IGNORE_UNTIL=$((WATCHER_READY_AT + HOT_DEV_WATCHER_STARTUP_GRACE_SECONDS + 5))'
}

test_hot_dev_bg_script_advertises_managed_entrypoint() {
  local output
  output="$(cat "${ROOT_DIR}/scripts/hot-dev-bg.sh")"

  assert_contains "hot-dev-bg usage points to managed runtime first" "${output}" "npm run dev                             # Canonical managed dev runtime"
  assert_contains "hot-dev-bg still documents direct launcher start" "${output}" "./scripts/hot-dev-bg.sh start [--takeover]"
  assert_contains "hot-dev-bg routes log guidance to managed wrapper" "${output}" "Check logs with: npm run dev:logs"
  assert_contains "hot-dev-bg routes verify guidance to managed wrapper" "${output}" "Rerun with: npm run dev:verify"
  assert_contains "hot-dev-bg routes launchd supervision guidance to managed wrapper" "${output}" "Rerun with: npm run dev"
}

test_hot_dev_bg_usage_prefers_managed_wrappers() {
  local output
  output="$("${HOT_DEV_BG}" 2>&1 || true)"

  assert_contains "hot-dev-bg usage shows managed entrypoints heading" "${output}" "Managed entrypoints:"
  assert_contains "hot-dev-bg usage advertises npm dev wrapper" "${output}" "npm run dev"
  assert_contains "hot-dev-bg usage advertises npm verify wrapper" "${output}" "npm run dev:verify"
  assert_contains "hot-dev-bg usage retains raw command list" "${output}" "Commands:"
  assert_contains "hot-dev-bg usage retains direct start subcommand" "${output}" "start [--takeover]"
}

test_integration_readme_uses_managed_backend_restart_wrapper() {
  local output
  output="$(sed -n '132,150p' "${INTEGRATION_README}")"

  assert_contains "integration readme documents managed backend restart wrapper" "${output}" "npm run dev:backend-restart"
  assert_contains "integration readme documents owner-process recovery proof" "${output}" "kills the supervised"
  assert_contains "integration readme names the owner process" "${output}" "owner process"
  assert_contains "integration readme documents recovery layout proof" "${output}" "tests/17-recovery-layout.spec.ts"
  assert_not_contains "integration readme no longer documents raw backend restart script" "${output}" "./scripts/hot-dev-bg.sh backend-restart"
}

test_clean_mock_alerts_prefers_managed_runtime() {
  local test_dir fake_bin alert_history fake_hot_dev_bg action_log output
  test_dir="$(mktemp -d)"
  temp_dirs+=("${test_dir}")
  fake_bin="${test_dir}/bin"
  mkdir -p "${fake_bin}"
  action_log="${test_dir}/actions.log"
  alert_history="${test_dir}/alert-history.json"
  fake_hot_dev_bg="${test_dir}/hot-dev-bg.sh"

  cat > "${alert_history}" <<'EOF'
[
  {
    "alert": {
      "resourceId": "mock-resource-1"
    }
  },
  {
    "alert": {
      "resourceId": "real-resource-1"
    }
  }
]
EOF

  cat > "${fake_hot_dev_bg}" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf 'hot-dev-bg %s\n' "$*" >> "${ACTION_LOG}"
case "${1:-}" in
  status)
    echo "[hot-dev-bg] Running (pid: 12345)"
    ;;
  stop)
    echo "[hot-dev-bg] Stopped"
    ;;
  *)
    echo "unexpected hot-dev-bg command: $*" >&2
    exit 1
    ;;
esac
EOF
  chmod +x "${fake_hot_dev_bg}"

  cat > "${fake_bin}/sudo" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf 'sudo %s\n' "$*" >> "${ACTION_LOG}"
exec "$@"
EOF
  chmod +x "${fake_bin}/sudo"

  cat > "${fake_bin}/systemctl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf 'systemctl %s\n' "$*" >> "${ACTION_LOG}"
exit 0
EOF
  chmod +x "${fake_bin}/systemctl"

  cat > "${fake_bin}/chown" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf 'chown %s\n' "$*" >> "${ACTION_LOG}"
exit 0
EOF
  chmod +x "${fake_bin}/chown"

  cat > "${fake_bin}/pkill" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf 'pkill %s\n' "$*" >> "${ACTION_LOG}"
exit 0
EOF
  chmod +x "${fake_bin}/pkill"

  output="$(
    PATH="${fake_bin}:$PATH" \
    ACTION_LOG="${action_log}" \
    ALERT_HISTORY="${alert_history}" \
    HOT_DEV_BG_PATH="${fake_hot_dev_bg}" \
    "${CLEAN_MOCK_ALERTS}"
  )"

  local actions
  actions="$(cat "${action_log}")"
  assert_contains "clean-mock-alerts checks managed runtime state" "${actions}" "hot-dev-bg status"
  assert_contains "clean-mock-alerts stops managed runtime first" "${actions}" "hot-dev-bg stop"
  assert_contains "clean-mock-alerts still stops compatibility services" "${actions}" "systemctl stop pulse-hot-dev"
  assert_contains "clean-mock-alerts advertises managed restart" "${output}" "npm run dev"
  assert_contains "clean-mock-alerts advertises foreground escape hatch" "${output}" "npm run dev:foreground"

  local order_output
  order_output="$(
    ACTION_LOG_PATH="${action_log}" python3 - <<'PY'
import os
from pathlib import Path

lines = Path(os.environ["ACTION_LOG_PATH"]).read_text(encoding="utf-8").splitlines()
stop_index = next(i for i, line in enumerate(lines) if line == "hot-dev-bg stop")
systemctl_index = next(i for i, line in enumerate(lines) if line == "systemctl stop pulse-hot-dev")
print(f"managed_before_systemctl={stop_index < systemctl_index}")
PY
  )"
  assert_contains "managed runtime stop precedes legacy service stop" "${order_output}" "managed_before_systemctl=True"

  local cleaned_count mock_count
  cleaned_count="$(jq 'length' "${alert_history}")"
  mock_count="$(jq '[.[] | select((.alert.resourceId // "" | contains("mock")))] | length' "${alert_history}")"
  assert_contains "clean-mock-alerts removes only mock alerts" "${cleaned_count}" "1"
  assert_contains "clean-mock-alerts leaves no mock alerts" "${mock_count}" "0"
}

test_dev_check_uses_managed_runtime_status() {
  local test_dir fake_hot_dev_bg output
  test_dir="$(mktemp -d)"
  temp_dirs+=("${test_dir}")
  fake_hot_dev_bg="${test_dir}/hot-dev-bg.sh"

  cat > "${fake_hot_dev_bg}" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
case "${1:-}" in
  status)
    echo "[hot-dev-bg] Running (pid: 4242)"
    echo "[hot-dev-bg] Browser entrypoint: http://127.0.0.1:5173"
    echo "[hot-dev-bg] Runtime summary: frontend shell is up, but the backend health endpoint is unavailable."
    ;;
  *)
    echo "unexpected command: $*" >&2
    exit 1
    ;;
esac
EOF
  chmod +x "${fake_hot_dev_bg}"

  output="$(
    PULSE_DEV_CHECK_SKIP_AUXILIARY_CHECKS=true \
    HOT_DEV_BG_PATH="${fake_hot_dev_bg}" \
    "${DEV_CHECK}"
  )"

  assert_contains "dev-check prints managed runtime heading" "${output}" "=== Managed Runtime Status ==="
  assert_contains "dev-check relays hot-dev-bg status" "${output}" "[hot-dev-bg] Running (pid: 4242)"
  assert_contains "dev-check relays runtime summary" "${output}" "frontend shell is up, but the backend health endpoint is unavailable"
  assert_contains "dev-check recommends managed restart for unhealthy runtime" "${output}" "npm run dev:restart"
}

test_backend_restart_requires_managed_runtime() {
  local frontend_port backend_port output
  local state_dir
  frontend_port="$(pick_free_port)"
  backend_port="$(pick_free_port)"
  if [[ "${backend_port}" == "${frontend_port}" ]]; then
    backend_port="$(pick_free_port)"
  fi
  state_dir="$(make_isolated_hot_dev_bg_state)"

  output="$(
    set +e
    HOT_DEV_BG_PID_FILE="${state_dir}/hot-dev-bg.pid" \
    HOT_DEV_BG_LOG_FILE="${state_dir}/hot-dev-bg.log" \
    FRONTEND_DEV_HOST=127.0.0.1 \
    FRONTEND_DEV_PORT="${frontend_port}" \
    PULSE_DEV_API_HOST=127.0.0.1 \
    PULSE_DEV_API_PORT="${backend_port}" \
    "${HOT_DEV_BG}" backend-restart 2>&1
    printf '\nexit_code=%s' "$?"
  )"

  assert_contains "backend restart refuses missing managed runtime" "${output}" "Managed hot-dev session is not running"
  assert_contains "backend restart returns failure without managed runtime" "${output}" "exit_code=1"
}

test_detects_unmanaged_listeners() {
  local frontend_port backend_port status_output start_output
  local state_dir
  frontend_port="$(pick_free_port)"
  backend_port="$(pick_free_port)"
  if [[ "${backend_port}" == "${frontend_port}" ]]; then
    backend_port="$(pick_free_port)"
  fi
  state_dir="$(make_isolated_hot_dev_bg_state)"

  start_http_server "${frontend_port}"
  start_http_server "${backend_port}"

  status_output="$(
    HOT_DEV_BG_PID_FILE="${state_dir}/hot-dev-bg.pid" \
    HOT_DEV_BG_LOG_FILE="${state_dir}/hot-dev-bg.log" \
    FRONTEND_DEV_HOST=127.0.0.1 \
    FRONTEND_DEV_PORT="${frontend_port}" \
    PULSE_DEV_API_HOST=127.0.0.1 \
    PULSE_DEV_API_PORT="${backend_port}" \
    "${HOT_DEV_BG}" status
  )"

  assert_contains "status reports unmanaged frontend listener" "${status_output}" "[hot-dev-bg] Port ${frontend_port}: unmanaged listener"
  assert_contains "status reports unmanaged backend listener" "${status_output}" "[hot-dev-bg] Port ${backend_port}: unmanaged listener"
  assert_contains "status explains unmanaged runtime ownership" "${status_output}" "[hot-dev-bg] Detected unmanaged dev listeners. hot-dev-bg is not managing the current runtime."
  assert_contains "status reports shell HTTP" "${status_output}" "[hot-dev-bg] Frontend shell HTTP: 200"
  assert_contains "status reports proxied health mismatch" "${status_output}" "[hot-dev-bg] Frontend proxy /api/health: 404"
  assert_contains "status reports backend health probe" "${status_output}" "[hot-dev-bg] Backend /api/health: 404"

  start_output="$(
    set +e
    HOT_DEV_BG_PID_FILE="${state_dir}/hot-dev-bg.pid" \
    HOT_DEV_BG_LOG_FILE="${state_dir}/hot-dev-bg.log" \
    FRONTEND_DEV_HOST=127.0.0.1 \
    FRONTEND_DEV_PORT="${frontend_port}" \
    PULSE_DEV_API_HOST=127.0.0.1 \
    PULSE_DEV_API_PORT="${backend_port}" \
    "${HOT_DEV_BG}" start 2>&1
    printf '\nexit_code=%s' "$?"
  )"

  assert_contains "start refuses unmanaged takeover by default" "${start_output}" "Refusing to take over unmanaged listeners."
  assert_contains "start recommends canonical managed entrypoint" "${start_output}" "npm run dev"
  assert_contains "start returns failure for unmanaged takeover" "${start_output}" "exit_code=1"
}

main() {
  test_cli_parses_takeover_flag
  test_verify_command_injects_managed_runtime_env
  test_default_verify_command_runs_runtime_and_layout_proofs
  test_takeover_avoids_killing_current_shell_lineage
  test_launchd_session_supervises_managed_runtime
  test_start_bg_reports_browser_entrypoint
  test_supervisor_restarts_unexpected_child_exit
  test_launchd_wrapper_uses_managed_supervisor
  test_launchd_setup_advertises_managed_runtime_controls
  test_root_package_exposes_managed_runtime_entrypoints
  test_frontend_package_exposes_managed_runtime_entrypoints
  test_makefile_routes_managed_runtime_through_npm
  test_hot_dev_script_advertises_foreground_escape_hatch
  test_hot_dev_script_ignores_test_only_backend_churn
  test_hot_dev_bg_script_advertises_managed_entrypoint
  test_hot_dev_bg_usage_prefers_managed_wrappers
  test_integration_readme_uses_managed_backend_restart_wrapper
  test_clean_mock_alerts_prefers_managed_runtime
  test_dev_check_uses_managed_runtime_status
  test_backend_restart_requires_managed_runtime
  test_status_without_runtime
  test_detects_unmanaged_listeners

  if (( failures > 0 )); then
    echo "Total failures: ${failures}" >&2
    exit 1
  fi

  echo "hot-dev-bg ownership diagnostics smoke tests passed."
}

main "$@"
