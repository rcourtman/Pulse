#!/usr/bin/env bash
#
# HTTP helpers for Pulse shell scripts (downloads, API calls, GitHub queries).

set -euo pipefail

# Ensure common library is loaded when sourced directly.
if ! declare -F common::log_info >/dev/null 2>&1; then
  # shellcheck disable=SC1091
  source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"
fi

declare -g HTTP_DEFAULT_RETRIES="${HTTP_DEFAULT_RETRIES:-3}"
declare -g HTTP_DEFAULT_BACKOFF="${HTTP_DEFAULT_BACKOFF:-1 3 5}"

# http::detect_download_tool
# Emits the preferred download tool (curl/wget) or fails if neither exist.
http::detect_download_tool() {
  if command -v curl >/dev/null 2>&1; then
    printf 'curl\n'
    return 0
  fi
  if command -v wget >/dev/null 2>&1; then
    printf 'wget\n'
    return 0
  fi
  return 1
}

# http::download --url URL --output PATH [--insecure] [--quiet] [--header "Name: value"]
#               [--retries N] [--backoff "1 3 5"]
# Downloads the specified URL to PATH using curl or wget.
http::download() {
  local url=""
  local output=""
  local insecure=false
  local quiet=false
  local retries="${HTTP_DEFAULT_RETRIES}"
  local backoff="${HTTP_DEFAULT_BACKOFF}"
  local -a headers=()

  while (($#)); do
    case "$1" in
      --url)
        url="$2"
        shift 2
        ;;
      --output)
        output="$2"
        shift 2
        ;;
      --insecure)
        insecure=true
        shift
        ;;
      --quiet)
        quiet=true
        shift
        ;;
      --header)
        headers+=("$2")
        shift 2
        ;;
      --retries)
        retries="$2"
        shift 2
        ;;
      --backoff)
        backoff="$2"
        shift 2
        ;;
      *)
        common::log_warn "Unknown flag for http::download: $1"
        shift
        ;;
    esac
  done

  if [[ -z "${url}" || -z "${output}" ]]; then
    common::fail "http::download requires --url and --output"
  fi

  if common::is_dry_run; then
    common::log_info "[dry-run] Download ${url} -> ${output}"
    return 0
  fi

  local tool
  if ! tool="$(http::detect_download_tool)"; then
    common::fail "No download tool available (install curl or wget)"
  fi

  mkdir -p "$(dirname "${output}")"

  local -a cmd
  if [[ "${tool}" == "curl" ]]; then
    cmd=(curl -fL --connect-timeout 15)
    if [[ "${quiet}" == true ]]; then
      cmd+=(-sS)
    else
      cmd+=(--progress-bar)
    fi
    if [[ "${insecure}" == true ]]; then
      cmd+=(-k)
    fi
    local header
    for header in "${headers[@]}"; do
      cmd+=(-H "${header}")
    done
    cmd+=(-o "${output}" "${url}")
  else
    cmd=(wget --tries=3)
    if [[ "${quiet}" == true ]]; then
      cmd+=(-q)
    else
      cmd+=(--progress=bar:force)
    fi
    if [[ "${insecure}" == true ]]; then
      cmd+=(--no-check-certificate)
    fi
    local header
    for header in "${headers[@]}"; do
      cmd+=(--header="${header}")
    done
    cmd+=(-O "${output}" "${url}")
  fi

  local -a run_args=(--label "download ${url}")
  [[ -n "${retries}" ]] && run_args+=(--retries "${retries}")
  [[ -n "${backoff}" ]] && run_args+=(--backoff "${backoff}")

  common::run "${run_args[@]}" -- "${cmd[@]}"
}

# http::api_call --url URL [--method METHOD] [--token TOKEN] [--bearer TOKEN]
#                [--body DATA] [--header "Name: value"] [--insecure]
# Performs an API request and prints the response body.
http::api_call() {
  local url=""
  local method="GET"
  local token=""
  local bearer=""
  local body=""
  local insecure=false
  local -a headers=()

  while (($#)); do
    case "$1" in
      --url)
        url="$2"
        shift 2
        ;;
      --method)
        method="$2"
        shift 2
        ;;
      --token)
        token="$2"
        shift 2
        ;;
      --bearer)
        bearer="$2"
        shift 2
        ;;
      --body)
        body="$2"
        shift 2
        ;;
      --header)
        headers+=("$2")
        shift 2
        ;;
      --insecure)
        insecure=true
        shift
        ;;
      *)
        common::log_warn "Unknown flag for http::api_call: $1"
        shift
        ;;
    esac
  done

  if [[ -z "${url}" ]]; then
    common::fail "http::api_call requires --url"
  fi

  if common::is_dry_run; then
    common::log_info "[dry-run] API ${method} ${url}"
    return 0
  fi

  local tool
  if ! tool="$(http::detect_download_tool)"; then
    common::fail "No HTTP client available (install curl or wget)"
  fi

  local -a cmd
  if [[ "${tool}" == "curl" ]]; then
    cmd=(curl -fsSL)
    [[ "${insecure}" == true ]] && cmd+=(-k)
    [[ -n "${method}" ]] && cmd+=(-X "${method}")
    if [[ -n "${body}" ]]; then
      cmd+=(-d "${body}")
    fi
    if [[ -n "${token}" ]]; then
      headers+=("X-API-Token: ${token}")
    fi
    if [[ -n "${bearer}" ]]; then
      headers+=("Authorization: Bearer ${bearer}")
    fi
    local header
    for header in "${headers[@]}"; do
      cmd+=(-H "${header}")
    done
    cmd+=("${url}")
  else
    cmd=(wget -qO-)
    [[ "${insecure}" == true ]] && cmd+=(--no-check-certificate)
    [[ -n "${method}" ]] && cmd+=(--method="${method}")
    if [[ -n "${body}" ]]; then
      cmd+=(--body-data="${body}")
    fi
    if [[ -n "${token}" ]]; then
      cmd+=(--header="X-API-Token: ${token}")
    fi
    if [[ -n "${bearer}" ]]; then
      cmd+=(--header="Authorization: Bearer ${bearer}")
    fi
    local header
    for header in "${headers[@]}"; do
      cmd+=(--header="${header}")
    done
    cmd+=("${url}")
  fi

  common::run_capture --label "api ${method} ${url}" -- "${cmd[@]}"
}

# http::get_github_latest_release owner/repo
# Echoes the latest release tag for a GitHub repository.
http::get_github_latest_release() {
  local repo="${1:-}"
  if [[ -z "${repo}" ]]; then
    common::fail "http::get_github_latest_release requires owner/repo argument"
  fi

  local response
  response="$(http::api_call --url "https://api.github.com/repos/${repo}/releases/latest" --header "Accept: application/vnd.github+json" 2>/dev/null || true)"
  if [[ -z "${response}" ]]; then
    return 1
  fi

  if [[ "${response}" =~ \"tag_name\"[[:space:]]*:[[:space:]]*\"([^\"]+)\" ]]; then
    printf '%s\n' "${BASH_REMATCH[1]}"
    return 0
  fi

  if [[ "${response}" =~ \"name\"[[:space:]]*:[[:space:]]*\"([^\"]+)\" ]]; then
    printf '%s\n' "${BASH_REMATCH[1]}"
    return 0
  fi

  common::log_warn "Unable to parse GitHub release tag for ${repo}"
  return 1
}

# http::parse_bool value
# Parses truthy/falsy strings and prints canonical true/false.
http::parse_bool() {
  local input="${1:-}"
  local lowered="${input,,}"
  case "${lowered}" in
    1|true|yes|y|on)
      printf 'true\n'
      return 0
      ;;
    0|false|no|n|off|"")
      printf 'false\n'
      return 0
      ;;
  esac
  return 1
}
