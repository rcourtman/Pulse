# Pulse Automatic Update Runbook

Automatic updates are handled by three systemd units that live on host-mode
installations:

| Component | Purpose | File |
| --- | --- | --- |
| `pulse-update.timer` | Schedules daily checks (02:00 + 0‑4 h jitter) | `/etc/systemd/system/pulse-update.timer` |
| `pulse-update.service` | Runs a single update cycle when triggered | `/etc/systemd/system/pulse-update.service` |
| `scripts/pulse-auto-update.sh` | Fetches release metadata, downloads binaries, restarts Pulse | `/opt/pulse/scripts/pulse-auto-update.sh` |

> Docker and Kubernetes deployments do **not** use this flow—manage upgrades via
> your orchestrator.

## Prerequisites

- `autoUpdateEnabled` must be `true` in `/var/lib/pulse/system.json` (or toggled in
  **Settings → System → Updates → Automatic Updates**).
- `pulse.service` must be healthy—the update service short-circuits if Pulse is
  not running.
- Host needs outbound HTTPS access to `github.com` and `objects.githubusercontent.com`.

## Enable or Disable

### From the UI
1. Navigate to **Settings → System → Updates**.
2. Toggle **Automatic Updates** on. The backend persists `autoUpdateEnabled:true`
   and surfaces a reminder to enable the timer.
3. On the host, run:
   ```bash
   sudo systemctl enable --now pulse-update.timer
   sudo systemctl status pulse-update.timer --no-pager
   ```
4. To disable later, toggle the UI switch off **and** run
   `sudo systemctl disable --now pulse-update.timer`.

### From the CLI only
```bash
# Opt in
sudo jq '.autoUpdateEnabled=true' /var/lib/pulse/system.json | sudo tee /var/lib/pulse/system.json >/dev/null
sudo systemctl daemon-reload
sudo systemctl enable --now pulse-update.timer

# Opt out
sudo jq '.autoUpdateEnabled=false' /var/lib/pulse/system.json | sudo tee /var/lib/pulse/system.json >/dev/null
sudo systemctl disable --now pulse-update.timer
```
> Editing `system.json` while Pulse is running is safe, but prefer the UI so
> validation rules stay in place.

## Trigger a Manual Run

Use this when testing new releases or after changing firewall rules:

```bash
sudo systemctl start pulse-update.service
sudo journalctl -u pulse-update -n 50
```

The oneshot service exits when the script finishes. A successful run logs the new
version and writes an entry to `update-history.jsonl`.

## Observability Checklist

- **Timer status**: `systemctl list-timers pulse-update`
- **History API**: `curl -s http://localhost:7655/api/updates/history | jq '.entries[0]'`
- **Raw log**: `/var/log/pulse/update-*.log` (referenced inside the history entry’s
  `log_path` field)
- **Journal**: `journalctl -u pulse-update -f`
- **Backups**: The script records `backup_path` in history (defaults to
  `/etc/pulse.backup.<timestamp>`). Ensure the path exists before acknowledging
  the rollout.

## Failure Handling & Rollback

1. Inspect the failing history entry:
   ```bash
   curl -s http://localhost:7655/api/updates/history?limit=1 | jq '.entries[0]'
   ```
   Common statuses: `failed`, `rolled_back`, `succeeded`.
2. Review `/var/log/pulse/update-YYYYMMDDHHMMSS.log` for the stack trace.
3. To revert, redeploy the previous release:
   ```bash
   sudo /opt/pulse/install.sh --version v4.30.0
   ```
   or use the main installer command from the update history output. The installer
   restores the `backup_path` recorded earlier when you choose **Rollback** in the
   UI.
4. Confirm Pulse is healthy (`systemctl status pulse.service`) and that
   `/api/updates/history` now contains a `rolled_back` entry referencing the same
   `event_id`.

## Troubleshooting

| Symptom | Resolution |
| --- | --- |
| `Auto-updates disabled in configuration` in journal | Set `autoUpdateEnabled:true` (UI or edit `system.json`) and restart the timer. |
| `pulse-update.timer` immediately exits | Ensure `systemd` knows about the units (`sudo systemctl daemon-reload`) and that `pulse.service` exists (installer may not have run with `--enable-auto-updates`). |
| `github.com` errors / rate limit | The script retries via the release redirect. For proxied environments set `https_proxy` before the service runs. |
| Update succeeds but Pulse stays on previous version | Check `journalctl -u pulse-update` for `restart failed`; Pulse only switches after the service restarts successfully. |
| Timer enabled but no history entries | Verify time has passed since enablement (timer includes random delay) or start the service manually to seed the first run. |

Document each run (success or rollback) in your change journal with the
`event_id` from `/api/updates/history` so you can cross-reference audit trails.
