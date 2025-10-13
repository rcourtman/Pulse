#!/usr/bin/env bash
# pulse-proxy-rotate-keys.sh
# Rotate pulse-sensor-proxy SSH keys with staging, verification, and rollback support.

set -euo pipefail

BASE_DIR="/var/lib/pulse-sensor-proxy"
ACTIVE_DIR="${BASE_DIR}/ssh"
POOL_DIR="${BASE_DIR}/ssh.d"
STAGING_DIR="${POOL_DIR}/next"
BACKUP_DIR="${POOL_DIR}/prev"
SOCKET_PATH="/run/pulse-sensor-proxy/pulse-sensor-proxy.sock"
SCRIPT_TAG="pulse-proxy-rotate"
SSH_KEY_TYPE="ed25519"
SSH_KEY_COMMENT="pulse-sensor-proxy"
SSH_KEY_FILE="id_${SSH_KEY_TYPE}"

dry_run=false
do_rollback=false

usage() {
  cat <<'EOF'
Usage: pulse-proxy-rotate-keys.sh [--dry-run] [--rollback]

Options:
  --dry-run   Walk through all steps without modifying state or contacting nodes.
  --rollback  Restore the previously active keypair (requires ssh.d/prev).
  -h, --help  Show this help.

Examples:
  ./pulse-proxy-rotate-keys.sh --dry-run
  ./pulse-proxy-rotate-keys.sh
  ./pulse-proxy-rotate-keys.sh --rollback
EOF
}

log_info()  { logger -t "${SCRIPT_TAG}" "INFO: $*";  printf '[INFO] %s\n' "$*"; }
log_warn()  { logger -t "${SCRIPT_TAG}" "WARN: $*";  printf '[WARN] %s\n' "$*"; }
log_error() { logger -t "${SCRIPT_TAG}" "ERROR: $*"; printf '[ERROR] %s\n' "$*" >&2; }

require_root() {
  if (( EUID != 0 )); then
    log_error "This script must be run as root."
    exit 1
  fi
}

require_cmds() {
  local missing=()
  for cmd in ssh-keygen ssh jq socat python3 stat mkdir; do
    if ! command -v "$cmd" >/dev/null 2>&1; then
      missing+=("$cmd")
    fi
  done
  if ((${#missing[@]} > 0)); then
    log_error "Missing required commands: ${missing[*]}"
    exit 1
  fi
}

parse_args() {
  while (($#)); do
    case "$1" in
      --dry-run) dry_run=true ;;
      --rollback) do_rollback=true ;;
      -h|--help) usage; exit 0 ;;
      *) log_error "Unknown option: $1"; usage; exit 1 ;;
    esac
    shift
  done
  if $dry_run && $do_rollback; then
    log_error "Cannot combine --dry-run and --rollback."
    exit 1
  fi
}

ensure_socket() {
  if [[ ! -S "$SOCKET_PATH" ]]; then
    log_error "Proxy socket not found at $SOCKET_PATH. Is pulse-sensor-proxy running?"
    exit 1
  fi
}

run_cmd() {
  if $dry_run; then
    log_info "[dry-run] $*"
  else
    "$@"
  fi
}

json_rpc() {
  local method=$1
  local params_json=${2:-"{}"}
  local response
  if $dry_run; then
    log_info "[dry-run] would call RPC ${method} with params ${params_json}"
    printf '{"success":true,"data":{}}'
    return 0
  fi

  response=$(SOCKET="$SOCKET_PATH" METHOD="$method" PARAMS="$params_json" python3 - <<'PY'
import json
import os
import socket
import sys
import uuid

sock_path = os.environ["SOCKET"]
method = os.environ["METHOD"]
params = json.loads(os.environ["PARAMS"]) if os.environ["PARAMS"] else {}
payload = {
    "correlation_id": str(uuid.uuid4()),
    "method": method,
    "params": params,
}

data = (json.dumps(payload) + "\n").encode()
with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as sock:
    sock.connect(sock_path)
    sock.sendall(data)
    sock.shutdown(socket.SHUT_WR)
    chunks = []
    while True:
        chunk = sock.recv(65536)
        if not chunk:
            break
        chunks.append(chunk)
    sys.stdout.write(b"".join(chunks).decode())
PY
) || {
    log_error "RPC '${method}' failed to execute."
    exit 1
  }
  echo "$response"
}

require_success() {
  local resp=$1
  local method=$2
  local ok
  ok=$(echo "$resp" | jq -r '.success // false')
  if [[ "$ok" != "true" ]]; then
    local err
    err=$(echo "$resp" | jq -r '.error // empty')
    log_error "RPC '${method}' failed: ${err:-unknown error}"
    exit 1
  fi
}

prepare_dirs() {
  for dir in "$BASE_DIR" "$POOL_DIR" "$STAGING_DIR"; do
    if $dry_run; then
      log_info "[dry-run] ensure directory $dir owned by pulse-proxy:pulse-proxy"
      continue
    fi
    mkdir -p "$dir"
    chown pulse-proxy:pulse-proxy "$dir"
    chmod 0750 "$dir"
  done
}

clean_staging() {
  if [[ -d "$STAGING_DIR" ]]; then
    if $dry_run; then
      log_info "[dry-run] would remove existing staging directory $STAGING_DIR"
    else
      rm -rf "$STAGING_DIR"
      mkdir -p "$STAGING_DIR"
      chown pulse-proxy:pulse-proxy "$STAGING_DIR"
      chmod 0750 "$STAGING_DIR"
    fi
  fi
}

generate_keypair() {
  local key_path="$STAGING_DIR/${SSH_KEY_FILE}"
  if $dry_run; then
    log_info "[dry-run] would generate new ${SSH_KEY_TYPE} keypair at $key_path"
    return
  fi
  clean_staging
  log_info "Generating new ${SSH_KEY_TYPE} keypair in staging..."
  ssh-keygen -t "$SSH_KEY_TYPE" -N '' -C "$SSH_KEY_COMMENT rotation $(date -u +%Y%m%dT%H%M%SZ)" -f "$key_path" >/dev/null
  chown pulse-proxy:pulse-proxy "$key_path" "${key_path}.pub"
  chmod 0600 "$key_path"
  chmod 0640 "${key_path}.pub"
}

ensure_cluster_keys() {
  local key_dir=$1
  local payload
  payload=$(jq -cn --arg dir "$key_dir" '{key_dir: $dir}')
  local resp
  resp=$(json_rpc "ensure_cluster_keys" "$payload")
  require_success "$resp" "ensure_cluster_keys"
  log_info "Proxy reported successful key distribution."
}

list_nodes() {
  local resp
  resp=$(json_rpc "register_nodes")
  require_success "$resp" "register_nodes"
  echo "$resp" | jq -r '.data.nodes[]?.name // empty' | sort -u
}

verify_nodes() {
  local key_file="$1"
  local -a bad_nodes=()
  local rc
  while read -r node; do
    [[ -z "$node" ]] && continue
    log_info "Verifying SSH access on ${node}..."
    if $dry_run; then
      log_info "[dry-run] would run ssh -i $key_file root@${node} sensors -j"
      continue
    fi
    if ssh -i "$key_file" -o BatchMode=yes -o StrictHostKeyChecking=no -o ConnectTimeout=10 "root@${node}" "sensors -j" >/dev/null 2>&1; then
      log_info "Verification succeeded for ${node}."
    else
      log_warn "Verification failed for ${node}."
      bad_nodes+=("$node")
    fi
  done < <(list_nodes)

  if ((${#bad_nodes[@]} > 0)); then
    log_error "Verification failed for: ${bad_nodes[*]}"
    exit 1
  fi
}

swap_keys() {
  local timestamp
  timestamp=$(date -u +%Y%m%dT%H%M%SZ)

  if $dry_run; then
    log_info "[dry-run] would rotate directories:"
    log_info "[dry-run]   mv ${BACKUP_DIR} ${POOL_DIR}/prev.${timestamp} (if exists)"
    log_info "[dry-run]   mv ${ACTIVE_DIR} ${BACKUP_DIR}"
    log_info "[dry-run]   mv ${STAGING_DIR} ${ACTIVE_DIR}"
    return
  fi

  log_info "Activating new keypair..."
  if [[ -d "$BACKUP_DIR" ]]; then
    mv "$BACKUP_DIR" "${POOL_DIR}/prev.${timestamp}"
  fi
  mv "$ACTIVE_DIR" "$BACKUP_DIR"
  mv "$STAGING_DIR" "$ACTIVE_DIR"
  chown -R pulse-proxy:pulse-proxy "$ACTIVE_DIR" "$BACKUP_DIR"
  chmod 0750 "$ACTIVE_DIR" "$BACKUP_DIR"
  chmod 0600 "$ACTIVE_DIR/${SSH_KEY_FILE}"
  chmod 0640 "$ACTIVE_DIR/${SSH_KEY_FILE}.pub"
  log_info "Key rotation complete. Previous keys stored at ${BACKUP_DIR}."
}

rollback_keys() {
  if [[ ! -d "$BACKUP_DIR" ]]; then
    log_error "No backup directory (${BACKUP_DIR}) present. Cannot rollback."
    exit 1
  fi
  local timestamp
  timestamp=$(date -u +%Y%m%dT%H%M%SZ)

  if $dry_run; then
    log_info "[dry-run] would rollback by swapping ${ACTIVE_DIR} with ${BACKUP_DIR}"
    return
  fi

  log_warn "Rolling back to previous keypair..."
  local failed_dir="${POOL_DIR}/failed.${timestamp}"
  if [[ -d "$ACTIVE_DIR" ]]; then
    mv "$ACTIVE_DIR" "$failed_dir"
  fi
  mv "$BACKUP_DIR" "$ACTIVE_DIR"
  chown -R pulse-proxy:pulse-proxy "$ACTIVE_DIR"
  chmod 0600 "$ACTIVE_DIR/${SSH_KEY_FILE}"
  chmod 0640 "$ACTIVE_DIR/${SSH_KEY_FILE}.pub"
  log_info "Rollback complete. Old keys preserved at ${failed_dir}."

  log_info "Re-pushing restored keypair to cluster nodes..."
  ensure_cluster_keys "$ACTIVE_DIR"
}

main() {
  parse_args "$@"
  require_root
  require_cmds

  if $do_rollback; then
    ensure_socket
    rollback_keys
    return
  fi

  prepare_dirs
  ensure_socket

  generate_keypair

  local staging_key="${STAGING_DIR}/${SSH_KEY_FILE}"
  if [[ ! -f "${staging_key}" && $dry_run == false ]]; then
    log_error "Staged private key missing at ${staging_key}"
    exit 1
  fi

  ensure_cluster_keys "$STAGING_DIR"
  verify_nodes "$staging_key"
  swap_keys

  log_info "Rotation workflow finished successfully."
}

main "$@"
