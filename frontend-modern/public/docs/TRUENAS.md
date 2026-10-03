# TrueNAS Integration

Pulse v6 includes first-class monitoring for **TrueNAS SCALE** and **TrueNAS CORE** systems. TrueNAS data flows through the unified resource model, appearing alongside Proxmox, Docker, Kubernetes, and host agent data throughout the UI.

## Quick Start

1. Go to **Settings → Infrastructure**.
2. Click **Add infrastructure** and choose **TrueNAS**. Existing connections
   are listed under **Platform connections**.
3. Enter the TrueNAS URL (e.g., `https://truenas.local`), the API key, and the
   username that owns the key.
4. Click **Test Connection** → **Save**.
5. Data appears within one configured polling cycle (60 seconds by default).

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

| Data | Pulse page / tab | Details |
|---|---|---|
| System info (hostname, version, uptime) | TrueNAS → Overview | CPU, memory, health status |
| Virtual machines | TrueNAS → VMs | State, CPU, memory, boot mode, devices, and security flags from the TrueNAS VM API |
| Apps | TrueNAS → Apps | Native app state, image/version, ports, volumes, networks, and runtime container details |
| ZFS Pools | TrueNAS → Storage | Total/used/free capacity, pool status (ONLINE/DEGRADED/FAULTED) |
| ZFS Datasets | TrueNAS → Storage | Used/available space, mount status, read-only flag |
| Physical Disks | TrueNAS → Storage | Model, serial, size, transport type, rotational flag, temperature, and native SMART failure/counter evidence when TrueNAS reports it |
| ZFS Snapshots | TrueNAS → Protection | Dataset, creation time, size, referenced data |
| Replication Tasks | TrueNAS → Protection | Source/target datasets, direction, last run status |
| TrueNAS Alerts | Alerts | Native TrueNAS alert messages and severity levels |

## Unified Resource Mapping

TrueNAS resources are mapped into the unified resource model:

- **TrueNAS host** → appears as a resource with `source: truenas` under **TrueNAS → Overview** (`/truenas/overview`).
- **TrueNAS VMs** → appear as canonical `vm` workloads under **TrueNAS → VMs** (`/truenas/vms`).
- **TrueNAS apps** → appear as canonical `app-container` workloads under **TrueNAS → Apps** (`/truenas/apps`).
- **ZFS pools, datasets and physical disks** → appear under **TrueNAS → Storage** (`/truenas/storage`).
- **ZFS snapshots and replication** → appear under **TrueNAS → Protection** (`/truenas/protection`) as recovery points.
- **TrueNAS alerts** → surfaced on the **Alerts** page alongside Proxmox and other platform alerts.

TrueNAS drive-health alerts that identify a specific disk remain disk-health
evidence after they are dismissed in TrueNAS. Dismissal acknowledges the
notification; Pulse continues to show the affected disk risk while TrueNAS
continues to report the underlying SMART condition. Other dismissed TrueNAS
alerts remain suppressed. When a supported SMART alert includes TrueNAS's
typed uncorrectable-error or spare-reserve argument, Pulse also projects that
value into the disk's SMART details. Pulse does not infer counters from alert
text, and current TrueNAS APIs do not expose every raw SMART attribute.

The unified resource model is a backend contract, not a top-level
Infrastructure, Storage or Recovery menu. TrueNAS tabs follow the inventory
Pulse has collected; an absent tab is not proof of successful collection.
For missing or stale data, use the [polling checks](#stale-truenas-data) before
testing the connection or restarting.

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

The **Settings → Infrastructure → Platform connections** flow is the simplest option. For an API client,
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
- Allow one configured polling cycle (60 seconds by default), not a fixed
  30-second wait. Check that the saved connection is enabled.
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
empty.

If a maintainer requests a **CORE 13 native graph response**, use your existing
signed-in **TrueNAS** browser session, not the Pulse page:

1. In Firefox on macOS, open Network with **⌘⌥E**, then reload with Network
   recording. Select **WS**, select the WebSocket connection and open its
   **Response** pane.
2. Show **All** messages and clear any message search for `reporting.get_data`.
   The incoming reply does not need to repeat the method name, so that search
   can hide the evidence. Open the CPU or Memory graph on TrueNAS's Reporting
   page while recording.
3. Find the outgoing `"msg":"method"` request with `"name":"cpu"` or
   `"name":"memory"`; `cputemp` is temperature, not CPU usage. Each request has
   a different `id`, which is normal. Find the incoming `"msg":"result"` reply
   with the **same `id` as that request**, and expand its `result` (or `error`).
   The `legend`, timing fields and `data` belong to the response, not the
   outgoing request's `params`.
4. Share only the requested graph name, `legend`, timing fields and one `data`
   row, or the matching sanitized error. If no matching incoming reply is
   visible after the graph loads, report that instead of sending more requests.

Omit authentication messages, keys, cookies, private hostnames and addresses.
Do not upload a full browser network capture or paste code into the browser
console to collect this evidence.

### Stale TrueNAS data

A **Pending** or **Stale** badge is not an authentication diagnosis. Keep the
connection and key unchanged while checking whether ordinary polling still
collects data; a working connection can have missing telemetry or a stalled
collection step.

1. **Record the existing state before testing or restarting.** In your
   signed-in Pulse admin browser session, open `/api/truenas/connections` on
   the same Pulse origin. Inspect only the affected connection. This is a
   read-only request using your existing session; do not copy a token or cookie.
2. Note `enabled`, `poll.intervalSeconds`, `poll.lastAttemptAt`,
   `poll.lastSuccessAt`, `poll.consecutiveFailures`, `poll.lastError`, and
   `observed.collectedAt`. When present, also note `transport.mode`,
   `transport.reconnects` and `transport.lastError`. If your build does not
   expose a field, record that rather than interpreting its absence as success.
3. Compare the timestamps after two configured polling cycles, without
   pressing **Test Connection**, saving changes or restarting between reads.
   Slow requests can take longer than the interval. `lastAttemptAt` records a
   completed attempt, not an in-flight request; a frozen value alone cannot
   distinguish a blocked request from polling that has not run.

| Existing evidence | What it distinguishes and what to check next |
|---|---|
| `lastAttemptAt` advances, but `lastSuccessAt` and `observed.collectedAt` do not; failures increase | Polling returns failures. Use the actual error category and method to distinguish TLS, authentication, permissions and collection errors; a stale badge alone does not distinguish them. |
| `observed.collectedAt` advances, but some usage or History panels stay empty | Inventory is refreshing, not necessarily every metric. Follow the [missing-telemetry checks](#inventory-works-but-cpu-memory-or-history-is-missing); do not replace a working key to populate a chart. |
| No timestamps advance, or no completed attempt is recorded | Check that the connection and integration are enabled, then preserve the bounded local error above. This is not proof of an invalid key. |
| Reconnects or TrueNAS sign-ins increase while `observed.collectedAt` advances | Session turnover alone does not establish failed authentication or stopped polling. Record the timing and any existing close/error message separately from the data freshness. |

**Test Connection** checks connectivity and authentication, not a complete
poll. Testing a saved connection can update `lastAttemptAt` and `lastSuccessAt`
and reset its failure count without refreshing `observed.collectedAt`. Record
any manual test and its time separately; a successful test is not evidence
that missing data or stopped polling has recovered.

An `app.stats` error such as **Apps are not available** concerns Apps
collection, not necessarily the key. Check the existing Apps status in
TrueNAS, but do not stop Apps or reproduce a hang to gather evidence. Preserve
the existing error before changing anything. Do not repeatedly restart Pulse,
clear History, rotate keys or disable TLS verification as a diagnostic shortcut.

For a report, include the running Pulse and TrueNAS versions, configured poll
interval, affected panels, before/after timestamps and a reviewed, bounded
error excerpt from the local logs above. Remove private names and addresses
from errors. Do not post the full connection response, credential files,
authentication messages, cookies or a full browser network capture. If safe
inspection is unavailable, describe what you could observe without repeating
the failure or collecting a larger dump.

### Disabling TrueNAS integration
Set `PULSE_ENABLE_TRUENAS=false` and restart Pulse. Existing connection data is preserved but polling stops.

## See Also

- [TrueNAS API Reference](https://www.truenas.com/docs/scale/api/) — current
  JSON-RPC transport, API-key, and TLS requirements
- [Configuration Guide](CONFIGURATION.md#truenas) — environment variables and setup
- [ZFS Monitoring](ZFS_MONITORING.md) — Proxmox-native ZFS pool monitoring
- [Recovery data](RECOVERY.md) — the snapshot and replication data shown under **TrueNAS → Protection**
