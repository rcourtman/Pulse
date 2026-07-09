#!/usr/bin/env bash
set -euo pipefail

: "${DEMO_SERVER_HOST:?DEMO_SERVER_HOST is required}"
: "${DEMO_SERVER_SSH_KEY:?DEMO_SERVER_SSH_KEY is required}"

SSH_DIR="${HOME}/.ssh"
IDENTITY_FILE="${DEMO_SSH_IDENTITY_FILE:-${SSH_DIR}/id_ed25519}"
KNOWN_HOSTS_FILE="${DEMO_SSH_KNOWN_HOSTS_FILE:-${SSH_DIR}/known_hosts}"

is_ip_literal() {
  python3 - "$1" <<'PY'
import ipaddress
import sys

try:
    ipaddress.ip_address(sys.argv[1])
except ValueError:
    sys.exit(1)
sys.exit(0)
PY
}

mkdir -p "$SSH_DIR"
chmod 700 "$SSH_DIR"
printf '%s\n' "$DEMO_SERVER_SSH_KEY" > "$IDENTITY_FILE"
chmod 600 "$IDENTITY_FILE"

: > "$KNOWN_HOSTS_FILE"
keyscan_output="$(mktemp)"
keyscan_error="$(mktemp)"
trap 'rm -f "$keyscan_output" "$keyscan_error"' EXIT

host_needs_dns=true
if is_ip_literal "$DEMO_SERVER_HOST"; then
  host_needs_dns=false
  echo "Demo SSH host is an IP literal; skipping DNS resolution wait."
fi

MAX_SSH_SETUP_ATTEMPTS=18
for attempt in $(seq 1 "$MAX_SSH_SETUP_ATTEMPTS"); do
  if [ "$host_needs_dns" = "true" ] && ! getent hosts "$DEMO_SERVER_HOST" >/dev/null 2>&1; then
    echo "Demo SSH host is not resolvable yet on attempt ${attempt}/${MAX_SSH_SETUP_ATTEMPTS}."
  elif ssh-keyscan -T 10 -H "$DEMO_SERVER_HOST" > "$keyscan_output" 2>"$keyscan_error" && [ -s "$keyscan_output" ]; then
    cat "$keyscan_output" >> "$KNOWN_HOSTS_FILE"
    chmod 600 "$KNOWN_HOSTS_FILE"
    echo "Demo SSH host key captured."
    exit 0
  else
    echo "ssh-keyscan did not return demo host keys on attempt ${attempt}/${MAX_SSH_SETUP_ATTEMPTS}."
  fi

  if [ "$attempt" -lt "$MAX_SSH_SETUP_ATTEMPTS" ]; then
    sleep 10
  fi
done

echo "::error::Timed out waiting for the demo SSH host to become reachable and return host keys after Tailscale setup."
if command -v tailscale >/dev/null 2>&1; then
  tailscale status --peers=false || true
fi
if [ -s "$keyscan_error" ]; then
  sed 's/^/ssh-keyscan: /' "$keyscan_error" || true
fi
exit 1
