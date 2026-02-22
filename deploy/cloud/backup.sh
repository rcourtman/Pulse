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
BACKUP_METRICS_FILE="${PULSE_BACKUP_METRICS_FILE:-/var/lib/node_exporter/textfile_collector/pulse_cloud_backup.prom}"

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

write_backup_metrics() {
  local success="$1"
  local now
  now="$(date -u +%s)"

  local metrics_dir
  metrics_dir="$(dirname "${BACKUP_METRICS_FILE}")"
  if ! mkdir -p "${metrics_dir}" >/dev/null 2>&1; then
    return 0
  fi

  local tmp="${BACKUP_METRICS_FILE}.tmp"
  cat >"${tmp}" <<EOF
# HELP pulse_cloud_backup_success Whether the most recent backup run succeeded (1) or failed (0).
# TYPE pulse_cloud_backup_success gauge
pulse_cloud_backup_success ${success}
# HELP pulse_cloud_backup_last_run_timestamp_seconds Unix timestamp of the most recent backup run completion.
# TYPE pulse_cloud_backup_last_run_timestamp_seconds gauge
pulse_cloud_backup_last_run_timestamp_seconds ${now}
EOF
  mv "${tmp}" "${BACKUP_METRICS_FILE}" || true
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
  while IFS= read -r -d '' db_file; do
    verify_sqlite_file "${db_file}"
  done < <(find "${root}" -type f -name '*.db' -print0 2>/dev/null || true)
}

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
  trap 'rc=$?; if [[ $rc -eq 0 ]]; then write_backup_metrics 1; else write_backup_metrics 0; fi' EXIT

  have rsync || die "missing rsync"
  have sqlite3 || die "missing sqlite3 (required for backup integrity checks)"

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
    local container_name="pulse-${tenant_id}"
    local paused="0"

    # Copy one tenant at a time to avoid an I/O spike. If the tenant container is
    # running, briefly pause it for a crash-consistent snapshot.
    if docker ps --format '{{.Names}}' | grep -qx "${container_name}"; then
      if docker pause "${container_name}" >/dev/null 2>&1; then
        paused="1"
        log "tenant paused for snapshot: ${tenant_id}"
      else
        log "warning: failed to pause tenant container ${container_name}; proceeding with live copy"
      fi
    fi

    log "rsync tenant: ${tenant_id} (paused=${paused})"
    mkdir -p "${dest}/tenants/${tenant_id}"
    if ! rsync -a --delete --numeric-ids "${tenant_path}/" "${dest}/tenants/${tenant_id}/"; then
      if [[ "${paused}" == "1" ]]; then
        docker unpause "${container_name}" >/dev/null 2>&1 || true
      fi
      die "rsync failed for tenant ${tenant_id}"
    fi
    verify_sqlite_tree "${dest}/tenants/${tenant_id}"
    if [[ "${paused}" == "1" ]]; then
      if docker unpause "${container_name}" >/dev/null 2>&1; then
        log "tenant resumed after snapshot: ${tenant_id}"
      else
        log "warning: failed to unpause tenant container ${container_name}; manual intervention may be required"
      fi
    fi
  done

  if [[ -d "${CONTROL_PLANE_DIR}" ]]; then
    log "rsync control-plane dir"
    mkdir -p "${dest}/control-plane"
    rsync -a --delete --numeric-ids "${CONTROL_PLANE_DIR}/" "${dest}/control-plane/"
    verify_sqlite_tree "${dest}/control-plane"
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
