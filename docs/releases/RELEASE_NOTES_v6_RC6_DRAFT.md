# Pulse v6.0.0-rc.6 Draft Release Notes

_Draft only. Do not treat this as published until the governed
`v6.0.0-rc.6` tag and GitHub prerelease exist._

## What this RC is, and what it is not

`v6.0.0-rc.6` is a pre-release for testing, not a candidate for general
availability. It is published on the existing `rc` channel because the
prerelease update path (`internal/updates/version.go`,
`internal/config/config.go`) is RC-shaped, but the framing is closer to
"come kick the tyres" than "this is days from GA". Pulse v5.1.32 remains
the current stable line.

## Why the frontend looks like v5 again

`rc.1` through `rc.5` introduced a unified information architecture for
v6: `/infrastructure`, `/workloads`, `/storage`, and `/recovery` as
top-level pages, with platform names demoted to filters and source
badges inside those views. Across five prereleases, the feedback I
received was consistent: existing users preferred the platform-shaped
navigation from v5. I did not receive feedback that the unified IA was
working better than the per-platform pages it replaced.

With that signal pointing in one direction and no countervailing signal,
I reverted the frontend information architecture in `rc.6` to
platform-shaped top-level navigation:

- Proxmox, Docker, Kubernetes, TrueNAS, vSphere, and Standalone are
  each their own top-level page again.
- Alerts, Patrol, and Settings keep their own top-level pages.
- The unified resource model is still the backend contract.
  `/api/resources` remains the canonical read; the platform pages
  consume it and apply platform-shaped presentation on top.

In short: v5 navigation, v6 data model. Same backend, the navigation
shape you already know.

The keyboard shortcuts follow the new shape:

- `g p` Proxmox, `g d` Docker, `g k` Kubernetes, `g n` TrueNAS,
  `g v` vSphere, `g s` Standalone
- `g a` Alerts, `g r` Patrol, `g t` Settings, `?` shortcuts help

The unified IA pages (`/infrastructure`, `/workloads`, `/storage`,
`/recovery`) and their aggregate route aliases are gone. The Command
Palette (`Cmd/Ctrl+K`) and global search (`/` to focus) still work and
are the fastest way across platforms.

## Support Stance

- Pulse v5.1.32 remains the current stable line.
- Pulse v6 `rc.6` is an opt-in evaluation build, not the default
  production recommendation.
- Existing v5 users should still prefer staging, lab, or otherwise
  controlled evaluation first.
- If you previously evaluated `rc.1` through `rc.5` on the unified IA
  and held back because of the navigation, `rc.6` is the build to look
  at again.
- The stable rollback target for this candidate is `v5.1.32`:
  `./scripts/install.sh --version v5.1.32`

## What changed since `rc.5`

### Frontend information architecture reverted to platform-shaped

The unified `/infrastructure`, `/workloads`, `/storage`, and `/recovery`
pages have been retired in favour of per-platform top-level pages. The
aggregate route aliases and legacy top-level workspace routes were
removed in the same pass, along with the orphaned summary components,
aggregate state hooks, and presentation utilities the unified pages
relied on. The unified resource model and `/api/resources` contract are
retained on the backend; per-platform pages consume that contract and
add platform-shaped presentation. Drawer shells, inline detail rows,
and per-platform filters were unified across Workloads, Docker, K8s,
and host drawers so the platform-shaped pages share their detail
surface.

### vSphere as a first-class platform

vSphere is a top-level platform in `rc.6`, parallel to Proxmox, Docker,
Kubernetes, and TrueNAS. The surface carries vSphere VMs through the
shared workloads pipeline, an inventory of vSphere networks, hosts with
version and uptime, cluster services, VM hardware configuration, VMware
Tools status, vCenter MoRef in the workload ID column, snapshot trees,
and a vSphere placement card in the workload drawer. vSphere tables
follow the canonical platform-table column alignment helper and the
shared platform conventions used by Proxmox and Docker.

### Standalone surface renamed to Machines, with native detail UX

The surface previously labelled "Agents" and then "Standalone" is now
"Machines". The rename consolidates the IA decision and the landing
behaviour into one name. The Machines table gained row identity
context, an expansion affordance, IP / disk I/O / RAID / network /
temperature detail tooltips, an aggregate disk summary in machine
details, SMART temperature fallback, machine discovery promoted into
drawer tabs, and a richer agent telemetry view. Machines is now
restricted to Pulse Agent resources.

### TrueNAS native detail surfacing

TrueNAS got native inline detail rendering across storage, system,
service, protection, and health rows, plus a shared TrueNAS detail
table extraction. TrueNAS health alerts surface on the overview, and
TrueNAS alert detail rows now appear in the drawer.

### Proxmox backup recovery coverage and tab polish

Proxmox backup tabs got click-to-sort across all three tabs, visual
density alignment with the Storage and Ceph pages, a canonical
ProgressBar for metric bars, a Backups column hidden on non-Proxmox
workload surfaces, a workload-row backup-age display, a consistency
pass on the sub-page conventions, and a coverage view that surfaces
which workloads have recent PBS artifacts. The Replication tab is
hidden when no replication signals exist.

### FilterBar adoption and SavedViews wiring

The Alerts history page, the audit log filter form, and the embedded
workloads filter all migrated to the canonical FilterBar pattern with
URL-backed filter state and SavedViews wiring. SavedViews are now wired
into Storage with a platform-scoped key, and the default-star is always
visible. Workloads search and `statusMode` migrated to URL params, and
the localStorage backup for `viewMode` and `containerRuntime` was
dropped so URL state is the single source of truth.

### Patrol intelligence: capacity-forecast and reliability findings

Patrol gained a capacity-forecast action template registry, forecast
proposals that attach onto a Patrol `RemediationPlan`, a reliability
finding that fires when an alert starts flapping, a PDM (Proxmox
Datacenter Manager) HTTP alert bridge that emits and resolves through
`FindingsStore.Add`, and a verification-outcome and capability-
postcondition substrate for finding lifecycle.

### Self-hosted commercial posture: free-first, no caps

Self-hosted commercial framing moved to free-first. The self-hosted
trial start route, hosted AI quickstart runtime, self-hosted AI
quickstart surfaces, monitored-system handoff prompts, and inactive Pro
upsell helpers are retired. Self-hosted guest capacity caps are gone,
self-hosted Pro continuity holds with no caps, and the Pulse Pro value
copy was aligned to the free-first posture. The Pro upsell path remains
opt-in.

The licensing posture from `rc.5` carries through unchanged: Community,
Relay, and Pro include core monitoring included by default; Relay
remains secure remote access to the Pulse web UI, Pulse Mobile pairing for handoff,
push notifications, and 14-day history; Pro remains Relay plus AI
operations, automation, advanced admin features, and 90-day history.

### Install pipeline hardening

The release pipeline now ships `install.sh` as a GitHub Release asset,
gates the release on an end-to-end `install.sh` smoke test against the
published release, and self-tests the smoke gate on every workflow
edit. The archive install path now requires a `.sshsig` sidecar.
Windows agent onboarding moved to a seamless install flow with a
corrected installer readiness path.

### Connection identity and source badges

The platform source badge is now live and de-duplicates cluster
members. A connection-degraded alert fires for wedged platform
connections. Platform identity badges were clarified on the
infrastructure system rows.

### Performance and correctness

Workload charts are faster (TTL cache plus removal of redundant
clones). Concurrent `/api/state` and `/api/diagnostics` reads are
deduplicated through a singleflight gate. A regression that stripped
workloads `runtime` and `namespace` before guest data loaded is fixed.
Grouped notification cancellation, mixed quiet-hours notification
replay queueing, and resolved notifications after direct alert dispatch
are fixed.

## Validation

This packet is audited against the commit range from the published
`v6.0.0-rc.5` tag through the validation-risk commit:

- `v6.0.0-rc.5`: `604a94d46e3be3687229e429aea282d3c3015fa4`
- validation-risk commit: `df793493683737c31961dd5b770fd98d37fa15d8`
- range: `v6.0.0-rc.5..df793493683737c31961dd5b770fd98d37fa15d8`
- commit count: `616`
- changed scope: `1379` files, `139185` insertions, `67870` deletions

The bulk of the range is concentrated in `frontend-modern/src`
(information architecture revert, vSphere surface, Machines detail
UX), `internal/vmware` (vSphere collector and inventory work),
`internal/api` (recovery and resources contract continuity),
`internal/ai` (capacity-forecast, PDM bridge, verification
substrate), and `.github/workflows` (install.sh smoke gate).

## Retest Plan

1. Frontend information architecture: each of Proxmox, Docker,
   Kubernetes, TrueNAS, vSphere, Standalone (Machines) loads as its
   own top-level page; no `/infrastructure`, `/workloads`,
   `/storage`, or `/recovery` aggregate routes remain; Command Palette
   (`Cmd/Ctrl+K`) navigates across platforms; keyboard shortcuts
   `g p / g d / g k / g n / g v / g s / g a / g r / g t` work.
2. vSphere: VMs through workloads pipeline, network inventory, hosts
   with version and uptime, cluster services, VM hardware config,
   VMware Tools status, vCenter MoRef in workload ID column, snapshot
   trees in drawer, vSphere placement card.
3. Machines: row identity, expansion affordance, IP / disk I/O / RAID
   / network / temperature detail tooltips, aggregate disk summary,
   SMART fallback, machine discovery drawer tabs.
4. TrueNAS: native storage/system/service/protection details, alert
   detail rows, overview health alerts.
5. Proxmox backups: click-sort across the three tabs, canonical
   ProgressBar, hidden Replication tab on no-signals, backup-age on
   workload rows, recovery coverage view.
6. FilterBar + SavedViews: Alerts history, audit log filter, embedded
   workloads filter, Storage SavedViews, default-star visible, URL-
   backed workload search and `statusMode`.
7. Patrol: capacity-forecast action template, forecast proposals on
   RemediationPlan, flapping-alert reliability finding, PDM HTTP
   alert bridge.
8. Self-hosted commercial: no trial start route, no AI quickstart
   surfaces, no guest capacity caps, Pro continuity, free-first copy.
9. Install pipeline: `install.sh` smoke gate against published
   release, `.sshsig` sidecar required on archive install path.
10. Release artifact download, checksum/signature, and installer
    paths before broad retesting.

## Evidence Appendix

For the code-backed evidence packet that maps these claims to the
current release line, see:

- `docs/release-control/v6/internal/subsystems/api-contracts.json`
- `docs/release-control/v6/internal/subsystems/unified-resources.json`
- `docs/release-control/v6/internal/subsystems/monitoring.json`
- `docs/release-control/v6/internal/subsystems/patrol-intelligence.json`
- `docs/release-control/v6/internal/subsystems/ai-runtime.json`
- `frontend-modern/src/features/platformPage/columnAlignment.ts`
- `frontend-modern/src/App.tsx` (top-level route shape)
- `frontend-modern/src/hooks/useKeyboardShortcuts.ts` (platform-keyed
  shortcuts)
