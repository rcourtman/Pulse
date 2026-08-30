# Pulse Unified Agent

The unified agent (`pulse-agent`) is the single host-installed Pulse infrastructure agent binary. It combines host, Docker/Podman, Kubernetes, Proxmox-local, and other enabled node-local telemetry modules into one deployment and one service.
Install it on standalone hosts and on machines where Pulse needs full node-local telemetry.
For API-backed platforms, start with the platform connection first and add the agent only where local telemetry is needed.

For Proxmox, install the agent only where you need telemetry that the Proxmox
API cannot provide, such as host SMART and temperature data, local
ZFS/Ceph/mdadm detail, arbitrary host mount reads, or the full mounted
filesystem breakdown for running LXCs. Docker containers inside LXCs can be
reported by a Proxmox host agent when the server has explicitly enabled the
privacy-bounded LXC inventory mode; Docker/Podman inside VMs still needs a
guest-local agent or another explicit guest reporting path.
Basic Proxmox inventory and utilization can use a read-only or narrowly scoped
Proxmox API token instead. Settings uses that API inventory path as the
default for new PVE/PBS setup. See [Agent Security](AGENT_SECURITY.md) for
the root-service trade-off, restricted-user expectations, and supply-chain
verification guidance.

> Note: For agent-based temperature monitoring, use `pulse-agent --enable-proxmox` or SSH-based collection. The legacy sensor proxy has been removed. See `docs/TEMPERATURE_MONITORING.md`.

## Quick Start

Generate an installation command in the UI:
**Settings → Infrastructure → Install on a host**

Choose a target profile in that screen when you want explicit install flags for Docker, Kubernetes, Proxmox VE, or Proxmox Backup Server.

The same generated command is also the supported v5-to-v6 agent upgrade path.
Run it on the host that already has the v5 `pulse-agent` service to replace the
binary and service configuration in place; do not uninstall the old service
first unless you are intentionally removing that host from Pulse.

### Moving Pulse to a new address

Configuration export/import restores the server-side agent records and API
tokens. It cannot rewrite the primary Pulse URL on remote machines because
agents initiate the connection. Prefer a stable DNS name for the primary URL
so replacing the Pulse host does not require an agent migration.

After importing the configuration on a Pulse server with a different address,
retarget each existing standard Linux agent from that agent machine:

```bash
curl -fsSL https://pulse.example.com:7655/install.sh | \
  sudo bash -s -- --retarget --url https://pulse.example.com:7655
```

The retarget operation recovers the existing token, agent ID, enabled
collectors, and other service options. It does not carry the old endpoint's
TLS bypass, custom CA, or certificate fingerprint to the new address. Supply
`--cacert`, `--server-fingerprint`, or (only on a trusted network)
`--insecure` explicitly when the new endpoint requires it. The script must
come from the new server so it supports the retarget operation. A newly
generated full installation command from **Settings → Infrastructure → Install
on a host** remains the fallback.

On Windows, run the full generated PowerShell installation command from the
new Pulse server as Administrator, including the desired collector options.
Do not expect the configuration import itself to make agent-only machines
appear at the new address.

An installed agent has one **primary** Pulse URL and token. The primary is the
only server allowed to supply remote configuration, commands, enrollment, or
updates. The same collection can also be sent to explicitly configured,
report-only **observer** instances; see [Observer destinations](#observer-destinations).
After an upgrade, check the relevant platform page or **Machines** view once
the agent has reported, and confirm the host-local version with
`pulse-agent --version` if the UI has not received a fresh report yet.

This is the agent installer served by your Pulse server. It is separate from the
top-level GitHub `install.sh`, which installs or updates the Pulse server itself.

### Linux (systemd)
```bash
curl -fsSL http://<pulse-ip>:7655/install.sh | \
  bash -s -- --url http://<pulse-ip>:7655 --token <api-token>
```

### macOS
```bash
curl -fsSL http://<pulse-ip>:7655/install.sh | \
  bash -s -- --url http://<pulse-ip>:7655 --token <api-token>
```

### Windows (PowerShell, run as Administrator)
```powershell
irm http://<pulse-ip>:7655/install.ps1 | iex
```

With environment variables:
```powershell
$env:PULSE_URL="http://<pulse-ip>:7655"
$env:PULSE_TOKEN="<api-token>"
irm http://<pulse-ip>:7655/install.ps1 | iex
```

### Synology NAS
```bash
curl -fsSL http://<pulse-ip>:7655/install.sh | \
  bash -s -- --url http://<pulse-ip>:7655 --token <api-token>
```

### TrueNAS SCALE/CORE
TrueNAS SCALE and TrueNAS CORE are both supported. The installer auto-detects the platform and configures the appropriate service manager (systemd for SCALE, rc.d for CORE).
```bash
curl -fsSL http://<pulse-ip>:7655/install.sh | \
  bash -s -- --url http://<pulse-ip>:7655 --token <api-token>
```

## Features

- **Host Metrics**: CPU, memory, disk, network I/O, temperatures
- **Docker Monitoring**: Container metrics, health checks, Swarm support (when enabled)
- **Kubernetes Monitoring**: Cluster, node, pod, and deployment health (when enabled)
- **Libvirt/KVM Monitoring**: Read-only VM inventory, state, vCPU, memory, and disk/network rates when the Linux host exposes `virsh`
- **XCP-ng Monitoring**: Read-only pool and VM inventory, power state, vCPU, and memory when an XCP-ng control domain exposes `xe`
- **External Probes** (Pro): runs availability checks assigned to this agent from the Pulse server and reports the results back — see below
- **Auto-Update**: Automatically updates when a new version is released
- **Multi-Platform**: Linux, macOS, Windows support

The opt-in Linux typed-helper collector profile keeps auto-update enabled
without making its root-owned binary writable by the service account. The
collector downloads and self-tests the signed artifact in a fixed
collector-owned quarantine; the no-network helper revalidates it, promotes it
to root-only staging, atomically activates it, and owns identity-bound rollback
if restart fails. There is no fallback to direct unprivileged replacement.
This transaction is covered by unit, race, release-build, and installer
contract tests; live-host restart/health and rollback qualification remains
required before the profile becomes the general default.
See [Agent Security](AGENT_SECURITY.md#least-privilege-agent-profile).

On Linux systemd, the safe monitoring profile and remediation lifecycle are
separate install choices. `--least-privilege --enable-privileged-helper`
selects the opt-in unprivileged collector and no-network helper. Adding
`--enable-action-runner --action-token-file <private-file>` installs the
root-owned runner with a separately issued, host-bound action credential.
`--disable-action-runner` and `--uninstall-action-runner` remove remediation
without removing the collector. The runner accepts only the documented typed
host, Proxmox guest, and container operations; shell, generic exec,
unrestricted file reads, and deploy requests remain forbidden.

Agent Doctor shows the reported collector service user, command-authority
ceiling, collector credential scope, and typed-helper configuration separately
from the tenant- and host-bound action-runner credential and connected-session
state. It does not treat a healthy collector as proof that actions are ready.
For an eligible Linux systemd host, enrollment returns one credential reveal
kept only in the open page's memory. Start the generated `/dev/tty` prompt,
copy and paste the secret into that prompt to write the private root-owned
token file, and then run the installer command, which carries only
`--action-token-file <private-file>` in its arguments. Re-enrollment atomically
replaces the prior credential for the same tenant and canonical agent ID; a
failed persistence write retains the previous credential, while a successful
rotation invalidates it.

Use `--safe-profile-inspect` to report the current profile and calculated
differences without changing the host. `--safe-profile-apply` performs the
explicit collector/helper migration and retains a rollback snapshot;
`--safe-profile-rollback` restores it. These commands are proven only for
Linux systemd and fail closed elsewhere. Appliance, non-systemd, Windows, and
macOS installs remain explicit legacy/full-trust profiles until their own
runtime and migration boundaries are qualified.
Inspection includes unit identity/groups, ambient capabilities, binary
ownership/mode, provider flags, helper/command state, independent runner
presence, and the expected telemetry/action degradation. Apply preserves the
collector token and agent identity, commits only after local health, helper
socket, and server registration checks, and restores the exact prior
collector/helper profile on failure. It never changes a separately installed
runner, and ordinary `--update` never implies a profile migration. The secure
runtime separation remains a proposed hardening lane; the product-wide default
does not change until representative real Linux hosts pass migration,
activation, rollback, provider-parity, and action-session qualification.

On Linux, the host module automatically checks for `virsh`. When the agent can
open the default libvirt connection read-only, defined domains appear as VM
workloads under that host. Collection uses libvirt's bounded bulk statistics
interface and does not grant Pulse VM start, stop, console, or configuration
authority. If `virsh` is absent, the socket is inaccessible, or the driver does
not support the requested statistics, normal host reporting continues without
libvirt inventory.

Appliance packaging can still differ. In particular, a QNAP installation must
make its Container Station libvirt client/socket available to the agent service;
the presence of KVM processes alone is not enough to establish a readable
libvirt connection.

On XCP-ng, the host module automatically checks for the local `xe` CLI. It
uses only bounded `pool-list`, `host-list`, and `vm-list` queries: no XAPI
credentials or VM lifecycle authority are added. The XCP-ng pool becomes the
host's cluster grouping, and pool-wide VMs are de-duplicated by UUID and
parented to the resident Pulse host when that node also reports. If several
pool nodes run the Unified Agent, their identical pool views coalesce rather
than creating duplicate workloads. A failed `xe` query preserves the last
successful inventory while normal host metrics continue to report.

This local integration covers one XCP-ng pool. Multi-pool deployments that
need a central Xen Orchestra connection remain a separate integration surface.

### Windows CPU and motherboard temperatures

Windows does not expose dependable built-in CPU or motherboard temperature
readings. The Unified Agent can import those readings from
[LibreHardwareMonitor](https://github.com/LibreHardwareMonitor/LibreHardwareMonitor),
which supplies the required driver-backed hardware access:

1. Run LibreHardwareMonitor as Administrator on the Windows host.
2. Keep its HTTP port at the default `8085`.
3. Select **Options → Remote Web Server → Run**.
4. Do not permit inbound network access to port `8085` in Windows Firewall.
   Pulse connects only to `http://127.0.0.1:8085/data.json`.

LibreHardwareMonitor's web authentication must remain disabled for this
loopback-only integration. Pulse uses a fixed local URL, follows no redirects,
and accepts only bounded, validated CPU and motherboard Celsius readings.
If LibreHardwareMonitor is stopped, unavailable, or returns unsupported data,
the rest of the Windows host report continues normally.

Native Windows Storage reliability counters remain the source for supported
physical-disk temperatures. NVIDIA GPU telemetry continues to come directly
from `nvidia-smi`; Pulse deliberately ignores LibreHardwareMonitor GPU and
storage nodes to avoid duplicate or ambiguously correlated readings.

## External Probes (Pro)

With the Pro `external_probe` entitlement, availability checks configured in
Pulse can be assigned to run from a specific agent instead of the Pulse
server (Settings -> Monitoring -> Availability checks -> "Run from"). This is
how you monitor a site from the outside: deploy the agent on a machine
elsewhere — a cloud VM, a Docker host at another location — and assign checks
to it. Target failures are evaluated on the Pulse server through your normal
alert routes.

There is nothing to configure on the agent itself. Assignments arrive through
the agent's signed remote configuration, the agent runs each check on its
configured interval, and results are delivered with its regular reports.
Results survive temporary connectivity loss to the Pulse server in a bounded
in-memory queue; if the agent cannot deliver for several check intervals the
check shows as indeterminate in Pulse until reports resume. After the
five-minute minimum grace window, Pulse raises one
`availability_probe_unavailable` warning per disconnected probe, regardless of
how many checks it owns. Pulse measures that reporting window from server receipt
time rather than the agent's clock, so clock skew cannot create or conceal the
disconnect. That warning uses the normal email, webhook, Apprise, and
recovery-notification pipeline. When Pulse Mobile is paired through Relay, Pulse
also sends a privacy-safe `external_probe_offline` push linked to the canonical
mobile attention item without exposing target names or addresses. The alert
identity belongs to the probe agent, so adding or removing an assigned check
does not resolve and reopen it.

When the host heartbeat itself is offline, Pulse keeps the existing
host-offline alert as the single canonical incident and suppresses the
probe-results warning. Assigned probe hosts still receive the external-probe
mobile push, but operators do not get two normal alerts for the same agent
failure.

This has a complementary dark-site path: if the entire Pulse instance or its
site goes offline, Pulse Relay independently sends its existing instance
offline push after five minutes. Together, probe-loss alerts while Pulse is
online and Relay's instance-loss alert while Pulse is dark ensure the
outside-monitoring path cannot disappear silently. Relay does not evaluate
individual target results while the Pulse server is offline.

The module appears as `availability` in the agent's module status when at
least one check is assigned.

Note for ICMP (ping) checks: the probe uses the system `ping` binary. In
containers or hardened service units without `CAP_NET_RAW`, ICMP checks fail;
prefer TCP or HTTP checks there, or grant the capability. See "ICMP probe
privileges" in docs/CONFIGURATION.md.

## Custom metrics

The host module can report numeric, boolean, and timestamp metrics produced by
local executables or HTTP(S) REST endpoints. This is intended for site-specific
signals such as queue depth, UPS load, service status, DNS update age, or a
backup timestamp that Pulse cannot collect natively.

Create a private YAML file:

```yaml
version: 1
sensors:
  - id: queue_depth
    name: Queue depth
    command: /usr/local/libexec/pulse-queue-depth
    unit: items
    interval: 1m
    timeout: 2s
    warningAbove: 20
    criticalAbove: 50
    alertOnError: true

  - id: main_dns_update
    name: Main DNS update
    group: Main server
    subgroup: Domain
    kind: timestamp
    url: https://metrics.example.net/dns/main
    interval: 1m
    timeout: 2s
    staleAfter: 10m
    warningAbove: 3600
    criticalAbove: 7200

  - id: checkout_online
    name: Checkout service
    group: Main server
    subgroup: Service statuses
    kind: boolean
    url: http://monitoring.internal/checkout
    criticalBelow: 0.5
```

Then start or restart the agent with
`--custom-sensors-file /etc/pulse/custom-sensors.yaml`, or set
`PULSE_CUSTOM_SENSORS_FILE` to that absolute path. Each metric configures
exactly one source:

- `command`: an absolute executable path. It receives no arguments and writes
  one scalar to standard output.
- `url`: an absolute HTTP(S) URL polled with `GET`. Redirects are not followed,
  non-2xx responses fail, and the response is limited to 4 KiB.

REST endpoints can return a plain scalar or a JSON object:

```json
{"value": 42.5, "observedAt": "2026-07-30T20:00:00Z"}
```

`value` may be a number, string, or boolean. `observedAt` is optional RFC3339
source time. When `staleAfter` is configured, older source data becomes a stale
error and follows `alertOnError`.

`kind` defaults to `number`. Boolean metrics accept true/false, 1/0, yes/no,
on/off, up/down, and online/offline; Pulse stores true as 1 and false as 0, so
`criticalBelow: 0.5` alerts when a service is offline. Timestamp metrics accept
RFC3339 or Unix seconds, display time since the event, and apply thresholds to
the age in seconds. Optional `group` and `subgroup` values organize labels in
the **Custom Metrics** card.

The agent evaluates optional `warningAbove`, `criticalAbove`, `warningBelow`,
and `criticalBelow` thresholds locally. Pulse displays the typed value and unit
under **Custom Metrics** and creates normal warning/critical alerts. A collection
failure alerts by default; set `alertOnError: false` to make failures
report-only. If a probe fails after a successful reading, the last good value is
shown as stale with its original observation time.

Configuration is deliberately local-only. The Pulse server and remote agent
configuration cannot supply commands, URLs, or arguments. The file is limited
to 32 metrics; intervals must be between 10 seconds and 24 hours; timeouts must
be between 100 milliseconds and 10 seconds and shorter than the interval.
`staleAfter` must be between 10 seconds and 30 days. At most four probes run
concurrently, output is bounded, HTTP credentials in URLs are rejected, and
each executable is revalidated before use. Standard TLS certificate validation
applies to HTTPS endpoints; use network policy to constrain destinations where
required.

On POSIX systems, the YAML file must be a regular, non-symlink file owned by
the agent service user with no group/other permissions (normally mode `0600`).
Commands and their immediate parent directories must also be regular,
non-symlink, owned by the service user, and not group/other writable. Commands
must have an executable bit. For example:

```bash
sudo chown root:root /etc/pulse/custom-sensors.yaml /usr/local/libexec/pulse-queue-depth
sudo chmod 0600 /etc/pulse/custom-sensors.yaml
sudo chmod 0700 /usr/local/libexec/pulse-queue-depth
```

## Configuration

| Flag | Env Var | Description | Default |
|------|---------|-------------|---------|
| `--url` | `PULSE_URL` | Pulse server URL | `http://localhost:7655` |
| `--token` | `PULSE_TOKEN` | API token | *(required)* |
| `--observers-file` | `PULSE_OBSERVERS_FILE` | Private JSON file defining report-only destinations | *(none)* |
| `--custom-sensors-file` | `PULSE_CUSTOM_SENSORS_FILE` | Private YAML file defining command/REST custom metrics and thresholds | *(none)* |
| `--token-file` | - | Read API token from file | *(unset)* |
| `--interval` | `PULSE_INTERVAL` | Reporting interval | `30s` |
| `--enable-host` | `PULSE_ENABLE_HOST` | Enable host metrics | `true` |
| `--enable-docker` | `PULSE_ENABLE_DOCKER` | Enable Docker / Podman metrics | `false` (auto-detect if not configured) |
| `--docker-runtime` | `PULSE_DOCKER_RUNTIME` | Force Docker / Podman runtime: `auto`, `docker`, or `podman` | `auto` |
| `--enable-kubernetes` | `PULSE_ENABLE_KUBERNETES` | Enable Kubernetes metrics | `false` (installer auto-detect if not configured) |
| `--enable-proxmox` | `PULSE_ENABLE_PROXMOX` | Enable Proxmox integration | `false` |
| `--proxmox-type` | `PULSE_PROXMOX_TYPE` | Proxmox type: `pve` or `pbs` | *(auto-detect)* |
| `--enable-commands` | `PULSE_ENABLE_COMMANDS` | Enable Pulse command execution: Docker / Podman container actions from the UI (start/stop/restart/update), Patrol actions, and Proxmox LXC Docker inventory (disabled by default) | `false` |
| `--disable-commands` | `PULSE_DISABLE_COMMANDS` | **Deprecated** (commands are disabled by default) | - |
| `--disk-exclude` | `PULSE_DISK_EXCLUDE` | Device name/path or mount point patterns to exclude from disk and S.M.A.R.T. monitoring (repeatable or CSV) | *(none)* |
| `--disk-include` | `PULSE_DISK_INCLUDE` | Device name/path or mount point patterns to include despite automatic filesystem filtering (repeatable or CSV) | *(none)* |
| `--kubeconfig` | `PULSE_KUBECONFIG` | Kubeconfig path (optional) | *(auto)* |
| `--kube-context` | `PULSE_KUBE_CONTEXT` | Kubeconfig context (optional) | *(auto)* |
| `--kube-include-namespace` | `PULSE_KUBE_INCLUDE_NAMESPACES` | Limit namespaces (repeatable or CSV, wildcards supported) | *(all)* |
| `--kube-exclude-namespace` | `PULSE_KUBE_EXCLUDE_NAMESPACES` | Exclude namespaces (repeatable or CSV, wildcards supported) | *(none)* |
| `--kube-include-all-pods` | `PULSE_KUBE_INCLUDE_ALL_PODS` | Include all non-succeeded pods | `false` |
| `--kube-include-all-deployments` | `PULSE_KUBE_INCLUDE_ALL_DEPLOYMENTS` | Include all deployments, not just problems | `false` |
| `--kube-max-pods` | `PULSE_KUBE_MAX_PODS` | Max pods per report | `200` |
| `--disable-auto-update` | `PULSE_DISABLE_AUTO_UPDATE` | Disable auto-updates | `false` |
| `--disable-docker-update-checks` | `PULSE_DISABLE_DOCKER_UPDATE_CHECKS` | Disable Docker image update detection | `false` |
| `--disable-registry-credentials` | `PULSE_DISABLE_REGISTRY_CREDENTIALS` | Do not read host Docker credentials (config.json / credential helpers) for registry update checks | `false` |
| `--insecure` | `PULSE_INSECURE_SKIP_VERIFY` | Skip TLS verification | `false` |
| `--allow-plaintext-http` | `PULSE_AGENT_ALLOW_PLAINTEXT_HTTP` | Allow plain HTTP to a Pulse server that does not look local (private IP, single-label, `.local`/`.lan`/`.home`/`.home.arpa`/`.internal`, or resolves to private addresses). Sends the API token in cleartext; only for networks you fully control, e.g. internal networks numbered from public IP space | `false` |
| `--hostname` | `PULSE_HOSTNAME` | Override hostname | *(OS hostname)* |
| `--agent-id` | `PULSE_AGENT_ID` | Unique agent identifier | *(machine-id)* |
| `--report-ip` | `PULSE_REPORT_IP` | Override reported IP (multi-NIC) | *(auto)* |
| `--disable-ceph` | `PULSE_DISABLE_CEPH` | Disable local Ceph status polling | `false` |
| `--tag` | `PULSE_TAGS` | Apply tags (repeatable or CSV) | *(none)* |
| `--log-level` | `LOG_LEVEL` | Log verbosity (`debug`, `info`, `warn`, `error`) | `info` |
| `--health-addr` | `PULSE_HEALTH_ADDR` | Health/metrics server address | `127.0.0.1:9191` |

Use `--health-addr :9191` only when another host must scrape the
health/metrics endpoint over the network. Use `--health-addr ""` or
`PULSE_HEALTH_ADDR=off` to disable that listener.

**Token resolution order**: `--token` → `--token-file` → `PULSE_TOKEN` → `/var/lib/pulse-agent/token`.

### Agent log level

The default `info` level records normal lifecycle and connection messages.
Use `warn` to retain warnings and errors while suppressing routine messages;
log level does not change collection, reporting, alerts, or notifications.

For an agent installed as a Linux systemd service, use a persistent drop-in
instead of editing the generated unit (which an agent update may replace):

```bash
sudo systemctl edit pulse-agent
```

Add the following content, then save and exit:

```ini
[Service]
Environment="LOG_LEVEL=warn"
```

Apply and verify the change:

```bash
sudo systemctl daemon-reload
sudo systemctl restart pulse-agent
sudo journalctl -u pulse-agent --since "5 minutes ago"
```

Set the value to `debug` temporarily when collecting diagnostics, then restore
`info` or `warn`. For a container agent, set `LOG_LEVEL=warn` in the container
environment and recreate the container. Pro installations can also manage
`log_level` with an [agent configuration profile](CENTRALIZED_MANAGEMENT.md).

## Observer destinations

Observer destinations receive the same already-collected host, Docker/Podman,
and Kubernetes reports. Collection runs once per interval. Delivery, retries,
and persisted host-report buffers are isolated per destination, so an observer
outage does not replay or block the primary stream. Observer responses cannot
change configuration, execute commands, enroll the agent, or select updates.

Create a separate API token on each observer and store every token in its own
absolute-path file. On Unix, both the JSON file and token files must be regular,
non-symlink files with no group or other permissions (for example mode `0600`).

```json
{
  "version": 1,
  "observers": [
    {
      "name": "dev",
      "url": "https://pulse-dev.example.test",
      "tokenFile": "/etc/pulse-agent/dev-observer.token",
      "serverFingerprint": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
      "provisionProxmox": true
    }
  ]
}
```

Start or install the service with
`--observers-file /etc/pulse-agent/observers.json`. Plaintext remote HTTP is
rejected unless that observer explicitly sets `"allowPlaintextHTTP": true`.
`insecureSkipVerify` is available per observer but should be replaced with a
CA file or certificate fingerprint wherever possible.

When Proxmox integration is enabled, each observer gets a distinct
destination-scoped PVE/PBS API token and registration-state directory. Pulse
must answer the registration check before the agent creates or rotates any
Proxmox token; an unavailable destination therefore leaves existing
credentials unchanged. Set `"provisionProxmox": false` when an observer should
receive only Unified Agent telemetry and no separately registered PVE/PBS
source.

Per-destination delivery status is exported on the health listener as
`pulse_agent_destination_configured` and
`pulse_agent_destination_delivery_up`, labelled by module, destination, and
role.

### Advanced Flags

- `--version`: Print the agent version and exit.
- `--self-test`: Perform a self-test and exit (used during auto-update).

## Auto-Detection

Auto-detection behavior:

- **Host metrics**: Enabled by default.
- **Docker/Podman**: Enabled automatically by the agent if Docker/Podman is detected and `PULSE_ENABLE_DOCKER` was not explicitly set. A local `--enable-docker=false` or `PULSE_ENABLE_DOCKER=false` is a hard opt-out and is not re-enabled by auto-detection or remote profile config.
- **Kubernetes**: Enabled automatically by the installer when a kubeconfig is detected and `PULSE_ENABLE_KUBERNETES` was not explicitly set.
- **Proxmox**: Enabled automatically by the installer when Proxmox is detected. Type auto-detects `pve` vs `pbs` if not specified.

To disable auto-detection, explicitly set the relevant flags or env vars, for example:

- `--enable-docker=false` or `PULSE_ENABLE_DOCKER=false`
- `--enable-kubernetes=false` or `PULSE_ENABLE_KUBERNETES=false`
- `--enable-proxmox=false` or `PULSE_ENABLE_PROXMOX=false`

### Inside-Guest Runtime Boundaries

Docker/Podman inside a VM or LXC is monitored from inside that guest. Install the
Unified Agent in the guest when you want full Docker host, container, service,
and task inventory on the Docker page.

Pulse does not use a Proxmox node agent to look inside LXCs by default. The
node agent does automatically collect filesystem capacity for running LXCs
when the local `pct` tool is available. It uses bounded `pct list` and
`pct df <vmid>` calls and reports only mount keys, volume labels, mount paths,
and capacity/usage values; it does not run commands inside a guest or read
guest files. Stopped LXCs retain the normal API-derived disk view.

The optional Proxmox-side LXC Docker hint is off unless the Pulse server is started
with `PULSE_ENABLE_PROXMOX_GUEST_DOCKER_DETECTION=true`. That hint uses
`pct exec` only to check whether `/var/run/docker.sock` exists in a running LXC;
it does not enumerate containers, images, environment variables, files, or
processes. The stronger Proxmox-side LXC Docker inventory path is separately
disabled by default. An admin can turn it on with the **Discover Docker in
LXC guests** toggle in Settings → System → General, or the server can be
started with `PULSE_ENABLE_PROXMOX_GUEST_DOCKER_INVENTORY=true`, which locks
the toggle to the environment value. Use either path only when
operators are comfortable with Proxmox-side guest probing.

## Installation Options

### Simple Install (host + Docker auto-detect)
```bash
curl -fsSL http://<pulse-ip>:7655/install.sh | \
  bash -s -- --url http://<pulse-ip>:7655 --token <token>
```

### Proxmox VE Node (explicit profile)
```bash
curl -fsSL http://<pulse-ip>:7655/install.sh | \
  bash -s -- --url http://<pulse-ip>:7655 --token <token> --enable-proxmox --proxmox-type pve
```

### Proxmox Backup Server Node (explicit profile)
```bash
curl -fsSL http://<pulse-ip>:7655/install.sh | \
  bash -s -- --url http://<pulse-ip>:7655 --token <token> --enable-proxmox --proxmox-type pbs
```

### Force Enable Docker (if auto-detection fails)
```bash
curl -fsSL http://<pulse-ip>:7655/install.sh | \
  bash -s -- --url http://<pulse-ip>:7655 --token <token> --enable-docker
```

### Disable Docker (even if detected)
```bash
curl -fsSL http://<pulse-ip>:7655/install.sh | \
  bash -s -- --url http://<pulse-ip>:7655 --token <token> --enable-docker=false
```

### Host + Kubernetes Monitoring
```bash
curl -fsSL http://<pulse-ip>:7655/install.sh | \
  bash -s -- --url http://<pulse-ip>:7655 --token <token> --enable-kubernetes
```

### Docker Monitoring Only
```bash
curl -fsSL http://<pulse-ip>:7655/install.sh | \
  bash -s -- --url http://<pulse-ip>:7655 --token <token> --enable-host=false --enable-docker
```

### Exclude Specific Disks from Monitoring
```bash
# Exclude whole block devices by name or path
pulse-agent --disk-exclude sda --disk-exclude /dev/sdb

# Exclude specific mount points
pulse-agent --disk-exclude /mnt/backup --disk-exclude /var/run/samba/fd

# Exclude using patterns (prefix match)
pulse-agent --disk-exclude '/mnt/pbs*'  # Matches /mnt/pbs-data, /mnt/pbs-backup, etc.

# Exclude using patterns (contains match)
pulse-agent --disk-exclude '*pbs*'  # Matches any path containing 'pbs'

# Via environment variable (comma-separated)
PULSE_DISK_EXCLUDE=/dev/sda,*pbs*,/var/run/samba/fd
```

**Pattern types:**
- Exact: `/dev/sda`, `sda`, or `/mnt/backup` - matches that device path, device name, or mount point
- Prefix: `/dev/nvme*` or `/mnt/ext*` - matches device paths or mount points with that prefix
- Contains: `*cache*` or `*pbs*` - matches device paths, device names, or mount points containing that text

Exclusions are applied before filesystem usage, disk I/O, and S.M.A.R.T. collection.
On linked Proxmox hosts, matching physical-disk health and SSD wear alerts are
also suppressed.

### Include an Automatically Filtered Filesystem

Pulse normally filters pseudo-filesystems such as `tmpfs` to keep system mounts
out of disk monitoring. A specific mount can be opted back in when its capacity
matters, such as a log2ram volume mounted at `/var/log`.

```bash
pulse-agent --disk-include /var/log

# Via environment variable
PULSE_DISK_INCLUDE=/var/log
```

Include patterns use the same exact, prefix, and contains matching rules as
exclusions. They only override automatic filesystem filtering. An explicit
`--disk-exclude` match still wins.

## S.M.A.R.T. Disk Health

The agent can report S.M.A.R.T. disk temperatures, health status, identity, and normalized health counters when running in Agent mode. This requires:

1. **smartmontools** installed on the host:
   ```bash
   # Debian/Ubuntu
   apt install smartmontools

   # RHEL/CentOS
   yum install smartmontools

   # Alpine
   apk add smartmontools
   ```

2. The agent must have permission to run `smartctl` (typically requires root)

**Notes:**
- Disks in standby mode are reported as such (no temperature) to avoid waking them
- S.M.A.R.T. data is collected alongside other host metrics and can enrich the Physical Disks view with temperature, stable disk identity, power-on hours, SSD life, pending sectors, media errors, and related counters
- If `smartctl` is not available, S.M.A.R.T. monitoring is silently skipped
- **Disk exclusions** (`--disk-exclude` / `PULSE_DISK_EXCLUDE`) also apply to S.M.A.R.T. monitoring.
  Use patterns like `sda`, `/dev/sdb`, `nvme*`, or `*cache*` to exclude specific block devices.

## Auto-Update

Eligible v6 agents automatically check the Pulse server for updates every hour.
The check is asynchronous: updating the Pulse server changes the target version,
but does not prove every agent is online, eligible, or already current. When a
new version is available:

1. Agent downloads the new binary from the Pulse server
2. Verifies the checksum
3. Verifies the release signature when trusted update keys are embedded
4. Runs the downloaded binary with `--self-test`
5. Replaces itself atomically (with backup)
6. Restarts with the same configuration

Use the manual update path for v5 agents, PVE host agents, agents with
auto-update disabled, and agents blocked by authentication, missing connection
state, download, trust, or self-test failures. Open an outdated-agent notice or
`/settings/infrastructure?agentDoctor=1` to open **Agent Doctor** and
copy the command for each reported host. Pulse does not remotely execute those
commands.

If an already-installed v5 `pulse-agent` follows its legacy automatic updater
path instead of the supported manual installer path, the first hop is performed
by the v5 updater. That hop verifies TLS by default, the SHA-256 checksum,
executable magic, size limits, and atomic replacement, but the newer v6
signature and `--self-test` checks apply only after the agent has landed on v6.
Use HTTPS or a trusted local network for that legacy migration. For
high-assurance environments, install the v6 `pulse-agent` through the signed
installer path instead of relying on a plain-HTTP first hop.

To disable auto-updates:
```bash
# During installation
curl -fsSL http://<pulse-ip>:7655/install.sh | \
  bash -s -- --url http://<pulse-ip>:7655 --token <token> --disable-auto-update

# Or set environment variable
PULSE_DISABLE_AUTO_UPDATE=true
```

## Remote Configuration (Agent Profiles, Pro/legacy Pro+/Cloud)

Pro, legacy Pro+, and Cloud can push centralized settings to agents via Agent Profiles.

Behavior:
- The agent fetches remote config on startup from `/api/agents/agent/{agent_id}/config`.
- Profile settings override local flags/env for supported keys.
- Profile changes take effect on the next agent restart.
- Command execution (`commandsEnabled`) is controlled per agent from the Infrastructure agent controls and can change live.
- Remote config responses can be signed with `PULSE_AGENT_CONFIG_SIGNING_KEY` (base64 Ed25519 private key).
- To require signed payloads, set `PULSE_AGENT_CONFIG_SIGNATURE_REQUIRED=true` on Pulse and agents.
- If you use a custom signing key, set `PULSE_AGENT_CONFIG_PUBLIC_KEYS` on agents to trust the matching public key.

See [Centralized Agent Management](CENTRALIZED_MANAGEMENT.md) for supported keys and profile setup.

## Uninstall

```bash
curl -fsSL http://<pulse-ip>:7655/install.sh | bash -s -- --uninstall
```

This removes:
- The agent binary
- The systemd/launchd service

## Migration Notes

Use the unified installer (`install.sh`) for all new and existing deployments.

## Health Checks & Metrics

The agent exposes HTTP endpoints for health checks and Prometheus metrics on port 9191 by default.

### Endpoints

| Endpoint | Description |
|----------|-------------|
| `/healthz` | Liveness probe - returns 200 if agent is running |
| `/readyz` | Readiness probe - returns 200 when agents are initialized |
| `/metrics` | Prometheus metrics |

### Prometheus Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `pulse_agent_info` | Gauge | Agent info with version, host_enabled, docker_enabled labels |
| `pulse_agent_up` | Gauge | 1 when running, 0 when shutting down |

### Kubernetes Probes

```yaml
livenessProbe:
  httpGet:
    path: /healthz
    port: 9191
  initialDelaySeconds: 5
  periodSeconds: 10

readinessProbe:
  httpGet:
    path: /readyz
    port: 9191
  initialDelaySeconds: 5
  periodSeconds: 5
```

### Disable Health Server

Set `--health-addr=""` or `PULSE_HEALTH_ADDR=off` to disable the health/metrics server. Set `--health-addr :9191` when network Prometheus scraping is intentional.

## Troubleshooting

### pfSense service disabled after a major upgrade

Pulse installs two service files on pfSense: the FreeBSD rc.d service at
`/usr/local/etc/rc.d/pulse-agent` and an executable
`/usr/local/etc/rc.d/pulse_agent.sh` boot wrapper. pfSense only runs custom
scripts from that directory at boot when their filename ends in `.sh`.

If both files still exist after a pfSense upgrade, run the following as root to
restore the enable flag, permissions and running service:

```sh
sysrc pulse_agent_enable=YES
chmod 755 /usr/local/etc/rc.d/pulse-agent /usr/local/etc/rc.d/pulse_agent.sh
service pulse-agent start
service pulse-agent status
```

If either file is missing, copy the current command from **Settings →
Infrastructure → Install on a host** and run it again on the same firewall. Do
not uninstall the existing agent first. The installer repairs the rc.d service,
enable flag and pfSense boot wrapper in place and reuses saved connection and
agent identity state when it is still present.

### Installer Fails With "Not enough free disk space"

The installer stages the agent binary (~34 MiB) in a temporary directory before
moving it to the install directory, and checks free space in both before
downloading. On appliances whose root filesystem is a small RAM disk (QNAP QTS,
Unraid), `/tmp` and `/usr/local/bin` share that filesystem, so both the staged
and installed copy must fit at once.

If the check fails because `/tmp` is on a constrained root, point `TMPDIR` at a
directory on a data volume and re-run the installer:

```bash
TMPDIR=/share/CACHEDEV1_DATA/tmp bash install.sh --url http://pulse --token <token>
```

(`mktemp` honours `TMPDIR`, so this moves the staging copy off the RAM root.
Create the directory first if it does not exist.)

On QNAP the agent's rotating log is written to the data volume
(`<data-volume>/.pulse-agent/logs/pulse-agent.log`); on Unraid it is written to
`/var/log/pulse-agent/pulse-agent.log` with size-capped rotation. If an older
install filled `/var/log/pulse-agent.log` on the root filesystem, delete that
file and re-run the installer to pick up the rotating configuration.

### Agent Not Updating
- Check logs: `journalctl -u pulse-agent -f`
- Verify network connectivity to Pulse server
- Ensure auto-update is not disabled
- Confirm the agent can authenticate and that its saved connection state still
  identifies the Pulse URL and token.
- Open **Agent Doctor** from an outdated-agent notice or
  `/settings/infrastructure?agentDoctor=1` and use the command for that reported
  host. Do not substitute the public GitHub server installer.
- Administrators can query the read-only Agent Fleet Doctor endpoint,
  `GET /api/agents/diagnostics`, for liveness, version, profile, telemetry, and
  identity evidence. The endpoint reports repair handoffs but does not run them.

### Duplicate Agents
If cloned VMs appear as the same agent:
```bash
sudo rm /etc/machine-id && sudo systemd-machine-id-setup
```

Or set a unique agent ID:
```bash
--agent-id my-unique-agent-id
```

The displayed or reported IP is not the durable agent identity. Pulse normally
uses the machine ID (or an explicit `--agent-id`), so cloned systems must have
unique machine and agent IDs even when their hostnames, MAC addresses, and IPs
differ.

### Permission Denied (Docker)
Ensure the agent can access the Docker socket:
```bash
sudo usermod -aG docker $USER
```

### Check Status
```bash
# Linux
systemctl status pulse-agent

# macOS
launchctl list | grep pulse
```

### Docker Swarm Not Detected

If your Docker Swarm cluster isn't being detected:

1. **Check runtime detection**: Pulse disables Swarm for Podman. Look for "Podman runtime detected" in logs:
   ```bash
   journalctl -u pulse-agent | grep -i podman
   ```

2. **Force Docker runtime**: If auto-detection is incorrect:
   ```bash
   --docker-runtime docker
   # Or set environment variable
   PULSE_DOCKER_RUNTIME=docker
   ```

3. **Check Docker info**: Verify Swarm is active on the host:
   ```bash
   docker info | grep -i swarm
   # Should show "Swarm: active"
   ```

4. **Check socket permissions**: The agent needs access to the Docker socket:
   ```bash
   ls -la /var/run/docker.sock
   ```

5. **Enable debug logging**: For more detail:
   ```bash
   # Set the service to debug as described under "Agent log level", restart it,
   # then follow the service journal.
   journalctl -u pulse-agent -f
   ```

### PVE Backups Not Showing (Recovery)

If local PVE backups aren't appearing in Pulse after setting up via `--enable-proxmox`:

1. **Check permissions**: The API token needs `PVEDatastoreAdmin` on `/storage`:
   ```bash
   pveum aclmod /storage -user pulse-monitor@pve -role PVEDatastoreAdmin
   pveum aclmod /storage -token 'pulse-monitor@pve!<token-name>' -role PVEDatastoreAdmin
   ```
   Replace `pulse-monitor@pve!<token-name>` with the full token ID shown in Pulse.
   Privilege-separated PVE tokens need the storage ACL on the token as well as the service user.

2. **Re-run setup**: Delete the node in Pulse Settings and re-run the agent with `--enable-proxmox`. Recent versions grant this permission automatically.

3. **Check state file**: If re-running doesn't trigger setup, remove the state file:
   ```bash
   rm /var/lib/pulse-agent/proxmox-pve-registered
   ```
   Then restart the agent.
