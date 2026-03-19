# Unified Resources Contract

## Contract Metadata

```json
{
  "subsystem_id": "unified-resources",
  "lane": "L13",
  "contract_file": "docs/release-control/v6/internal/subsystems/unified-resources.md",
  "status_file": "docs/release-control/v6/internal/status.json",
  "registry_file": "docs/release-control/v6/internal/subsystems/registry.json",
  "dependency_subsystem_ids": []
}
```

## Purpose

Own canonical resource identity, type normalization, typed views, and
cross-source deduplication.

## Canonical Files

1. `internal/unifiedresources/types.go`
2. `internal/unifiedresources/views.go`
3. `internal/unifiedresources/read_state.go`
4. `internal/unifiedresources/adapters.go`
5. `internal/unifiedresources/monitor_adapter.go`
6. `internal/unifiedresources/canonical_identity.go`
7. `internal/unifiedresources/metrics.go`
8. `internal/unifiedresources/metrics_targets.go`
9. `internal/unifiedresources/registry.go`
10. `internal/unifiedresources/resolve.go`
11. `internal/unifiedresources/resolve_context.go`
12. `internal/unifiedresources/resolved_host_set.go`
13. `internal/unifiedresources/snapshot_source_filter.go`
14. `internal/unifiedresources/store.go`
15. `internal/unifiedresources/kubernetes_capabilities.go`
16. `internal/unifiedresources/pbs_rollups.go`
17. `internal/unifiedresources/monitored_systems.go`
18. `internal/unifiedresources/capabilities.go`
19. `internal/unifiedresources/changes.go`
20. `internal/unifiedresources/relationships.go`
21. `internal/unifiedresources/privacy.go`
22. `internal/unifiedresources/actions.go`

## Shared Boundaries

1. `frontend-modern/src/components/Infrastructure/infrastructureSelectors.ts` shared with `performance-and-scalability`: the infrastructure selector pipeline is both a canonical unified-resource consumer surface and a fleet-scale performance hot-path boundary.
2. `frontend-modern/src/components/Infrastructure/resourceDetailMappers.ts` shared with `performance-and-scalability`: resource detail mappers are both a canonical unified-resource consumer surface and a fleet-scale performance hot-path boundary.
3. `frontend-modern/src/components/Infrastructure/UnifiedResourceTable.tsx` shared with `performance-and-scalability`: the unified resource table is both a canonical unified-resource consumer surface and a fleet-scale performance hot-path boundary.
4. `internal/api/resources.go` shared with `api-contracts`: the unified resource endpoint is both a backend payload contract surface and a unified-resource runtime boundary.

## Extension Points

1. Add new resource types and identity fields in `internal/unifiedresources/types.go`
2. Add typed accessors and views in `internal/unifiedresources/views.go`
3. Add source ingestion/adaptation in the adapter layer only
4. Add metrics-target normalization or synthetic metrics support through `internal/unifiedresources/metrics_targets.go` and `internal/unifiedresources/metrics.go`
5. Add platform registry, resolution, or host-dedup behavior through `internal/unifiedresources/registry.go`, `internal/unifiedresources/resolve.go`, `internal/unifiedresources/resolved_host_set.go`, `internal/unifiedresources/snapshot_source_filter.go`, `internal/unifiedresources/store.go`, `internal/unifiedresources/kubernetes_capabilities.go`, and `internal/unifiedresources/pbs_rollups.go`
6. Add canonical governed name-resolution or policy-aware resource lookup behavior through `internal/unifiedresources/resolve.go` and `internal/unifiedresources/resolve_context.go`

## Forbidden Paths

1. New ad hoc resource-type aliases outside unified resource normalization
2. New duplicate ID normalization logic outside unified resources
3. Reintroducing legacy runtime resource contracts as live truth

## Completion Obligations

1. Update this contract when canonical resource identity or type rules change
2. Update contract and guardrail tests when a new resource type is added
3. Route runtime changes through the explicit unified-resource proof policies in `registry.json`; default fallback proof routing is not allowed
4. Tighten banned-path tests when a compatibility bridge is removed

## Current State

This subsystem now sits under the dedicated core monitoring runtime lane so
canonical resource identity, discovery normalization, and platform-runtime
coverage stay governed as a first-class Pulse product surface.

The unified-resource runtime now also owns the durable change timeline for the
canonical resource view. `internal/unifiedresources/monitor_adapter.go` feeds
registry rebuilds and supplemental ingest into `ResourceChange` records, and
`internal/unifiedresources/store.go` persists those changes so `RecentChanges`
can round-trip through the SQLite-backed resource store instead of living only
in memory or adapter-local state.
Timeline records now keep graph context in `relatedResources` for every
meaningful canonical change kind, so the durable history preserves the same
cross-resource context the detail drawer can surface later instead of
collapsing state, restart, anomaly, or config changes down to resource-only
hints.
Those same relationship changes now summarize the actual edge(s) in `from` and
`to`, so the canonical timeline keeps the graph transition readable without
needing the drawer to reconstruct an edge summary from raw endpoints.
The backend AI and Patrol context renderers now derive their canonical change
kind, source type, source adapter, actor, reason, and related-resource
fragments from `internal/unifiedresources/change_presentation.go`, so the
semantic mapping lives with the resource model instead of being duplicated in
lane-local prompt helpers.
The backend AI and Patrol graph context renderers now derive their canonical
relationship labels, direction, provenance, freshness, and metadata flags
from `internal/unifiedresources/relationship_presentation.go`, so the graph
semantics live with the resource model instead of being duplicated in prompt
helpers or drawer-specific markdown.
The same AI resource-intelligence payload now also carries canonical
correlation evidence from the shared detector, so the drawer can show learned
edge patterns alongside the dependency graph without rebuilding correlation
reasoning from raw events. The Patrol intelligence page now also renders that
correlation evidence through the shared
`frontend-modern/src/components/Infrastructure/ResourceGraphSummary.tsx`
card, so the same learned-edge list stays governed by one frontend surface
instead of separate page-local implementations. That shared card also owns
the correlation ordering and truncation rule, so callers pass raw correlation
lists instead of encoding their own sort or top-N behavior.
The same surfaces now also render recent changes through the shared
`frontend-modern/src/components/Infrastructure/ResourceChangeSummary.tsx`
card, so canonical timeline wording and ordering stay governed by one
frontend feed instead of separate page-local loops.
The change emitter now also classifies canonical restart changes for Docker
and Kubernetes resources when restart counters increase or uptime resets, so
the timeline can distinguish restarts from generic state transitions instead
of flattening them into status-only noise.
The same change emitter now also classifies canonical incident changes as
`metric_anomaly` records when the incident rollup changes, so resource
anomalies stay attached to the canonical incident surface instead of being
inferred later from metric noise or alert-adjacent heuristics.
That store also now migrates legacy `resource_changes` tables that still carry
the pre-v6 `timestamp` column by backfilling canonical `observed_at` values,
adding the newer `occurred_at` field, and preserving the legacy timestamp on
write when the target database still requires it.
`internal/api/resources.go` now exposes that same history through dedicated
`/api/resources/{id}/timeline` reads, while `/api/resources/{id}/capabilities`
and `/api/resources/{id}/relationships` expose the current graph facets as
separate queryable surfaces instead of forcing consumers to parse the full
resource payload.
Those filtered timeline reads are backed by dedicated `resource_changes`
indexes on `canonical_id`, `kind`, `source_type`, and `observed_at`, so the
canonical history path stays fast as the filtered timeline grows instead of
falling back to a consumer-local scan.
The frontend now also consumes those facet reads through
`frontend-modern/src/api/resources.ts` and the dedicated resource detail
drawer, which keeps the presentation surface aligned with the governed API
contract instead of rebuilding the graph and timeline inline.
That drawer now also uses a shared frontend relationship-presentation helper
for graph labels and provenance wording, so the UI stays aligned with the
canonical relationship semantics instead of keeping drawer-local token
humanization.
The same facet bundle now also returns grouped recent-change counts by
canonical change kind, so the detail drawer can surface the distribution of
state transitions, restarts, config updates, anomalies, relationships, and
capabilities without recomputing timeline history in the browser.
That same facet bundle now also returns grouped recent-change provenance
counts by source type, so the detail drawer can distinguish platform events,
pulse diffs, heuristics, user actions, and agent actions without re-deriving
adapter provenance from the loaded slice.
That same facet bundle now also returns grouped recent-change adapter counts
by source adapter, so the detail drawer can distinguish Docker, Proxmox,
TrueNAS, and ops-helper provenance without re-deriving integration origin
from the loaded slice.
The same unified resource model now also feeds the canonical AI policy
posture summary, so sensitivity, routing, and redaction distributions stay
derived from the shared resource view instead of being rebuilt as a
page-local governance rollup.
The same shared policy presentation helper now also formats governed mention
policy lines and redaction lists for AI chat prefetch, so prompt context
stays aligned with the canonical sensitivity, routing, and redaction labels
instead of rebuilding them in lane-local helpers.
The same shared policy helper also owns the `aiSafeSummary` decision and
redaction predicates used by AI chat knowledge extraction and resource
context rendering, so governed labels and summary selection stay rooted in
the unified resource policy model instead of being duplicated in chat-local
helpers.

That same shared store now also persists append-only action lifecycle, action
audit, and export audit records, giving the control-plane verbs a durable home
next to the resource timeline instead of leaving those records isolated in
memory-only models.
The in-memory store mirrors the durable audit contract by upserting action
audits on action ID, so tests and runtime callers observe the same current
record state that SQLite persists for the control-plane execution trail.
The enterprise audit API now reads those same unified-resource action and
export records back out, so the durable store is not just a write sink but the
canonical history surface for the control-plane verbs.

The unified resource core is strong and canonical, but monitoring and some
frontend/API consumers are still being tightened around it.

Tenant-scoped API resource seeding now also stays on unified-resource ownership
end to end: `internal/api/resources.go` consumes
`UnifiedResourceSnapshotForTenant` as the canonical tenant registry seed, and
no longer falls back to raw tenant `StateSnapshot` seeding on the live request
path when that unified seed is empty.

The registry proof map now also breaks out metrics-target normalization and
platform-runtime registry support as explicit governed proof routes. Changes to
history-query target shaping, registry resolution, Kubernetes capability
derivation, store/source-filter state, PBS/PMG rollups, or resolved-host
fallback behavior must stay attached to those specific proof policies rather
than disappearing into a generic unified-resource runtime bucket.

Canonical storage metadata now carries runtime `enabled` and `active` flags so
monitoring and API export paths can derive `models.Storage` from unified views
without depending on legacy snapshot ownership.

Canonical Proxmox node metadata now carries node-only boundary fields such as
guest URL, connection health, temperature details, and pending-update metadata
so monitoring can derive `models.Node` from unified views without depending on
legacy snapshot ownership.

Canonical host-agent metadata now carries host-only runtime fields such as CPU
count, load average, machine/report identity, command capability, exclude
patterns, and host I/O rates so monitoring can derive `models.Host` from
unified views without depending on legacy snapshot ownership.

Canonical Docker host metadata now carries Docker-host-only runtime fields such
as display-name identity, CPU/memory sizing, interval/load averages, raw
container membership, and host I/O rates so monitoring can derive
`models.DockerHost` from unified views without depending on legacy snapshot
ownership.

Canonical Proxmox guest metadata now carries workload boundary fields such as
guest OS identity, guest agent version, guest network interfaces, VM disk
status reason, and container OCI/Docker-detection metadata so monitoring can
derive `models.VM` and `models.Container` from unified views without depending
on legacy snapshot ownership.

Canonical PBS metadata now carries full instance boundary payload such as host
and guest URLs, full datastore details, and PBS job arrays so monitoring can
derive `models.PBSInstance` from unified views without depending on legacy
snapshot ownership.

Canonical physical-disk views now expose the full disk identity and SMART
metadata needed by monitoring refresh paths, so physical-disk temperature and
SMART merges can run from unified `ReadState` instead of from snapshot-owned
disk arrays.

Canonical identity now also treats Proxmox-backed infrastructure parents as
node-owned resources first: when an agent resource carries canonical Proxmox
node metadata, `canonicalIdentity.primaryId` must remain stable as
`node:<proxmox-source-id>` even if agent discovery metadata is also present,
so merged node-plus-agent views do not drift to transient agent identifiers.

Frontend/API consumers and backend support files now require explicit registry
path-policy coverage, so new unified-resource-owned runtime files must be added
to a concrete proof route instead of falling back to subsystem-default
verification.

The infrastructure table, selector, and detail-mapper frontend consumers are
now governed as explicit shared boundaries with the performance lane rather
than implicit downstream usage. That means future fleet-scale table changes
must preserve both canonical unified-resource semantics and the table
performance proof route. The shared resource table now also surfaces compact
facet summary chips for capabilities, relationships, and timeline events, so
facet presentation changes must continue to flow through the same governed
resource-row surface rather than inventing a separate ad hoc summary path.
Those row summaries now prefer canonical `facetCounts` on the resource object
when available, so the backend list/read shapes remain the source of truth
instead of forcing the frontend to infer totals only from loaded slices.
Those chips now appear on both the primary fleet rows and the PBS/PMG service
rows, so the unified consumer surface must remain consistent across the full
table instead of diverging by resource class. The same facet summary now also
appears in the resource drawer's runtime overview card through a shared
`ResourceFacetSummary` component, so table and detail views stay aligned on a
single canonical rendering path for the resource graph counts. The drawer now
fetches those facets through one backend bundle endpoint, and that shared
facet bundle preserves backend counts for capabilities, relationships, and the
timeline slice so the overview card and history summary can report the total
facet history instead of collapsing to the currently loaded page when the
timeline endpoint is paginated. Relationship and timeline references in that
drawer now route through the canonical infrastructure resource filter, so the
resource graph remains navigable from the history surface instead of being
purely descriptive text.
`ResourceFacetSummary` now consumes the shared
`frontend-modern/src/utils/resourceChangePresentation.ts` label helper for
canonical change kinds, source types, and adapter provenance, so the chip
wording stays aligned across table, drawer, and intelligence surfaces.
Relationship cards in that drawer also surface `lastSeenAt` freshness and
optional metadata blocks, and timeline cards surface change metadata when it
is present, so the graph history view preserves the richer provenance already
carried by the unified-resource model instead of flattening those fields away.
The same Infrastructure resource-only links now also default through the
shared `frontend-modern/src/components/Infrastructure/ResourceChangeSummary.tsx`
and `frontend-modern/src/components/Infrastructure/ResourceGraphSummary.tsx`
cards from the Patrol page, resource drawer, and problem-resource dashboard
panels, so canonical resource-filter path construction stays owned by the
shared summary cards rather than being duplicated per surface.
The same timeline and facet-bundle reads now also accept governed `kind` and
`sourceType` filters, plus a governed `sourceAdapter` filter for adapter-level
provenance drill-down, so history can narrow by canonical change class and
integration source while the store still owns the filtered total counts.
Invalid `sourceAdapter` values are rejected at the API boundary, keeping the
timeline query contract aligned with the canonical adapter set instead of
silently accepting arbitrary strings.
The Connected infrastructure settings surface now also depends on a backend
owned `connectedInfrastructure` projection derived from unified resources plus
reporting-ignore state. That projection is now also the only v6 client
contract for reporting/ignored infrastructure state; future settings-row
grouping or reporting-surface scope changes must be routed through that backend
projection instead of teaching the frontend to reinterpret raw resource facets
or removed-runtime arrays locally.
Canonical route helpers must also preserve recovery-specific drill-down state
when they serialize governed resource views. Recovery timeline day selection is
part of the durable route contract, so `/recovery` links must round-trip the
selected day instead of dropping it as transient local UI state.
The same recovery route contract also applies to the selected timeline range:
canonical `/recovery` links must preserve explicit non-default chart windows
such as `7d`, `90d`, and `1y` so recovery drill-down transport does not widen
back to the default `30d` window on reload or shared navigation.
That same shared recovery route helper contract now also owns canonical
boolean filter encoding for protected-inventory drill-down state. Visible
recovery toggles such as `stale` must round-trip through the owned `stale=1`
query form instead of leaking ad hoc truthy strings or disappearing from
shared links on reload.
Shared API consumers now also depend on a single registry-list snapshot per
request when deriving canonical type aggregations for resource list and stats
responses. Re-reading `registry.List()` for the same `/api/resources` request
is forbidden because it adds avoidable clone churn to the hot path and breaks
the guarantee that aggregations describe the exact resource snapshot used to
build the response.

Canonical parent lineage is now source-tracked internal state, not sticky
merged payload state. `parentId`, `parentName`, and `childCount` must be
re-derived from the live per-source parent map on every ingest/build pass so
same-source reparenting, orphaning, and parent removal clear old lineage
instead of leaking stale topology into API and typed-view consumers.
The registry now also owns the canonical monitored-system projection used by
commercial entitlement and ledger surfaces. `MonitoredSystems(...)` must keep
top-level counted-system identity, representative resource selection, and
agent/API deduplication on unified-resource ownership instead of letting API
handlers or licensing helpers rebuild their own counted-system grouping logic.

Canonical source-owned identifiers must also normalize surrounding whitespace
before they become by-source map keys or source-specific hash IDs. The same
runtime object must not fork into distinct canonical resources just because an
ingest path supplied `"vm-100"` in one pass and `" vm-100 "` in another.

Canonical target-derived identities must also normalize resource-type aliases
through the v6 type map before they become `primaryId` or alias entries.
Mixed-case or compatibility target values such as `HOST` and `docker_host`
must collapse to canonical v6 prefixes instead of leaking raw source labels
into merged resource identity.

Canonical metrics targets must also trim source-owned target IDs before they
become query coordinates. The same resource must not query different history
series just because one ingest path emitted `" host-1 "` and another emitted
`"host-1"`.
If the canonicalized target ID is empty after that normalization, the metrics
target must fail closed to `nil` instead of emitting an empty query coordinate.

ReadState resource-resolution lookups must also normalize surrounding
whitespace on the incoming name before matching canonical resources. A valid
resource must not look missing just because a consumer asked for `" myserver "`
instead of `"myserver"`.
The same governed lookup boundary now also owns policy-aware resolved context:
downstream consumers that need routing plus canonical policy metadata must use
the unified-resource resolution context instead of rescanning typed views or
re-deriving AI redaction rules locally.

Typed view accessors for linked topology IDs must also return canonical
trimmed values. Callers must not observe `" node-99 "` or `" agent-123 "`
through host/node view accessors when the canonical linkage is `node-99` or
`agent-123`.
The same rule applies to source-owned typed-view IDs. VM, container, node,
storage, and docker host/container source-ID accessors must return canonical
trimmed identifiers rather than leaking outer whitespace from ingest payloads.
Proxmox topology coordinates exposed through typed views must also be trimmed
before they reach consumers. Node, cluster, and instance accessors must not
present `" pve-a "` or `" lab "` as distinct topology values from `pve-a`
and `lab`.

Frontend unified-resource consumers must now also normalize legacy discovery
resource type aliases before storing `discoveryTarget`. Backend `k8s`
discovery coordinates collapse to the canonical frontend `pod` target, and
typed PBS/storage facets must be preserved as the explicit frontend resource
meta interfaces instead of floating as untyped platform-data consumers.

Resolved host deduplication must also fail safe on unfamiliar connector types.
Unknown source types may contribute identity and source-label evidence, but
they must not outrank the known canonical primary-type order when a merged host
contains a governed connector such as `proxmox-pve` or `agent`.

Infrastructure selector consumers must also preserve the canonical
`KnownSourcePlatform` normalization boundary when collecting source filters and
status facets. The selector layer may accept arbitrary user-visible filter
strings, but it must not widen the canonical unified-resource source/status
contracts that feed the infrastructure table and workload links.

The same source-filter boundary now also applies to infrastructure filter UI
options: `frontend-modern/src/pages/Infrastructure.tsx` may render friendly
string keys, but membership checks against available sources must normalize
through the canonical selector helper before consulting `KnownSourcePlatform`
sets.
Canonical monitored-system counting now also depends on this subsystem. The
counted commercial unit is a deduped top-level monitored system assembled from
canonical unified-resource roots, so read-state helpers that derive
commercial-count groups must union agent, Proxmox, Docker, PBS, PMG, TrueNAS,
and Kubernetes cluster views through canonical identity evidence instead of
through transport-local counters or child-resource totals.

Canonical unified resources now also own first-class policy metadata for the
v6 bridge release. Cloned and API-exported resources must carry
`policy.sensitivity`, `policy.routing`, and `aiSafeSummary` derived from the
canonical resource model itself, with routing scopes constrained to the owned
`cloud-summary`, `local-first`, and `local-only` contract plus explicit
redaction hints for hostname, IP, platform-identity, alias, and path-bearing
surfaces. Downstream API, AI, and frontend consumers may read those fields,
but they must not replace them with local sensitivity inference or ad hoc
privacy heuristics.
The AI runtime now also uses the canonical policy presentation helpers to
surface those routing and redaction labels in shared context output, so the
same policy model is reflected in prompt summaries instead of being
re-described independently per surface.
Those helpers now own the canonical redaction-hint order and count-to-label
projection, so the AI summary and any other backend policy posture surface do
not re-sort redaction labels locally.
They also own the canonical sensitivity and routing order used to format
policy-posture count summaries with human-readable labels, so the AI summary
and frontend policy card both read the same presentation sequence from the
shared resource model.
Canonical resources now carry first-class graph-expansion fields: `Capabilities`
(bounded action definitions with approval levels), `Relationships` (typed
inter-resource links with direction and confidence), and `RecentChanges` (typed
change timeline entries with source, confidence, and related-resource
references). These fields are defined in `capabilities.go`, `relationships.go`,
`changes.go`, `privacy.go`, and `actions.go`. The store now also owns a
`resource_changes` persistence table with `RecordChange` and `GetRecentChanges`
methods so change history is queryable by canonical ID and time window.

That frontend consumer rule now applies on the canonical decode path too:
`frontend-modern/src/hooks/useUnifiedResources.ts` must preserve backend-owned
policy metadata and AI-safe summaries as first-class `Resource` fields, and
shared infrastructure consumers such as the unified resource table and detail
drawer must present that owned metadata through shared helpers instead of
reconstructing privacy posture from display names, source types, or other
incidental runtime hints.
