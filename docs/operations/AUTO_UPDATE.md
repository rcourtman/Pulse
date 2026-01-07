# 🔄 Automatic Updates
Manage Pulse auto-updates on host-mode installations.

> **Note**: Docker/Kubernetes users should manage updates via their orchestrator.

## ⚙️ Components
| File | Purpose |
| :--- | :--- |
| `pulse-update.timer` | Daily check (02:00 + jitter). |
| `pulse-update.service` | Runs the update script. |
| `pulse-auto-update.sh` | Fetches release & restarts Pulse (`/opt/pulse/scripts/pulse-auto-update.sh`). |

**Release channel note:** the systemd timer script tracks GitHub `releases/latest` (stable). RC channel settings only affect the in-app update checker.

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
*   **History**: in-app updates are tracked via `GET /api/updates/history` (admin auth required) and stored in `update-history.jsonl` under `/etc/pulse` or `/data`. The systemd timer script does not record update history entries.
*   **Logs**: `journalctl -u pulse-update -f` or `journalctl -t pulse-auto-update -f` for timer runs. In-app updates write detailed logs to `/var/log/pulse/update-<timestamp>.log`.

## ↩️ Rollback
If an update fails:
1.  Check logs: `journalctl -u pulse-update -f` or `journalctl -t pulse-auto-update -f`.
2.  The timer script keeps a temporary backup under `/tmp/pulse-backup-<timestamp>` during the update; failures auto-restore from that backup and then clean it up.
3.  If you need to pin a specific version, re-run the installer with a version:
    ```bash
    curl -fsSL https://github.com/rcourtman/Pulse/releases/latest/download/install.sh | \
      sudo bash -s -- --version vX.Y.Z
    ```
    This installer updates the **Pulse server**. Agent updates use the `/install.sh` command generated in **Settings → Agents → Installation commands**.
