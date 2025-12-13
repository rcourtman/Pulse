# Metrics History

Pulse 5.0 introduces persistent metrics history, allowing you to view historical resource usage data and trends over time.

## Features

- **Persistent Storage**: Metrics are saved to disk and survive restarts
- **Configurable Retention**: Set how long to keep different metric types
- **Trend Analysis**: View resource usage patterns over time
- **Spark Lines**: See at-a-glance trends in the dashboard

## Configuration

### Retention Settings

Configure retention periods in **Settings → General → Metrics History**:

| Metric Type | Default | Description |
|-------------|---------|-------------|
| **Host Metrics** | 7 days | CPU, memory, disk for hypervisors |
| **Guest Metrics** | 7 days | VM and container metrics |
| **Container Metrics** | 3 days | Docker/Podman container stats |
| **Aggregate Metrics** | 30 days | Cluster-wide summaries |

### Environment Variables

```bash
# Override via environment
PULSE_METRICS_HOST_RETENTION_DAYS=14
PULSE_METRICS_GUEST_RETENTION_DAYS=14
PULSE_METRICS_CONTAINER_RETENTION_DAYS=7
PULSE_METRICS_AGGREGATE_RETENTION_DAYS=60
```

## Storage

Metrics are stored in `/etc/pulse/data/metrics/` (or your configured data directory).

### Disk Usage

Approximate storage requirements:
- ~1 KB per resource per hour
- 10 hosts × 50 guests × 7 days ≈ 8 MB

### Database Maintenance

Pulse automatically:
- Compacts old data
- Prunes metrics beyond retention period
- Optimizes storage during low-usage periods

## API Access

Query historical metrics via the API:

```bash
# Get metrics for a specific resource
curl -H "X-API-Token: $TOKEN" \
  "http://localhost:7655/api/metrics/history?resource=vm-100&hours=24"

# Get aggregated cluster metrics
curl -H "X-API-Token: $TOKEN" \
  "http://localhost:7655/api/metrics/history?type=aggregate&days=7"
```

## Visualization

### Dashboard Sparklines
The dashboard shows 24-hour trend sparklines for each resource, updating in real-time.

### Detailed Charts
Click on any resource to see detailed historical charts with:
- Selectable time ranges (1h, 6h, 24h, 7d, 30d)
- Multiple metric overlays (CPU, memory, disk, network)
- Zoom and pan controls

## Troubleshooting

### Metrics not persisting
1. Check data directory permissions
2. Verify disk space availability
3. Check logs: `journalctl -u pulse | grep metrics`

### High disk usage
1. Reduce retention periods in Settings
2. Exclude low-value resources from history
3. Run manual cleanup: Settings → General → Clear Old Metrics
