#!/usr/bin/env bash
set -euo pipefail

: "${DEMO_SERVER_HOST:?DEMO_SERVER_HOST is required}"

MODE="${1:-check}"
TCP_PORT="${DEMO_SERVER_PORT:-22}"
TCP_ATTEMPTS="${DEMO_TCP_ATTEMPTS:-6}"
TCP_RETRY_SECONDS="${DEMO_TCP_RETRY_SECONDS:-5}"

if [ "$MODE" != "check" ] && [ "$MODE" != "diagnose" ]; then
  echo "Usage: $0 [check|diagnose]" >&2
  exit 2
fi

# Only local daemon state is collected here. Never print tailnet addresses,
# names, tags, relay locations or raw CLI errors into public workflow logs.
# Success means the daemon is running, not that the demo is reachable.
print_safe_status() {
  local status_file status_result=0
  if ! command -v tailscale >/dev/null 2>&1; then
    echo "Tailscale CLI is not available."
    return 1
  fi

  status_file="$(mktemp)"
  if ! tailscale status --json >"$status_file" 2>/dev/null; then
    echo "Tailscale status JSON is unavailable."
    rm -f "$status_file"
    return 1
  fi

  python3 - "$DEMO_SERVER_HOST" "$status_file" <<'PY' || status_result=$?
import json
import sys

host = sys.argv[1]
try:
    with open(sys.argv[2], encoding="utf-8") as status_file:
        status = json.load(status_file)
except (ValueError, OSError):
    print("Tailscale status JSON is unavailable.")
    raise SystemExit(1)

states = {"NoState", "InUseOtherUser", "NeedsLogin", "NeedsMachineAuth", "Stopped", "Starting", "Running"}
state = status.get("BackendState") if isinstance(status, dict) else None
if not isinstance(state, str) or state not in states:
    print("Tailscale backend: unknown")
    raise SystemExit(1)
print(f"Tailscale backend: {state}")
if state != "Running":
    raise SystemExit(1)

target = None
peers = status.get("Peer")
for peer in peers.values() if isinstance(peers, dict) else ():
    if not isinstance(peer, dict):
        continue
    peer_ips = peer.get("TailscaleIPs")
    peer_ips = peer_ips if isinstance(peer_ips, list) else []
    peer_dns = peer.get("DNSName")
    peer_dns = peer_dns.rstrip(".") if isinstance(peer_dns, str) else ""
    if host in peer_ips or host.rstrip(".") == peer_dns:
        target = peer
        break

if target is None:
    print("Demo peer is not present in the runner peer map yet.")
else:
    print(
        "Demo peer state: "
        f"online={target.get('Online') is True} "
        f"active={target.get('Active') is True}"
    )
PY
  rm -f "$status_file"
  return "$status_result"
}

if [ "$MODE" = "diagnose" ]; then
  # Both callers use this after the setup action fails. A provider refusal must
  # not become another network attempt or a direct-host fallback diagnostic.
  print_safe_status || true
  echo "Diagnostic mode is local-only; demo connectivity and installed state remain unverified."
  exit 0
fi

if ! print_safe_status; then
  echo "::error::Tailscale setup has not established a running backend; no demo reachability probes attempted. This is not evidence that the demo host is down."
  exit 1
fi

# Probe output can contain private peer names/addresses or raw client errors.
# Retain the exit verdict, not that topology, in public workflow diagnostics.
if ! tailscale ping --c 3 --timeout 10s "$DEMO_SERVER_HOST" >/dev/null 2>&1; then
  echo "::error::Tailscale cannot reach the demo peer. Verify that the workflow tag is authorized to reach the demo host tag and that the peer is online."
  # Never try TCP after a failed tailnet readiness probe, or replay that probe
  # as a diagnostic. Preserve the failed result for the workflow owner.
  exit 1
fi

for attempt in $(seq 1 "$TCP_ATTEMPTS"); do
  if nc -z -w 5 "$DEMO_SERVER_HOST" "$TCP_PORT" >/dev/null 2>&1; then
    echo "Demo SSH transport is reachable over Tailscale."
    exit 0
  fi

  echo "Demo TCP/${TCP_PORT} is not reachable on attempt ${attempt}/${TCP_ATTEMPTS}."
  if [ "$attempt" -lt "$TCP_ATTEMPTS" ]; then
    sleep "$TCP_RETRY_SECONDS"
  fi
done

echo "::error::Tailscale reached the demo peer, but TCP/${TCP_PORT} remained closed. Verify sshd and the host firewall on tailscale0."
exit 1
