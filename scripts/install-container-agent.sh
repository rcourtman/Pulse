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
    --enable-docker --disable-host
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
  forward_args+=(--enable-docker --disable-host)
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

curl "${curl_flags[@]}" "$URL/install.sh" | bash -s -- "${forward_args[@]}"
