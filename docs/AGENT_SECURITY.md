# Agent Security

Pulse agents incorporate several security mechanisms to ensure that the code running on your infrastructure is authentic and untampered with.

**Start with the least privilege that answers your monitoring question.** For
Proxmox VE, PBS, and PMG, that is usually no agent at all: API-only monitoring
with a read-only token covers inventory, status, and metrics, and the
generated setup script creates a privilege-separated monitoring user for it
(see [Proxmox Deployment Choices](#proxmox-deployment-choices)). Install a
host agent only where you want data the platform API cannot provide, and on
Linux consider the supported
[least-privilege profile](#least-privilege-agent-profile) before the root
default.

## Agent Privilege Model

Pulse's Linux/systemd installer runs the unified agent as `root` by default.
That is intentional for full host telemetry: disk SMART data, mdadm/RAID state,
temperature sensors, Docker or Podman socket reads, Proxmox host-local details
that are not available through the API, and some NAS/platform integrations
commonly require root or equivalent local privileges. On Linux/systemd hosts,
the supported alternative is the least-privilege profile documented below; it
trades the root-only collectors it has not been granted for a dedicated
non-root service user.

Treat a host agent like other infrastructure monitoring software with local
root read access:

- install it only on hosts you trust Pulse to monitor;
- keep the agent token scoped to that Pulse server;
- keep the local command-authority profile `monitoring-only` unless you
  explicitly accept the transitional combined command runtime;
- update from signed release assets rather than arbitrary branch snapshots.

The agent is primarily an outbound reporter to your Pulse server. By default it
binds the health and Prometheus endpoints to `127.0.0.1:9191`, so a root agent
does not expose that HTTP surface to the network unless you explicitly opt in.
Set `--health-addr :9191` only when you intentionally scrape the agent from
another host. Use `--health-addr ""` or `PULSE_HEALTH_ADDR=off` to disable the
listener.

Generated Linux/systemd units also include conservative sandboxing such as
`NoNewPrivileges=true`, `PrivateTmp=true`, kernel/control-group write
protection, a private umask, and setuid/personality restrictions. Those
directives reduce service blast radius while keeping the filesystem and device
access needed for full host telemetry, Proxmox token setup, SMART, Docker, and
NAS integrations.

Disk monitoring normally filters pseudo-filesystems such as `tmpfs`. The local
`--disk-include` option can opt a specific device or mount point back into
capacity monitoring, for example a log2ram `/var/log` mount. This reports
filesystem capacity and usage metadata. It does not read or transmit file
contents. Local `--disk-exclude` rules still take precedence.

Fresh installs use a local `monitoring-only` command-authority ceiling and a
credential without `agent:exec`. Remote configuration cannot promote that
service. Selecting the advanced legacy combined command profile at install time
adds `--enable-commands`, records `command-capable`, and issues an execution-
scoped credential. The root monitoring process can then also accept server
command requests through the existing policy and approval surfaces. Existing
unmarked services upgrade as `legacy` during the migration window so upgrades
do not silently revoke an operator's prior command choice. Agent Doctor shows
the process privilege, local authority ceiling, and credential execution scope
separately and warns about mismatches.

Custom numeric sensors are a separate, local configuration boundary. Enabling
them with `--custom-sensors-file` does not enable remote commands and
`--enable-commands` is not required. The server cannot add or alter a custom
sensor command. The agent accepts only absolute executable paths with no
arguments or shell interpretation, bounds concurrency, time, and output, and
revalidates the command before every run. On POSIX systems the configuration,
commands, and immediate command directories must pass ownership, symlink, and
write-permission checks. Treat the configured executables as trusted agent
code: the service commonly runs as root, so only administrators should be able
to replace them.

Agent command tokens must be bound to a host or agent identity before command
registration is accepted. Pulse-minted install-command tokens for the generic
host and Proxmox flows are the only first-use exception: because the server
mints them before the installer knows the final hostname, Pulse binds them to
the first command agent that registers with that token. Those tokens also carry
the operator's command-policy choice into the first accepted report so a stale
policy from an earlier installation cannot immediately disable the replacement
agent. Generic unbound `agent:exec` tokens still fail closed.

## Proxmox Deployment Choices

You do not need a Pulse agent on every Proxmox-related host just to see basic
cluster inventory and utilization. Start with the least-privilege path that
answers your monitoring question:

| Goal | Recommended path | Root agent needed? |
|---|---|---|
| PVE/PBS/PMG inventory, node status, VM/container status, storage usage, and normal Proxmox API metrics | Add the Proxmox connection with a read-only or narrowly scoped API token | No |
| VM guest disk and memory details through QEMU Guest Agent | Use Proxmox API permissions such as `VM.GuestAgent.Audit` and `VM.GuestAgent.FileRead` where supported | No host agent for the Proxmox node |
| All mounted LXC filesystem capacities and usage | Install the Unified Agent on the owning PVE node; it automatically uses bounded `pct list` and `pct df` reads for running LXCs | Yes, on the PVE node |
| Docker/Podman containers inside a VM or LXC through guest-local reporting | Install the agent inside that VM/LXC with Docker/Podman monitoring enabled, or use another explicit guest access/reporting path | Usually requires root or Docker socket-equivalent access |
| Docker containers inside an LXC from a Proxmox host agent | Turn on **Discover Docker in LXC guests** in Settings → System → General (admin only), or start Pulse with `PULSE_ENABLE_PROXMOX_GUEST_DOCKER_INVENTORY=true` to lock it on; optionally limit guests with `PULSE_PROXMOX_GUEST_DOCKER_INVENTORY_VMIDS=101,102` | Requires a root/equivalent Pulse agent on the Proxmox node and explicit server opt-in |
| Host SMART, temperatures, local ZFS/Ceph/mdadm detail, arbitrary mount reads, and full host telemetry | Install the agent on that host | Yes, for the supported full-telemetry profile |
| Kubernetes node/pod monitoring from a cluster | Use the Kubernetes agent/DaemonSet profile | Depends on whether host metrics are enabled |

Inside-guest runtime visibility is explicit. Installing the agent inside a VM or
LXC authorizes that guest-local agent to report Docker/Podman monitoring data
according to its local module flags. A Proxmox node agent does not look inside
LXCs by default. Its automatic LXC filesystem collector is a node-local
capacity query only: it runs `pct list`, then `pct df <vmid>` for guests already
reported running, and reports mount keys, volume labels, mount paths, and
capacity/usage numbers. It does not run a command inside the guest or read
guest files, processes, environment, or container-runtime metadata. It skips
guests reported stopped and bounds command time, output, guest count, and disk
count.

The node agent can collect Docker container inventory from LXC guests
through `pct exec`, but only after an explicit server-side opt-in: the
admin-only **Discover Docker in LXC guests** toggle in Settings → System →
General, or starting the server with
`PULSE_ENABLE_PROXMOX_GUEST_DOCKER_INVENTORY=true` (which locks the toggle).
Inventory collection is disabled by default, can be VMID-allowlisted, and is
limited to the Docker page summary path: Docker host/runtime version, container
ID, name, image, state/status, ports, and aggregate `docker stats` counters.
It does not run `docker inspect` and does not collect guest environment values,
mount sources, container commands, files, or process details. The lighter
socket-presence hint remains separately available through
`PULSE_ENABLE_PROXMOX_GUEST_DOCKER_DETECTION=true`.

For VMs, a Proxmox host agent still cannot see Docker/Podman inventory without
guest cooperation such as a guest-local Pulse agent, QEMU guest-agent mediated
integration, SSH, or an explicitly exposed Docker/Podman reporting endpoint.

If Proxmox API data is enough for your use case, prefer API-only monitoring and
do not install a host agent just because the installer exists. Install agents
where you need data that Proxmox cannot provide through its API, or where the
data lives inside a guest/container rather than at the Proxmox node layer.
The Settings Proxmox setup flow uses this API inventory path as the default;
the host telemetry agent path is for the full-telemetry cases above.
Generated API Inventory setup still needs a one-time privileged shell on the
Proxmox host so it can create the `pulse-monitor` account, token, and ACLs, but
steady-state monitoring uses the Proxmox API rather than a root Pulse agent.
For PVE, the generated script creates a privilege-separated API token and
mirrors the generated read/monitoring ACLs onto both the service user and the
token. For PBS, the generated script grants the `Audit` ACL to both the service
user and token.

## Least-Privilege Agent Profile

For a standard Linux systemd host, the stronger opt-in profile is:

```bash
install.sh --least-privilege --enable-privileged-helper ...
```

This profile keeps the networked collector unprivileged and keeps both the
collector and helper binaries root-owned. Its installer token lives under the
root-owned `/etc/pulse-agent` directory with `root:pulse-agent` group-read
access; mutable identity, buffering, and enrolled monitoring-token state remain
under the collector-owned state directory. The collector is not added to the
rootful Docker group and cannot enable command execution. Automatic binary
replacement uses a separate typed transaction: the collector downloads and
self-tests a signed artifact inside its fixed quarantine, while the helper
revalidates ownership, digest, ELF shape, and signature before promoting it to
root-only staging and atomically activating it. A failed process restart asks
the helper to restore the identity-bound last-known-good binary; there is no
collector-writable direct-replacement fallback.

Exceptional telemetry crosses `/run/pulse-agent/helper.sock` to a separate
root process. The socket admits only the `pulse-agent` UID, and the helper has
no Pulse URL, API token, or network namespace. Its protocol exposes bounded,
versioned SMART and Proxmox LXC filesystem snapshots, not a shell, executable
path, device path, VMID, environment, or caller-selected arguments. The helper
service keeps `PrivateNetwork=true`, `RestrictAddressFamilies=AF_UNIX`,
`NoNewPrivileges=true`, `ProtectSystem=strict`, and `ProtectHome=true`.
`PrivateDevices` is intentionally not enabled because SMART needs the host
block devices. If the helper is missing, incompatible, or rejects a request,
only the affected telemetry disappears; the collector does not fall back to
sudo, root, or a broader local command path.

The typed-helper profile cannot be combined with `--grant-smart` or
`--grant-pct`. Rootful Docker-socket monitoring is also unavailable because
membership in the Docker group is root-equivalent; API monitoring or a
separately scoped rootless runtime socket is required instead. The profile is
currently explicit rather than the installer default. Its inspect, apply, and
rollback transaction is implemented for Linux systemd, but representative
live-host migration and helper update staging/activation/rollback exercises,
container-runtime parity, and appliance qualification are still required
before the profile can become the general default.

Monitoring never implies remediation. On the supported Linux systemd profile,
an operator may separately enroll the typed action runner:

```bash
install.sh --least-privilege --enable-privileged-helper \
  --enable-action-runner --action-token-file /root/pulse-runner.token ...
```

Create that token through the authenticated action-runner credential endpoint
for the exact monitored host; do not reuse the collector token. The installer
keeps the runner binary, service, credential, health record, and receipts
root-owned. Disabling or uninstalling the runner leaves monitoring active.
The safe runner accepts only versioned host update/storage-cleanup, Proxmox
guest lifecycle, and container lifecycle/update requests. Generic shell,
`exec`, unrestricted `read_file`, and deploy operations are rejected.

The existing combined collector command path remains available only as the
explicit legacy/full-trust migration profile. It is not part of the typed
helper/runner security claim and will remain until supported command-enabled
installs have a runner enrollment path and live action-session parity has been
qualified.

Safe-profile conversion is always deliberate:

```bash
install.sh --safe-profile-inspect ...
install.sh --safe-profile-apply ...
install.sh --safe-profile-rollback ...
```

Inspection makes no changes. Apply snapshots collector/helper files and
identity before switching profiles, and rollback restores that snapshot. The
separately installed action runner is not changed by either operation.

The older `--least-privilege` profile without `--enable-privileged-helper`
remains available for compatibility. Its trade-offs are documented below.

On standard Linux systemd hosts, `install.sh --least-privilege` is a supported
alternative to the root profile. It runs the service as a dedicated
`pulse-agent` system user (nologin shell, owning only its state directory and
binary), joins the `docker` group when Docker monitoring is enabled so socket
reads keep working, and keeps every hardening directive of the root unit while
dropping the LXC-attach ambient capability grant entirely.

Two optional flags restore the collectors that genuinely need elevation, each
through an exact-command sudoers grant validated with `visudo` and a
root-owned wrapper the agent is pointed at via an absolute-path-only
environment override. Because `NoNewPrivileges` blocks `sudo` entirely, a
unit with an active grant sets `NoNewPrivileges=false` while keeping the
remaining hardening; a grantless least-privilege install keeps
`NoNewPrivileges=true`. Choose grants deliberately: each one is a scoped,
auditable widening of the profile.

- `--grant-smart` allows exactly `smartctl`, restoring SMART disk health.
- `--grant-pct` allows exactly `pct list` and `pct df`, restoring Proxmox LXC
  filesystem capacity. The grant deliberately excludes `pct exec`, `start`,
  `stop`, and `enter`, so guest Docker inventory stays a root-profile feature.

What the profile gives up: command execution (`--enable-commands` is refused
and a later server-side enable requires reinstalling the root profile),
`pct exec` guest Docker inventory, and any platform integration that needs
device or socket access you have not granted. Core metrics, mounts, `/proc`
RAID state, hwmon temperatures, and Docker socket reads work without root.
Ungranted collectors fail soft, and the agent reports its privilege profile so
**Settings → Infrastructure → Agent Doctor** shows the service user and active
helpers instead of presenting missing collectors as a fault. Appliance
platforms (TrueNAS, Synology, QNAP, Unraid) and non-systemd init systems keep
the root profile; the installer refuses `--least-privilege` there rather than
silently falling back to root.

`--update` preserves either installed least-privilege profile without the flags
being repeated. Uninstall removes the typed helper socket/service and its
installer credential directory, or the legacy sudoers file and wrappers; the
inert system user is left behind deliberately.

## Supply-Chain Boundary

The agent self-update path is not just "download the latest binary and run it".
Release builds require checksum validation, and when trusted update keys are
embedded they also require an Ed25519 release signature before replacing the
running binary.

The initial installer is different: if you paste and run a shell command as
root, you are granting root to that installer at that moment. Prefer the
release-pinned, signature-verified server installer flow documented in
[README.md](../README.md) and [INSTALL.md](INSTALL.md), then use the agent
install command generated by your own Pulse server.

For the server installer, avoid `latest` when you want a tighter change-control
boundary. Download a specific release tag, verify the `install.sh.sshsig`
signature, and pass that same tag to `bash install.sh --version`. Agent
self-updates still verify checksum headers, and release builds require
signatures when a trusted update key is embedded.

The first automatic hop from an already-installed v5 `pulse-agent` to v6 is
performed by the v5 updater. That updater verifies TLS by default, requires the
server-provided SHA-256 checksum, validates executable magic, enforces the size
limit, and swaps atomically, but it does not yet have the v6 Ed25519 signature
requirement or downloaded-binary `--self-test`. For that migration hop, use
HTTPS or a trusted local network. In high-assurance environments, reinstall the
v6 `pulse-agent` through the signed installer path instead of relying on the
automatic v5-to-v6 first hop over plain HTTP.

## Self-Update Security

The agent's self-update mechanism is critical for security and stability. To prevent supply chain attacks or compromised update servers from distributing malicious or broken agents, Pulse employs a rigorous verification process.

### 1. Checksum Verification
The agent verifies a SHA-256 checksum of the downloaded binary. The server must provide
`X-Checksum-Sha256`; updates are rejected if the header is missing or mismatched.

### 2. Signature Verification
Release builds embed trusted Ed25519 update public keys and require
`X-Signature-Ed25519` in addition to the checksum header. Updates are rejected
when the signature is missing or does not verify against the embedded trust
root.

### 3. Pre-Flight Checks
To prevent "brick-updates"—bad updates that crash immediately and require manual recovery—agents perform pre-flight validation before replacing the running executable.

Unified agent (`pulse-agent`):
1. Download new binary.
2. Verify checksum (required).
3. Verify the Ed25519 release signature when trusted update keys are embedded.
4. Validate binary magic (ELF/Mach-O/PE) and size limits (100MB max).
5. Run the downloaded binary with `--self-test`, passing any live token through a short-lived `0600` token file rather than argv.
6. Make executable and swap atomically.

## API Security

- **Token Authentication**: All agent-to-server communication requires a valid API token.
- **TLS**: Encrypted by default (unless specifically disabled).
- **Network Isolation (optional)**: The agent control plane can be served on a dedicated, separately firewalled port. It exposes the bounded report/config, command WebSocket, version, and bootstrap routes needed for the full agent lifecycle, but not the web UI or management API. See [Split-Port Agent Ingest](CONFIGURATION.md#split-port-agent-ingest-network-isolation).
