# 🔄 Automatic Updates
Manage Pulse auto-updates on host-mode installations.

> **Note**: Docker/Kubernetes users should manage updates via their orchestrator.

## ⚙️ Components
| File | Purpose |
| :--- | :--- |
| `pulse-update.timer` | Daily check (02:00 + jitter). |
| `pulse-update.service` | Runs the update script. |
| `pulse-auto-update.sh` | Fetches release & restarts Pulse. |

## 🚀 Enable/Disable

### Via UI (Recommended)
**Settings → System → Updates**.

### Via CLI
```bash
# Enable
sudo jq '.autoUpdateEnabled=true' /etc/pulse/system.json > /tmp/system.json && sudo mv /tmp/system.json /etc/pulse/system.json
sudo systemctl enable --now pulse-update.timer

# Disable
sudo jq '.autoUpdateEnabled=false' /etc/pulse/system.json > /tmp/system.json && sudo mv /tmp/system.json /etc/pulse/system.json
sudo systemctl disable --now pulse-update.timer
```

## 🧪 Manual Run
Test the update process:
```bash
sudo systemctl start pulse-update.service
journalctl -u pulse-update -f
```

## 🔍 Observability
*   **History**: `curl -s http://localhost:7655/api/updates/history | jq`
*   **Logs**: `/var/log/pulse/update-*.log`

## ↩️ Rollback
If an update fails:
1.  Check logs: `/var/log/pulse/update-YYYYMMDDHHMMSS.log`.
2.  Use the **Rollback** action in **Settings → System → Updates** if available for your deployment type.
3.  If you need to pin a specific version, re-run the installer with a version:
    ```bash
    curl -fsSL https://github.com/rcourtman/Pulse/releases/latest/download/install.sh | \
      sudo bash -s -- --version vX.Y.Z
    ```
