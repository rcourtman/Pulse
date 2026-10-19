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
- ✅ [Sync, verify, prune, and GC job status](#a-pbs-job-looks-healthy-but-its-status-disagrees)
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

Create a separate Pulse token in **API Access**, using the **Agent** preset
for reporting and configuration reads; it does not include agent lifecycle
management (`agent:manage`). This is a **Pulse**
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

### PBS is Active but missing from monitoring

**Active** in **Settings → Infrastructure** describes the configured
connection's polling health. It does not establish that Pulse has a matching
PBS resource, every datastore or fresh readings. A direct PBS API connection
does not require a Pulse Agent; adding one does not prove that the API path
recovered.

Use the existing views to distinguish what is missing:

| Observation | Where to look next |
| --- | --- |
| The PBS server itself is missing | Open **Proxmox → Backups** and look for its **Backup server** row. Note any existing search or view filter. The PBS VM/LXC in **Overview** is a different resource. If Proxmox navigation is absent too, retain that fact. |
| The server row exists, but datastores or backups are missing | Check the [affected datastore and namespace](#one-pbs-namespace-is-missing-or-stops-changing). PVE passthrough backups do not establish that direct PBS inventory is available. Do not widen permissions or change scope just to diagnose the gap. |
| Current readings exist, but a chart is empty | Use the [PBS History checks](#pbs-is-connected-but-history-stays-empty) for that server and time range. An empty chart is not the same as a missing server. |

When reporting a missing server, keep the Pulse server version separate from
the PBS version and whether a Pulse Agent is installed. Say which existing
views lack it, whether Infrastructure says Active, and whether it was visible
before. A last known working version is useful only if already known; unknown
is valid. Use consistent aliases for private server names. Existing error text
or screenshots are enough to start; do not run Diagnostics, enable Debug or
collect a full response just to fill the report.

Do not delete and re-add the connection, replace tokens, install an agent,
restart PBS or Pulse, clear History or run a backup to make the server appear.
These actions can lose evidence or change the affected path. Keep certificate
verification and existing access restrictions intact. If the original incident
also involved an unresponsive guest or uncertain freeze/thaw, follow
[Backup safety](VM_DISK_MONITORING.md#backup-safety) before any live checks.

### One PBS namespace is missing or stops changing

**A fresh sibling namespace does not prove that this namespace was read.**
A PBS poll can succeed for one scope and fail for another. A visible older
row is not a fresh inventory result, and a missing row does not establish
that PBS deleted the backup.

In **Proxmox → Backups → By date**, keep the current search and location filter
with your observation, then clear them to distinguish a hidden row from a
missing one. Match **Source** and **Location** with the existing PBS record:
owning server, datastore, full namespace path (including root), guest type/ID
and backup time. Root and nested namespaces are different scopes; the same
guest ID in a sibling namespace is not the same stored backup. Use the
[copy checks](#duplicate-backups) if two collection paths show the same point.

| Existing evidence | What it means |
| --- | --- |
| PBS lists the expected point, but Pulse lacks it or shows only older points | An inventory disagreement, not proof that the backup failed. Keep the affected scope distinct from a sibling that refreshed. |
| The scope's last read failed or timed out | Its current inventory is unknown. Retained rows can describe an earlier read; their presence is not proof of current collection or protection. |
| Access to that scope was rejected | Missing rows are not evidence of an empty namespace. Preserve the intended access boundary; do not grant broader access to make the list agree. |
| A successful read returned no points | That result applies only to the exact scope and filters read, not its parent, children, siblings or the whole datastore. |

**Created** is the backup's time, not the time Pulse last read its namespace.
An advancing connection time, a changing “ago” label or an OK job elsewhere
cannot supply the missing read result. If no scope-specific error or collection
time is available, keep it unknown rather than running a request to obtain one.

For help, retain one affected scope and, if already observed, one working
sibling, the original time, running Pulse/PBS versions, Source/Location,
filters and relevant redacted error. Use consistent aliases that preserve the
namespace hierarchy; keep full inventories, logs, tokens and private paths out
of public reports. Existing observations are enough: do not run Diagnostics,
retry the poll, restart, recreate the connection or change permissions,
retention or backups as a test. Use PBS's existing records meanwhile; see
[backup health](#backups-are-visible-but-coverage-says-unprotected) separately
for a Coverage disagreement.

### A PBS job looks Healthy but its status disagrees

**An error, warning or running state must not be treated as a completed OK
job just because Pulse shows a Healthy badge.** Job status text can contain
`OK` or `success` inside a longer message; those words alone do not establish
successful completion. Use the same task's existing PBS result when the badge
and status disagree.

In the PBS **Backup server** resource's details, look for **Job health
evidence** when present. Compare its **State**, job family/ID, datastore,
namespace and run time with the same job in PBS. Keep an unavailable identity
or time unknown; a successful sibling job or earlier run cannot supply the
missing outcome.

| Observed result for the same task | What it establishes |
| --- | --- |
| Error or failed | Not successful completion, even if the message also contains `OK` or `success`. |
| Warnings | A warning result, not a clean successful run. Read the existing PBS task summary for the affected work. |
| Running | Work is in progress; neither completed success nor failure is established. |
| No matching result, a partial read or a permission gap | The outcome is unknown. An empty list or a green connection does not supply it. |
| Completed OK | That task reported success. It does not establish current inventory, verification of every snapshot, restore readiness or guest thaw. |

Use PBS's existing task records while the readings disagree. Do not run a new
job, backup, verification, restore or Diagnostics, restart Pulse, replace tokens
or widen permissions to test the badge. When asking for help, retain the running
Pulse/PBS versions, job family/ID, matching time and relevant redacted status;
use consistent aliases for private names and keep full task logs and tokens
private. No new capture or probe is needed just to fill a report.

An unresponsive guest or uncertain freeze/thaw needs the separate
[Backup safety](VM_DISK_MONITORING.md#backup-safety) procedure, not these display
checks. A corrected badge alone is not monitoring or guest recovery.

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

  For renewal, a fingerprint mismatch or contradictory saved pins, use the
  [certificate-trust checks](TROUBLESHOOTING.md#proxmox-or-pbs-certificate-validation-failed)
  before changing this connection. A saved leaf pin is a different trust
  method from CA validation; do not disable verification or clear it simply
  because a new certificate connects.

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

### Backup age colours for a longer backup cycle

The VM/LXC backup shield uses **hours**, separately from the backup-age alert
thresholds in **days**. An amber age badge does not by itself mean that a
backup failed or that a notification was sent.

Open **Alerts → Thresholds → Proxmox**, then expand **Recovery**. In
**Global Defaults**:

| Control | What it changes |
| --- | --- |
| **Fresh Hours** | How long the latest completed backup keeps a green shield. The default is 24 hours. |
| **Stale Hours** | The end of the next age band, after which the backup is classified as overdue. Keep it at least as large as Fresh Hours. |
| **Warning Days** / **Critical Days** | Backup-age alert thresholds, not the shield's freshness window. Leave these unchanged unless you intend to change alert policy. |

For a three-day cycle, **Fresh Hours = 72** keeps a completed backup fresh
through 72 hours. **Stale Hours = 96**, for example, leaves a further day in
the stale band. Choose any scheduling margin to match your existing backup
policy; these are global display settings, not a change to the backup schedule.
Review the alert-day values before saving, then check the same guest's badge.
The completed backup's time does not change, and missing or unknown backup
evidence is not made healthy by a longer freshness window.

**v6.5.0 caution:** if **Critical Days** is Off while **Warning Days** is
enabled, do not save threshold edits on that version: its editor can re-enable
critical alerts at the warning age. Cancel rather than changing alert policy
just to remove an amber badge.

If the problem is missing backups or conflicting protection evidence rather
than an expected longer cycle, use the [backup health checks](#backups-are-visible-but-coverage-says-unprotected)
instead. Do not run another backup, change retention, disable alerts or clear
History just to change a colour.

### Backups are visible but Coverage says Unprotected

The **Backup health** strip and the backup list answer different questions.
If they disagree, use **PBS's own backup and verification records** for the
same guest while checking Pulse's assessment; do not assume either a failed
backup or working protection from the strip alone.

| Observation in Pulse | What it does not establish |
| --- | --- |
| **With PBS snapshots** or a matching backup row | That Pulse's protection assessment received the same subject-linked backup evidence. A similar name or guest ID alone is not identity proof. |
| **Verified** on a PBS backup | That every backup is current, every covered disk is included, or the guest recovered after the backup. Verification is evidence about that backup, not a successful restore. |
| **Prune protected** in a restore point's **State** | That this point is verified. This is the provider's protection-from-pruning flag, shown separately from verification; it is not the workload's protection assessment. See [prune protection](#prune-protection-is-not-verification) before changing that flag. |
| **Protected**, **Attention**, **Unprotected** or **Unknown** on the workload | An independent check that backups are restorable. These are Pulse's assessment from the backup evidence it can read, not instructions to change backups. |

Check one affected guest without changing its setup:

1. Open **Proxmox → Backups → Coverage** and expand that guest. Read its
   protection explanation and PBS's **Scope** and **Source** to identify each
   observation; keep different PBS connections separate. Read its **Result**,
   **History** and **Access** together. A successful inventory read is not a
   successful backup job. Older versions call Result Job and may omit Scope
   and Source; keep absent identity unknown rather than guessing from guest
   placement. Access describes permissions; if provider evidence is absent,
   record that rather than interpreting it as healthy. These are backup
   evidence states, not the PBS host's CPU/metrics History chart.
2. Compare the listed backup with the existing record in PBS: server,
   datastore, namespace (including root), guest type/ID and backup time.
   Independent PVE installations can reuse an ID; confirm the owning
   installation too. Under **Restore evidence**, choose the newest point whose
   **Source** is **PBS**, and read its **Created** time and **State**. Hover the
   Created age for its absolute timestamp; if unavailable, keep the time
   unknown. A relative age or Pulse's refresh time is not a matching backup
   timestamp. A **guest snapshot** is not a PBS backup. Check the existing
   backup task and verification result for that same point in PBS.
3. If the records still disagree, report the Pulse version, one guest's
   protection explanation, the PBS scope/source and Result/History/Access values,
   and the matching
   point's Created time, State and PBS verification result. Use consistent
   placeholders for private server, datastore, namespace and guest identities.
   Do not share tokens, full API responses, HAR exports, or screenshots with secrets.

**If PBS says verified but Pulse disagrees:** an older or absent point in Pulse
is an inventory discrepancy, before comparing verification. **Unverified** on
the matching point means Pulse has no successful verification evidence for it;
it does not prove that PBS's verification job failed. **Verified** with a
workload still needing attention is a separate assessment disagreement: retain
its explanation and Provider evidence. **Prune protected** does not change the
verification result alongside it. **Unknown** on a PBS point means no verification
result was reported; it does not mean verification failed. **Job OK** is not an
individual point's verification result. Use the records already available,
without running Diagnostics or another verification just to compare them.
If the latest job is **Unknown**, use the [job uncertainty checks](#latest-backup-job-is-unknown)
before interpreting that as no job or a failed backup.

#### Latest backup job is Unknown

In **Proxmox → Backups → Coverage**, expand the affected guest and read the
explanation beside its latest-job result. Pulse cannot choose a latest job
when a job has a missing, invalid or future start, or equally recent jobs disagree.
**Unknown** is not evidence that no job ran, that one is queued or that the
backup failed.

Where shown, **Collected backup jobs** keeps the observations separate from
the latest result. Its list order is not a timeline. Read each **Job ID**,
**Result**, **Job connection**, **Job node**, **Started**, **Last observed**
and **Reported error** together. If **Reported status** and error disagree,
retain both rather than treating an OK status as success. Older versions may
omit collected jobs; use the same job's existing Proxmox records instead.

- **Started** is the reported start, which may be unusable. **Last observed**
  is when Pulse read the job history, not when the job finished. Refreshing
  the page or a changing relative age does not establish a new collection.
- The recorded job node can differ from the guest's current node after a
  move. Match the owning connection, guest type/ID and original time with
  the existing Proxmox task, not another installation's same name or VMID.
- Search can find a collected job ID, reported error or recorded node. Clear
  search and view filters before concluding a row is absent. Search only
  covers collected evidence, not every native task or a complete job history.
- An ordinary unambiguous or empty replacement can remove collected
  observations and their search matches. This is not a permanent job log;
  their disappearance is not evidence that the original backup recovered.

For help, retain the version, explanation and only the relevant redacted job
fields and matching native result already available. Keep full job logs,
private names, paths and tokens out of public reports. Do not run a new
backup, verification, restore, Diagnostics or guest-agent probe, restart,
clear History, change permissions or repeat the failed job just to fill a gap.

**An OK job is not proof of backup usability, guest thaw or filesystem writes.**
If the guest is frozen or unresponsive, keep affected Pulse monitoring stopped
and follow [Backup safety](VM_DISK_MONITORING.md#backup-safety), not these display
checks. Readable diagnostics alone do not establish recovery.

#### Prune protection is not verification

A PBS snapshot's **protected** flag excludes it from ordinary pruning. It is
**retention protection**, not a check of the stored data, another backup copy,
or protection against storage loss. Do not remove the flag to reveal another
State label in Pulse: that can make the snapshot eligible for a later prune.

Read the **same snapshot's existing record in PBS**, matching its server,
datastore, namespace, guest type/ID and backup time. Read its protected flag and
last verification result/time separately:

- **Protected with successful verification:** these are two independent facts.
  The result applies to that snapshot at the recorded verification time, not
  every backup or a successful restore.
- **Protected with failed verification:** retention has not repaired the failed
  check. Keep the snapshot and its existing failure evidence; a Protected label
  must not be used to dismiss it.
- **Protected with no recorded verification result:** verification is unknown,
  not successful or failed. A different snapshot's result or an OK backup job
  cannot fill that gap.

If Pulse shows separate prune-protection and verification labels, read both.
If it shows only Protected, that label alone cannot distinguish these cases;
use the existing PBS record, not a flag change or a new verification job. Do
not run prune, garbage collection, a backup or a restore just to compare the
displays. For a discrepancy, retain only the matching point's redacted result
and time alongside the observations above, not a full job log or inventory.

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

Open **Proxmox → Backups**, choose the **Backup Server** row, then use the
drawer's **Overview** charts and **History range** selector. If your installed
version still has a separate **History** tab, use that tab instead; no upgrade
or restart is needed just to find the chart. This is not a datastore chart,
**Alerts → History**, or the PBS virtual machine's row in Proxmox **Overview**.
Record which route fails. A working VM chart can use a different metrics target
from the PBS host chart; it does not establish that the PBS chart can read the
same series. Current CPU and memory values, a green connection badge, and an
**API+Agent** label also do not prove that stored host history is available.

Use the chart's selected stored range, not a current summary card or table
sparkline. For observation times, missing samples and aggregated low–high bands,
see [reading a History panel](METRICS_HISTORY.md#read-a-history-panel). On a
phone, secondary panels may be collapsed behind **Show …** below the charts;
reveal them before treating a panel as missing. This changes the layout, not
the selected range or collection. CPU and memory are in **Utilization**, which
is already visible when the charts are shown; the button is not a fix for an
empty CPU or memory chart.

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
   affected drawer's **Overview** charts (or **History** tab on older versions).
   Do not refresh or restart the server. Inspect one
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

### Backup-age alerts name a deleted or renamed guest

PBS can retain backups after a VM or container has been removed. A retained
backup is not evidence that the guest is still running, and a last-known name
or location does not prove live ownership. With orphaned-backup alerts enabled,
those retained backups can still reach the configured age threshold. A wrong
node on a **live** guest is a separate attribution problem; use the
[cross-installation identity checks](TROUBLESHOOTING.md#monitoring-is-mixed-between-proxmox-installations),
not an ignore rule to hide it.

Keep the intended policy unchanged while diagnosing. If you deliberately want
to change which retained backups raise age alerts, the existing controls are
under **Alerts → Thresholds → Proxmox → Recovery** (called **Proxmox Backups**
in older versions). Expand Recovery and find **Orphaned backups**:

- Its **Alerts** switch applies to **all orphaned guests**, not just the named
  guest. Turning it off is suppression, not a repaired identity or successful
  backup. It does not delete the stored backups or History.
- **Ignore VMIDs** also excludes **live guests with the same VMID**, across
  connected installations. A trailing `*` matches a VMID prefix. Do not use this
  global filter to silence only one guest when another installation reuses its
  ID; it would remove that other guest's backup-age protection too.

Save an intentional policy change with **Save changes**, then observe ordinary
polling. Do not treat an old received message or a retained History entry as a
new firing, or a disappeared alert as proof that backups are healthy. If new
notifications continue despite the intended saved policy, or the corresponding
active alert cannot be found, retain that as an unresolved alerting problem.

For a report, use the already-received message's time and the affected view
(**Alerts → Overview**, History, or notification only), plus the saved orphan
policy if already known. Use consistent aliases for the source, guest type/VMID
and datastore/namespace; share only the relevant redacted facts, not a full
payload or queue export. Do not edit Pulse's state files or databases, and do
not delete backups, clear History, rename/re-add sources or restart to clear
the symptom. No new backup, guest-agent probe, notification test or queue replay
is needed. An unresponsive guest needs the separate [Backup safety](VM_DISK_MONITORING.md#backup-safety)
procedure, not an alert-policy change.

### Slow Backup Loading

A slow backup list does not establish an encrypted-datastore or PVE-proxy
cause. First distinguish the affected reading, without changing the setup:

- **Slow Proxmox → Backups list:** note the source indicator on an existing
  affected row. **PBS** means direct PBS inventory. **PVE file** means PVE
  storage inventory, which can include a PBS-backed target; that badge alone
  does not identify the storage type or prove a direct PBS connection exists.
  See [Data Source Indicator](#data-source-indicator). A loaded row does not
  prove that every backup, job or History reading is current.
- **“Backup-age alerts were not evaluated” warning:** this is a failed monitoring
  check, not evidence that a backup failed. Follow the
  [evaluation warning checks](TROUBLESHOOTING.md#backup-age-alerts-were-not-evaluated).
  Adding a connection or making the list load does not establish that this
  separate evaluation recovered.
- **PVE or PBS inventory “loading”/“unavailable” notice:** the list and counts
  are incomplete even if the other inventory loaded. Missing rows do not mean
  no backups exist. Record which source is affected and its existing error;
  do not repeatedly press Retry to test recovery.
- **Wrong installation, overlapping guest IDs or conflicting Coverage:** use
  the [backup health checks](#backups-are-visible-but-coverage-says-unprotected)
  and [cross-installation identity checks](TROUBLESHOOTING.md#monitoring-is-mixed-between-proxmox-installations).
  Keep the existing connections and records while comparing their origin.

For a report, retain the Pulse version, affected view, source indicator,
observed loading duration and relevant existing redacted error. Use consistent
placeholders for private server, datastore, namespace and guest identities;
do not share full responses, HAR exports or credentials. If loading makes
Pulse unresponsive, stop and retain the observations already available, rather
than repeatedly refreshing or running diagnostics.

A direct PBS connection is an optional planned setup change when backups are
currently available only through PVE passthrough and you need the additional
API-backed readings described above. It bypasses PVE's proxy listing for those
direct observations; it is **not a guaranteed performance or evaluation fix**.
Check for an existing direct connection first and verify the owning PBS server
before adding anything. Keep certificate verification and privilege separation
enabled, and confirm the expected datastores and readings after normal polling.
Do not add or recreate connections, install an agent, restart Pulse, clear
History, change retention/polling limits, or run a backup, verification or restore
just to diagnose slow loading. Use the existing native backup records meanwhile;
a loaded list or an OK task is not proof of restore readiness or guest thaw.

### Duplicate Backups

**Matching names, VMIDs and backup times do not prove that two rows are the
same stored copy.** Open **Proxmox → Backups → By date** and compare **Source**
and **Location**, keeping any search or location filter with the observation.
Use the provider's existing inventory in an authorised session to distinguish:

| Existing evidence | How to interpret it |
| --- | --- |
| Two PVE nodes each have node-local storage named `local`, with the same archive filename | These can be independent stored copies, even within one cluster. Compare the **storage node**, not just the guest's current node: a guest can migrate without moving its older backup files. |
| The same archive on genuinely shared PVE storage is visible through several nodes | Several observations can refer to one stored copy. A common storage name alone does not establish that the storage is shared; use its existing PVE configuration. |
| Direct PBS and a PBS-backed PVE storage both show the same point | They can be two collection paths to one PBS snapshot. Match the owning PBS server, datastore, namespace, guest type/ID and backup time before treating them as duplicates. |

Keep independent PVE installations separate too: they can reuse node names,
storage names and VMIDs. If Pulse does not show a storage node or other needed
distinction, retain it as **unknown**, not proof of shared storage or a safe
copy to remove. A Location filter changes the view, not what exists on disk.
One row does not prove that there is only one copy, and two rows do not prove
two independent recovery options.

**A missing row after a collection error is not a deletion receipt.** An empty
inventory on one node does not establish that another node's local archive is
gone. Compare the original location and time with existing provider records;
do not rerun a backup or rescan repeatedly just to make the rows agree.

If the discrepancy remains, add the affected view/filter, Source and Location,
owning installation, storage node or known shared-storage relationship, guest
type/ID, backup time and any existing collection error to the existing report.
Use consistent aliases for private names and paths; unknown is valid. Do not
attach a full inventory, rename storage, migrate a guest, delete backups or
change retention to hide a display problem. Row counts do not establish restore
readiness or guest thaw; use [Backup safety](VM_DISK_MONITORING.md#backup-safety)
for an unresponsive guest or uncertain freeze/thaw, not these display checks.

If overlapping VMIDs also coincide with wrong-node errors or lost agent-backed
Docker monitoring after adding another Proxmox connection, use the
[cross-installation identity checks](TROUBLESHOOTING.md#monitoring-is-mixed-between-proxmox-installations).
A duplicate-backup check alone does not resolve those other symptoms.

---

## Data Source Indicator

Under **Proxmox → Backups** (`/proxmox/backups`), PBS backups show a data source
indicator. There is no current top-level Recovery page:

The **By date** view's **Source** labels describe the collection origin:

| Source | What it means |
| --- | --- |
| **PBS** | Direct snapshot inventory from Proxmox Backup Server. It does not prove that every backup, job or History reading is current. |
| **PVE file** | A backup file or volume reported by Proxmox VE storage. This can include a PBS-backed storage target; the label alone does not identify the storage type or establish a direct PBS connection. Compare the row's location with the existing PVE storage configuration. |
| **Snapshot** | A Proxmox VE guest snapshot, not a separate backup. It does not count as independent backup coverage. |

Do not look for an old **PBS via PVE** badge in this table. For a PBS-backed
PVE file, the source explanation identifies a backup volume reported by PVE
from a PBS-backed storage target. If that explanation is unavailable, keep the
origin unknown rather than inferring it from a matching guest ID or name.

A source label is not a protection, verification, restore or thaw result.
**PVE/PBS inventory loading or unavailable** notices mean counts are incomplete,
not that missing backups do not exist. Existing PBS and PVE observations can
refer to the same backup; compare their owning installation, storage,
namespace, guest type/ID and time before treating records as duplicates. See
[Duplicate Backups](#duplicate-backups) and the
[backup health checks](#backups-are-visible-but-coverage-says-unprotected).

---

## Related Documentation

- [Unified Agent Setup](UNIFIED_AGENT.md) - Installing agents on PBS/PVE/PMG hosts
- [Configuration Reference](CONFIGURATION.md) - Environment variables including PBS settings
- [Troubleshooting](TROUBLESHOOTING.md) - General troubleshooting guide
