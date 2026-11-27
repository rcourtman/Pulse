# ❓ Frequently Asked Questions

## 🛠️ Installation & Setup

### What's the easiest way to install?
If you run Proxmox VE, use the signed LXC installer flow in [INSTALL.md](INSTALL.md).

If you prefer Docker:

Use a pinned image tag such as `rcourtman/pulse:vX.Y.Z` instead of `:latest`.

See [INSTALL.md](INSTALL.md) for all options (Docker Compose, Kubernetes, systemd).

### How do I add a node?
Go to **Settings → Infrastructure → Install on a host** for systems that should
run the unified agent directly.

- **Agent setup**: use the generated command on the target host, entering the
  token only at its private prompt, never in the command itself. Alternatively,
  follow the [private-file preparation](UNIFIED_AGENT.md#private-file-installation-linux-macos-and-nas).
- **Manual/API-backed platforms**: use **Settings → Infrastructure** (**Add infrastructure**) for systems such as Proxmox, PBS, PMG, or TrueNAS that connect over an API instead of running the agent locally.

If you want Pulse to find servers automatically, open **Settings → Infrastructure**, select **Configure discovery** (it reads **Settings** once discovery is on), turn on **Automatic scanning** in the Discovery settings dialog, and then review discovered servers in the same section.

### Why does the copied agent command say no installation was attempted?

That message describes a stopped preparation step, not the health of an agent
already installed. If it follows `stty: command not found`, the private token
prompt cannot prepare the terminal; having an interactive shell does not supply
the missing tool. Do not remove the privacy check, put a token in the command,
generate another token or uninstall the existing agent to get past it.

Use the [agent command failure guide](UNIFIED_AGENT.md#copied-agent-command-fails-before-installation)
to distinguish the prompt, preflight and removal paths. Keep the original error
and existing state; a failed new command does not explain an earlier outage or
automatic-update failure.

### How do I change the port?

Distinguish Pulse's web listener from a container's published host port.
`FRONTEND_PORT` controls the listener (default **7655**); the separate
`PULSE_AGENT_INGEST_PORT` is not the web UI port.

- **Systemd / Proxmox LXC**: change the active Pulse service's managed port
  setting. For LXC, work inside the Pulse container, not on the Proxmox host.
  A unit or drop-in change needs a service-manager reload and a restart of that
  service during a suitable maintenance window.
- **Docker / Compose**: with the default listener, `8080:7655` publishes the UI
  on host port **8080**; it does not require changing `FRONTEND_PORT`. The
  repository's Compose file uses `PULSE_PORT` for this host-side mapping.
  Apply the change through your existing container manager's recreate/redeploy
  operation, preserving the same image, data mount and other settings.
  `docker restart` does not apply a new port mapping. Do not delete a data volume
  or replace your existing deployment with a fresh `docker run` command.

Update the reverse proxy's upstream port separately if needed; do not expose
Pulse directly or broadly open the firewall to bypass it. Follow the
[deployment-specific port checks](TROUBLESHOOTING.md#port-change-didnt-take-effect)
and keep full service environments and container inspections private.

### Does updating the Pulse server update every agent immediately?

No. The server and agent have separate update lifecycles. Eligible agents
check for and apply the server's target version asynchronously. PVE host agents,
legacy agents, agents with auto-update disabled, and agents with failed or
missing update prerequisites need a manual command.

Open an outdated-agent notice or
`/settings/infrastructure?agentDoctor=1` to open **Agent Doctor** and
copy the correct per-host command. The surface does not remotely execute the
update. Use **Settings → Infrastructure → Install on a host** for first installs
and in-place agent reinstalls. See [Unified Agent](UNIFIED_AGENT.md#auto-update).

### Why can't I change settings in the UI?

A warning that names an environment variable means that deployment setting
manages the value, rather than the UI. If it is intentional, keep it and ask
the deployment administrator to change that setting; do not remove unrelated
authentication or security overrides.

To hand that particular setting back to the UI, remove only its override from
the active deployment source. Apply through the same deployment: environment
changes for Docker require recreation/redeployment, not `docker restart`; a
systemd unit or drop-in change needs a service-manager reload and restart.
Preserve the image, data mount, credentials and other settings. Merely editing
a file does not change the running process's environment.

Re-open Settings afterwards and check both the warning and effective value.
Removing an override does not necessarily erase the saved value underneath it
or turn the setting off. For example, a saved CORS allowlist can remain after
`ALLOWED_ORIGINS` is removed. See
[environment precedence](CONFIGURATION.md#common-overrides-environment-variables) and
[CORS checks](TROUBLESHOOTING.md#cors-errors). If no environment warning is
shown, retain the displayed error rather than assuming an override is the cause.
Do not post full environment, Compose or configuration dumps; they can contain
passwords and tokens.

---

## 🔍 Monitoring & Metrics

### What do Pro and Cloud unlock?
Pro and Cloud unlock **hands-on Patrol modes, issue investigation, governed fixes, and verified outcomes** along with the broader operations feature set. Existing legacy Pro+ holders keep their current continuity, but self-hosted pricing no longer sells more monitoring volume. Pulse Patrol is available to everyone on Community with BYOK and provides scheduled, cross-system analysis that correlates real-time state, recent metrics history, and diagnostics to surface actionable findings.

Example output includes trend-based capacity warnings, backup regressions, Kubernetes cluster analysis, and correlated container failures that simple threshold alerts miss.
See [Pulse Intelligence](AI.md), [Plans and entitlements](PULSE_PRO.md), and <https://pulserelay.pro>.

An accepted licence does not install the private Pro runtime. If a paid plan is
active but its features remain unavailable, use the [licence recovery distinctions](PULSE_PRO.md#activation-succeeds-but-pro-features-are-unavailable)
before applying the key again, clearing it or changing the installation.

### What happens to Pulse Mobile and Relay?

Pulse Mobile and Relay retire on **31 March 2027**. Existing paired phones keep
working until then. Relay is no longer sold; existing Relay subscribers receive
Pro features at their current price for as long as their subscription continues.
Those Pro features do not end with the app's retirement.

Relay connects the app, not the web UI. To reach Pulse away from home, use your
own VPN or tunnel. For alerts on your phone afterwards, add an ntfy, Gotify or
Pushover destination under **Alerts** and open Pulse in your phone's browser.
See [Relay / Pulse Mobile](RELAY.md) for the existing pairing and security details.

### Why do VMs show "-" for disk usage?

A dash means filesystem usage is unavailable, **not zero**. Allocated virtual-disk
size is not used space inside the guest. Pulse can read that usage through a
supported QEMU Guest Agent, but a failed read does not establish that the agent
is absent or stopped. Read the explanation and observation time; use the guest's
own filesystem readings rather than treating a retained Pulse value as current.

Do not install, enable or restart an agent solely to clear a disk dash. During a
backup, freeze/thaw or an unresponsive-guest incident, defer setup and live probes.
Follow [Backup safety](VM_DISK_MONITORING.md#backup-safety); an OK backup does not
prove thaw. Keep the monitoring-outage precaution until independent checks confirm
thaw, fresh writes to every covered filesystem and workload liveness. Restore only
services and timers that were previously active.

Outside those conditions, review the existing configuration and supported setup
for the guest OS. The Linux package name is not a Windows or Android installation
instruction; if no supported agent is available, use guest-local filesystem tools.
See [Missing-reading and setup guidance](VM_DISK_MONITORING.md#a-missing-reading-is-not-an-installation-diagnosis).

### Does Pulse monitor Ceph?
Yes. When Pulse detects Ceph storage on Proxmox, **Proxmox → Ceph** shows cluster
health, quorum, OSDs, pools, and capacity automatically. If a cluster reports
topology or capacity without a recognised health verdict, Pulse shows **Unknown**
in both the table and cluster drawer and keeps that row out of the **Healthy**
filter (visible under **All**).

### Does Pulse monitor TrueNAS?
Yes. Pulse includes first-class TrueNAS SCALE/CORE integration. Add your
TrueNAS server under **Settings → Infrastructure** (**Add infrastructure**) with
the URL and API key. Pulse monitors the appliance, native VMs, apps, pools,
datasets, disks, ZFS snapshots, replication tasks, and alerts. Those resources
appear on the dedicated TrueNAS page.

### How is navigation organised in Pulse v6?
Pulse uses platform-shaped top-level pages:

- **Proxmox**, **Docker**, **Kubernetes**, **TrueNAS**, **vSphere**, and
  **Machines** keep platform-specific inventory and workflows together.
- **Availability** (`/availability`) provides website, service, device, and TLS
  certificate checks when availability checks are enabled or configured.
- Storage, snapshots, backups, and replication appear inside the platform page
  they belong to.
- **Alerts**, **Actions**, and **Patrol** provide cross-platform operational
  views.

The short-lived unified `/workloads`, `/storage`, and `/recovery` top-level
navigation was retired during the v6 prerelease cycle. See the
[historical migration note](MIGRATION_UNIFIED_NAV.md) if you are comparing an
older release candidate.

### Can I disable alerts for specific metrics?

Yes, for the controls offered in **Alerts → Thresholds**. Edit the relevant
default or resource row, use its **On/Off** toggle, save, then reopen the editor
to check the saved setting. The available resource overrides differ by platform;
not every default has a per-resource control. Use the editor rather than treating
`-1` as a universal value or hand-editing `alerts.json`.

**Off applies to that alert check, not all monitoring:**

- A metric's Off toggle does not hide its readings, stop collection or disable
  unrelated checks. Memory alerts, connectivity alerts and backup-age alerts
  are separate controls. Check warning and critical levels separately too.
- Check both the default and any override on the affected resource. A resource's
  explicit threshold can remain active even when its default is Off.
- Disabling a notification destination is different from disabling alert
  evaluation. Already queued notifications can reflect earlier settings; a
  message alone does not prove a new alert was evaluated after the save.
- **Patrol findings are separate from threshold alerts.** For CPU, memory,
  guest-disk and storage usage, Patrol currently uses its built-in threshold
  when the corresponding alert default is Off. Turning these alerts Off does
  not suppress a Patrol finding. See [how Patrol works](AI.md#how-patrol-works).

Use the existing alert entry or Patrol finding to identify which path produced
the warning. For missing messages, use the
[ordinary notification checks](TROUBLESHOOTING.md#test-succeeds-but-real-alerts-are-missing).
Do not induce an alert, send another Test or retry the queue merely to check Off.

For Docker or Podman, [runtime, container and machine alert scopes](DOCKER.md#runtime-and-container-alert-scopes)
are separate: runtime Offline Alerts does not mute its containers, and machine
maintenance is not a container-only mute.

### How do I monitor temperature?

Check the affected host's active agent version, sensor and observation time
before changing its setup. A current Pulse server does not update every agent.
Compare the same host and sensor at the same time; CPU/SoC temperature is not
physical-disk SMART temperature. A missing reading is unavailable, **not zero**.

On Linux, the agent reads existing `sensors -j` output and recognised CPU/SoC
thermal sysfs sources. `lm-sensors` is not required for the CPU/SoC fallback.
Start with the [bounded, private local reading check](TEMPERATURE_MONITORING.md#check-existing-linux-readings-safely)
on the monitored host, not inside the Pulse container.

Do not run hardware detection, bus scans, load drivers or reboot the host merely
to fill a temperature row. Necessary sensor setup belongs in that host's normal
maintenance window. During backups, freeze/thaw or an unresponsive-host incident,
defer setup and diagnostics.

For a new Proxmox agent installation, follow the
[verified HTTPS and token-file setup](TEMPERATURE_MONITORING.md#recommended-pulse-agent-proxmox).
Recent usable agent temperature data does not also require SSH for that host;
do not mount root SSH keys into Pulse or loosen an existing restricted key to
repair a missing reading. Other platforms have different providers and limits:
see [Temperature Monitoring](TEMPERATURE_MONITORING.md).

---

## 🔐 Security & Access

### Does Pulse need root access on every Proxmox host?

No. Start with a dedicated read-only or narrowly scoped Proxmox API token; that
provides normal inventory, status, utilization, guest, and storage monitoring
without installing Pulse on the hypervisor. The optional Linux agent runs as
`root` by default when you need host-local telemetry such as SMART,
temperatures, mdadm, or full mount details. Agent commands and network
discovery are disabled by default. See
[Production Deployment and Security](PRODUCTION_SECURITY.md) for the full
trust boundary and rollout checklist.

### I forgot my password. How do I reset it?
Use the [password recovery guide](TROUBLESHOOTING.md#i-forgot-my-password).
For the local administrator, update only the password in its active credential
source, preserving the username, tokens, configuration and data; deleting `.env`
is not a universal reset. The guide covers Docker, systemd and Proxmox LXC,
including deployment overrides, private backups and verification. SSO login,
temporary lockouts and a fresh install's missed bootstrap token need different
steps; do not reset a working installation to retrieve that token.

### How do I enable HTTPS?
Set `HTTPS_ENABLED=true` and provide `TLS_CERT_FILE` and `TLS_KEY_FILE` environment variables. See [Configuration](CONFIGURATION.md#-https--tls).

### Can I use Single Sign-On (SSO)?

Yes. You can keep more than one **OIDC** or **SAML** provider active in
**Settings → Security → Single Sign-On**. Follow the [OIDC setup guide](OIDC.md#-quick-start)
for OIDC. If your reverse proxy handles sign-in instead, see the
[Proxy Auth Guide](PROXY_AUTH.md) for Authentik, Authelia and Cloudflare.

**Keep an independent administrator login and leave its session open.**
For SAML, use your identity provider's metadata URL or XML, or enter its SSO URL
and certificate manually. When both are present, **Test** checks metadata first.
**URL format OK** applies only to manual SSO URL checks, and a passed **Metadata
read** still reports **Sign-in not tested**. Resolve **No signing cert read** or
**Expired cert found** warnings before relying on the provider. A new SAML draft
defaults the email attribute to `email`, while editing a saved provider keeps an
unset email attribute blank.

You can register Pulse in your identity provider before turning SAML sign-in on:
open **Edit** on a saved disabled provider (or use the card once enabled) to copy
its **SP Metadata** and **ACS URL**. If either URL is unavailable, check
**Settings → System** public URL and the saved provider settings rather than
inventing a URL. Sign in with the new provider in a separate browser profile or
private window and check that the account has the expected Pulse permissions.
Keep your independent administrator login available.

---

## ⚠️ Troubleshooting

### No data showing?

First distinguish what is missing; Pulse monitors more than Proxmox.

**Several independent sources stopped together?** Note the latest stored time
in **History** for a resource Pulse polls through its platform API and the
**Seen** time in **Machines** for a host that uses a Pulse agent. Compare them
over the same period, allowing for their usual poll and report intervals and
any retry delays.

If neither time advances while the monitored systems still respond, note when
the gap began and use the [stalled-dashboard checks](TROUBLESHOOTING.md#health-is-green-but-the-dashboard-is-stalled)
before changing a connection. A green Health check or a watchdog heartbeat
does not tell you whether alert rules ran.

| Missing item | Check existing evidence first |
| --- | --- |
| Whole page or resource | Open its platform page, not just Machines. Follow the [current navigation guide](TROUBLESHOOTING.md#old-bookmarks-dont-work) for moved pages and bookmarks. |
| Current readings | Check the affected connection in **Settings → Infrastructure**: is it enabled, and does its last activity advance during ordinary polling? For an agent, compare **Last seen** with the particular missing reading. |
| History only | Check the selected resource and time range. A current value does not prove that samples were stored for that chart; use [History troubleshooting](METRICS_HISTORY.md#troubleshooting). |

A green connection badge, recent **Last seen** or successful **Test Connection**
does not prove that every reading is fresh. Missing or unavailable is not zero.

- **Proxmox nodes** show reading status beside the node name on both desktop and
  phones, withholding live metrics and uptime when readings are not current:
  **Offline** (a failed Proxmox read does not prove the host is powered off),
  **Stale · API error** (a linked agent is online while the Proxmox connection
  failed), **Stale · Degraded** (the Proxmox provider is degraded), **Stale**
  (Pulse has no recent Proxmox readings), and **Degraded** (a node warning with
  current Proxmox readings).
- **vSphere datastores** keep their last reported capacity, active alarms,
  maintenance state and **Last reported inaccessible** status visible while
  marking stale rows **Stale data** beside the name (and noting `vCenter has not
  updated this datastore recently.` on inline Capacity History). Missing or
  unreadable capacity displays as a dash (`—`), not `0%`.
- **vSphere ESXi hosts and networks** also show **Stale data** beneath stale
  names on desktop and phones while preserving severe failure states (including
  **Last reported offline** for offline networks with stale vCenter reads) and
  retained metrics, uptime and counts. Expanded vSphere drawers surface the
  stale or collection-error detail under **Worth checking** (**Readings**), and
  inline host History defers live metrics with `vCenter has not updated this
  host recently.` while keeping last-known values visible until live polling
  recovers.
- **Kubernetes clusters, nodes, pods, Deployments and controllers** show
  **Stale data** when Kubernetes reporting goes quiet, even if a merged host
  agent or attached availability check keeps the host online. Worse states such
  as **Offline**, **NotReady**, **Unschedulable**, **Failed**,
  **CrashLoopBackOff** and replica shortfalls stay ahead of staleness. Stale
  resources move out of **Healthy** into **Degraded** and **Nodes needing
  attention**, and expanded object drawers show
  `Showing the last Kubernetes report — no fresh update received recently.`
  without hiding retained metrics or counts.
- **Docker Compose applications and Swarm** (`/docker/applications` and
  `/docker/swarm`, covering Services, Tasks, Nodes, Configs and Secrets) show
  **Stale data** when Docker reporting goes quiet or hits a collection error,
  turning stale healthy or muted snapshots amber instead of green while keeping
  worse states (**Failed**, **Rejected**, **Orphaned**, **Manager unreachable**,
  **Down**, **Drained**, **Update paused**, **Rolling back**, **Rollback
  paused**, and `0 / N running`) ahead of staleness. Stale Compose applications
  move out of **Healthy** into **Attention**, stale Swarm resources move out of
  **Healthy** into **Degraded**, and expanded detail panels show an inline
  warning notice
  (`Showing the last Docker report — no fresh update received recently.`,
  `No Docker report for <age> — showing the last values received.`, or
  `Docker collection error — showing the last report received.`, plus
  `Last error:` when reported) without hiding retained counts, states or
  metadata.

Use the relevant checks for [stale TrueNAS data](TRUENAS.md#stale-truenas-data),
[stale Kubernetes readings](KUBERNETES.md#stale-kubernetes-readings),
[empty PBS History](PBS.md#pbs-is-connected-but-history-stays-empty),
[missing or stale PVE replication jobs](TROUBLESHOOTING.md#replication-jobs-are-pending-stale-or-missing) or
[missing VM disk usage](VM_DISK_MONITORING.md#a-missing-reading-is-not-an-installation-diagnosis). Check Proxmox
[effective permissions](TROUBLESHOOTING.md#check-permissions-proxmox) only when
the affected operation reports an access error, not on a missing chart alone.

Keep the original time and displayed error. Do not replace credentials, delete
connections or re-enrol agents just to populate a chart. During a backup,
freeze/thaw or an unresponsive-host incident, defer setup, live tests and
**Run Diagnostics**; follow [safe evidence collection](TROUBLESHOOTING.md#collect-diagnostics-safely).
If logs are needed, use a bounded excerpt from the original incident and review
it locally before sharing, as described in [Getting Help](TROUBLESHOOTING.md#-getting-help).

### Connection refused?

Distinguish your browser's connection to Pulse from Pulse's connection to a
monitored platform. If the browser cannot reach Pulse, check the actual service,
listening port and proxy using the [connection guide](TROUBLESHOOTING.md#connection-refused).
If Pulse opens but a platform request is refused, preserve that request's
redacted error and use its platform guide above; changing Pulse's listening
port does not repair the platform connection.

### CORS errors?

Keep the default same-origin policy when Pulse and its API use the same public
origin, including behind a reverse proxy. If a separate trusted browser app
needs cross-origin API access, allow only its exact origin (scheme, host and
port); `*` is not a login or proxy repair. Follow the
[CORS checks](TROUBLESHOOTING.md#cors-errors) before broadening browser access.

### High memory usage?
If Pulse stopped and you suspect it ran out of memory, start with
[the checks after a memory-cgroup kill](TROUBLESHOOTING.md#after-a-memory-cgroup-kill).
Use the evidence you already have. Once Pulse is running again, check that a
resource you already monitor is receiving new readings before taking any more
memory samples.

For a monitored VM, first check [guest memory, cache and available memory](TROUBLESHOOTING.md#guest-memory-is-high-but-available-memory-is-plentiful).
A high hypervisor footprint and one application's RSS measure different things;
compare existing readings from the same guest and time, not the Proxmox node.

First distinguish container usage from Pulse's resident memory (RSS); a high
LXC or Docker chart alone does not establish a leak. Use the
[read-only memory checks](TROUBLESHOOTING.md#memory-use-keeps-growing) for your
deployment while it is responsive.

A higher steady baseline is worth reporting even without continuing growth.
A lower reading ten minutes into a new process is not a like-for-like comparison
with a week-old run, or proof of recovery. Keep already available readings,
versions, uptime and workload context; do not restart to create a comparison.

Do not shorten retention, slow polling, restart Pulse or drop caches just to
lower the reading. Shorter retention removes history, and slower polling can
delay monitoring; neither establishes the cause. If the container is near its
memory limit or the host is unresponsive, stop sampling and prioritise safe
recovery.

### Can Pulse monitor 50 or more Proxmox hosts?

Pulse has automated API and metrics-store load coverage for a simulated
500-node estate, including 2,500 VMs and concurrent readers and writers. That
is regression evidence rather than a universal production certification:
retention, polling, storage latency, integrations, and guest count all affect
real capacity. Stage a large rollout and measure it using the checklist in
[Production Deployment and Security](PRODUCTION_SECURITY.md#production-rollout-checklist).

---

## 🧑‍💻 The Project

### Is Pulse developed with AI?
Yes. Pulse uses coding agents and other automation across development and triage. The maintainer sets product direction, controls releases, and remains responsible for the result. See [Development and Automation Transparency](AI_TRANSPARENCY.md) for the standing policy.
