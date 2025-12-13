#!/usr/bin/env bash
#
# Systemd management helpers for Pulse shell scripts.

set -euo pipefail

# Ensure the common library is available when sourced directly.
if ! declare -F common::log_info >/dev/null 2>&1; then
  # shellcheck disable=SC1091
  source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"
fi

declare -g SYSTEMD_TIMEOUT_SEC="${SYSTEMD_TIMEOUT_SEC:-20}"

# systemd::safe_systemctl args...
# Executes systemctl with sensible defaults and optional timeout guards.
systemd::safe_systemctl() {
  if ! command -v systemctl >/dev/null 2>&1; then
    common::fail "systemctl not available on this host"
  fi

  local -a cmd
  systemd::__build_cmd cmd "$@"
  systemd::__wrap_timeout cmd

  local label="systemctl $*"
  common::run --label "${label}" -- "${cmd[@]}"
}

# systemd::detect_service_name [service...]
# Returns the first existing service name from the provided list.
systemd::detect_service_name() {
  local -a candidates=("$@")
  if ((${#candidates[@]} == 0)); then
    candidates=(pulse.service pulse-backend.service pulse-docker-agent.service pulse-hot-dev.service)
  fi

  local name
  for name in "${candidates[@]}"; do
    if systemd::service_exists "${name}"; then
      printf '%s\n' "$(systemd::__normalize_unit "${name}")"
      return 0
    fi
  done
  return 1
}

# systemd::service_exists service
# Returns success if the given service unit exists.
systemd::service_exists() {
  local unit
  unit="$(systemd::__normalize_unit "${1:-}")" || return 1

  if command -v systemctl >/dev/null 2>&1 && ! common::is_dry_run; then
    local -a cmd
    systemd::__build_cmd cmd "list-unit-files" "${unit}"
    systemd::__wrap_timeout cmd
    local output=""
    if output="$("${cmd[@]}" 2>/dev/null)"; then
      [[ "${output}" =~ ^${unit}[[:space:]] ]] && return 0
    fi
  fi

  local paths=(
    "/etc/systemd/system/${unit}"
    "/lib/systemd/system/${unit}"
    "/usr/lib/systemd/system/${unit}"
  )
  local path
  for path in "${paths[@]}"; do
    if [[ -f "${path}" ]]; then
      return 0
    fi
  done
  return 1
}

# systemd::is_active service
# Checks whether the given service is active.
systemd::is_active() {
  local unit
  unit="$(systemd::__normalize_unit "${1:-}")" || return 1

  if common::is_dry_run; then
    return 1
  fi

  if ! command -v systemctl >/dev/null 2>&1; then
    return 1
  fi

  local -a cmd
  systemd::__build_cmd cmd "is-active" "--quiet" "${unit}"
  systemd::__wrap_timeout cmd
  if "${cmd[@]}" >/dev/null 2>&1; then
    return 0
  fi
  return 1
}

# systemd::create_service /path/to/unit.service
# Reads unit file content from stdin and writes it to the supplied path.
systemd::create_service() {
  local target="${1:-}"
  local mode="${2:-0644}"
  if [[ -z "${target}" ]]; then
    common::fail "systemd::create_service requires a target path"
  fi

  local content
  content="$(cat)"

  if common::is_dry_run; then
    common::log_info "[dry-run] Would write systemd unit ${target}"
    common::log_debug "${content}"
    return 0
  fi

  mkdir -p "$(dirname "${target}")"
  printf '%s' "${content}" > "${target}"
  chmod "${mode}" "${target}"
  common::log_info "Wrote unit file: ${target}"
}

# systemd::enable_and_start service
# Reloads systemd, enables, and starts the given service.
systemd::enable_and_start() {
  local unit
  unit="$(systemd::__normalize_unit "${1:-}")" || common::fail "Invalid systemd unit name"

  systemd::safe_systemctl daemon-reload
  systemd::safe_systemctl enable "${unit}"
  systemd::safe_systemctl start "${unit}"
}

# systemd::restart service
# Safely restarts the given service.
systemd::restart() {
  local unit
  unit="$(systemd::__normalize_unit "${1:-}")" || common::fail "Invalid systemd unit name"
  systemd::safe_systemctl restart "${unit}"
}

# Internal: build systemctl command array.
systemd::__build_cmd() {
  local -n ref=$1
  shift
  ref=("systemctl" "--no-ask-password" "--no-pager")
  ref+=("$@")
}

# Internal: wrap command with timeout if necessary.
systemd::__wrap_timeout() {
  local -n ref=$1
  if ! command -v timeout >/dev/null 2>&1; then
    return
  fi
  if systemd::__should_timeout; then
    local -a wrapped=("timeout" "${SYSTEMD_TIMEOUT_SEC}s")
    wrapped+=("${ref[@]}")
    ref=("${wrapped[@]}")
  fi
}

# Internal: determine if we are in a container environment.
systemd::__should_timeout() {
  if [[ -f /run/systemd/container ]]; then
    return 0
  fi
  if command -v systemd-detect-virt >/dev/null 2>&1; then
    if systemd-detect-virt --quiet --container; then
      return 0
    fi
  fi
  return 1
}

# Internal: normalize service names to include .service suffix.
systemd::__normalize_unit() {
  local unit="${1:-}"
  if [[ -z "${unit}" ]]; then
    return 1
  fi
  if [[ "${unit}" != *.service ]]; then
    unit="${unit}.service"
  fi
  printf '%s\n' "${unit}"
}
