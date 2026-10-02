# 🔧 Troubleshooting Guide

## ⚡ Quick Fixes

### I forgot my password

Pulse does not provide an email password-reset flow. Choose the path that
matches how this self-hosted instance authenticates:

- **Local Pulse username and password**: recovery requires shell access to the
  Pulse host or container. Follow the deployment-specific steps below.
- **OIDC, SAML, or proxy authentication**: contact the identity-provider or
  Pulse administrator. If the administrator deliberately kept local login as a
  fallback, they can open the Pulse URL with `?show_local=true`; this only
  reveals the existing local form and does not reset credentials.
- **Temporary lockout**: wait for the lockout to expire. Another signed-in
  administrator can use the lockout reset in Pulse; password recovery is not
  required.

The local recovery steps remove only the Pulse-generated authentication file.
After restart, Pulse still requires the host-only bootstrap token before it
will accept replacement credentials. If `PULSE_AUTH_USER` or `PULSE_AUTH_PASS`
is supplied by Docker Compose, Kubernetes, systemd, or another deployment
manager, update that deployment configuration instead; its environment values
override the generated file.

**Docker**:

```bash
docker exec pulse rm /data/.env
docker restart pulse
# Access UI again. Pulse will require a bootstrap token for setup.
# Get it with:
docker exec pulse /app/pulse bootstrap-token
```

**Systemd**:

```bash
sudo rm /etc/pulse/.env
sudo systemctl restart pulse
sudo pulse bootstrap-token
```

**Proxmox LXC** (installed from the Proxmox shell):
Pulse runs inside the container, so run the same steps through `pct exec` on the Proxmox host. The binary needs its absolute path here, because `pct exec` runs with `PATH=/sbin:/bin:/usr/sbin:/usr/bin` and that does not include `/usr/local/bin`:

```bash
pct exec <ctid> -- rm /etc/pulse/.env
pct exec <ctid> -- systemctl restart pulse
pct exec <ctid> -- /usr/local/bin/pulse bootstrap-token
```

If you only missed the token during a fresh install (no password set yet), skip the first two commands and just read it back with the last one.

Treat the bootstrap token like a password: enter it only in the Pulse setup
screen for this instance and do not paste it into support requests or issue
reports.

### Port change didn't take effect

The web UI and API listen on `FRONTEND_PORT` (default `7655`). The deprecated
`PORT` alias applies only when `FRONTEND_PORT` is unset; changing
`frontendPort` in `system.json` has no effect. `PULSE_AGENT_INGEST_PORT` is a
separate agent listener, not the web UI port. See
[Port configuration](CONFIGURATION.md#common-overrides-environment-variables).

- **Systemd / Proxmox LXC**: identify the active service with
  `systemctl is-active pulse` (legacy installs may use `pulse-backend`). Inspect
  only the port setting in its managed configuration locally. If you change a
  unit or drop-in, run `sudo systemctl daemon-reload`, then restart the affected
  service during a suitable maintenance window. For LXC, run these checks inside
  the Pulse container, not on the Proxmox host.
- **Docker / Compose**: distinguish the published host port from the listener
  inside the container. With the default listener, `8080:7655` exposes the UI on
  host port `8080`; changing the host port does not require `FRONTEND_PORT`.
  In the repository's Compose file, `PULSE_PORT` controls this host-side mapping.
  Save the mapping in your existing Compose project and apply it with
  `docker compose up -d pulse`, keeping the same image and mounted data volume.
  Restarting an existing container does not apply a new port mapping. For other
  container managers, use their recreate/redeploy action while preserving the
  data mount; do not delete the volume.
- **Reverse proxy**: check its upstream port and the firewall separately. A
  working agent connection on a split-port deployment does not prove that the
  web UI is reachable.

Do not post full service environments, `docker inspect` output or resolved
`docker compose config` output: they can include passwords and tokens. Share
only the relevant port numbers and a redacted error if help is needed.

### "Connection Refused"
- Check if Pulse is running.
- Verify the port is open on your firewall.
- **PBS**: Remember PBS uses port **8007** and requires **HTTPS**.

---

## 🔍 Common Issues

### Authentication

#### "Invalid username or password" after setup
- **Docker Compose**: Did you escape the `$` signs in your hash? Use `$$2a$$...`.
- **Truncated Hash**: Ensure your bcrypt hash is exactly 60 characters.

#### Cannot login / 401 Unauthorized
- Clear browser cookies.
- Check if your IP is locked out (wait 15 mins).
- If another admin can log in, use `POST /api/security/reset-lockout` to clear the lockout for your username or IP.

#### Audit Log verification shows unsigned events

**Unsigned** means no signature was stored for that event; it is not the same
as **Failed** verification or a request **Error**. Signing can be unavailable
when Pulse cannot initialise its encryption manager. Check a bounded startup
log excerpt and the persistent data mount and access for the service account,
without printing key contents. Do not delete or regenerate `.encryption.key`
or an audit signing key to make the warning disappear. Restoring signing for
new events cannot authenticate an old unsigned event.

See [Audit verification and safe recovery](AUDIT_LOGGING.md#verification-failures-and-safe-recovery)
for the different results and evidence to retain.

#### Audit Log is empty

Clear the event, user, date and success filters, and check the selected
organisation first. A query error is not an empty history. **Pulse Pro runtime
required** means an active licence is running on the public community runtime;
follow the panel's **Download Pulse Pro** link rather than buying another
licence or resetting storage. Without the audit capability, reads and exports
are gated, but Pulse still attempts to capture events persistently on all
plans. **Console Logging Only** can also reflect unavailable persistent
storage: inspect the bounded startup logs for audit initialisation errors.
Do not change passwords or create tokens merely to populate the panel.

#### Audit Log verification fails for older events

A failed signature check does not by itself prove tampering. An event signed
with a different key can fail even when its contents are unchanged; missing
signatures and an unsupported or damaged signature format also cannot verify.
Keep the failure as evidence. Do not swap an old key into the live instance,
edit audit rows or re-sign old events. Preserve the current data and keys
privately before any recovery; compare a matching backup only in an isolated
restore, not by overwriting today's history. Follow
[safe audit recovery](AUDIT_LOGGING.md#verification-failures-and-safe-recovery).

### Monitoring Data

#### Agent fleet update or identity issue

- Open an outdated-agent notice or
  `/settings/infrastructure?agentDoctor=1` to open **Agent Doctor** and
  copy the platform-specific command for each reported host. This is a manual
  handoff; Pulse does not remotely execute the command.
- Administrators can call the read-only Agent Fleet Doctor endpoint,
  `GET /api/agents/diagnostics`, to inspect liveness, version drift, profile
  deployment drift, expected telemetry gaps, and identity-split evidence. It
  does not change agent configuration or enqueue a repair.
- A current Pulse server does not prove fleet convergence. Eligible v6 agents
  update asynchronously; v5, PVE, disabled, and failed updates require manual
  handling.

#### Removed Pulse server but `pulse-agent` still logs connection failures

Removing the Pulse server does not remove agent services installed on monitored
hosts. On a systemd host, stop and disable the orphaned service to halt retries:

```bash
sudo systemctl disable --now pulse-agent.service
```

If the Pulse server is still reachable, use its generated uninstall command so
the agent can deregister cleanly. Otherwise, stopping the service is the safe
first step before platform-local cleanup.

#### Filter Pulse's systemd journal by severity

Current systemd installs preserve Pulse's structured log level as the journal
priority. For example, show warnings and more important records with:

```bash
sudo journalctl -u pulse -p warning
```

The stored message remains JSON (`"level":"warn"`, for example), while
`PRIORITY` is available to `journalctl` and syslog forwarding. If a current
Pulse warning appears only with `-p info`, inspect `systemctl cat pulse`; the
installed service must contain `SyslogLevelPrefix=true` and
`PULSE_LOG_JOURNAL_LEVEL_PREFIX=true`. Re-run the current signed installer to
repair an older generated unit rather than adding a JSON-parsing wrapper.

#### VMs show "-" for disk usage
- Install **QEMU Guest Agent** in the VM.
- Enable "QEMU Guest Agent" in Proxmox VM Options.
- Restart the VM.
- See [VM Disk Monitoring](VM_DISK_MONITORING.md).

#### Temperature data missing
- Install `lm-sensors` on the host.
- Run `sensors-detect`.
- Install the unified agent on the Proxmox host with `--enable-proxmox`.
- See [Temperature Monitoring](TEMPERATURE_MONITORING.md).

#### Docker hosts appearing/disappearing

Cloned hosts can share a **saved Pulse agent ID**, not just an OS machine ID.
The agent uses an explicit ID first, then its saved `agent-id` file, and derives
one from the machine only when neither is available. Changing a hostname, IP or
`/etc/machine-id` therefore does not necessarily change its Pulse identity.

Compare the affected hosts in **Agent Doctor** and inspect only their configured
`agent-id` files locally. Do not delete the OS machine ID, agent state or Pulse
history as a troubleshooting step. If a duplicate is confirmed, give only the
clone a stable, unique ID in its managed service or container configuration;
leave the original host unchanged. Follow
[Clone identity recovery](UNIFIED_AGENT.md#duplicate-agents) for the configuration
precedence, systemd example and checks after restart.

#### Excessive CPU, writes or database growth

First distinguish **Pulse server activity**, agent activity and total host or
storage-device activity. A database's size is retained space, not the number of
bytes written: repeated updates can produce high writes without growing the file.
Keep the original running version, uptime, measurement time and workload before
changing anything. A different binary can migrate persistent data; switching
back to an older version does not make that data a clean older-version baseline.

- For CPU, record the measuring tool, allocated CPUs or container quota and
  whether its percentage represents one core or the whole allocation. A `ps`
  `%CPU` value is an average since process start, not a recent sampling window.
- Note fleet size, polling interval and whether Pulse dashboards are open. If
  safe, close all Pulse tabs and compare another equal-length window, without
  restarting the server or changing polling. A decrease narrows the workload;
  it does not establish the cause or prove monitoring is healthy.
- If the host is unresponsive or storage is nearly full, do not prolong the
  failure to collect a benchmark. Keep existing observations and report the
  interruption instead. Do not enable Debug or repeatedly export diagnostics
  merely to measure performance; those actions add work and can change the result.

For a responsive **Linux systemd / Proxmox LXC** install, the following reads
two process-I/O samples, waiting 60 seconds between them. Run it inside the
Pulse container for LXC, not on the Proxmox host. Substitute the actual service
name (`pulse-backend` on some older installs). Use an account authorised to
read the process counters; no service restart or database access is needed.

```bash
# systemd / Proxmox LXC: bounded process-write samples
(
  set -e
  for sample in 1 2; do
    date -u +'%Y-%m-%dT%H:%M:%SZ'
    pid=$(systemctl show pulse --property=MainPID --value)
    case "$pid" in
      ''|0|*[!0-9]*) printf 'No running Pulse PID; sample unavailable.\n' >&2; exit 1 ;;
    esac
    TZ=UTC ps -p "$pid" -o pid=,lstart=
    sudo awk '
      $1 == "write_bytes:" || $1 == "cancelled_write_bytes:" { print; fields++ }
      END { if (fields != 2) exit 1 }
    ' "/proc/$pid/io"
    if [ "$sample" -eq 1 ]; then sleep 60; fi
  done
)
```

Compare `write_bytes` only when both samples have the same PID and process
start time, no restart occurred, and the counter did not decrease. Divide the
byte difference by the **actual elapsed seconds**. This is storage-accounted
process I/O, not filesystem growth or physical SSD wear; cancelled writes and
background writeback can differ from device measurements. Keep
`cancelled_write_bytes` alongside it, not as proof of bytes reaching the drive.
An extrapolated GB/day rate is a projection of that short window, not a measured
day's total. Preserve the window and units with the result.

For **Docker / Compose**, run this on the Docker host, replacing `pulse` with
the running container name. It reads only the start time and selected statistics,
not the container environment or configuration.

```bash
# Docker: bounded container statistics
(
  set -e
  for sample in 1 2; do
    date -u +'%Y-%m-%dT%H:%M:%SZ'
    docker inspect --format 'Started={{.State.StartedAt}}' pulse
    stats=$(docker stats --no-stream --format \
      'CPU={{.CPUPerc}} Memory={{.MemUsage}} BlockIO={{.BlockIO}}' pulse)
    if [ -z "$stats" ]; then
      printf 'Container statistics unavailable; no zero inferred.\n' >&2
      exit 1
    fi
    printf '%s\n' "$stats"
    if [ "$sample" -eq 1 ]; then sleep 60; fi
  done
)
```

Docker **BlockIO** is cumulative read / write activity, not bytes per second;
compare its write side only across the same uninterrupted container run. Its
displayed units are rounded. Container, process and whole-device counters have
different scopes and must not be added together or compared as interchangeable
measurements. A failed, denied or empty read is unavailable, not zero activity.

Keep persistent data on durable storage. Do not delete or truncate history,
incident, queue or audit files, remove database indexes, or move the data
directory to tmpfs as a diagnostic workaround. These actions can lose evidence
or protections without fixing the writer. Do not post databases, profiles, full
`/proc` dumps, container configuration or raw environments. Share only the bounded
measurements, workload, versions and a relevant manually redacted error; follow
[Getting Help](#-getting-help) for any additional evidence.

### Notifications

#### No alert when Pulse, power or internet goes down

Pulse cannot send a notification while its host is stopped or its outbound
network is unavailable. A local delivery-health warning is not an external
outage detector, and a successful **Send test** does not prove outage coverage.

- In **Alerts → Notifications → External watchdog**, configure a
  Healthchecks-compatible **success ping URL** and save the configuration.
  Keep the URL secret; do not include it in screenshots or support reports.
- Run the watchdog outside the failure you want to detect. Another machine on
  the same power supply or internet connection does not cover a whole-site
  outage. Its notification destination must also remain reachable independently
  of that site.
- At the watchdog, configure a one-minute period and a three-minute grace
  period, and enable its notification integration. Pulse sends a heartbeat
  every minute and a `/fail` signal if its monitoring loop stalls. For
  Healthchecks simple schedules, a missing heartbeat becomes down after
  **period plus grace**: approximately four minutes after the last success,
  not three. Recipient delivery can take longer. See
  [Healthchecks timing and notification concepts](https://healthchecks.io/docs/).
- Verify in an authorised test environment: confirm incoming heartbeats at
  the watchdog, stop the test Pulse instance, and check that the intended
  recipient actually receives the external alert. Restore Pulse and verify
  heartbeat recovery. Test loss of internet separately if you need that
  coverage; a stopped-process test does not prove it.
- To retire the check, pause or remove it at the watchdog too. Clearing the
  URL in Pulse stops its signals but does not pause the remote check.

A healthy heartbeat indicates Pulse monitoring-loop progress, not successful
delivery of every resource alert or external reachability of your services.
Continue checking delivery activity for destination failures.

#### Test succeeds but real alerts are missing

A test sends directly to its destination: it skips the persistent delivery queue
and is not listed in **Recent delivery activity**. It does not prove that a real
alert was generated, routed or delivered to the intended recipient.

- Open **Alerts → Notifications**. If **Notifications are paused** is shown,
  configured destinations and a successful test do not enable real delivery.
  Turn delivery on there only when you intend to send alerts.
- Check the affected alert, the destination's **Enabled** state, minimum alert
  severity and tag filters. Review quiet hours and any mute, acknowledgement or
  maintenance policy before treating an absent attempt as a transport failure.
- Use **Recent delivery activity** to correlate the original alert, destination
  and absolute timestamp, including held-notification reasons. An empty window
  is not proof of healthy delivery; an **unavailable** read is not an empty log.
  Do not create an outage or repeat a notification storm to populate it.

#### Recover retained delivery failures

Pulse shows a delivery warning for failed or dead-lettered notifications in its
persistent queue, not for every recoverable retry. **Recent delivery activity**
includes safely redacted provider errors; completed attempts remain for 7 days
and dead-letter attempts for 30 days. Start with the failure class and timestamp:

| Failure | Check before retrying |
| --- | --- |
| Authentication | Destination credentials and account permissions, locally; never post them. |
| Rate limited | Provider limits and delivery volume; repeated tests or retries can make this worse. |
| Connectivity | DNS, firewall, proxy and reachability from the Pulse server, not just your browser. |
| TLS | Certificate trust, expiry and hostname matching; do not disable verification to diagnose it. |
| Configuration / rejected | Enabled destination, required fields and the provider's endpoint or payload requirements. |
| Server error / unknown | Destination service status and a relevant, bounded local error excerpt. |

Save the corrected destination settings and send one test; check receipt at the
intended destination. **Retry retained deliveries** gives terminal failures a
fresh retry budget, but a destination that accepted an earlier attempt may
receive a duplicate. Review the confirmation's delivery count and provider
limits before retrying. A successful test does not itself retry retained items.

Use **Dismiss retained failures** only when those deliveries should not be sent.
Dismissal clears the warning without retrying them; delivery history remains.
Neither action deletes the audit trail. Do not delete `notification_queue.db`
or audit data to clear the warning.

#### Emails not sending

Follow [retained-failure recovery](#recover-retained-delivery-failures) first.
Check SMTP host, port, sender, recipients, authentication and TLS settings in
**Alerts → Notifications** against your provider's requirements (some providers
require an app password). Keep passwords in the settings form, not a diagnostic
command or report. If the delivery error is insufficient, inspect
[bounded notification logs](#inspect-notification-logs) locally.

#### Webhooks failing

Follow [retained-failure recovery](#recover-retained-delivery-failures) first.
Use the failure class and HTTP status to check the provider's endpoint and
payload requirements. Verify reachability from the Pulse server. For an intended
private destination, review **Settings → System → Network → Webhook Security**;
do not broadly weaken network or TLS controls just to make a test pass.

Prefer the redacted delivery error over raw provider response bodies. A provider
can echo credentials or private content in its response; do not post it wholesale
or enable debug logging just to collect it. If needed, inspect
[bounded notification logs](#inspect-notification-logs) and share only the
consequential, manually redacted error.

### TrueNAS

#### "TrueNAS service unavailable"
- Ensure TrueNAS was added in **Settings → TrueNAS** with a valid HTTPS URL,
  API key, and the username that owns the key.
- Check that the TrueNAS system is reachable from the Pulse server (default
  HTTPS port).
- Verify the API-key owner has read access, then use **Test Connection** in
  Pulse. TrueNAS 25.04 and later should report the `jsonrpc-websocket`
  transport; TrueNAS 26 removed the former `/api/v2.0` REST endpoints.

#### TrueNAS pools/datasets not appearing
- TrueNAS data appears in the unified resource model and may take one configured
  polling cycle (60 seconds by default) to appear.
- Check **Infrastructure** (TrueNAS host), **Storage** (pools/datasets), and **Recovery** (snapshots/replication).
- For data that stops refreshing, use the [TrueNAS polling checks](TRUENAS.md#stale-truenas-data)
  before testing or restarting. A stale badge is not proof of an invalid key,
  and a successful connection test is not proof that collection has recovered.

### Navigation (v6)

#### Old bookmarks don't work
- Legacy URLs (`/proxmox`, `/docker`, `/kubernetes`, `/hosts`, `/services`) are not supported in v6.
- Update bookmarks to canonical routes. See [Migration Guide](MIGRATION_UNIFIED_NAV.md).

### Relay / Mobile

#### Relay showing "Disconnected"
- Confirm a valid Relay, Pro, grandfathered Pro+, or Cloud license is active (**Settings → Plans & Billing**).
- Check Pulse server can reach the relay server (outbound WebSocket to `relay.pulserelay.pro`).
- Review logs: `journalctl -u pulse | grep relay` or `docker logs pulse | grep relay`.

---

## 🛠️ Advanced Diagnostics

### Inspect Notification Logs

Prefer **Recent delivery activity** in **Alerts → Notifications**. If a local log
is needed, run only the command for your deployment, on the Pulse host with an
account authorised to read its logs. For Proxmox LXC, run the systemd command
inside the Pulse container, not on the Proxmox host. Adjust the time window to
the original incident and substitute your actual service or container name
(`pulse-backend` on some older systemd installs). These examples read at most
200 records from the last 15 minutes; they do not follow the log or send a test.

```bash
# systemd / Proxmox LXC
journalctl -u pulse --since '15 minutes ago' --lines 200 --no-pager
```

```bash
# Docker
docker logs --since 15m --tail 200 pulse
```

Docker can write application logs to either stdout or stderr; inspect both.
Do not pipe the reader into `grep email`: it can miss SMTP or webhook errors
and hide a failed read behind a matching partial line. A nonzero reader exit,
access error or missing service/container is a failed read, not “no delivery
errors”. Even a successful empty read is inconclusive: the window, retained
logs or selected instance may differ.

These local excerpts are **not sanitised**. Do not post them wholesale. Share
only the relevant timestamp, method, HTTP status or SMTP error code and a
manually redacted error. Remove credentials, cookies, secret URLs, addresses
and private host or personal information, including anything echoed by the
provider. Never upload full environments, configuration, a queue database or
audit data. See [Getting Help](#-getting-help).

### Correlate Logs with Requests

For a failed HTTP API request, inspect its response in your authenticated
browser's **Developer tools → Network** panel. Copy only the `X-Request-ID`
response header, if present, and keep the HTTP status and time. Do not copy a
session cookie, **Copy as cURL** command or full network export into a report.
WebSocket upgrades do not pass through this request-ID middleware.

Service logs normally use JSON (`"request_id":"abc123"`); console logs may use
`request_id=abc123`. Search for the literal ID value so both formats work.
Replace `abc123` below with the response's ID. Run only the command for your
deployment, on the Pulse host using an account authorised to read its logs.
These examples limit collection to the last 15 minutes and 1,000 lines; adjust
the time window to the original incident rather than repeating the failed action.

```bash
# systemd / Proxmox LXC
set -o pipefail
REQUEST_ID='abc123'
journalctl -u pulse --since '15 minutes ago' --lines 1000 --no-pager |
  grep -F -- "$REQUEST_ID"
```

```bash
# Docker
REQUEST_ID='abc123'
if pulse_logs=$(docker logs --since 15m --tail 1000 pulse 2>&1); then
  printf '%s\n' "$pulse_logs" | grep -F -- "$REQUEST_ID"
else
  printf '%s\n' "$pulse_logs" >&2
  false
fi
```

A log-reader failure is not an empty search result: resolve any access or
container/service error locally first. Even a successful read with no match
does not prove the request succeeded. At the default log level, this middleware
logs HTTP 5xx failures but not successful requests; HTTP 4xx failures are logged
at debug level. The selected window, retained logs or deployment may also differ.
Keep the original response status, time and ID even when there is no matching log;
do not enable debug logging or retry a state-changing request just to fill that gap.

These local excerpts are **not sanitised**. Before sharing a relevant line,
remove credentials, cookies, secret URLs and private host, network or personal
information. See [Getting Help](#-getting-help) for safe evidence collection.

### Check Permissions (Proxmox)
If Pulse can't see VMs or storage, check the user permissions on Proxmox:
```bash
pveum user permissions <user>@pam
```
At minimum, ensure the user/token has read access for inventory and metrics:

- `Sys.Audit`
- `Datastore.Audit`

For VM guest agent features on PVE 9+, prefer:

- `VM.GuestAgent.Audit` — required for disk usage and guest info
- `VM.GuestAgent.FileRead` — required for accurate memory monitoring (excludes buff/cache)

For PVE 8 only, use `VM.Monitor` instead of the `VM.GuestAgent.*` privileges.

Note: The built-in `PVEAuditor` role cannot be modified. Create a custom role (e.g. `PulseMonitor`) with the above privileges added, and assign it to your Pulse API token. After upgrading to PVE 9, add the `VM.GuestAgent.*` privileges and remove legacy `VM.Monitor` from the custom role.

**Rocky Linux / RHEL VMs**: The default qemu-guest-agent configuration may block file-read RPCs (`guest-file-open`, `guest-file-read`, `guest-file-close`). If memory or disk data is missing for these VMs, check `/etc/sysconfig/qemu-ga` and ensure those operations are not blocked, then restart the agent. Refer to your distro's qemu-guest-agent documentation for the exact config syntax.

### Proxmox pending-update access

A successful inventory check does not establish access to the package-update list.
Pulse requests `GET /nodes/{node}/apt/update` with its configured Proxmox credential.
[Proxmox's endpoint implementation](https://github.com/proxmox/pve-manager/blob/614bede5d65599c67e068cbf18d49717ea8ab33b/PVE/API2/APT.pm)
requires `Sys.Modify` on `/nodes/{node}` for that GET; `Sys.Audit` alone is not sufficient
in that implementation. Check the API requirements for your installed Proxmox version.

`Sys.Modify` is broader than read-only monitoring: the same privilege also authorises
the separate POST that refreshes package indexes. Do not add it automatically to a
monitoring role just to obtain an update badge. Keeping the narrower role and an
unavailable update check is a valid choice; unavailable is not a confirmed zero.
Older Pulse wording that says “Sys.Audit permission required” does not identify the
actual missing privilege.

For diagnosis, compare the affected node and endpoint using the exact credential
configured in Pulse, not an administrator's browser session. Keep TLS verification
enabled and credentials private; report only the HTTP status and a redacted denial
or whether `data` is empty/nonempty. Do not use POST, Refresh or Upgrade as a test.
If the matching GET succeeds but Pulse remains unavailable after its next update
check, report the Pulse version and displayed check time/status separately; API
success alone does not confirm the Pulse display has recovered.

### Recovery Mode

For a forgotten local password, follow [I forgot my password](#i-forgot-my-password)
above, using the steps for your deployment. Enter the host-only bootstrap token
in that instance's setup screen; do not paste it into a command or a report.
For OIDC, SAML or proxy login, use the identity-provider or administrator path
described there instead.

The advanced `/api/security/recovery` API creates a **browser-bound recovery
session**, not a server-wide authentication bypass. Despite its legacy name,
`disable_auth` does not disable authentication for other clients. Recovery
sessions work only over direct loopback requests; remote and reverse-proxy
requests cannot use them. A successful curl response does not unlock a separate
browser: the session cookie belongs to the client that made the request.
`enable_auth` clears that recovery session; it does not reset a password.

Do not transfer recovery cookies between clients or paste recovery tokens into
command arguments, URLs, screenshots or GitHub threads. The password-reset steps
above avoid that credential-handling detour.

---

## 🆘 Getting Help

If you're still stuck:

1. **Keep the original evidence**: note what you did, when it happened and the
   exact error. Do not repeat an update, outage or notification storm merely to
   reproduce it. A failed update banner does not prove the action left the
   target unchanged; check its current state before another attempt.
2. **Identify the affected version**: give the running Pulse and relevant agent
   versions, not just the version before an upgrade. For Docker, include the
   running image tag or digest. If installation never started Pulse, give the
   attempted release and public installer/helper source, or say "unknown".
3. **Choose relevant, safe evidence**: if Pulse is running and collection is
   safe, use **Settings → Diagnostics → Export for GitHub (sanitized)** for
   connection or data failures. For a visual problem, a screenshot or the exact
   error may be enough. If logs are needed, inspect a bounded local excerpt
   (`journalctl -u pulse -n 100 --no-pager` or `docker logs --tail 100 pulse`),
   not a full configuration or data-directory upload.
4. **Review before posting**: even a sanitized export or screenshot can contain
   identifying details. Remove credentials, session cookies, webhook URLs and
   private host, network or personal information. Never post bootstrap/recovery
   tokens, `.env` files, private keys or an unsanitized export.
5. **Use the appropriate thread**: [GitHub Issues](https://github.com/rcourtman/Pulse/issues)
   for a bug, or [Discussions](https://github.com/rcourtman/Pulse/discussions) for
   a setup question. Add new evidence to an existing matching report rather
   than opening a duplicate. Do not refile information you have already supplied.
