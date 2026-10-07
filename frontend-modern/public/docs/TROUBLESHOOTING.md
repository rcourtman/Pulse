# 🔧 Troubleshooting Guide

## ⚡ Quick Fixes

### I forgot my password

Pulse does not provide an email password-reset flow. Choose the path that
matches how this self-hosted instance authenticates:

- **Local Pulse administrator username and password**: recovery requires
  authorised access to the host or deployment's credential source. Follow the
  steps below; keep the existing username.
- **OIDC, SAML, or proxy authentication**: contact the identity-provider or
  Pulse administrator. If the administrator deliberately kept local login as a
  fallback, they can open the Pulse URL with `?show_local=true`; this only
  reveals the existing local form and does not reset credentials.
- **Temporary lockout**: wait for the displayed lockout to expire. An authorised
  administrator can use the [manual recovery API](../SECURITY.md#manual-recovery-admin);
  this does not reset a password or bypass SSO.

#### Recover the existing local administrator login

Do not delete `.env` or repeat first-time setup to recover a password. The file
can also hold deployment settings, and setup replaces the primary API token.
Replace only the password in its active credential source instead, preserving
the username, API tokens, encryption keys, monitoring configuration and data.
This is login recovery, not a complete response to a compromised account;
existing sessions and API tokens need separate review if credentials leaked.

1. **Find the active credential source locally.** Deployment-supplied
   `PULSE_AUTH_USER` and `PULSE_AUTH_PASS` take precedence over Pulse's `.env`.
   Do not post full service units, container inspections or resolved Compose
   configuration; they can contain secrets.
   - **Docker**: the generated file is in the mounted data directory, normally
     `/data/.env` inside the container. If Compose, `--env-file`, Kubernetes or
     another manager supplies the password, edit that managed source instead.
   - **Systemd**: check the active service's unit, environment files and
     drop-ins privately. Root-run setup can also save credentials in
     `/etc/systemd/system/pulse.service.d/override.conf` (or the corresponding
     `pulse-backend.service.d` path on legacy installs). Editing only
     `/etc/pulse/.env` will not override those values. A custom
     `PULSE_DATA_DIR` changes the generated file's location.
   - **Proxmox LXC**: perform the systemd steps inside the Pulse container, not
     on the Proxmox host. Use its console or `pct enter <ctid>`.
2. **Prepare a new password hash privately.** Use a trusted local bcrypt tool
   that prompts for a password of at least 12 characters, never one that needs
   the password in its command arguments. For example, if Apache's `htpasswd`
   is installed on your trusted administration machine:

   ```bash
   (
   set -eu
   umask 077
   recovery_dir="$(mktemp -d "${TMPDIR:-/tmp}/pulse-password.XXXXXX")"
   htpasswd -nB -C 12 pulse-recovery > "$recovery_dir/password-record"
   printf 'Private password record saved in %s/password-record\n' "$recovery_dir"
   )
   ```

   This prints only the file location. Open the record in a private editor and
   copy the complete 60-character hash after `pulse-recovery:`; that label is
   not a new Pulse username. Do not post the record or hash. If the tool fails,
   stop rather than saving a partial hash or putting the password in a shell
   command.
3. **Back up and edit only the active password setting.** Keep an owner-only
   backup of each file you change, then replace `PULSE_AUTH_PASS` in a private
   editor. Keep `PULSE_AUTH_USER` and unrelated settings unchanged. In a Pulse
   `.env`, put the hash in single quotes with literal `$` characters. Compose
   YAML has different interpolation rules; follow the
   [authentication guide](CONFIGURATION.md#private-docker-authentication-file)
   for the source you actually use. If a systemd drop-in supplies the password,
   update it and the generated authentication file consistently so a later
   restart or reload does not restore the old value. Preserve file permissions
   and ownership; hashes and backups are sensitive too.
4. **Apply through the existing deployment.** For a generated Docker data-file
   change, restart the same container. Changing Docker's managed environment
   requires its recreate/redeploy operation, not just `docker restart`; preserve
   the same image, mounted data and other settings. For systemd, reload the
   service manager after a unit or drop-in change, then restart the active Pulse
   service during a suitable maintenance window. Do not remove a data volume,
   reinstall Pulse or re-enrol agents.
5. **Verify the login.** Use a fresh browser session to sign in with the same
   local administrator username and new password, then check that monitoring
   and agent connections remain intact. If it still fails, stop and reconcile
   the effective source and any lockout; do not delete more state. Restore the
   private backup through the same deployment path if you need to undo the
   change. Remove the temporary password record after verification,
   and retain or dispose of the backup under your normal credential policy.

#### A fresh install's bootstrap token was missed

If no local password has ever been set, there is nothing to reset. Follow
[first login](INSTALL.md#step-1-get-the-token) to read the existing bootstrap
token for that instance, using its actual data directory. Enter it only in the
Pulse setup screen, not in command arguments, URLs or issue reports.

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

Identify which connection failed before changing ports or credentials:

- **Your browser cannot open Pulse:** check that the intended Pulse service or
  container is running, then its actual listening port, container port mapping
  and reverse proxy target. Pulse's default port is **7655**, but an existing
  deployment can use a different one. Use the [port-change checks](#port-change-didnt-take-effect)
  and [reverse proxy guide](REVERSE_PROXY.md) for that path; do not broadly open
  firewall access or expose Pulse directly to bypass a proxy.
- **Pulse opens, but a monitored platform request is refused:** check the
  affected connection's saved hostname, scheme and port in **Settings →
  Infrastructure**, and the network path **from the Pulse server**. Browser
  access to the platform is a different path. Proxmox VE normally uses HTTPS
  on **8006**; PBS uses HTTPS on **8007**. Other platforms use their saved
  endpoint, not an assumed Proxmox port. Changing Pulse's listening port does
  not repair this connection.

A connection refusal is not an authentication response (**401/403**) or a
certificate-validation error. Keep the original time and redacted error;
do not replace credentials or disable TLS verification on a refusal alone.
For a page that opens but has missing readings, use the
[missing-data checks](FAQ.md#no-data-showing) instead of resetting the connection.

### CORS errors

CORS controls whether a browser can read a response from a different origin.
It is not Pulse's connection to Proxmox, PBS or TrueNAS, and it does not replace
authentication, CSRF protection or TLS.

In your browser's developer tools, compare the page's origin with the failed
API request's origin and HTTP status. An origin consists of **scheme, host and
port**: HTTP and HTTPS, or two different ports, are different origins. Review
the original console error locally; a certificate failure, login redirect or
401/403 response needs its own correction, not a broader CORS allowlist.

- **Pulse UI and API share a public origin:** a same-origin reverse proxy needs
  no CORS exception. Keep the API behind that proxy and correct its routing or
  [trusted forwarded headers](REVERSE_PROXY.md#before-configuring-the-proxy);
  do not expose the backend or add wildcard response headers to bypass it.
- **A separate trusted browser app needs API access:** in **Settings → System →
  Network → CORS Allowed Origins**, list only that app's exact origin, for
  example `https://app.example.com:8443`. Do not include a path, trailing slash
  or a hostname pattern. Multiple exact origins are comma-separated.
  `ALLOWED_ORIGINS` overrides the saved setting; removing that environment
  override does not erase an existing saved allowlist. Re-open settings after
  saving and verify the effective value and the ordinary browser request.

An empty policy grants no cross-origin browser permission. `*` allows any
origin **without credentialed browser access**; it cannot fix a request that
needs a browser session cookie. Do not disable authentication, CSRF protection
or TLS verification to make such a request work. Iframe embedding and proxy
authentication have separate settings; CORS is not their trust boundary.

Keep cookies, authorization headers, API tokens and full network exports
private. A report needs only the redacted error, HTTP status and relevant
origins, not a credential-bearing request or HAR file.

---

## 🔍 Common Issues

### Authentication

#### "Invalid username or password" after setup

When the local login form reports this error, stop repeated password guesses:
failed attempts count against both the username and client IP. Privately check
the existing username and the active credential source. Deployment-supplied
`PULSE_AUTH_USER` and `PULSE_AUTH_PASS` take precedence over Pulse's generated
`.env`, so changing only that file may leave the running password unchanged.

A bcrypt hash must be complete (60 characters). Compose YAML interpolation,
Docker `--env-file` and Pulse's generated `.env` use different quoting rules;
do not blindly replace every `$` with `$$`. Follow the
[authentication-file guidance](CONFIGURATION.md#private-docker-authentication-file)
for your actual source, keeping hashes and resolved deployment configuration
private. If the password is genuinely lost, use
[deployment-specific recovery](#i-forgot-my-password), not setup or re-enrolment.

#### Cannot login / 401 Unauthorized

Identify which request failed before clearing cookies or changing credentials:

- **Browser session:** a 401 on an ordinary UI request can mean a missing or
  expired session, not a wrong password. Open the same public Pulse URL in a
  fresh browser session and sign in through the configured method. Preserve any
  working administrator session; do not clear unrelated sites' cookies.
- **SSO or proxy login:** use the identity-provider or Pulse administrator's
  recovery path. Check the [proxy authentication guide](PROXY_AUTH.md) when the
  proxy signs you in but Pulse rejects the request. A local lockout reset does
  not repair an identity-provider account or missing trusted proxy headers.
- **Local login form:** an **Account locked** response gives the remaining wait.
  Lockout applies separately to username and client IP, normally for 15 minutes;
  clearing cookies does not reset it. A **Too many requests** response (429) is
  rate limiting, not proof that the password is wrong. Wait rather than retrying
  rapidly, restarting Pulse or disabling authentication.
- **API client:** a missing or invalid API token can cause 401 even when browser
  login works. Check the token's status and intended permissions privately.
  A 403 can instead mean insufficient permissions or failed CSRF protection;
  do not disable those checks or reset the password to repair API access. Use
  the [private header-file examples](../SECURITY.md#usage), never a token or
  session cookie pasted into a command or URL.

Manual lockout reset requires an authenticated administrator with
`settings:write` authority; session requests also require CSRF protection. The
[manual recovery API](../SECURITY.md#manual-recovery-admin) resets one username
or IP identifier at a time, not a password, SSO account or every lockout. Do not
assume resetting the username also clears a separately locked IP.

For a report, retain the time, sign-in method, redacted error, HTTP status and
request path (without query strings). Keep passwords, hashes, cookies, tokens,
full headers, **Copy as cURL** commands and network exports private. A 401 alone
does not justify deleting configuration, recreating the data volume or
re-enrolling agents.

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
- Read the disk value's explanation and observation time first; a dash is not
  proof that the agent is missing.
- Check the guest-local service, current VM Options and the configured API
  token's read permissions. Schedule any setup change or restart outside backups.
- Do not run guest-agent probes during backup freeze/thaw. An OK backup task
  or an absent lock does not confirm thaw.
- See [VM Disk Monitoring](VM_DISK_MONITORING.md) for the passive host preflight;
  it does not verify a fresh disk poll. For an affected installation, follow
  the [manual backup precaution](VM_DISK_MONITORING.md#backup-safety) for your
  actual server deployment: **systemd or Docker/Compose**. A planned pause is
  **not incident recovery**, and stopping Pulse does not cancel a guest-agent
  request already issued. If an existing operation's state is unknown, do not
  start a backup on the strength of a stopped server or an elapsed wait.
- Keep Pulse stopped until the backup has ended and independent post-backup
  checks confirm **thaw, fresh successful workload writes to every filesystem
  covered by the backup, and workload liveness**. Use established safe checks,
  independent of Pulse and the QEMU Guest Agent; a console connection or a
  successful read alone is not enough. If any check fails or is unavailable,
  leave Pulse and its automatic updater paused and use the guest/platform's
  recovery procedure, not new probes, forced writes or another backup.
- After all checks pass, restore **only services and timers that were active
  before the pause**, following the deployment-specific precaution. Unknown
  pre-pause states are not permission to start them. **Pulse monitoring and
  alerts are unavailable while stopped**; arrange independent outage coverage.

#### Backup health disagrees with PBS

A visible or Verified PBS backup is not the same reading as a workload's
Coverage posture. Use the [backup health checks](PBS.md#backups-are-visible-but-coverage-says-unprotected)
to compare one affected row's explanation, Job, History and Access with the
matching native PBS record. Do not run a new backup, restart or clear history
just to diagnose the disagreement.

#### Temperature data missing
- Compare the affected host's active agent version, last report, sensor and
  observation time. A current server or another sensor's value is not evidence
  that this reading is available.
- Use the [bounded local reading check](TEMPERATURE_MONITORING.md#check-existing-linux-readings-safely)
  on the monitored host, not the Pulse container. Linux agents can use existing
  recognised CPU/SoC sysfs readings without `lm-sensors`; unavailable is not zero.
- Do not run automatic hardware detection, load drivers, reboot or loosen SSH
  restrictions merely to fill a temperature row. Retain output privately and
  follow the [platform-specific guide](TEMPERATURE_MONITORING.md) for any needed
  setup change during a maintenance window.

#### Container CPU differs from Docker or Podman stats

Pulse uses the runtime host's total CPU capacity; `docker stats` and
`podman stats` normally use 100% per logical CPU. Divide a per-CPU stats reading
by the **reported runtime host CPU count**, not the physical Proxmox host's
count when the engine runs inside a VM. Compare the same container and time
interval in **CPU History**, not just a rounded table label. A dash or missing
History is unavailable, not zero; an Active connection alone is not fresh CPU
evidence.

See [container CPU readings](DOCKER.md#container-cpu-readings) for worked
examples, rounding and alert limits, and a passive comparison. Do not restart
workloads, create CPU load or lower alert thresholds to test this.

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

#### Memory use keeps growing

A Proxmox LXC memory chart covers the container, not just Pulse. It can include
other processes and filesystem cache; Docker's displayed memory also uses its
own cache accounting. Neither is interchangeable with Pulse's resident memory
(RSS), a Go heap size, or virtual address space (`VSZ` / `VmSize`). A large
container reading alone does not establish a Pulse memory leak.

For a responsive **Linux systemd / Proxmox LXC** install, take this small,
read-only sample at a normally occurring high point. Run it **inside the Pulse
container** for LXC, not on the Proxmox host. Substitute the actual service name
(`pulse-backend` on older installs) in both `systemctl` calls. Use an account
authorised to read these counters; a failed read is unavailable, not zero.

```bash
# systemd / Proxmox LXC: Pulse resident-memory sample
(
  set -eu
  date -u +'%Y-%m-%dT%H:%M:%SZ'
  pid=$(systemctl show pulse --property=MainPID --value)
  case "$pid" in
    ''|0|*[!0-9]*) printf 'No running Pulse PID; sample unavailable.\n' >&2; exit 1 ;;
  esac
  identity=$(TZ=UTC ps -p "$pid" -o pid=,lstart=)
  if [ -z "$identity" ]; then
    printf 'Process identity unavailable.\n' >&2; exit 1
  fi
  counters=$(awk '
    $1 == "VmRSS:" || $1 == "RssAnon:" || $1 == "RssFile:" ||
    $1 == "RssShmem:" || $1 == "VmSwap:" {
      if (NF != 3 || $2 !~ /^[0-9]+$/ || $3 != "kB" || seen[$1]++) exit 1
      print; fields++
    }
    END { if (fields != 5) exit 1 }
  ' "/proc/$pid/status")
  current_pid=$(systemctl show pulse --property=MainPID --value)
  current_identity=$(TZ=UTC ps -p "$pid" -o pid=,lstart=)
  if [ "$pid" != "$current_pid" ] || [ "$identity" != "$current_identity" ]; then
    printf 'Pulse changed during collection; discard this sample.\n' >&2; exit 1
  fi
  printf 'Pulse process (PID and UTC start): %s\n%s\n' "$identity" "$counters"
)
```

Linux labels these values `kB`, meaning 1,024 bytes; divide by 1,024 for MiB.
`VmRSS` is resident process memory, split into anonymous (`RssAnon`),
file-backed (`RssFile`) and shared-memory (`RssShmem`) pages. `VmSwap` is swapped
private anonymous memory, not additional resident memory or all container swap.
These counters are approximate and not an atomic snapshot; a nonzero
`RssFile` is not proof of a leak, and `RssAnon` is not specifically the Go heap.
Missing fields on an older kernel make this recipe fail rather than invent
values. If it fails or Pulse stops, retain that fact; do not loosen access
controls or keep retrying against an unresponsive installation.

Keep the sample time, PID/start time, running version, container memory limit,
fleet size, polling interval and whether dashboards were open. If safe, compare
another naturally occurring point with the **same PID and process start time**;
record any restart or upgrade as a different run, not evidence that the cause
was fixed. For Docker, the [bounded container statistics below](#excessive-cpu-writes-or-database-growth)
retain the container start time and memory usage but do not measure Pulse RSS;
do not assume its PID 1 is Pulse.

Do not restart Pulse, drop caches, force garbage collection, change memory limits
or delete history just to obtain a lower reading. If the container is near its
limit or the host is unresponsive, stop sampling and prioritise safe recovery.
Do not post a heap dump, profile, full `/proc` status or process command line:
they can contain private details. Share only these counters and the relevant
context after local review; existing screenshots remain useful if collection
is unsafe.

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
persistent queue, not for every recoverable retry. Authentication,
configuration and rejected failures stop automatic retries as soon as they are
classified, even with attempts left. A terminal failure can therefore appear
without exhausting the retry budget; waiting alone will not resend it. See
[webhook retry behaviour](WEBHOOKS.md#-delivery-contract) for HTTP exceptions,
transient failures and the normal queued attempt limit.

**Recent delivery activity** includes safely redacted provider errors;
completed attempts remain for 7 days and dead-letter attempts for 30 days. Start with the failure class and timestamp:

| Failure | Check before retrying |
| --- | --- |
| Authentication | Destination credentials and account permissions, locally; never post them. |
| Rate limited | Provider limits and delivery volume; repeated tests or retries can make this worse. |
| Connectivity | DNS, firewall, proxy and reachability from the Pulse server, not just your browser. |
| TLS | Certificate trust, expiry and hostname matching; do not disable verification to diagnose it. |
| Configuration / rejected | Enabled destination, required fields and the provider's endpoint or payload requirements. |
| Server error / unknown | Destination service status and a relevant, bounded local error excerpt. |

Save the corrected destination settings and send one test; check receipt at the
intended destination. **Retained deliveries keep the destination settings saved
when they were queued.** Editing a URL, recipient, credential, header or template
does not replace that saved configuration. A test uses the edited settings;
retrying an old delivery can still use the old endpoint or credential and fail
again. If the old destination must no longer receive data, disable it rather
than relying on a URL edit to redirect queued work.

**Retry retained deliveries** gives all retained terminal failures a fresh retry
budget, not just the destination you tested. Use it only when sending those
original deliveries is still intended and their original settings remain
appropriate, for example after a temporary provider outage. A destination that
accepted an earlier attempt may receive a duplicate. Review the confirmation's
delivery count and provider limits before retrying. A successful test does not
itself retry retained items or prove that their saved settings now work. If
uncertain, leave the failures retained and check the next normally occurring
alert instead; do not use a batch retry to test a settings edit.

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

#### Telegram Test works but real alerts say "message text is empty"

With the built-in Telegram template (no custom template), a blank or incorrect
saved `Content-Type` can make Telegram ignore the JSON body. The HTTP 400 error
does not by itself prove that Pulse omitted the `text` field. Test can supply
the correct header even when real delivery uses the saved one.

In **Alerts → Notifications**, edit that Telegram destination's **Custom
headers**: set one static `Content-Type` header to `application/json` and remove
any conflicting duplicate, including differently capitalised names. Save,
leaving the bot URL, `chat_id`, template and grouping settings unchanged. Check
whether the next normally occurring alert reaches the intended chat and compare
its timestamp with **Recent delivery activity**; do not induce an alert or retry
the old rejected batch to test this edit. Retained jobs keep their original
settings as explained [above](#recover-retained-delivery-failures).

For a custom template, check its JSON and required Telegram fields locally
instead of assuming this built-in-template workaround applies. If reporting a
continuing failure, share only the version, redacted delivery error, timestamp
and whether a custom template is used. Keep the bot URL/token, chat ID and full
notification configuration private.

### TrueNAS

#### "TrueNAS service unavailable"

The exact `truenas_unavailable` error (HTTP **500** or **503**) concerns Pulse's
connection-management handler or configuration persistence, not proof of an
appliance outage or an invalid TrueNAS API key. Keep the existing request's
status/code and time, then inspect a bounded Pulse log excerpt locally. Do not
rotate the key, recreate the connection or weaken TLS verification to clear it.

Follow [TrueNAS error diagnosis](TRUENAS.md#truenas-service-unavailable) to
separate this from an explicitly disabled integration or a failed live test.
For missing or stale readings, use the [polling checks](TRUENAS.md#stale-truenas-data);
a successful **Test Connection** does not prove collection has recovered.

#### TrueNAS pools/datasets not appearing
- TrueNAS data appears in the unified resource model and may take one configured
  polling cycle (60 seconds by default) to appear.
- Open **TrueNAS → Overview** for the appliance, **TrueNAS → Storage**
  (`/truenas/storage`) for pools, datasets and disks, and **TrueNAS → Protection**
  (`/truenas/protection`) for snapshots and replication. These are tabs within
  TrueNAS, not separate top-level Storage or Recovery pages.
- For data that stops refreshing, use the [TrueNAS polling checks](TRUENAS.md#stale-truenas-data)
  before testing or restarting. A stale badge is not proof of an invalid key,
  and a successful connection test is not proof that collection has recovered.

### Navigation (v6)

#### Old bookmarks don't work

Current Pulse uses platform navigation. `/proxmox`, `/docker` and `/kubernetes`
are supported; do not replace them with the retired task-based routes.
Open Pulse at its base URL and use the menu to find the relevant page:

| Menu | Entry route |
| --- | --- |
| Proxmox | `/proxmox/overview` |
| Docker | `/docker/overview` |
| Kubernetes | `/kubernetes/overview` |
| TrueNAS | `/truenas/overview` |
| vSphere | `/vmware/overview` |
| Machines | `/standalone/machines` |

The short-lived top-level `/workloads`, `/storage` and `/recovery` layout is
retired. `/infrastructure` now opens the default workspace, not the former
unified host page. For old `/hosts` or `/services` bookmarks, select the current
platform or Machines page instead of assuming an automatic redirect. PBS
backups are under **Proxmox → Backups**, and TrueNAS snapshots and replication
are under **TrueNAS → Protection**.

Platform-connected hosts can appear under their platform rather than Machines.
If a menu or expected resource is missing, check its saved connection and last
successful collection in **Settings → Infrastructure**; a missing page alone
does not prove a host was deleted. Do not delete connections or re-enrol agents
just to repair a bookmark.

If menu navigation works but reloading the same URL returns a proxy 404, check
the proxy's route handling using [Reverse Proxy Configuration](REVERSE_PROXY.md).
For the current layout, see [FAQ](FAQ.md#how-is-navigation-organised-in-pulse-v6).
The [unified-navigation migration](MIGRATION_UNIFIED_NAV.md) is historical,
not a guide to the current menu.

### Relay / Mobile

#### Relay showing "Disconnected"
- Confirm a valid Relay, Pro, grandfathered Pro+, or Cloud license is active (**Settings → Plans & Billing**).
- Check Pulse server can reach the relay server (outbound WebSocket to `relay.pulserelay.pro`).
- Review logs: `journalctl -u pulse | grep relay` or `docker logs pulse | grep relay`.

---

## 🛠️ Advanced Diagnostics

### Collect diagnostics safely

**Run Diagnostics** in **Settings → Diagnostics** is not just a passive export.
It can make live Proxmox/PBS API and guest-agent requests; results may also come
from a short-lived cache. Do not run it during a backup, freeze/thaw or an
unresponsive-host incident merely to obtain a report. Keep the original errors
and existing observations instead. A successful one-off check does not prove
that normal monitoring has recovered or that a guest has thawed.

After a result is displayed, the download buttons reuse that result without
running the checks again. They save a local JSON file, **not an upload**:

- **Full (private)** retains identifying diagnostic details. Keep it private;
  do not attach it to a public issue or discussion.
- **GitHub (review first)** replaces selected infrastructure and token
  identifiers, private filesystem paths and raw disk-response fields. Counts,
  measurements, collection times and diagnostic states remain useful for
  triage. It is not a guarantee that every free-text error or future field is
  free of private information. Open the file locally and review it before sharing.

Remove credentials, cookies, secret URLs, private host/network or personal
information, including details echoed in errors or notes. Do not paste a
**Copy as cURL** command, full network export, configuration or data directory.
Share only evidence relevant to the symptom; a screenshot or exact redacted
error may be enough. See [Getting Help](#-getting-help).

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

If Pulse cannot see VMs, storage or guest readings, start with the **user,
realm and token ID configured for the affected Pulse connection**, not a
guessed account or your administrator login. Use the existing connection
settings; do not dump the server configuration or expose the token secret.

With privilege separation enabled, the token's effective access is the
**intersection of the user and token permissions**: both must allow the
required privilege on the affected resource path. Ask the Proxmox
administrator to inspect both sides, including inherited ACLs and their
propagation, for that VM, storage or node. An administrator session or
user-only permission listing does not prove the token has access.

Inspect the existing denial and compare it with the relevant read privileges:

- Inventory and metrics: `Sys.Audit` and `Datastore.Audit`, on the relevant
  node/storage scope.
- VM filesystem usage and guest information on PVE 9+: `VM.GuestAgent.Audit`.
- Guest memory through `/proc/meminfo` on PVE 9+: also
  `VM.GuestAgent.FileRead`.
- On PVE 8, `VM.Monitor` is the legacy guest-agent fallback, not an extra
  requirement for PVE 9+.

The built-in `PVEAuditor` role cannot be modified. Where a read privilege is
actually missing, use the existing narrow custom role and matching scoped
user/token ACLs. **Do not disable privilege separation, grant Administrator or
add guest execution/write privileges** to diagnose missing data. Do not rerun
setup or replace a token as a permissions test: setup can rotate an existing
credential. Make any necessary access repair in your normal maintenance
window, outside backups, then observe normal polling without manual
guest-agent probes. `Sys.Modify` is not a read-only monitoring privilege; see
[pending-update access](#proxmox-pending-update-access) before considering it.

Keep token secrets and full permission listings private. A report needs only
the relevant missing privilege and a redacted denial, not private identities,
resource paths or the complete ACL tree. Successful inventory does not prove
that guest readings are fresh, or that a guest has thawed.

**Rocky Linux / RHEL VMs**: File-read restrictions in `/etc/sysconfig/qemu-ga`
can explain missing guest memory; they do not by themselves establish why disk
usage is absent. Review the guest's policy before changing it. Schedule any
allowlist change or agent restart outside backups, following the guest OS's
documentation. See [VM Disk Monitoring](VM_DISK_MONITORING.md) for the distinct
permissions and backup safety boundary.

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

For a forgotten local administrator password, follow
[I forgot my password](#i-forgot-my-password) above to update its active
credential source without deleting configuration or repeating first-time setup.
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
   safe, use **Settings → Diagnostics → GitHub (review first)** for
   connection or data failures, following [safe diagnostics collection](#collect-diagnostics-safely).
   For a visual problem, a screenshot or the exact
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
