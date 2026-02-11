#!/usr/bin/env bash
# Pulse Cloud — Tarball Backup with DO Spaces Upload
# Creates encrypted tarballs of CP data + tenant directories, uploads to DO Spaces.
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

# Create tarball of a directory
create_tarball() {
  local src_dir="$1"
  local output_path="$2"
  local label="$3"

  if [[ ! -d "${src_dir}" ]]; then
    log "WARN: ${label} directory not found: ${src_dir} — skipping"
    return 0
  fi

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

  have age || die "PULSE_BACKUP_ENCRYPT=true but 'age' is not installed. Install: https://github.com/FiloSottile/age"

  if [[ -z "${AGE_RECIPIENT}" ]]; then
    die "PULSE_BACKUP_ENCRYPT=true but PULSE_BACKUP_AGE_RECIPIENT is not set"
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

  have rclone || die "PULSE_RCLONE_REMOTE set but rclone is not installed"

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
  find "${BACKUP_DIR}" -name "pulse-backup-*.tar.gz*" -mtime "+${RETENTION_DAYS}" -delete 2>/dev/null || true
}

# Rotate remote backups
rotate_remote() {
  if [[ -z "${PULSE_RCLONE_REMOTE:-}" ]]; then
    return 0
  fi

  local remote_dir="${PULSE_RCLONE_REMOTE}/pulse-cloud-tarballs"
  log "rotating remote backups (${RETENTION_DAYS} day retention)"
  rclone delete "${remote_dir}" --min-age "${RETENTION_DAYS}d" 2>/dev/null || true
}

main() {
  # Set up logging
  mkdir -p "$(dirname "${LOG_FILE}")"
  touch "${LOG_FILE}" 2>/dev/null || true
  chmod 0640 "${LOG_FILE}" 2>/dev/null || true
  exec >>"${LOG_FILE}" 2>&1

  log "=== tarball backup started ==="

  have tar || die "tar is required"

  local timestamp
  timestamp="$(date -u +'%Y%m%d-%H%M%S')"
  umask 077
  mkdir -p "${BACKUP_DIR}"
  chmod 0700 "${BACKUP_DIR}"

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

  log "=== tarball backup finished ==="
}

main "$@"

