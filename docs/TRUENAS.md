# TrueNAS Integration

Pulse v6 includes first-class monitoring for **TrueNAS SCALE** and **TrueNAS CORE** systems. TrueNAS data flows through the unified resource model, appearing alongside Proxmox, Docker, Kubernetes, and host agent data throughout the UI.

## Quick Start

1. Go to **Settings → TrueNAS**.
2. Click **Add Connection**.
3. Enter the TrueNAS URL (e.g., `https://truenas.local`) and an API key.
4. Click **Test Connection** → **Save**.
5. Data appears within one polling cycle (~30 seconds).

## Creating a TrueNAS API Key

On your TrueNAS system:

1. Navigate to **Settings → API Keys** (SCALE) or **System → API Keys** (CORE).
2. Click **Add** and create a new key.
3. Copy the key value and paste it into Pulse.

> **Tip**: A read-only key is sufficient for monitoring. Pulse does not write to TrueNAS.

## What Gets Monitored

| Data | Unified Page | Details |
|---|---|---|
| System info (hostname, version, uptime) | Infrastructure | CPU, memory, health status |
| ZFS Pools | Storage | Total/used/free capacity, pool status (ONLINE/DEGRADED/FAULTED) |
| ZFS Datasets | Storage | Used/available space, mount status, read-only flag |
| Physical Disks | Storage | Model, serial, size, transport type, rotational flag |
| ZFS Snapshots | Recovery | Dataset, creation time, size, referenced data |
| Replication Tasks | Recovery | Source/target datasets, direction, last run status |
| TrueNAS Alerts | Alerts | Native TrueNAS alert messages and severity levels |

## Unified Resource Mapping

TrueNAS resources are mapped into the unified resource model:

- **TrueNAS host** → appears as a resource with `source: truenas` on the **Infrastructure** page.
- **ZFS pools and datasets** → appear on the **Storage** page.
- **ZFS snapshots and replication** → appear on the **Recovery** page as recovery points.
- **TrueNAS alerts** → surfaced on the **Alerts** page alongside Proxmox and other platform alerts.

Resources from TrueNAS can be filtered using the **source** filter on any page.

## Multiple TrueNAS Systems

Add as many TrueNAS connections as needed. Each connection is polled independently. Resources from all connected systems are merged into the unified view.

## Configuration

### Environment Variables

| Variable | Description | Default |
|---|---|---|
| `PULSE_ENABLE_TRUENAS` | Enable/disable TrueNAS integration | `true` |

### Storage

TrueNAS connection credentials are stored encrypted in `truenas.enc` in the Pulse data directory (`/etc/pulse` or `/data`).

## API Reference

All endpoints require admin authentication.

| Method | Endpoint | Description |
|---|---|---|
| `GET` | `/api/truenas/connections` | List all configured TrueNAS connections |
| `POST` | `/api/truenas/connections` | Add a new TrueNAS connection |
| `DELETE` | `/api/truenas/connections/{id}` | Remove a TrueNAS connection |
| `POST` | `/api/truenas/connections/test` | Test a connection before saving |

### Adding a connection (API)

```bash
curl -X POST http://localhost:7655/api/truenas/connections \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"nas-1","host":"https://truenas.local","api_key":"your-api-key"}'
```

### Testing a connection (API)

```bash
curl -X POST http://localhost:7655/api/truenas/connections/test \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"nas-1","host":"https://truenas.local","api_key":"your-api-key"}'
```

## Troubleshooting

### "TrueNAS service unavailable"
- Check that the TrueNAS system is reachable from the Pulse server.
- Verify the URL includes the protocol (`https://`).
- Test connectivity manually:
  ```bash
  curl -sk -H "Authorization: Bearer <api-key>" https://<truenas-ip>/api/v2.0/system/info
  ```

### No data appearing after adding connection
- Wait at least 30 seconds for the first poll cycle.
- Check Pulse logs for TrueNAS-related errors:
  ```bash
  journalctl -u pulse | grep -i truenas
  # or
  docker logs pulse | grep -i truenas
  ```

### Stale TrueNAS data
- If TrueNAS data stops updating, the source status transitions to `stale` after ~120 seconds.
- Check TrueNAS connectivity and API key validity.
- Verify with the API:
  ```bash
  curl -H "Authorization: Bearer $TOKEN" http://localhost:7655/api/resources \
    | jq '.resources[] | select(.platformType == "truenas")'
  ```

### Disabling TrueNAS integration
Set `PULSE_ENABLE_TRUENAS=false` and restart Pulse. Existing connection data is preserved but polling stops.

## See Also

- [Configuration Guide](CONFIGURATION.md#truenas) — environment variables and setup
- [ZFS Monitoring](ZFS_MONITORING.md) — Proxmox-native ZFS pool monitoring
- [Recovery](RECOVERY.md) — TrueNAS snapshots in the recovery view
