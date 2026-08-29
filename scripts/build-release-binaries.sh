#!/usr/bin/env bash

# Build the credential-free release payload that can run in parallel with
# platform-native signing. Packaging and release signing happen separately on
# a hosted runner after this payload's exact-SHA manifest is verified.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${REPO_ROOT}"

source "${SCRIPT_DIR}/release_build_targets.sh"

VERSION="${1:-$(tr -d '\n\r[:space:]' < VERSION)}"
OUTPUT_ROOT="${2:-}"
PROFILE="full"
if [[ "${3:-}" == "--profile" ]]; then
    PROFILE="${4:-}"
    if [[ -z "${PROFILE}" || -n "${5:-}" ]]; then
        echo "Usage: $0 <version> <output-directory> [--profile full|pro-packaging]" >&2
        exit 2
    fi
elif [[ -n "${3:-}" ]]; then
    echo "Usage: $0 <version> <output-directory> [--profile full|pro-packaging]" >&2
    exit 2
fi
if [[ -z "${OUTPUT_ROOT}" ]]; then
    echo "Usage: $0 <version> <output-directory> [--profile full|pro-packaging]" >&2
    exit 2
fi
case "${PROFILE}" in
    full|pro-packaging) ;;
    *)
        echo "Error: unsupported release compilation profile: ${PROFILE}" >&2
        exit 2
        ;;
esac
OUTPUT_ROOT="$(python3 -c 'import os, sys; print(os.path.abspath(sys.argv[1]))' "${OUTPUT_ROOT}")"
case "${OUTPUT_ROOT}" in
    /|"${REPO_ROOT}"|"${REPO_ROOT}/scripts")
        echo "Error: refusing unsafe release compilation output: ${OUTPUT_ROOT}" >&2
        exit 2
        ;;
esac

required_go="go1.26.7"
current_go="$(go env GOVERSION 2>/dev/null || true)"
if [[ "${current_go}" != "${required_go}" ]]; then
    echo "Error: Go toolchain must be ${required_go} (got ${current_go:-unknown})." >&2
    exit 3
fi
for command_name in go npm python3 openssl getconf pgrep; do
    command -v "${command_name}" >/dev/null 2>&1 || {
        echo "Error: required release compilation command is missing: ${command_name}" >&2
        exit 3
    }
done
for variable_name in PULSE_LICENSE_PUBLIC_KEY PULSE_UPDATE_SIGNING_PUBLIC_KEY; do
    if [[ -z "${!variable_name:-}" ]]; then
        echo "Error: ${variable_name} is required for release compilation." >&2
        exit 3
    fi
done

decoded_key_len="$(printf '%s' "${PULSE_LICENSE_PUBLIC_KEY}" | openssl base64 -d -A 2>/dev/null | wc -c | tr -d ' ')"
if [[ "${decoded_key_len}" != "32" ]]; then
    echo "Error: PULSE_LICENSE_PUBLIC_KEY must decode to 32 bytes." >&2
    exit 3
fi

SOURCE_SHA="$(git rev-parse HEAD)"
if [[ ! "${SOURCE_SHA}" =~ ^[0-9a-f]{40}$ ]]; then
    echo "Error: release compilation requires an exact Git commit." >&2
    exit 3
fi
if [[ "$(tr -d '\n\r[:space:]' < VERSION)" != "${VERSION}" ]]; then
    echo "Error: requested version ${VERSION} does not match VERSION." >&2
    exit 3
fi

PAYLOAD_DIR="${OUTPUT_ROOT}/payload"
BINARIES_DIR="${PAYLOAD_DIR}/binaries"
FRONTEND_DIR="${PAYLOAD_DIR}/frontend-dist"
MANIFEST_DIR="${OUTPUT_ROOT}/manifest"
if [[ -e "${OUTPUT_ROOT}" ]] && find "${OUTPUT_ROOT}" -mindepth 1 -print -quit | grep -q .; then
    echo "Error: release compilation output must be absent or empty: ${OUTPUT_ROOT}" >&2
    exit 2
fi
mkdir -p "${BINARIES_DIR}" "${MANIFEST_DIR}"

build_frontend() {
    echo "Building exact-SHA frontend embed prerequisite..."
    npm --prefix frontend-modern ci
    npm --prefix frontend-modern run build
    if [[ "${PROFILE}" == "full" ]]; then
        mkdir -p "${FRONTEND_DIR}"
        cp -a frontend-modern/dist/. "${FRONTEND_DIR}/"
    fi
}

frontend_log="${OUTPUT_ROOT}/frontend.log"
build_frontend >"${frontend_log}" 2>&1 &
frontend_pid=$!
if [[ "${PROFILE}" == "full" ]]; then
    echo "Building frontend bundle concurrently with the release binary matrix."
else
    echo "Using Pro packaging profile: build the required frontend embed locally; transfer public agent-side binaries only."
fi

export CGO_ENABLED=0
release_go_build_args=(-buildvcs=false -trimpath)
agent_ldflags="$(./scripts/release_ldflags.sh agent \
    --version "v${VERSION}" \
    --update-public-keys "${PULSE_UPDATE_SIGNING_PUBLIC_KEY}")"
build_time="$(date -u '+%Y-%m-%d_%H:%M:%S')"
git_commit="$(git rev-parse --short HEAD)"
server_ldflags="$(./scripts/release_ldflags.sh server \
    --version "v${VERSION}" \
    --build-time "${build_time}" \
    --git-commit "${git_commit}" \
    --license-public-key "${PULSE_LICENSE_PUBLIC_KEY}" \
    --update-public-keys "${PULSE_UPDATE_SIGNING_PUBLIC_KEY}")"

vcpus="$(getconf _NPROCESSORS_ONLN 2>/dev/null || printf '1')"
build_jobs="${PULSE_RELEASE_BUILD_JOBS:-$((vcpus / 2))}"
if (( build_jobs < 1 )); then build_jobs=1; fi
if (( build_jobs > 4 )); then build_jobs=4; fi
go_procs="${PULSE_RELEASE_GO_PROCS:-$((vcpus / build_jobs))}"
if (( go_procs < 1 )); then go_procs=1; fi
echo "Compiling release matrix with ${build_jobs} workers and GOMAXPROCS=${go_procs} per worker..."

declare -a task_components=()
declare -a task_targets=()
for target in "${PULSE_RELEASE_AGENT_TARGETS[@]}"; do
    task_components+=(agent)
    task_targets+=("${target}")
    if [[ "${PROFILE}" == "full" ]]; then
        task_components+=(mcp)
        task_targets+=("${target}")
    fi
done
for target in "${PULSE_RELEASE_AGENT_HELPER_TARGETS[@]}"; do
    task_components+=(agent-helper)
    task_targets+=("${target}")
done
for target in "${PULSE_RELEASE_AGENT_RUNNER_TARGETS[@]}"; do
    task_components+=(agent-runner)
    task_targets+=("${target}")
done
if [[ "${PROFILE}" == "full" ]]; then
    for target in "${PULSE_RELEASE_SERVER_TARGETS[@]}"; do
        task_components+=(server)
        task_targets+=("${target}")
    done
    for target in "${PULSE_RELEASE_CONTROL_PLANE_TARGETS[@]}"; do
        task_components+=(control-plane)
        task_targets+=("${target}")
    done
fi

build_one() {
    local component="$1"
    local target="$2"
    local target_env output package ldflags
    local -a target_env_parts command
    target_env="$(pulse_release_target_env "${target}")"
    output="${BINARIES_DIR}/$(pulse_release_binary_filename "${component}" "${target}")"
    case "${component}" in
        agent)
            package=./cmd/pulse-agent
            ldflags="${agent_ldflags}"
            ;;
        agent-helper)
            package=./cmd/pulse-agent-helper
			ldflags="${agent_ldflags}"
            ;;
        agent-runner)
            package=./cmd/pulse-agent-runner
            ldflags=""
            ;;
        mcp)
            package=./cmd/pulse-mcp
            ldflags=""
            ;;
        server)
            package=./cmd/pulse
            ldflags="${server_ldflags}"
            ;;
        control-plane)
            package=./cmd/pulse-control-plane
            ldflags="${server_ldflags}"
            ;;
    esac
    read -r -a target_env_parts <<<"${target_env}"
    command=(go build "${release_go_build_args[@]}")
    if [[ "${component}" == server || "${component}" == control-plane ]]; then command+=(-tags release); fi
    if [[ -n "${ldflags}" ]]; then command+=("-ldflags=${ldflags}"); fi
    command+=(-o "${output}" "${package}")
    env "${target_env_parts[@]}" GOMAXPROCS="${go_procs}" "${command[@]}"
}

declare -a active_pids=()
declare -A task_by_pid=()
terminate_tree() {
    local parent_pid="$1"
    local child_pid
    while IFS= read -r child_pid; do
        [[ -n "${child_pid}" ]] && terminate_tree "${child_pid}"
    done < <(pgrep -P "${parent_pid}" || true)
    kill -TERM "${parent_pid}" >/dev/null 2>&1 || true
}
terminate_active() {
    local pid
    for pid in "${active_pids[@]:-}"; do
        [[ -n "${pid}" ]] && terminate_tree "${pid}"
    done
    wait "${active_pids[@]:-}" >/dev/null 2>&1 || true
    if [[ -n "${frontend_pid:-}" ]]; then
        terminate_tree "${frontend_pid}"
        wait "${frontend_pid}" >/dev/null 2>&1 || true
        frontend_pid=""
    fi
}
trap terminate_active INT TERM

finish_frontend() {
    local pid status
    if [[ -z "${frontend_pid:-}" ]]; then
        return
    fi
    pid="${frontend_pid}"
    frontend_pid=""
    if wait "${pid}"; then
        status=0
    else
        status=$?
        echo "Error: frontend embed prerequisite failed." >&2
        cat "${frontend_log}" >&2
        terminate_active
        exit "${status}"
    fi
    rm -f "${frontend_log}"
    echo "Built frontend embed prerequisite."
}

next_task=0
completed_tasks=0
total_tasks="${#task_components[@]}"
while (( completed_tasks < total_tasks )); do
    while (( next_task < total_tasks && ${#active_pids[@]} < build_jobs )); do
        component="${task_components[next_task]}"
        target="${task_targets[next_task]}"
        if [[ "${component}" == server || "${component}" == control-plane ]]; then
            finish_frontend
        fi
        log_path="${OUTPUT_ROOT}/${component}-${target}.log"
        build_one "${component}" "${target}" >"${log_path}" 2>&1 &
        pid=$!
        active_pids+=("${pid}")
        task_by_pid["${pid}"]="${component}-${target}:${log_path}"
        next_task=$((next_task + 1))
    done

    completed_pid=""
    # Restrict wait -n to compilation children. The frontend build is also a
    # child of this shell, and an unrestricted wait can reap it as if it were
    # one of the matrix tasks. That advances completed_tasks early, leaves one
    # binary build unjoined, and can publish an incomplete compiled manifest.
    if wait -n -p completed_pid "${active_pids[@]}"; then
        status=0
    else
        status=$?
    fi
    task_record="${task_by_pid[${completed_pid}]:-}"
    if [[ -z "${task_record}" ]]; then
        echo "Error: completed release compilation child is not in the active task set: ${completed_pid:-unknown}." >&2
        terminate_active
        exit 4
    fi
    task_name="${task_record%%:*}"
    log_path="${task_record#*:}"
    remaining_pids=()
    for pid in "${active_pids[@]}"; do
        if [[ "${pid}" != "${completed_pid}" ]]; then remaining_pids+=("${pid}"); fi
    done
    active_pids=("${remaining_pids[@]}")
    unset "task_by_pid[${completed_pid}]"
    if (( status != 0 )); then
        echo "Error: release compilation task failed: ${task_name}" >&2
        [[ -f "${log_path}" ]] && cat "${log_path}" >&2
        terminate_active
        exit "${status}"
    fi
    rm -f "${log_path}"
    completed_tasks=$((completed_tasks + 1))
    echo "Compiled ${task_name} (${completed_tasks}/${total_tasks})."
done

finish_frontend
trap - INT TERM

python3 scripts/release_candidate_manifest.py create \
    --release-dir "${PAYLOAD_DIR}" \
    --version "${VERSION}" \
    --source-sha "${SOURCE_SHA}" \
    --output "${MANIFEST_DIR}/release-compiled.json"

echo "Credential-free release compilation complete: ${OUTPUT_ROOT}"
