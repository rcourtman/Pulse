# Pulse v6.0.0 Release Notes (Draft)

This release is the Unified Resources + unified navigation foundation, plus opt-in multi-tenant and hosted-mode building blocks.

## New Features

### Unified Resources + Unified Navigation

- Unified Resources model (server + API + UI) with a single resource registry across Proxmox, Docker, Kubernetes, PBS/PMG, and agent sources.
- Unified navigation redesign organized by task (Infrastructure, Workloads, Storage, Backups) with global search and keyboard shortcuts.
- Canonical Unified Resources API endpoints for resource listing, stats, and resource-level metrics.

### Dashboard Redesign

- High-density dashboard layout with sparklines and trend delta badges.
- Alerts and findings highlights panels.
- Prioritized action queue to surface what changed and what needs attention.

### Storage + Backups (Unified Views)

- Storage and Backups shipped as unified, source-agnostic views (formerly the "V2" architecture).
- Full pools/disks views, filtering, and enriched storage metadata.
- Ceph and physical disk resources integrated into the unified registry.

### TrueNAS Integration

- TrueNAS provider scaffold, REST client, and fixture-first testing.
- Encrypted persistence for TrueNAS connection config plus add/list/delete/test endpoints.
- Runtime TrueNAS poller with periodic polling and dynamic connection syncing.
- ZFS health mapping, tags, UI source badges, and AI context enrichment for TrueNAS-backed infrastructure.

### Relay + Mobile Remote Access

- Relay encryption and instance identity keys, plus Settings UI wiring.
- Push notification pipeline (client-side) and Patrol-to-relay notification wiring (best-effort).

### Centralized Agent Profiles (Pro)

- Centralized agent configuration via Agent Profiles (create/edit/assign/deploy) with API gating and UI panels.

### Organizations + Multi-Tenant (Enterprise, Opt-In)

- Organizations: CRUD, member roles, and cross-org resource sharing.
- Tenant-aware isolation for state/monitoring, WebSocket broadcasts, persistence, audit scope, and API authorization.
- Tenant-aware rate limiting and safety-focused tenant lifecycle plumbing (reaper/cleanup cascade).

### Hosted Mode (Opt-In)

- Passwordless magic-link signup/login flows (hosted mode only).
- Stripe webhook handlers with signature verification and org provisioning.
- Trial subscription state end-to-end, plus server-driven entitlements and frontend gating cutover.
- Conversion telemetry and paywall/trial UX (pricing page, trial CTAs, conversion tracking).

### Audit Logging + RBAC Operations

- Per-tenant SQLite audit logging with license gating for access/query/export.
- RBAC integrity verification and operational endpoints for admin recovery workflows, plus Prometheus metrics for RBAC manager health.

## Improvements

### Performance and UI Responsiveness

- Row windowing and selector pipeline refactors for Infrastructure and Workloads to contain render costs on large inventories.
- Hook-level caching and request de-duplication for unified resources.
- Charts endpoint optimization to fetch only sparkline-relevant metrics.

### Monitoring, Metrics, and Alerting Quality

- Metrics history normalization fixes across resource types (including Docker) and storage metric mapping corrections.
- Automatic refresh for changed/stale service discovery resources.
- Reduced Swarm alert noise and better preservation of Docker-related alert state across host identity churn.
- Temperature sensor parsing fixes to avoid silently zeroing readings on string-valued sensors.

### Platform and Agent Refinements

- Proxmox permission setup hardening, fewer false failures during apt update checks, and better behavior on empty polls.
- Agent improvements: FreeBSD S.M.A.R.T. disk collection support and correct honoring of `--disk-exclude` for Docker disk metrics.

### Release and Build Hygiene

- Update-check/UI version badge correctness improvements (including cached update paths).
- Go toolchain pinned for reproducible builds and vulnerability-baseline alignment.

## Breaking Changes

- Unified Resources is now the canonical resource model.
  - Canonical API is now `/api/resources`.
  - Deprecated alias: `/api/v2/resources` (temporary compatibility shim for older clients; will be removed after the migration window).
- Frontend routing is now canonicalized around unified navigation.
  - Legacy routes (for example `/services` and `/kubernetes`) are transitional aliases and will be removed after the migration window.
- Storage/Backups naming and routing were de-V2'ed.
  - Internal and UI "V2" suffixes were removed; use canonical `/storage` and `/backups` routes.

## Migration Notes

### Unified Navigation (Bookmarks and Deep Links)

- Update bookmarks, documentation, and runbooks to canonical unified routes.
- Reference: `docs/MIGRATION_UNIFIED_NAV.md`.
- To disable all legacy route redirects (and surface stale bookmarks immediately), set `PULSE_DISABLE_LEGACY_ROUTE_REDIRECTS=true` (or `disableLegacyRouteRedirects: true` in `system.json`).

### API Clients and Integrations

- Replace calls to `/api/v2/resources` with `/api/resources` (recommended).
- If an integration depended on legacy resource arrays or legacy resource endpoints, migrate to unified resource types and unified resource IDs.

### Multi-Tenant Organizations (Enterprise, Opt-In)

- Enablement requires both `PULSE_MULTI_TENANT_ENABLED=true` and an Enterprise license with the `multi_tenant` capability.
- Reference: `docs/MULTI_TENANT.md` for behavior, headers/cookies for org selection, and the storage layout/migration model.

### Hosted Mode (Opt-In)

- Hosted signup/login uses passwordless magic links.
- Stripe provisioning/trials/entitlements endpoints are only relevant when hosted mode is enabled.
