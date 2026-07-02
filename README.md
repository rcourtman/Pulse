# Pulse

<div align="center">
  <img src="docs/images/pulse-logo.svg" alt="Pulse Logo" width="120" />
  <p><strong>Real-time monitoring for Proxmox, Docker, Kubernetes, and TrueNAS infrastructure.</strong></p>

  [![GitHub Stars](https://img.shields.io/github/stars/rcourtman/Pulse?style=flat&logo=github)](https://github.com/rcourtman/Pulse)
  [![GitHub release](https://img.shields.io/github/v/release/rcourtman/Pulse)](https://github.com/rcourtman/Pulse/releases/latest)
  [![Docker Pulls](https://img.shields.io/docker/pulls/rcourtman/pulse)](https://hub.docker.com/r/rcourtman/pulse)
  [![License](https://img.shields.io/github/license/rcourtman/Pulse)](LICENSE)

  [Live Demo](https://demo.pulserelay.pro) • [Pulse Pro](https://pulserelay.pro) • [Documentation](docs/README.md) • [Report Bug](https://github.com/rcourtman/Pulse/issues)

  Localized getting started: [Deutsch](docs/i18n/de/README.md) • [Español](docs/i18n/es/README.md)
</div>

---

Issue-first contribution policy: please open an issue or discussion before
investing time in a code change. External pull requests are not part of the
normal contribution flow for this repository. See [CONTRIBUTING.md](CONTRIBUTING.md).

## 🚀 Overview

Pulse is a modern, unified monitoring workspace for your **infrastructure** across Proxmox, Docker, Kubernetes, and TrueNAS. It consolidates metrics, alerts, and AI-powered insights from all your systems into a single, beautiful interface.

Designed for homelabs, sysadmins, internal IT teams, and providers who need a clear monitoring view without the complexity of enterprise monitoring stacks. MSP access is a separate, request-assisted provider path and is not part of ordinary self-hosted setup.

![Pulse Infrastructure](docs/images/01-dashboard.jpg)

## 🧭 Unified Navigation

Pulse now groups everything by task instead of data source:
- **Infrastructure** for hosts and nodes
- **Workloads** for VMs, containers, and Kubernetes pods
- **Storage** and **Backups** as top-level views
- PMG now routes into **Infrastructure** (source filter), and Kubernetes routes into **Workloads** (K8s filter)
- Legacy URLs are no longer routed as compatibility aliases; use canonical v6 routes.

Power-user shortcuts:
- `g i` → Infrastructure, `g w` → Workloads, `?` → shortcuts help
- `/` or `Cmd/Ctrl+K` → global search

## ✨ Features

### Core Monitoring
- **Unified Monitoring**: View health and metrics for PVE, PBS, PMG, Docker, Kubernetes, and TrueNAS in one place
- **Smart Alerts**: Get notified via Discord, Slack, Telegram, Email, and more
- **Auto-Discovery**: Automatically finds Proxmox nodes on your network
- **Metrics History**: Persistent storage with configurable retention
- **Recovery Central**: Unified backup/snapshot/replication timeline across PBS and TrueNAS

### AI-Powered
- **Chat Assistant (BYOK)**: Ask questions about your infrastructure in natural language
- **Patrol**: Background health checks that generate findings on a schedule. Community self-hosted installs can run Patrol with your own AI provider or a local model.
- **Alert Analysis (Pro / hosted Cloud)**: Optional AI analysis when alerts fire
- **Cost Tracking**: Track usage and costs per provider/model

### Multi-Platform
- **Proxmox VE/PBS/PMG**: Full monitoring and management
- **TrueNAS**: Pools, datasets, disks, ZFS snapshots, replication tasks, and alerts
- **Kubernetes**: Complete K8s cluster monitoring via agents
- **Docker/Podman**: Container and Swarm service monitoring
- **OCI Containers**: Proxmox 9.1+ native container support

### Security & Operations
- **Secure by Design**: Credentials encrypted at rest, strict API scoping, agent commands disabled by default
- **One-Click Updates**: Easy upgrades for supported deployments
- **OIDC/SSO/SAML**: Single sign-on with multi-provider support
- **Mobile Remote Access**: Relay protocol with end-to-end encryption for supported Pulse Mobile clients (Relay and above)
- **Privacy Focused**: Outbound usage telemetry is enabled by default and [fully documented](docs/PRIVACY.md) — the payload uses a rotating pseudonymous install ID and does not include hostnames, credentials, names, email addresses, IP addresses, or infrastructure identifiers. Disable any time in Settings or via `PULSE_TELEMETRY=false`.

## ⚡ Quick Start

> **Paid Pulse Pro / Relay / legacy customers:** GitHub release assets and the
> public `rcourtman/pulse` Docker image are community builds. Activate your
> license key under **Settings → Plans → Existing purchases** to unlock Pro
> features. These community builds do not include the private Pulse Pro runtime hooks
> (Audit Log, Audit Webhooks, RBAC, governed remediation). For those, use
> <https://pulserelay.pro/download.html> with a **v6 activation key** (starts
> with `ppk_live_`) to get the private Pulse Pro Docker image or Linux archive.
> A v5 or legacy license key is not a `ppk_live_` activation key and will not
> work on that page.

### Option 1: Proxmox LXC (Recommended)
Replace `vX.Y.Z` with the exact release tag you want, verify the signed installer, then run it on your Proxmox host:

```bash
export PULSE_VERSION=vX.Y.Z
curl -fsSLO "https://github.com/rcourtman/Pulse/releases/download/${PULSE_VERSION}/install.sh"
curl -fsSLO "https://github.com/rcourtman/Pulse/releases/download/${PULSE_VERSION}/install.sh.sshsig"
ssh-keygen -Y verify \
  -f <(printf '%s\n' 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMZd/DaH+BldzOkq1A8KVTcFk73nAyrE8aJOyf7i00jm pulse-installer') \
  -I pulse-installer \
  -n pulse-install \
  -s install.sh.sshsig < install.sh
bash install.sh --version "${PULSE_VERSION}"
rm -f install.sh install.sh.sshsig
```

Note: this installs the Pulse **server**. Agent installs and v5-to-v6 agent upgrades use the command generated in **Settings → Infrastructure → Install on a host** (served from `/install.sh` on your Pulse server).

### Option 2: Docker
```bash
docker run -d \
  --name pulse \
  -p 7655:7655 \
  -v pulse_data:/data \
  --restart unless-stopped \
  rcourtman/pulse:vX.Y.Z
```

Open Pulse at `http://<your-ip>:7655`.

## Local Development

Use the managed dev runtime from the repo root:

```bash
npm run dev
```

Open `http://127.0.0.1:5173` in the browser. `5173` is the frontend dev shell,
and it proxies `/api` and `/ws` to the backend on `7655`. `7655` is the backend
dependency for API and websocket traffic, not the primary browser URL for local
frontend development.

The managed dev runtime resets its local login to `admin` / `adminadminadmin`
on startup unless you override it with `HOT_DEV_AUTH_USER` and
`HOT_DEV_AUTH_PASS`.

Canonical local dev commands:

- `npm run dev` — start the managed runtime and reclaim the canonical dev ports if an older unmanaged session is still using them
- `npm run dev:lab` — start the managed runtime in lab-agent mode, with the frontend/backend exposed on the LAN and Proxmox LXC Docker inventory enabled for installed lab agents
- `npm run dev:status` — show frontend shell health, proxied API health, direct backend health, and listener ownership
- `npm run dev:status:lab` — show status using the same LAN-bound lab-agent defaults used by `dev:lab`
- `npm run dev:verify` — run the managed browser proof pack against the live dev runtime, including runtime recovery, the Patrol blocked-runtime page contract, and the desktop Recovery layout guard while the launcher suppresses unrelated backend rebuild churn for the duration of the proof pack
- `npm run dev:verify:lab` — run the managed proof pack after applying lab-agent runtime defaults
- `npm run dev:logs` — tail the managed runtime log
- `npm run dev:backend-restart` — bounce only the managed backend through the launcher contract
- `npm run dev:stop` — stop the managed runtime
- `npm run dev:foreground` — run the foreground hot-reload launcher intentionally if you need an attached shell
- `npm run dev:foreground:lab` — run the foreground hot-reload launcher with lab-agent defaults for troubleshooting

If `npm run dev:verify` passes, the managed dev shell, proxy path, backend
health endpoint, browser recovery path, Patrol blocked-runtime page behavior,
and Recovery desktop history-table layout are all aligned.

## 📚 Documentation

- **[Installation Guide](docs/INSTALL.md)**: Detailed instructions for Docker, Kubernetes, and bare metal.
- **[Upgrade to v6](docs/UPGRADE_v6.md)**: Migration guide for upgrading from v5 to v6.
- **[Configuration](docs/CONFIGURATION.md)**: Setup authentication, notifications, and advanced settings.
- **[Security](SECURITY.md)**: Learn about Pulse's security model and best practices.
- **[API Reference](docs/API.md)**: Integrate Pulse with your own tools.
- **[Architecture](ARCHITECTURE.md)**: High-level system design and data flow.
- **[AI Features](docs/AI.md)**: Pulse Assistant (Chat) and Pulse Patrol documentation.
- **[Multi-Tenant](docs/MULTI_TENANT.md)**: Enterprise/internal multi-organization setup and configuration.
- **[Troubleshooting](docs/TROUBLESHOOTING.md)**: Solutions to common issues.
- **[Agent Security](docs/AGENT_SECURITY.md)**: Agent privilege model, Proxmox API-only choices, and checksum/signature verification.
- **[Docker Monitoring](docs/DOCKER.md)**: Setup and management of Docker agents.
- **[Unified Navigation](docs/MIGRATION_UNIFIED_NAV.md)**: Guide to the new task-based navigation.

## 🌐 Community Integrations

Community-maintained integrations and addons:

- **[Home Assistant Addons](https://github.com/Kosztyk/homeassistant-addons)** - Run Pulse Agent and Pulse Server as Home Assistant addons.

## 💳 Plans (Community / Relay / Pro / Cloud)

Pulse is full-featured for core monitoring in every self-hosted tier. Self-hosted
pricing no longer sells more room for monitoring volume; paid value comes from
convenience, history, AI operations, and advanced administration. Cloud remains
the hosted Pulse path. MSP is request-assisted provider hosting, with one
isolated Pulse runtime per client.

Self-hosted tiers:

| Plan | Price | Core monitoring | Metric history | Main value |
|---|---:|---|---:|---|
| Community | Free | Included | 7 days | Full self-hosted monitoring |
| Relay | $39/yr or $4.99/mo | Included | 14 days | Remote web access, mobile app pairing, and push notifications |
| Pro | $79/yr or $8.99/mo | Included | 90 days | Hands-on Patrol modes, issue investigation, verified fixes, and operations tooling |

Pulse still counts top-level monitored systems once no matter how they are
collected. VMs, containers, pods, disks, backups, and other child resources
under that system are included rather than counted separately, but that count is
no longer the self-hosted paid gate.

Community keeps Patrol available with your own provider or local model. Relay
remains the convenience tier, and Pro is the paid operations tier.

Runtime-aligned capability summary:

| Capability | Community | Relay | Pro | Cloud |
|---|:---:|:---:|:---:|:---:|
| Pulse Patrol (Background Health Checks) | ✅ | ✅ | ✅ | ✅ |
| Remote Access / Mobile / Push | — | ✅ | ✅ | ✅ |
| Patrol Investigates Issues | — | — | ✅ | ✅ |
| Patrol Handles Safe Fixes | — | — | ✅ | ✅ |
| Centralized Agent Profiles | — | — | ✅ | ✅ |
| Update Alerts (Container/Package Updates) | ✅ | ✅ | ✅ | ✅ |
| SSO (OIDC/SAML/Multi-Provider) | ✅ | ✅ | ✅ | ✅ |
| Role-Based Access Control (RBAC) | — | — | ✅ | ✅ |
| Enterprise Audit Logging | — | — | ✅ | ✅ |
| Advanced Infrastructure Reporting (PDF/CSV) | — | — | ✅ | ✅ |
| Extended Metric History | 7 days | 14 days | 90 days | 90 days |

Pulse Patrol runs on your schedule (every 10 minutes to every 7 days, default 6 hours) and finds:
- ZFS pools approaching capacity
- Backup jobs that silently failed
- VMs stuck in restart loops
- Clock drift across cluster nodes
- Container health check failures

On self-hosted installs, Pulse Patrol uses the provider you configure from your
Pulse server. That can be a commercial API key or a local model endpoint. Chat
Assistant follows the same self-managed provider model.

Technical highlights:
- Cross-system context (nodes, VMs, backups, containers, and metrics history)
- LLM analysis with your provider plus alert-triggered root-cause investigations (Pro / hosted Cloud)
- Optional safe remediation execution with command safety policies and audit trail
- Centralized agent profiles for consistent fleet settings

**[Try the live demo →](https://demo.pulserelay.pro)** or **[learn more at pulserelay.pro](https://pulserelay.pro)**

Pulse plan technical details: [docs/PULSE_PRO.md](docs/PULSE_PRO.md)

## ❤️ Support Pulse Development

Pulse is maintained by one person. Sponsorships help cover the costs of the demo server, development tools, and domains. If Pulse saves you time, please consider supporting the project!

[![GitHub Sponsors](https://img.shields.io/github/sponsors/rcourtman?label=Sponsor)](https://github.com/sponsors/rcourtman)
[![ko-fi](https://ko-fi.com/img/githubbutton_sm.svg)](https://ko-fi.com/rcourtman)

## 📄 License

MIT © [Richard Courtman](https://github.com/rcourtman). Use of Pulse Pro is subject to the [Terms of Service](TERMS.md).
