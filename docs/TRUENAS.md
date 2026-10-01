# TrueNAS Integration

Pulse v6 includes first-class monitoring for **TrueNAS SCALE** and **TrueNAS CORE** systems. TrueNAS data flows through the unified resource model, appearing alongside Proxmox, Docker, Kubernetes, and host agent data throughout the UI.

## Quick Start

1. Go to **Settings → TrueNAS**.
2. Click **Add Connection**.
3. Enter the TrueNAS URL (e.g., `https://truenas.local`), the API key, and the
   username that owns the key.
4. Click **Test Connection** → **Save**.
5. Data appears within one polling cycle (~30 seconds).

## Creating a TrueNAS API Key

On your TrueNAS system:

1. Navigate to **Settings → API Keys** (SCALE) or **System → API Keys** (CORE).
2. Click **Add** and create a new key.
3. Copy the key value and paste it into Pulse.

> **Tip**: Pulse uses the supported JSON-RPC WebSocket API on TrueNAS 25.04
> and later. TrueNAS 26 removes the former REST API entirely. API keys inherit
> the linked user's roles, so enter the key owner's username and ensure that
> user can read the methods Pulse polls. Native app control actions require the
> corresponding TrueNAS app permissions. Recognized legacy SCALE and CORE
> releases continue to use the version-gated REST compatibility path.

## What Gets Monitored

| Data | Unified Page | Details |
|---|---|---|
| System info (hostname, version, uptime) | Infrastructure | CPU, memory, health status |
| Virtual machines | TrueNAS Overview | State, CPU, memory, boot mode, devices, and security flags from the TrueNAS VM API |
| Apps | TrueNAS Overview | Native app state, image/version, ports, volumes, networks, and runtime container details |
| ZFS Pools | Storage | Total/used/free capacity, pool status (ONLINE/DEGRADED/FAULTED) |
| ZFS Datasets | Storage | Used/available space, mount status, read-only flag |
| Physical Disks | Storage | Model, serial, size, transport type, rotational flag, temperature, and native SMART failure/counter evidence when TrueNAS reports it |
| ZFS Snapshots | Recovery | Dataset, creation time, size, referenced data |
| Replication Tasks | Recovery | Source/target datasets, direction, last run status |
| TrueNAS Alerts | Alerts | Native TrueNAS alert messages and severity levels |

## Unified Resource Mapping

TrueNAS resources are mapped into the unified resource model:

- **TrueNAS host** → appears as a resource with `source: truenas` on the **Infrastructure** page.
- **TrueNAS VMs** → appear as canonical `vm` workloads on the **TrueNAS** page.
- **TrueNAS apps** → appear as canonical `app-container` workloads on the **TrueNAS** page.
- **ZFS pools and datasets** → appear on the **Storage** page.
- **ZFS snapshots and replication** → appear on the **Recovery** page as recovery points.
- **TrueNAS alerts** → surfaced on the **Alerts** page alongside Proxmox and other platform alerts.

TrueNAS drive-health alerts that identify a specific disk remain disk-health
evidence after they are dismissed in TrueNAS. Dismissal acknowledges the
notification; Pulse continues to show the affected disk risk while TrueNAS
continues to report the underlying SMART condition. Other dismissed TrueNAS
alerts remain suppressed. When a supported SMART alert includes TrueNAS's
typed uncorrectable-error or spare-reserve argument, Pulse also projects that
value into the disk's SMART details. Pulse does not infer counters from alert
text, and current TrueNAS APIs do not expose every raw SMART attribute.

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

### Testing and adding a connection (API)

The **Settings → TrueNAS** flow is the simplest option. For an API client,
prepare the private Pulse header file as described in the
[API authentication guide](API.md). The Pulse API token authenticates to
Pulse; the separate TrueNAS API key belongs in the connection's JSON file.
Do not put either credential in shell arguments, URLs or a public thread.

Create or protect the connection file before opening it in a trusted editor:

```bash
umask 077
mkdir -p "$HOME/.config/pulse"
touch "$HOME/.config/pulse/truenas-connection.json"
chmod 600 "$HOME/.config/pulse/truenas-connection.json"
vi "$HOME/.config/pulse/truenas-connection.json"
```

In that file, enter a JSON object with `name`, `host` (the TrueNAS HTTPS URL),
`username` (the key owner) and `apiKey`. Keep both credential files private;
do not paste their contents into a terminal command or upload them.

Test the connection before saving it:

```bash
curl --fail-with-body --silent --show-error \
  --header "@$HOME/.config/pulse/api-header" \
  --header 'Content-Type: application/json' \
  --data-binary "@$HOME/.config/pulse/truenas-connection.json" \
  http://127.0.0.1:7655/api/truenas/connections/test
```

After checking the test response, save the connection with:

```bash
curl --fail-with-body --silent --show-error \
  --header "@$HOME/.config/pulse/api-header" \
  --header 'Content-Type: application/json' \
  --data-binary "@$HOME/.config/pulse/truenas-connection.json" \
  http://127.0.0.1:7655/api/truenas/connections
```

These Pulse URLs are loopback-only examples. For a remote Pulse server, use
its trusted HTTPS origin without disabling certificate verification.

## Troubleshooting

### "TrueNAS service unavailable"

- Check that the TrueNAS system is reachable from the Pulse server.
- Verify the URL uses `https://`. Current TrueNAS releases require TLS for
  remote API-key authentication.
- Verify that the configured username owns the API key and has permission to
  read the monitored methods.
- Use **Test Connection** in Pulse. The connection's transport diagnostics
  should report `jsonrpc-websocket` for TrueNAS 25.04 and later; do not test a
  current appliance through the removed `/api/v2.0` REST endpoints.

### No data appearing after adding connection
- Wait at least 30 seconds for the first poll cycle.
- **Test Connection** checks authentication and the selected transport, not
  successful collection of every metric. The legacy REST diagnostic is
  expected for recognized CORE 13 systems; it is not itself a connection error.
- Inspect a bounded local log excerpt for TrueNAS-related errors:
  ```bash
  journalctl -u pulse -n 100 --no-pager | grep -i truenas
  # or
  docker logs --tail 100 pulse 2>&1 | grep -i truenas
  ```
- If collection is safe, **Settings → Diagnostics → Export for GitHub
  (sanitized)** can provide connection evidence. Review the export or log
  excerpt before sharing: remove credentials, cookies, private hostnames,
  addresses and webhook URLs. Do not upload configuration or credential files.

### Inventory works but CPU, memory or History is missing

Services, uptime, hardware capacity and disk temperatures can be collected
without CPU or memory usage. A Host panel showing core count and total memory
does not establish a live reading; check the CPU/memory usage columns and
History series separately. A disk-utilisation line does not establish that
CPU, memory or network History is available.

Check whether the corresponding graphs work in the TrueNAS web UI. If Pulse
still shows missing readings, report the affected panels, running Pulse and
TrueNAS versions, and the relevant sanitized evidence above. Do not replace
an otherwise working key or clear stored History merely because a graph is
empty. When sharing a requested native graph response, include only its graph
name, `legend`, timing fields and one `data` row or error; omit authentication
messages and identifying details. Do not upload a full browser network capture.

### Stale TrueNAS data
- If TrueNAS data stops updating, the source status transitions to `stale` after ~120 seconds.
- Check TrueNAS connectivity and API key validity.
- In your signed-in Pulse browser session, open `/api/resources` on the same
  Pulse origin and inspect the entries with `platformType: "truenas"`. This
  uses your existing session without copying a token or cookie. Do not post
  the full response; use the sanitized export or a reviewed relevant excerpt.

### Disabling TrueNAS integration
Set `PULSE_ENABLE_TRUENAS=false` and restart Pulse. Existing connection data is preserved but polling stops.

## See Also

- [TrueNAS API Reference](https://www.truenas.com/docs/scale/api/) — current
  JSON-RPC transport, API-key, and TLS requirements
- [Configuration Guide](CONFIGURATION.md#truenas) — environment variables and setup
- [ZFS Monitoring](ZFS_MONITORING.md) — Proxmox-native ZFS pool monitoring
- [Recovery](RECOVERY.md) — TrueNAS snapshots in the recovery view
