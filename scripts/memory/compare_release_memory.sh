#!/usr/bin/env bash
# compare_release_memory.sh — run two Pulse source trees against the same fixed
# small mock estate and compare their steady-state memory (#1803).
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
# Mock mode keeps its own fixtures in the heap, so a fixture change shows up
# here too. Read the heap profile diff before blaming production code.

set -euo pipefail

BASELINE_DIR="${1:?baseline tree}"
CANDIDATE_DIR="${2:?candidate tree}"
OUT_DIR="${3:?output directory}"
DURATION="${PULSE_MEMORY_DURATION:-900}"
INTERVAL="${PULSE_MEMORY_INTERVAL:-15}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

mkdir -p "${OUT_DIR}"
OUT_DIR="$(cd "${OUT_DIR}" && pwd)"

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

run_tree() {
  local label=$1 port=$2
  local data="${OUT_DIR}/data-${label}" log="${OUT_DIR}/run-${label}.log"
  local samples="${OUT_DIR}/samples-${label}.tsv"
  rm -rf "${data}" && mkdir -p "${data}"
  env -i HOME="${HOME}" PATH="${PATH}" \
    PULSE_MOCK_MODE=true PULSE_MOCK_NODES=2 PULSE_MOCK_VMS_PER_NODE=6 PULSE_MOCK_LXCS_PER_NODE=6 \
    PULSE_MOCK_DOCKER_HOSTS=1 PULSE_MOCK_DOCKER_CONTAINERS=10 PULSE_MOCK_GENERIC_HOSTS=1 \
    PULSE_MOCK_K8S_CLUSTERS=0 \
    PULSE_DATA_DIR="${data}" PORT="${port}" FRONTEND_PORT="${port}" \
    PULSE_DEV=true ALLOW_ADMIN_BYPASS=1 PULSE_DEV_DISABLE_BACKGROUND_AI=true \
    PULSE_TELEMETRY=false PULSE_LICENSE_SERVER_URL=http://127.0.0.1:9 \
    GOMAXPROCS=1 \
    "${OUT_DIR}/pulse-${label}" > "${log}" 2>&1 &
  local pid=$!
  printf 'elapsed_s\trss_kib\tgo_heap_kib\tgo_other_kib\tanon_other_kib\tfile_kib\n' > "${samples}"
  local start now
  start=$(date +%s)
  while kill -0 "${pid}" 2>/dev/null; do
    now=$(( $(date +%s) - start ))
    (( now >= DURATION )) && break
    local rss
    rss=$(awk '/^VmRSS:/ {print $2}' "/proc/${pid}/status" 2>/dev/null || true)
    if [[ -n "${rss}" ]]; then
      printf '%s\t%s\t%s\n' "${now}" "${rss}" "$(classify_smaps "/proc/${pid}/smaps")" >> "${samples}"
    fi
    sleep "${INTERVAL}"
  done
  if kill -0 "${pid}" 2>/dev/null; then
    curl -fsS -o "${OUT_DIR}/heap-${label}.pb.gz" "http://127.0.0.1:${port}/debug/pprof/heap" || true
    kill "${pid}" 2>/dev/null || true
    wait "${pid}" 2>/dev/null || true
  else
    echo "${label} exited before the measurement ended; see ${log}" >&2
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
