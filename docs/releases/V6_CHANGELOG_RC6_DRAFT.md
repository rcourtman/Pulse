# Pulse v6.0.0-rc.6 Draft Changelog

_Draft only. This changelog describes the current `pulse/v6-release` delta
since the published `v6.0.0-rc.5` tag. Do not treat it as published until
the governed `v6.0.0-rc.6` prerelease exists._

## What `rc.6` changes compared with `rc.5`

`v6.0.0-rc.6` is the frontend-reset RC. It is published on the `rc`
update channel for continuity with the existing prerelease pipe, but
its purpose is closer to "open this for testing under the new
navigation shape" than to "this is days from GA". It does not reopen
the `rc.2` commercial model or replace `v5.1.32` as the stable line.

The headline change is the revert of the unified information
architecture introduced across `rc.1` through `rc.5`. The unified
`/infrastructure`, `/workloads`, `/storage`, and `/recovery` pages,
their aggregate route aliases, and the orphaned summary components,
aggregate state hooks, and presentation utilities that fed them are
removed. The frontend top level is platform-shaped again: Proxmox,
Docker, Kubernetes, TrueNAS, vSphere, Standalone (Machines), plus
Alerts, Patrol, and Settings. The unified resource model and
`/api/resources` contract are retained on the backend; the per-
platform pages consume that contract and add platform-shaped
presentation.

Beyond the IA revert, `rc.6` carries the new vSphere first-class
platform surface, the Standalone-to-Machines surface evolution with
native detail UX, TrueNAS native detail rendering, Proxmox backup
recovery coverage and tab polish, FilterBar adoption with SavedViews,
Patrol capacity-forecast and reliability-finding work, the free-first
self-hosted commercial posture, and `install.sh` release-pipeline
hardening.

## Commit Coverage Audit

The changelog should be audited against every feature/runtime commit
in the exact code-backed release-validation range for the current
candidate:

- `v6.0.0-rc.5`: `<populate at packet finalisation>`
- validation-risk commit: `<populate at packet finalisation>`
- range: `v6.0.0-rc.5..<validation-risk-sha>`
- expected commit count: 600+
- expected changed scope: 1300+ files, 130000+ insertions, 65000+
  deletions

Those commits are grouped in this changelog rather than listed one by
one. The range carries: the frontend information architecture revert
to platform-shaped navigation, the vSphere first-class platform
surface, the Standalone-to-Machines rename and native detail UX, the
TrueNAS native detail rendering pass, Proxmox backup recovery
coverage and tab polish, FilterBar adoption with SavedViews wiring on
Alerts history / audit log / embedded workloads / Storage, Patrol
capacity-forecast action templates and forecast proposals on
RemediationPlan, the PDM HTTP alert bridge and reliability finding
for flapping alerts, the free-first self-hosted commercial posture
with trial and AI quickstart retirement, `install.sh` release-asset
shipping with end-to-end smoke gate and `.sshsig` sidecar requirement,
Windows agent onboarding rework, connection-degraded alerting and
live platform source badges, drawer shell unification across
Workloads / Docker / host detail drawers, and a set of correctness
fixes around workloads runtime/namespace stripping, grouped
notification cancellation, quiet-hours notification replay, and
resolved notifications after direct alert dispatch.

## Major Changes

### 1. Frontend information architecture reverted to platform-shaped navigation

The unified Infrastructure / Workloads / Storage / Recovery pages
introduced across `rc.1`-`rc.5` are removed. The frontend top level is
platform-shaped again:

- Proxmox, Docker, Kubernetes, TrueNAS, vSphere, and Standalone
  (Machines) are each their own top-level page.
- Alerts, Patrol, and Settings keep their own top-level pages.
- Aggregate top-level workspace routes and legacy infrastructure
  route aliases are retired; aggregate route-state path builders are
  removed.
- Orphaned summary components, aggregate state hooks, and
  presentation utilities are deleted as part of the same pass.
- Drawer shells are unified across Workloads, Docker, K8s, and host
  detail drawers so the platform pages share their detail surface.
- Keyboard shortcuts moved to platform keys: `g p` Proxmox, `g d`
  Docker, `g k` Kubernetes, `g n` TrueNAS, `g v` vSphere, `g s`
  Standalone, `g a` Alerts, `g r` Patrol, `g t` Settings.

The unified resource model and `/api/resources` contract are retained
on the backend; the platform-shaped pages consume that contract and
apply platform-shaped presentation on top.

This is the reason `rc.6` exists. Across `rc.1`-`rc.5` the feedback on
the unified IA was consistently in favour of the platform-shaped
navigation v5 already had, with no countervailing signal in favour of
the unified shape. `rc.6` realigns the frontend to the navigation
operators already had a mental model for, while keeping the v6
backend.

### 2. vSphere as a first-class platform

vSphere is a top-level platform in `rc.6`, parallel to Proxmox, Docker,
Kubernetes, and TrueNAS:

- vSphere VMs route through the shared Workloads pipeline (the legacy
  `VsphereVirtualMachinesTable` is deleted).
- vSphere networks inventory.
- vSphere Hosts table with version and uptime columns.
- vSphere VM uptime and guest disk usage.
- vSphere cluster services.
- vSphere VM hardware config.
- VMware Tools status carried into the workload row.
- vCenter MoRef shown in the workload ID column.
- vSphere snapshot trees.
- vSphere placement card in the workload drawer.
- vSphere tables follow the canonical platform-table column-alignment
  helper and the shared platform conventions used by Proxmox and
  Docker.
- Guest OS column lights up for Proxmox and vSphere workloads in
  unison.

### 3. Standalone surface renamed to Machines with native detail UX

The surface previously labelled "Agents" and then "Standalone" is now
"Machines". The IA decision and landing behaviour are consolidated
into one name and the surface is restricted to Pulse Agent resources:

- Row identity context.
- Row expansion affordance.
- IP, disk I/O, RAID, network, and temperature detail tooltips.
- Aggregate disk summary in machine details.
- SMART temperature fallback.
- Richer agent telemetry preserved in the Machines table.
- Machine discovery promoted into drawer tabs.
- Machine facts promoted in drawer rows.

### 4. TrueNAS native detail rendering

TrueNAS gained native inline detail rendering across the row types:

- Native storage details, system details, service detail rows,
  protection detail rows.
- Shared TrueNAS detail table extraction (single rendering path).
- TrueNAS health alerts on the overview.
- TrueNAS alert detail rows in the drawer.

### 5. Proxmox backup recovery coverage and tab polish

The three Proxmox Backups tabs got:

- Click-to-sort across all three tabs.
- Visual density alignment with the Storage and Ceph pages.
- A consistency pass on sub-page conventions.
- Canonical `ProgressBar` for metric bars.
- A coverage view that surfaces which workloads have recent PBS
  artifacts.
- Backup-age display on workload rows.
- A Backups column hidden on non-Proxmox workload surfaces.
- The Replication tab is hidden when no replication signals exist.
- A backup artifact surface fix on the PBS path.
- A simplification pass on Proxmox backup recovery navigation.

### 6. FilterBar adoption with SavedViews

The canonical FilterBar pattern (chips plus `+ Filter`) expanded:

- Alerts history converted to FilterBar with URL-backed filter state.
- Audit log filter form converted to FilterBar with SavedViews.
- Embedded workloads filter wired to `savedViewsKey`.
- Storage wired to SavedViews with a platform-scoped key.
- SavedViews default-star always visible.
- Workloads search and `statusMode` moved to URL params; the
  localStorage backup for `viewMode` and `containerRuntime` is
  dropped. URL state is now the single source of truth.
- Audit filters moved to URL with live-apply.

### 7. Patrol intelligence additions

Patrol gained:

- Capacity-forecast action template registry.
- Forecast proposals attached onto Patrol `RemediationPlan`.
- Reliability finding emitted when an alert starts flapping.
- PDM (Proxmox Datacenter Manager) HTTP alert source and bridge,
  emitting and resolving through `FindingsStore.Add`.
- Verification-outcome and capability-postcondition substrate for
  finding lifecycle.
- A connection-degraded alert for wedged platform connections.
- Patrol approval-section assertions and shipped CONFIGURATION docs
  synced after the changes landed.

A storage-growth-planner runway widget was prototyped and removed in
the same range; it is not present in `rc.6`.

### 8. Self-hosted commercial posture: free-first

Self-hosted commercial framing moved to free-first:

- Self-hosted trial start route, trial signup control plane, trial
  activation callback, and trial-expired purchase handoff retired.
- Self-hosted AI quickstart surfaces and hosted AI quickstart runtime
  retired.
- Hosted quickstart backend retired (the inactive Pro upsell helper
  and self-hosted Pro upsell copy are gone).
- Self-hosted guest capacity caps removed; self-hosted Pro continuity
  holds with no caps. A self-hosted commercial continuity proof is
  shipped.
- Monitored-system handoffs routed to usage review.
- Self-hosted Pro prompts remain opt-in; trial starts are kept out of
  feature gates.
- Pulse Pro v6 value copy aligned to the new posture.
- Stripped self-hosted monitoring upsell copy and stale Relay
  onboarding price copy.
- A relationship-first monitoring v2 prototype with platform-layer
  filters is in the range; it is exploratory and not the shipped
  product surface in `rc.6`.

### 9. Install pipeline hardening

The release pipeline now treats `install.sh` as a release-blocking
artifact:

- `install.sh` is shipped as a GitHub Release asset alongside the
  binaries.
- An end-to-end `install.sh` smoke gate runs against the published
  release on every workflow edit and on every create-release run.
- Archive install path requires a `.sshsig` sidecar.
- Installer extraction is hardened.
- Manual systemd install snippet binary path fixed.
- Windows agent onboarding moved to a seamless install flow with a
  corrected installer readiness path.
- Proxmox install command tokens bound on first use.
- `publish-helm-chart` triggered via `workflow_call` from
  `create-release`.
- Helm chart `agent.enabled` routed through the main pulse image.

### 10. Connection identity, source badges, and notifications

- Platform source badge is live and de-duplicates cluster members.
- Connection-degraded alert for wedged platform connections.
- Platform identity badges clarified on infrastructure system rows.
- `/api/state` and `/api/diagnostics` concurrent reads deduplicated
  through a singleflight gate.
- Monitor reload skipped on no-op auto-register.
- Auto-register refresh notifications fixed.
- Grouped notification cancellation fixed.
- Mixed quiet-hours notification replay queueing fixed; quiet-hours
  alert notifications now replay.
- Resolved notifications after direct alert dispatch fixed.

### 11. Performance and correctness fixes

- Workload charts use a TTL cache and skip redundant clones.
- Workloads `runtime` and `namespace` no longer stripped before guest
  data loads.
- Table-only workload filtered empty state fixed.
- vSphere workload-type toolbar fix.
- Kubernetes nodes table reworked with Kubelet, runtime, and capacity
  columns.
- Ceph drawer capacity bars use canonical metric color tokens.

### 12. Documentation and accessibility

- Notification and settings switches labelled.
- Alert schedule toggles labelled by section.
- Alert schedule and notification form labels bound.
- Patrol configuration toggles labelled.
- Four customer-facing doc drift findings fixed (RBAC, OIDC, helm,
  webhooks).
- LLM markdown renderer DOMPurify config tightened.

## Validation

This range should be re-validated against the GitHub release pipeline
before `rc.6` is published:

- Release artifact download, checksum, and signature verification.
- `install.sh` end-to-end smoke gate against the published release.
- Helm chart publication path.
- Frontend route inventory (no `/infrastructure`, `/workloads`,
  `/storage`, `/recovery` top-level routes; per-platform routes
  present).
- API route inventory (`/api/resources`, `/api/recovery/*` still
  served).
- Keyboard shortcut surface matches `useKeyboardShortcuts.ts`
  platform-keyed shape.
- Subsystem proofs across api-contracts, unified-resources,
  monitoring, patrol-intelligence, and ai-runtime.

## Evidence Appendix

- `docs/release-control/v6/internal/subsystems/api-contracts.json`
- `docs/release-control/v6/internal/subsystems/unified-resources.json`
- `docs/release-control/v6/internal/subsystems/monitoring.json`
- `docs/release-control/v6/internal/subsystems/patrol-intelligence.json`
- `docs/release-control/v6/internal/subsystems/ai-runtime.json`
- `frontend-modern/src/App.tsx` (top-level route shape)
- `frontend-modern/src/hooks/useKeyboardShortcuts.ts`
- `frontend-modern/src/features/platformPage/columnAlignment.ts`
- `frontend-modern/src/features/platformPage/sharedPlatformPage.tsx`
