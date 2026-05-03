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

Pulse keeps metrics history on disk by default. SSD-sensitive installs can move
only the metrics SQLite database without moving secrets or general config:

```bash
PULSE_METRICS_DB_PATH=/dev/shm/pulse/metrics.db
```

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

Using tmpfs makes metrics history ephemeral across restarts. It should not be
used for `/data`, because `/data` also contains config, encrypted credentials,
tokens, and other state that must remain durable.

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

- `resourceType` (required): `node`, `vm`, `container`, `storage`, `dockerHost`, `dockerContainer`
- `resourceId` (required): resource identifier (for guests use `instance:node:vmid`)
- `metric` (optional): `cpu`, `memory`, `disk`, etc. Omit to return all metrics for the resource.
- `range` (optional): `1h`, `6h`, `12h`, `24h`, `1d`, `7d`, `30d`, `90d` (default `24h`; duration strings also accepted)
- `maxPoints` (optional): Downsample to a target number of points

Example:

```bash
curl -H "X-API-Token: $TOKEN" \
  "http://localhost:7655/api/metrics-store/history?resourceType=vm&resourceId=pve1:node1:100&range=7d&metric=cpu"
```

> **License**: Requests beyond Community's `7d` floor require the paid `long_term_metrics` entitlement. Relay unlocks `14d`, Pro and legacy Pro+ unlock `90d`, and requests beyond the active tier's limit return `402 Payment Required`.
> **Aliases**: `guest` (VM/LXC) and `docker` (Docker container) are accepted, but persistent store data uses the canonical types above.

## Troubleshooting

- **No sparklines / empty history**: confirm the instance can write to the data directory and that `metrics.db` exists.
- **Large disk usage**: reduce polling frequency first. If you need tighter retention, adjust the tiered retention settings in `system.json` (advanced) and restart Pulse.
