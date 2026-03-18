#!/usr/bin/env bash
#
# Validates repository script references:
# 1) `./scripts/*.sh` references inside tracked shell scripts resolve.
# 2) `scripts/bundle.manifest` source entries exist.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BUNDLE_MANIFEST="${ROOT_DIR}/scripts/bundle.manifest"
STANDALONE_MANIFEST="${ROOT_DIR}/scripts/standalone.manifest"

failures=0
declare -A STANDALONE_DECLARED=()

record_failure() {
  echo "[FAIL] $1" >&2
  ((failures+=1))
}

record_pass() {
  echo "[PASS] $1"
}

check_script_refs() {
  local checked=0
  local script rel_ref target

  while IFS= read -r script; do
    [[ -f "${script}" ]] || continue
    while IFS= read -r rel_ref; do
      target="${ROOT_DIR}/${rel_ref#./}"
      ((checked += 1))
      if [[ ! -f "${target}" ]]; then
        record_failure "${script#${ROOT_DIR}/}: missing referenced script ${rel_ref}"
      fi
    done < <(grep -Eo '\./scripts/[A-Za-z0-9_.-]+\.sh' "${script}" | sort -u || true)
  done < <(git -C "${ROOT_DIR}" ls-files '*.sh' | sed "s#^#${ROOT_DIR}/#")

  if (( checked == 0 )); then
    record_failure "no script references scanned"
    return 1
  fi

  if (( failures == 0 )); then
    record_pass "tracked script references resolve (${checked} references scanned)"
  fi
}

check_bundle_manifest() {
  local checked=0
  local line source_path

  if [[ ! -f "${BUNDLE_MANIFEST}" ]]; then
    record_failure "missing bundle manifest (${BUNDLE_MANIFEST})"
    return 1
  fi

  while IFS= read -r line; do
    [[ -z "${line}" || "${line}" =~ ^[[:space:]]*# ]] && continue
    [[ "${line}" =~ ^output:[[:space:]]+ ]] && continue

    source_path="${ROOT_DIR}/${line}"
    ((checked += 1))
    if [[ ! -f "${source_path}" ]]; then
      record_failure "bundle manifest entry missing: ${line}"
    fi
  done < "${BUNDLE_MANIFEST}"

  if (( checked == 0 )); then
    record_failure "bundle manifest contains no source entries"
    return 1
  fi

  if (( failures == 0 )); then
    record_pass "bundle manifest source entries resolve (${checked} entries checked)"
  fi
}

load_standalone_manifest() {
  local line script reason

  if [[ ! -f "${STANDALONE_MANIFEST}" ]]; then
    record_failure "missing standalone manifest (${STANDALONE_MANIFEST})"
    return 1
  fi

  while IFS= read -r line; do
    [[ -z "${line}" || "${line}" =~ ^[[:space:]]*# ]] && continue
    if [[ "${line}" != *"|"* ]]; then
      record_failure "standalone manifest malformed line: ${line}"
      continue
    fi

    script="$(echo "${line%%|*}" | xargs)"
    reason="$(echo "${line#*|}" | xargs)"

    if [[ -z "${script}" || -z "${reason}" ]]; then
      record_failure "standalone manifest missing script or reason: ${line}"
      continue
    fi

    if [[ "${script}" != scripts/*.sh ]]; then
      record_failure "standalone manifest entry must be scripts/*.sh: ${script}"
      continue
    fi

    if ! git -C "${ROOT_DIR}" ls-files --error-unmatch "${script}" >/dev/null 2>&1; then
      record_failure "standalone manifest entry not tracked: ${script}"
      continue
    fi

    STANDALONE_DECLARED["${script}"]="${reason}"
  done < "${STANDALONE_MANIFEST}"

  if (( ${#STANDALONE_DECLARED[@]} == 0 )); then
    record_failure "standalone manifest has no usable entries"
    return 1
  fi
}

has_external_path_reference() {
  local script="$1"
  local pattern result_line ref_file normalized

  for pattern in "${script}" "./${script}"; do
    while IFS= read -r result_line; do
      ref_file="${result_line%%:*}"
      normalized="${ref_file#${ROOT_DIR}/}"
      normalized="${normalized#./}"
      [[ "${normalized}" == "${script}" ]] && continue
      [[ "${normalized}" == "scripts/standalone.manifest" ]] && continue
      return 0
    done < <(rg --hidden -n -F "${pattern}" "${ROOT_DIR}" --glob '!**/.git/**' || true)
  done

  return 1
}

check_unreferenced_scripts_are_declared() {
  local script checked=0 unreferenced=0

  while IFS= read -r script; do
    [[ "${script}" == scripts/tests/* ]] && continue
    [[ "${script}" == scripts/lib/* ]] && continue
    ((checked += 1))

    if has_external_path_reference "${script}"; then
      continue
    fi

    ((unreferenced += 1))
    if [[ -z "${STANDALONE_DECLARED[${script}]:-}" ]]; then
      record_failure "unreferenced script must be declared in scripts/standalone.manifest: ${script}"
    fi
  done < <(git -C "${ROOT_DIR}" ls-files '*.sh' | grep '^scripts/' || true)

  if (( checked == 0 )); then
    record_failure "no scripts checked for standalone declaration"
    return 1
  fi

  if (( failures == 0 )); then
    record_pass "unreferenced script declaration check passed (${unreferenced} unreferenced scripts declared)"
  fi
}

main() {
  check_script_refs
  check_bundle_manifest
  load_standalone_manifest
  check_unreferenced_scripts_are_declared

  if (( failures > 0 )); then
    echo "script reference integrity checks failed: ${failures}" >&2
    exit 1
  fi

  echo "All script reference integrity checks passed."
}

main "$@"
