#!/usr/bin/env bash
# Pulse Cloud — Tarball Backup with DO Spaces Upload
# Creates optionally encrypted tarballs of CP data + tenant directories.
# Both source directories are required, even when remote upload is disabled.
# Configure physical source directories: tar would store a root symlink alone.
# Archive, encryption and configured uploads must succeed before retention.
# A timestamp must be exclusively reserved and must not replace existing files.
# A completed tar run is not application-consistency or restore verification.
# Supports 30-day retention with automatic cleanup.
#
# Usage: ./scripts/cloud-backup.sh
#
# Environment variables:
#   PULSE_DATA_DIR           — Root data directory (default: /data)
#   PULSE_TENANTS_DIR        — Tenant data directory (default: $PULSE_DATA_DIR/tenants)
#   PULSE_CONTROL_PLANE_DIR  — CP data directory (default: $PULSE_DATA_DIR/control-plane)
#   PULSE_BACKUP_DIR         — Local backup staging directory (default: /data/backups/tarballs)
#   PULSE_BACKUP_RETENTION   — Retention in days (default: 30)
#   PULSE_BACKUP_ENCRYPT     — Enable age encryption (default: false, set to "true")
#   PULSE_BACKUP_AGE_RECIPIENT — age recipient public key for encryption
#   PULSE_RCLONE_REMOTE      — rclone remote for DO Spaces (e.g. "do-spaces:pulse-backups")
#   PULSE_BACKUP_LOG_FILE    — Log file path (default: /var/log/pulse-cloud-backup-tarball.log)

set -euo pipefail
IFS=$'\n\t'

# Configuration
DATA_DIR="${PULSE_DATA_DIR:-/data}"
TENANTS_DIR="${PULSE_TENANTS_DIR:-${DATA_DIR}/tenants}"
CONTROL_PLANE_DIR="${PULSE_CONTROL_PLANE_DIR:-${DATA_DIR}/control-plane}"
BACKUP_DIR="${PULSE_BACKUP_DIR:-${DATA_DIR}/backups/tarballs}"
RETENTION_DAYS="${PULSE_BACKUP_RETENTION:-30}"
LOG_FILE="${PULSE_BACKUP_LOG_FILE:-/var/log/pulse-cloud-backup-tarball.log}"
ENCRYPT="${PULSE_BACKUP_ENCRYPT:-false}"
AGE_RECIPIENT="${PULSE_BACKUP_AGE_RECIPIENT:-}"

log() {
  echo "[$(date -u +'%Y-%m-%dT%H:%M:%SZ')] $*"
}

die() {
  log "ERROR: $*"
  exit 1
}

have() { command -v "$1" >/dev/null 2>&1; }

# tar archives a command-line symlink as a link, even though Bash's -d follows
# it. Reject that false-success path rather than enabling global dereferencing,
# which would also copy unrelated targets of links inside the data directories.
reject_linked_source() {
  local src_dir="$1"
  local label="$2"

  # A trailing slash makes -L follow the link. Match the root that basename
  # passes to tar without changing the configured path or archive layout.
  while [[ "${src_dir}" == */ && "${src_dir}" != / ]]; do
    src_dir="${src_dir%/}"
  done
  [[ ! -L "${src_dir}" ]] ||
    die "Required ${label} directory is a symbolic link; configure its physical directory; no backup or retention performed"
}

# Admit the whole configured job before creating any new archives. Otherwise a
# missing source can become a successful empty run that ages out good backups.
preflight() {
  [[ "${RETENTION_DAYS}" =~ ^[0-9]+$ && ! "${RETENTION_DAYS}" =~ ^0+$ ]] ||
    die "PULSE_BACKUP_RETENTION must be a positive whole number of days"
  [[ "${ENCRYPT}" == "true" || "${ENCRYPT}" == "false" ]] ||
    die "PULSE_BACKUP_ENCRYPT must be true or false"

  have tar || die "tar is required"
  have find || die "find is required for local retention"
  have rmdir || die "rmdir is required for backup timestamp reservations"
  [[ -d "${TENANTS_DIR}" ]] || die "Required tenants directory not found; no backup or retention performed"
  [[ -d "${CONTROL_PLANE_DIR}" ]] || die "Required control-plane directory not found; no backup or retention performed"
  reject_linked_source "${TENANTS_DIR}" "tenants"
  reject_linked_source "${CONTROL_PLANE_DIR}" "control-plane"

  if [[ "${ENCRYPT}" == "true" ]]; then
    have age || die "PULSE_BACKUP_ENCRYPT=true but 'age' is not installed. Install: https://github.com/FiloSottile/age"
    [[ -n "${AGE_RECIPIENT}" ]] || die "PULSE_BACKUP_ENCRYPT=true but PULSE_BACKUP_AGE_RECIPIENT is not set"
  fi
  if [[ -n "${PULSE_RCLONE_REMOTE:-}" ]]; then
    have rclone || die "PULSE_RCLONE_REMOTE set but rclone is not installed"
  fi
}

# Create tarball of a directory
create_tarball() {
  local src_dir="$1"
  local output_path="$2"
  local label="$3"

  if [[ ! -d "${src_dir}" ]]; then
    # Recheck after preflight as a source may disappear during the job. Never
    # continue to upload/retention with just the other source's archive.
    die "Required ${label} directory disappeared; backup incomplete, retention not performed"
  fi
  reject_linked_source "${src_dir}" "${label}"

  log "creating tarball: ${label} -> ${output_path}"
  tar -czf "${output_path}" -C "$(dirname "${src_dir}")" "$(basename "${src_dir}")"
  log "tarball created: ${output_path} ($(du -sh "${output_path}" | cut -f1))"
}

# Encrypt a file with age
encrypt_file() {
  local input_path="$1"
  local output_path="${input_path}.age"

  if [[ "${ENCRYPT}" != "true" ]]; then
    return 0
  fi

  log "encrypting: ${input_path}"
  age -r "${AGE_RECIPIENT}" -o "${output_path}" "${input_path}"
  rm -f "${input_path}"
  log "encrypted: ${output_path}"
}

# Upload to DO Spaces via rclone
upload_remote() {
  local file_path="$1"
  local remote_prefix="$2"

  if [[ -z "${PULSE_RCLONE_REMOTE:-}" ]]; then
    log "remote upload not configured (set PULSE_RCLONE_REMOTE)"
    return 0
  fi

  local filename
  filename="$(basename "${file_path}")"
  local dest="${PULSE_RCLONE_REMOTE}/${remote_prefix}/${filename}"

  log "uploading to remote: ${dest}"
  rclone copyto "${file_path}" "${dest}"
  log "upload complete: ${dest}"
}

# Rotate local backups (keep last N days)
rotate_local() {
  [[ -d "${BACKUP_DIR}" ]] || return 0

  log "rotating local backups (keeping last ${RETENTION_DAYS} days)"
  if ! find "${BACKUP_DIR}" -name "pulse-backup-*.tar.gz*" -mtime "+${RETENTION_DAYS}" -delete 2>/dev/null; then
    die "Local retention failed; new backup artifacts are retained, but cleanup may be incomplete"
  fi
}

# Rotate remote backups
rotate_remote() {
  if [[ -z "${PULSE_RCLONE_REMOTE:-}" ]]; then
    return 0
  fi

  local remote_dir="${PULSE_RCLONE_REMOTE}/pulse-cloud-tarballs"
  log "rotating remote backups (${RETENTION_DAYS} day retention)"
  if ! rclone delete "${remote_dir}" --min-age "${RETENTION_DAYS}d" 2>/dev/null; then
    die "Remote retention failed; backup uploads completed, but cleanup may be incomplete"
  fi
}

# Manual and scheduled invocations can start in the same UTC second. Reserve
# that name atomically before opening any artifact, and never reuse artifacts
# left by an earlier run (including a failed or encrypted one).
reserve_timestamp() {
  local timestamp="$1"
  local lock_dir="${BACKUP_DIR}/.pulse-backup-${timestamp}.lock"
  if ! mkdir -- "${lock_dir}"; then
    die "Backup timestamp is already reserved; no backup or retention performed"
  fi
  # Install cleanup only after acquiring our own directory. A losing invocation
  # must never release another run's reservation. SIGKILL may leave it behind;
  # later timestamps remain usable, and reusing that timestamp fails safely.
  run_lock_dir="${lock_dir}"
  trap 'rmdir -- "${run_lock_dir}" || { log "ERROR: Could not release backup timestamp reservation"; exit 1; }' EXIT

  local artifact
  for artifact in \
    "${BACKUP_DIR}/pulse-backup-tenants-${timestamp}.tar.gz" \
    "${BACKUP_DIR}/pulse-backup-tenants-${timestamp}.tar.gz.age" \
    "${BACKUP_DIR}/pulse-backup-cp-${timestamp}.tar.gz" \
    "${BACKUP_DIR}/pulse-backup-cp-${timestamp}.tar.gz.age" \
    "${BACKUP_DIR}/pulse-backup-meta-${timestamp}.txt"; do
    [[ ! -e "${artifact}" && ! -L "${artifact}" ]] ||
      die "Backup timestamp already has artifacts; no backup or retention performed"
  done
}

main() {
  # Set up logging
  mkdir -p "$(dirname "${LOG_FILE}")"
  touch "${LOG_FILE}" 2>/dev/null || true
  chmod 0640 "${LOG_FILE}" 2>/dev/null || true
  exec >>"${LOG_FILE}" 2>&1

  log "=== tarball backup started ==="

  preflight

  local timestamp
  timestamp="$(date -u +'%Y%m%d-%H%M%S')"
  umask 077
  mkdir -p "${BACKUP_DIR}"
  chmod 0700 "${BACKUP_DIR}"
  # Global for the EXIT trap, which also runs after main's local scope ends.
  run_lock_dir=""
  reserve_timestamp "${timestamp}"

  # Backup tenants
  local tenants_tarball="${BACKUP_DIR}/pulse-backup-tenants-${timestamp}.tar.gz"
  create_tarball "${TENANTS_DIR}" "${tenants_tarball}" "tenants"
  encrypt_file "${tenants_tarball}"

  # Backup control plane
  local cp_tarball="${BACKUP_DIR}/pulse-backup-cp-${timestamp}.tar.gz"
  create_tarball "${CONTROL_PLANE_DIR}" "${cp_tarball}" "control-plane"
  encrypt_file "${cp_tarball}"

  # Record metadata
  local meta_file="${BACKUP_DIR}/pulse-backup-meta-${timestamp}.txt"
  {
    echo "timestamp: ${timestamp}"
    echo "hostname: $(hostname)"
    echo "encrypted: ${ENCRYPT}"
    docker ps -a --no-trunc 2>/dev/null || echo "docker ps: unavailable"
  } > "${meta_file}"

  # Upload to remote
  local remote_prefix="pulse-cloud-tarballs"
  if [[ "${ENCRYPT}" == "true" ]]; then
    upload_remote "${tenants_tarball}.age" "${remote_prefix}"
    upload_remote "${cp_tarball}.age" "${remote_prefix}"
  else
    upload_remote "${tenants_tarball}" "${remote_prefix}"
    upload_remote "${cp_tarball}" "${remote_prefix}"
  fi
  upload_remote "${meta_file}" "${remote_prefix}"

  # Rotate
  rotate_local
  rotate_remote

  trap - EXIT
  rmdir -- "${run_lock_dir}" || die "Could not release backup timestamp reservation; backup and retention already completed"
  log "=== tarball backup finished ==="
}

main "$@"

