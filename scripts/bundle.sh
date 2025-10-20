#!/usr/bin/env bash
#
# Bundle modular installer scripts into distributable single files.
# Reads scripts/bundle.manifest for bundling instructions.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MANIFEST_PATH="${ROOT_DIR}/scripts/bundle.manifest"
HEADER_FMT='# === Begin: %s ==='
FOOTER_FMT='# === End: %s ==='

main() {
  [[ -f "${MANIFEST_PATH}" ]] || {
    echo "Missing manifest: ${MANIFEST_PATH}" >&2
    exit 1
  }

  local current_output=""
  local -a buffer=()
  local found_output=false

  while IFS= read -r line || [[ -n "${line}" ]]; do
    # Skip blank lines and comments.
    [[ -z "${line}" || "${line}" =~ ^[[:space:]]*# ]] && continue

    if [[ "${line}" =~ ^output:[[:space:]]+(.+) ]]; then
      flush_buffer "${current_output}" buffer
      buffer=()
      current_output="$(normalize_path "${BASH_REMATCH[1]}")"
      mkdir -p "$(dirname "${current_output}")"
      found_output=true
      continue
    fi

    [[ -n "${current_output}" ]] || {
      echo "Encountered source before output directive: ${line}" >&2
      exit 1
    }

    local source_path
    source_path="$(normalize_path "${line}")"

    if [[ ! -f "${source_path}" ]]; then
      echo "Source file missing: ${source_path}" >&2
      exit 1
    fi

    buffer+=("$(printf "${HEADER_FMT}" "$(relative_path "${source_path}")")")
    buffer+=("$(<"${source_path}")")
    buffer+=("$(printf "${FOOTER_FMT}" "$(relative_path "${source_path}")")"$'\n')
  done < "${MANIFEST_PATH}"

  flush_buffer "${current_output}" buffer

  if [[ "${found_output}" == false ]]; then
    echo "No output directive found in ${MANIFEST_PATH}" >&2
    exit 1
  fi

  echo "Bundling complete."
}

# flush_buffer <output_path> <buffer_array>
flush_buffer() {
  local target="$1"
  shift
  [[ $# -gt 0 ]] || return 0
  local -n buf_ref=$1

  if [[ -z "${target}" ]]; then
    return 0
  fi

  {
    echo "# Generated file. Do not edit."
    printf '# Bundled on: %s\n' "$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
    printf '# Manifest: %s\n\n' "$(relative_path "${MANIFEST_PATH}")"
    printf '%s\n' "${buf_ref[@]}"
  } > "${target}.tmp"

  mv "${target}.tmp" "${target}"
  if [[ "${target}" == *.sh ]]; then
    chmod +x "${target}"
  fi
  echo "Wrote $(relative_path "${target}")"
}

# normalize_path <relative_or_absolute_path>
normalize_path() {
  local path="$1"
  if [[ "${path}" == /* ]]; then
    printf '%s\n' "${path}"
  else
    printf '%s\n' "${ROOT_DIR}/${path}"
  fi
}

# relative_path <absolute_path>
relative_path() {
  local path="$1"
  python3 - "$ROOT_DIR" "$path" <<'PY'
import os
import sys

root = os.path.abspath(sys.argv[1])
target = os.path.abspath(sys.argv[2])
print(os.path.relpath(target, root))
PY
}

main "$@"
