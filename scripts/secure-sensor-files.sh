#!/usr/bin/env bash
set -euo pipefail

if [[ $EUID -ne 0 ]]; then
  echo "Run as root." >&2
  exit 1
fi

SERVICE_USER="pulse-sensor"
SERVICE_GROUP="$SERVICE_USER"
HOME_DIR="/opt/pulse/sensor-proxy"
BIN_PATH="$HOME_DIR/bin/pulse-sensor-proxy"
SSH_DIR="$HOME_DIR/.ssh"
PRIVATE_KEY="$SSH_DIR/id_ed25519"
PUBLIC_KEY="$SSH_DIR/id_ed25519.pub"
KNOWN_HOSTS="$SSH_DIR/known_hosts"
LOG_DIR="/var/log/pulse/sensor-proxy"
LOG_FILE="$LOG_DIR/proxy.log"
AUDIT_LOG="$LOG_DIR/audit.log"

umask 077

install -d -o "$SERVICE_USER" -g "$SERVICE_GROUP" -m 0700 "$SSH_DIR"

if [[ ! -f "$PRIVATE_KEY" ]]; then
  sudo -u "$SERVICE_USER" ssh-keygen -t ed25519 -N '' -C "pulse-sensor@$(hostname -f)" -f "$PRIVATE_KEY"
else
  chown "$SERVICE_USER:$SERVICE_GROUP" "$PRIVATE_KEY"
  chmod 0600 "$PRIVATE_KEY"
fi

chown "$SERVICE_USER:$SERVICE_GROUP" "$PRIVATE_KEY"
chmod 0600 "$PRIVATE_KEY"

if [[ -f "$PUBLIC_KEY" ]]; then
  chown "$SERVICE_USER:$SERVICE_GROUP" "$PUBLIC_KEY"
  chmod 0640 "$PUBLIC_KEY"
else
  sudo -u "$SERVICE_USER" ssh-keygen -y -f "$PRIVATE_KEY" >"$PUBLIC_KEY"
  chown "$SERVICE_USER:$SERVICE_GROUP" "$PUBLIC_KEY"
  chmod 0640 "$PUBLIC_KEY"
fi

if [[ ! -f "$KNOWN_HOSTS" ]]; then
  install -o "$SERVICE_USER" -g "$SERVICE_GROUP" -m 0640 /dev/null "$KNOWN_HOSTS"
else
  chown "$SERVICE_USER:$SERVICE_GROUP" "$KNOWN_HOSTS"
  chmod 0640 "$KNOWN_HOSTS"
fi

install -d -o "$SERVICE_USER" -g "$SERVICE_GROUP" -m 0750 "$LOG_DIR"

for log_path in "$LOG_FILE" "$AUDIT_LOG"; do
  if [[ ! -f "$log_path" ]]; then
    install -o "$SERVICE_USER" -g "$SERVICE_GROUP" -m 0640 /dev/null "$log_path"
  else
    chown "$SERVICE_USER:$SERVICE_GROUP" "$log_path"
    chmod 0640 "$log_path"
  fi

  if command -v chattr >/dev/null 2>&1; then
    if ! lsattr "$log_path" 2>/dev/null | grep -q 'a'; then
      chattr +a "$log_path" || echo "Warning: could not set append-only attribute on $log_path" >&2
    fi
  else
    echo "Warning: chattr not available; skipping append-only for $log_path." >&2
  fi
done

if [[ -f "$BIN_PATH" ]]; then
  chown root:"$SERVICE_GROUP" "$BIN_PATH"
  chmod 0750 "$BIN_PATH"
fi

echo "SSH artifacts:"
ls -l "$PRIVATE_KEY" "$PUBLIC_KEY" "$KNOWN_HOSTS"

echo "Log files:"
ls -l "$LOG_FILE" "$AUDIT_LOG"
if command -v lsattr >/dev/null 2>&1; then
  lsattr "$LOG_FILE" "$AUDIT_LOG" || true
fi

if [[ -f "$BIN_PATH" ]]; then
  echo "Binary permissions:"
  ls -l "$BIN_PATH"
fi

echo "sensor proxy file permissions secured."
