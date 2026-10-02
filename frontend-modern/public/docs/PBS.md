# Proxmox Backup Server (PBS) Integration

This guide explains how to connect Pulse to your Proxmox Backup Server for comprehensive backup monitoring.

## Two Ways to Monitor PBS Backups

Pulse can monitor PBS backups in two ways:

### 1. Direct PBS Connection (Recommended)

Connect to the PBS API with a dedicated read-only token. An agent is not
required for these API-backed readings:

**Benefits:**
- ✅ Deduplication factor and storage efficiency stats
- ✅ PBS server health monitoring (CPU, memory, uptime)
- ✅ Datastore usage and namespace hierarchy
- ✅ Sync, verify, prune, and GC job status
- ✅ Backup owner information
- ✅ Faster queries (no PVE proxy overhead)

### 2. PVE Passthrough (Automatic)

If your PVE cluster has PBS storage configured, Pulse automatically fetches backup data through the PVE API.

**Limitations:**
- ❌ No deduplication stats
- ❌ No PBS server health data
- ❌ No job monitoring
- ❌ Can be slow for encrypted PBS storage
- ❌ Limited metadata per backup

**Recommendation:** Start with a direct API connection when you need PBS
datastore, job or server status beyond the PVE passthrough. Add a host agent
only for extra local telemetry such as SMART and temperatures; see
[Agent Security](AGENT_SECURITY.md#proxmox-deployment-choices).

---

## Setting Up Direct PBS Connection

### Method 1: API-Only Connection (Recommended)

Use a dedicated PBS monitoring user and token, not a root password. On a new
setup, run these commands in an administrator shell **on the PBS server**:

```bash
proxmox-backup-manager user create pulse-monitor@pbs --comment "Pulse monitoring"
proxmox-backup-manager user generate-token pulse-monitor@pbs pulse-token
proxmox-backup-manager acl update / Audit --auth-id pulse-monitor@pbs
proxmox-backup-manager acl update / Audit --auth-id 'pulse-monitor@pbs!pulse-token'
```

PBS tokens have their own permissions, limited by their owning user's
permissions. Grant `Audit` to **both the user and the token**. If either already
exists, inspect and reuse it instead of deleting it or rerunning token creation.
Do not disable privilege separation or grant `Admin` to work around a missing
permission.

The token secret is shown once. Save it privately and enter it only in Pulse's
**Token Value** field; do not paste it into a shell command, URL, screenshot or
issue report. Keep any terminal recording containing that output private.

In Pulse:
1. Open **Settings → Infrastructure → Add infrastructure** and choose
   **Proxmox Backup Server**.
2. Enter the PBS HTTPS URL, normally `https://pbs.example.com:8007`.
3. Select **API Token** and **Manual Token Setup**.
4. Enter **Token ID** `pulse-monitor@pbs!pulse-token` and its secret in
   **Token Value**.
5. Keep certificate verification enabled. For a self-signed certificate, use
   an independently verified **SSL Fingerprint**; see the TLS guidance below.
6. Test and save the connection. Check that the expected datastores and backups
   are visible, not just that the connection test succeeds.

### Method 2: Optional Host Agent

Use this only when you also need host-local telemetry. The Linux agent normally
runs as root; it is not required to fix API authentication or missing backup
permissions. Review [Agent Security](AGENT_SECURITY.md) first.

Create a separate Pulse token in **API Access**, using the **Agent host** preset
for reporting, configuration reads and agent management. This is a **Pulse**
token, not the **PBS** token used above. Do not enable command execution just
for monitoring.

On the PBS host, enter an administrator root shell before preparing the private
file below. In the editor, save only the Pulse agent token, with no header or
quotes; never put the secret in a command argument:

```bash
umask 077
mkdir -p "$HOME/.config/pulse"
chmod 700 "$HOME/.config/pulse"
touch "$HOME/.config/pulse/pbs-agent-token"
chmod 600 "$HOME/.config/pulse/pbs-agent-token"
vi "$HOME/.config/pulse/pbs-agent-token"
```

Download the agent installer from **your Pulse server's HTTPS address**, then
inspect the saved script before running it. Replace the example Pulse URL in
both commands. Stop if the download fails; run the next command only after a
successful download and inspection. Do not substitute GitHub's top-level `install.sh`: that installs
the Pulse server, not the agent.

```bash
curl --fail --silent --show-error \
  --output "$HOME/.config/pulse/pbs-agent-install.sh" \
  https://pulse.example.com/install.sh
```

```bash
bash "$HOME/.config/pulse/pbs-agent-install.sh" \
  --url https://pulse.example.com \
  --token-file "$HOME/.config/pulse/pbs-agent-token" \
  --enable-proxmox --proxmox-type pbs --enable-docker=false
```

Do not bypass certificate checks to fetch or run the installer. Use a trusted
Pulse certificate or a CA file you verified separately. See
[Unified Agent Setup](UNIFIED_AGENT.md) for installer trust and other profiles.
After installation, check a fresh agent report and the host's
`pulse-agent --version`; installation or hardware capacity alone does not prove
that every CPU, memory or History reading is available. Keep the token file
private and remove the bootstrap copy when no longer needed; do not remove the
installed agent's runtime credential.

---

## PBS Permissions

The Pulse monitoring user needs minimal permissions:

| Role | Path | Purpose |
|------|------|---------|
| `Audit` | `/` | Read-only access to all datastores, backups, and server status |

The `Audit` role provides:
- List datastores and their usage
- View backup groups and snapshots
- Read server status (CPU, memory, uptime)
- View job history and status

It does **not** allow:
- Creating, modifying, or deleting backups
- Running backup/restore operations
- Changing server configuration

---

## Multiple PBS Servers

If you have multiple PBS servers, add each one separately in Settings. Pulse will:
- Monitor each server independently
- Show backups from all servers in the unified Recovery view
- Deduplicate if the same backup appears via both PVE passthrough and direct PBS

---

## Troubleshooting

### "Connection Failed" Error

Check from the Pulse host or container's network, if possible. A request from
another machine does not prove that Pulse can reach PBS.

- **Address/network:** use the PBS HTTPS URL and port `8007`; check DNS, routing
  and the firewall before changing credentials.
- **TLS:** keep verification enabled. Use a certificate trusted by the Pulse
  runtime, or set its **SSL Fingerprint** after verifying the SHA-256 value
  through the PBS console or another already-trusted administrative channel.
  Do not accept a fingerprint solely from the failed connection. For example,
  on the PBS server you can inspect its public certificate (not its private key):

  ```bash
  openssl x509 -in /etc/proxmox-backup/proxy.pem -noout -fingerprint -sha256
  ```

- **Authentication/permissions:** test a datastore request, not just `/version`.
  Prepare a private header file on the machine running curl:

  ```bash
  umask 077
  mkdir -p "$HOME/.config/pulse"
  chmod 700 "$HOME/.config/pulse"
  touch "$HOME/.config/pulse/pbs-header"
  chmod 600 "$HOME/.config/pulse/pbs-header"
  vi "$HOME/.config/pulse/pbs-header"
  ```

  In the editor, save this line, replacing `<pbs-token-secret>` with the secret
  for the PBS token being tested:

  ```text
  Authorization: PBSAPIToken=pulse-monitor@pbs!pulse-token:<pbs-token-secret>
  ```

  Then use curl 7.76 or later, with your PBS hostname:

  ```bash
  curl --fail-with-body --silent --show-error --connect-timeout 5 --max-time 15 \
    --header "@$HOME/.config/pulse/pbs-header" \
    https://pbs.example.com:8007/api2/json/admin/datastore
  ```

  For curl with a private CA or self-signed certificate, add
  `--cacert "$HOME/.config/pulse/pbs-ca.pem"` using a public certificate obtained
  and verified through a trusted channel. Its hostname must still match. Do not
  use `--insecure` or `-k`.

  `401` means authentication was rejected; `403` means the requested access was
  refused. Both return a non-zero curl exit while retaining the error body.
  Inspect the existing token and both `Audit` grants before replacing anything.
  A `200` response listing the expected datastore establishes access to that
  list, not successful collection of every backup, job or History graph. An
  empty list is not proof that all permissions are correct.

Keep the header file outside shared repositories and diagnostics. Do not share
verbose/trace curl output, full infrastructure responses, token values or
private keys. If help is needed, provide only the HTTP status, relevant redacted
error and which expected reading is missing. Do not clear History or recreate a
working connection to make missing data look resolved.

### PBS is connected but History stays empty

Open **Proxmox → Backups → Backup Server → History**, not a datastore or the
PBS virtual machine's row in **Overview**. Record which route fails. A working
VM chart can use a different metrics target from the PBS host chart; it does
not establish that the PBS chart can read the same series. Current CPU and
memory values, a green connection badge, and an **API+Agent** label also do not
prove that stored host history is available.

Start with one affected PBS and a recent, unlocked time range:

1. In the drawer's identity details, note **Metrics Target**. Compare it locally
   with that PBS agent's **Connection** in **Settings → Infrastructure → Agent
   Doctor**. Record whether they agree and whether the target changes during a
   normal refresh; do not rename an agent or edit its ID to make them match.
   An `agent:` prefix alone does not prove that the target is the reporting
   agent. A hostname-based target is evidence to check, not proof of the cause.
2. If the target is already known, inspect the chart's request instead of
   collecting the identities again. In your signed-in browser, open developer
   tools → **Network**, filter for `metrics-store/history`, then open the
   affected **History** tab. Do not refresh or restart the server. Inspect one
   completed request with `metric=cpu` and its `resourceType`, `resourceId` and
   `range` query fields. Do not edit or replay the request.
3. Check its HTTP status and **Response** locally. A successful JSON response
   with an empty `points` array means this request returned no samples for that
   target and range; it does not prove there is no data under another target.
   Non-empty `points` with an empty chart points to a different display problem.
   An error, a cancelled request, or an HTML sign-in page is not an empty series.
   Record a redacted error if present rather than changing permissions.

If no matching request appears, record that fact and the visible chart message;
do not paste scripts into the browser console to force one. If opening the
page makes the browser unresponsive, stop and retain the observations already
available. No repeated reproduction is needed.

When reporting this, share only the failing route, time range, whether the
request target agrees with the drawer, HTTP status, and whether `points` is
empty or non-empty. Use consistent placeholders for private hostnames and IDs.
Do not share a HAR export, **Copy as cURL** output, request headers, cookies,
tokens, or a full response. These checks use the existing browser session;
they need no new API token or command-execution permission.

Do not delete and re-add connections, re-enrol agents, clear History, or enable
Debug logging for this check. A Proxmox log saying **Guest agent returned no
filesystem info** refers to its QEMU guest-agent filesystem probe, not the
Pulse Agent's stored History request. Restarting that guest is not a diagnosis
of an empty PBS chart. See [VM Disk Monitoring](VM_DISK_MONITORING.md) if the
separate symptom is missing filesystem information in a PVE VM row.

### Slow Backup Loading

If you notice slow loading for PBS storage accessed via PVE:
- This often happens with encrypted PBS datastores
- The fix is to add PBS directly (this guide)
- Direct PBS connections bypass the slow PVE content listing

### Duplicate Backups

Check the source, datastore, namespace, guest type/ID and backup time on each
entry. Independent PVE installations can reuse a guest ID; matching VMIDs alone
are not proof of a duplicate. Keep both records while checking their origin.
If the same backup remains listed twice, report those redacted distinctions and
whether each entry came from direct PBS or PVE passthrough. Do not delete backups
or change retention to hide a display problem.

---

## Data Source Indicator

In the Recovery view, PBS backups show a data source indicator:

- **"PBS"** badge alone = Direct PBS connection (full data)
- **"PBS via PVE"** = Passthrough via PVE storage (limited data)

When the same backup is reconciled across both sources, Pulse prefers the
direct PBS observation. Check the actual source and expected readings after
adding the connection; the presence of a badge alone is not collection proof.

---

## Related Documentation

- [Unified Agent Setup](UNIFIED_AGENT.md) - Installing agents on PBS/PVE/PMG hosts
- [Configuration Reference](CONFIGURATION.md) - Environment variables including PBS settings
- [Troubleshooting](TROUBLESHOOTING.md) - General troubleshooting guide
