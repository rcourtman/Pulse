# Deployment Models

Pulse supports multiple deployment models. This page clarifies what differs between them and where “truth” lives (paths, updates, and operational constraints).

## Summary

| Model | Recommended for | Data/config path | Updates |
| --- | --- | --- | --- |
| Proxmox VE LXC (installer) | Proxmox-first deployments | `/etc/pulse` | In-app updates supported |
| systemd (bare metal / VM) | Traditional Linux hosts | `/etc/pulse` | In-app updates supported |
| Docker | Quick evaluation and container stacks | `/data` (bind mount / volume) | Image pull + restart |
| Kubernetes (Helm) | Cluster operators | `/data` (PVC) | Helm upgrade |
| Provider-hosted MSP | Managed service providers, request-assisted | Provider control plane data plus one tenant data directory per client runtime | Provider control plane rollout plus per-client runtime rollout |

## Common Ports

- UI/API: `7655/tcp`
- Prometheus metrics: `9091/tcp` (`/metrics` on a separate listener)

Docker and Kubernetes do not publish `9091` unless you explicitly expose it.

## Where Configuration Lives

Pulse uses a split config model:

- **Local auth and secrets**: `.env` (managed by Quick Security Setup or environment overrides, not shown in the UI)
- **Encryption key**: `.encryption.key` (required to decrypt `.enc` files)
- **Audit signing key**: `.audit-signing.key` (Pro/legacy Pro+/Cloud, encrypted)
- **System settings**: `system.json` (editable in the UI unless locked by env)
- **Nodes and credentials**: `nodes.enc` (encrypted)
- **Notification config**: `email.enc`, `webhooks.enc`, `apprise.enc` (encrypted)
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
- **Audit log database**: `audit.db` (Pro/legacy Pro+/Cloud, SQLite)
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
- **TrueNAS connections**: `truenas.enc` (encrypted)
- **Relay config**: `relay.enc` (encrypted, Relay and above)
- **RBAC roles**: `rbac_roles.json` (Pro/legacy Pro+/Cloud)

Path mapping:

- systemd/LXC: `/etc/pulse/*`
- Docker/Helm: `/data/*`

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

### systemd and Proxmox LXC

Use the UI:

- **Settings → System → Updates**

These deployments can apply updates by downloading a release and swapping binaries/config safely with backups and history.

### Docker

Pull a new image and restart:

```bash
docker pull rcourtman/pulse:latest
docker compose up -d
```
### Kubernetes (Helm)

Upgrade the chart:

```bash
helm repo update
helm upgrade pulse pulse/pulse -n pulse
```

### Provider-hosted MSP

Provider-hosted MSP is not the same as enabling shared-process organizations in a normal Pulse install. The provider-hosted path runs a control plane that creates an isolated Pulse runtime for each client workspace. Alerts, webhook destinations, branded report settings, users, audit history, and metrics stay inside the client runtime. Duplicate hostnames across clients do not collide because they never share the same runtime namespace.

Access is request-assisted while MSP is staged for rollout. The deployable model is license-backed by a signed MSP license, not by Stripe checkout or environment-only plan selection.
