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
PACKAGE_LOCK_JSON="${ROOT_DIR}/package-lock.json"
FRONTEND_PACKAGE_JSON="${ROOT_DIR}/frontend-modern/package.json"
FRONTEND_PACKAGE_LOCK_JSON="${ROOT_DIR}/frontend-modern/package-lock.json"
FRONTEND_VITE_CONFIG="${ROOT_DIR}/frontend-modern/vite.config.ts"
GO_MOD="${ROOT_DIR}/go.mod"
GO_SUM="${ROOT_DIR}/go.sum"
DEV_LAUNCHD_WRAPPER="${ROOT_DIR}/scripts/dev-launchd-wrapper.sh"
DEV_LAUNCHD_SETUP="${ROOT_DIR}/scripts/dev-launchd-setup.sh"
INTEGRATION_README="${ROOT_DIR}/tests/integration/README.md"
INTEGRATION_EVAL_SCENARIOS="${ROOT_DIR}/tests/integration/evals/scenarios.json"
DEPLOYMENT_INSTALLABILITY_CONTRACT="${ROOT_DIR}/docs/release-control/v6/internal/subsystems/deployment-installability.md"
SUBSYSTEM_REGISTRY="${ROOT_DIR}/docs/release-control/v6/internal/subsystems/registry.json"

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
print(f\"lock={os.environ.get('\''HOT_DEV_VERIFY_LOCK_FILE'\'')}\")
PY"
      run_verify_proof_command
    '
  )"

  assert_contains "verify proof forces managed hot-dev mode" "${output}" "mode=1"
  assert_contains "verify proof defaults username" "${output}" "username=admin"
  assert_contains "verify proof defaults password" "${output}" "password=admin"
  assert_contains "verify proof passes the managed runtime lock path" "${output}" "lock=${ROOT_DIR}/tmp/hot-dev.verify.lock"
}

test_default_verify_command_runs_runtime_and_layout_proofs() {
  local test_dir fake_bin output
  test_dir="$(mktemp -d)"
  temp_dirs+=("${test_dir}")
  fake_bin="${test_dir}/bin"
  mkdir -p "${fake_bin}"

  cat > "${fake_bin}/node" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf 'cwd=%s\n' "$PWD"
printf 'args=%s\n' "$*"
EOF
  chmod +x "${fake_bin}/node"

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
  assert_contains "default verify proof runs through the dedicated Playwright harness" "${output}" "args=./scripts/run-playwright.mjs"
  assert_contains "default verify proof includes dev runtime recovery spec" "${output}" "tests/16-dev-runtime-recovery.spec.ts"
  assert_contains "default verify proof includes recovery layout spec" "${output}" "tests/17-recovery-layout.spec.ts"
  assert_contains "default verify proof includes patrol runtime-state spec" "${output}" "tests/18-patrol-runtime-state.spec.ts"
  assert_contains "default verify proof keeps chromium project pin" "${output}" "--project=chromium"
}

test_verify_bg_holds_runtime_lock_for_proof_duration() {
  local output
  output="$(
    HOT_DEV_BG_PATH="${HOT_DEV_BG}" \
    bash -lc '
      source "${HOT_DEV_BG_PATH}"
      set +e
      HOT_DEV_VERIFY_LOCK="$(mktemp)"
      rm -f "${HOT_DEV_VERIFY_LOCK}"
      require_python(){ :; }
      is_running(){ return 0; }
      managed_session_pid(){ printf "4242\n"; }
      has_unmanaged_listeners(){ return 1; }
      runtime_healthy(){ return 0; }
      run_verify_proof_command() {
        if [[ -f "${HOT_DEV_VERIFY_LOCK}" ]]; then
          printf "lock_present=yes\n"
          cat "${HOT_DEV_VERIFY_LOCK}"
        else
          printf "lock_present=no\n"
        fi
        return 7
      }

      verify_bg false
      status=$?
      printf "verify_status=%s\n" "${status}"
      if [[ -f "${HOT_DEV_VERIFY_LOCK}" ]]; then
        printf "lock_after=yes\n"
      else
        printf "lock_after=no\n"
      fi
    '
  )"

  assert_contains "verify holds the runtime lock while proofs run" "${output}" "lock_present=yes"
  assert_contains "verify lock records the owning process id" "${output}" "pid="
  assert_contains "verify returns the underlying proof failure code" "${output}" "verify_status=7"
  assert_contains "verify clears the runtime lock after proofs finish" "${output}" "lock_after=no"
}

test_managed_dev_runtime_restarts_existing_session_for_verification() {
  local output
  output="$(
    ROOT_DIR="${ROOT_DIR}" \
    node --input-type=module <<'EOF'
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';

const rootDir = process.env.ROOT_DIR;
const modulePath = path.join(rootDir, 'tests', 'integration', 'scripts', 'managed-dev-runtime.mjs');
const { shouldRestartManagedDevRuntimeForVerification } = await import(`file://${modulePath}`);

const tempDir = await fs.mkdtemp(path.join(os.tmpdir(), 'pulse-managed-dev-verify-'));
const lockPath = path.join(tempDir, 'hot-dev.verify.lock');
await fs.writeFile(lockPath, `pid=${process.pid}\ncreated_at=2026-03-28T23:00:00Z\n`, 'utf8');

console.log(
  `running=${shouldRestartManagedDevRuntimeForVerification({ env: { HOT_DEV_VERIFY_LOCK_FILE: lockPath }, wasRunning: true })}`,
);
console.log(
  `stopped=${shouldRestartManagedDevRuntimeForVerification({ env: { HOT_DEV_VERIFY_LOCK_FILE: lockPath }, wasRunning: false })}`,
);
EOF
  )"

  assert_contains "managed dev runtime restarts existing sessions during verification" "${output}" "running=true"
  assert_contains "managed dev runtime does not restart absent prior session" "${output}" "stopped=false"
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

test_stop_hot_dev_child_targets_child_process_group() {
  local output
  output="$(
    HOT_DEV_BG_PATH="${HOT_DEV_BG}" \
    bash -lc '
      source "${HOT_DEV_BG_PATH}"
      kill() {
        if [[ "${1:-}" == "-0" ]]; then
          return 1
        fi
        printf "kill %s\n" "$*"
      }
      stop_hot_dev_child 4242
    '
  )"

  assert_contains "stop_hot_dev_child terminates the isolated child process group" "${output}" "kill -TERM -4242"
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

  assert_contains "frontend package delegates managed dev start to repo-root wrapper" "${output}" "dev=npm --prefix .. run dev"
  assert_contains "frontend package delegates managed dev logs to repo-root wrapper" "${output}" "dev:logs=npm --prefix .. run dev:logs"
  assert_contains "frontend package delegates managed dev restart to repo-root wrapper" "${output}" "dev:restart=npm --prefix .. run dev:restart"
  assert_contains "frontend package delegates managed backend restart to repo-root wrapper" "${output}" "dev:backend-restart=npm --prefix .. run dev:backend-restart"
  assert_contains "frontend package delegates managed dev status to repo-root wrapper" "${output}" "dev:status=npm --prefix .. run dev:status"
  assert_contains "frontend package delegates managed dev stop to repo-root wrapper" "${output}" "dev:stop=npm --prefix .. run dev:stop"
  assert_contains "frontend package delegates managed dev verify to repo-root wrapper" "${output}" "dev:verify=npm --prefix .. run dev:verify"
  assert_contains "frontend package delegates foreground escape hatch to repo-root wrapper" "${output}" "dev:foreground=npm --prefix .. run dev:foreground"
  assert_contains "frontend package keeps explicit frontend-only escape hatch" "${output}" "dev:frontend-only=vite"
}

test_dev_runtime_dependency_manifests_are_governed() {
  local output
  output="$(
    REGISTRY_PATH="${SUBSYSTEM_REGISTRY}" \
    CONTRACT_PATH="${DEPLOYMENT_INSTALLABILITY_CONTRACT}" \
    PACKAGE_JSON_PATH="${PACKAGE_JSON}" \
    PACKAGE_LOCK_JSON_PATH="${PACKAGE_LOCK_JSON}" \
    FRONTEND_PACKAGE_JSON_PATH="${FRONTEND_PACKAGE_JSON}" \
    FRONTEND_PACKAGE_LOCK_JSON_PATH="${FRONTEND_PACKAGE_LOCK_JSON}" \
    FRONTEND_VITE_CONFIG_PATH="${FRONTEND_VITE_CONFIG}" \
    GO_MOD_PATH="${GO_MOD}" \
    GO_SUM_PATH="${GO_SUM}" \
    python3 - <<'PY'
import json
import os
from pathlib import Path

registry = json.loads(Path(os.environ["REGISTRY_PATH"]).read_text(encoding="utf-8"))
contract = Path(os.environ["CONTRACT_PATH"]).read_text(encoding="utf-8")
paths = [
    "package.json",
    "package-lock.json",
    "frontend-modern/package.json",
    "frontend-modern/package-lock.json",
    "frontend-modern/vite.config.ts",
    "go.mod",
    "go.sum",
]
path_env = {
    "package.json": "PACKAGE_JSON_PATH",
    "package-lock.json": "PACKAGE_LOCK_JSON_PATH",
    "frontend-modern/package.json": "FRONTEND_PACKAGE_JSON_PATH",
    "frontend-modern/package-lock.json": "FRONTEND_PACKAGE_LOCK_JSON_PATH",
    "frontend-modern/vite.config.ts": "FRONTEND_VITE_CONFIG_PATH",
    "go.mod": "GO_MOD_PATH",
    "go.sum": "GO_SUM_PATH",
}

subsystem = next(item for item in registry["subsystems"] if item["id"] == "deployment-installability")
policy = next(
    item
    for item in subsystem["verification"]["path_policies"]
    if item["id"] == "dev-runtime-orchestration"
)
owned = set(subsystem["owned_files"])
matched = set(policy["match_files"])

for path in paths:
    exists = Path(os.environ[path_env[path]]).is_file()
    print(f"{path}:exists={exists}")
    print(f"{path}:owned={path in owned}")
    print(f"{path}:policy={path in matched}")
    print(f"{path}:contract={f'`{path}`' in contract}")
PY
  )"

  for manifest_path in \
    "package.json" \
    "package-lock.json" \
    "frontend-modern/package.json" \
    "frontend-modern/package-lock.json" \
    "frontend-modern/vite.config.ts" \
    "go.mod" \
    "go.sum"; do
    assert_contains "dev runtime manifest exists: ${manifest_path}" "${output}" "${manifest_path}:exists=True"
    assert_contains "dev runtime manifest owned: ${manifest_path}" "${output}" "${manifest_path}:owned=True"
    assert_contains "dev runtime manifest has proof policy: ${manifest_path}" "${output}" "${manifest_path}:policy=True"
    assert_contains "dev runtime manifest is in contract: ${manifest_path}" "${output}" "${manifest_path}:contract=True"
  done
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
  assert_contains "hot-dev watcher rebuilds for embedded frontend demo dist changes" "${output}" '[[ "${changed_file}" == "${EMBEDDED_FRONTEND_DIST_DIR}" ]] || [[ "${changed_file}" == "${EMBEDDED_FRONTEND_DIST_DIR}/"* ]]'
  assert_contains "hot-dev fswatch covers the embedded frontend parent dir" "${output}" '"${ROOT_DIR}/pulse" "${HOT_DEV_RESTART_SENTINEL}" "${EMBEDDED_FRONTEND_DIR}"'
  assert_contains "hot-dev fswatch only treats the pulse binary path as a manual build trigger" "${output}" 'elif [[ "$changed_file" == "${ROOT_DIR}/pulse" ]]; then'
  assert_contains "hot-dev watcher declares a managed build lock path" "${output}" 'HOT_DEV_BUILD_LOCK="${HOT_DEV_BUILD_LOCK_FILE:-${ROOT_DIR}/tmp/hot-dev.build.lock}"'
  assert_contains "hot-dev watcher suppresses manual binary restarts while a managed build is active" "${output}" 'if build_lock_active; then'
  assert_contains "hot-dev watcher suppresses self-build binary restart loops" "${output}" 'if manual_build_event_suppressed; then'
  assert_contains "hot-dev watcher suppresses startup self-build pulse events" "${output}" 'SELF_BUILD_IGNORE_UNTIL=$((WATCHER_READY_AT + HOT_DEV_WATCHER_STARTUP_GRACE_SECONDS + 5))'
  assert_contains "hot-dev watcher seeds the startup pulse marker" "${output}" 'LAST_PULSE_BINARY_MARKER="$(file_event_marker "${ROOT_DIR}/pulse" || true)"'
  assert_contains "hot-dev watcher suppresses source churn during managed verification" "${output}" 'verify_lock_active'
}

test_hot_dev_script_marks_managed_rebuild_output_before_build() {
  local rebuild_block mark_line build_line
  rebuild_block="$(
    awk '
      /rebuild_backend\(\)/ { capture=1 }
      capture { print }
      capture && /^    if command -v inotifywait/ { exit }
    ' "${HOT_DEV}"
  )"

  assert_contains "hot-dev rebuild path suppresses self-build binary churn" "${rebuild_block}" 'mark_self_build_output'
  assert_contains "hot-dev rebuild path raises the managed build lock" "${rebuild_block}" 'set_build_lock'
  assert_contains "hot-dev rebuild path clears the managed build lock" "${rebuild_block}" 'clear_build_lock'

  mark_line="$(printf '%s\n' "${rebuild_block}" | awk '/mark_self_build_output/ { print NR; exit }')"
  build_line="$(printf '%s\n' "${rebuild_block}" | awk '/if build_backend_binary; then/ { print NR; exit }')"

  if [[ -n "${mark_line}" && -n "${build_line}" && "${mark_line}" -lt "${build_line}" ]]; then
    echo "[PASS] hot-dev rebuild path marks self-build output before compiling pulse"
  else
    echo "[FAIL] hot-dev rebuild path marks self-build output before compiling pulse" >&2
    ((failures++))
  fi
}

test_hot_dev_bg_script_advertises_managed_entrypoint() {
  local output
  output="$(cat "${ROOT_DIR}/scripts/hot-dev-bg.sh")"

  assert_contains "hot-dev-bg usage points to managed runtime first" "${output}" "npm run dev                             # Canonical managed dev runtime"
  assert_contains "hot-dev-bg documents direct launcher as troubleshooting only" "${output}" "./scripts/hot-dev-bg.sh <command>       # Direct troubleshooting only"
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
  assert_contains "hot-dev-bg usage labels raw commands as troubleshooting-only" "${output}" "Direct troubleshooting subcommands:"
  assert_contains "hot-dev-bg usage retains direct start subcommand" "${output}" "start [--takeover]"
}

test_integration_readme_uses_managed_backend_restart_wrapper() {
  local output
  output="$(
    awk '
      /^### Run Against The Managed Hot-Dev Browser Runtime$/ { capture=1 }
      capture { print }
      /^For deterministic paid-feature runs against an existing instance, provide one of:$/ {
        exit
      }
    ' "${INTEGRATION_README}"
  )"

  assert_contains "integration readme documents managed backend restart wrapper" "${output}" "npm run dev:backend-restart"
  assert_contains "integration readme documents owner-process recovery proof" "${output}" "kills the supervised"
  assert_contains "integration readme names the owner process" "${output}" "owner process"
  assert_contains "integration readme documents recovery layout proof" "${output}" "tests/17-recovery-layout.spec.ts"
  assert_contains "integration readme documents patrol runtime-state proof" "${output}" "tests/18-patrol-runtime-state.spec.ts"
  assert_contains "integration readme documents explicit browser override precedence" "${output}" "PLAYWRIGHT_BASE_URL"
  assert_contains "integration readme documents backend base split" "${output}" "backend-oriented base"
  assert_contains "integration readme documents split browser/backend example" "${output}" "PLAYWRIGHT_BASE_URL='http://127.0.0.1:4174'"
  assert_not_contains "integration readme no longer documents raw backend restart script" "${output}" "./scripts/hot-dev-bg.sh backend-restart"
}

test_integration_readme_documents_trial_retry_burst_contract() {
  local output
  output="$(
    awk '
      /^### Snapshot-Clean Proxmox LXC Trial SAT$/ { capture=1 }
      capture { print }
      /^### Run Theme Visual Regression Suite$/ {
        exit
      }
    ' "${INTEGRATION_README}"
  )"

  assert_contains "integration readme names trial-start route" "${output}" "POST /api/license/trial/start"
  assert_contains "integration readme names hosted-signup redirect code" "${output}" "409 trial_signup_required"
  assert_contains "integration readme names rate-limited code" "${output}" "429 trial_rate_limited"
  assert_contains "integration readme documents hosted-signup retry burst" "${output}" "retry-burst"
  assert_contains "integration readme documents retry contract wording" "${output}" "contract:"
  assert_contains "integration readme keeps hosted-signup wording" "${output}" "hosted-signup"
  assert_contains "integration readme documents retry burst exhaustion" "${output}" "retry burst is exhausted"
  assert_contains "integration readme documents retry-after backoff" "${output}" "Retry-After"
  assert_contains "integration readme references hosted trial probe script" "${output}" "tests/integration/scripts/trial-signup-contract.sh"
  assert_contains "integration readme references pulse pro retry-after ui proof" "${output}" "tests/58-self-hosted-trial-rate-limit-ui.spec.ts"
}

test_integration_eval_scenario_documents_trial_retry_burst_contract() {
  local output
  output="$(sed -n '24,34p' "${INTEGRATION_EVAL_SCENARIOS}")"

  assert_contains "integration eval scenario names trial-start route" "${output}" "POST /api/license/trial/start"
  assert_contains "integration eval scenario names hosted-signup redirect code" "${output}" "409 trial_signup_required"
  assert_contains "integration eval scenario names rate-limited code" "${output}" "429 trial_rate_limited"
  assert_contains "integration eval scenario names retry-after metadata" "${output}" "Retry-After"
}

test_integration_quick_start_distinguishes_embedded_frontend_from_hot_dev() {
  local output
  output="$(sed -n '82,102p' "${ROOT_DIR}/tests/integration/QUICK_START.md")"

  assert_contains "integration quick start labels 7655 as embedded frontend" "${output}" "Pulse test UI (embedded frontend)"
  assert_contains "integration quick start points local managed dev runtime to 5173" "${output}" "http://127.0.0.1:5173"
  assert_contains "integration quick start names hot-dev browser shell" "${output}" "hot-dev browser shell"
}

test_acceptance_doc_distinguishes_embedded_frontend_from_hot_dev() {
  local output
  output="$(sed -n '412,420p' "${ROOT_DIR}/docs/architecture/v6-acceptance-tests.md")"

  assert_contains "acceptance doc keeps embedded frontend wording" "${output}" "against the embedded frontend at \`http://localhost:7655\`"
  assert_contains "acceptance doc distinguishes managed hot-dev shell" "${output}" "managed hot-dev browser shell on \`http://127.0.0.1:5173\`"
}

test_playwright_defaults_prefer_managed_hot_dev_runtime() {
  local output
  output="$(cat "${ROOT_DIR}/tests/integration/playwright.config.ts")"

  assert_contains "playwright config imports shared browser default helper" "${output}" "import { preferredBrowserBaseURL } from './tests/runtime-defaults';"
  assert_contains "playwright config delegates base url to shared helper" "${output}" "baseURL: preferredBrowserBaseURL(),"
}

test_root_playwright_wrapper_prefers_managed_browser_runtime() {
  local test_dir runtime_state output
  test_dir="$(mktemp -d)"
  temp_dirs+=("${test_dir}")
  runtime_state="${test_dir}/runtime-state.json"

  cat > "${runtime_state}" <<'EOF'
{
  "baseURL": "http://127.0.0.1:5173"
}
EOF

  output="$(
    cd "${ROOT_DIR}" && \
      PULSE_E2E_RUNTIME_STATE_PATH="${runtime_state}" \
      npx tsx --eval "import config from './playwright.config.ts'; console.log(config.use?.baseURL || '');"
  )"

  assert_contains "root playwright wrapper prefers runtime-state browser url" "${output}" "http://127.0.0.1:5173"
  assert_not_contains "root playwright wrapper no longer falls back to embedded frontend by default" "${output}" "http://localhost:7655"
}

test_integration_helpers_prefer_managed_hot_dev_runtime() {
  local output
  output="$(cat "${ROOT_DIR}/tests/integration/tests/helpers.ts")"

  assert_contains "integration helpers import preferred browser helper" "${output}" "preferredBrowserBaseURL"
  assert_contains "integration helpers import shared route helper" "${output}" "preferredPlaywrightRouteBaseURL"
  assert_contains "integration helpers import shared browser defaults" "${output}" "runtime-defaults"
  assert_contains "integration helpers delegate browser context base url to shared helper" "${output}" "baseURL: preferredBrowserBaseURL(),"
  assert_contains "integration helpers delegate api request base url to shared route helper" "${output}" "const baseURL = preferredPlaywrightRouteBaseURL()"
}

test_integration_runtime_defaults_centralize_managed_browser_detection() {
  local output
  output="$(cat "${ROOT_DIR}/tests/integration/tests/runtime-defaults.ts")"

  assert_contains "runtime defaults declare managed hot-dev pid path" "${output}" "managedHotDevPidPath"
  assert_contains "runtime defaults expose runtime-state reader" "${output}" "export const readRuntimeState ="
  assert_contains "runtime defaults expose managed browser detection" "${output}" "export const managedDevBrowserBaseURL ="
  assert_contains "runtime defaults expose preferred browser base url" "${output}" "export const preferredBrowserBaseURL ="
  assert_contains "runtime defaults expose normalized Playwright route helper" "${output}" "export const preferredPlaywrightRouteBaseURL ="
}

test_integration_runtime_defaults_prefer_explicit_browser_override() {
  local output
  output="$(
    ROOT_DIR="${ROOT_DIR}" \
    node --input-type=module <<'EOF'
import path from 'node:path';

const rootDir = process.env.ROOT_DIR;
const modulePath = path.join(rootDir, 'tests', 'integration', 'tests', 'runtime-defaults.ts');
const { preferredBrowserBaseURL } = await import(`file://${modulePath}`);

console.log(
  preferredBrowserBaseURL({
    PLAYWRIGHT_BASE_URL: 'http://127.0.0.1:4174',
    PULSE_BASE_URL: 'http://127.0.0.1:7655',
  }),
);
EOF
  )"

  assert_contains "runtime defaults prefer explicit Playwright browser override" "${output}" "http://127.0.0.1:4174"
  assert_not_contains "runtime defaults do not reuse backend base as browser target when override exists" "${output}" "http://127.0.0.1:7655"
}

test_integration_runtime_defaults_honor_repo_root_override() {
  local output
  output="$(
    ROOT_DIR="${ROOT_DIR}" \
    node --input-type=module <<'EOF'
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';

const rootDir = process.env.ROOT_DIR;
const modulePath = path.join(rootDir, 'tests', 'integration', 'tests', 'runtime-defaults.ts');
const { managedDevBrowserBaseURL, runtimeStatePath } = await import(`file://${modulePath}`);

const repoRoot = await fs.mkdtemp(path.join(os.tmpdir(), 'pulse-runtime-defaults-'));
await fs.mkdir(path.join(repoRoot, 'tmp'), { recursive: true });
await fs.writeFile(path.join(repoRoot, 'tmp', 'hot-dev.bg.pid'), `${process.pid}\n`, 'utf8');

try {
  console.log(
    `stateMatches=${runtimeStatePath({ PULSE_E2E_REPO_ROOT: repoRoot }) === path.join(repoRoot, 'tmp', 'e2e-runtime-state.json')}`,
  );
  console.log(
    `managed=${managedDevBrowserBaseURL({
      PULSE_E2E_REPO_ROOT: repoRoot,
      FRONTEND_DEV_HOST: '127.0.0.1',
      FRONTEND_DEV_PORT: '4174',
    })}`,
  );
} finally {
  await fs.rm(repoRoot, { recursive: true, force: true });
}
EOF
  )"

  assert_contains "runtime defaults resolve runtime-state path from overridden repo root" "${output}" "stateMatches=true"
  assert_contains "runtime defaults resolve managed dev browser url from overridden repo root" "${output}" "managed=http://127.0.0.1:4174"
}

test_cloud_and_commercial_specs_use_shared_route_defaults() {
  local output

  output="$(cat "${ROOT_DIR}/tests/integration/tests/08-cloud-hosting.spec.ts")"
  assert_contains "cloud hosting spec imports shared route helper" "${output}" "preferredPlaywrightRouteBaseURL"
  assert_contains "cloud hosting spec uses cloud override through shared route helper" "${output}" "process.env.PULSE_CLOUD_BASE_URL"
  assert_not_contains "cloud hosting spec no longer duplicates Pulse-vs-Playwright precedence" "${output}" "process.env.PULSE_BASE_URL ||"

  output="$(cat "${ROOT_DIR}/tests/integration/tests/09-cloud-billing-lifecycle.spec.ts")"
  assert_contains "cloud billing spec imports shared route helper" "${output}" "preferredPlaywrightRouteBaseURL"
  assert_contains "cloud billing spec uses cloud override through shared route helper" "${output}" "process.env.PULSE_CLOUD_BASE_URL"
  assert_not_contains "cloud billing spec no longer duplicates Pulse-vs-Playwright precedence" "${output}" "process.env.PULSE_BASE_URL ||"

  output="$(cat "${ROOT_DIR}/tests/integration/tests/14-commercial-cancellation-reactivation.spec.ts")"
  assert_contains "commercial cancellation spec imports shared route helper" "${output}" "preferredPlaywrightRouteBaseURL"
  assert_contains "commercial cancellation spec uses commercial override through shared route helper" "${output}" "process.env.PULSE_COMMERCIAL_BASE_URL"
  assert_not_contains "commercial cancellation spec no longer duplicates Pulse-vs-Playwright precedence" "${output}" "process.env.PULSE_BASE_URL ||"
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
  test_verify_bg_holds_runtime_lock_for_proof_duration
  test_managed_dev_runtime_restarts_existing_session_for_verification
  test_takeover_avoids_killing_current_shell_lineage
  test_launchd_session_supervises_managed_runtime
  test_start_bg_reports_browser_entrypoint
  test_supervisor_restarts_unexpected_child_exit
  test_stop_hot_dev_child_targets_child_process_group
  test_launchd_wrapper_uses_managed_supervisor
  test_launchd_setup_advertises_managed_runtime_controls
  test_root_package_exposes_managed_runtime_entrypoints
  test_frontend_package_exposes_managed_runtime_entrypoints
  test_dev_runtime_dependency_manifests_are_governed
  test_makefile_routes_managed_runtime_through_npm
  test_hot_dev_script_advertises_foreground_escape_hatch
  test_hot_dev_script_ignores_test_only_backend_churn
  test_hot_dev_bg_script_advertises_managed_entrypoint
  test_hot_dev_bg_usage_prefers_managed_wrappers
  test_integration_readme_uses_managed_backend_restart_wrapper
  test_integration_readme_documents_trial_retry_burst_contract
  test_integration_quick_start_distinguishes_embedded_frontend_from_hot_dev
  test_acceptance_doc_distinguishes_embedded_frontend_from_hot_dev
  test_playwright_defaults_prefer_managed_hot_dev_runtime
  test_root_playwright_wrapper_prefers_managed_browser_runtime
  test_integration_helpers_prefer_managed_hot_dev_runtime
  test_integration_runtime_defaults_centralize_managed_browser_detection
  test_integration_runtime_defaults_prefer_explicit_browser_override
  test_integration_runtime_defaults_honor_repo_root_override
  test_cloud_and_commercial_specs_use_shared_route_defaults
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
