# Metrics History (Persistent)

Pulse persists metrics history to disk so trend views and sparklines survive restarts.

## Storage Location

Metrics history is stored in a SQLite database named `metrics.db` under the Pulse data directory:

- **systemd/LXC installs**: typically `/etc/pulse/metrics.db`
- **Docker/Kubernetes installs**: typically `/data/metrics.db`

## Retention Model (Tiered)

Pulse keeps multiple resolutions of the same data, which allows longer history without storing raw samples forever:

- **Raw** (high-resolution, short window)
- **Minute aggregates**
- **Hourly aggregates**
- **Daily aggregates**

Default retention values (subject to change) are:

- Raw: 2 hours
- Minute: 24 hours
- Hourly: 7 days
- Daily: 90 days
- Rollups: every 15 minutes by default, bounded so rollups still run well
  before raw samples expire

## Advanced: Retention Tuning

Do not change retention or polling merely because a chart is empty or writes
are high. First follow [Troubleshooting](#troubleshooting) and preserve the
original observations. Shorter retention removes history; slower polling also
changes how quickly monitoring can notice a problem.

Tiered retention is stored in `system.json` in the Pulse data directory:

- **systemd/LXC installs**: typically `/etc/pulse/system.json`
- **Docker/Kubernetes installs**: typically `/data/system.json`

Keys:

```json
{
  "metricsRetentionRawHours": 2,
  "metricsRetentionMinuteHours": 24,
  "metricsRetentionHourlyDays": 7,
  "metricsRetentionDailyDays": 90
}
```

After changing these values, restart Pulse.

## Advanced: Disk Write Tuning

This is a deliberate storage trade-off, not a repair for missing history or
unexplained writes. Changing the path opens a different history store; it does
not migrate the existing history. Keep the original durable data and establish
the writer using [bounded performance checks](TROUBLESHOOTING.md#excessive-cpu-writes-or-database-growth)
before changing storage.

Pulse keeps metrics history on disk by default. SSD-sensitive installs can move
only the metrics SQLite database without moving secrets or general config:

```bash
PULSE_METRICS_DB_PATH=/dev/shm/pulse/metrics.db
```

Use a dedicated directory owned by the account running Pulse. Do not place the
database directly in a shared parent such as `/tmp/metrics.db`: Pulse secures
the database directory to mode `0700` and will reject a directory owned by
another user. On systemd installs, `PrivateTmp=true` also means a path below
`/tmp` is private to the service and will not appear in the host's `/tmp`.

The default systemd sandbox permits `/dev/shm/pulse` and Pulse can create that
subdirectory itself. A dedicated mount elsewhere, such as `/mnt/ramdisk`, also
needs an explicit writable-path grant:

```bash
sudo install -d -o pulse -g pulse -m 0700 /mnt/ramdisk/pulse
sudo systemctl edit pulse
```

Add this drop-in (replace `pulse` with the installed unit name if different):

```ini
[Service]
ReadWritePaths=/mnt/ramdisk/pulse
```

Then set `PULSE_METRICS_DB_PATH=/mnt/ramdisk/pulse/metrics.db` in
`/etc/pulse/.env` and restart Pulse. `ProtectSystem=strict` deliberately keeps
paths outside the unit's writable allowlist read-only even when Unix ownership
would otherwise permit writes.

For Docker, mount a tmpfs at the selected directory and keep `/data` on a
persistent volume:

```yaml
services:
  pulse:
    environment:
      PULSE_METRICS_DB_PATH: /metrics-tmpfs/metrics.db
    tmpfs:
      - /metrics-tmpfs:size=512m,uid=1000,gid=1000,mode=0700
```

Using tmpfs loses that history when the RAM-backed mount is discarded, for
example on a host reboot or container recreation. A service restart alone is
not guaranteed to clear it. Do not use tmpfs for `/data`: it also contains
config, encrypted credentials, tokens, and other state that must remain durable.

The aggregation cadence can also be lengthened when an install prefers fewer,
larger rollup writes over more frequent smaller writes:

```bash
PULSE_METRICS_ROLLUP_INTERVAL=30m
```

Values below 5 minutes are ignored. Values longer than half of the raw-retention
window are capped by the metrics store so raw samples are still rolled up before
retention pruning can remove them.

## API Access

Pulse exposes the persistent metrics store via:

- `GET /api/metrics-store/stats`
- `GET /api/metrics-store/history`

These endpoints require authentication with the `monitoring:read` scope.

### History Query Parameters

`GET /api/metrics-store/history` supports:

- `resourceType` (required): `node`, `storage`, `agent`, `disk`, `k8s`, `vm`,
  `system-container`, `oci-container`, `app-container`, or `docker-host`
- `resourceId` (required): the source identifier used by that chart, not its
  display name (for Proxmox guests this is normally `instance:node:vmid`)
- `metric` (optional): `cpu`, `memory`, `disk`, etc. Omit to return all metrics for the resource.
- `range` (optional): `1h`, `6h`, `12h`, `24h`, `1d`, `7d`, `30d`, `90d` (default `24h`; duration strings also accepted)
- `maxPoints` (optional): Downsample to a target number of points

For a one-off check, open the affected resource's History panel in your signed-in
Pulse browser. If an API read is needed, use the path and query from that
panel's request on the same instance; do not extract its session cookie, use
**Copy as cURL**, or share a full network export. `/api/metrics-store/stats`
describes the store overall, not whether this resource has the requested metric.

For curl, follow [API authentication](API.md#-authentication) to prepare the
private header file with only `monitoring:read` and define its `pulse_api`
helper **in the same Bash session**. It is example code, not an installed Pulse
command. The default helper runs **on the Pulse host**; for remote access,
follow that guide's HTTPS-origin and verified-CA instructions. Then make one
read-only request (curl 7.76 or later):

```bash
pulse_api GET '/api/metrics-store/history?resourceType=vm&resourceId=pve1%3Anode1%3A100&range=7d&metric=cpu'
```

Replace the example type and ID with the chart's values, URL-encoding each
query value (`:` becomes `%3A`; an ID's `&` must be `%26`, not another query
parameter). Do not substitute a hostname or strip a provider prefix to guess
an ID. The helper saves each response, including an error body, in a new
owner-only file and prints only the HTTP status and file location. Open that
file privately; do not print or post it. HTTP 401, 402, 403 and server errors,
redirects, or incomplete transport are not an empty history or zero usage.
There is no automatic retry. Never put a token in a command, URL or report;
share only the relevant manually redacted error and bounded observations.

With `metric=cpu` (or another explicit metric), the response has a `points`
array. Omitting `metric` instead returns a `metrics` object keyed by metric
name, not the same single-series shape. Inspect timestamps and `source` too:
`store` is persisted history; `memory` or `live` fallback does not establish
durable historical coverage. An HTTP 200 with no points proves neither zero
usage nor successful collection, and a non-zero current value does not prove
every historical interval is present.

> **License**: Requests beyond Community's `7d` floor require the paid `long_term_metrics` entitlement. Relay unlocks `14d`, Pro and legacy Pro+ unlock `90d`, and requests beyond the active tier's limit return `402 Payment Required`.
> **Older examples**: `container`, `dockerHost`, `dockerContainer`, `guest` and
> `docker` are not current `resourceType` query values. Use the chart's current
> type rather than a display label or internal storage name.

## Troubleshooting

- **Missing or stale history**: retain the affected panel, metric, range,
  collection time and current server/agent versions. Check the connection's
  last normal poll or agent report in **Settings → Infrastructure**, without
  running Diagnostics, a guest-agent probe or a restart during backup freeze/thaw.
  Compare the panel's actual request, HTTP result, point timestamps and source.
  A valid connection test, another resource's graph or the existence of
  `metrics.db` does not establish this metric's collection or display.
- **Access or query error**: keep it separate from an empty result. Check the
  selected organisation, `monitoring:read` access, exact query type/ID and the
  requested range. A 402 is a history-range entitlement failure, not evidence
  that storage is empty; try an allowed range without buying another licence
  or resetting data.
- **Storage warning**: inspect a bounded existing startup/runtime error and
  the deployment's persistent mount and service-account access privately.
  Follow the [safe troubleshooting guide](TROUBLESHOOTING.md#excessive-cpu-writes-or-database-growth)
  for high writes or space pressure. Do not delete history, edit a database,
  shorten retention, move data to tmpfs or re-enrol a host as a diagnostic fix.
- **Report still needed**: share the affected metric/range, versions,
  whether normal collection advances, and a manually redacted error; keep
  resource identities, response files and databases private. Follow
  [Getting Help](TROUBLESHOOTING.md#-getting-help), not a full response dump.
