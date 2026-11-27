# Deployment Models

Pulse supports multiple deployment models. This page clarifies what differs between them and where “truth” lives (paths, updates, and operational constraints).

## Summary

| Model | Recommended for | Default data/config path | Updates |
| --- | --- | --- | --- |
| Proxmox VE LXC (installer) | Proxmox-first deployments | `/etc/pulse` | In-app updates supported |
| systemd (bare metal / VM) | Traditional Linux hosts | `/etc/pulse` | In-app updates supported |
| Docker | Quick evaluation and container stacks | `/data` (bind mount / volume) | [Pull + recreate](#docker) |
| Kubernetes (Helm) | Cluster operators | `/data` (PVC) | Helm upgrade |
| Provider-hosted MSP | Managed service providers (self-serve 60-day evaluation and paid plans) | Provider control plane data plus one tenant data directory per client runtime | Provider control plane rollout plus per-client runtime rollout |

## Common Ports

- UI/API: `7655/tcp`
- Optional split-port agent ingest: `PULSE_AGENT_INGEST_PORT` (for example `7656/tcp`, see [Split-Port Agent Ingest](CONFIGURATION.md#split-port-agent-ingest-network-isolation))
- Prometheus metrics: `9091/tcp` (`/metrics` on a separate listener)

Docker and Kubernetes do not publish `9091` unless you explicitly expose it.

## Where Configuration Lives

Pulse uses a split config model:

- **Local auth and secrets**: `.env` (managed by Quick Security Setup or environment overrides, not shown in the UI)
- **Encryption key**: `.encryption.key` (required to decrypt `.enc` files)
- **Audit signing key**: `audit/.audit-signing.key` (default store, encrypted; preserve with its matching encryption key and history)
- **System settings**: `system.json` (editable in the UI unless locked by env)
- **Nodes and credentials**: `nodes.enc` (encrypted)
- **TrueNAS connections**: `truenas.enc` (encrypted)
- **VMware vSphere connections**: `vmware.enc` (encrypted)
- **Availability check targets**: `availability_targets.enc` (encrypted)
- **Alert configuration and overrides**: `alerts.json`
- **Alert intent policies**: `alert_intent_policies.json`
- **Notification config**: `email.enc`, `webhooks.enc`, `apprise.enc` (encrypted)
- **External watchdog (dead-man switch)**: `deadman.enc` (encrypted)
- **Report schedules**: `report_schedules.json`
- **OIDC config**: `oidc.enc` (encrypted)
- **SSO config**: `sso.enc` (encrypted)
- **API tokens**: `api_tokens.json`
- **AI config**: `ai.enc` (encrypted)
- **AI patrol data**: `ai_findings.json`, `ai_patrol_runs.json`, `ai_usage_history.json`
- **AI chat sessions**: `ai_chat_sessions.json` (legacy UI sync)
- **AI baseline data**: `baselines.json`
- **AI correlation data**: `ai_correlations.json`
- **AI pattern data**: `ai_patterns.json`
- **AI remediation data**: `ai_remediations.json`
- **AI incident tracking**: `ai_incidents.json`
- **Audit log database**: `audit/audit.db` (default store; persistent capture on all plans, licensed query/export)
- **Relay/Pro/legacy Pro+/Cloud license**: `license.enc` (encrypted)
- **Host metadata**: `host_metadata.json`
- **Docker metadata**: `docker_metadata.json`
- **Guest metadata**: `guest_metadata.json`
- **Agent profiles**: `agent_profiles.json`
- **Agent profile assignments**: `agent_profile_assignments.json`
- **Agent profile versions**: `profile-versions.json`
- **Agent profile deployments**: `profile-deployments.json`
- **Agent profile changelog**: `profile-changelog.json`
- **Sessions**: `sessions.json` (persistent sessions, sensitive)
- **Recovery tokens**: `recovery_tokens.json`
- **Update history**: `update-history.jsonl`
- **Metrics history**: `metrics.db` (SQLite)
- **Organization metadata**: `org.json` (Enterprise/internal multi-org)
- **Relay config**: `relay.enc` (encrypted, existing paired Pulse Mobile devices until retirement on 31 March 2027)
- **RBAC roles**: `rbac_roles.json` (Pro/legacy Pro+/Cloud)

Default path mapping:

- systemd/LXC: `/etc/pulse/*`
- Docker/Helm: `/data/*`

These are defaults, not a backup inventory. Check the active deployment's
`PULSE_DATA_DIR`, mounts and any external stores before copying or restoring
files. Metrics can use a separate `PULSE_METRICS_DB_PATH`; see
[Metrics storage](METRICS_HISTORY.md#storage-location).

The audit paths above are the default store, not a licence-dependent guarantee
that storage initialised successfully. Runtime-specific storage can differ;
see [Audit storage and safe recovery](AUDIT_LOGGING.md#storage) before restoring
history or keys. Do not replace keys in a live instance to clear a verification
failure.

Enterprise/internal multi-org layout:
- Default org uses the root data dir for backward compatibility.
- Non-default orgs use `/orgs/<org-id>/`.
- Migration may create `/orgs/default/` and symlinks in the root data dir.

Provider-hosted MSP layout:
- The MSP runs a Stripe-free provider control plane.
- A signed MSP license is the activation source and sets the provider plan plus client workspace cap.
- Each client workspace runs as its own isolated Pulse runtime/container with its own data, metrics, alerts, webhooks, report settings, users, and audit history.
- Pulse Account is the provider control plane for creating client workspaces and handing operators into the correct tenant-local Pulse runtime.
- Ordinary self-hosted Pulse deployments do not use this model unless the operator deliberately enters the MSP path.

## Updates by Model

Before changing the server binary or image, record its running version and
edition, keep the saved deployment configuration, and take a consistent private
backup of the actual data and external stores. Preserve encrypted files with
their matching `.encryption.key` and audit signing key. Use a stopped, consistent
filesystem/volume backup or supported backup tooling; copying a live `.db` file
alone can omit SQLite sidecar files. Do not post configuration files, environment
dumps or Helm secret values in an issue.

An update snapshot is not necessarily a complete data backup, and reverting a
binary or image does not undo data migrations. Check
[rollback and backup scope](AUTO_UPDATE.md#rollback) before updating or reverting;
keep the previous binary/image and matching data backup until recovery is checked.

### systemd and Proxmox LXC

For a planned rollout on a memory-limited LXC, save a baseline on the current
version **before** starting the update:

- Note the Pulse version and uptime, the LXC memory limit and swap allowance,
  and the start and end of a window that includes your usual busy polls.
- Keep the container's peak memory for that window and any recorded swap use,
  pressure counters, kernel kills or unexpected restarts. Keep Pulse's process
  RSS separate. A peak from before the window is not a measurement of it.
- Save a recent guest History timestamp and, if you monitor backups, the
  archive count and date at an existing backup location. Check the same guests
  and backup location after the update.

Compare windows of the same length at similar uptime and at the same limit,
swap allowance and workload. Without that comparison, leave memory headroom
unverified. Fresh readings still matter, but they are not enough to widen the
rollout. Do not change limits, reset counters or start another backup to fill
in missing evidence.

Use **Settings → System → Updates**. Follow
[server update guidance](AUTO_UPDATE.md) for the deployment's update controls and
Update History. Pulse attempts snapshot and rollback operations; a recorded
backup path alone does not establish that all active data was copied or that
recovery succeeded.

For LXC, Pulse's service and data are inside the container, not on the Proxmox
host. Keep the failed state and check the running version and service health
before retrying a failed update.

### Docker

Follow [Docker server updates](DOCKER.md#-updates) from the original Compose
project or saved container deployment. Persist the exact target in its `image:`
line, or in `PULSE_IMAGE` when that line uses the variable; pulling a different
tag alone does not change the configured image. Paid Pro installs must keep
their private image, not replace it with the Community image.

Pull and recreate only the Pulse service, stopping if the pull fails. Preserve
its data volume, ports and settings, and check the running image and server
version afterwards. This updates the Pulse server, not its agents or the
containers it monitors; there is no need to restart unrelated Compose services.

### Kubernetes (Helm)

Use the existing Helm release and namespace, the chosen chart version and your
saved deployment values. Review changes to persistence, Secret references and
image edition before rollout; keep the current PVC, keys and settings. Do not
let a default Community image replace paid Pro.

See [Kubernetes deployment settings](KUBERNETES.md) and use your cluster's
rollout and recovery procedure. A Helm rollback changes release resources; it
does not guarantee restoration of Pulse data changed by a newer binary.

### Provider-hosted MSP

Provider-hosted MSP is not the same as enabling shared-process organizations in a normal Pulse install. The provider-hosted path runs a control plane that creates an isolated Pulse runtime for each client workspace. Alerts, webhook destinations, branded report settings, users, audit history, and metrics stay inside the client runtime. Duplicate hostnames across clients do not collide because they never share the same runtime namespace.

The deployable model is backed by a signed MSP licence (Solo, Starter, Growth or Scale by client workspace cap). The 60-day, two-workspace evaluation and paid provider-hosted plans are self-serve through the provider bundle and **Plan** in the provider portal, with Enterprise or custom terms available for larger deployments. See [MSP.md](MSP.md) for installation, portal sign-in and licensing.
