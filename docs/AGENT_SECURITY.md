# Agent Security

Pulse agents incorporate several security mechanisms to ensure that the code running on your infrastructure is authentic and untampered with.

## Agent Privilege Model

Pulse's Linux/systemd installer runs the unified agent as `root` by default.
That is intentional for full host telemetry: disk SMART data, mdadm/RAID state,
temperature sensors, Docker or Podman socket reads, Proxmox host-local details
that are not available through the API, and some NAS/platform integrations
commonly require root or equivalent local privileges. Running the service as a
lower-privilege user may work for a narrow subset of metrics, but it is not a
supported full-telemetry profile today.

Treat a host agent like other infrastructure monitoring software with local
root read access:

- install it only on hosts you trust Pulse to monitor;
- keep the agent token scoped to that Pulse server;
- keep command execution disabled unless you explicitly need governed
  remediation;
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

Command execution is disabled by default. It can be enabled with
`--enable-commands`, `PULSE_ENABLE_COMMANDS=true`, or the centralized agent
command setting after enrollment. Leave it disabled for read-only monitoring.
When enabled, commands still flow through Pulse's command policy and approval
surfaces instead of silently turning every agent into an unrestricted remote
shell.

Agent command tokens must be bound to a host or agent identity before command
registration is accepted. Proxmox install-command tokens are the only first-use
exception: because the server mints them before the installer knows the final
hostname, Pulse binds them to the first command agent that registers with that
token. Generic unbound `agent:exec` tokens still fail closed.

## Proxmox Deployment Choices

You do not need a Pulse agent on every Proxmox-related host just to see basic
cluster inventory and utilization. Start with the least-privilege path that
answers your monitoring question:

| Goal | Recommended path | Root agent needed? |
|---|---|---|
| PVE/PBS/PMG inventory, node status, VM/container status, storage usage, and normal Proxmox API metrics | Add the Proxmox connection with a read-only or narrowly scoped API token | No |
| VM guest disk and memory details through QEMU Guest Agent | Use Proxmox API permissions such as `VM.GuestAgent.Audit` and `VM.GuestAgent.FileRead` where supported | No host agent for the Proxmox node |
| Docker/Podman containers inside a VM or LXC through guest-local reporting | Install the agent inside that VM/LXC with Docker/Podman monitoring enabled, or use another explicit guest access/reporting path | Usually requires root or Docker socket-equivalent access |
| Docker containers inside an LXC from a Proxmox host agent | Start Pulse with `PULSE_ENABLE_PROXMOX_GUEST_DOCKER_INVENTORY=true`; optionally limit guests with `PULSE_PROXMOX_GUEST_DOCKER_INVENTORY_VMIDS=101,102` | Requires a root/equivalent Pulse agent on the Proxmox node and explicit server opt-in |
| Host SMART, temperatures, local ZFS/Ceph/mdadm detail, arbitrary mount reads, and full host telemetry | Install the agent on that host | Yes, for the supported full-telemetry profile |
| Kubernetes node/pod monitoring from a cluster | Use the Kubernetes agent/DaemonSet profile | Depends on whether host metrics are enabled |

Inside-guest runtime visibility is explicit. Installing the agent inside a VM or
LXC authorizes that guest-local agent to report Docker/Podman monitoring data
according to its local module flags. A Proxmox node agent does not look inside
LXCs by default. It can collect Docker container inventory from LXC guests
through `pct exec`, but only when the server is started with
`PULSE_ENABLE_PROXMOX_GUEST_DOCKER_INVENTORY=true`.
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

Running `pulse-agent` as a custom non-root systemd user is possible by editing
the service unit, but it is not a supported full-telemetry mode today. Expect
gaps in SMART, temperature, Docker socket, ZFS/Ceph/mdadm, mount, and platform
integration data unless you deliberately grant equivalent capabilities or group
access. If you choose that route, treat it as a local hardening profile and
verify the exact metrics you care about after the change.

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
- **Network Isolation (optional)**: Agent check-in can be served on a dedicated, separately firewalled port that exposes only the agent-ingest routes (`/api/agents/*`), so a host that can reach the agent endpoint over an untrusted network cannot pivot to the web UI or management API. See [Split-Port Agent Ingest](CONFIGURATION.md#split-port-agent-ingest-network-isolation).
