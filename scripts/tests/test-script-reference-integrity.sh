#!/usr/bin/env bash
#
# Validates repository script references:
# 1) `./scripts/*.sh` references inside tracked shell scripts resolve.
# 2) `scripts/bundle.manifest` source entries exist.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BUNDLE_MANIFEST="${ROOT_DIR}/scripts/bundle.manifest"

failures=0

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

main() {
  check_script_refs
  check_bundle_manifest

  if (( failures > 0 )); then
    echo "script reference integrity checks failed: ${failures}" >&2
    exit 1
  fi

  echo "All script reference integrity checks passed."
}

main "$@"
