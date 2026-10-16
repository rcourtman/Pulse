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
  --cacert <path>         Verified CA bundle (used for bootstrap and forwarded)
  --insecure             Forward to a local install.sh only; never disable bootstrap TLS
  --uninstall             Uninstall Pulse agent
  --help                  Show this help message

Notes:
  This wrapper forwards to install.sh with:
    --enable-docker --enable-host

  Use install.sh directly with --enable-host=false for an intentional
  workload-only deployment.

  Without a local install.sh, bootstrap requires authenticated HTTPS and a
  complete HTTP 200 download. Redirects are not followed. For a private CA,
  use --cacert with a bundle verified independently; do not disable TLS checks.
  Ambient curl configuration and proxies are ignored during bootstrap.
USAGE
}

URL=""
TOKEN=""
INTERVAL=""
CA_CERT=""
INSECURE="false"
UNINSTALL="false"

unsupported_flag() {
  echo "[ERROR] Unsupported option for v6 wrapper." >&2
  echo "[ERROR] Use install.sh directly for advanced install modes." >&2
  exit 1
}

require_value() {
  if [[ $# -ne 2 || -z "$2" || "$2" == --* ]]; then
    printf '[ERROR] %s requires a non-empty value.\n' "$1" >&2
    exit 1
  fi
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --help|-h)
      usage
      exit 0
      ;;
    --url)
      require_value "$1" "${2:-}"
      URL="${2:-}"
      shift 2
      ;;
    --token)
      require_value "$1" "${2:-}"
      TOKEN="${2:-}"
      shift 2
      ;;
    --interval)
      require_value "$1" "${2:-}"
      INTERVAL="${2:-}"
      shift 2
      ;;
    --cacert)
      require_value "$1" "${2:-}"
      CA_CERT="$2"
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
if [[ -n "$CA_CERT" ]]; then
  forward_args+=(--cacert "$CA_CERT")
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

if [[ "$INSECURE" == "true" ]]; then
  echo "[ERROR] Remote bootstrap requires TLS verification. Use --cacert or a trusted, saved local install.sh." >&2
  exit 1
fi

# This URL may include a reverse-proxy base path, but not embedded credentials,
# query/fragment data or controls. Curl still validates the HTTPS authority.
if [[ ! "$URL" =~ ^[Hh][Tt][Tt][Pp][Ss]:// || "$URL" =~ [[:space:][:cntrl:]] ||
      "$URL" == *\?* || "$URL" == *\#* || "$URL" == *\\* ]]; then
  echo "[ERROR] Remote bootstrap requires an https:// URL without credentials, query, fragment or whitespace." >&2
  exit 1
fi
authority="${URL#*://}"
authority="${authority%%/*}"
if [[ -z "$authority" || "$authority" == *@* ]]; then
  echo "[ERROR] Remote bootstrap requires an HTTPS server authority without embedded credentials." >&2
  exit 1
fi

# Never pipe a download to a shell: a partial script can execute even when
# pipefail eventually reports the failed transfer. Keep the file private and
# remove only this invocation's owned paths on every normal failure/success.
bootstrap_dir="$(mktemp -d "${TMPDIR:-/tmp}/pulse-container-bootstrap.XXXXXXXX")"
installer="$bootstrap_dir/install.sh"
cleanup_bootstrap() {
  if [[ -f "$installer" ]]; then rm -- "$installer"; fi
  rmdir -- "$bootstrap_dir"
}
trap cleanup_bootstrap EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

curl_flags=(--disable --fail --silent --show-error --globoff --proto '=https'
  --noproxy '*' --connect-timeout 5 --max-time 60 --output "$installer"
  --write-out '%{http_code}')
if [[ -n "$CA_CERT" ]]; then
  curl_flags+=(--cacert "$CA_CERT")
fi
if ! status="$(umask 077; curl "${curl_flags[@]}" --url "${URL%/}/install.sh" 2>/dev/null)"; then
  echo "[ERROR] Installer download failed; nothing was executed. Check HTTPS trust and the server address privately." >&2
  exit 1
fi
if [[ "$status" != 200 || ! -s "$installer" ]]; then
  echo "[ERROR] Installer download was not a complete, non-empty HTTP 200 response; nothing was executed." >&2
  exit 1
fi

bash "$installer" "${forward_args[@]}"
