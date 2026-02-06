# Pulse Settings Audit (Unified Resource Model)

Date: 2026-02-06
Scope: Frontend settings IA, route model, backend system-settings contracts, and fit with unified resource navigation (`Infrastructure`, `Workloads`, `Storage`, `Backups`).

## Executive Summary

Settings is still organized around platform-era concepts (`Proxmox`, `Docker`, `Agents`) while the rest of the app is now resource-first. This creates cognitive mismatch, route drift, and hidden regressions.

Top issues:
1. Broken settings route used by update flow (`/settings/updates` opens a tab state with no render branch).
2. Security CTA deep-link is broken (`/settings?tab=security` is unsupported and lands on default settings content).
3. Settings IA is not aligned with unified resource pages, which increases user friction and maintenance cost.
4. Settings implementation is a 4,193-line monolith with duplicated per-platform UI blocks.
5. Backend and frontend settings contracts include stale/underexposed fields and duplicate endpoints.

---

## Current State Inventory

### App navigation (resource-first)
- `Infrastructure`, `Workloads`, `Storage`, `Backups` are first-class platform tabs in app shell.
- Source: `frontend-modern/src/App.tsx:1248`, `frontend-modern/src/App.tsx:1250`, `frontend-modern/src/App.tsx:1263`, `frontend-modern/src/App.tsx:1276`, `frontend-modern/src/App.tsx:1289`

### Settings navigation (platform-first)
- Settings sidebar still starts with `Platforms` and items `Proxmox`, `Agents`, `Docker`.
- Source: `frontend-modern/src/components/Settings/Settings.tsx:902`, `frontend-modern/src/components/Settings/Settings.tsx:917`, `frontend-modern/src/components/Settings/Settings.tsx:920`

### Legacy sub-nav inside Settings
- Proxmox tab still splits into `Virtual Environment`, `Backup Server`, `Mail Gateway`.
- Source: `frontend-modern/src/components/Settings/SettingsSectionNav.tsx:15`, `frontend-modern/src/components/Settings/SettingsSectionNav.tsx:22`, `frontend-modern/src/components/Settings/SettingsSectionNav.tsx:27`, `frontend-modern/src/components/Settings/SettingsSectionNav.tsx:32`

---

## Findings (Severity-Ordered)

### P0: Broken route used by update workflow
- `UpdateProgressModal` navigates to `/settings/updates`.
- Source: `frontend-modern/src/App.tsx:214`
- `deriveTabFromPath` maps `/settings/updates` to `updates`.
- Source: `frontend-modern/src/components/Settings/Settings.tsx:454`
- There is header metadata for `updates`, but no sidebar tab and no `activeTab() === 'updates'` render branch.
- Sources: `frontend-modern/src/components/Settings/Settings.tsx:366`, `frontend-modern/src/components/Settings/Settings.tsx:902`
- Impact: clicking “View History” can land users on an effectively empty/unreachable settings state.

### P0: Broken security deep-link from warning banner
- Security warning links to `/settings?tab=security`.
- Source: `frontend-modern/src/components/SecurityWarning.tsx:212`
- Settings routing logic ignores query params and defaults `/settings` to `proxmox`.
- Source: `frontend-modern/src/components/Settings/Settings.tsx:528`
- Impact: high-friction path for a security-critical CTA.

### P1: IA mismatch with unified model
- App shell is resource-first; settings is platform-first.
- Sources: `frontend-modern/src/App.tsx:1248`, `frontend-modern/src/components/Settings/Settings.tsx:917`
- Impact: users must mentally switch models between primary pages and configuration pages.

### P1: Settings monolith + duplicated platform blocks
- `Settings.tsx` is 4,193 lines and mixes routing, data fetching, mutation logic, and modal orchestration.
- Source: `frontend-modern/src/components/Settings/Settings.tsx`
- Nearly identical PVE/PBS/PMG sections are repeated three times.
- Sources: `frontend-modern/src/components/Settings/Settings.tsx:2473`, `frontend-modern/src/components/Settings/Settings.tsx:2763`, `frontend-modern/src/components/Settings/Settings.tsx:3050`
- Impact: change-risk and regression risk increase for every settings update.

### P1: Tab capability metadata is defined but not enforced
- `tabGroups` defines `features` and `permissions` metadata.
- Source: `frontend-modern/src/components/Settings/Settings.tsx:912`
- Sidebar render only respects `disabled`/`badge`; feature/permission checks are not applied.
- Source: `frontend-modern/src/components/Settings/Settings.tsx:2357`
- Impact: IA intent and actual gating diverge.

### P2: Contract drift in system settings model
- Frontend type includes fields like `pbsPollingInterval`, `frontendPort`, `sshPort`.
- Source: `frontend-modern/src/types/config.ts:27`
- Backend model includes even broader fields (`pmgPollingInterval`, adaptive polling, metrics retention, etc.).
- Source: `internal/config/persistence.go:1105`
- UI only surfaces a subset (notably PVE polling + backup polling; no PBS/PMG polling controls).
- Sources: `frontend-modern/src/components/Settings/GeneralSettingsPanel.tsx:146`, `frontend-modern/src/components/Settings/BackupsSettingsPanel.tsx:71`
- Impact: users cannot manage parts of persisted config via UI, and maintainers must reason about partially surfaced state.

### P2: Duplicate system settings endpoints/handlers remain
- Legacy endpoint: `/api/config/system` served by `ConfigHandlers.HandleGetSystemSettings`.
- Source: `internal/api/router.go:546`
- Primary endpoint: `/api/system/settings` served by `SystemSettingsHandler`.
- Source: `internal/api/router.go:1335`
- Legacy handler returns a reduced env override map (only `hideLocalLogin`).
- Source: `internal/api/config_handlers.go:3295`
- New handler returns `h.config.EnvOverrides`.
- Source: `internal/api/system_settings.go:442`
- Impact: dual contracts increase long-term drift risk.

### P2: Reporting resource selector is still workload/infra-biased
- `ResourcePicker` intentionally limits reportable resource types to node/vm/container.
- Source: `frontend-modern/src/components/Settings/ResourcePicker.tsx:81`
- This omits storage/backups resources despite the product’s unified resource model.
- Impact: reporting settings lags the unified surface and user expectations.

---

## Target Settings Information Architecture

Use a resource-aligned first layer, then cross-cutting controls.

### 1) Resource Settings
- Infrastructure
  - Discovery and source connections
  - Host/node onboarding defaults
  - Agent-link behavior
- Workloads
  - Workload polling cadence
  - Workload metadata/discovery behavior
- Storage
  - Storage polling and retention policies
  - Ceph/storage-specific polling controls
- Backups
  - Backup polling cadence and backup data source controls
  - Export/import of configuration

### 2) Integrations
- Providers & connectors (OIDC/SSO, relay, API tokens, webhooks)
- Agent installers and profiles

### 3) Security
- Overview, Auth, SSO, Roles, Users, Audit, Audit Webhooks

### 4) Operations
- Updates, Diagnostics, System Logs, Reporting

### 5) System
- Appearance/UI, network/CORS/embedding, global runtime preferences

---

## Recommended Execution Plan

### Phase 0: Correctness (immediate)
1. Fix `/settings/updates` by either:
   - mapping it to `system-updates`, or
   - implementing a real `updates` view and tab.
2. Fix security CTA route by linking to `/settings/security` (or implement query-param tab parsing).
3. Add test coverage for settings route-to-tab resolution to prevent dead-tab regressions.

### Phase 1: IA shell refactor
1. Replace `Platforms` group with `Resources` group (`Infrastructure`, `Workloads`, `Storage`, `Backups`) in settings.
2. Move platform-specific copy under resource-oriented labels.
3. Keep old routes as aliases with explicit redirects and analytics counters.

### Phase 2: Structural refactor
1. Split `Settings.tsx` into:
   - route shell,
   - section registry,
   - per-section data hooks,
   - shared mutation/state primitives.
2. Extract shared PVE/PBS/PMG list behavior into a generic “connection catalog” component.
3. Normalize save semantics (all immediate or all explicit-save per section).

### Phase 3: Contract cleanup
1. Publish a canonical `SystemSettingsDTO` shared across frontend/backend.
2. Decide field strategy for non-surfaced settings:
   - expose in advanced settings UI, or
   - mark internal-only and remove from frontend type.
3. Deprecate `/api/config/system` once no internal callers remain.

---

## Acceptance Criteria

1. Every settings deep-link resolves to a rendered section.
2. Settings sidebar first group aligns to resource model.
3. No platform-specific route names are user-facing (legacy aliases remain functional).
4. `Settings.tsx` reduced to orchestration only (target: < 1000 lines).
5. Single canonical system-settings endpoint and DTO contract.
6. E2E tests cover all settings routes and key CTA deep-links.

---

## Suggested Next Work Package

If you want me to start implementation now, the highest-value first PR is:
1. Fix route regressions (`/settings/updates`, security CTA).
2. Introduce a settings section registry + route map (no UI redesign yet).
3. Add route conformance tests.

That creates a safe base before the larger IA migration.
