# Production Deployment and Security

This guide answers the questions that matter before connecting Pulse to a
production or multi-cluster estate: whether it needs root, what it can change,
what it scans, how credentials and updates are handled, what Community includes,
and what scale has actually been tested.

The short version is: **start with API-only monitoring and add privileged agents
only where the extra telemetry is worth the additional trust**. Pulse does not
require an agent on every Proxmox host to monitor normal cluster inventory,
utilization, storage, guests, and status.

## Choose the least-privileged collection path

| Collection path and coverage | Privilege boundary |
| --- | --- |
| **Proxmox API only:** PVE, PBS, and PMG inventory; node and guest state; storage usage; and normal API metrics | No Pulse software runs on the Proxmox host. Use a dedicated read-only or narrowly scoped API token. |
| **Proxmox API plus host agent:** API coverage plus host-local SMART, temperatures, ZFS/Ceph/mdadm detail, arbitrary mount capacity, and full LXC filesystem capacity | The Linux agent runs as `root` by default for the supported full-telemetry profile. |
| **Guest-local agent:** Host metrics and optional Docker/Podman inventory inside a VM or LXC | The agent is trusted inside that guest only; it does not grant Pulse access to the hypervisor. |

For most Proxmox installations, begin with **Settings → Infrastructure →
Platform connections**. Add a host agent only after identifying a metric that
the Proxmox API cannot provide. The complete capability matrix is in
[Agent Security](AGENT_SECURITY.md#proxmox-deployment-choices).

Generated Proxmox API setup uses a separate `pulse-monitor` account and token
with monitoring ACLs. Review the generated script before running it, especially
if your organization maintains its own Proxmox roles. Do not reuse an
administrator's interactive account or a token that has unrelated write
permissions.

## What trusting a root agent means

Pulse's Linux/systemd agent runs as `root` by default because SMART devices,
temperature sensors, Docker or Podman sockets, host-local storage state, and
some platform integrations require root or equivalent access. That is a real
security boundary, not a cosmetic implementation detail.

The fresh-install posture limits that boundary:

- the service is marked `monitoring-only`, remote configuration cannot promote
  it, and its credential omits `agent:exec`;
- the health and Prometheus listener binds to `127.0.0.1:9191` by default;
- generated systemd units apply service hardening including
  `NoNewPrivileges=true`, private temporary storage, and kernel/control-group
  write protection;
- an agent reports to one authoritative Pulse server; additional observer
  connections are report-only;
- Proxmox guest Docker inventory through `pct exec` is disabled by default and
  requires an explicit server setting.

The advanced **legacy combined command profile** is a separate trust decision.
It marks the root collector command-capable and gives its credential execution
scope so the same process can accept governed server requests. Existing
unmarked installations remain in a visible `legacy` compatibility state during
the migration. Agent Doctor reports both local authority and credential scope
so an over-scoped monitoring install is not mistaken for a safe default.

On standard Linux systemd hosts the installer also offers a supported
least-privilege profile: `--least-privilege` runs the service as a dedicated
`pulse-agent` system user, with optional `--grant-smart` and `--grant-pct`
flags that restore SMART and Proxmox LXC filesystem collection through
exact-command sudoers grants. Command execution and `pct exec` guest inventory
stay root-profile features. If API data is sufficient, API-only monitoring
remains the cleanest least-privilege choice of all — it needs no agent.

See [Agent Security](AGENT_SECURITY.md) for the precise command, guest-access,
update, and service-hardening boundaries.

## Discovery and network access

Network discovery is **disabled by default** (`DISCOVERY_ENABLED=false`). Pulse
does not need discovery once you have configured known platform connections.

When enabled, discovery probes Proxmox service ports `8006` and `8007` across
the selected address range. It does not perform a general all-port scan. The
scan is bounded by configurable subnet allowlists and blocklists, a default
maximum of 1,024 hosts per run, 50 concurrent probes, and one- and two-second
dial and HTTP timeouts. In a fixed production inventory, leave it off. If you
do use it, set an explicit allowlist rather than relying on automatic subnet
selection.

Normal network paths are:

- Pulse server → Proxmox API over HTTPS;
- agent → Pulse server for enrollment, reports, configuration, and updates;
- browser → Pulse web UI/API, preferably through HTTPS or a trusted reverse
  proxy.

For agents crossing an untrusted network, use HTTPS and consider the
[split-port agent ingest](CONFIGURATION.md#split-port-agent-ingest-network-isolation)
mode so the agent control plane can be firewalled separately from the web UI
and management API.

## Credentials, authentication, and data

- Complete the bootstrap-token flow and keep authentication enabled. Do not
  expose an unfinished first-run setup to an untrusted network.
- Pulse encrypts stored platform credentials with AES-256-GCM. Back up the
  encryption key with the data directory; without it, encrypted configuration
  cannot be recovered.
- Use a separate API token for each platform or trust boundary. Grant only the
  permissions Pulse needs, and rotate a token before revoking the credential
  currently used by a live connection or agent.
- Do not put host SSH private keys in a Pulse Docker or LXC container. Pulse
  blocks that pattern; use API-only monitoring or a host-local agent instead.
- Restrict the Pulse data directory and backups as secrets. They contain
  encrypted credentials as well as sessions, API-token state, and the key
  material needed by the installation.

The canonical disclosure, reporting process, and hardening checklist are in
the [Security Policy](../SECURITY.md). Data and optional telemetry behavior are
covered in [Privacy](PRIVACY.md).

## Installation and update integrity

For a controlled production installation:

1. Choose an exact release tag rather than `latest`.
2. Download `install.sh` and `install.sh.sshsig` from that release.
3. Verify the installer with the published Ed25519 installer key and the
   `pulse-install` namespace.
4. Run the verified installer with the same pinned version.
5. Install or upgrade agents with the per-host command shown by your own Pulse
   server under **Settings → Infrastructure → Install on a host**.

The agent update path requires a server-provided SHA-256 checksum. Release
builds with embedded trusted update keys also require a valid Ed25519 signature
before replacing the running binary, then validate the executable and run its
self-test before the atomic swap.

Piping any network-fetched shell script into a root shell grants that script
root at that moment. The signature-verified, version-pinned flow exists so you
can verify the installer before execution. See [Installation](INSTALL.md) and
[Agent Security](AGENT_SECURITY.md#supply-chain-boundary) for the commands and
the documented v5-to-v6 first-update limitation.

## Community, Relay, and Pro boundaries

Current self-hosted plans do not sell monitoring capacity by node, VM,
container, or other child-resource volume. Community includes core self-hosted
monitoring and seven days of metric history. Relay and Pro add capabilities
such as remote access, longer history, Pulse Mobile, Patrol investigation and
governed fixes, RBAC, audit logging, reporting, and centralized agent profiles.

That means an estate with 50 Proxmox hosts does not need Pro merely to add the
fiftieth host. Review the current [Plans and Entitlements](PULSE_PRO.md) for the
runtime-aligned feature boundary.

## What the scale evidence proves

Pulse has repeatable automated performance coverage for a simulated 500-node
Proxmox estate. The API load suite builds 500 nodes plus 2,500 VMs (3,000
resources) and exercises concurrent resource, metrics-history, dashboard, and
mixed-endpoint workloads. The metrics-store suite separately exercises 500
nodes, 2,000 metric series, concurrent dashboard readers, continuous writes,
rollups, retention, and query-latency budgets.

Those tests are regression evidence, not a blanket certification of every
500-node topology. Real capacity also depends on guest count, polling interval,
history retention, storage latency, enabled integrations, alert rules, and the
resources assigned to Pulse. A 50-host operator should still stage the rollout
and measure their own workload.

## Production rollout checklist

- [ ] Put Pulse on a dedicated VM, container, Kubernetes deployment, or host
  with durable storage and tested backups.
- [ ] Pin and verify the server release before installation.
- [ ] Enable HTTPS and restrict web/API access at the firewall or reverse
  proxy.
- [ ] Start with a dedicated Proxmox API token and no host agents.
- [ ] Leave discovery disabled, or constrain it to an explicit subnet
  allowlist for a temporary discovery run.
- [ ] Confirm inventory, storage, backup, replication, and alert behavior on
  one cluster before adding the rest.
- [ ] Add root agents only to hosts that need specific local telemetry; keep
  commands disabled unless there is a reviewed operational need.
- [ ] Test loss-of-node, failed-backup, high-capacity, notification, credential
  rotation, upgrade, backup, and restore paths.
- [ ] Observe CPU, memory, database size, write rate, and dashboard latency
  during the staged rollout, then set retention and polling to fit the estate.
- [ ] Record the Pulse version, enabled integrations, privileges, exposed
  ports, backup location, and rollback procedure in the site's runbook.

Pulse is intended to be the operational monitoring layer for the platforms it
supports. It is not a SIEM, a general log-ingestion platform, or a substitute
for an organization's incident escalation and change-control process.

## Independent review

Pulse does not currently claim an independent security certification. The
[Security Review Scope](SECURITY_REVIEW.md) maps the main trust
boundaries to source files, tests, reproducible baseline commands, and the
private disclosure channel so external reviewers can evaluate concrete
behavior rather than marketing claims.
