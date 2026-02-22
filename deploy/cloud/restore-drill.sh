#!/usr/bin/env bash
# Pulse Cloud — Backup Restore Drill
# Validates that a daily snapshot can be restored and key SQLite files are sane.
#
# Usage:
#   ./restore-drill.sh --day 2026-02-10 --tenant t-ABCDEFGHJK

set -euo pipefail
IFS=$'\n\t'

BACKUP_ROOT="${PULSE_BACKUP_ROOT:-/data/backups/daily}"
OUTPUT_ROOT="${PULSE_RESTORE_DRILL_ROOT:-/tmp/pulse-restore-drill}"
DAY=""
TENANT_ID=""
KEEP_OUTPUT="0"

log() {
  echo "[$(date -u +'%Y-%m-%dT%H:%M:%SZ')] $*"
}

die() {
  echo "error: $*" >&2
  exit 1
}

have() { command -v "$1" >/dev/null 2>&1; }

usage() {
  cat <<EOF
Usage:
  $0 --day YYYY-MM-DD --tenant TENANT_ID [--keep-output]

Options:
  --day           Backup day directory under ${BACKUP_ROOT}
  --tenant        Tenant ID to restore (e.g. t-ABCDEFGHJK)
  --keep-output   Keep restored files under ${OUTPUT_ROOT}/<timestamp> after success

Environment:
  PULSE_BACKUP_ROOT        (default: ${BACKUP_ROOT})
  PULSE_RESTORE_DRILL_ROOT (default: ${OUTPUT_ROOT})
EOF
}

verify_sqlite_file() {
  local db_file="$1"
  local result
  result="$(sqlite3 "${db_file}" 'PRAGMA quick_check;' 2>/dev/null || true)"
  if [[ "${result}" != "ok" ]]; then
    die "sqlite integrity check failed for ${db_file}: ${result:-<empty>}"
  fi
}

verify_sqlite_tree() {
  local root="$1"
  local found=0
  while IFS= read -r -d '' db_file; do
    found=1
    verify_sqlite_file "${db_file}"
  done < <(find "${root}" -type f -name '*.db' -print0 2>/dev/null || true)

  if [[ "${found}" -eq 0 ]]; then
    log "no sqlite files found under ${root} (skipping sqlite checks there)"
  fi
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --day)
        [[ $# -ge 2 ]] || die "--day requires a value"
        DAY="$2"
        shift 2
        ;;
      --tenant)
        [[ $# -ge 2 ]] || die "--tenant requires a value"
        TENANT_ID="$2"
        shift 2
        ;;
      --keep-output)
        KEEP_OUTPUT="1"
        shift
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        die "unknown argument: $1"
        ;;
    esac
  done
}

main() {
  parse_args "$@"

  [[ -n "${DAY}" ]] || die "--day is required"
  [[ -n "${TENANT_ID}" ]] || die "--tenant is required"
  have rsync || die "missing rsync"
  have sqlite3 || die "missing sqlite3"

  local source_dir="${BACKUP_ROOT}/${DAY}"
  local source_tenant="${source_dir}/tenants/${TENANT_ID}"
  local source_control_plane="${source_dir}/control-plane"
  [[ -d "${source_dir}" ]] || die "backup day not found: ${source_dir}"
  [[ -d "${source_tenant}" ]] || die "tenant backup not found: ${source_tenant}"
  [[ -d "${source_control_plane}" ]] || die "control-plane backup not found: ${source_control_plane}"

  local run_dir="${OUTPUT_ROOT}/$(date -u +'%Y%m%dT%H%M%SZ')-${TENANT_ID}"
  local out_tenant="${run_dir}/restore/tenants/${TENANT_ID}"
  local out_control_plane="${run_dir}/restore/control-plane"
  mkdir -p "${out_tenant}" "${out_control_plane}"
  chmod 0700 "${run_dir}" || true

  log "restore drill started (day=${DAY}, tenant=${TENANT_ID})"
  log "restoring tenant snapshot"
  rsync -a --delete --numeric-ids "${source_tenant}/" "${out_tenant}/"

  log "restoring control-plane snapshot"
  rsync -a --delete --numeric-ids "${source_control_plane}/" "${out_control_plane}/"

  log "verifying SQLite integrity under restored tenant data"
  verify_sqlite_tree "${out_tenant}"

  log "verifying SQLite integrity under restored control-plane data"
  verify_sqlite_tree "${out_control_plane}"

  log "restore drill succeeded: ${run_dir}"
  if [[ "${KEEP_OUTPUT}" != "1" ]]; then
    rm -rf "${run_dir}"
    log "temporary restore output removed (use --keep-output to retain it)"
  fi
}

main "$@"
