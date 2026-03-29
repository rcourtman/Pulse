# Storage Recovery Contract

## Contract Metadata

```json
{
  "subsystem_id": "storage-recovery",
  "lane": "L15",
  "contract_file": "docs/release-control/v6/internal/subsystems/storage-recovery.md",
  "status_file": "docs/release-control/v6/internal/status.json",
  "registry_file": "docs/release-control/v6/internal/subsystems/registry.json",
  "dependency_subsystem_ids": [
    "api-contracts",
    "frontend-primitives",
    "unified-resources"
  ]
}
```

## Purpose

Own the storage and recovery product surfaces, recovery-point persistence and
querying, and the operator-facing storage health presentation layer.

## Canonical Files

1. `internal/recovery/index.go`
2. `internal/recovery/manager/manager.go`
3. `internal/recovery/store/store.go`
4. `frontend-modern/src/components/Recovery/Recovery.tsx`
5. `frontend-modern/src/features/recovery/useRecoverySurfaceState.ts`
6. `frontend-modern/src/components/Recovery/RecoveryProtectedInventorySection.tsx`
7. `frontend-modern/src/components/Recovery/RecoveryActivitySection.tsx`
8. `frontend-modern/src/components/Recovery/RecoverySummary.tsx`
9. `frontend-modern/src/components/Recovery/RecoveryHistorySection.tsx`
10. `frontend-modern/src/components/Recovery/RecoveryHistoryTable.tsx`
11. `frontend-modern/src/components/Recovery/RecoveryPointDetails.tsx`
12. `frontend-modern/src/components/Recovery/useRecoveryHistorySectionState.ts`
13. `frontend-modern/src/pages/Storage.tsx`
14. `frontend-modern/src/components/Storage/Storage.tsx`
15. `frontend-modern/src/features/storageBackups/storageModelCore.ts`
16. `frontend-modern/src/hooks/useRecoveryPoints.ts`
17. `frontend-modern/src/hooks/useRecoveryRollups.ts`
18. `frontend-modern/src/pages/RecoveryRoute.tsx`
19. `frontend-modern/src/routing/resourceLinks.ts`
20. `frontend-modern/src/pages/Dashboard.tsx`
21. `frontend-modern/src/features/dashboardOverview/dashboardWidgets.ts`
22. `frontend-modern/src/components/Recovery/DashboardRecoveryStatusPanel.tsx`
23. `frontend-modern/src/components/Storage/DashboardStoragePanel.tsx`
24. `frontend-modern/src/types/recovery.ts`
25. `frontend-modern/src/utils/recoverySummaryPresentation.ts`
26. `frontend-modern/src/utils/recoveryTablePresentation.ts`
27. `frontend-modern/src/utils/textPresentation.ts`

## Shared Boundaries

1. None.

## Extension Points

1. Add or change recovery-point persistence, rollups, or series derivation through `internal/recovery/`
2. Add or change recovery page UX through `frontend-modern/src/components/Recovery/` and keep canonical route/query/filter state ownership in `frontend-modern/src/features/recovery/useRecoverySurfaceState.ts`
3. Add or change storage page UX through `frontend-modern/src/pages/Storage.tsx`, `frontend-modern/src/components/Storage/`, and `frontend-modern/src/features/storageBackups/`
4. Route transport changes for storage and recovery endpoints through `internal/api/` and the owning `api-contracts` proof routes
5. Route canonical storage/recovery resource selection through `frontend-modern/src/hooks/useUnifiedResources.ts` and the owning `unified-resources` contract
   That shared hook now also projects resource `clusterId` through the shared cluster-name helper, so storage and recovery links keep the same cluster-context label as other unified-resource consumers instead of rebuilding a local fallback chain.
6. Preserve API-owned node identity continuity in shared `internal/api/` helpers so storage and recovery transport attachments do not fork by hostname-versus-IP drift across the same runtime.
7. Preserve fail-closed API assignment and lookup behavior in shared `internal/api/` helpers so storage and recovery surfaces do not inherit orphaned profile or resource references from unrelated transport mutations.
8. Preserve canonical configured public endpoint selection in shared `internal/api/` helpers so recovery and storage links do not inherit loopback-local scheme drift from admin-originated setup/install flows.
9. Preserve trailing-slash normalization in those shared install-command helpers so recovery-adjacent transport and link surfaces do not inherit double-slash installer paths or slash-suffixed public endpoint drift from canonical backend install payloads.
10. Preserve canonical /api/auto-register token-action truth in shared `internal/api/` helpers so adjacent setup and recovery-adjacent transport flows stay on caller-supplied credential completion instead of reviving deleted alternate completion modes.
11. Preserve the canonical setup-script `source="script"` marker through those same shared auto-register helpers, and reject non-canonical source labels there, so later canonical reruns can keep treating script-confirmed tokens differently from agent-created tokens without reviving arbitrary caller-label compatibility.
12. Preserve the canonical auto-register node-type boundary in those same shared helpers so only supported `pve` and `pbs` registrations can complete, and unsupported runtime labels cannot bleed fake node identities into adjacent transport or recovery-adjacent state.
13. Preserve the canonical auto-register token-identity boundary in those same shared helpers so only Pulse-managed `pulse-monitor@{pve|pbs}!pulse-<canonical-scope-slug>` token IDs matching the requested node type can complete, and arbitrary, cross-type, or non-Pulse-managed token identities cannot bleed into adjacent transport or recovery-adjacent state.
14. Preserve canonical /api/auto-register DHCP continuity in those shared helpers so a PVE or PBS node that reruns registration from a new IP with the same canonical node name and deterministic Pulse-managed token identity updates in place instead of duplicating the inventory record.
15. Preserve the governed root-or-sudo Unix wrapper in shared backend install-command helpers so storage- and recovery-adjacent transport surfaces do not inherit a stale raw `| bash -s --` install payload shape from the canonical agent-install-command API and hosted Proxmox install responses.
16. Preserve optional-auth tokenless behavior in those same shared backend install-command helpers so adjacent transport surfaces do not implicitly persist API tokens and flip auth-configured state when an operator only requested a Proxmox install command on a token-optional Pulse instance.
17. Preserve backend-owned Pulse Mobile relay runtime credential minting in those same shared `internal/api/` auth/security helpers so storage- and recovery-adjacent transport surfaces do not inherit browser-authored wildcard token bundles when they depend on the canonical security helper layer.
18. Preserve the dedicated backend-owned `relay:mobile:access` capability and its governed backward-compatible route inventory plus the shared helper call sites around it, so storage- and recovery-adjacent transport surfaces do not treat the mobile relay credential as a general AI scope bundle.
19. Preserve shipped local security-doc guidance in shared `internal/api/` config/setup helpers so storage- and recovery-adjacent transport surfaces do not reintroduce GitHub `main` security links when the running build already serves its own local security documentation route.
20. Keep shared `internal/api/` Patrol transport and alert-trigger edits feature-isolated: Patrol-specific recency fields, callback fan-out, or alert-bridge wiring changes must not leak into recovery queries, storage links, or recovery-adjacent install/setup flows unless this contract changes in the same slice.

## Forbidden Paths

1. Reintroducing storage or recovery product logic as ad hoc dashboard-only summaries without a canonical page-surface owner
2. Duplicating recovery-point normalization or rollup derivation outside `internal/recovery/`
3. Letting storage health presentation rules drift between `frontend-modern/src/components/Storage/` and `frontend-modern/src/features/storageBackups/`
4. Treating storage and recovery as implicit leftovers inside broad monitoring or E2E lanes instead of governed product surfaces
5. Writing internal `NormalizedHealth` values directly to the storage URL status param; the URL must use the canonical option values from `STORAGE_STATUS_FILTER_OPTIONS` (e.g., `available` for the Healthy filter) so that shared links and bookmarks reflect the same values that the filter dropdowns present to operators
6. Letting whitespace-padded storage route params hydrate non-canonical page state; shared storage URLs must trim and normalize `tab`, `source`, `status`, `node`, `group`, `sort`, `order`, `query`, and deep-link `resource` before the page model consumes them so pasted or hand-edited links resolve to the same canonical state as UI-authored routes without dropping adjacent unmanaged params
7. Letting storage `source` aliases or case drift survive in canonical route state; shared storage URLs must rewrite pasted values like `PVE`, `pbs`, or `ALL` to the owned source option values (for example `proxmox-pve`) or the canonical unset state so copied links match the same source filter values the storage toolbar presents
8. Letting explicit storage `all` sentinels survive in canonical route state; shared storage URLs must collapse case- or whitespace-variant `all` values for the managed `node` filter back to the canonical unset state so copied links do not preserve a fake active node filter
9. Letting whitespace-padded recovery timeline params fall off canonical route state; shared recovery URLs must trim and normalize `day`, `range`, `scope`, `status`, `verification`, `cluster`, `node`, `namespace`, `itemType`, and adjacent history filters before the page model validates them so pasted or hand-edited links resolve to the same canonical timeline and filter state as UI-authored routes
10. Letting explicit recovery `all` sentinels survive in canonical route state; shared recovery URLs must collapse case- or whitespace-variant `all` values for `cluster`, `node`, `namespace`, and `itemType` back to the canonical unset route state so copied links do not preserve fake active filters
11. Letting non-canonical recovery platform values survive in route or transport state; shared recovery URLs must collapse unsupported or fake `platform` values back to the canonical unset state, and only owned source-platform options or canonical legacy aliases may reach rollups, points, series, and facets transport filters
12. Letting protected-item recovery outcome filtering fork from the canonical history status filter; the protected inventory status control must drive the same route-backed `status` field and the same rollups, points, series, and facets transport filters as the history surface instead of keeping a protected-only local outcome branch
13. Letting visible protected-item filters fall out of shared recovery links; the protected `Stale only` toggle must restore from the canonical recovery URL and rewrite to one owned `stale=1` route form instead of disappearing on refresh or copy/paste
14. Reintroducing stacked full-width recovery tables as the primary desktop layout; the governed recovery surface must expose one primary data region at a time with explicit protected-items versus recovery-events view switching so Pulse stays inventory-first for Proxmox operators without collapsing the page back into a single-platform backup screen
15. Letting the primary recovery workspace tabs drift out of canonical route state; when operators explicitly switch between protected items and recovery events, the shared recovery link builder and page model must preserve that selection in route state unless the active `rollupId` or `day` context already implies the default workspace

## Completion Obligations

1. Update this contract when canonical storage or recovery entry points move
2. Keep recovery store/runtime changes aligned with the storage and recovery frontend proofs in `registry.json`
3. Tighten guardrails when legacy storage or recovery presentation paths are removed
4. Preserve the dependency split: API payload ownership stays in `api-contracts`, settings shell ownership stays in `frontend-primitives`, and canonical resource truth stays in `unified-resources`
5. Keep recovery history table width budgeting derived from the canonical column specs in `frontend-modern/src/utils/recoveryTablePresentation.ts`, not from raw visible-column counts, so normalized subject labels and optional column sets cannot drift the right-edge badges and controls off-screen
6. Keep at least one browser-level desktop recovery proof in the governed `recovery-product-surface` policy so right-edge column visibility and wrapper-fit regressions are caught at rendered layout time instead of only through unit-level width math
7. Keep the dashboard route's no-resources state on an explicit first-session
   handoff. `frontend-modern/src/pages/Dashboard.tsx` may stay storage/recovery-
   owned for dashboard composition, but when no resources have reported yet it
   must route operators to `/settings/infrastructure/install` instead of
   leaving the dashboard as a passive dead end.

## Current State

This subsystem now sits under the dedicated storage and recovery lane so the
operator-facing storage page, recovery timeline, and recovery-point persistence
engine stop hiding inside broader monitoring and E2E buckets.

Storage and recovery still consume the shared unified-resource contract, but
they do not own the timeline store itself. The canonical resource-change
history now lives in `internal/unifiedresources/store.go` and is surfaced
through the shared API/resource wiring, which keeps storage and recovery focused
on presentation and query shape rather than re-implementing change persistence.

The recovery backend is a real product boundary, not just a helper package:
`internal/recovery/` owns per-tenant SQLite persistence, rollup derivation,
query filtering, and recovery-point indexing for the `/api/recovery/*`
surfaces.
That same shared `internal/api/` dependency also assumes auth-persistence
teardown is synchronous when recovery-adjacent runtimes reinitialize. Session,
CSRF, and recovery-token workers may not leave stale background goroutines or
half-shutdown path ownership behind, because hosted handoff, recovery
inspection, and adjacent temp-path tests all depend on the same canonical
runtime data-dir authority being replaceable without hangs or leaked state,
and router teardown must close the exact session, CSRF, and recovery-token
workers that router initialized instead of assuming a later global auth-store
binding will clean them up.
That same runtime data-dir authority also assumes file-backed stores keep
canonical filenames opaque and machine-owned. Recovery-adjacent session,
knowledge, and discovery records may discover legacy identifier-derived files
only for migration, and the next successful write must replace those legacy
paths with hashed canonical names so operator-controlled identifiers do not
become durable filesystem path segments.
That same hosted handoff dependency also assumes the exchange path repairs
tenant org access before redirecting the browser into protected routes.
Recovery- and storage-adjacent hosted pages that open immediately after
control-plane handoff must see a real tenant member session, not a freshly
minted browser cookie that still fails tenant authorization because the org
metadata lagged behind control-plane account membership.
That shared `internal/api/` dependency now also assumes hosted tenant AI
bootstrap and chat-runtime reads resolve through one effective hosted billing
lease before storage- or recovery-adjacent runtime consumers inspect
assistant availability, so recovery points, restore guidance, and related
operator surfaces do not read a tenant-org AI readiness state that diverges
from the machine-owned hosted entitlement already governing the instance.
That same shared `internal/api/` dependency also now assumes hosted runtime
websocket upgrades trust the cloud proxy only through explicit tenant
`PULSE_TRUSTED_PROXY_CIDRS` wiring, so storage- and recovery-adjacent live
status surfaces do not fall into reconnect loops after a hosted workspace
handoff.
That same shared `internal/api/` dependency also assumes telemetry
transparency stays on its governed system-settings trust surface. When shared
router or config-system files move under storage- or recovery-adjacent work,
telemetry preview and install-ID reset routes must keep reusing the canonical
system-settings boundary and the server-owned telemetry runtime instead of
being treated as generic storage/recovery transport fallout.
That same shared `internal/api/ai_handlers.go` dependency also now assumes
Patrol-specific AI settings and status transport stay isolated from
storage/recovery product state. When shared AI handlers add split Patrol
trigger-source fields, scoped-activity recency, or queued-trigger status,
recovery queries, storage links, and recovery-adjacent setup flows must treat
those as Patrol-only runtime facts rather than inheriting them as recovery
verification or storage-health transport.
That same shared helper layer also now assumes the Pulse Mobile relay runtime
credential reaches only the explicit backend-owned route inventory, so
storage- and recovery-adjacent transport work cannot accidentally widen that
credential into a broader AI access bundle by touching neighboring routes.
The recovery frontend now also separates that ownership more explicitly:
`frontend-modern/src/features/recovery/useRecoverySurfaceState.ts` owns
canonical route parsing, filter/query state, transport hook inputs, and URL
synchronization, while `frontend-modern/src/components/Recovery/Recovery.tsx`
is the composition root for the operator-facing recovery surface and the split
section owners under `frontend-modern/src/components/Recovery/` hold the
protected inventory, activity, and history presentation layers. The history
surface is further split so `RecoveryHistorySection.tsx` owns the toolbar and
controller boundary, `useRecoveryHistorySectionState.ts` owns local section UI
state, and `RecoveryHistoryTable.tsx` owns the row/detail renderer.
That composition root now also owns one primary recovery workspace rather than
stacking protected-inventory and event-history tables on the same desktop page.
The governed default remains inventory-first so Proxmox-oriented operators land
on the familiar protected-items view, but drill-in actions such as selecting a
subject or a timeline day must switch the primary workspace to recovery events
instead of leaving two competing table surfaces visible at once.
That same operator-facing workspace must lead with current protection status
rather than only the latest backup outcome. Protected items should surface
stale, never-succeeded, failed, warning, and running rollups as the primary
monitoring status so an item with an old successful point does not scan as
healthy when it still needs operator attention.
That same workspace contract also keeps Pulse's provider-neutral recovery model
explicit in the page language: recovery sections should talk about protected
items, recovery events, and latest points so PBS backups, TrueNAS snapshots,
Kubernetes artifacts, and future providers all fit the same first-class UI
frame without removing the source badges and row-level cues that make Proxmox
operators productive.
Operator-facing filter and detail labels should likewise prefer `platform`
wording over implementation-facing `provider` wording, so the recovery surface
describes the monitored platform families Pulse covers rather than exposing
backend transport vocabulary as the primary UI model.
That same operator-facing vocabulary should also prefer `item` over backend
`subject` wording, and `platform` over generic `source` wording, across the
primary recovery headers, tables, focus chips, and detail metadata labels.
The data model can keep its internal subject/provider fields, but the page
frame that operators read should present one consistent protected-item and
platform model from summary through drill-in. Shared recovery URLs and
transport filters should likewise treat `platform` as the canonical
operator-facing query field, with legacy `provider` aliases accepted only as
compatibility input that rewrites back to canonical `platform` route state.
Shared recovery link builders should therefore accept canonical `platform`
inputs only; legacy `provider` belongs at parse-time compatibility boundaries,
not in new caller-facing recovery route helpers.
Cross-surface recovery drill-in links must also declare the correct primary
workspace instead of relying on the inventory-first page default. When a
platform service surface such as PBS links into recovery activity, that shared
entry point should land on canonical `view=events` and describe the destination
as recovery events rather than reverting to PBS-backup wording.
That same recovery contract should keep response payloads canonical as well:
recovery points and protected rollups should expose `platform` and
`platforms` as the primary transport fields, while any legacy
`provider` / `providers` aliases stay compatibility-only so the page does not
silently drift back to backend-shaped vocabulary during decode.
That same shared recovery table contract should keep its runtime column model
canonical as well. Recovery inventory and event-history columns should use
`item` and `platform` identities rather than preserving `subject` and `source`
as the primary runtime model, and any saved legacy column IDs must migrate at
the shared column-visibility boundary instead of forcing recovery renderers to
carry deleted column identities indefinitely. Once that migration exists,
recovery tables and shared table presenters should not continue accepting
legacy `subject` and `source` ids in the live runtime path.
That same runtime-helper contract should prefer `item` terminology in shared
recovery presenters too. Helper exports that resolve labels or item-type badges
should expose canonical item-facing names, while any retained `subject` aliases
remain compatibility wrappers instead of the primary runtime boundary.
That same shared badge contract applies to table rendering too. Recovery item
type cells should use the same compact monitoring-table badge base that
workloads uses for `VM` and `Container`, rather than copying only the colors
and drifting on padding or visual weight.
The same rule applies inside recovery-owned helpers and selectors. Shared
summary helpers and platform filter renderers should use canonical `item` and
`platform` naming internally once compatibility boundaries already exist,
rather than keeping fresh `subject` or `provider` terminology alive in the
live recovery runtime path.
The same runtime vocabulary rule applies to cross-section recovery props too.
Live page-to-section boundaries should carry item-focused names like
`selectedHistoryItemLabel` instead of preserving `subject` labels after the
shared recovery presenters already expose canonical item terminology.
That same rule applies to recovery detail helpers. Provider-specific helper
names like `isPbsProvider` should become platform-specific helpers like
`isPbsPlatform` once the runtime recovery model is already canonically
platform-first.
The same canonical boundary applies to linked-resource identifiers. Recovery
API payloads, query filters, and normalized frontend runtime models should use
`itemResourceId` as the canonical field while accepting or emitting
`subjectResourceId` only as a compatibility alias during the transition.
That same canonical boundary also applies to external item references. Recovery
API payloads and normalized frontend runtime models should use `itemRef` as
the canonical item-reference field while treating `subjectRef` only as a
compatibility alias during the transition.
That same presenter boundary should also own canonical item-type derivation.
Recovery surfaces must resolve rollup and point item types through one shared
item-type helper instead of repeating `display.itemType` / `subjectType` /
`subjectRef.type` fallback chains across state, summary, details, and table
renderers.
That same recovery-store decode boundary must fail soft on malformed persisted
metadata. If a stored recovery row contains bad `subject_ref_json`,
`repository_ref_json`, or `details_json`, the list endpoints should log and
drop only the malformed derived field for that row rather than returning `500`
for the entire recovery points or rollups surface.
That same fail-soft contract also applies to downstream consumers that reuse
those shared store reads, including recovery-backed reporting, alert rollups,
and tenant-scoped AI recovery-point adapters. A malformed metadata blob may
degrade row-local enrichment, but it must not take down adjacent readers that
consume the same canonical recovery store.
That same recovery-store migration boundary must keep legacy schema upgrades in
dependency order. When a persisted `recovery.db` predates columns such as
`item_type`, the store must add the migrated columns before creating indexes or
running query paths that reference them, so opening a legacy store backfills
cleanly instead of returning `500` from `/api/recovery/points` or
`/api/recovery/rollups` during schema initialization.
That same recovery-store key boundary must keep `subject_key` genuinely stable
across ingest generations. Protected rollups must not split one Proxmox guest
into stale and fresh rows just because older points stored legacy linked IDs
like `lxc-*` or raw source IDs while newer points carry hashed canonical
resource IDs, and proxmox guest external keys must ignore display-name churn so
renaming a backup comment does not fork the protected inventory from recent
event history. That same store-owned continuity contract also applies when
Proxmox PBS guest points temporarily lose unified-resource linkage or drift
between historical PBS namespaces: if recovery history already proves one
canonical linked guest identity for the same friendly label, guest type, and
VMID/CTID, the store must relink later unresolved PBS points and backfill older
split rows onto that canonical protected item instead of leaving protected
inventory freshness to disagree with recovery events.
That same hook-boundary normalization also owns the runtime recovery display
model. Canonical recovery points and rollups must expose `display.itemLabel`
and `display.itemType` to recovery consumers, while legacy transport fields
such as `subjectLabel` and `subjectType` remain decode-only compatibility
aliases in the shared normalization layer instead of leaking into runtime
presenters.
That same canonical item-label boundary must prefer recognizable protected-item
names over raw entity IDs. When unresolved Proxmox-backed recovery points only
have a VMID/CTID in the subject ref but still carry a richer backup comment or
notes label, the canonical recovery index and store backfill must promote that
human-readable label into the persisted subject/item label instead of leaving
protected inventory rows to lead with bare numeric IDs.
That same operator-facing row-identity rule should still preserve the governed
entity identifier as secondary context when it exists. Recovery inventory and
event rows should lead with the canonical item name, then show a muted
secondary compact `VMID`/`CTID`/`ID` cue when `display.entityIdLabel` is
available, so operators can disambiguate familiar names without turning the
primary scan path back into raw numeric identifiers or bloating the table with
an extra recovery-only identity row.
That same shared presentation layer also owns the distinction between
aggregate recovery-method language and single-record recovery-method language.
Timeline legends and daily breakdowns must use aggregate labels such as
`Snapshots`, `Local Copies`, and `Remote Copies`, while event rows, filters,
and point details must use the singular operator-facing forms `Snapshot`,
`Local Copy`, and `Remote Copy`. Recovery point detail summaries must also
humanize backend fields like kind, mode, outcome, and boolean state into
operator-facing labels such as `Point Type`, `Method`, `Outcome`, `Verified`,
and `Encrypted` instead of leaking raw transport values like `backup`,
`remote`, or lowercase outcome tokens into the primary drawer surface.
That primary workspace selection now also lives in canonical recovery route
state through `frontend-modern/src/routing/resourceLinks.ts` and
`frontend-modern/src/features/recovery/useRecoverySurfaceState.ts`, so copied
links and browser restores reopen the same protected-items versus
recovery-events workspace an operator explicitly chose instead of silently
falling back to page-local UI state. Focused recovery routes with an active
`rollupId` or `day` remain recovery-events-first by default, but the explicit
workspace selection must still serialize through the owned recovery URL model
whenever an operator overrides that implied default.
That history table layout now also derives its minimum width from the same
canonical column-width spec that owns the header sizing in
`frontend-modern/src/utils/recoveryTablePresentation.ts`, so longer governed
subject labels do not force the trailing outcome/status columns off-screen by
budget drift.
That same recovery product proof surface now also includes a browser-level
desktop layout guard in `tests/integration/tests/17-recovery-layout.spec.ts`,
which opens the recovery page against deterministic recovery payloads and
fails when the history table needs horizontal scrolling or lets the outcome
column drift outside the visible wrapper at desktop width.
That same shared `internal/api/` dependency now also assumes tenant-scoped
resource handlers seed registries from canonical unified resources only:
recovery- and storage-adjacent API helpers may not fall back to raw tenant
`StateSnapshot` seeding once `UnifiedResourceSnapshotForTenant` is available.
That same shared `internal/api/` dependency now also assumes tenant AI
handlers stay on canonical Patrol runtime wiring: recovery- and
storage-adjacent API helpers must not revive tenant snapshot-provider bridges
through `internal/api/ai_handlers.go` once Patrol can initialize from tenant
`ReadState` and unified-resource providers directly.
That same adjacent AI handler boundary now also keeps Patrol runtime
availability explicit as API-owned state. Storage and recovery consumers may
share the handler layer, but they must not treat a blocked Patrol runtime as
healthy only because the last completed summary snapshot remained green.
That same shared dependency also assumes the Patrol-backed recent-changes
API surface reads through the canonical intelligence facade first, so
storage and recovery handlers do not bypass the shared unified timeline
through the older detector-only path.
The same shared boundary applies to the Patrol-backed correlation API
surface, which must read through the canonical intelligence facade before it
exposes learned relationship context to adjacent storage and recovery flows.
The same shared API runtime also exposes unified-resource action, lifecycle,
and export audit reads, but storage and recovery must continue to treat that
as adjacent governed API ownership rather than timeline-store ownership. The
storage and recovery lanes still own their own persistence and query
contracts, while the control-plane execution trail remains governed by the
unified-resource and audit contracts.
That same shared `internal/api/` dependency now also includes monitored-system
ledger explanation reads: storage- and recovery-adjacent surfaces may coexist
with counted monitored-system inventory, but any support-facing count
reasoning must come from the canonical unified-resource grouping explanation
payload rather than from storage or recovery heuristics.
That same shared hosted-entitlement refresh path must also preserve the
canonical quickstart grant metadata carried in billing state. Storage- and
recovery-adjacent hosted tenants may share Patrol-backed investigation and
recovery context with the rest of the app, but the shared `internal/api/`
lease refresh must not clear quickstart inventory and leave adjacent product
surfaces inferring a fake "unavailable" runtime from rewritten billing state.
That adjacent ledger read must also preserve canonical grouped system status,
including `warning`, so recovery- and storage-adjacent support views do not
flatten governed degraded state into a fake `unknown` label when the shared
unified-resource resolver already computed the top-level status.
That same adjacent ledger read now also carries backend-owned status
explanation copy, and support-facing details must render it beside the
counting rationale so operators can interpret warning, offline, and unknown
states without inventing page-local status wording.
The same API resource serializer also refreshes canonical identity and policy
metadata through the shared unified-resource helper before it writes resource
payloads, so storage and recovery links inherit the same canonical metadata
pass instead of carrying local attach wrappers in adjacent transport code.
The shared unified-resource facet bundle that storage-adjacent detail views
consume now also carries grouped `recentChangeKinds` counts by canonical change
kind, so storage and recovery surfaces can show the distribution of restarts,
anomalies, relationships, and capability changes without re-deriving their own
timeline breakdowns.
That same shared facet bundle now also carries grouped
`recentChangeSourceTypes` counts by canonical source type, so storage and
recovery surfaces can separate platform events, pulse diffs, heuristics,
user actions, and agent actions without inferring provenance from the loaded
slice.
That same shared facet bundle now also carries grouped
`recentChangeSourceAdapters` counts by canonical source adapter, so storage
and recovery surfaces can separate Docker, Proxmox, TrueNAS, and ops-helper
provenance without inferring integration origin from the loaded slice.
Those same resource timeline records also preserve `relatedResources`
relationship context for non-relationship changes, so storage and recovery
views can still link neighboring resources when the timeline entry is a
restart, anomaly, or config update rather than only when the edge itself
changes.
Those unified audit list endpoints also clamp oversized `limit` requests to
the governed maximum, so adjacent recovery and storage workflows do not turn
bounded history reads into unbounded collection scans.
The same shared API runtime now also exposes dedicated
`/api/resources/{id}/timeline` reads plus the bundled
`/api/resources/{id}/facets` surface, but storage and recovery must continue
to treat those as adjacent governed API ownership rather than storage/recovery
timeline ownership.
That same adjacent API layer now also exposes a VM inventory CSV export for the
reporting surface. Storage and recovery workflows may consume similar current-
state VM facts, but `internal/api/reporting_inventory_handlers.go` and
`internal/api/router_routes_licensing.go` remain API/reporting transport
ownership rather than storage/recovery contract ownership.
That adjacent reporting transport now also includes a reporting catalog route
whose nested VM inventory definition owns panel copy, stable column schema,
and filename prefix. Storage and recovery flows may read those facts when they
need fleet context, but they must not fork their own reporting or inventory
column contract.
That catalog route is intentionally metadata-readable without the
`advanced_reporting` feature gate so locked admin reporting shells can stay on
the same API-owned definition before upsell; storage- and recovery-adjacent
surfaces must not treat that metadata visibility as permission to execute paid
report/export routes.
That same API-owned performance-report definition also governs transport-side
validation and attachment naming. Storage and recovery flows may consume those
downloads, but they must treat allowed formats, multi-resource caps, optional
metric/title support, default fallback range windows, attachment filename
stems, and invalid-format validation copy as API/reporting contract rather than
rebuilding local reporting constants.
That same transport contract also owns report time-window validation. Storage-
and recovery-adjacent flows may omit `start`/`end` to use the canonical default
window, but when they provide either bound it must be RFC3339 and `end` must
not be earlier than `start`; invalid values fail as `400 invalid_time_range`
instead of silently shifting the exported reporting window.
That same adjacent API/reporting transport also owns the optional reporting
field limits and multi-report request parsing. Storage and recovery consumers
must treat `metricType`, `title`, request-body size, unknown JSON fields, and
trailing payload rejection as API-owned validation semantics rather than
counting on permissive backend coercion.
Those same transport rules now also carry explicit failure modes that adjacent
storage and recovery automation must preserve: bad or oversized multi-report
payloads fail as `invalid_body` or `body_too_large`, and overlong report
windows or invalid optional fields fail through the API-owned reporting
validation contract instead of being clipped or normalized locally.
That same API-owned contract also classifies those validation failures with
stable error codes, so storage and recovery flows must not derive behavior by
inspecting human-readable error text from adjacent reporting calls.
That adjacent export contract now also includes canonical Proxmox pool
membership for each VM row. Storage and recovery flows may use those current-
state facts when they need fleet context, but they must consume the API-owned
pool column rather than rebuilding pool membership from storage-side queries.
Those resource timeline reads now also accept governed kind and source-type
filters plus source-adapter filters, with filtered history counts owned by the
unified-resource store so storage and recovery views can consume the same
canonical history contract without re-deriving their own timeline slices.
Invalid `sourceAdapter` values are rejected at the API boundary, which keeps
storage and recovery reads aligned with the canonical adapter set instead of
turning the timeline filter into an arbitrary free-text escape hatch.
The router now wires the tenant resource state provider during initial setup
when a multi-tenant monitor is present, so tenant-scoped storage and recovery
pages do not hit a missing-provider 500 before the monitor is fully wired.
The shared unified-resource consumer hook now also preserves `recentChanges`,
`facetCounts`, `policy`, and `aiSafeSummary` fields when storage and recovery
surfaces read unified resources, so those pages see the same control-plane
timeline facets and recent-change totals as the dedicated resource drawer
instead of flattening them away locally.
That shared policy payload now remains intentionally minimal as well: storage
and recovery consumers should expect only routing scope and redaction hints;
the cloud-summary decision is derived from scope rather than stored as a
separate boolean flag.
The same storage-facing runtime paths now also normalize org scope through
`frontend-modern/src/utils/orgScope.ts` before building cache keys or
multi-tenant fetch state, so Dashboard, StorageSummary, and other storage
adjacent consumers do not each keep a local `getOrgID() || 'default'`
fallback.

The frontend storage and recovery surfaces are also first-class runtime entry
points. `frontend-modern/src/pages/Storage.tsx` is the storage route shell,
`frontend-modern/src/components/Storage/Storage.tsx` is the operator-facing
storage surface owner, and `frontend-modern/src/components/Storage/` plus
`frontend-modern/src/features/storageBackups/` define the storage health model
and presentation, while
the storage page's readiness now stays route-owned as well:
`frontend-modern/src/components/Storage/useStoragePageModel.ts` and
`frontend-modern/src/features/storageBackups/storagePageStatus.ts` must derive
loading, reconnect, and disconnect presentation from the storage unified-resource
fetch contract before consulting websocket churn so the storage surface does
not present healthy REST-backed data as down or stale.
Meanwhile,
`frontend-modern/src/components/Recovery/` and the recovery hooks define the
timeline, protected-item, and recovery-summary UX.
That governed page frame now starts with the shared `RecoverySummary.tsx`
overview card so operators land on a provider-neutral recovery posture model
before they drill into either protected-item inventory or recovery events.
The summary remains additive to the inventory-first workflow rather than a
replacement for it: Proxmox-heavy operators still land on protected items by
default, but the first visible framing now reflects the whole multi-platform
recovery system instead of a single backup table metaphor.
That top recovery frame must rely on solid elevated operator panels and border
hierarchy rather than decorative gradients so the page reads like a monitoring
workspace instead of a marketing-style dashboard shell.
That same framing must stay compact enough to keep the primary workspace in the
first scroll window. `RecoverySummary.tsx` and the workspace shell in
`Recovery.tsx` should compress posture, coverage, and mode framing into concise
operator panels instead of stacking multiple dashboard-like slabs that push the
main inventory or event table too far down the page.
That same summary shell should read as one dominant posture surface with a
supporting rail, not as several equal-weight dashboard cards. The governed
recovery summary should keep the left side focused on posture, attention, and
freshness, while the right side carries compact coverage and recent-history
reference detail instead of competing headline widgets.
That same workspace shell should stay a compact control strip rather than a
second summary card: the protected-items versus recovery-events switcher belongs
close to the active table surface, and it should not restate table counts that
are already carried by the selected tab labels or the summary frame above.
That same summary contract now also includes platform- and item-coverage
framing through `frontend-modern/src/utils/recoverySummaryPresentation.ts`, so
the first visible recovery overview explicitly shows which protected item
types Pulse is covering before it shows supporting platform mix, with
primary-item and primary-platform readouts kept as distinct governed summary
fields instead of one ambiguous "primary" label. The summary may still surface
platform breadth and cross-platform protected items, but the recovery page
must read item-first so the unified recovery model is not visually anchored to
one platform family.
That same item-first rule also applies to the protected inventory table:
`RecoveryProtectedInventorySection.tsx` must surface protected item type as a
first-class column in the main inventory grid rather than leaving platform as
the only structural classifier beside the item label. Platform badges remain
important supporting operator context, especially for Proxmox-heavy fleets,
but the table frame itself must make protected item class explicit.
That same inventory contract must keep the protected-items grid operationally
bounded. The governed desktop recovery surface should not dump the entire
protected estate into one endless slab; it should page or otherwise bound the
primary inventory table so the workspace, filters, and adjacent activity panel
remain readable as one monitoring surface instead of dissolving into a raw list
dump.
That same protected-inventory surface should carry compact operator orientation
inside the table shell itself. `RecoveryProtectedInventorySection.tsx` should
expose the current bounded range, page, and sort state near the primary grid,
and the first column should carry enough secondary item metadata to read as a
monitored inventory row rather than a bare export line.
That same hierarchy rule also applies to the activity timeline. The governed
recovery surface should not append `RecoveryActivitySection.tsx` underneath the
default protected-items view as if trend telemetry were a second page bolted
onto inventory. The timeline owns recovery-event day selection, so it belongs
inside the `Recovery events` workspace and should read as history analysis for
the selected window rather than as a second copy of the page-level posture
summary.
The same owned vocabulary applies to recovery events as well:
`frontend-modern/src/utils/recoveryTablePresentation.ts` must keep the
history-table `type` column labeled as `Item Type` within recovery surfaces so
event history does not fall back to a generic shared `Type` header once the
recovery lane has already established item-first operator vocabulary.
That same item-first vocabulary must carry through the point-details drawer:
when a recovery point includes canonical item-class metadata,
`RecoveryPointDetails.tsx` must surface it as `Item Type` in the summary grid
instead of jumping directly from item identity to platform and point-method
metadata.
That same shared presentation layer also owns recovery placement vocabulary.
Cluster, node, and namespace facets remain valid supporting filters for
Proxmox-heavy and Kubernetes-heavy operators, but the governed recovery
surface must present them through platform-neutral labels such as
`Cluster / Site`, `Host / Agent`, and `Namespace / Group` across advanced
filters, active chips, table headers, column-picker entries, and point
details so the page treats placement as optional context inside a
multi-platform recovery model rather than a Proxmox-native spine. When
normalized display labels are present, the visible history rows must prefer
those labels over raw transport values for the same placement dimensions.
The recovery table presentation helper now owns the canonical subject-type
label fallback for recovery rows and delegates its title-casing to the shared
`frontend-modern/src/utils/textPresentation.ts` helper rather than keeping a
local recovery-only formatter, so subject and outcome labels stay aligned with
the shared frontend label contract.
That same recovery drill-in surface now also keeps provider-specific metadata
inside a provider-neutral detail shell through
`frontend-modern/src/components/Recovery/RecoveryPointDetails.tsx`, so PBS
datastore and verification enrichments remain available without presenting the
event drawer itself as if PBS were the native recovery model. Operator-facing
detail labels in that drawer should prefer neutral target wording such as
`Target Ref`, `Target Resource`, and `Target Health`.
Those transport hooks are direct governed runtime surfaces, not just page
implementation detail: `frontend-modern/src/hooks/useRecoveryPoints.ts`,
`frontend-modern/src/hooks/useRecoveryPointsFacets.ts`,
`frontend-modern/src/hooks/useRecoveryPointsSeries.ts`, and
`frontend-modern/src/hooks/useRecoveryRollups.ts` must stay on the explicit
`recovery-product-surface` proof path instead of inheriting release-control
coverage only through `frontend-modern/src/pages/RecoveryRoute.tsx`.
Those same hooks now also own recovery transport normalization at the frontend
boundary: raw compatibility fields such as `provider` / `providers` may be
accepted from older `/api/recovery/*` payloads, but the runtime values they
return to the rest of the recovery UI must be canonical `platform` /
`platforms` models.
That same rule applies to the dashboard recovery entry points too:
`frontend-modern/src/hooks/useDashboardRecovery.ts`,
`frontend-modern/src/pages/Dashboard.tsx`,
`frontend-modern/src/features/dashboardOverview/dashboardWidgets.ts`,
`frontend-modern/src/components/Recovery/DashboardRecoveryStatusPanel.tsx`, and
`frontend-modern/src/utils/dashboardRecoveryPresentation.ts` must stay on
explicit direct dashboard/recovery proof routing instead of inheriting
coverage only through the full recovery route or broader dashboard shells.
The storage dashboard entry point must be treated the same way on the storage
side: `frontend-modern/src/pages/Dashboard.tsx`,
`frontend-modern/src/features/dashboardOverview/dashboardWidgets.ts`, and
`frontend-modern/src/components/Storage/DashboardStoragePanel.tsx` and
`frontend-modern/src/utils/dashboardStoragePresentation.ts` must stay on
explicit direct dashboard/storage proof routing instead of borrowing release-
control coverage only from the broader storage page, component, and model
surfaces.
That route shell now also composes the recent-alerts widget directly from the
alert-owned `frontend-modern/src/components/Alerts/RecentAlertsPanel.tsx`
surface instead of via a dashboard-panels-local alert implementation, so the
dashboard route stays storage/recovery-owned while alert widget runtime remains
owned by the alerts subsystem. That route handoff must stay thin: the dashboard
page should pass the live alert list into `RecentAlertsPanel` and let the
alert-owned surface derive its own summary and acknowledgement state instead of
rebuilding alert summary counts or alert-action runtime inside the
storage/recovery-governed dashboard route.
The shared recovery type contract must be pinned the same way:
`frontend-modern/src/types/recovery.ts` must stay on the explicit
`recovery-product-surface` proof path instead of riding indirectly on route or
component coverage.
That same direct proof rule applies to the shared recovery date helper:
`frontend-modern/src/utils/recoveryDatePresentation.ts` must stay on the
explicit `recovery-product-surface` proof path instead of inheriting coverage
only through pages or higher-level recovery components.
That same direct proof rule also applies to the shared recovery status helper:
`frontend-modern/src/utils/recoveryStatusPresentation.ts` must stay on the
explicit `recovery-product-surface` proof path instead of inheriting coverage
only through pages or higher-level recovery components.
That same direct proof rule also applies to the shared recovery summary helper:
`frontend-modern/src/utils/recoverySummaryPresentation.ts` must stay on the
explicit `recovery-product-surface` proof path instead of inheriting coverage
only through pages or higher-level recovery components.
That same direct proof rule also applies to the shared recovery record helper:
`frontend-modern/src/utils/recoveryRecordPresentation.ts` must stay on the
explicit `recovery-product-surface` proof path instead of inheriting coverage
only through pages or higher-level recovery components.
That shared recovery record contract now also includes rollup-side display
payload continuity: the recovery backend must preserve the latest normalized
subject label on rollups, and recovery UI helpers must prefer that canonical
display label before raw subject ids whenever the live unified-resource map is
missing or only resolves to opaque machine identifiers.
That same direct proof rule also applies to the shared recovery outcome helper:
`frontend-modern/src/utils/recoveryOutcomePresentation.ts` must stay on the
explicit `recovery-product-surface` proof path instead of inheriting coverage
only through pages or higher-level recovery components.
That same direct proof rule also applies to the shared recovery action helper:
`frontend-modern/src/utils/recoveryActionPresentation.ts` must stay on the
explicit `recovery-product-surface` proof path instead of inheriting coverage
only through pages or higher-level recovery components.
That same direct proof rule also applies to the shared recovery artifact mode
helper: `frontend-modern/src/utils/recoveryArtifactModePresentation.ts` must
stay on the explicit `recovery-product-surface` proof path instead of
inheriting coverage only through pages or higher-level recovery components.
That same direct proof rule also applies to the shared recovery empty-state
helper: `frontend-modern/src/utils/recoveryEmptyStatePresentation.ts` must stay
on the explicit `recovery-product-surface` proof path instead of inheriting
coverage only through pages or higher-level recovery components.
That same direct proof rule also applies to the shared recovery filter-chip
helper: `frontend-modern/src/utils/recoveryFilterChipPresentation.ts` must stay
on the explicit `recovery-product-surface` proof path instead of inheriting
coverage only through pages or higher-level recovery components.
That same direct proof rule also applies to the shared recovery issue helper:
`frontend-modern/src/utils/recoveryIssuePresentation.ts` must stay on the
explicit `recovery-product-surface` proof path instead of inheriting coverage
only through pages or higher-level recovery components.
That same direct proof rule also applies to the shared recovery table helper:
`frontend-modern/src/utils/recoveryTablePresentation.ts` must stay on the
explicit `recovery-product-surface` proof path instead of inheriting coverage
only through pages or higher-level recovery components.
That same direct proof rule also applies to the shared recovery timeline-chart
helper: `frontend-modern/src/utils/recoveryTimelineChartPresentation.ts` must
stay on the explicit `recovery-product-surface` proof path instead of
inheriting coverage only through pages or higher-level recovery components.
That same direct proof rule also applies to the shared recovery timeline
helper: `frontend-modern/src/utils/recoveryTimelinePresentation.ts` must stay
on the explicit `recovery-product-surface` proof path instead of inheriting
coverage only through pages or higher-level recovery components.

Those recovery transport surfaces now also share one normalized filter
contract: protected-item rollups, point history, facets, and chart series must
all honor the same canonical `platform`, canonical `itemType`, cluster, node, namespace,
workload-scope, verification, and route-backed free-text `q` filter so the
protected-items list cannot drift from the timeline and facet state under the
same active recovery view. That same recovery filter contract now depends on
the canonical recovery index carrying a normalized `itemType` instead of
forcing each UI surface to re-derive protected item classes from raw
provider-native `subjectType` values.
That same recovery product surface keeps the primary workspace visually ahead
of secondary analytics: once the summary frame ends,
`frontend-modern/src/components/Recovery/Recovery.tsx` must lead directly into
the route-backed recovery workspace, with protected-items defaulting straight
to the primary inventory table and recovery-events owning both the activity
timeline and the event table together. The activity timeline remains required
even when point-history loading fails, but it belongs to the events workspace
rather than hanging below the inventory workspace as a competing page-level
peer.
That same recovery product surface must also stay stylistically aligned with
established Pulse monitoring surfaces once shared primitives already exist.
`frontend-modern/src/components/Recovery/RecoverySummary.tsx` should compose
the shared `SummaryPanel` and `SummaryMetricCard` primitives the way
infrastructure and workloads do, and recovery item-type labels should render
through canonical workload/resource badge classes instead of adding
recovery-only wrapper chrome around VM, container, or other resource badges.
That same summary frame should stay compact and scan-first, not turn into a
recovery-specific mini report. Recovery summary cards should prefer concise
counts, bars, and badge-backed coverage cues over tall explanatory copy blocks
so the protected-item table takes over the page at roughly the same visual
density as the infrastructure and workloads monitoring surfaces.
That same summary rule also applies to the panel header: the top-line summary
count strip should stay terse and avoid repeating the full posture telemetry
already rendered inside the cards, so the header acts as orientation rather
than a second miniature dashboard row.
That same header rule should default to the protected-item total only rather
than carrying separate attention, healthy, or running counters; those signals
belong in the cards themselves.
That same summary layout should also stay on the shared monitoring rhythm:
recovery should use the standard four-card `SummaryPanel` grid without letting
one oversized posture card dominate the full strip, so operators can scan
posture, freshness, footprint, and recent history at the same glance depth
they get on infrastructure and workloads.
That same summary rule also applies within individual cards: recovery posture,
freshness, attention, footprint, and history cards should favor compact rows
and metric lists over stacked prose callouts so the summary strip reads like
Pulse monitoring telemetry rather than a page-local narrative panel.
That same orientation-first rule also applies at the page-shell boundary:
recovery should hand straight from the summary strip into the active workspace
card without an extra page-local spacer band that makes the shell feel softer
than storage or workloads.
That same storage-style page rhythm also applies to the workspace tabs:
`Protected items` / `Recovery events` should live as a standalone subtab row
above the active section, not inside the data card itself.
That same card-level scan rule should prefer one dominant metric per card with
short supporting readouts, the same quick-scan rhythm operators already get on
infrastructure and workloads, instead of nested sub-cards that turn recovery
summary into a denser bespoke dashboard than the rest of Pulse.
That same summary-card rule should also avoid inset mini-panels inside the
cards themselves. Recovery summary cards should prefer direct metric rows,
lists, and badge-backed counts over "cards inside cards", which make the strip
feel heavier than the equivalent infrastructure and workloads telemetry.
That same compact-summary rule should also avoid repeating the same attention,
stale, and footprint telemetry in multiple stacked readouts inside one card.
Recovery summary cards should keep one dominant metric with one short support
band so the strip scans at the same speed as infrastructure and workloads
instead of behaving like a recovery-only executive summary.
That same card-headline rule should keep the helper label terse too. Recovery
headline captions should prefer short monitoring terms like `attention`,
`healthy`, `running`, `types`, or `points` over sentence-like helper copy that
makes the strip feel more bespoke than storage or workloads.
That support band should stay genuinely short, ideally one compact support line
and only occasionally two, so the operator can scan the summary as orientation
instead of reading each card like a dense checklist.
When a second support line does not materially change triage, recovery should
prefer a single support line over filling the card just because there is room.
Those support rows should also stay signal-driven. Recovery summary cards
should omit zero-value or low-value support rows like `Primary Item` or `Peak
Day` when they do not materially improve operator triage, so the top strip
stays closer to the fast-scan rhythm used by the infrastructure and workloads
surfaces.
That same summary-scan rule should also trim derivative rows that restate the
headline instead of adding a new operator question. `Freshness` should not add
an extra `<24h` row under a `fresh in 24h` headline, `Coverage` should not
spend a row repeating a primary-platform label when platform count already
frames the footprint, and `Activity` should stay on a short activity readout
rather than carrying a four-line micro report.
That same summary-card rule should also avoid inline distribution bars or
dashboard-style subvisualizations inside the cards when the same signal can be
expressed as short metric rows. Recovery summary cards should stay on the
shared monitoring-card rhythm of one dominant metric plus compact supporting
rows rather than reviving bespoke visual telemetry that makes the strip read
heavier than the rest of Pulse.
That same scan rule should also keep the summary header to orientation rather
than another status rail. The header should carry total protected items only;
the `Posture` card owns the attention-versus-healthy composition so the strip
does not repeat the same posture cue both above and inside the cards.
That same differentiation rule applies across cards too. `Posture` and
`Freshness` should not lead with the same stale/attention headline; the
freshness card should emphasize recent successful coverage such as fresh-in-24h
reach, while posture owns the attention-state headline.
That same page-level ownership applies to the recovery time window. Recovery
should use the shared summary-panel range control as the canonical time-range
selector, the same way infrastructure and workloads do, instead of hiding a
separate range selector inside the activity strip and making operators manage
two competing range affordances for one page.
That same density rule should route through the shared summary primitives
instead of feature-local spacing overrides. Recovery may select the shared
default `SummaryPanel` / `SummaryMetricCard` density mode, and it should not
reintroduce one-off padding hacks or right-side duplicate KPI blocks inside
`RecoverySummary.tsx`.
That same one-headline rule applies to the footprint and activity cards too.
`Coverage` should lead with one dominant item-type count and route platform
count through the shared card-header secondary label path instead of another
body row, and `Activity` should route latest-activity context through that same
secondary-label path instead of adding recovery-only support text under the
metric.
That same summary-header rule also applies to posture and freshness. Recovery
should prefer shared card-header secondary labels such as healthy-count or
stale-count context instead of stacking extra recovery-only support rows under
those headline metrics.
That same scan-first rule applies to the workspace strip. Recovery should not
show tab labels and then repeat the same workspace count as standalone text in
the same strip; the workspace tabs should carry their own counts, while the
remaining strip cues focus on issues or drill-in context.
That same inventory surface should also follow the established monitoring-table
scan pattern in its first column. Protected-item rows should lead with a clear
status cue, the primary item name, and compact badge-backed item/platform
context instead of relying on recovery-only rails or plain-text metadata lines
that make the table read like a report instead of an operational grid.
That same triage rule applies to the default inventory sort. The protected
items workspace should open with attention-state rollups first instead of
defaulting to newest-successful backups, so operators land on failed,
never-succeeded, stale, warning, and running items before the healthy catalog.
That same row contract should avoid duplicating context that already has a
dedicated column. When `Item Type` and `Platform` columns are visible, the
primary item cell should not restate those same badges on desktop; duplicate
context belongs only as a small-screen fallback when those columns collapse.
That same scan rule applies to the supporting columns themselves. Recovery
tables should keep one dominant identity cue and one canonical platform cue.
`Item Type` should use the same shared workload/resource badge treatment that
other Pulse tables use for `VM` and `Container`, while `Method` and similar
supporting fields stay on restrained metadata text instead of turning every
adjacent column into another colored badge.
That same item-identity contract also applies to synthetic Proxmox task
recovery points. When the persisted subject label is just a raw
`pve-task:*`/`UPID:*` identifier or `vmid=0`, the canonical recovery index
should derive a readable task label and `task` item type from point details so
recovery tables scan by operator meaning instead of transport IDs.
That same inventory surface should stay on the flat monitoring-table pattern
already used elsewhere in Pulse. Protected items should surface posture through
row-level status cues, outcome pills, and filters rather than inserting extra
`Needs Attention` / `Healthy Coverage` section rows that add height and turn
the primary table into a recovery-only grouped report.
That same protected-items table should also avoid recovery-local pagination
chrome. The workspace already holds the filtered rollups client-side, so it
should read as one continuous monitoring table with a simple protected-item
count instead of introducing `Prev` / `Next` buttons and page counters that do
not match the canonical Pulse scan pattern.
That same table-shell contract must avoid duplicate framing once the view tabs
already establish the active workspace. `RecoveryProtectedInventorySection.tsx`
should keep page/count/sort orientation inside a slim table-shell status row
and let the filter strip lead directly into the grid instead of reintroducing a
second large inventory header card above the same table.
That same shell rule should also avoid low-signal bookkeeping above the grid.
The protected-items status row should surface the active workspace, protected
item count, and issue cues, but page-number and sort-direction bookkeeping
belongs in the table chrome itself rather than competing with the primary scan
path before operators even reach the rows.
That same table density rule also applies to recovery table chrome and filter
rows. Recovery inventory and event tables should use the same restrained
title-case header typography, compact control heights, and thin row density as
the established Pulse monitoring tables instead of drifting into report-style
uppercase headers or oversized filter chrome.
That same protected-items table contract should stay on the canonical shared
table separator treatment used by the rest of Pulse. Recovery inventory should
use the standard shared header/body dividers and avoid both local suppression
of those separators and local duplicate row or header borders that make the
lines read heavier than other monitoring tables.
That same workspace-shell rule should also avoid a dedicated recovery-only
status strip above the control bar. Recovery should use the same handoff shape
as storage: a standalone shared `Subtabs.tsx` row, then any workspace-specific
context like activity, then a shared controls card, then a data card. Recovery
should not collapse controls and content back into one fused workspace slab or
bury the switcher inside the filter row.
That same strip should not repeat page-level counts or posture cues once the
summary already owns those signals. The recovery workspace tabs should stay on
plain canonical labels like `Protected items` and `Recovery events` rather than
embedding per-view totals in the tab text, and the protected-items control row
should stay focused on navigation, drill-in context, and active filters instead
of echoing page-wide posture pills above the same table.
That same workspace handoff should stay on shared primitive styling too. When
recovery renders its workspace tabs, the switcher should use the same canonical
shared `Subtabs.tsx` shell, list, and button class pattern already used by
storage and other established Pulse tab surfaces instead of inventing a
recovery-only variant or recovery-only class stack.
That same workspace rule also means the protected-items versus recovery-events
switcher should live in its own standalone row above the active workspace,
ahead of activity, controls, and data, so the page hands off from summary into
the active recovery surface the same way storage does rather than fusing the
entire workspace into one recovery-only card.
That same canonical-row rule also means the subtabs row should stand on its own
full-width shell instead of sharing a flex line with recovery-only chips or
adjacent badges that break the storage-style border and spacing treatment.
That same shared page-controls contract applies to recovery search width too.
The protected-items and recovery-events workspaces should keep the search field
on the standard full-width shared search row, and any counts or utility cues
should live in the toolbar actions instead of narrowing the search row through
recovery-local grid overrides or width hacks. Protected-items controls should
also use the same shared `Reset all` page-controls action pattern as storage
and workloads when visible filters are active, instead of forcing operators to
clear each inventory filter manually.
That same handoff should keep recovery on the standard Pulse summary density.
`RecoverySummary.tsx` should use the shared default `SummaryPanel` /
`SummaryMetricCard` rhythm that infrastructure and workloads use, instead of
opting into a recovery-only compact density that makes the top strip harder to
scan than the rest of the monitoring product.
That same shell rule applies to the recovery-events workspace.
`RecoveryHistorySection.tsx` should use the same slim status-row-plus-filter-row
pattern as the protected inventory surface, not a separate large titled header
bar plus another full toolbar slab. Event filter labels should also stay on the
canonical short Pulse vocabulary like `Platform` and `Status` instead of
recovery-only variants such as `History platform` or `History status`. Both
recovery toolbars should also stay on compact shared select sizing instead of
inflating the row with recovery-local min-width overrides that make the
controls denser and wider than storage for the same amount of operator input.
That same events-workspace rule should keep the activity strip as orientation
for the event list rather than burying it at the bottom. The events workspace
should move from the subtabs row to `RecoveryActivitySection.tsx`, then shared
controls, then the recovery history table as sibling sections, so the timeline
frames the event list without turning the page back into stacked primary
tables or embedding the activity strip inside the history card.
That same events-shell contract should avoid repeating page-state bookkeeping
ahead of the history grid. Recovery events should keep the toolbar utility area
focused on actual controls like advanced filters and column visibility instead
of passive `day groups` narration; day grouping should stay legible through the
history surface itself, while current page and other table bookkeeping remain
in the table footer instead of competing with the scan path above the filters.
That same activity panel should stay compact and analytical rather than
becoming a second dashboard header. `RecoveryActivitySection.tsx` should keep a
single slim telemetry header, compact active-filter chips, a shorter chart
frame, reduced vertical insets, and a smaller legend footprint so the events
workspace hands off quickly from activity context to the history table instead
of spending a disproportionate slice of the screen on chart chrome. The range
picker and legend should share one compact control row, and the activity strip
should not burn a separate descriptive subtitle row once the headline metrics
already explain the chart context.
That same timeline contract must keep long-range activity fully constrained to
the card width. Extended ranges such as `365d` should compress their day
columns to fit the available plot width instead of carrying per-column minimum
widths that make the chart overflow its containing card.
That telemetry header should also avoid derivative pace rows once the chart
already carries the rhythm. Total points, active days, and issue cues can
stay, but average-per-day style readouts should not re-expand the strip into a
second mini report above the event table.
That same events-table contract should also keep the default column set on a
monitoring-style scan path rather than a report-export path. Recovery events
should default to the concise columns operators need to triage quickly, while
secondary fields such as verification, size, target, and details remain
available through the shared column picker instead of crowding the baseline
grid.
That same event-row scan rule should mirror the protected-items table in the
primary identity cell. `RecoveryHistoryTable.tsx` should lead each event row
with a compact outcome status cue plus the canonical item name, so operators
can scan event health by row without relying only on the far-right outcome
column.
That same density rule should also keep history grouping and badges restrained.
Day-group headers should read as slim dividers instead of banner rows, and
platform/method/outcome pills should stay compact enough that the event grid
still scans like a monitoring table rather than a report export.
That shared unified-resource dependency now also includes policy-governed
resource metadata on the frontend decode path: storage and recovery surfaces
that route through `frontend-modern/src/hooks/useUnifiedResources.ts` must
preserve canonical `policy` and `aiSafeSummary` fields so storage-bearing
resources do not silently lose their routing or redaction posture when they
cross from unified-resource ownership into storage or recovery presentation.
That same decode path now trusts the backend canonical `policy` and
`aiSafeSummary` values directly, so storage and recovery surfaces keep the
canonical summary text aligned with the policy-aware resource contract
instead of reformatting or re-normalizing it locally.
That same shared `internal/api/` dependency now also routes resource-timeline
filters through the unified-resource change parser, so storage and recovery
surfaces do not inherit a second local decoder for `kind`, `sourceType`, or
`sourceAdapter` values.
That same shared `internal/api/` dependency now also assumes the resource
timeline parser is owned by unified resources, so storage and recovery
surfaces rely on one canonical change-filter contract instead of re-decoding
timeline query values in the handler layer.
That same shared `internal/api/` dependency now also assumes canonical install
payload URLs are slash-normalized before they become response fields or helper
attachments, so recovery-adjacent links and transport surfaces cannot inherit
double-slash installer paths from backend public-endpoint configuration.
That same shared `internal/api/` dependency must also preserve the governed
agent-lifecycle shell payload shape when adjacent diagnostics responses expose
install transport: `router.go` may not reintroduce stale lifecycle flag aliases
or raw sudo-only install pipes in container-runtime migration payloads that
share the same backend response surface.
That same shared dependency now also assumes those diagnostics install payloads
route through the canonical backend install-command helper, so recovery-adjacent
transport surfaces do not inherit handler-local drift in token omission,
plain-HTTP `--insecure`, or trailing-slash normalization.
That same shared `internal/api/` dependency also assumes diagnostics memory
source breakdowns backfill canonical fallback reasons even when a raw legacy
snapshot reaches `internal/api/diagnostics.go` without one, so
recovery-adjacent consumers do not observe alias-normalized sources paired
with empty or drifted fallback-reason payloads.
That same shared `internal/api/` dependency now also assumes auth persistence
compatibility stays on an explicit migration/import boundary: legacy
raw-token `sessions.json` and `csrf_tokens.json` files may load for upgrade
continuity, but `session_store.go` and `csrf_store.go` must immediately
rewrite hashed canonical persistence on load so adjacent storage and recovery
transport does not keep running against primary-path raw-token files.
That same shared `internal/api/` dependency also assumes customer-visible
commercial retry guidance stays canonical when storage- or recovery-adjacent
surfaces invoke trial or billing handoffs: the backend must return the real
remaining backoff through `Retry-After` and `details.retry_after_seconds`
instead of leaving neighboring surfaces to guess or hardcode retry windows.
That same shared `internal/api/` dependency now also assumes adjacent
commercial helper surfaces speak in monitored-system terms: recovery- or
storage-adjacent API wiring may consume the canonical monitored-system ledger
and monitored-system cap helpers, but it must not revive deleted agent-era
helper names or imply that API-backed infrastructure sits outside the counted
system model.
That same shared `internal/api/` dependency now also assumes monitored-system
ledger status details stay canonical and source-aware: storage- or recovery-
adjacent consumers may read the ledger’s nested status explanation, but they
must preserve the backend-provided reason list for stale or offline grouped
sources, including the canonical `reported_at` timestamp, instead of reducing
those mixed fresh/stale system states back to a generic label.
That same ledger dependency also treats the canonical `latest_included_signal`
object as the freshest grouped observation. Storage- or recovery-adjacent
consumers must not present that data with bare single-source `Last Seen`
wording that hides grouped stale/offline conditions, and should use the
canonical object when they need attribution for which grouped surface most
recently reported. Retired flat alias fields must not reappear as separate
freshness signals or adjacent contract wording.
That same shared `internal/api/` dependency now also assumes self-hosted
commercial counting is canonical at the top-level monitored-system boundary:
shared setup, deploy, entitlement, and API-backed monitoring helpers may not
preserve an API-only exemption that would let storage- or recovery-adjacent
systems consume no commercial slot when the same monitored system is visible
through canonical unified-resource roots.
That same shared `internal/api/` dependency also assumes session-carried OIDC
refresh tokens stay fail-closed at rest: `session_store.go` may only persist
or recover those tokens through encrypted-at-rest session payloads, and any
missing-crypto or invalid-ciphertext path must drop the refresh token instead
of preserving plaintext-at-rest session state that storage and recovery
surfaces might inherit through shared auth runtime helpers.
That same shared `internal/api/` dependency also assumes notification test
handlers stay decode-and-delegate only: `internal/api/notifications.go` may
share the API helper boundary with storage-adjacent routes, but service-template
selection and generic webhook-test payload fallback must remain
notifications-owned instead of becoming a second API-layer owner.
That same shared API boundary also assumes legacy service-specific webhook
aliases are rewritten at ingress only: `internal/api/notifications.go` may
accept compatibility keys like Pushover `app_token` / `user_token`, but it
must return and forward only canonical `token` / `user` fields so storage-
adjacent shared `internal/api/` helpers do not inherit a second live alias
contract.
That same shared `internal/api/` dependency now also assumes recovery-token
persistence follows the same canonical rule: raw recovery secrets may be minted
for immediate operator use, but `recovery_tokens.go` must persist only token
hashes and treat any legacy plaintext-token file as a one-time migration input
that is rewritten immediately into hashed canonical persistence on load.
That same storage-adjacent persistence rule also applies to
`internal/config/persistence.go` API token metadata: `api_tokens.json` may hold
only hashed token records, but a legacy plaintext metadata file may only be
migration input and must be rewritten immediately into encrypted-at-rest
storage on load instead of staying on the runtime primary path.
That same shared `internal/api/` dependency also assumes those auth stores stay
owned by the configured router data path: session, CSRF, and recovery-token
runtime state may not silently bind to hidden `/etc/pulse` fallback
initialization or leak old-path store contents forward after reconfiguration.
That same path-ownership rule also governs adjacent hosted billing and
bootstrap artifacts that share the `internal/api/` boundary: webhook dedupe
state, customer indexes, and bootstrap-token lookup must resolve their base
directory through the shared runtime data-dir helper instead of carrying
neighboring `/etc/pulse` fallback logic of their own.
That same shared boundary also assumes manual auth env writes and auth-status
reads resolve `.env` through the shared auth-path helper, so storage-adjacent
recovery and setup flows do not keep neighboring `/etc/pulse/.env` fallback
logic alive after the runtime data-dir authority has been centralized.
The same boundary also owns first-session reset cleanup during managed-backend
proof: the dev-only `/api/security/dev/reset-first-run` route must clear auth
env and persisted API-token state through the shared helpers, and adjacent test
or recovery tooling may not delete those files directly.
That same shared `internal/api/` dependency also assumes generated developer
warnings keep the local browser/runtime split accurate: the embedded frontend
notice under `internal/api/DO_NOT_EDIT_FRONTEND_HERE.md` may describe `:7655`
as the proxied backend dependency, but it must preserve
`http://127.0.0.1:5173` as the hot-reload browser entrypoint so storage- and
recovery-adjacent setup guidance does not drift back to the backend port.
That same shared boundary now also owns writable auth-env fallback order, so
storage-adjacent setup and recovery flows may not keep per-handler config-path
write branches with private data-path fallback logic once the shared helper
exists.
That same shared `internal/api/` dependency also assumes bootstrap-token
persistence follows the same boundary discipline: the first-session setup
secret may remain recoverable through the supported `pulse bootstrap-token`
command, but `.bootstrap_token` may not stay a primary-path plaintext secret
file. Canonical runtime persistence must encrypt that token at rest, and any
legacy plaintext bootstrap-token file must be rewritten immediately into the
encrypted canonical format on load.
That same shared `internal/api/` dependency now also assumes Proxmox
setup-command payloads stay on the governed fail-fast shell transport, so
adjacent setup and recovery-linked flows do not inherit stale `curl -sSL`
quick-setup commands from handler-local string assembly.
That same dependency also assumes the generated setup scripts echo the same
fail-fast guidance back to operators during retry and validation failures, so
adjacent setup flows do not preserve stale `curl -sSL` examples after the API
response itself has already moved to the governed transport.
That same shared dependency now also assumes those quick-setup commands and
embedded retry examples preserve root-or-sudo continuity, so adjacent setup
flows do not regress to direct-root-only command guidance on hosts where the
operator enters through a non-root shell.
That same dependency also assumes the embedded retry examples preserve the
active setup token through those root-or-sudo paths, so adjacent setup flows
do not regress from non-interactive reruns back to prompt-only recovery.
That same shared dependency also assumes generated setup scripts hydrate
`PULSE_SETUP_TOKEN` from any embedded setup token before they print rerun
guidance, so adjacent setup flows do not lose non-interactive continuity just
because the next hop entered through a generated script body instead of the
original API command.
That same shared dependency also assumes `/api/setup-script-url` and the
generated rerun guidance draw from one canonical bootstrap artifact builder, so
adjacent setup and recovery-linked transport flows do not drift on download
URLs, script filenames, token hints, or env-wrapped rerun command shape across
the setup bootstrap boundary.
That same shared dependency also assumes generated PVE setup scripts actually
remove the discovered legacy token sets they enumerate during cleanup, so
adjacent operator recovery flows do not present a fake cleanup option that
quietly leaves stale `pve` and `pam` Pulse tokens behind. That same cleanup
dependency also assumes candidate discovery stays on the canonical
Pulse-managed token prefix for the active Pulse URL, so adjacent setup flows
do not drift onto IP-pattern token matching that misses hostname-scoped legacy
tokens. That dependency applies to both generated PVE and PBS setup scripts,
so adjacent setup flows do not fork cleanup discovery rules by node type.
That same shared discovery dependency also assumes runtime discovery state owns
only structured errors, while adjacent API and WebSocket payloads may derive
the deprecated string `errors` list only as a compatibility field from those
canonical structured errors.
That same dependency also assumes rerun token-rotation detection uses exact
managed token-name matches, so adjacent setup flows do not collide with
unrelated partial-name tokens and rotate the wrong state.
That same dependency also assumes generated PBS setup scripts only print the
token-copy banner after a successful token-create result, so adjacent setup
flows do not advertise a non-existent token on failure.
That same dependency also assumes generated PBS setup scripts only print the
auto-register attempt banner on the real request path, so adjacent setup flows
do not claim an in-flight registration attempt on branches that are actually
skipped before any request is sent.
That same shared dependency also assumes generated setup scripts preserve the
canonical encoded rerun URL contract, so adjacent setup flows do not drop the
selected `host`, `pulse_url`, or `backup_perms` state when operators rerun the
embedded quick-setup command from the script body.
That same shared dependency also assumes generated setup scripts fail closed on
auto-register success parsing, so adjacent setup flows do not misreport
registration success when a shared backend response still carries a
`success:false` payload.
That same dependency also assumes those generated setup scripts fail closed on
auto-register HTTP and transport failures, so adjacent setup flows do not
reinterpret shared backend stderr or HTTP-failure output as a successful
registration payload.
That same shared dependency also assumes generated setup scripts preserve
setup-token auth guidance, so adjacent setup flows do not regress back to
stale API-token instructions after the backend has already standardized on the
one-time setup-token contract.
That same dependency also assumes generated setup scripts preserve truthful
registration-outcome messaging, so adjacent setup flows do not claim a node
was successfully registered when the shared backend path actually fell back to
manual completion.
That same dependency also assumes manual completion stays on the canonical
node-add path, so adjacent setup flows do not regress back to a stale
secondary registration-token rerun contract after the backend already emitted
manual token details for Pulse Settings → Nodes.
That same dependency also assumes auth-failure messaging stays truthful once a
shared setup script has already entered the registration-request path, so
adjacent setup flows do not regress into missing-token copy when the real next
step is to fetch a fresh setup token from Pulse Settings → Nodes and rerun.
That same auth-failure state must also suppress the later manual-details
footer, so adjacent setup flows do not contradict the rerun contract.
That same dependency also assumes the auto-register failure summary stays on
that canonical node-add path, so adjacent setup flows do not regress to vague
"manual configuration may be needed" wording once the backend already emitted
the exact Pulse Settings → Nodes completion path.
That same dependency also assumes the immediate failure branch reuses that same
manual-completion contract instead of drifting into a numbered manual-setup
list before the final token-details footer, including request-failure branches
that never receive a parseable backend response.
That same dependency also assumes those manual-add instructions preserve the
canonical node host already known to the script, so adjacent setup flows do
not regress to placeholder host guidance when shared backend continuity is
otherwise intact.
That same dependency also assumes the PBS setup script binds that canonical
host before setup-token gating can skip auto-registration, so adjacent manual
fallback output does not lose the host URL just because setup-token input was
omitted.
That same dependency also assumes the canonical PBS host is already bound
before token-creation failure fallback, so adjacent manual completion output
does not drop the host URL just because token minting failed earlier in the
same shared script.
That same dependency also assumes token-creation failure stays truthful in
those generated setup scripts, so adjacent flows do not regress into fake token
details or a false "token setup completed" state after shared backend token
minting already failed.
That same dependency also assumes token-extraction failure stays on the same
rerun-after-fix path, so adjacent setup flows do not regress into a false
manual-registration fallback when the shared backend still has not produced a
usable token secret, and do not enter the shared manual-completion footer until
that usable secret actually exists.
That same shared setup dependency also assumes skipped PBS auto-register paths
stay truthful, so adjacent flows do not regress into a fake request-failure
banner when the backend intentionally never attempted registration.
That same shared setup dependency also assumes missing-host script payloads
stay fail closed, so adjacent flows do not regress into placeholder manual
registration targets when the backend never received a canonical node URL.
That same dependency also assumes PBS follows the identical host rule, so
adjacent setup flows do not regress from the backend-requested canonical PBS
host to a runtime-local interface address when manual completion is rendered.
That same dependency also assumes those manual-add instructions preserve
canonical Settings → Nodes phrasing across node types, so adjacent setup flows
do not drift into inconsistent manual-completion language for equivalent
fallback paths.
That same dependency also assumes the earlier auto-register failure branch uses
that identical Settings → Nodes destination, so adjacent setup flows do not
observe one manual-completion path in the immediate error guidance and a
different one in the final backend-owned footer.
That same dependency also assumes the off-host PVE fallback stays on the
canonical rerun-on-host contract, so adjacent setup flows do not regress into
a separate manual `pveum` plus Pulse Settings token-entry path that the shared
backend no longer owns.
That same dependency also assumes direct script launches preserve the canonical
root requirement wording, so adjacent setup flows do not regress to a stale
"Please run this script as root" branch while the governed retry transport
already uses the newer privilege guidance.
That same dependency also assumes manual-add token placeholder text stays
canonical across those generated setup branches, so adjacent setup flows do
not surface conflicting "see above" instructions for the same backend-owned
token continuity contract.
That same dependency also assumes successful generated setup flows preserve one
canonical success message across node types, so adjacent setup surfaces do not
drift into type-specific completion wording for the same backend-confirmed
registration state.
That same dependency also assumes token-extraction failures stop before shared
registration assembly, so adjacent setup flows do not proceed with an empty
token secret after the backend already determined the generated token value was
unavailable.
That same dependency also assumes canonical PVE auto-register payloads carry
real caller-supplied token secrets only, so adjacent setup flows do not treat
placeholder response state as a usable credential or persist dead pending
branches into shared node state.
That same shared `internal/api/` dependency also assumes the canonical
`/api/auto-register` success payload keeps canonical node identity in
`nodeId` instead of the raw host URL or requested server name, so adjacent
setup and recovery-linked transport attachments do not fork between stored
node name and request-form identity.
That same dependency also assumes the shared `node_auto_registered` event from
canonical /api/auto-register keeps the normalized stored host and canonical node
identity in its payload, so adjacent transport surfaces do not fork between
saved node state and raw request-form event data.
That same shared dependency also assumes canonical /api/auto-register success
responses mirror that stored identity and normalized host through `type`,
`source`, `host`, `nodeId`, and `nodeName`, so installer and runtime-side
Unified Agent success paths cannot drift into a second local identity after
registration.
That same dependency also assumes the setup-token bootstrap response from
`/api/setup-script-url` carries canonical `type`, normalized `host`, and live
expiry metadata, so adjacent setup and recovery-linked transport surfaces do
not consume a mismatched bootstrap token after host normalization.
That same shared dependency also assumes installer and runtime-side Unified
Agent callers fail closed on already-expired bootstrap responses instead of
treating any populated `expires` field as sufficient.
That same shared dependency also assumes Pulse-managed Proxmox monitor-token
names stay bound to the canonical Pulse endpoint across setup/bootstrap
surfaces, so adjacent setup and recovery-linked flows may not derive token
scope from request-local `Host` fallbacks and accidentally fork monitor-token
identity for the same Pulse instance.
That same shared dependency also assumes `/api/setup-script` stays on one
canonical artifact contract: manual setup downloads must ship as
`text/x-shellscript` attachments with deterministic `pulse-setup-*.sh`
filenames, so adjacent setup and recovery-linked transport surfaces do not
flatten governed script delivery into untyped text blobs.
That same shared dependency also assumes `/api/setup-script-url` carries that
canonical setup-script filename as bootstrap metadata, so adjacent setup and
recovery-linked surfaces do not reintroduce hardcoded local filenames that can
drift from the downloaded artifact.
That same shared dependency also assumes settings quick-setup treats
`/api/setup-script-url` as one canonical bootstrap artifact per active
endpoint, so adjacent setup and recovery-linked surfaces do not fork copy and
manual-download behavior onto separate lane-local bootstrap requests.
That same shared dependency now also assumes that bootstrap artifact is owned
by one shared backend install-artifact model rather than mirrored local
bootstrap structs and response envelopes, so adjacent setup and
recovery-linked surfaces do not inherit drift between downloads, rerun
guidance, and script rendering. Generated PVE/PBS setup-script bodies must also
come from the same shared backend render helpers instead of a handler-local
template engine, so recovery-linked copy and rerun flows do not fork the shell
transport contract by route implementation.
guidance, and the setup-script-url payload.
That same shared dependency also assumes that bootstrap artifact includes a
dedicated token-bearing `downloadURL`, so manual setup-script downloads remain
non-interactive without forcing adjacent surfaces to re-expose the raw setup
token or rebuild a second setup-script request from partial bootstrap state.
That same shared dependency also assumes runtime-side Unified Agent and
installer consumers keep the full setup bootstrap envelope coherent: adjacent
transport surfaces may not silently accept `/api/setup-script-url` responses
that drop canonical script URL, filename, or command metadata while still
returning a token.
That same shared dependency also assumes `/api/setup-script-url` keeps a strict
canonical request shape: adjacent setup and recovery-linked surfaces may not
quietly accept unknown request fields or trailing JSON on that bootstrap route,
because typo-compatible or concatenated payloads would fork the governed setup
artifact contract from the direct handler proofs.
That same shared dependency also assumes bootstrap backup permissions stay on
the canonical PVE-only path: adjacent setup and recovery-linked surfaces may
not accept `backup_perms` / `backupPerms` for PBS and then silently drift onto
an unsupported no-op request contract.
That same shared dependency also assumes both setup routes keep canonical host
identity explicit: adjacent setup and recovery-linked surfaces may not allow
`/api/setup-script` to fall back to placeholder host artifacts after
`/api/setup-script-url` already requires a real normalized `host`.
That same shared dependency also assumes those setup routes share one canonical
type and host-normalization boundary: adjacent setup and recovery-linked
surfaces may not allow `/api/setup-script` to treat unknown `type` values as
implicit PBS requests or emit unnormalized host state after
`/api/setup-script-url` has already normalized the bootstrap node identity.
That same shared dependency also assumes both setup routes keep canonical
Pulse identity explicit: adjacent setup and recovery-linked surfaces may not
allow `/api/setup-script` to rebuild `pulse_url` from request-local origin
state after `/api/setup-script-url` already binds the canonical Pulse URL into
the returned bootstrap artifact.
That same shared dependency also assumes `/api/setup-script` now rejects
missing `pulse_url` input outright, so adjacent setup and recovery-linked
surfaces may not rely on request-local origin fallback once the bootstrap
artifact already carries an explicit canonical Pulse URL.
That same shared dependency also assumes `/api/setup-script` keeps one
bootstrap token name end to end: embedded setup-script bootstrap uses the
canonical `setup_token` query and the rendered script body uses only
`PULSE_SETUP_TOKEN`, so adjacent setup and recovery-linked reruns do not drift
across alias variables or deleted query naming.
That same shared `internal/api/` dependency also assumes canonical /api/auto-register
responses keep `nodeId` on the resolved stored node identity after name
disambiguation, so adjacent setup and recovery-linked transport attachments do
not fork between saved node state and raw requested server names.
That same shared dependency also assumes canonical /api/auto-register triggers the same
canonical post-registration refresh and live event flow as legacy
auto-register, so adjacent transport surfaces do not miss discovery refresh or
canonical node event payloads just because the node entered through the
path.
That same shared `internal/api/` dependency also assumes canonical /api/auto-register
accepts caller-supplied token completion directly on that contract, so
adjacent lifecycle transport stays on one explicit-token registration contract
instead of forking a second completion path.
That same shared `internal/api/` dependency also assumes the primary runtime
ingest surface is the Pulse Unified Agent boundary in
`internal/api/agent_ingest.go` and `internal/api/router*.go`, so adjacent
storage and recovery transport may not depend on `host`-named handler or
router state as if `/api/agents/host/*` were still a first-class API family
instead of a compatibility alias.
That same shared `internal/api/` dependency also assumes the canonical
Unified Agent route family remains the primary auth/management surface:
adjacent storage and recovery-linked transport must treat `/api/agents/agent/*`
as the owned route family, while `/api/agents/host/*` and legacy
`host-agent:*` scope names remain compatibility aliases only.
That same owned route family must also fail closed on ambiguous hostname
lookups: `/api/agents/agent/lookup` may resolve a unique hostname match, but
it must not pick an arbitrary agent when exact, display-name, or short-hostname
matches are duplicated across the live inventory.
That same shared `internal/api/` dependency also assumes adjacent recovery and
storage-linked transport continues to describe those legacy names as
compatibility aliases rather than active product surfaces, so route/auth
guidance does not drift back into “host-agent” ownership language once the
canonical `agent:*` and `/api/agents/agent/*` boundary is set.
That same shared dependency now also assumes generated setup scripts use that
canonical caller-supplied completion path, so adjacent setup and recovery-linked
transport stay on the canonical registration payload shape.
That same shared dependency also assumes /api/auto-register uses one canonical
caller-supplied completion payload: transport must send `tokenId` and
`tokenValue` directly, so adjacent surfaces do not preserve a mode-switch
field or alternate payload gate.
That same shared dependency also assumes one-time setup-token auth uses the
canonical `authToken` request field only, so adjacent transport does not keep a
duplicate `setupCode` payload alias alive after the canonical field is set.
That same shared dependency also assumes the live runtime keeps that
terminology canonical after the contract cleanup: auto-register auth failures
and handler ownership paths must refer to setup tokens rather than preserving
setup-code residue.
That same shared dependency also assumes missing-token requests fail with the
canonical setup-token requirement itself rather than a generic authentication
message, so adjacent transport and setup-linked recovery proof keep the route
narrowed to one-time setup-token auth.
That same shared dependency also assumes canonical field-validation failures
stay specific on `/api/auto-register`: mismatched `tokenId`/`tokenValue` input
may not collapse into generic missing-field output, and other missing
canonical fields must return explicit `Missing required canonical
auto-register fields: ...` guidance.
That same shared dependency also assumes the public `/api/auto-register` route
and the direct canonical handler path keep those validation failures aligned,
so adjacent shared helpers do not inherit diverging missing-field or token-pair
messages from two nearby entry points on the same runtime surface.
That same shared dependency also assumes canonical auto-register callers send
explicit `serverName` identity, so the backend does not recreate node identity
from `host` and drift adjacent shared state onto handler-local fallback rules.
That same shared dependency also assumes overlap and rerun continuity logs stay
on canonical `/api/auto-register` wording, so adjacent shared helpers do not
reintroduce a deleted "secure auto-register" split while describing resolved
host matches, DHCP continuity matches, or in-place token updates.
That same shared dependency also assumes token-completion validation logs stay
on canonical `/api/auto-register` wording, so adjacent shared helpers do not
reintroduce deleted "secure token completion" wording when `tokenId` and
`tokenValue` drift out of sync.
That same shared dependency also assumes hostagent-driven canonical
/api/auto-register requests use that same request-body `authToken` field
for one-time setup-token auth instead of a header-auth fallback or direct
admin-token completion, so adjacent transport and recovery-linked proof do not
preserve parallel authentication paths.
That same shared `internal/api/` dependency also assumes the canonical helper
and proof surface describe one /api/auto-register path instead of a fake
/api/auto-register/secure sibling, so adjacent transport and governed
evidence do not drift onto a route split that the runtime does not actually
expose.
That same filter contract applies to the advanced history facets transport as a
whole: changing node or namespace filters must narrow the facets request too,
so node and namespace option sets cannot drift back to the broader chart window
while the visible history table is already scoped to a smaller recovery slice.
That same narrowing rule now also applies when a timeline day is selected: the
facets request must use the same narrowed day window as the points request so
node and namespace option sets stay coherent with the visible history slice
instead of showing options from the full chart range while the table is already
scoped to a single day.
The recovery timeline drill-down now also treats day selection as a real
history transport boundary: choosing a day in the "Backups By Date" chart must
narrow the point-history request window to that selected local day rather than
only updating local selection chrome while the table remains on the broader
chart window.
That selected-day boundary must also be durable route state: the recovery URL
must preserve the active timeline day so reload, navigation, and shared links
reconstruct the same point-history window instead of silently widening back to
the broader chart range.
That same route continuity rule also applies to the selected chart window
itself: changing the recovery timeline range to `7d`, `90d`, or `1y` must stay
in canonical `/recovery` route state so reload, navigation, and shared links
reconstruct the same rollup and series transport window instead of widening
back to the default `30d` range.

This lane intentionally depends on other governed boundaries instead of
overreaching into them. API transport and payload contract ownership remain in
`api-contracts`, the settings recovery panel remains in `frontend-primitives`,
and canonical resource identity stays in `unified-resources`.

That same shared `internal/api/` resource boundary now also carries governed
policy-aware metadata. Storage and recovery consumers that read canonical
resource payloads must preserve backend-derived `policy` and `aiSafeSummary`
fields for storage, backup, and data-bearing resources instead of rebuilding
their own sensitivity or routing guesses in page-local presentation code. The
shared `frontend-modern/src/hooks/useUnifiedResources.ts` decode path now
trusts the backend canonical metadata directly instead of re-normalizing it
locally, so storage and recovery views see the same policy posture the API
publishes. The same hook and the resource-identity helpers it depends on now
share the canonical trimmed-string utility instead of each surface rebuilding
    its own whitespace cleanup, so storage and recovery identity checks stay
    aligned with the other unified-resource consumers. That same decode path
    also projects Kubernetes cluster identity through the shared cluster-context
    helper, so storage and recovery surfaces see the same canonical cluster
    prefix as the dashboard and unified-resource store instead of rebuilding
    their own fallback. That same boundary now also owns the backend facet-bundle
    route for timeline history and related change counts, so storage and recovery
    surfaces must continue to consume the shared bundle rather than issuing
    separate local resource-detail fetches.
That same shared `internal/api/` dependency now also assumes canonical
security-token lifecycle reads. Storage- and recovery-adjacent consumers of
shared auth/security helpers may inspect token metadata, but they must not
treat a displayed relay pairing token as disposable once canonical metadata
shows `lastUsedAt`. Shared transport mutations must preserve used-token
continuity instead of deleting a credential that an already paired device
still depends on.
That same shared backend helper layer now also owns hosted relay bootstrap
continuity. Storage- and recovery-adjacent consumers may read relay status or
mobile onboarding payloads, but they must not assume hosted tenants need a
manual relay settings write before those reads become valid. When hosted
billing state grants relay, the shared runtime helper must persist the
canonical hosted relay config and keep subsequent reads on that same
machine-owned state instead of letting adjacent surfaces invent their own
fallback bootstrap or disable heuristics.
That same shared `internal/api/` boundary also owns hosted browser-session
precedence for adjacent protected reads. Storage- and recovery-adjacent hosted
surfaces may run without local auth configured, but a valid tenant
`pulse_session` must still authenticate before any API-only token fallback or
the anonymous optional-auth fallback so hosted recovery, onboarding, and
support flows do not silently degrade into unauthenticated state or bearer-
token-only mode after cloud handoff.
That same shared `internal/api/` boundary also owns hosted AI bootstrap
continuity. Storage- and recovery-adjacent hosted flows may surface Patrol-
backed investigation or AI-assisted recovery guidance before an operator has
ever opened AI Settings, so the shared hosted runtime helper must persist the
canonical quickstart-backed `ai.enc` from entitled billing state when no
explicit AI config exists. Adjacent recovery surfaces must not invent their
own "AI disabled until configured" fallback when the hosted runtime already
has enough entitlement proof to bootstrap the machine-owned default.
That same shared settings helper layer must then preserve canonical
org-management privilege for non-default tenant requests. Storage- and
recovery-adjacent hosted flows that reuse settings-bound helpers must allow
the current org owner/admin membership to continue through privileged tenant
routes after cloud handoff instead of requiring a separate configured local
admin identity that hosted tenants do not carry.
That same hosted continuity assumption also applies when operators arrive via
the older direct tenant magic-link path. Recovery- and storage-adjacent hosted
opens that still redirect through `/auth/cloud-handoff` must carry enough
canonical account/role identity for the tenant runtime to repair membership
before protected routes load, not just the newer portal exchange path.
That same adjacent onboarding boundary must also keep the dedicated
relay-mobile bootstrap credential sufficient for QR, deep-link, and
connection-validation reads, so hosted recovery/support flows that hand a
paired device back into onboarding do not need to escalate the mobile token to
the broader settings-read privilege just to fetch the canonical bootstrap
payload.
