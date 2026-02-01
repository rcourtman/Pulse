# Deployment Models

Pulse supports multiple deployment models. This page clarifies what differs between them and where “truth” lives (paths, updates, and operational constraints).

## Summary

| Model | Recommended for | Data/config path | Updates |
| --- | --- | --- | --- |
| Proxmox VE LXC (installer) | Proxmox-first deployments | `/etc/pulse` | In-app updates supported |
| systemd (bare metal / VM) | Traditional Linux hosts | `/etc/pulse` | In-app updates supported |
| Docker | Quick evaluation and container stacks | `/data` (bind mount / volume) | Image pull + restart |
| Kubernetes (Helm) | Cluster operators | `/data` (PVC) | Helm upgrade |

## Common Ports

- UI/API: `7655/tcp`
- Prometheus metrics: `9091/tcp` (`/metrics` on a separate listener)

Docker and Kubernetes do not publish `9091` unless you explicitly expose it.

## Where Configuration Lives

Pulse uses a split config model:

- **Local auth and secrets**: `.env` (managed by Quick Security Setup or environment overrides, not shown in the UI)
- **Encryption key**: `.encryption.key` (required to decrypt `.enc` files)
- **Audit signing key**: `.audit-signing.key` (Pulse Pro, encrypted)
- **System settings**: `system.json` (editable in the UI unless locked by env)
- **Nodes and credentials**: `nodes.enc` (encrypted)
- **Notification config**: `email.enc`, `webhooks.enc`, `apprise.enc` (encrypted)
- **API tokens**: `api_tokens.json`
- **Legacy token suppressions**: `env_token_suppressions.json`
- **AI config**: `ai.enc` (encrypted)
- **AI patrol data**: `ai_findings.json`, `ai_patrol_runs.json`, `ai_usage_history.json`
- **AI baseline data**: `baselines.json`
- **AI correlation data**: `ai_correlations.json`
- **AI pattern data**: `ai_patterns.json`
- **AI remediation data**: `ai_remediations.json`
- **AI incident tracking**: `ai_incidents.json`
- **Audit log database**: `audit.db` (Pulse Pro, SQLite)
- **Pulse Pro license**: `license.enc` (encrypted)
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
- **Organization metadata**: `org.json` (multi-tenant)

Path mapping:

- systemd/LXC: `/etc/pulse/*`
- Docker/Helm: `/data/*`

Multi-tenant layout:
- Default org uses the root data dir for backward compatibility.
- Non-default orgs use `/orgs/<org-id>/`.
- Migration may create `/orgs/default/` and symlinks in the root data dir.

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
