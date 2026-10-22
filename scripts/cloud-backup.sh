#!/usr/bin/env bash
# Pulse Cloud — Tarball Backup with DO Spaces Upload
# Creates optionally encrypted tarballs of CP data + tenant directories.
# Encryption also covers metadata; container inventory never includes commands.
# Encrypted jobs stream into age without writing new plaintext staging files.
# Both source directories are required, even when remote upload is disabled.
# Configure physical source directories: tar would store a root symlink alone.
# Staging must be outside both source trees, including through path aliases.
# Source directory names are literal tar members, never options or file lists.
# Ambient TAR_OPTIONS must not omit data or change the archive/link policy.
# Helpers sharing a staging directory reserve names before writing any output.
# Failed producers retain .partial evidence, not ordinary backup filenames.
# Archive, encryption and configured uploads must succeed before retention.
# Remote jobs reject rclone dry-run/interactive settings, never override them.
# Every remote copy must transfer its file, not merely exit zero after a skip.
# Local retention also ages out this helper's old top-level partial evidence.
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
#   PULSE_BACKUP_ENCRYPT     — Encrypt archives and metadata with age (default: false, set to "true")
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
BACKUP_NAME_RESERVATION=""
# Keep local and remote ownership identical. Match the generated timestamp
# shape, not arbitrary files that happen to share a prefix or archive suffix.
BACKUP_TIMESTAMP_PATTERN='[0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9]-[0-9][0-9][0-9][0-9][0-9][0-9]'
BACKUP_FILE_PATTERNS=(
  "pulse-backup-tenants-${BACKUP_TIMESTAMP_PATTERN}.tar.gz"
  "pulse-backup-tenants-${BACKUP_TIMESTAMP_PATTERN}.tar.gz.age"
  "pulse-backup-cp-${BACKUP_TIMESTAMP_PATTERN}.tar.gz"
  "pulse-backup-cp-${BACKUP_TIMESTAMP_PATTERN}.tar.gz.age"
  "pulse-backup-meta-${BACKUP_TIMESTAMP_PATTERN}.txt"
  "pulse-backup-meta-${BACKUP_TIMESTAMP_PATTERN}.txt.age"
)

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

# Never archive the staging area itself. Besides growing every successive
# backup, a streamed archive can include the file age is still writing. Resolve
# physical paths before mkdir/chmod so aliases and not-yet-created descendants
# cannot hide this mistake. NUL delimiters preserve newlines in configured paths.
reject_staging_in_source() {
  local staging tenants control_plane
  IFS= read -r -d '' staging < <(realpath -m -z -- "${BACKUP_DIR}") ||
    die "Cannot resolve PULSE_BACKUP_DIR; no archives or retention performed"
  IFS= read -r -d '' tenants < <(realpath -e -z -- "${TENANTS_DIR}") ||
    die "Cannot resolve required tenants directory; no archives or retention performed"
  IFS= read -r -d '' control_plane < <(realpath -e -z -- "${CONTROL_PLANE_DIR}") ||
    die "Cannot resolve required control-plane directory; no archives or retention performed"

  # Removing the root's trailing slash also handles a source configured as /.
  if [[ "${staging}/" == "${tenants%/}/"* || "${staging}/" == "${control_plane%/}/"* ]]; then
    die "PULSE_BACKUP_DIR must be outside both source directories; no archives or retention performed; choose a separate staging directory"
  fi
}

# A successful dry-run or declined interactive copy does not upload anything,
# but local find retention is independent of rclone. Refuse these modes before
# producing any artifacts rather than forcing a real mutation the operator did
# not request. Accept only unset/empty or Go's explicit false boolean spellings;
# invalid values fail closed without logging potentially private environment data.
reject_unconfirmed_remote_uploads() {
  local setting
  for setting in RCLONE_DRY_RUN RCLONE_INTERACTIVE; do
    case "${!setting:-false}" in
      false|False|FALSE|0|f|F) ;;
      *) die "${setting} must be unset or false for remote backups; this helper does not support dry-run or interactive operation; no archives, uploads or retention performed" ;;
    esac
  done
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
  have rmdir || die "rmdir is required to release backup name reservations"
  have mv || die "mv (GNU coreutils) is required to publish completed backup files"
  have realpath || die "realpath (GNU coreutils) is required to validate the backup staging location"
  [[ -d "${TENANTS_DIR}" ]] || die "Required tenants directory not found; no backup or retention performed"
  [[ -d "${CONTROL_PLANE_DIR}" ]] || die "Required control-plane directory not found; no backup or retention performed"
  reject_linked_source "${TENANTS_DIR}" "tenants"
  reject_linked_source "${CONTROL_PLANE_DIR}" "control-plane"
  reject_staging_in_source

  if [[ "${ENCRYPT}" == "true" ]]; then
    have age || die "PULSE_BACKUP_ENCRYPT=true but 'age' is not installed. Install: https://github.com/FiloSottile/age"
    [[ -n "${AGE_RECIPIENT}" ]] || die "PULSE_BACKUP_ENCRYPT=true but PULSE_BACKUP_AGE_RECIPIENT is not set"
  fi
  if [[ -n "${PULSE_RCLONE_REMOTE:-}" ]]; then
    reject_unconfirmed_remote_uploads
    have rclone || die "PULSE_RCLONE_REMOTE set but rclone is not installed"
  fi
}

# A timestamp is only second-resolution. Reserve its whole output set before
# opening any file, so overlapping helpers cannot both pass an existence check.
  # Keep the flat archive names and refuse existing files, directories and links.
release_backup_names() {
  [[ -n "${BACKUP_NAME_RESERVATION}" ]] || return 0
  rmdir -- "${BACKUP_NAME_RESERVATION}" || return 1
  BACKUP_NAME_RESERVATION=""
}

reserve_backup_names() {
  local timestamp="$1"
  shift
  local reservation="${BACKUP_DIR}/.pulse-backup-${timestamp}.lock"
  mkdir -- "${reservation}" 2>/dev/null ||
    die "Backup timestamp already reserved or reservation cannot be created; no archives or retention performed; retry at a later time; preserve existing files and check staging permissions"
  BACKUP_NAME_RESERVATION="${reservation}"
  trap 'release_backup_names || log "ERROR: backup name reservation could not be released"' EXIT
  trap 'exit 129' HUP
  trap 'exit 130' INT
  trap 'exit 143' TERM

  local output candidate
  for output in "$@"; do
    for candidate in "${output}" "${output}.partial"; do
      [[ ! -e "${candidate}" && ! -L "${candidate}" ]] ||
        die "Backup output or partial file already exists; refusing to overwrite it; no archives or retention performed; retry at a later time; preserve existing files and check staging permissions"
    done
  done
}

# Each rename stays in the same directory/filesystem. Never give a truncated
# producer stream a normal backup name. Publish only after all three producers
# succeed; a later publication/upload failure is still not a completed job.
publish_backup_file() {
  local output="$1"
  # -T prevents moving into an existing directory, and -n preserves even a
  # late destination collision. GNU mv can return zero after skipping with -n,
  # so the still-present partial file must also make publication fail closed.
  if ! mv -T -n -- "${output}.partial" "${output}" ||
    [[ -e "${output}.partial" || -L "${output}.partial" ]]; then
    die "Backup file publication failed; preserve existing files and .partial evidence; no uploads or retention performed"
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

  local parent_dir source_name
  # Command substitution strips trailing newlines, which are valid path bytes.
  # Preserve the configured parent/member with GNU coreutils' NUL delimiters.
  IFS= read -r -d '' parent_dir < <(dirname -z -- "${src_dir}") ||
    die "Cannot determine required ${label} parent directory; backup incomplete, retention not performed"
  IFS= read -r -d '' source_name < <(basename -z -- "${src_dir}") ||
    die "Cannot determine required ${label} archive member; backup incomplete, retention not performed"

  if [[ "${ENCRYPT}" == "true" ]]; then
    log "creating encrypted tarball: ${label} -> ${output_path}.age.partial"
    # Keep plaintext in the pipe, not in the backup directory. pipefail must
    # reject a failed tar even when age successfully encrypts its partial stream.
    # End option parsing: a source named --files-from=... is still a directory,
    # not authority to replace the archive's input with another file list.
    # Operator shell defaults must not silently exclude or rename recovery
    # data, dereference nested links, or suppress tar's failure reporting.
    TAR_OPTIONS='' tar -czf - -C "${parent_dir}" -- "${source_name}" |
      age -r "${AGE_RECIPIENT}" -o "${output_path}.age.partial"
    log "encrypted tarball staged: ${output_path}.age.partial ($(du -sh "${output_path}.age.partial" | cut -f1))"
    return 0
  fi

  log "creating tarball: ${label} -> ${output_path}.partial"
  TAR_OPTIONS='' tar -czf "${output_path}.partial" -C "${parent_dir}" -- "${source_name}"
  log "tarball staged: ${output_path}.partial ($(du -sh "${output_path}.partial" | cut -f1))"
}

# Emit metadata to the selected plaintext file or encryption pipe.
write_metadata() {
  local timestamp="$1"
  echo "timestamp: ${timestamp}"
  echo "hostname: $(hostname)"
  echo "encrypted: ${ENCRYPT}"
  # Docker's default COMMAND column may contain inline credentials. Keep an
  # explicit inventory rather than collecting commands, even without encryption.
  docker ps -a --no-trunc --format 'table {{.ID}}\t{{.Image}}\t{{.Names}}\t{{.Status}}' 2>/dev/null || echo "docker ps: unavailable"
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

  # The destination is opaque operator configuration and may be private. Our
  # own progress messages need only identify the generated backup file; do not
  # echo connection settings or destination names. Keep client diagnostics
  # unchanged for the operator, not as a general rclone-output redaction claim.
  log "uploading backup file: ${filename}"
  # An existing-destination skip can return exit zero. Require a transfer for
  # each of this job's three new files before local
  # retention may delete any previous recovery copy. Do not disable the user's
  # selection rules or force overwrites; a skipped copy must instead stop here.
  if ! rclone copyto "${file_path}" "${dest}" --error-on-no-transfer; then
    die "Remote upload failed or transferred no file; no retention performed; preserve local backup files and check rclone configuration and output"
  fi
  log "upload complete: ${filename}"
}

# Rotate local backups (keep last N days)
rotate_local() {
  [[ -d "${BACKUP_DIR}" ]] || return 0

  local pattern
  local names=()
  for pattern in "${BACKUP_FILE_PATTERNS[@]}"; do
    if (( ${#names[@]} > 0 )); then names+=(-o); fi
    names+=(-name "${pattern}" -o -name "${pattern}.partial")
  done
  log "rotating local backups (keeping last ${RETENTION_DAYS} days)"
  # Do not walk operator-created subdirectories or remove links/directories.
  # Metadata and failed-run partial evidence have the same age/ownership bounds.
  if ! find "${BACKUP_DIR}" -maxdepth 1 -type f \( "${names[@]}" \) \
    -mtime "+${RETENTION_DAYS}" -delete 2>/dev/null; then
    die "Local retention failed; new backup artifacts are retained, but cleanup may be incomplete"
  fi
}

# Rotate remote backups
rotate_remote() {
  if [[ -z "${PULSE_RCLONE_REMOTE:-}" ]]; then
    return 0
  fi

  local remote_dir="${PULSE_RCLONE_REMOTE}/pulse-cloud-tarballs"
  local pattern
  local filters=()
  for pattern in "${BACKUP_FILE_PATTERNS[@]}"; do
    filters+=(--include "/${pattern}")
  done
  log "rotating remote backups (${RETENTION_DAYS} day retention)"
  # Anchor each include at the remote root, then exclude everything else.
  # Sharing the prefix must not make unrelated objects eligible for deletion.
  if ! rclone delete "${remote_dir}" --min-age "${RETENTION_DAYS}d" \
    --max-depth 1 "${filters[@]}" --exclude '*' 2>/dev/null; then
    die "Remote retention failed; backup uploads completed, but cleanup may be incomplete"
  fi
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

  local tenants_tarball="${BACKUP_DIR}/pulse-backup-tenants-${timestamp}.tar.gz"
  local cp_tarball="${BACKUP_DIR}/pulse-backup-cp-${timestamp}.tar.gz"
  local meta_file="${BACKUP_DIR}/pulse-backup-meta-${timestamp}.txt"
  reserve_backup_names "${timestamp}" "${tenants_tarball}" "${tenants_tarball}.age" \
    "${cp_tarball}" "${cp_tarball}.age" "${meta_file}" "${meta_file}.age"

  # Backup tenants
  create_tarball "${TENANTS_DIR}" "${tenants_tarball}" "tenants"

  # Backup control plane
  create_tarball "${CONTROL_PLANE_DIR}" "${cp_tarball}" "control-plane"

  # Host/container identity is private backup data too. Stream it directly to
  # age, so encryption failure cannot leave a new plaintext inventory on disk.
  # Complete every encryption before the first upload or retention.
  if [[ "${ENCRYPT}" == "true" ]]; then
    meta_file="${meta_file}.age"
    write_metadata "${timestamp}" | age -r "${AGE_RECIPIENT}" -o "${meta_file}.partial"
  else
    write_metadata "${timestamp}" > "${meta_file}.partial"
  fi

  # No ordinary filenames are visible while any archive/encryption producer
  # may still fail. Metadata is published last, but is not an upload receipt.
  if [[ "${ENCRYPT}" == "true" ]]; then
    publish_backup_file "${tenants_tarball}.age"
    publish_backup_file "${cp_tarball}.age"
  else
    publish_backup_file "${tenants_tarball}"
    publish_backup_file "${cp_tarball}"
  fi
  publish_backup_file "${meta_file}"

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

  release_backup_names || die "Backup artifacts retained, but name reservation cleanup failed"
  log "=== tarball backup finished ==="
}

main "$@"

