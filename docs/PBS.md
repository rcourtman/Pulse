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
quotes; never put the secret in a command argument. The preparation preserves
an existing file and refuses symlinked or non-regular credential paths. Stop
if it fails; do not continue to the installer:

```bash
(
  set -eu
  umask 077
  config_dir="$HOME/.config/pulse"
  credential_file="$config_dir/pbs-agent-token"
  if [ -L "$HOME/.config" ] || [ -L "$config_dir" ] || [ -L "$credential_file" ]; then
    printf 'Refusing a symlinked credential path.\n' >&2
    exit 1
  fi
  if [ -e "$credential_file" ] && [ ! -f "$credential_file" ]; then
    printf 'Credential file must be a regular file.\n' >&2
    exit 1
  fi
  mkdir -p "$config_dir"
  chmod 700 "$config_dir"
  touch "$credential_file"
  chmod 600 "$credential_file"
  vi "$credential_file"
)
```

Download the agent installer from **your Pulse server's HTTPS address**, then
inspect the saved script before running it. Replace the example Pulse URL in
both commands. Stop if the download fails; run the next command only after a
successful HTTP **200** download and inspection. This ignores curl configuration
and saves only a successful download to the installer path. It refuses an
existing installer rather than overwriting it; review that file locally before
deciding whether a new download is needed. Do not substitute GitHub's top-level
`install.sh`: that installs the Pulse server, not the agent.

```bash
(
  set -eu
  umask 077
  config_dir="$HOME/.config/pulse"
  installer_file="$config_dir/pbs-agent-install.sh"
  if [ -L "$HOME/.config" ] || [ -L "$config_dir" ] || [ ! -d "$config_dir" ]; then
    printf 'Prepare the private token directory first.\n' >&2
    exit 1
  fi
  if [ -e "$installer_file" ] || [ -L "$installer_file" ]; then
    printf 'Refusing to replace an existing installer path.\n' >&2
    exit 1
  fi
  chmod 700 "$config_dir"
  download_file=$(mktemp "$config_dir/pbs-agent-download.XXXXXX")
  curl_exit=0
  status=$(curl --disable --fail --silent --show-error --proto '=https' \
    --connect-timeout 5 --max-time 60 --output "$download_file" \
    --write-out '%{http_code}' https://pulse.example.com/install.sh) || curl_exit=$?
  printf 'HTTP %s\n' "$status"
  [ "$curl_exit" -eq 0 ] || exit "$curl_exit"
  [ "$status" = 200 ]
  mv -n "$download_file" "$installer_file"
  [ ! -e "$download_file" ]
)
```

```bash
bash "$HOME/.config/pulse/pbs-agent-install.sh" \
  --url https://pulse.example.com \
  --token-file "$HOME/.config/pulse/pbs-agent-token" \
  --enable-proxmox --proxmox-type pbs --enable-docker=false
```

Do not bypass certificate checks to fetch or run the installer. Use a trusted
Pulse certificate or add `--cacert` with a CA file you verified separately.
A failed or redirected download is not an installer: do not run its temporary
file. Keep `--disable` first in both curl examples on this page; it ignores
local settings that could enable tracing, bypass TLS or follow redirects. See
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
- Show backups from all servers under **Proxmox → Backups** (`/proxmox/backups`)
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
  Prepare a private header file on the machine running curl. Stop if preparation
  fails; an existing file is preserved, and symlinked or non-regular credential
  paths are refused before opening the editor:

  ```bash
  (
    set -eu
    umask 077
    config_dir="$HOME/.config/pulse"
    credential_file="$config_dir/pbs-header"
    if [ -L "$HOME/.config" ] || [ -L "$config_dir" ] || [ -L "$credential_file" ]; then
      printf 'Refusing a symlinked credential path.\n' >&2
      exit 1
    fi
    if [ -e "$credential_file" ] && [ ! -f "$credential_file" ]; then
      printf 'Credential file must be a regular file.\n' >&2
      exit 1
    fi
    mkdir -p "$config_dir"
    chmod 700 "$config_dir"
    touch "$credential_file"
    chmod 600 "$credential_file"
    vi "$credential_file"
  )
  ```

  In the editor, save this line, replacing `<pbs-token-secret>` with the secret
  for the PBS token being tested:

  ```text
  Authorization: PBSAPIToken=pulse-monitor@pbs!pulse-token:<pbs-token-secret>
  ```

  Then use curl 7.76 or later, with your PBS hostname. The request prints only
  the HTTP status and a new private response-file path; inspect that file locally,
  not in a public terminal recording or thread:

  ```bash
  (
    set -eu
    umask 077
    config_dir="$HOME/.config/pulse"
    credential_file="$config_dir/pbs-header"
    if [ -L "$HOME/.config" ] || [ -L "$config_dir" ] || [ -L "$credential_file" ] || [ ! -f "$credential_file" ]; then
      printf 'Prepare the private header file first.\n' >&2
      exit 1
    fi
    chmod 700 "$config_dir"
    chmod 600 "$credential_file"
    result_file=$(mktemp "$config_dir/pbs-response.XXXXXX")
    printf 'Private response file: %s\n' "$result_file"
    curl_exit=0
    status=$(curl --disable --fail-with-body --silent --show-error --proto '=https' \
      --connect-timeout 5 --max-time 15 --header "@$credential_file" \
      --output "$result_file" --write-out '%{http_code}' \
      https://pbs.example.com:8007/api2/json/admin/datastore) || curl_exit=$?
    printf 'HTTP %s\n' "$status"
    [ "$curl_exit" -eq 0 ] || exit "$curl_exit"
    [ "$status" = 200 ]
  )
  ```

  For curl with a private CA or self-signed certificate, add
  `--cacert "$HOME/.config/pulse/pbs-ca.pem"` using a public certificate obtained
  and verified through a trusted channel. Its hostname must still match. Do not
  use `--insecure` or `-k`.

  `401` means authentication was rejected; `403` means the requested access was
  refused. Both return a non-zero curl exit while retaining the error body
  privately. A redirect or `000` is not successful access; this command neither
  follows redirects nor retries.
  Inspect the existing token and both `Audit` grants before replacing anything.
  A `200` response listing the expected datastore establishes access to that
  list, not successful collection of every backup, job or History graph. An
  empty list is not proof that all permissions are correct.

Keep the header file outside shared repositories and diagnostics. Do not share
verbose/trace curl output, full infrastructure responses, token values or
private keys. If help is needed, provide only the HTTP status, relevant redacted
error and which expected reading is missing. Do not clear History or recreate a
working connection to make missing data look resolved.

### Backups are visible but Coverage says Unprotected

The **Backup health** strip and the backup list answer different questions.
If they disagree, use **PBS's own backup and verification records** for the
same guest while checking Pulse's assessment; do not assume either a failed
backup or working protection from the strip alone.

| Observation in Pulse | What it does not establish |
| --- | --- |
| **With PBS snapshots** or a matching backup row | That Pulse's protection assessment received the same subject-linked backup evidence. A similar name or guest ID alone is not identity proof. |
| **Verified** on a PBS backup | That every backup is current, every covered disk is included, or the guest recovered after the backup. Verification is evidence about that backup, not a successful restore. |
| **Protected**, **Attention**, **Unprotected** or **Unknown** | An independent check that backups are restorable. These are Pulse's assessment from the backup evidence it can read, not instructions to change backups. |

Check one affected guest without changing its setup:

1. Open **Proxmox → Backups → Coverage** and expand that guest. Read its
   protection explanation and the PBS **Job**, **History** and **Access**
   evidence. Access describes permissions; if provider evidence is absent,
   record that rather than interpreting it as healthy. These are backup
   evidence states, not the PBS host's CPU/metrics History chart.
2. Compare the listed backup with the existing record in PBS: server,
   datastore, namespace (including root), guest type/ID and backup time.
   Independent PVE installations can reuse an ID; confirm the owning
   installation too. A **guest snapshot** is not a PBS backup. Check the
   existing backup task and verification result in PBS, not just the last
   time Pulse refreshed.
3. If the records still disagree, report the Pulse version, one guest's
   protection explanation, PBS Job/History/Access values and the matching
   backup's time and verification result. Use consistent placeholders for
   private server, datastore, namespace and guest identities. Do not share
   tokens, full API responses, HAR exports, or screenshots with secrets.

Do not delete backups, clear History, change retention or freshness settings,
recreate tokens/connections, restart or downgrade Pulse, or run a new backup,
verification or restore just to clear the strip. Missing or contradictory
Pulse evidence needs investigation, not a destructive diagnostic or an
assumed freshness-policy cause. Use your established backup checks meanwhile.

**An OK backup task or a Verified label does not prove the guest thawed.** If a
backup left a guest unresponsive or frozen, follow [Backup safety](VM_DISK_MONITORING.md#backup-safety)
and the platform's established recovery procedure, not this display check.
Keep affected Pulse monitoring stopped until independent post-backup checks
confirm thaw, fresh successful writes to every filesystem covered by the
backup and workload liveness; restore only services and timers active before
the pause. Monitoring and alerts are unavailable while Pulse is stopped.

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
   completed request for the drawer's `resourceType`, `resourceId` and `range`.
   The drawer normally fetches all chart metrics together: a request **without
   `metric`** is expected, so do not wait for a separate `metric=cpu` request.
   Do not edit or replay the request.
3. Check its HTTP status and **Response** locally. The query determines which
   response field to inspect:
   - **No `metric` query field:** inspect the `metrics` object. An empty
     `metrics: {}` means this request returned no series. If it has a `cpu`
     entry, note whether the `metrics.cpu` array is empty or non-empty; other
     metrics can be present even when CPU is absent.
   - **`metric=cpu`:** inspect the `points` array for that single metric.
     An empty array means this request returned no CPU samples.
   A successful empty result applies only to this target and range; it does
   not prove there is no data under another target. Non-empty samples with an
   empty chart point to a different display problem. A measured zero is a
   sample, not an empty result. An error, a cancelled request, or an HTML
   sign-in page is not an empty series. Record a redacted error if present
   rather than changing permissions.

If no matching request appears, record that fact and the visible chart message;
do not paste scripts into the browser console to force one. If opening the
page makes the browser unresponsive, stop and retain the observations already
available. No repeated reproduction is needed.

When reporting this, share only the failing route, time range, whether the
request target agrees with the drawer, HTTP status, and the response shape:
`metrics` empty, CPU absent, CPU empty/non-empty, or single-metric `points`
empty/non-empty. Use consistent placeholders for private hostnames and IDs.
Do not share a HAR export, **Copy as cURL** output, request headers, cookies,
tokens, or a full response. These checks use the existing browser session;
they need no new API token or command-execution permission.

Do not delete and re-add connections, re-enrol agents, clear History, or enable
Debug logging for this check. A Proxmox log saying **Guest agent returned no
filesystem info** refers to its QEMU guest-agent filesystem probe, not the
Pulse Agent's stored History request. Restarting that guest is not a diagnosis
of an empty PBS chart. See [VM Disk Monitoring](VM_DISK_MONITORING.md) if the
separate symptom is missing filesystem information in a PVE VM row.

### Backup health disagrees with visible PBS backups

The backup health strip rates monitored workloads, not individual artifacts.
A count of **with PBS snapshots**, a visible restore point or a **Verified**
label does not by itself establish the workload's protection posture:

| Reading | What it tells you | What it does not prove |
|---|---|---|
| With PBS snapshots / restore points | Pulse lists backup artifacts matched in that view | The protection calculation has the same linked evidence and readable history |
| Verified | PBS reported verification for that artifact | Every workload is protected, guest thaw succeeded or an application restore works |
| Coverage posture | Pulse's assessment of linked backup and provider evidence | A recovery guarantee or a reason to delete other backups |

**Attention** can mean an old qualifying backup, a newer failed job, partial
history or missing/overdue verification when expected. **Unprotected** means
Pulse's protection calculation sees complete history but no qualifying backup;
it is not the label for an old successful backup alone. **Unknown** means the
available evidence cannot support a protection claim. A guest-local snapshot
alone is not an independent backup; a PBS backup snapshot is different. See
[protection posture](RECOVERY.md#protection-posture) for the policy distinctions.

If the strip disagrees with recent PBS records, keep the disagreement visible
and use PBS's own backup and verification records to judge coverage meanwhile:

1. In **Proxmox → Backups → Coverage**, expand one affected workload. Note its
   posture explanation and the PBS provider's **Job**, **History** and **Access**
   values (Access describes permissions). If the provider evidence is absent,
   record that rather than assuming access is complete. Use the existing page;
   do not run **Run Diagnostics** or a guest-agent probe to fill the gap.
2. Compare the corresponding **By date** artifact with PBS's own inventory in
   an existing authorised session: PBS connection, datastore, namespace, guest
   type/ID, backup time and verification result. Keep its PVE connection/node
   context too; independent PVE installations can reuse a VMID. A recent backup
   for a same-named or same-numbered guest is not necessarily this workload's
   backup. A successful connection test is not proof of readable backup history.
3. If they still disagree, report just that row's explanation, Job/History/Access
   values, artifact source/time/verification and the running Pulse version.
   Use consistent placeholders for private identities; retain full inventories
   and credentials locally. An absent or unavailable value is not zero or
   evidence that no backup exists.

Do not restart, downgrade, recreate connections, re-enrol agents, delete history,
change retention or run another backup just to make the strip green. Do not
relax TLS or grant write/admin permissions for this check. **Verified** or an
OK backup task does not prove guest thaw or a tested restore. If a workload
stopped responding during a backup, stop this display check and follow the
[backup safety precaution](VM_DISK_MONITORING.md#backup-safety); do not reproduce
freeze/thaw to diagnose a status disagreement.

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

Under **Proxmox → Backups** (`/proxmox/backups`), PBS backups show a data source
indicator. There is no current top-level Recovery page:

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
