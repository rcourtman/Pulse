#!/usr/bin/env bash
set -euo pipefail

FORCE_COMMAND="${FORCE_COMMAND:-/opt/pulse/bin/sensor-proxy-wrapper}"
CONF_PATH="/etc/ssh/sshd_config.d/pulse-sensor-proxy.conf"

if [[ ! -x "$FORCE_COMMAND" ]]; then
  echo "Error: FORCE_COMMAND '$FORCE_COMMAND' not found or not executable" >&2
  exit 1
fi

TMP_CONF="$(mktemp)"
trap 'rm -f "$TMP_CONF"' EXIT

cat >"$TMP_CONF" <<EOF
# Hardening for Pulse sensor proxy access
PasswordAuthentication no
KbdInteractiveAuthentication no
ChallengeResponseAuthentication no
PermitRootLogin no
AllowAgentForwarding no
AllowTcpForwarding no
PermitTunnel no
X11Forwarding no
PermitUserEnvironment no
ForceCommand $FORCE_COMMAND
EOF

install -o root -g root -m 0644 "$TMP_CONF" "$CONF_PATH"

sshd -t
systemctl reload sshd

echo "sshd hardening applied to $CONF_PATH"

# Verification
echo "Verifying hardening settings:"
sshd -T | grep -E 'passwordauthentication|allowagentforwarding|allowtcpforwarding|x11forwarding|permittunnel' || true
