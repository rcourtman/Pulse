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

- **Recommended (agent setup)**: copy the generated install command and run it on the target host.
- **Manual/API-backed platforms**: use **Settings → Infrastructure → Platform connections** for systems such as Proxmox, PBS, PMG, or TrueNAS that connect over an API instead of running the agent locally.

If you want Pulse to find servers automatically, enable discovery in **Settings → System → Network** and then review discovered servers in **Settings → Infrastructure**.

### How do I change the port?
- **Systemd**: `sudo systemctl edit pulse`, add `Environment="FRONTEND_PORT=8080"`, restart.
- **Docker**: Use `-p 8080:7655` in your run command.

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
If a setting is disabled with an amber warning, it's being overridden by an environment variable (e.g., `DISCOVERY_ENABLED`). Remove the env var to regain UI control.

---

## 🔍 Monitoring & Metrics

### What do Pro and Cloud unlock?
Pro and Cloud unlock **hands-on Patrol modes, issue investigation, governed fixes, verified outcomes, and 90-day history** along with the broader operations feature set. Existing legacy Pro+ holders keep their current continuity, but self-hosted pricing no longer sells more monitoring volume. Pulse Patrol is available to everyone on Community with BYOK and provides scheduled, cross-system analysis that correlates real-time state, recent metrics history, and diagnostics to surface actionable findings.

Example output includes trend-based capacity warnings, backup regressions, Kubernetes cluster analysis, and correlated container failures that simple threshold alerts miss.
See [Pulse Intelligence](AI.md), [Plans and entitlements](PULSE_PRO.md), and <https://pulserelay.pro>.

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
Proxmox API returns `0` for VM disk usage by default. You must install the **QEMU Guest Agent** inside the VM and enable it in Proxmox (VM → Options → QEMU Guest Agent).
See [VM Disk Monitoring](VM_DISK_MONITORING.md) for details.

### Does Pulse monitor Ceph?
Yes! If Pulse detects Ceph storage, it automatically queries cluster health, OSD status, and pool usage. No extra config needed.

### Does Pulse monitor TrueNAS?
Yes. Pulse includes first-class TrueNAS SCALE/CORE integration. Add your
TrueNAS server under **Settings → Infrastructure → Platform connections** with
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
Yes. Go to **Alerts → Thresholds** and use the On/Off toggle next to any metric while editing, or set the value to `-1`. You can do this globally or per-resource (VM/Node).

### How do I monitor temperature?
Recommended: install the unified agent on your Proxmox hosts with Proxmox integration enabled:

1. Install `lm-sensors` on the host (`apt install lm-sensors && sensors-detect`)
2. Install `pulse-agent` with `--enable-proxmox`

If you do not run the agent, Pulse can collect temperatures over SSH. When the agent is reporting usable temperatures, Pulse uses the agent path and does not also require SSH for that host. See [Temperature Monitoring](TEMPERATURE_MONITORING.md).

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
Yes. Pulse supports **OIDC** and **SAML** SSO providers, with multi-provider support (multiple IdPs active simultaneously). Configure in **Settings → Security → SSO Providers**. Pulse also supports Proxy Auth (Authentik, Authelia, Cloudflare). See [Proxy Auth Guide](PROXY_AUTH.md).

---

## ⚠️ Troubleshooting

### No data showing?
- Check Proxmox API is reachable (port 8006).
- Verify credentials in **Settings → Infrastructure**.
- Check logs: `journalctl -u pulse -f` or `docker logs -f pulse`.

### Connection refused?
- Check if Pulse is running: `systemctl status pulse` or `docker ps`.
- Verify the port (default 7655) is open on your firewall.

### CORS errors?
Pulse defaults to same-origin only. If you access the API from a different domain, set **Settings → System → Network → Allowed Origins** or use `ALLOWED_ORIGINS` (single origin, or `*` if you explicitly want all origins).

### High memory usage?
First distinguish container usage from Pulse's resident memory (RSS); a high
LXC or Docker chart alone does not establish a leak. Use the
[read-only memory checks](TROUBLESHOOTING.md#memory-use-keeps-growing) for your
deployment while it is responsive.

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
