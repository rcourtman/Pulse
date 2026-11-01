#!/usr/bin/env bash
#
# Pulse container installer compatibility wrapper.
#
# Canonical v6 installs use install.sh with explicit docker flags.
# This wrapper preserves the old entrypoint name while routing to install.sh.

set -euo pipefail

usage() {
  cat <<'USAGE'
Pulse Container Agent Installer (v6 wrapper)

Usage:
  install-container-agent.sh [options]

Supported options:
  --url <url>             Pulse server URL (required when local install.sh is unavailable)
  --token <token>         API token
  --interval <duration>   Reporting interval (forwarded)
  --insecure              Skip TLS verification for self-signed certs
  --uninstall             Uninstall Pulse agent
  --help                  Show this help message

Notes:
  This wrapper forwards to install.sh with:
    --enable-docker --enable-host

  Use install.sh directly with --enable-host=false for an intentional
  workload-only deployment.
USAGE
}

URL=""
TOKEN=""
INTERVAL=""
INSECURE="false"
UNINSTALL="false"

unsupported_flag() {
  echo "[ERROR] Unsupported option for v6 wrapper: $1" >&2
  echo "[ERROR] Use install.sh directly for advanced install modes." >&2
  exit 1
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --help|-h)
      usage
      exit 0
      ;;
    --url)
      URL="${2:-}"
      shift 2
      ;;
    --token)
      TOKEN="${2:-}"
      shift 2
      ;;
    --interval)
      INTERVAL="${2:-}"
      shift 2
      ;;
    --insecure)
      INSECURE="true"
      shift
      ;;
    --uninstall)
      UNINSTALL="true"
      shift
      ;;
    --runtime|--runtime=*|--container-socket|--container-socket=*|--rootless|--system|--target|--target=*|--agent-path|--agent-path=*|--kube-include-all-pods|--kube-include-all-deployments|--purge)
      unsupported_flag "$1"
      ;;
    *)
      unsupported_flag "$1"
      ;;
  esac
done

forward_args=()
if [[ -n "$URL" ]]; then
  forward_args+=(--url "$URL")
fi
if [[ -n "$TOKEN" ]]; then
  forward_args+=(--token "$TOKEN")
fi
if [[ -n "$INTERVAL" ]]; then
  forward_args+=(--interval "$INTERVAL")
fi
if [[ "$INSECURE" == "true" ]]; then
  forward_args+=(--insecure)
fi
if [[ "$UNINSTALL" == "true" ]]; then
  forward_args+=(--uninstall)
else
  forward_args+=(--enable-docker --enable-host)
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [[ -f "$SCRIPT_DIR/install.sh" ]]; then
  exec "$SCRIPT_DIR/install.sh" "${forward_args[@]}"
fi

if [[ -z "$URL" ]]; then
  echo "[ERROR] --url is required when local install.sh is unavailable." >&2
  exit 1
fi

curl_flags=(-fsSL)
if [[ "$INSECURE" == "true" ]]; then
  curl_flags=(-kfsSL)
fi

# A pipe can execute an installer's first commands before curl discovers a
# truncated transfer. Admit the complete script before running any of it.
download_dir="$(mktemp -d "${TMPDIR:-/tmp}/pulse-container-installer.XXXXXX")" || {
  echo "[ERROR] Could not prepare a private installer download directory." >&2
  exit 1
}
download_file="$download_dir/install.sh"
cleanup_download() {
  local status=$?
  trap - EXIT
  if ! rm -f -- "$download_file" || ! rmdir -- "$download_dir"; then
    echo "[WARNING] Could not remove the temporary installer download." >&2
  fi
  exit "$status"
}
trap cleanup_download EXIT

# Keep downloaded code private even when the caller's umask is permissive.
if ! (umask 077; : > "$download_file"); then
  echo "[ERROR] Could not prepare the installer download file." >&2
  exit 1
fi
if ! curl "${curl_flags[@]}" --output "$download_file" -- "$URL/install.sh"; then
  echo "[ERROR] Could not download the complete install.sh; nothing was executed." >&2
  exit 1
fi
if [[ ! -s "$download_file" ]] || ! bash -n -- "$download_file" >/dev/null 2>&1; then
  echo "[ERROR] Downloaded install.sh is empty or invalid Bash; nothing was executed." >&2
  exit 1
fi

# Keep the stdin identity: the canonical installer must still obtain and verify
# its signed offline-removal copy, not trust this temporary transport file as a
# locally supplied installer. Do not exec; the wrapper still owns cleanup.
bash -s -- "${forward_args[@]}" < "$download_file"
