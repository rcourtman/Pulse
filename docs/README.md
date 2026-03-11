# 📚 Pulse Documentation

Welcome to the Pulse documentation portal. Here you'll find everything you need to install, configure, and master Pulse.

---

## v6 Execution Canonical Source

For Pulse v6 build/release execution work, do not start from this broad docs index.
Use:

1. `docs/release-control/v6/SOURCE_OF_TRUTH.md` for stable human governance and locked decisions
2. `docs/release-control/v6/status.json` for live lane state, lane-to-subsystem ownership, structured evidence references, typed lane/subsystem decision records, and canonical ordered lists
3. `docs/release-control/v6/status.schema.json` for the machine-readable status contract
4. `docs/release-control/v6/subsystems/registry.json` and `docs/release-control/v6/subsystems/registry.schema.json` for subsystem ownership and proof-routing rules
5. `python3 scripts/release_control/status_audit.py --check` if you need a machine-derived evidence health audit
6. `python3 scripts/release_control/registry_audit.py --check` if you need a machine-derived subsystem registry audit
7. `python3 scripts/release_control/subsystem_lookup.py <path> [<path> ...]` if you need subsystem ownership, proof routing, lane context, and relevant decision records for a change

All other documents are supporting references unless explicitly required for evidence.

---

## 🚀 Getting Started

- **[Installation Guide](INSTALL.md)**
  Step-by-step guides for Docker, Kubernetes, and bare metal.
- **[Configuration](CONFIGURATION.md)**  
  Learn how to configure authentication, notifications (Email, Discord, etc.), and system settings.
- **[Deployment Models](DEPLOYMENT_MODELS.md)**  
  Where config lives, how updates work, and what differs per deployment.
- **[Migration Guide](MIGRATION.md)**  
  Moving to a new server? Here's how to export and import your data safely.
- **[Upgrade to v6](UPGRADE_v6.md)**  
  Practical upgrade guidance and post-upgrade checks for Pulse v6.
- **[FAQ](FAQ.md)**
  Common questions and quick answers.

## 🛠️ Deployment & Operations

- **[Docker Guide](DOCKER.md)** – Advanced Docker & Compose configurations.
- **[Kubernetes](KUBERNETES.md)** – Helm charts, ingress, and HA setups.
- **[Reverse Proxy](REVERSE_PROXY.md)** – Nginx, Caddy, Traefik, and Cloudflare Tunnel recipes.
- **[Troubleshooting](TROUBLESHOOTING.md)** – Deep dive into common issues and logs.

## 🔐 Security

- **[Security Policy](../SECURITY.md)** – The core security model (Encryption, Auth, API Scopes).
- **[Privacy](PRIVACY.md)** – What leaves your network (and what doesn’t).
- **[OIDC / SSO](OIDC.md)** – OIDC Single Sign-On configuration (Authentik, Keycloak, Azure AD, etc.).
- **[Proxy Auth](PROXY_AUTH.md)** – Authentik/Authelia/Cloudflare proxy authentication configuration.
- **[Agent Security](AGENT_SECURITY.md)** – Agent self-update verification and API security.

## 📖 Advanced Topics (Pro / Cloud)

- **[AI Autonomy & Safety](AI_AUTONOMY.md)** – Configure patrol autonomy levels, assistant control levels, investigation tuning, and safety guardrails.
- **[Role-Based Access Control (RBAC)](RBAC.md)** – Define custom roles, assign permissions, and integrate with OIDC group mapping.
- **[Audit Logging](AUDIT_LOGGING.md)** – Tamper-evident event logging for compliance, with query, export, and signature verification.

## ✨ New in 6.0

- **[Unified Resource Model](UNIFIED_RESOURCES.md)** – How all platforms merge into one model with task-based navigation.
- **[Unified Navigation Migration](MIGRATION_UNIFIED_NAV.md)** – Upgrading from platform-specific tabs to v6 navigation.
- **[TrueNAS Integration](TRUENAS.md)** – First-class TrueNAS SCALE/CORE monitoring (pools, datasets, disks, snapshots, replication).
- **[Relay / Mobile Remote Access](RELAY.md)** – End-to-end encrypted relay (mobile app public rollout is coming soon; Pro).
- **[Recovery Central](RECOVERY.md)** – Unified backup, snapshot, and replication view across all providers.
- **[Pulse Cloud (Hosted)](CLOUD.md)** – Fully managed hosting with automatic updates and backups.
- **[Pulse AI](AI.md)** – Chat assistant, patrol findings, alert analysis, intelligence, and forecasts.
- **[Metrics History](METRICS_HISTORY.md)** – Persistent metrics storage with configurable retention.
- **[Mail Gateway](MAIL_GATEWAY.md)** – Proxmox Mail Gateway (PMG) monitoring.
- **[Auto Updates](AUTO_UPDATE.md)** – One-click updates for supported deployments.
- **[Multi-Tenant Organizations](MULTI_TENANT.md)** – Isolate infrastructure by organization (Enterprise, opt-in).
- **[Entitlements Overhaul](PULSE_PRO.md)** – Capability-key-based feature gating across Community/Pro/Cloud.

## 💳 Plans (Community / Pro / Cloud)

Pulse is available in three plans:

- **Community**: Pulse Patrol is available to everyone with BYOK (your own AI provider).
- **Pro**: Unlocks auto-fix, alert-triggered analysis, Kubernetes AI analysis, reporting, and agent profiles.
- **Cloud**: Everything in Pro, plus enterprise-grade multi-tenant and volume capabilities (where licensed).

- **[Learn more at pulserelay.pro](https://pulserelay.pro)**
- **[Plans and entitlements](PULSE_PRO.md)** (includes the Community/Pro/Cloud matrix)
- **[AI deep dive](AI.md)**
- **[Multi-Tenant Organizations (Enterprise)](MULTI_TENANT.md)** — Isolate infrastructure by organization for MSPs and multi-datacenter deployments.

## 📡 Monitoring & Agents

- **[Unified Agent](UNIFIED_AGENT.md)** – Single binary for host, Docker, and Kubernetes monitoring.
- **[Centralized Agent Management (Pro/Cloud)](CENTRALIZED_MANAGEMENT.md)** – Agent profiles and remote config.
- **[Proxmox Backup Server](PBS.md)** – PBS integration, direct API vs PVE passthrough, token setup.
- **[TrueNAS](TRUENAS.md)** – TrueNAS SCALE/CORE integration.
- **[ZFS Monitoring](ZFS_MONITORING.md)** – Proxmox-native ZFS pool monitoring.
- **[Storage Architecture](STORAGE_ARCHITECTURE.md)** – Proposed canonical storage, disk, S.M.A.R.T., and topology model for making storage genuinely operator-useful.
- **[VM Disk Monitoring](VM_DISK_MONITORING.md)** – Enabling QEMU Guest Agent for disk stats.
- **[Temperature Monitoring](TEMPERATURE_MONITORING.md)** – Agent-based temperature monitoring (`pulse-agent --enable-proxmox`). Sensor proxy has been removed.
- **[Webhooks](WEBHOOKS.md)** – Custom notification payloads.

## 💻 Development

- **[API Reference](API.md)** – Complete REST API documentation.
- **[Architecture](../ARCHITECTURE.md)** – System design and component interaction.
- **[Contributing](../CONTRIBUTING.md)** – How to contribute to Pulse.

## 📁 Previous Versions

- **[Upgrade to v5](UPGRADE_v5.md)** – Upgrade guidance for v4 → v5 migrations.
- **[v6 Prerelease Runbook](releases/V6_PRERELEASE_RUNBOOK.md)** – Internal release operations used during the v6 RC period.

---

Found a bug or have a suggestion?

[![GitHub Issues](https://img.shields.io/badge/GitHub-Issues-green)](https://github.com/rcourtman/Pulse/issues)
