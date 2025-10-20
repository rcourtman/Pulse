#!/usr/bin/env bash
set -euo pipefail

if [[ $EUID -ne 0 ]]; then
  echo "Run as root." >&2
  exit 1
fi

SERVICE_USER="pulse-sensor"
SERVICE_GROUP="$SERVICE_USER"
HOME_DIR="/opt/pulse/sensor-proxy"
BIN_DIR="$HOME_DIR/bin"
CONFIG_DIR="$HOME_DIR/etc"
SSH_DIR="$HOME_DIR/.ssh"
LOG_DIR="/var/log/pulse/sensor-proxy"
SUDOERS_FILE="/etc/sudoers.d/pulse-sensor-proxy"
NOLOGIN_SHELL="/usr/sbin/nologin"

if id -u "$SERVICE_USER" >/dev/null 2>&1; then
  usermod --home "$HOME_DIR" --shell "$NOLOGIN_SHELL" "$SERVICE_USER"
else
  useradd --system --home "$HOME_DIR" --shell "$NOLOGIN_SHELL" --user-group "$SERVICE_USER"
fi

install -d -o "$SERVICE_USER" -g "$SERVICE_GROUP" -m 0750 "$HOME_DIR"
install -d -o "$SERVICE_USER" -g "$SERVICE_GROUP" -m 0750 "$BIN_DIR"
install -d -o root          -g "$SERVICE_GROUP" -m 0750 "$CONFIG_DIR"
install -d -o "$SERVICE_USER" -g "$SERVICE_GROUP" -m 0700 "$SSH_DIR"
install -d -o "$SERVICE_USER" -g "$SERVICE_GROUP" -m 0750 "$LOG_DIR"

TMP_SUDOERS="$(mktemp)"
trap 'rm -f "$TMP_SUDOERS"' EXIT

cat >"$TMP_SUDOERS" <<'EOF'
pulse-sensor ALL=(root) NOPASSWD: /usr/bin/sensors, /usr/sbin/ipmitool
EOF

install -o root -g root -m 0440 "$TMP_SUDOERS" "$SUDOERS_FILE"

if ! visudo -cf "$SUDOERS_FILE" >/dev/null; then
  echo "sudoers validation failed" >&2
  exit 1
fi

echo "User $(id "$SERVICE_USER")"
namei -om "$BIN_DIR" >/dev/null
namei -om "$CONFIG_DIR" >/dev/null
namei -om "$SSH_DIR" >/dev/null
namei -om "$LOG_DIR" >/dev/null

echo "Sudo privileges for $SERVICE_USER:"
sudo -l -U "$SERVICE_USER" || true

echo "pulse-sensor service account ready."
