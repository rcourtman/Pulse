#!/usr/bin/env bash
# compare_release_memory.sh — run two Pulse source trees against the same
# estate and compare their steady-state memory (#1803).
#
# Usage: bash scripts/memory/compare_release_memory.sh <baseline-tree> <candidate-tree> <out-dir> [stable-tag baseline-sha candidate-sha]
#
# Each tree is built from source with a placeholder frontend (release builds
# refuse mock fixtures), started with telemetry off and no licence server, and
# sampled every PULSE_MEMORY_INTERVAL seconds for PULSE_MEMORY_DURATION seconds.
# Samples split resident memory into the Go heap mapping, unnamed anonymous
# memory (where the modernc SQLite driver allocates) and file pages. At the end
# each run's heap profile is saved, and summarize_release_memory.py writes the
# comparison and exits 1 when the candidate grew past the tolerance.
# The mapping breakdown is optional; VmRSS is read separately for the verdict.
# Each sample also retains Linux's process RSS high-water mark (VmHWM), which
# can include peaks between samples. This is not cgroup memory usage.
#
# Each sample also records the process's CPU ticks (/proc/<pid>/stat) and its
# storage and write-syscall byte counters (/proc/<pid>/io), so the summary can
# report steady-state CPU and disk-write rates beside memory (#2199, #1966).
#
# PULSE_FOOTPRINT_ESTATE picks the estate:
#   mock    (default) the fixed small mock estate. Mock mode keeps its own
#           fixtures in the heap, so a fixture change shows up here too. Read
#           the heap profile diff before blaming production code.
#   agents  a real, non-mock Pulse with authentication configured, fed by
#           agent_load.py: synthetic Unified Agents posting reports with their
#           own install tokens, some with Docker, and one connected dashboard.
#           Mock mode drops pushed agent reports, so only this estate reaches
#           the per-report registry refresh and broadcast cost (#2199). The
#           driver fails the run when reports were not accepted or the dashboard
#           did not receive an initial state and a subsequent state update.
#   proxmox a real, non-mock Pulse with authentication configured, fed by
#           proxmox_load.py: a synthetic 3-node Proxmox VE cluster (35 guests),
#           2 Proxmox Backup Server instances (~5,250 snapshots total), and one
#           connected dashboard (#1803). Exercises PVE/PBS polling, recovery.db
#           ingestion, posture refreshes, and backup state cloning.

set -euo pipefail

if (( $# != 3 && $# != 6 )); then
  echo "Usage: $0 baseline-tree candidate-tree out-dir [stable-tag baseline-sha candidate-sha]" >&2
  exit 2
fi
BASELINE_DIR="${1:?baseline tree}"
CANDIDATE_DIR="${2:?candidate tree}"
OUT_DIR="${3:?output directory}"
DURATION="${PULSE_MEMORY_DURATION:-900}"
INTERVAL="${PULSE_MEMORY_INTERVAL:-15}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ESTATE="${PULSE_FOOTPRINT_ESTATE:-mock}"

# The hosted watch passes the pair selected before any estate job starts.
# Verify the actual checkouts before creating build inputs or reporting a pair.
# Local, unbound comparisons still accept the original three arguments.
source_binding=()
check_source() {
  local label=$1 tree=$2 expected=$3 actual
  actual="$(git -C "$tree" rev-parse --verify HEAD)" || return 1
  if [[ "$actual" != "$expected" ]]; then
    printf '%s checkout %s does not match expected source %s\n' "$label" "$actual" "$expected" >&2
    return 1
  fi
  if ! git -C "$tree" diff --quiet HEAD --; then
    printf '%s checkout is changed or could not be verified\n' "$label" >&2
    return 1
  fi
}
if (( $# == 6 )); then
  if [[ ! "$4" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ || ! "$5" =~ ^[0-9a-f]{40}$ || ! "$6" =~ ^[0-9a-f]{40}$ ]]; then
    echo "Invalid stable tag or comparison source SHA" >&2
    exit 2
  fi
  check_source baseline "$BASELINE_DIR" "$5" || exit 2
  check_source candidate "$CANDIDATE_DIR" "$6" || exit 2
  source_binding=("$4" "$5" "$6")
fi

if [[ ! "${DURATION}" =~ ^[1-9][0-9]*$ || ! "${INTERVAL}" =~ ^[1-9][0-9]*$ ]]; then
  echo "Memory sampling duration and interval must be positive whole seconds" >&2
  exit 2
fi

case "${ESTATE}" in
  mock)
    ESTATE_ENV=(
      PULSE_MOCK_MODE=true PULSE_MOCK_NODES=2 PULSE_MOCK_VMS_PER_NODE=6 PULSE_MOCK_LXCS_PER_NODE=6
      PULSE_MOCK_DOCKER_HOSTS=1 PULSE_MOCK_DOCKER_CONTAINERS=10 PULSE_MOCK_GENERIC_HOSTS=1
      PULSE_MOCK_K8S_CLUSTERS=0
    )
    ;;
  agents)
    # Any configured credential turns authentication on, so reports carry and
    # check real install tokens. Nobody logs in: the driver mints tokens
    # through the development admin bypass. This is the fixed development
    # hash from scripts/lib/hot-dev-auth.sh.
    ESTATE_ENV=(
      BIND_ADDRESS=127.0.0.1 PULSE_AUTH_USER=footprint
      'PULSE_AUTH_PASS=$2a$12$J/Vu6FlBUJDTK.VAkysjB.AnvFOcijDbETyumhCB.nJVes5gvpiI6'
    )
    ;;
  proxmox)
    ESTATE_ENV=(
      BIND_ADDRESS=127.0.0.1 PULSE_AUTH_USER=footprint
      'PULSE_AUTH_PASS=$2a$12$J/Vu6FlBUJDTK.VAkysjB.AnvFOcijDbETyumhCB.nJVes5gvpiI6'
      "POLLING_INTERVAL=${PULSE_PROXMOX_POLL_INTERVAL:-5s}"
      "BACKUP_POLLING_INTERVAL=${PULSE_PROXMOX_BACKUP_POLL_INTERVAL:-30s}"
    )
    ;;
  *)
    echo "Unknown PULSE_FOOTPRINT_ESTATE '${ESTATE}'; use mock, agents, or proxmox" >&2
    exit 2
    ;;
esac

mkdir -p "${OUT_DIR}"
OUT_DIR="$(cd "${OUT_DIR}" && pwd)"
getconf CLK_TCK > "${OUT_DIR}/clk_tck"
printf '%s\n' "${ESTATE}" > "${OUT_DIR}/estate"

build_tree() {
  local label=$1 tree=$2
  local dist="${tree}/internal/api/frontend-modern/dist"
  mkdir -p "${dist}"
  [[ -e "${dist}/index.html" ]] || printf '<!doctype html><title>pulse</title>\n' > "${dist}/index.html"
  (cd "${tree}" && CGO_ENABLED=0 go build -o "${OUT_DIR}/pulse-${label}" ./cmd/pulse)
}

# Resident KiB by mapping class: Go heap, other Go runtime, unnamed anonymous,
# file-backed. Go 1.25+ names its own anonymous mappings.
classify_smaps() {
  awk '
    /^[0-9a-f]+-[0-9a-f]+ / {
      name = ""
      for (i = 6; i <= NF; i++) name = name (i > 6 ? " " : "") $i
      if (name == "[anon: Go: heap]") cls = "go_heap"
      else if (name ~ /^\[anon: Go:/) cls = "go_other"
      else if (name == "") cls = "anon_other"
      else if (name ~ /^\//) cls = "file"
      else cls = "special"
      next
    }
    /^Rss:/ && cls != "" && $2 ~ /^[0-9]+$/ && $3 == "kB" {
      kib[cls] += $2
      observed = 1
    }
    END {
      if (!observed) exit 1
      printf "%d\t%d\t%d\t%d", kib["go_heap"], kib["go_other"], kib["anon_other"], kib["file"]
    }
  ' "$1"
}

# User plus system CPU ticks. The command name sits in parentheses and may hold
# spaces, so count fields after the last ")": utime and stime are 12th and 13th.
cpu_ticks() {
  sed 's/.*) //' "$1" | awk '{ print $12 + $13 }'
}

# Bytes this process sent to storage, then bytes it passed to write calls.
# Either is empty when the kernel has no task I/O accounting.
io_bytes() {
  awk '/^write_bytes:/ { w = $2 } /^wchar:/ { c = $2 } END { printf "%s\t%s", w, c }' "$1"
}

read_memory_status() {
  awk '
    /^VmRSS:/ { rss = $2 }
    /^VmHWM:/ { hwm = $2 }
    END { if (rss != "") printf "%s\t%s\n", rss, hwm }
  ' "$1"
}

run_tree() {
  local label=$1 port=$2
  local data="${OUT_DIR}/data-${label}" log="${OUT_DIR}/run-${label}.log"
  local samples="${OUT_DIR}/samples-${label}.tsv"
  rm -rf "${data}" && mkdir -p "${data}"
  # A host with an installed Pulse can hold an unreadable /etc/pulse; point
  # the legacy key lookup at a path that does not exist instead.
  env -i HOME="${HOME}" PATH="${PATH}" "${ESTATE_ENV[@]}" \
    PULSE_DATA_DIR="${data}" PORT="${port}" FRONTEND_PORT="${port}" \
    PULSE_METRICS_BIND_ADDRESS=127.0.0.1 PULSE_METRICS_PORT=0 \
    PULSE_LEGACY_KEY_PATH="${data}/no-legacy-key" \
    PULSE_DEV=true ALLOW_ADMIN_BYPASS=1 PULSE_DEV_DISABLE_BACKGROUND_AI=true \
    PULSE_TELEMETRY=false PULSE_LICENSE_SERVER_URL=http://127.0.0.1:9 \
    GOMAXPROCS=1 \
    "${OUT_DIR}/pulse-${label}" > "${log}" 2>&1 &
  local pid=$!
  local driver=""
  local driver_log=""
  local start now rss rss_hwm mappings
  start=$(date +%s)
  if [[ "${ESTATE}" == agents ]]; then
    # The sampling loop ends the driver with SIGTERM. A driver deadline equal
    # to the sampling window can expire as that signal arrives, and Python then
    # exits by the signal (143) instead of handling it, failing the run.
    driver_log="${OUT_DIR}/agents-${label}.log"
    python3 "${SCRIPT_DIR}/agent_load.py" --port "${port}" --duration "$((DURATION + 120))" \
      --stats "${OUT_DIR}/agents-${label}.json" > "${driver_log}" 2>&1 &
    driver=$!
  elif [[ "${ESTATE}" == proxmox ]]; then
    driver_log="${OUT_DIR}/proxmox-${label}.log"
    python3 "${SCRIPT_DIR}/proxmox_load.py" --port "${port}" --duration "$((DURATION + 120))" \
      --steady-since "$((start + DURATION * 2 / 3))" \
      --sample-until "$((start + DURATION))" \
      --stats "${OUT_DIR}/proxmox-${label}.json" > "${driver_log}" 2>&1 &
    driver=$!
  fi
  printf 'elapsed_s\trss_kib\tgo_heap_kib\tgo_other_kib\tanon_other_kib\tfile_kib\tcpu_ticks\twrite_bytes\twchar_bytes\trss_hwm_kib\n' > "${samples}"
  while kill -0 "${pid}" 2>/dev/null; do
    now=$(( $(date +%s) - start ))
    (( now >= DURATION )) && break
    IFS=$'\t' read -r rss rss_hwm < <(read_memory_status "/proc/${pid}/status" 2>/dev/null) || true
    if [[ -n "${rss}" ]]; then
      if ! mappings="$(classify_smaps "/proc/${pid}/smaps" 2>/dev/null)"; then
        # Keep all four fields empty. A failed read can still emit partial
        # output, and a short row would move CPU, I/O and high-water fields.
        mappings=$'\t\t\t'
      fi
      printf '%s\t%s\t%s\t%s\t%s\t%s\n' "${now}" "${rss}" "${mappings}" \
        "$(cpu_ticks "/proc/${pid}/stat" 2>/dev/null || true)" \
        "$(io_bytes "/proc/${pid}/io" 2>/dev/null || printf '\t')" "${rss_hwm}" >> "${samples}"
    fi
    sleep "${INTERVAL}"
  done
  local driver_status=null
  if [[ -n "${driver}" ]]; then
    kill -TERM "${driver}" 2>/dev/null || true
    driver_status=0
    wait "${driver}" || driver_status=$?
  fi
  if kill -0 "${pid}" 2>/dev/null; then
    local heap="${OUT_DIR}/heap-${label}.pb.gz"
    # Profiles are optional. An unresponsive endpoint must not prevent
    # checking the process outcome or expose a partial profile as evidence.
    if curl -fsS --max-time 5 -o "${heap}.partial" "http://127.0.0.1:${port}/debug/pprof/heap"; then
      mv "${heap}.partial" "${heap}" || true
    fi
  fi
  local stop_request_status=0 pulse_status=0 before_stop=null after_stop_request=null
  # Pulse can exit between the liveness check and the end of the profile read.
  # Require the stop request itself to succeed, then inspect the actual wait.
  kill -TERM "${pid}" 2>/dev/null || stop_request_status=$?
  wait "${pid}" 2>/dev/null || pulse_status=$?
  if (( stop_request_status != 0 )); then
    before_stop="${pulse_status}"
    echo "${label} exited before the watch stopped it (wait status ${pulse_status}); see ${log}" >&2
  elif (( pulse_status != 0 && pulse_status != 143 )); then
    after_stop_request="${pulse_status}"
    echo "${label} returned unexpected wait status ${pulse_status} after the watch requested TERM; see ${log}" >&2
  fi

  # This is the only stdout from each measurement worker. The parent binds
  # this observation to the corresponding worker's wait status.
  printf '{"pulse_wait_status_before_stop":%s,"pulse_wait_status_after_stop_request":%s,"driver_wait_status":%s}\n' \
    "${before_stop}" "${after_stop_request}" "${driver_status}"
  if [[ "${driver_status}" != null && "${driver_status}" != 0 ]]; then
    echo "${label} ${ESTATE} load failed (exit ${driver_status}); see ${driver_log}" >&2
    return 1
  fi
  [[ "${before_stop}" == null && "${after_stop_request}" == null ]]
}

build_tree baseline "${BASELINE_DIR}"
build_tree candidate "${CANDIDATE_DIR}"

# Run both at once so they share host conditions. The kernel assigns each
# instance its own loopback metrics port, so neither loses that listener.
run_tree baseline 7801 > "${OUT_DIR}/run-baseline-wait.json" & baseline_pid=$!
run_tree candidate 7802 > "${OUT_DIR}/run-candidate-wait.json" & candidate_pid=$!
baseline_status=0 candidate_status=0
wait "${baseline_pid}" || baseline_status=$?
wait "${candidate_pid}" || candidate_status=$?

if (( baseline_status == 0 && candidate_status == 0 )) &&
   [[ -s "${OUT_DIR}/heap-baseline.pb.gz" && -s "${OUT_DIR}/heap-candidate.pb.gz" ]]; then
  (cd "${CANDIDATE_DIR}" && go tool pprof -sample_index=inuse_space -top -nodecount=25 \
    -base "${OUT_DIR}/heap-baseline.pb.gz" "${OUT_DIR}/heap-candidate.pb.gz") \
    > "${OUT_DIR}/heap-growth.txt" 2>/dev/null || true
fi

python3 "${SCRIPT_DIR}/summarize_release_memory.py" "${OUT_DIR}" "${DURATION}" "${INTERVAL}" \
  "${baseline_status}" "${candidate_status}" "${source_binding[@]}"
