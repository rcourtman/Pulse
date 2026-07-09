#!/usr/bin/env bash
set -euo pipefail

: "${DEMO_SERVER_HOST:?DEMO_SERVER_HOST is required}"

MODE="${1:-check}"
TCP_PORT="${DEMO_SERVER_PORT:-22}"
TCP_ATTEMPTS="${DEMO_TCP_ATTEMPTS:-6}"
TCP_RETRY_SECONDS="${DEMO_TCP_RETRY_SECONDS:-5}"

print_safe_status() {
  local status_file
  if ! command -v tailscale >/dev/null 2>&1; then
    echo "Tailscale CLI is not available."
    return 0
  fi

  status_file="$(mktemp)"
  if ! tailscale status --json >"$status_file" 2>/dev/null; then
    echo "Tailscale status JSON is unavailable."
    rm -f "$status_file"
    return 0
  fi

  python3 - "$DEMO_SERVER_HOST" "$status_file" <<'PY' || true
import json
import sys

host = sys.argv[1]
try:
    with open(sys.argv[2], encoding="utf-8") as status_file:
        status = json.load(status_file)
except (json.JSONDecodeError, OSError):
    print("Tailscale status JSON is unavailable.")
    raise SystemExit(0)

self_node = status.get("Self") or {}
self_ips = [ip for ip in self_node.get("TailscaleIPs") or [] if ":" not in ip]
target = None
for peer in (status.get("Peer") or {}).values():
    peer_ips = peer.get("TailscaleIPs") or []
    peer_dns = (peer.get("DNSName") or "").rstrip(".")
    if host in peer_ips or host.rstrip(".") == peer_dns:
        target = peer
        break

print(f"Tailscale backend: {status.get('BackendState', 'unknown')}")
print(f"Runner Tailscale IPv4: {self_ips[0] if self_ips else 'unavailable'}")
if target is None:
    print("Demo peer is not present in the runner peer map yet.")
else:
    print(
        "Demo peer state: "
        f"online={bool(target.get('Online'))} "
        f"active={bool(target.get('Active'))} "
        f"relay={target.get('Relay') or 'none'}"
    )
PY
  rm -f "$status_file"
}

diagnose() {
  print_safe_status
  if command -v tailscale >/dev/null 2>&1; then
    tailscale ping --c 1 --timeout 5s "$DEMO_SERVER_HOST" || true
  fi
  nc -z -w 5 "$DEMO_SERVER_HOST" "$TCP_PORT" || true
}

if [ "$MODE" = "diagnose" ]; then
  diagnose
  exit 0
fi
if [ "$MODE" != "check" ]; then
  echo "Usage: $0 [check|diagnose]" >&2
  exit 2
fi

print_safe_status

if ! tailscale ping --c 3 --timeout 10s "$DEMO_SERVER_HOST"; then
  echo "::error::Tailscale cannot reach the demo peer. Verify that the workflow tag is authorized to reach the demo host tag and that the peer is online."
  diagnose
  exit 1
fi

for attempt in $(seq 1 "$TCP_ATTEMPTS"); do
  if nc -z -w 5 "$DEMO_SERVER_HOST" "$TCP_PORT"; then
    echo "Demo SSH transport is reachable over Tailscale."
    exit 0
  fi

  echo "Demo TCP/${TCP_PORT} is not reachable on attempt ${attempt}/${TCP_ATTEMPTS}."
  if [ "$attempt" -lt "$TCP_ATTEMPTS" ]; then
    sleep "$TCP_RETRY_SECONDS"
  fi
done

echo "::error::Tailscale reached the demo peer, but TCP/${TCP_PORT} remained closed. Verify sshd and the host firewall on tailscale0."
diagnose
exit 1
