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

## API Access

Pulse exposes the persistent metrics store via:

- `GET /api/metrics-store/stats`
- `GET /api/metrics-store/history`

These endpoints require authentication with the `monitoring:read` scope.

### History Query Parameters

`GET /api/metrics-store/history` supports:

- `resourceType` (required): `node`, `guest`, `storage`, `docker`, `dockerHost`
- `resourceId` (required): resource identifier
- `metric` (optional): `cpu`, `memory`, `disk`, etc. Omit to return all metrics for the resource.
- `range` (optional): `1h`, `6h`, `12h`, `24h`, `7d`, `30d`, `90d` (default `24h`)

Example:

```bash
curl -H "X-API-Token: $TOKEN" \
  "http://localhost:7655/api/metrics-store/history?resourceType=guest&resourceId=vm-100&range=7d&metric=cpu"
```

## Troubleshooting

- **No sparklines / empty history**: confirm the instance can write to the data directory and that `metrics.db` exists.
- **Large disk usage**: reduce polling frequency first. If you need tighter retention, adjust the tiered retention settings in `system.json` (advanced) and restart Pulse.
