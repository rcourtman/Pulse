# ❓ Frequently Asked Questions

## 🛠️ Installation & Setup

### What's the easiest way to install?
If you run Proxmox VE, use the signed LXC installer flow in [INSTALL.md](INSTALL.md) and replace `vX.Y.Z` with the exact release tag you want.

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

No. The server and agent have separate update lifecycles. Eligible v6 agents
check for and apply the server's target version asynchronously. v5 agents, PVE
host agents, agents with auto-update disabled, and agents with failed or missing
update prerequisites need a manual command.

Open an outdated-agent notice or
`/settings/infrastructure?agentDoctor=1` to open **Agent Doctor** and
copy the correct per-host command. The surface does not remotely execute the
update. Use **Settings → Infrastructure → Install on a host** for first installs
and v5-to-v6 upgrades. See [Unified Agent](UNIFIED_AGENT.md#auto-update).

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

For a self-hosted installation, Pro adds capabilities to that installation;
it does not create a hosted server. **Pulse Cloud is not generally available.**
A Cloud tier in the feature matrix is not a public sign-up or provisioning
offer. If you already have hosted access, use the instance supplied by its
operator; see [existing hosted access](CLOUD.md#1-confirm-existing-hosted-access).
Pro adds **hands-on Patrol modes, issue investigation, governed fixes, verified outcomes, and 90-day history** along with the broader operations feature set. Existing legacy Pro+ holders keep their current continuity, but self-hosted pricing no longer sells more monitoring volume. Pulse Patrol is available to everyone on Community with BYOK and provides scheduled, cross-system analysis that correlates real-time state, recent metrics history, and diagnostics to surface actionable findings.

Example output includes trend-based capacity warnings, backup regressions, Kubernetes cluster analysis, and correlated container failures that simple threshold alerts miss.
See [Pulse Intelligence](AI.md), [Plans and entitlements](PULSE_PRO.md#cloud),
and <https://pulserelay.pro>.

Already paid but a feature is still unavailable? Check
[activation and the installed Pro runtime](PULSE_PRO.md#already-paid-but-pro-features-are-unavailable)
before buying again or clearing a key.

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
Yes! If Pulse detects Ceph storage, it automatically queries cluster health, OSD status, and pool usage. No extra config needed.

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
Yes. Pulse supports **OIDC** and **SAML** SSO providers, with multi-provider support (multiple IdPs active simultaneously). Configure in **Settings → Security → Single Sign-On**. Pulse also supports Proxy Auth (Authentik, Authelia, Cloudflare). See [Proxy Auth Guide](PROXY_AUTH.md).

---

## ⚠️ Troubleshooting

### No data showing?

First distinguish what is missing; Pulse monitors more than Proxmox.

| Missing item | Check existing evidence first |
| --- | --- |
| Whole page or resource | Open its platform page, not just Machines. Follow the [current navigation guide](TROUBLESHOOTING.md#old-bookmarks-dont-work) for moved pages and bookmarks. |
| Current readings | Check the affected connection in **Settings → Infrastructure**: is it enabled, and does its last activity advance during ordinary polling? For an agent, compare **Last seen** with the particular missing reading. |
| History only | Check the selected resource and time range. A current value does not prove that samples were stored for that chart; use [History troubleshooting](METRICS_HISTORY.md#troubleshooting). |

A green connection badge, recent **Last seen** or successful **Test Connection**
does not prove that every reading is fresh. Missing or unavailable is not zero.
Use the relevant checks for [stale TrueNAS data](TRUENAS.md#stale-truenas-data),
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
