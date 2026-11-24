# Push and Heartbeat Checks

Push checks monitor scheduled jobs, backups, cron tasks, and services that cannot be probed directly by Pulse. Instead of Pulse polling an address, your job calls a private Pulse URL when it starts, finishes, or fails. If a ping does not arrive before the expected deadline plus grace window, Pulse marks the check as missed and alerts through your normal availability alert rules.

## When to use a push check

- Nightly or hourly backups (`restic`, `borg`, `rsync`, database dumps)
- Cron jobs and systemd timers behind NAT or strict firewalls
- Batch scripts or maintenance workers that have no listening port

## Creating a push check

1. Open **Availability** (or **Settings -> Monitoring -> Availability checks**) and click **Add check**.
2. Choose the **Scheduled job / backup (Push)** preset, or set **Probe** to **Push / Heartbeat**.
3. Give the check a clear **Name** (for example, `Nightly Proxmox backup`).
4. Choose a schedule type:
   - **Every interval**: expect a ping at least once every `N` seconds (for example, `86400` for daily or `3600` for hourly).
   - **Cron schedule (UTC)**: enter a standard 5-field UTC cron expression (`minute hour day-of-month month day-of-week`, such as `0 3 * * *` for 03:00 UTC daily).
5. Set a **Grace period (seconds)** (default `300` seconds) to allow normal job runtime and small clock jitter before Pulse marks a check as missed.
6. Click **Add service/device check**. The editor stays open and displays your private **Ping URL** and a ready-to-paste `curl` command.

A newly created push check starts in **Waiting for first ping** so it does not trigger false alerts before you paste the URL into your job.

## Ping endpoints

Every push check receives its own 256-bit random token. The ping endpoint accepts `GET`, `POST`, or `HEAD` requests without a Pulse session cookie or API token header:

| Endpoint | Effect |
|----------|--------|
| `/api/push/{token}` | Marks the check healthy (`up`). If `/start` was sent earlier, Pulse records the elapsed run duration automatically. |
| `/api/push/{token}/start` | Marks the start of a job run so Pulse can measure elapsed duration when the completion ping arrives. |
| `/api/push/{token}/fail` | Marks the check failed (`down`) immediately and records the optional message. |

### Optional query parameters

- `status`: `up` or `down` (for example, `/api/push/{token}?status=down`).
- `msg`: Short status note (for example, `?msg=uploaded+4.2GB`). For `POST` requests without `msg`, Pulse reads a short plain-text body up to 1 KiB. Bodies larger than 1 KiB are rejected with HTTP `413`.
- `ping`: Explicit run duration in milliseconds (for example, `?ping=1420`) when you measure runtime in your own script.

### Example shell wrapper

```bash
#!/usr/bin/env bash
set -euo pipefail

PING_URL="https://pulse.example.com/api/push/YOUR_TOKEN"

curl -fsS -m 10 --retry 3 "${PING_URL}/start" >/dev/null || true

if /usr/local/bin/run-backup.sh; then
  curl -fsS -m 10 --retry 3 "${PING_URL}?msg=backup+ok" >/dev/null
else
  curl -fsS -m 10 --retry 3 "${PING_URL}/fail?msg=backup+failed" >/dev/null || true
  exit 1
fi
```

## Rotating a Ping URL

If a Ping URL is exposed in a script or log:

1. Open the check's **Edit** modal from **Availability** or **Settings -> Monitoring -> Availability checks**.
2. Click **Rotate URL**, then click **Confirm rotate URL**.
3. Copy the new URL into your job. The previous token is revoked immediately and returns HTTP `404`.

## Security and limits

- Ping tokens are 256-bit random secrets stored encrypted in `availability_targets.enc` and compared in constant time.
- Ping tokens are only returned to authorized administrators viewing or editing target configuration. They are never included in `/api/resources`, WebSocket broadcasts, or read-only viewer sessions.
- Unknown, malformed, or rotated tokens return HTTP `404`.
- Inbound ping endpoints enforce request rate limits (returning HTTP `429`) and a 1 KiB `POST` body cap (returning HTTP `413`).
- Deadlines are evaluated locally on the Pulse server every 15 seconds. Push checks are passive and do not use external probe agents or an outbound **Test now** button.
