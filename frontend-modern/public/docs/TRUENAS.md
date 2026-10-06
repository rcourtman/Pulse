# TrueNAS Integration

Pulse v6 includes first-class monitoring for **TrueNAS SCALE** and **TrueNAS CORE** systems. TrueNAS data flows through the unified resource model, appearing alongside Proxmox, Docker, Kubernetes, and host agent data throughout the UI.

## Quick Start

1. Go to **Settings → Infrastructure**.
2. Click **Add infrastructure** and choose **TrueNAS**. Existing connections
   are listed under **Platform connections**.
3. Enter the TrueNAS URL (e.g., `https://truenas.local`), the API key, and the
   username that owns the key.
4. Click **Test Connection**, then **Save**. The test reads system information
   on a separate connection; it does not validate inventory or metric collection.
5. Leave the saved connection enabled and allow at least one configured polling
   interval (60 seconds by default). Check that the appliance and expected
   inventory appear. If they do not, follow the [polling checks](#stale-truenas-data)
   without repeatedly testing or restarting; an elapsed interval is not proof
   that a poll completed.

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

Prefer **Settings → Infrastructure → Platform connections** for setup. These
API examples are for an authorised administrator using curl 7.76 or later,
not a way to diagnose stale polling. Testing makes a live system-information
request to TrueNAS; saving creates a connection and starts ordinary polling.
Do not use either during a backup, freeze/thaw or an unresponsive-host incident.

First prepare a private Pulse header file using the
[API token file procedure](API.md#api-token-recommended), with the admin access
needed for connection management. The Pulse token authenticates to Pulse;
the separate TrueNAS API key belongs in the connection's JSON file. Enter
credentials only in your local editor, never in arguments, URLs or a thread.

On a trusted machine, as the account running curl, prepare the connection
file below. Stop if either file-preparation step fails. This preserves an
existing file, restricts the directory and refuses symlinked credential paths
or a non-regular connection file before opening the editor:

```bash
(
  set -eu
  umask 077
  config_dir="$HOME/.config/pulse"
  connection_file="$config_dir/truenas-connection.json"
  if [ -L "$HOME/.config" ] || [ -L "$config_dir" ] || [ -L "$connection_file" ]; then
    printf 'Refusing a symlinked credential path.\n' >&2
    exit 1
  fi
  if [ -e "$connection_file" ] && [ ! -f "$connection_file" ]; then
    printf 'Connection file must be a regular file.\n' >&2
    exit 1
  fi
  mkdir -p "$config_dir"
  chmod 700 "$config_dir"
  touch "$connection_file"
  chmod 600 "$connection_file"
  vi "$connection_file"
)
```

In the editor, save a JSON object with `name`, `host` (the TrueNAS HTTPS URL),
`username` (the key owner) and `apiKey`. Keep both credential files private and
outside repositories and diagnostics. Do not upload them.

The following examples run **on the Pulse host**, using its direct loopback
address. They ignore curl configuration, do not follow redirects or retry,
and retain each response in a new private local file. Responses and errors can
contain identifying details or echoed credentials; review them locally, not in
a public terminal recording or thread. Keep `--disable` first so local curl
configuration cannot enable tracing or change the request.

Test once without saving:

```bash
(
  set -eu
  umask 077
  result_file=$(mktemp "$HOME/.config/pulse/truenas-test.XXXXXX")
  printf 'Private response file: %s\n' "$result_file"
  status=$(curl --disable --fail-with-body --silent --show-error \
    --connect-timeout 5 --max-time 20 --proto '=http' --noproxy 127.0.0.1 \
    --header "@$HOME/.config/pulse/api-header" \
    --header 'Content-Type: application/json' \
    --data-binary "@$HOME/.config/pulse/truenas-connection.json" \
    --output "$result_file" --write-out '%{http_code}' \
    http://127.0.0.1:7655/api/truenas/connections/test)
  printf 'HTTP %s\n' "$status"
  [ "$status" = 200 ]
)
```

Require exit 0, HTTP **200** and `success: true` in that private test response.
A redirect, HTTP error or `000` is not a successful test. Test success is not
inventory, metric or guest-thaw acceptance.

Only after reviewing the test response, create the connection once:

```bash
(
  set -eu
  umask 077
  result_file=$(mktemp "$HOME/.config/pulse/truenas-save.XXXXXX")
  printf 'Private response file: %s\n' "$result_file"
  status=$(curl --disable --fail-with-body --silent --show-error \
    --connect-timeout 5 --max-time 20 --proto '=http' --noproxy 127.0.0.1 \
    --header "@$HOME/.config/pulse/api-header" \
    --header 'Content-Type: application/json' \
    --data-binary "@$HOME/.config/pulse/truenas-connection.json" \
    --output "$result_file" --write-out '%{http_code}' \
    http://127.0.0.1:7655/api/truenas/connections)
  printf 'HTTP %s\n' "$status"
  [ "$status" = 201 ]
)
```

A successful save returns HTTP **201** and the new connection's `id`, not
proof of a complete poll. If saving times out or loses its response, it may
already have created the connection. Check **Platform connections** or
`/api/truenas/connections` in your existing signed-in Pulse browser before
trying again; do not blindly repeat POST and create duplicates. Continue with
the [ordinary polling checks](#stale-truenas-data) for collection acceptance.

For remote Pulse access, replace the loopback origin in the relevant command
with your trusted **HTTPS** origin and change `--proto '=http'` to
`--proto '=https'`. Keep certificate verification enabled. For a private CA,
add `--cacert /path/to/trusted-ca.pem` using a CA obtained independently from
your administrator, not from the failing connection. Do not add `--insecure`,
`--location`, verbose/trace flags or automatic retries. Share only the relevant
HTTP status and a manually redacted error, never the full response or headers.

## Troubleshooting

### "TrueNAS service unavailable"

This exact Pulse error (`truenas_unavailable`, HTTP **500** or **503**) means
Pulse's connection-management handler or configuration persistence is unavailable
for that request. It does **not** establish that the TrueNAS appliance is down,
that its API key is invalid, or that an ordinary poll failed.

Keep the existing failed request's path, HTTP status, error code and time from
your signed-in browser's **Developer tools → Network** panel; do not repeat a
save, delete or test to collect them. Inspect a bounded Pulse startup/service
log excerpt locally, using the readers below. Do not rotate the TrueNAS key,
recreate the saved connection or weaken TLS verification to clear this error.
Share only the Pulse version, request path, status/code and a manually redacted
error, not the full response, configuration, key or session cookie.

Distinguish these other results before choosing an action:

- **`truenas_disabled` / HTTP 404**: Pulse's TrueNAS integration is explicitly
  disabled. Check the intended `PULSE_ENABLE_TRUENAS` setting privately; this is
  not an appliance outage or an instruction to override an intentional opt-out.
- **`truenas_connection_failed` / HTTP 400**: a live connection test failed.
  Use its retained error to distinguish reachability, TLS, API-key ownership or
  read-permission problems. Keep HTTPS and certificate verification enabled;
  TrueNAS 25.04 and later use `jsonrpc-websocket`, while recognized CORE 13
  systems use legacy REST. Do not probe current TrueNAS through removed
  `/api/v2.0` endpoints or repeatedly test to diagnose ordinary collection.
- **Test succeeds but data is missing or stale**: Test reads system information
  on a separate connection, not inventory or metric collection. Use the
  [polling checks](#stale-truenas-data) and existing observation times instead
  of changing credentials or treating the test time as recovered freshness.

### No data appearing after adding connection
- Allow one configured polling cycle (60 seconds by default), not a fixed
  30-second wait. Check that the saved connection is enabled.
- **Test Connection** reads system information on a separate connection. Pools,
  datasets, disks and alerts can still fail during ordinary polling, even when
  the test succeeds. A successful test also does not establish live CPU, memory
  or History readings. The legacy REST diagnostic is expected for recognized
  CORE 13 systems; it is not itself a connection error.
- Preserve the existing error before testing or restarting. Follow the
  [polling checks](#stale-truenas-data) to distinguish a completed failure from
  missing or stale observations; a connection test is not a substitute.

If a local log excerpt is needed, run only the reader for your deployment on
the machine running **Pulse**, using an account authorised to read its logs.
For Proxmox LXC, run it **inside the Pulse container**, not on the Proxmox host
or the TrueNAS appliance. Substitute the actual service or container name
(`pulse-backend` on some older systemd installs), and adjust the time window to
the original incident. These commands read at most 100 records from the last
15 minutes; they do not follow logs, make API requests or change logging:

```bash
# systemd / Proxmox LXC
journalctl -u pulse --since '15 minutes ago' --lines 100 --no-pager
```

```bash
# Docker
docker logs --since 15m --tail 100 pulse
```

Inspect both output streams; Docker can write application logs to stderr.
Do not pipe the reader into `grep truenas`: it can hide a failed read behind a
matching partial line and omit relevant startup or storage errors. A nonzero
reader exit, denied read or missing service/container is a failed read, not
"no TrueNAS errors". Even a successful empty read is inconclusive: check the
window and selected instance, not the API key. Do not enable Debug or repeat
the failing action just to collect more logs.

Local logs are **not sanitised**. Share only the relevant timestamp, method,
HTTP status or error category and a manually redacted error, not the whole
excerpt. Remove credentials, cookies, secret URLs, private hostnames,
addresses and personal information, including anything echoed in an error.
Do not upload configuration or credential files.

In **Settings → Diagnostics**, an existing result's download buttons reuse it
without running the checks again; nothing is uploaded. Use **GitHub (review first)**
for an export intended for sharing, but sanitisation does not guarantee that
every error is safe to share. Open the downloaded file locally and review it
before sharing; keep **Full (private)** private. **Run Diagnostics** makes live
API and guest-agent requests:
do not run it during a backup, freeze/thaw or an unresponsive-host incident
merely to obtain an export. Keep the existing observations instead. See
[safe diagnostics collection](TROUBLESHOOTING.md#collect-diagnostics-safely).

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

**Test Connection** is not a complete poll. Some older builds, including Pulse
6.4.1 and 6.4.5, also let a saved-connection test update `lastAttemptAt` and
`lastSuccessAt` and reset its failure count without refreshing
`observed.collectedAt`. Record any manual test and its time separately, and
use the inventory observation time rather than the test time to check freshness.
A successful test is not evidence that missing data or stopped polling has
recovered.

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
