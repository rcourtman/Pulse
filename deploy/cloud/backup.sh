#!/usr/bin/env bash
# Pulse Cloud — Daily Backup to DigitalOcean Spaces
# Run via cron: 0 3 * * * /opt/pulse-cloud/backup.sh

set -euo pipefail
IFS=$'\n\t'

LOG_FILE="${PULSE_BACKUP_LOG_FILE:-/var/log/pulse-cloud-backup.log}"
DATA_DIR="${PULSE_DATA_DIR:-/data}"
TENANTS_DIR="${PULSE_TENANTS_DIR:-${DATA_DIR}/tenants}"
CONTROL_PLANE_DIR="${PULSE_CONTROL_PLANE_DIR:-${DATA_DIR}/control-plane}"
BACKUP_ROOT="${PULSE_BACKUP_ROOT:-${DATA_DIR}/backups/daily}"
RETENTION_DAYS="${PULSE_BACKUP_RETENTION_DAYS:-7}"

# Remote sync (optional):
# - rclone: set PULSE_RCLONE_REMOTE (e.g. "do-spaces:pulse-cloud-backups")
# - s3cmd:  set PULSE_S3CMD_BUCKET (e.g. "s3://pulse-cloud-backups")
# Both support optional prefix PULSE_BACKUP_REMOTE_PREFIX (default "pulse-cloud").
PULSE_BACKUP_REMOTE_PREFIX="${PULSE_BACKUP_REMOTE_PREFIX:-pulse-cloud}"

log() {
  echo "[$(date -u +'%Y-%m-%dT%H:%M:%SZ')] $*"
}

die() {
  log "error: $*"
  exit 1
}

have() { command -v "$1" >/dev/null 2>&1; }

rotate_local() {
  # Keep the newest N day directories (YYYY-MM-DD).
  local keep="${RETENTION_DAYS}"
  [[ -d "${BACKUP_ROOT}" ]] || return 0

  local dirs
  dirs="$(find "${BACKUP_ROOT}" -mindepth 1 -maxdepth 1 -type d -printf '%f\n' | sort)"
  local count
  count="$(echo "${dirs}" | sed '/^$/d' | wc -l | tr -d ' ')"
  if [[ "${count}" -le "${keep}" ]]; then
    return 0
  fi

  local to_delete
  to_delete="$(echo "${dirs}" | head -n "$((count - keep))")"
  if [[ -n "${to_delete}" ]]; then
    while IFS= read -r d; do
      [[ -n "${d}" ]] || continue
      log "rotating local backup (deleting): ${BACKUP_ROOT}/${d}"
      rm -rf "${BACKUP_ROOT:?}/${d}"
    done <<<"${to_delete}"
  fi
}

rclone_sync() {
  local src_dir="$1"
  local date_dir
  date_dir="$(basename "${src_dir}")"

  local remote="${PULSE_RCLONE_REMOTE:?missing PULSE_RCLONE_REMOTE}"
  local dst="${remote}/${PULSE_BACKUP_REMOTE_PREFIX}/daily/${date_dir}"

  log "rclone sync -> ${dst}"
  rclone sync --checksum --transfers 4 --checkers 8 "${src_dir}" "${dst}"

  # Best-effort remote rotation.
  local daily_root="${remote}/${PULSE_BACKUP_REMOTE_PREFIX}/daily"
  local dirs
  dirs="$(rclone lsf "${daily_root}" --dirs-only 2>/dev/null | sed 's:/*$::' | sort || true)"
  local count
  count="$(echo "${dirs}" | sed '/^$/d' | wc -l | tr -d ' ')"
  if [[ "${count}" -le "${RETENTION_DAYS}" ]]; then
    return 0
  fi
  local to_delete
  to_delete="$(echo "${dirs}" | head -n "$((count - RETENTION_DAYS))")"
  while IFS= read -r d; do
    [[ -n "${d}" ]] || continue
    log "rotating remote backup (deleting): ${daily_root}/${d}"
    rclone purge "${daily_root}/${d}" || true
  done <<<"${to_delete}"
}

s3cmd_sync() {
  local src_dir="$1"
  local date_dir
  date_dir="$(basename "${src_dir}")"

  local bucket="${PULSE_S3CMD_BUCKET:?missing PULSE_S3CMD_BUCKET}"
  local dst="${bucket%/}/${PULSE_BACKUP_REMOTE_PREFIX}/daily/${date_dir}/"

  log "s3cmd sync -> ${dst}"
  s3cmd sync --delete-removed --no-progress "${src_dir}/" "${dst}"

  # Best-effort remote rotation: list day prefixes and remove older ones.
  local daily_root="${bucket%/}/${PULSE_BACKUP_REMOTE_PREFIX}/daily/"
  local dirs
  dirs="$(s3cmd ls "${daily_root}" 2>/dev/null | awk '{print $2}' | sed -E 's:/*$::' | awk -F/ '{print $(NF)}' | sort -u || true)"
  local count
  count="$(echo "${dirs}" | sed '/^$/d' | wc -l | tr -d ' ')"
  if [[ "${count}" -le "${RETENTION_DAYS}" ]]; then
    return 0
  fi
  local to_delete
  to_delete="$(echo "${dirs}" | head -n "$((count - RETENTION_DAYS))")"
  while IFS= read -r d; do
    [[ -n "${d}" ]] || continue
    log "rotating remote backup (deleting): ${daily_root}${d}/"
    s3cmd del --recursive "${daily_root}${d}/" >/dev/null || true
  done <<<"${to_delete}"
}

main() {
  # Log everything (cron-friendly).
  mkdir -p "$(dirname "${LOG_FILE}")"
  touch "${LOG_FILE}"
  chmod 0640 "${LOG_FILE}" || true
  exec >>"${LOG_FILE}" 2>&1

  log "backup started"

  have rsync || die "missing rsync"
  have sqlite3 || log "note: sqlite3 not found (backup is rsync-based; sqlite3 not required)"

  local day
  day="$(date -u +%F)"
  local dest="${BACKUP_ROOT}/${day}"

  umask 077
  mkdir -p "${dest}/tenants"
  mkdir -p "${dest}/meta"
  chmod 0700 "${dest}"

  if [[ ! -d "${TENANTS_DIR}" ]]; then
    die "missing tenants dir: ${TENANTS_DIR}"
  fi

  log "snapshotting tenant data dirs (sequential)"
  local tenant_path tenant_id
  for tenant_path in "${TENANTS_DIR}"/*; do
    [[ -d "${tenant_path}" ]] || continue
    tenant_id="$(basename "${tenant_path}")"

    # Copy one tenant at a time to avoid an I/O spike.
    log "rsync tenant: ${tenant_id}"
    mkdir -p "${dest}/tenants/${tenant_id}"
    rsync -a --delete --numeric-ids "${tenant_path}/" "${dest}/tenants/${tenant_id}/"
  done

  if [[ -d "${CONTROL_PLANE_DIR}" ]]; then
    log "rsync control-plane dir"
    mkdir -p "${dest}/control-plane"
    rsync -a --delete --numeric-ids "${CONTROL_PLANE_DIR}/" "${dest}/control-plane/"
  else
    log "note: control-plane dir not found (${CONTROL_PLANE_DIR}); skipping"
  fi

  log "recording metadata"
  date -u +'%Y-%m-%dT%H:%M:%SZ' >"${dest}/meta/created_at_utc.txt"
  docker ps -a --no-trunc >"${dest}/meta/docker_ps.txt" 2>/dev/null || true
  docker images --digests >"${dest}/meta/docker_images.txt" 2>/dev/null || true

  ln -sfn "${dest}" "${BACKUP_ROOT}/latest"

  rotate_local

  # Remote sync is optional (local snapshots still useful for fast restores).
  if [[ -n "${PULSE_RCLONE_REMOTE:-}" ]]; then
    have rclone || die "PULSE_RCLONE_REMOTE set but rclone is missing"
    rclone_sync "${dest}"
  elif [[ -n "${PULSE_S3CMD_BUCKET:-}" ]]; then
    have s3cmd || die "PULSE_S3CMD_BUCKET set but s3cmd is missing"
    s3cmd_sync "${dest}"
  else
    log "remote sync not configured (set PULSE_RCLONE_REMOTE or PULSE_S3CMD_BUCKET)"
  fi

  log "backup finished: ${dest}"
}

main "$@"

