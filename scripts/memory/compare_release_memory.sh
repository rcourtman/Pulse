#!/usr/bin/env bash
# compare_release_memory.sh — run two Pulse source trees against the same
# estate and compare their steady-state memory (#1803).
#
# Usage: bash scripts/memory/compare_release_memory.sh <baseline-tree> <candidate-tree> <out-dir>
#
# Each tree is built from source with a placeholder frontend (release builds
# refuse mock fixtures), started with telemetry off and no licence server, and
# sampled every PULSE_MEMORY_INTERVAL seconds for PULSE_MEMORY_DURATION seconds.
# Samples split resident memory into the Go heap mapping, unnamed anonymous
# memory (where the modernc SQLite driver allocates) and file pages. At the end
# each run's heap profile is saved, and summarize_release_memory.py writes the
# comparison and exits 1 when the candidate grew past the tolerance.
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

set -euo pipefail

BASELINE_DIR="${1:?baseline tree}"
CANDIDATE_DIR="${2:?candidate tree}"
OUT_DIR="${3:?output directory}"
DURATION="${PULSE_MEMORY_DURATION:-900}"
INTERVAL="${PULSE_MEMORY_INTERVAL:-15}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ESTATE="${PULSE_FOOTPRINT_ESTATE:-mock}"

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
  *)
    echo "Unknown PULSE_FOOTPRINT_ESTATE '${ESTATE}'; use mock or agents" >&2
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
    /^Rss:/ { kib[cls] += $2 }
    END { printf "%d\t%d\t%d\t%d", kib["go_heap"], kib["go_other"], kib["anon_other"], kib["file"] }
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

run_tree() {
  local label=$1 port=$2
  local data="${OUT_DIR}/data-${label}" log="${OUT_DIR}/run-${label}.log"
  local samples="${OUT_DIR}/samples-${label}.tsv"
  rm -rf "${data}" && mkdir -p "${data}"
  # A host with an installed Pulse can hold an unreadable /etc/pulse; point
  # the legacy key lookup at a path that does not exist instead.
  env -i HOME="${HOME}" PATH="${PATH}" "${ESTATE_ENV[@]}" \
    PULSE_DATA_DIR="${data}" PORT="${port}" FRONTEND_PORT="${port}" \
    PULSE_LEGACY_KEY_PATH="${data}/no-legacy-key" \
    PULSE_DEV=true ALLOW_ADMIN_BYPASS=1 PULSE_DEV_DISABLE_BACKGROUND_AI=true \
    PULSE_TELEMETRY=false PULSE_LICENSE_SERVER_URL=http://127.0.0.1:9 \
    GOMAXPROCS=1 \
    "${OUT_DIR}/pulse-${label}" > "${log}" 2>&1 &
  local pid=$!
  local driver=""
  if [[ "${ESTATE}" == agents ]]; then
    python3 "${SCRIPT_DIR}/agent_load.py" --port "${port}" --duration "${DURATION}" \
      --stats "${OUT_DIR}/agents-${label}.json" > "${OUT_DIR}/agents-${label}.log" 2>&1 &
    driver=$!
  fi
  printf 'elapsed_s\trss_kib\tgo_heap_kib\tgo_other_kib\tanon_other_kib\tfile_kib\tcpu_ticks\twrite_bytes\twchar_bytes\n' > "${samples}"
  local start now
  start=$(date +%s)
  while kill -0 "${pid}" 2>/dev/null; do
    now=$(( $(date +%s) - start ))
    (( now >= DURATION )) && break
    local rss
    rss=$(awk '/^VmRSS:/ {print $2}' "/proc/${pid}/status" 2>/dev/null || true)
    if [[ -n "${rss}" ]]; then
      printf '%s\t%s\t%s\t%s\t%s\n' "${now}" "${rss}" "$(classify_smaps "/proc/${pid}/smaps")" \
        "$(cpu_ticks "/proc/${pid}/stat" 2>/dev/null || true)" \
        "$(io_bytes "/proc/${pid}/io" 2>/dev/null || printf '\t')" >> "${samples}"
    fi
    sleep "${INTERVAL}"
  done
  local driver_status=0
  if [[ -n "${driver}" ]]; then
    kill -TERM "${driver}" 2>/dev/null || true
    wait "${driver}" || driver_status=$?
  fi
  if kill -0 "${pid}" 2>/dev/null; then
    curl -fsS -o "${OUT_DIR}/heap-${label}.pb.gz" "http://127.0.0.1:${port}/debug/pprof/heap" || true
    kill "${pid}" 2>/dev/null || true
    wait "${pid}" 2>/dev/null || true
  else
    echo "${label} exited before the measurement ended; see ${log}" >&2
    return 1
  fi
  if (( driver_status != 0 )); then
    echo "${label} agent load failed (exit ${driver_status}); see ${OUT_DIR}/agents-${label}.log" >&2
    return 1
  fi
}

build_tree baseline "${BASELINE_DIR}"
build_tree candidate "${CANDIDATE_DIR}"

# Run both at once so they share host conditions. Only one can bind the fixed
# Prometheus port; the other logs a bind error, which costs no measurable memory.
run_tree baseline 7801 & baseline_pid=$!
run_tree candidate 7802 & candidate_pid=$!
status=0
wait "${baseline_pid}" || status=1
wait "${candidate_pid}" || status=1
(( status == 0 )) || exit 1

if [[ -s "${OUT_DIR}/heap-baseline.pb.gz" && -s "${OUT_DIR}/heap-candidate.pb.gz" ]]; then
  (cd "${CANDIDATE_DIR}" && go tool pprof -sample_index=inuse_space -top -nodecount=25 \
    -base "${OUT_DIR}/heap-baseline.pb.gz" "${OUT_DIR}/heap-candidate.pb.gz") \
    > "${OUT_DIR}/heap-growth.txt" 2>/dev/null || true
fi

python3 "${SCRIPT_DIR}/summarize_release_memory.py" "${OUT_DIR}"
