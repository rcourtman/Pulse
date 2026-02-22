# Pulse v6.0.0 Release Notes

Pulse v6 is the largest release in the project's history — 1,200+ commits touching 1,490 files across the Go backend and SolidJS frontend. It introduces a unified resource model, a completely redesigned navigation and dashboard, mobile remote access via Pulse Relay, TrueNAS integration, a full entitlement and licensing framework, and opt-in multi-tenant and hosted-mode building blocks.

---

## New Features

### Unified Resources + Unified Navigation

The core architectural change in v6: every monitored resource — Proxmox VMs, containers, nodes, Docker hosts, Kubernetes pods, PBS/PMG services, host agents, Ceph pools, physical disks, and TrueNAS systems — is now represented in a single unified resource registry.

**Backend:**
- Single `ResourceHandlers` serving the canonical `/api/resources` endpoint with per-tenant state providers, supplemental records, and registry caching.
- Expanded `Resource` type with source-specific payloads (Proxmox, Storage, Agent, Docker, PBS, PMG, Kubernetes, PhysicalDisk, Ceph), multi-source tracking, and identity deduplication.
- API router decomposed from a monolith into focused route-group files: monitoring, AI/relay, hosted, org/license, auth/security, and registration.

**Frontend:**
- Navigation reorganized from platform tabs (Proxmox | Docker | Kubernetes | Hosts) to task-oriented tabs: **Dashboard | Infrastructure | Workloads | Storage | Recovery**.
- New **Infrastructure page** at `/infrastructure` — all hosts and nodes across all platforms in one unified table. Filter by source (PVE, Agent, Docker, PBS, PMG, K8s, TrueNAS), filter by status, search across name/ID/IP/tags, toggle between grouped and flat views.
- **Resource detail drawer** with hardware cards, disk info, temperature readings, network interfaces, and platform-specific metadata.
- **Command palette** (Cmd+K / Ctrl+K) with fuzzy-search navigation to any page, resource, or action.
- **Keyboard shortcuts** — press `?` to see the reference. Vim-style `g` then key navigation: `g+i` Infrastructure, `g+w` Workloads, `g+s` Storage, `g+b` Recovery, `g+a` Alerts, `g+t` Settings. `/` focuses search.
- **Mobile bottom navigation bar** — fixed bottom nav for mobile and tablet with horizontally scrollable tabs and alert badge counts. Hidden on desktop.
- Legacy routes (`/proxmox/overview`, `/docker`, `/kubernetes`, `/hosts`, `/services`, `/mail`) redirect to their new destinations with toast notifications explaining the move. See Migration Notes below.
- **"What's New" modal** on first visit after upgrade, explaining the navigation changes with a link to the migration guide.
- **In-app migration guide** at `/migration-guide` showing the complete old-to-new route mapping.
- Optional migration aid: **Classic platform shortcuts** bar in the main navigation (can be hidden in Settings → System → General).

### Dashboard Redesign

A completely new environment-overview dashboard replaces the previous landing page.

- **Status hero card** with health indicators (critical / warning / healthy counts), mini donut charts, and gauge visualizations for CPU and memory.
- **Trend charts** showing per-host CPU and memory sparklines over configurable time ranges with interactive hover tooltips.
- **Prioritized action items** — a ranked queue surfacing critical alerts, offline infrastructure, storage capacity warnings, and high-CPU hosts, ordered by severity (critical > high > medium > low).
- **Composition panel** breaking down infrastructure and workload mix by platform and type.
- **Backup status panel** with health overview.
- **Storage panel** with capacity at-a-glance.
- **Recent alerts panel** with severity counts.
- **Relay onboarding card** prompting mobile device pairing (Pro feature, with trial CTA for Community users).

### Storage + Recovery (Unified Views)

Storage and Recovery are now unified, source-agnostic views — the "V2" naming has been removed.

- **Storage hero section** with pool count donut, capacity gauge, disk count, and free space KPIs.
- Per-pool detail expansion with live disk metrics.
- Source filter options for both Storage and Recovery views (filter by Proxmox, PBS, Ceph, Agent, etc.).
- Ceph and physical disk resources integrated into the unified registry.
- Storage sparklines for usage trends.
- Alert state integration for storage resources.

### TrueNAS Integration

First-class TrueNAS storage system monitoring, enabled via `PULSE_ENABLE_TRUENAS`.

- **TrueNAS API client** for fetching system info, pools, datasets, disks, alerts, and snapshots.
- **Provider architecture** supporting both live API fetching and fixture-based testing.
- **Runtime poller** integrated into the monitoring loop with periodic polling and dynamic connection syncing.
- **Encrypted persistence** for TrueNAS connection config with add/list/delete/test endpoints.
- **ZFS health mapping**, source badges, and AI context enrichment for TrueNAS-backed infrastructure.
- TrueNAS resources appear as a first-class source in the unified Infrastructure page alongside PVE, Docker, K8s, and others.

### Relay + Mobile Remote Access

Relay infrastructure and protocol support for mobile connectivity to Pulse instances without requiring direct network access. Public mobile app rollout is staged separately.

- **Binary frame protocol** (v1) with 13 frame types: Register, Connect, ChannelOpen/Close, Data, Ping/Pong, Error, Drain, KeyExchange, PushNotification.
- **End-to-end encryption** using X25519 key exchange, HKDF key derivation, AES-256-GCM with per-direction nonce counters and replay protection.
- **HTTP proxy** translating relay DATA frames into local API calls, with SSE streaming support for real-time data.
- **Push notification pipeline** — patrol findings, approval requests, and fix completions relayed as push notifications with infrastructure detail redaction (IPs and hostnames stripped for Apple/Google visibility).
- **Instance identity and key management** for persistent relay registration.
- **Settings UI** — Relay configuration panel with connection status indicator, enable/disable toggle, server URL config, instance fingerprint display, and QR code pairing for mobile devices.
- WebSocket relay client with reconnect backoff, channel state management, and concurrent data handler limiting.

### Centralized Agent Profiles (Pro)

- CRUD interface for agent configuration profiles — create, edit, assign to agents, deploy.
- Known settings editor with profile assignment to connected agents.
- AI-powered profile suggestion modal that analyzes the current environment and recommends optimal settings.
- API gating (Pro license required) with in-app paywall and trial CTA.

### Entitlements + Licensing Overhaul

The license system was rebuilt from simple tier checks into a full entitlement framework.

- **`/api/license/entitlements` endpoint** — the canonical way for the frontend and API clients to determine feature availability. Returns capabilities, quantitative limits with usage state (ok/warning/enforced), subscription state, upgrade reasons, trial info, and hosted mode flag.
- **Subscription state machine** with lifecycle states: `trial`, `active`, `grace`, `expired`, `suspended`, `canceled`. Each state defines full/degraded/locked operation behavior.
- **Pluggable entitlement sources** — JWT-based for self-hosted deployments, database-backed for hosted/SaaS.
- **Local trial lifecycle** — trial state managed locally via billing state files, no phone-home required. Trial countdown with `trial_expires_at` and `trial_days_remaining`.
- **Quantitative limit enforcement** for nodes and users, with warning and hard-block thresholds.
- **Capability alias and deprecation framework** — legacy feature keys are transparently mapped to canonical replacements with deprecation warnings.
- **Certificate Revocation List** cache with fail-open semantics and 72-hour staleness TTL.
- **Local upgrade metrics** — local-only event recording (paywall_viewed, trial_started, upgrade_clicked, limit_warning_shown, limit_blocked, agent_install_*, agent_first_connected) with an admin funnel API and Prometheus metrics. No third-party export. Disable via Settings → System → General → "Disable local upgrade metrics" or `PULSE_DISABLE_LOCAL_UPGRADE_METRICS=true`.
- **Usage metering** — windowed aggregator with per-tenant cardinality limits and idempotency dedup.
- **Upgrade reason engine** — generates actionable upgrade prompts based on current usage patterns.
- **In-app pricing page** at `/pricing` with Community/Pro/Cloud tier comparison table, feature matrix, and "Start Free 14-day Trial" CTA.
- **Trial banner** — global countdown banner with urgency coloring (blue > 3 days, amber 1–3 days, red < 1 day) and upgrade link.

### Organizations + Multi-Tenant (Enterprise, Opt-In)

Full multi-tenant organization support, disabled by default.

- **Organization CRUD** — create, get, update, delete organizations with display names and metadata.
- **Member management** — invite, remove, and role assignment (owner, admin, tech, read_only).
- **Cross-org resource sharing** — share VMs, containers, hosts, and storage between organizations with viewer/editor/admin access roles.
- **Tenant-aware isolation** across RBAC, state/monitoring, WebSocket broadcast hubs, persistence, audit scope, API authorization, and rate limiting.
- **Per-tenant RBAC managers** with lazy initialization and file-based isolation; integrity verification and admin recovery endpoints.
- **Org lifecycle operations** — suspend, unsuspend, soft-delete.
- **Background reaper** for expired/deleted organizations with configurable retention and dry-run mode.
- **Frontend org switcher** in the header — switch between organizations, with WebSocket reconnection and cache clearing on switch.
- **Settings panels** — Organization Overview (metadata, members), Access Control, Billing, and Resource Sharing.
- **Prometheus metrics** for RBAC operations and hosted mode.

### Hosted Mode (Opt-In)

Infrastructure for running Pulse as a hosted SaaS product — entirely opt-in.

- **Cloud Control Plane** — new `pulse-control-plane` binary managing tenant lifecycle, Docker containers, billing, and health monitoring.
- **Tenant registry** with lifecycle states (provisioning, active, suspended, canceled, deleting, deleted, failed) and Stripe account mapping.
- **Passwordless magic-link signup/login** with rate limiting (3/hour), 15-minute TTL, SQLite persistence, and URL redaction in logs.
- **Control plane → tenant auth handoff** using per-tenant HMAC keys.
- **Stripe integration** — webhook handlers with signature verification, subscription provisioning, grace period enforcement, and a background reconciler checking billing state drift every 6 hours.
- **Billing admin panel** — admin table showing all organizations with subscription state, trial status, Stripe customer ID, and suspend/activate actions.
- **QR code mobile onboarding** with relay details, deep links, and connection validation diagnostics.
- **Cloud deployment stack** — Docker Compose, Traefik config, rollout scripts, backup scripts, and operational runbook.

### Audit Logging + RBAC Operations

- Per-tenant SQLite audit logging with license gating for access, query, and export.
- RBAC integrity verification and operational endpoints for admin recovery workflows.
- Prometheus metrics for RBAC manager health.
- Audit log utilities for client IP and actor ID extraction.

---

## Improvements

### AI + Patrol Enhancements

- **Explore prepass** — a fast agentic pre-analysis pass using a lightweight model before the main chat response, with configurable turn limits and timeouts.
- **Investigation budget** for community/free-tier users — monthly investigation allowance with auto-reset, preventing runaway AI costs.
- **LLM-powered infrastructure discovery** — detects applications and services from Docker containers with confidence scoring, category classification, and CLI access paths.
- **Sensitive data redaction** from AI tool outputs — PEM blocks, passwords, tokens, AWS keys, JWTs, and bearer tokens are automatically stripped.
- **Sensitive file path detection** to prevent AI from reading credential files.
- **Fix verification improvements** with inconclusive outcome handling.
- **Agentic loop decomposition** — the monolithic chat loop was broken into focused modules for context building, control flow, intent detection, prompt construction, recovery, summaries, tool choice, verification, and wrap-up.
- **First-class OpenRouter provider support** for more model choices.

### Performance and UI Responsiveness

- **Virtual table windowing** for Infrastructure and Dashboard — large resource lists render only visible rows, dramatically reducing DOM cost.
- **Infrastructure chart prewarming** — chart data is preloaded during idle time (`requestIdleCallback`) so the Infrastructure page loads instantly. Respects `navigator.connection.saveData` and slow-connection detection.
- **Infrastructure summary caching** with prewarm support.
- Hook-level caching and request de-duplication for unified resources.
- Charts endpoint optimization to fetch only sparkline-relevant metrics.
- Replaced `metricsSampler` with a unified metrics collector store.

### Monitoring, Metrics, and Alerting Quality

- Metrics history normalization fixes across resource types (including Docker) and storage metric mapping corrections.
- Automatic refresh for changed/stale service discovery resources.
- Reduced Swarm alert noise and better preservation of Docker-related alert state across host identity churn.
- Temperature sensor parsing fixes to avoid silently zeroing readings on string-valued sensors.

### Platform and Agent Refinements

- **Docker/Kubernetes init retry with exponential backoff** — agents now start successfully even if Docker or Kubernetes is temporarily unavailable, and connect later when the runtime becomes available. Backoff pattern: 5s → 10s → 20s → 40s → 80s → 160s → capped at 300s.
- **Agent commands disabled by default** for security — the `--disable-commands` flag is now deprecated (commands are off by default). Users who want AI auto-fix must explicitly set `--enable-commands` or `PULSE_ENABLE_COMMANDS=true`.
- Proxmox permission setup hardening, fewer false failures during apt update checks, and better behavior on empty polls.
- FreeBSD S.M.A.R.T. disk collection support.
- Correct honoring of `--disk-exclude` for Docker disk metrics.
- Config validation hardening, graceful shutdown improvements, `--disable-ceph` flag propagation.
- Windows service shutdown flow hardening.
- **Encrypted configuration export/import** — export and import full config with passphrase-based encryption (minimum 12 characters).

### Settings Architecture

- Settings page restructured with a panel registry/routing system for dynamic layout and feature-gated panels.
- New settings panels: Relay, Organization Overview, Organization Access, Organization Billing, Organization Sharing, Billing Admin.

### New Shared UI Components

- **`InteractiveSparkline`** — multi-series interactive sparkline chart with hover tooltips, LTTB downsampling, and canvas/SVG rendering modes.
- **`Dialog`** — accessible modal with focus trapping, body scroll locking, and escape handling.
- **`ScrollToTopButton`** — floating "back to top" button that auto-discovers its scrollable ancestor.
- **`SearchInput` / `CollapsibleSearchInput`** — search inputs with history support.
- Hardware detail cards: Disks, Hardware, Network Interfaces, Root Disk, System Info, Temperatures.
- Platform and workload type badge systems.

### Code Quality and Refactoring

- Massive lint cleanup: 200+ `errcheck` and `dupl` fixes across all backend packages.
- Duplicate function consolidation across 20+ packages (crypto helpers, panic recovery, threshold normalization, polling interval parsing, etc.).
- Package consolidation: `internal/types` → `internal/models`, `internal/smartctl` → `internal/hostagent`, `internal/mdadm` → `internal/hostagent`, `internal/ceph` → `internal/hostagent`, `internal/health` → `internal/cloudcp`, `internal/errors` → `internal/monitoring`, `internal/buffer` → `internal/utils`, `internal/agentbinaries` → `internal/updates`.
- Logging consistency standardization across 30+ packages.
- Naming consistency fixes (acronym spelling, identifier clarity).
- Unused import and dead dependency removal.
- `jscpd` copy/paste detection (3% threshold) and `dupl` linter (150-token threshold) enforced in CI.

### Release and Build Hygiene

- **Go toolchain bumped to Go 1.25** for reproducible builds and vulnerability-baseline alignment.
- New dependencies: `golang-jwt/jwt/v5` for license/auth JWT handling, `stripe-go/v82` for billing integration.
- Update-check/UI version badge correctness improvements (including cached update paths).
- GitHub automation: issue templates, version triage workflow, stale-issue cleanup workflow.
- Integration tests: navigation performance, multi-tenant end-to-end.

---

## Breaking Changes

### API

- **Unified Resources is now the canonical resource model.**
  - Canonical API: `/api/resources`, `/api/resources/stats`, `/api/resources/{id}`.
  - Deprecated alias: `/api/v2/resources` — returns a `Deprecation: true` HTTP header. This shim exists for compatibility; migrate to `/api/resources`.
- **New `/api/license/entitlements` endpoint** is the canonical way to determine feature availability. Replaces inferring capabilities from tier names.

### Frontend Routing

- **Navigation restructured around unified resources.**
  - Legacy routes redirect with toast notifications:
    | Legacy Route | Redirects To |
    |---|---|
    | `/proxmox/overview` | `/infrastructure` |
    | `/hosts` | `/infrastructure?source=agent` |
    | `/docker` | `/infrastructure?source=docker` |
    | `/proxmox/mail`, `/mail`, `/services` | `/infrastructure?source=pmg` |
    | `/kubernetes` | `/workloads?type=k8s` |
  - These redirects exist as compatibility aliases; update bookmarks to canonical routes.
- **Storage/Recovery "V2" naming removed** — use canonical `/storage` and `/recovery` routes (`/backups` remains as a compatibility alias).

### Agent

- **Commands are now disabled by default.** The `--disable-commands` flag and `PULSE_DISABLE_COMMANDS` env var are deprecated (since the new default is already disabled). To enable AI auto-fix, explicitly set `--enable-commands` or `PULSE_ENABLE_COMMANDS=true`.

### Deprecated Environment Variables

| Deprecated | Replacement |
|---|---|
| `BACKEND_HOST` | `BIND_ADDRESS` |
| `POLLING_INTERVAL` in `.env` | Per-service intervals in `system.json` (`PVE_POLLING_INTERVAL`, `BACKUP_POLLING_INTERVAL`, etc.) |
| `API_TOKEN` / `API_TOKENS` in `.env` | UI-managed tokens in `api_tokens.json` (`.env` values ignored when `api_tokens.json` exists) |
| `DISABLE_AUTH` | Removed — stripped at runtime with a warning |
| `--disable-commands` / `PULSE_DISABLE_COMMANDS` (agent) | `--enable-commands` / `PULSE_ENABLE_COMMANDS` |
| `GroupingWindow` (alerts config) | `Grouping.Window` (auto-migrated on config update) |
| `QuickCheckInterval` (patrol config) | Kept for backwards compatibility but no longer preferred |

### Build Requirements

- **Go 1.25+ required** for building from source (up from Go 1.24).

---

## Migration Notes

### Before Upgrading

1. **Create an encrypted config backup** via Settings → System → Recovery → Create Backup (older versions labeled this **Backups**).
2. Confirm console access for rollback and bootstrap token retrieval.
3. Review API changes if you have external integrations.

### Unified Navigation (Bookmarks and Deep Links)

- Update bookmarks, documentation, and runbooks to canonical unified routes.
- Reference: `docs/MIGRATION_UNIFIED_NAV.md` for the complete old-to-new mapping.
- To disable all legacy route redirects (and surface stale bookmarks immediately), set `PULSE_DISABLE_LEGACY_ROUTE_REDIRECTS=true` (or `disableLegacyRouteRedirects: true` in `system.json`).

### API Clients and Integrations

- Replace calls to `/api/v2/resources` with `/api/resources` (recommended).
- If an integration depended on legacy resource arrays or legacy resource endpoints, migrate to unified resource types and unified resource IDs.
- Use the new `/api/license/entitlements` endpoint for feature availability checks instead of inferring from tier names.

### Agent Updates

- If you were using `--disable-commands`, remove that flag — it is now the default behavior.
- If you need AI auto-fix, switch to `--enable-commands` or set `PULSE_ENABLE_COMMANDS=true`.

### Post-Upgrade Checklist

1. Confirm version: `GET /api/version`
2. Confirm scheduler health: `GET /api/monitoring/scheduler/health`
3. Confirm unified resources API: `GET /api/resources`
4. Confirm nodes are polling and no circuit breakers are stuck open.
5. Confirm notifications still send (send a test).
6. Confirm agents are connected (if used).
7. Update bookmarks and automation to canonical routes (`/infrastructure`, `/workloads`, `/storage`, `/recovery`).
8. Migrate any scripts calling `/api/v2/resources` to `/api/resources`.

### Multi-Tenant Organizations (Enterprise, Opt-In)

- Disabled by default — requires both `PULSE_MULTI_TENANT_ENABLED=true` and an Enterprise license with the `multi_tenant` capability.
- Without enablement: non-default org requests get `501 Not Implemented`.
- Without license: non-default org requests get `402 Payment Required`.
- The `default` organization always works regardless of feature flag.
- Tenant selection via `X-Pulse-Org-ID` header, `pulse_org_id` cookie, or fallback to `"default"`.
- Legacy single-tenant data is migrated into `/orgs/default/` with symlinks for compatibility.
- Reference: `docs/MULTI_TENANT.md`.

### Hosted Mode (Opt-In)

- Hosted signup/login uses passwordless magic links.
- Stripe provisioning/trials/entitlements endpoints are only relevant when hosted mode is enabled.

### Upgrade Paths

- **Bare metal / LXC:** Installer script or Settings UI in-app update.
- **Docker:** `docker pull` + `docker compose up -d`.
- **Helm:** `helm upgrade`.
