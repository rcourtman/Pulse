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
7. `internal/unifiedresources/policy_metadata.go`
8. `internal/unifiedresources/metrics.go`
9. `internal/unifiedresources/metrics_targets.go`
10. `internal/unifiedresources/registry.go`
11. `internal/unifiedresources/resolve.go`
12. `internal/unifiedresources/resolve_context.go`
13. `internal/unifiedresources/resolved_host_set.go`
14. `internal/unifiedresources/snapshot_source_filter.go`
15. `internal/unifiedresources/store.go`
16. `internal/unifiedresources/kubernetes_capabilities.go`
17. `internal/unifiedresources/pbs_rollups.go`
18. `internal/unifiedresources/monitored_systems.go`
19. `internal/unifiedresources/top_level_systems.go`
20. `internal/unifiedresources/capabilities.go`
21. `internal/unifiedresources/changes.go`
22. `internal/unifiedresources/relationships.go`
23. `internal/unifiedresources/privacy.go`
24. `internal/unifiedresources/actions.go`
25. `frontend-modern/src/components/Infrastructure/ResourceDetailDrawer.tsx`
26. `frontend-modern/src/components/Infrastructure/ResourceDetailDrawerOverviewTab.tsx`
27. `frontend-modern/src/components/Infrastructure/ResourceDetailDrawerDebugTab.tsx`
28. `frontend-modern/src/components/Infrastructure/ResourceDetailDrawerSupportDisclosure.tsx`
29. `frontend-modern/src/components/Infrastructure/ResourceFacetSummary.tsx`
30. `frontend-modern/src/components/Infrastructure/ResourceChangeSummary.tsx`
31. `frontend-modern/src/components/Infrastructure/ResourceCorrelationSummary.tsx`
32. `frontend-modern/src/components/Infrastructure/ResourcePolicySummary.tsx`
33. `frontend-modern/src/components/Infrastructure/resourceBadges.ts`
34. `frontend-modern/src/components/Infrastructure/UnifiedResourceHostTableCard.tsx`
35. `frontend-modern/src/components/Infrastructure/UnifiedResourcePBSTableSection.tsx`
36. `frontend-modern/src/components/Infrastructure/UnifiedResourcePMGTableSection.tsx`
37. `frontend-modern/src/components/Infrastructure/UnifiedResourceServiceInfrastructureCard.tsx`
38. `frontend-modern/src/components/Infrastructure/unifiedResourceTableModel.ts`
39. `frontend-modern/src/components/Infrastructure/unifiedResourceTableStateModel.ts`
40. `frontend-modern/src/components/Infrastructure/useResourceDetailDrawerDerivedState.ts`
41. `frontend-modern/src/components/Infrastructure/resourceDetailDrawerServiceModel.ts`
42. `frontend-modern/src/components/Infrastructure/resourceDetailDiscoveryModel.ts`
43. `frontend-modern/src/components/Infrastructure/resourceDetailDrawerOperationalModel.ts`
44. `frontend-modern/src/components/Infrastructure/useResourceDetailDrawerHistoryState.ts`
45. `frontend-modern/src/components/Infrastructure/useResourceDetailDrawerDockerActionsState.ts`
46. `frontend-modern/src/components/Infrastructure/useResourceDetailDrawerState.ts`
47. `frontend-modern/src/components/Infrastructure/useUnifiedResourceTableState.ts`
48. `frontend-modern/src/components/Infrastructure/useUnifiedResourceTableViewportSync.ts`
49. `frontend-modern/src/components/Discovery/DiscoveryTab.tsx`
50. `frontend-modern/src/components/Discovery/useDiscoveryTabState.ts`
51. `frontend-modern/src/features/infrastructure/InfrastructurePageSurface.tsx`
52. `frontend-modern/src/features/infrastructure/useInfrastructurePageRouteState.ts`
53. `frontend-modern/src/features/infrastructure/useInfrastructurePageState.ts`
54. `frontend-modern/src/features/infrastructure/infrastructurePageModel.ts`
55. `frontend-modern/src/components/Infrastructure/InfrastructureSummary.tsx`
56. `frontend-modern/src/components/Infrastructure/useInfrastructureSummaryState.ts`
57. `frontend-modern/src/components/Infrastructure/infrastructureSummaryModel.ts`
58. `frontend-modern/src/utils/agentResources.ts`
59. `frontend-modern/src/utils/canonicalResourceTypes.ts`
60. `frontend-modern/src/utils/resourceBadgePresentation.ts`
61. `frontend-modern/src/utils/resourceChangePresentation.ts`
62. `frontend-modern/src/utils/resourceCorrelationPresentation.ts`
63. `frontend-modern/src/utils/resourcePlatformData.ts`
64. `frontend-modern/src/utils/resourcePolicyPresentation.ts`
65. `frontend-modern/src/utils/resourceStateAdapters.ts`
66. `frontend-modern/src/utils/resourceTypeCompat.ts`
67. `frontend-modern/src/utils/resourceTypePresentation.ts`
68. `frontend-modern/src/utils/resourceIdentity.ts`
69. `frontend-modern/src/components/Infrastructure/resourceDetailDrawerIdentityModel.ts`
70. `frontend-modern/src/hooks/useDashboardTrends.ts`

## Shared Boundaries

1. `frontend-modern/src/components/Infrastructure/infrastructureSelectors.ts` shared with `performance-and-scalability`: the infrastructure selector pipeline is both a canonical unified-resource consumer surface and a fleet-scale performance hot-path boundary.
2. `frontend-modern/src/components/Infrastructure/InfrastructureSummary.tsx` shared with `performance-and-scalability`: the infrastructure summary surface is both a canonical unified-resource consumer and a fleet-scale summary chart hot-path boundary.
3. `frontend-modern/src/components/Infrastructure/infrastructureSummaryModel.ts` shared with `performance-and-scalability`: infrastructure summary chart matching, focused-summary view derivation, and metric-series shaping are both a canonical unified-resource consumer surface and a fleet-scale summary chart hot-path boundary.
4. `frontend-modern/src/components/Infrastructure/resourceDetailMappers.ts` shared with `performance-and-scalability`: resource detail mappers are both a canonical unified-resource consumer surface and a fleet-scale performance hot-path boundary.
5. `frontend-modern/src/components/Infrastructure/UnifiedResourceHostTableCard.tsx` shared with `performance-and-scalability`: the unified resource host table card is both a canonical unified-resource consumer surface and a fleet-scale performance hot-path boundary.
6. `frontend-modern/src/components/Infrastructure/UnifiedResourcePBSTableSection.tsx` shared with `performance-and-scalability`: the unified resource PBS section is both a canonical unified-resource consumer surface and a fleet-scale performance hot-path boundary.
7. `frontend-modern/src/components/Infrastructure/UnifiedResourcePMGTableSection.tsx` shared with `performance-and-scalability`: the unified resource PMG section is both a canonical unified-resource consumer surface and a fleet-scale performance hot-path boundary.
8. `frontend-modern/src/components/Infrastructure/UnifiedResourceServiceInfrastructureCard.tsx` shared with `performance-and-scalability`: the unified resource service infrastructure card is both a canonical unified-resource consumer surface and a fleet-scale performance hot-path boundary.
9. `frontend-modern/src/components/Infrastructure/UnifiedResourceTable.tsx` shared with `performance-and-scalability`: the unified resource table is both a canonical unified-resource consumer surface and a fleet-scale performance hot-path boundary.
10. `frontend-modern/src/components/Infrastructure/unifiedResourceTableModel.ts` shared with `performance-and-scalability`: unified resource service row shaping and I/O emphasis are both a canonical unified-resource consumer surface and a fleet-scale performance hot-path boundary.
11. `frontend-modern/src/components/Infrastructure/unifiedResourceTableStateModel.ts` shared with `performance-and-scalability`: unified resource table state derivation, sort-cycle policy, service sorting, and responsive column layout are both a canonical unified-resource consumer surface and a fleet-scale performance hot-path boundary.
12. `frontend-modern/src/components/Infrastructure/useInfrastructureSummaryState.ts` shared with `performance-and-scalability`: infrastructure summary chart polling, cache hydration, and summary-state orchestration are both a canonical unified-resource consumer surface and a fleet-scale summary chart hot-path boundary.
13. `frontend-modern/src/components/Infrastructure/useUnifiedResourceTableState.ts` shared with `performance-and-scalability`: unified resource table state, grouping, and windowing are both a canonical unified-resource consumer surface and a fleet-scale performance hot-path boundary.
14. `frontend-modern/src/components/Infrastructure/useUnifiedResourceTableViewportSync.ts` shared with `performance-and-scalability`: unified resource table viewport sync and selected-row reveal are both a canonical unified-resource consumer surface and a fleet-scale performance hot-path boundary.
15. `internal/api/resources.go` shared with `api-contracts`: the unified resource endpoint is both a backend payload contract surface and a unified-resource runtime boundary.

## Extension Points

1. Add new resource types and identity fields in `internal/unifiedresources/types.go`
2. Add typed accessors and views in `internal/unifiedresources/views.go`
3. Add source ingestion/adaptation in the adapter layer only

Resource detail mappers now reuse the shared
`frontend-modern/src/utils/textPresentation.ts` title-case helper for sensor
labels so the canonical unified-resource presentation layer owns the wording.

The canonical AI-safe summary builder now owns the sensitivity-specific suffix
phrases for `sensitive` and `restricted` resources, so the backend policy
contract controls those strings instead of duplicating them inside the summary
assembly branch.
4. Add metrics-target normalization or synthetic metrics support through `internal/unifiedresources/metrics_targets.go` and `internal/unifiedresources/metrics.go`
5. Add platform registry, resolution, or host-dedup behavior through `internal/unifiedresources/registry.go`, `internal/unifiedresources/resolve.go`, `internal/unifiedresources/resolved_host_set.go`, `internal/unifiedresources/snapshot_source_filter.go`, `internal/unifiedresources/store.go`, `internal/unifiedresources/kubernetes_capabilities.go`, and `internal/unifiedresources/pbs_rollups.go`
6. Add canonical governed name-resolution or policy-aware resource lookup behavior through `internal/unifiedresources/resolve.go` and `internal/unifiedresources/resolve_context.go`
7. Add or change resource drawer timeline/facet presentation through `frontend-modern/src/components/Infrastructure/ResourceDetailDrawer.tsx`, `frontend-modern/src/components/Infrastructure/ResourceDetailDrawerOverviewTab.tsx`, `frontend-modern/src/components/Infrastructure/ResourceDetailDrawerDebugTab.tsx`, `frontend-modern/src/components/Infrastructure/useResourceDetailDrawerState.ts`, `frontend-modern/src/components/Infrastructure/ResourceFacetSummary.tsx`, `frontend-modern/src/components/Infrastructure/resourceDetailMappers.ts`, and the governed `internal/api/resources.go` facet/timeline contract together
8. Add or change discovery-support runtime under the resource drawer through `frontend-modern/src/components/Discovery/DiscoveryTab.tsx` for shell/presentation ownership and `frontend-modern/src/components/Discovery/useDiscoveryTabState.ts` for fetch, websocket-progress, and notes-mutation ownership

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
The canonical resource timeline now also owns durable incident-response facts
that materially changed resource investigation state. `ResourceChange` kinds
such as `alert_fired`, `alert_acknowledged`, `alert_unacknowledged`,
`alert_resolved`, `command_executed`, and `runbook_executed` are part of the
canonical history contract, not optional AI-local annotations. Alert-scoped
incident memory may still project those events for one investigation thread,
but the durable source of truth for resource-affecting alert lifecycle and
remediation activity now belongs to unified-resource history keyed by
canonical resource ID.
The canonical monitor adapter therefore also serves read-side incident
projection support: when a consumer needs an alert timeline, the incident view
should read canonical resource changes back through the attached timeline store
instead of rebuilding another durable event history beside `ResourceChange`.
Kubernetes node identity enrichment remains intentionally hostname-based when
the API cannot supply machine-level identity signals; the code may borrow
machine-id and MAC evidence from a uniquely matched host agent, but duplicate
short hostnames across domains or clusters stay an unresolved ambiguity rather
than being force-merged into a stronger canonical identity.
The canonical monitored-system and connected-infrastructure grouping contract
now lives in `internal/unifiedresources/top_level_systems.go`. Top-level
system identity may merge on strong runtime identity such as machine IDs,
agent IDs, connector-stable primary IDs, or the existing high-confidence
identity matcher, and it may use exact-host attachment only as a bounded
fallback onto a uniquely better existing surface. Friendly display names are
presentation-only and must not participate in monitored-system counting.
URL-backed host fields must be normalized down to canonical hostnames before
they participate in exact-host fallback attachment, and Kubernetes cluster
ownership metadata such as `AgentID` must not collapse a cluster into the
underlying host's monitored-system identity. The canonical resolver coverage
is pinned by an explicit top-level source matrix and mixed-environment
characterization tests so new top-level sources cannot quietly bypass the
counting contract.
That same resolver now also owns monitored-system grouping explanations. When
one counted system includes multiple top-level collection paths, the resolver
must record the actual canonical merge evidence it used, expose sanitized
grouping reasons plus included top-level surfaces, and fall back to an
explicit standalone explanation when no cross-source merge occurred. Support
and billing surfaces must consume that shared explanation contract instead of
reconstructing count reasons from API-local heuristics.
That same monitored-system contract now also owns canonical runtime-status
explanations. When a grouped monitored system resolves to warning, offline, or
unknown, unified resources must expose the shared summary plus structured
degraded-status reasons derived from the grouped top-level resources and their
source freshness state, including which source or surface degraded and the
canonical degraded-signal `reported_at` timestamp. Billing and support
surfaces must consume that shared reason list instead of trying to infer why a
fresh overall `last_seen` can still coincide with warning status.
That same status contract must choose the canonical monitored-system runtime
status from the actual grouped top-level resources rather than from an
implicit `unknown` baseline. Severity ordering is canonical: `offline`
outranks `warning`, `warning` outranks `unknown`, and `unknown` outranks
`online`, so all-online groups resolve online while any grouped offline view
still wins over merely stale or warning surfaces.
That shared summary must also explain mixed-source freshness directly when one
grouped source reported more recently than the degraded one, so consumers do
not present a fresh `Last Seen` timestamp beside warning or offline state
without the canonical explanation of which grouped source is still reporting
and which one drifted stale or disconnected.
That same monitored-system contract now also owns the structured freshest-
signal model. Unified resources must expose the latest included grouped signal
as one canonical object carrying its timestamp, source, display name, and type
so consumers can say exactly which top-level grouped surface most recently
reported instead of reconstructing attribution from separate fields.

The unified-resource runtime now also owns the durable change timeline for the
canonical resource view. `internal/unifiedresources/monitor_adapter.go` feeds
registry rebuilds and supplemental ingest into `ResourceChange` records, and
`internal/unifiedresources/store.go` persists those changes so `RecentChanges`
can round-trip through the SQLite-backed resource store instead of living only
in memory or adapter-local state.
Timeline records now keep correlation context in `relatedResources` for every
meaningful canonical change kind, so the durable history preserves the same
cross-resource context the detail drawer can surface later instead of
collapsing state, restart, anomaly, or config changes down to resource-only
hints.
Those same relationship changes now summarize the actual edge(s) in `from` and
`to`, so the canonical timeline keeps the relationship transition readable without
needing the drawer to reconstruct an edge summary from raw endpoints.
The infrastructure resource drawer now follows the explicit shell/state/render
split used elsewhere in v6: `ResourceDetailDrawer.tsx` owns composition,
`useResourceDetailDrawerState.ts` owns composition of drawer-local state,
`useResourceDetailDrawerHistoryState.ts` owns facet/intelligence/timeline
runtime orchestration, `useResourceDetailDrawerDerivedState.ts` owns the
canonical drawer derivation layer, and
`resourceDetailDiscoveryModel.ts` owns the pure canonical discovery-config
derivation that feeds the drawer discovery surface,
`resourceDetailDrawerOperationalModel.ts` owns the pure source-health,
platform-signal, related-link, and host-detail overview derivations that feed
the current-state and host-details surfaces,
and `useResourceDetailDrawerDerivedState.ts` now composes those operational
overview derivations through that canonical model owner instead of rebuilding
Kubernetes capability badges, source health, related links, and host-detail
coverage inline,
and the current-state summary now leaves normal source provenance to the
canonical header badges while only surfacing a `Sources` row when source
health is degraded or unhealthy,
and it no longer repeats that same provenance as a separate `Mode` row because
the drawer header badges already own canonical source display,
and the drawer header no longer carries the technical primary identity line;
that canonical identifier now lives in the `Identity` card as `Primary ID`,
`resourceDetailDrawerServiceModel.ts` owns the pure Docker/PBS/PMG service
summary and breakdown derivations that feed the overview service-details
surface,
`resourceDetailDrawerIdentityModel.ts` owns the pure identity-card,
discovery-summary, source-debug, and debug-bundle derivations that feed the
overview and debug drawer surfaces,
`useResourceDetailDrawerDockerActionsState.ts` owns Docker action runtime, and
the overview/debug render-heavy surfaces live in dedicated drawer-local owners
instead of staying inline in the shell.
The infrastructure summary surface now follows the same shell/runtime/model
shape: `InfrastructureSummary.tsx` is the render shell,
`useInfrastructureSummaryState.ts` owns chart polling, cache hydration,
org-scope lifecycle, and focused-summary state, and
`infrastructureSummaryModel.ts` owns chart matching, focused-summary display
selection, empty-state wording, and summary-series/metric derivation.
The dashboard overview trend hook now follows that same canonical consumer
contract for infrastructure sparklines: `frontend-modern/src/hooks/useDashboardTrends.ts`
must consume the infrastructure summary chart cache and shared unified-resource
series matching logic instead of issuing bespoke per-resource
`/api/metrics-store/history` fetches for top-CPU and top-memory cards. That
keeps dashboard summary sparklines aligned with canonical resource identity
matching, agent-facet fallback behavior, and first-sample empty-state semantics
already owned by the infrastructure summary surface.
The same contract now also treats dashboard top-card sparkline hydration as a
summary projection concern, not a per-resource history concern: top-CPU and
top-memory selections may still come from dashboard overview ranking, but the
series attached to those selections must be resolved through the same
resource-to-chart matching path used by infrastructure summary cards so agent
fallback and canonical identity aliases do not drift between the two surfaces.
The backend AI and Patrol context renderers now derive their canonical change
kind, source type, source adapter, actor, reason, and related-resource
fragments from `internal/unifiedresources/change_presentation.go`, so the
semantic mapping lives with the resource model instead of being duplicated in
lane-local prompt helpers.
That same presenter now also owns the resource state, restart, incident, and
config summary fragments used by change emission, so the human-readable
timeline values stay canonical even before they are wrapped into a full change
summary.
That same change-presentation helper now also owns the one-line
`FormatResourceChangeSummary` used by AI runtime recent-change sections and
Patrol seed context, so the change wording itself stays canonical before any
section-specific headings are applied.
The same helper also owns `FormatResourceRecentChangesContext`, so AI runtime
callers share the canonical recent-change section heading and resource
prefixing instead of rebuilding that wrapper locally.
The patrol-local memory conversion helpers also live in
`internal/ai/memory/presentation.go`, so the canonical change timeline and
the Patrol memory fallback both translate through the same adapter boundary
instead of carrying duplicate mapping code.
The canonical policy-posture aggregate now also lives in
`internal/unifiedresources/policy_posture.go`, so AI summaries and policy-aware
prompt context share the same sensitivity, routing, and redaction counts from
the resource model instead of rebuilding governance posture in AI-local code.
The same policy presenter now also owns the routing-scope labels used across
AI-facing policy surfaces, while the resource detail drawer stays on
per-resource policy lines instead of reconstructing a separate
`Allowed`/`Blocked` row or `Cloud Summary` decision row locally.
The infrastructure host-table shell now treats the default
`Internal` + `Cloud Summary` posture as canonical policy metadata that should
stay available in the drawer and AI/governance surfaces without being promoted
to always-on row chrome. Inline row badges are reserved for non-default policy
states so the canonical resource surface does not imply that every host carries
an operator-actionable governance exception.
The resource drawer now applies the same rule to its investigation-context
governance block: the default posture remains part of the canonical policy
contract, but the drawer only surfaces the governance section when the policy
state is non-default or otherwise consequential to operator understanding.
The drawer investigation summary line also follows that boundary now: default
`Cloud Summary` routing is not repeated in the collapsed summary text, while
non-default routing still appears when it materially changes how the operator
should read the resource.
The drawer now also suppresses the investigation-context section entirely when
Patrol returns only generic baseline health with no notes, changes,
correlations, dependencies, or other non-default governance signal. The
canonical resource surface should not advertise AI context unless there is
actual investigative value to show.
The shared routing policy itself now stays intentionally minimal: it carries
only the routing scope and the redaction hints derived from canonical
sensitivity, and the cloud-summary decision is derived from that scope
instead of being stored as a second boolean in the owned resource-policy
contract.
The canonical policy summary formatter also mirrors that shape now: it reports
the sensitivity and routing scope directly, then lists redactions, so
governed mention blocks no longer repeat a derived cloud-summary boolean in
their owned summary line.
The backend AI and Patrol correlation context renderers now derive their canonical
relationship labels, direction, provenance, freshness, and metadata flags
from `internal/unifiedresources/relationship_presentation.go`, so the correlation
semantics live with the resource model instead of being duplicated in prompt
helpers or drawer-specific markdown.
That same resource model now also owns the canonical
`FormatResourceRelationshipContext` helper, so service-layer callers only
resolve the resource and hand the model the relationship list instead of
rebuilding the relationship section header, ordering, or freshness wording
locally.
The same shared relationship presenter also owns the compact change-timeline
relationship summary used by resource change records, so change `from` and
`to` values stay aligned with the canonical relationship labels instead of
reconstructing a separate type-token summary in the emitter.
The same AI resource-intelligence payload now also carries canonical
correlation evidence from the shared detector, so the drawer can show learned
edge patterns alongside the dependency relationships without rebuilding correlation
reasoning from raw events. The Patrol intelligence page now also renders that
correlation evidence through the shared
`frontend-modern/src/components/Infrastructure/ResourceCorrelationSummary.tsx`
card, so the same learned-edge list stays governed by one frontend surface
instead of separate page-local implementations. That shared card also owns
the correlation ordering and truncation rule, so callers pass raw correlation
lists instead of encoding their own sort or top-N behavior.
The same surfaces now also render recent changes through the shared
`frontend-modern/src/components/Infrastructure/ResourceChangeSummary.tsx`
card, so canonical timeline wording and ordering stay governed by one
frontend feed instead of separate page-local loops.
The unified-resource detail drawer now also routes resource-tag presentation
through the shared `frontend-modern/src/components/shared/TagBadges.tsx`
primitive instead of importing a dashboard-local badge helper into the
resource surface. Future drawer tag presentation changes must extend through
that shared primitive boundary rather than recreating tag-dot logic inside the
drawer or pulling feature-local badge components across subsystem lines.
The same shared agent-resource module now also owns the canonical cluster-name
helpers, so Kubernetes context prefixes, Proxmox cluster labels, and
cluster-name fetch keys stay aligned instead of each surface rebuilding its
own pod, namespace, and VM routing fallbacks.
The shared node-state adapter also routes Proxmox cluster labels through that
same helper, so infrastructure summary projections keep the same canonical
cluster name as the rest of the unified resource model instead of rewriting
the label locally.
The unified resource table now routes reactive table-state composition,
grouping, and row-windowing through
`frontend-modern/src/components/Infrastructure/useUnifiedResourceTableState.ts`,
while pure table-state derivation, service sorting, sort-cycle policy, and
responsive column layout now route through
`frontend-modern/src/components/Infrastructure/unifiedResourceTableStateModel.ts`,
and viewport reveal plus scroll synchronization now route through
`frontend-modern/src/components/Infrastructure/useUnifiedResourceTableViewportSync.ts`,
so the shared consumer model is no longer interleaving selector derivation,
layout policy, and DOM viewport coordination inside one mixed state boundary.
The canonical unified-resource change and relationship presenters now also
share the same elapsed-time and "ago" wording utilities, so `observed`,
`last seen`, and `ago` fragments stay consistent without each formatter
maintaining its own "time ago" implementation.
The drawer's change timeline confidence labels now also use a shared frontend
formatter, so the same percentage wording is emitted across timeline
surfaces instead of each consumer rounding confidence values on its own.
Those same timeline surfaces now also share a token-humanization helper for
fallback labels, so underscore cleanup and title-casing for change and drawer
labels stay aligned without local copies.
The same resource-change contract now also owns the canonical filter parser
used by `/api/resources/{id}/timeline`, so `kind`, `sourceType`, and
`sourceAdapter` validation stays with the change model instead of being
reconstructed in the HTTP handler.
The canonical resource policy model also owns a shared clone helper, so AI
chat and tools consumers preserve policy metadata by copying through the same
unified-resource contract instead of maintaining their own deep-copy logic.
That same contract now also owns `RefreshCanonicalMetadata` and
`RefreshCanonicalMetadataSlice`, so API and AI consumers refresh identity and
policy in one shared pass instead of composing the same metadata steps in
consumer-local shims.
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
`/api/resources/{id}/timeline` reads, while the bundled `/api/resources/{id}/facets`
surface keeps the facet summary and recent-change history available without
forcing consumers to parse the full resource payload.
Those filtered timeline reads are backed by dedicated `resource_changes`
indexes on `canonical_id`, `kind`, `source_type`, and `observed_at`, so the
canonical history path stays fast as the filtered timeline grows instead of
falling back to a consumer-local scan.
The frontend now also consumes those facet reads through
`frontend-modern/src/api/resources.ts` and the dedicated resource detail
drawer, which keeps the presentation surface aligned with the governed API
contract instead of rebuilding the relationship and timeline inline.
The canonical routing owner now also lives in
`frontend-modern/src/routing/resourceLinks.ts`, including the
workload-to-infrastructure href builder used by dashboard row and drawer
consumers. Future workload-to-resource navigation changes must extend through
that shared routing contract instead of reintroducing dashboard-local path
builders.
That drawer shell now routes its canonical timeline filter, facet-bundle, and
resource-intelligence state through
`frontend-modern/src/components/Infrastructure/useResourceDetailDrawerHistoryState.ts`,
while canonical identity, source, service, and debug derivations route through
`frontend-modern/src/components/Infrastructure/useResourceDetailDrawerDerivedState.ts`
while Docker update mutations route through
`frontend-modern/src/components/Infrastructure/useResourceDetailDrawerDockerActionsState.ts`,
and `frontend-modern/src/components/Infrastructure/useResourceDetailDrawerState.ts`
stays the composition owner, so unified-resource history, investigation, and
drawer-local action runtime no longer accumulate inline beside the model layer.
The infrastructure summary path now routes chart polling, cache hydration, and
org-scope lifecycle through
`frontend-modern/src/components/Infrastructure/useInfrastructureSummaryState.ts`,
while chart matching and summary-series derivation route through
`frontend-modern/src/components/Infrastructure/infrastructureSummaryModel.ts`
and `frontend-modern/src/components/Infrastructure/InfrastructureSummary.tsx`
stays the render shell, so summary charts are no longer an unowned mixed
resource consumer surface.
The shared `ResourceFacetSummary` consumer now omits capability and
relationship badges from the default table/detail surface entirely, while the
backend contract keeps capability and relationship data on the owned resource
model for governed AI and correlation consumers.
That keeps the proven monitoring UX centered on factual timeline
investigation while the richer facet payloads remain available as backend and
AI-facing foundations instead of being presented as first-class product facts
before they are fully populated.
The resource drawer now also keeps change history embedded inside `Overview`
instead of surfacing a peer `History` tab, so resource investigation stays on
one coherent runtime surface: the overview card carries the compact recent
activity summary, while the embedded change-history section owns filters and
the event log without duplicating a second timeline-summary card.
That same change-history header now renders as title plus compact summary only,
without an explanatory subheading, so the section reads like investigation
state instead of inline documentation.
The event log cards inside that section now stack key/value fields vertically
instead of using a two-column micro-grid, so each event reads like a single
timeline record rather than a tiny dashboard.
That same overview now keeps AI and governance detail
inside a collapsed `Context` disclosure, so runtime status and
identity stay primary while secondary AI and policy signals remain available
without competing with the first-screen monitoring story.
Inside that disclosure, the AI summary now reads as compact
label/value rows instead of metric tiles or nested cards, so the opened
investigation surface stays scan-first before the change summary and
correlation reveal appear.
Inside that disclosure, the governance summary also stays label-first with
compact rows and badges instead of a second card stack, so policy detail reads
as supporting context rather than another dashboard.
Inside that disclosure, learned dependency and correlation detail now sits
behind its own reveal without an additional bordered card wrapper, so the
opened investigation surface stays label-first and leaves relationship pattern
detail behind the reveal.
Change-related summary badges now belong to the `Change history` section
instead of the `Runtime` card, so current-state facts and timeline context do
not compete for the same ownership on first read.
The overview now begins directly with paired `Current state` and `Identity`
cards instead of a wrapper section title or separate peer runtime shell, so
current state and canonical identity read as one first-screen answer rather
than layered labels around adjacent mini-surfaces.
Those cards now use the same shared `Card` primitive as the workload drawers,
with a responsive two-column grid on wider screens, so the first read stays
compact while each side still has a consistent bounded card.
The drawer header now stays focused on canonical identity and source/type
badges only, while workload/service drill-down links and Kubernetes platform
signals live with the current-state card, so the top strip does not compete
with the resource name, status, or primary identity line.
That header badge row now also deduplicates identical visible labels, so
agent-backed nodes do not repeat `Agent` when both the canonical resource type
and a merged source resolve to the same badge text.
Runtime-scoped workloads drill-down routes now live in a dedicated `Access`
disclosure instead of a `Current state` row, so ordinary host drawers do not
surface a generic host-wide `Workloads` jump as default runtime chrome and
first-read status stays separate from the next place a user can go or inspect.
That same `Current state` card now only shows `Mode` when the resource carries
an actual canonical source mode, so ordinary hosts do not surface an empty or
meaningless mode row when no source-type contract is present.
Inside that top card pair, the operational and supporting context rows stay inline
instead of sitting in a collapsed `Details` disclosure or nested bordered
cards, so the first read stays like one linear sheet rather than a stack of
cards inside the overview shell.
Discovery support now lives as an `Analysis` reveal inside that same
overview-only `Access` surface instead of a peer drawer tab, so supplemental
inspection stays available without claiming the same navigation weight as
runtime, identity, or service-specific operational views.
That access surface is now a compact support row with a one-line summary,
embedded web-interface controls, scoped runtime links, and an on-demand
analysis panel, so the actionable access path stays primary while deeper
discovery inspection remains available without reading like a second peer
overview surface.
For ordinary host discovery, that analysis entry stays even quieter: the
collapsed `Access` state does not repeat a baseline `Host analysis via
<hostname>` summary when the discovery target is just the same host identity
already shown elsewhere in the drawer.
The analysis panel now expands directly inside the outer `Access` disclosure
instead of as a second peer support block, so the access surface reads as one
flattened reveal instead of another card group under the overview.
That access-side analysis surface still follows the same shell/runtime split as
the rest of the drawer: `DiscoveryTab.tsx` owns presentation and disclosures,
while `useDiscoveryTabState.ts` owns API fetches, websocket progress, and
note/discovery mutations.
The overview keeps access, investigation detail, service detail, and host
detail as collapsed sibling disclosures under the primary card pair, so the
drawer keeps
the top-level shape to current-state/identity plus `Change history` before any
secondary operational context appears.
That secondary hierarchy now renders in two layers: a full-width `Change
history` surface first, followed by a separate support-disclosure grid ordered
as `Access`, `Context`, `Service`, and `Host`, so the operator sees links and
investigation context before deeper technical drill-down cards.
Inside `Change history`, the event list now renders directly in the parent
section instead of inside a second bordered `Event log` card, so the timeline
reads like one inspection surface rather than a card nested under its own
section title.
The `Change history` filter controls now stay behind an explicit `Filter
history` reveal until the operator asks for them or activates a filter, so the
timeline reads as the primary inspection surface instead of opening on a form.
When that reveal is open, the controls still stack vertically instead of using
a paired filter grid, so the filter state reads as one subordinate control
column instead of a competing secondary layout.
When filters are active, the filtered facet bundle drives both the summary
chips and the event log, so the header and the list stay aligned instead of
showing stale unfiltered counts above filtered results.
Those support surfaces still use the same title-plus-summary header and
explicit reveal control without extra explanatory chrome, so the drawer signals
secondary depth through one governed structure instead of repeating prose about
what is "supporting" or "secondary" in each block.
Host and node system or hardware cards now also live behind a collapsed
`Host` support block instead of rendering before the primary overview
cards, so runtime status, identity, and next investigation steps stay first
while deeper machine detail remains available on demand.
That host-details section now reads as a simple vertical stack of detail cards
instead of a wrapped card grid, so the opened state stays linear instead of
feeling like a second dashboard.
Within that summary shell, current-state facts now stay operational: only
distinct platform IDs and platform-signal badges remain with runtime status,
while scoped links move to `Access` and aliases, IPs, and tags live only under the dedicated
`Identity` card.
That keeps first read status-first while still preserving canonical identity
metadata on the same top-level summary surface instead of mixing identity
support details into current-state chrome.
The identity-side rows stay label-first and only expand when a specific value,
like alias overflow, needs its own reveal, so the summary answers the main
resource question before deeper metadata appears.
When the identity side has no owned rows or supporting labels yet, the sparse
fallback now stays terse (`No identity metadata yet.`) so empty state chrome
does not read heavier than the data it is standing in for.
Type-specific Docker, PBS, and PMG operational panels now also live inside a
collapsed `Service` support block, so lane-specific controls and
breakdowns stay available without displacing the common runtime and identity
hierarchy on first read.
The drawer’s secondary support sections now share the same responsive
flex-wrap card-group pattern used by the workloads drawer, so change history,
access, service, host, and investigation context
read side by side on wider screens instead of as a single full-width stack.
Host uses that same flex-wrap pattern inside the disclosure for the
system, hardware, storage, and network cards, so the drawer matches the
shared workload-card density instead of stretching those cards one per row.
The collapsed `Host` summary now names the available categories only,
instead of exposing internal card counts in the disclosure label.
When `Service` is expanded, each service card remains summary-first and
pushes heavier breakdowns or update controls behind one more service-local
reveal, so the opened state still scans as current state before deeper
operations.
That same ownership rule now keeps the service-summary sentence in the
`Service` disclosure header instead of repeating a second summary box
inside PBS or PMG cards, and the service-local reveal labels stay terse
(`Show jobs`, `Show mail flow`, `Jobs`, `Queue`), while the opened accordions
also use shorter section labels (`Types`, `Queue detail`, `Mail detail`) and
count-only summary badges so opened cards read like current state instead of
descriptive chrome.
That collapsed `Service` summary now also uses resource-facing count
phrasing (`1 datastore`, `7 containers`, `16 delayed messages`) instead of
implementation wording like `queue total`.
Within that same PMG opened state, queue and backlog remain the primary metric
tiles while node count moves into quieter support context beneath them, so the
first read stays on mail-flow state instead of cluster metadata.
That support context also owns PMG freshness metadata now, so `Updated` does
not compete with queue and mail breakdown entries inside the mail-detail
accordion.
Those PMG queue and mail-detail accordions render as simple key/value rows
without entry-count badges or multi-column mini-dashboard layout, so the opened
state stays readable and scan-first.
The Docker service card now follows the same rule: its opened state uses
compact labels (`Docker runtime`, `Updates`, `Checked`, `Show actions`,
`Check now`, `Update all`) and short queued/confirm feedback so action
surfaces stay readable without turning the card into a paragraph of control
copy.
The remaining PBS and PMG cards also stay neutral and resource-first now:
their headings read `PBS` and `PMG`, and the primary health row is `State`
instead of `Connection`, so service support blocks read like resource status
rather than branded sub-pages.
Unsupported secondary tabs now also use the same terse availability notices
(`PMG resources only.`, `Kubernetes clusters only.`, `Docker runtimes with
Swarm only.`) instead of explanatory sentences, so mismatch fallback state
stays readable without turning support surfaces into inline documentation.
The same facet bundle now also returns grouped recent-change counts by
canonical change kind, so the detail drawer can surface the distribution of
state transitions, restarts, config updates, and anomalies without
recomputing timeline history in the browser, while broader relationship or
capability facets stay available behind the same contract for non-default
consumers that can prove they are populated and justified.
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
The same helper now also owns the governed-summary gate for mention
prefetching, so the decision to surface the block follows the same canonical
local-only and redaction rules as the rendered policy text.
The same helper also owns the governed-mention preamble and footer copy, so
the warning language around the block stays centralized with the policy model.
The same helper now also assembles the complete governed mention block, so
chat prefetch only routes the output instead of rebuilding the layout.
The same shared policy helper also owns the `aiSafeSummary` decision and
redaction predicates used by AI chat knowledge extraction and resource
context rendering, so governed labels and summary selection stay rooted in
the unified resource policy model instead of being duplicated in chat-local
helpers. Chat consumers now call `CloneResourcePolicy(...)` directly from the
shared unified-resource contract, and their governed labels now flow through
shared `ResourcePolicyLabel(...)` and `ResourcePolicyRedactedValue(...)`
helpers instead of through AI-local presentation shims, so policy copying and
policy-bound labels both stay centralized in the resource model. When a
policy requires governed handling, the shared label helper now prefers the
canonical AI-safe summary for every redacted or local-only resource and only
falls back to the canonical redacted label if that governed summary is
missing, so path-only and IP-only redactions do not regress to raw identity
leakage or over-redaction.
The frontend unified-resource hook now trusts backend canonical `policy` and
`aiSafeSummary` values directly, so the canonical summary value stays
aligned with the same policy-aware contract that governs sensitivity and
routing metadata without re-normalizing locally.
The resource detail drawer and unified resource table now also render that
governed display label through the same policy-aware helper, and they suppress
the raw alternate name when policy requires governed handling, so the visible
label stays aligned with the backend redaction boundary instead of
reconstructing a local name fallback.
The resource detail drawer now also resolves its AI-safe summary through that
same helper, so governed resources still present the canonical redacted label
when the backend summary is missing instead of dropping the governed summary
block entirely.
The shared frontend resource identity helper now owns that policy-aware
display contract, so infrastructure surfaces that ask for the preferred
resource label no longer need to re-encode the governed summary boundary by
hand. Settings quick-picks, infrastructure selectors, and the connected-
infrastructure / monitored-system projections now all stay on that same
preferred-label helper instead of carrying a separate raw-name fallback fork.
That same helper also owns platform-id redundancy suppression for
infrastructure drawers, so surfaces only render platform IDs when they add
identity context beyond the canonical display name or hostname instead of
repeating the same identifier chrome in both runtime and identity sections.
The shared workloads-link helper now also uses that preferred-label helper
for Kubernetes-cluster navigation fallbacks, so drawer/table navigation
context stays inside the same governed resource-label boundary instead of
repeating a raw display-name fork.
The resource drawer's Kubernetes namespace and deployment tabs use the
canonical cluster-name helper for backend fetch keys, keeping lookup identity
separate from the governed display-label contract.
The shared workloads projection in `useWorkloads` also uses that helper for
pod context labels, so dashboard Kubernetes grouping follows the same
canonical cluster-name contract instead of re-encoding the fallback locally.
The drawer's discovery mapper also reuses that helper for pod fallback agent
IDs, so the resource-detail path and the dashboard path stay aligned on the
same cluster-name source of truth.
The dashboard workload projection and workloads-link route helpers also share
the same Kubernetes context prefix helper in the shared agent-resource
layer, so pod grouping and cluster navigation keep the same cluster-context
prefix before any surface-specific display fallback is applied.
The unified-resource projection also reuses that same prefix helper for
projected Kubernetes `clusterId`, so the shared resource store stays aligned
with the dashboard and detail surfaces on the same canonical cluster-context
source of truth.
That same contract also owns the canonical resource display-name fallback, so
name-or-ID presentation stays consistent between the unified AI adapter, the
AI resource context, and the shared resource selectors instead of being
recomputed locally.

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
That same ownership boundary applies to incident-adjacent runtime history:
durable backend facts about what changed on a resource belong in
`ResourceChange` and the shared unified-resource store, while
`internal/ai/memory/incidents.go` remains an alert-scoped investigation
projection for notes, analyses, command breadcrumbs, runbooks, and other
operator-facing incident memory. Agents must not model the same durable backend
fact in both places as competing primary histories.

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
performance proof route. The shared resource table and resource drawer now
surface compact timeline summary chips, so facet presentation changes must
continue to flow through the same governed resource-row surface rather than
inventing a separate ad hoc summary path. Those row summaries now prefer
canonical `facetCounts` on the resource object when available, so the backend
list/read shapes remain the source of truth instead of forcing the frontend to
infer totals only from loaded slices. The drawer now fetches those facets
through one backend bundle endpoint, and that shared facet bundle preserves
the timeline slice plus recent-change counts so the overview card and history
summary can report the loaded history instead of collapsing to the currently
loaded page when the timeline endpoint is paginated. Timeline references in
that drawer now route
through the canonical infrastructure resource filter, so the resource history
remains navigable from the history surface instead of being purely
descriptive text.
The same infrastructure selector pipeline now also uses the policy-aware
display contract for search candidates, so governed resources do not reappear
through raw-name search forks even though the selector stays on the same
hot-path budget.
The shared recent-change presentation boundary is also owned here now.
`frontend-modern/src/components/Infrastructure/ResourceChangeSummary.tsx`
is the canonical shared card for recent resource-change timelines, while
`frontend-modern/src/utils/resourceChangePresentation.ts` owns canonical
change-kind wording, source-type wording, source-adapter provenance labels,
headline formatting, and timeline sort order. Future recent-change wording or
badge changes should extend those unified-resource owners instead of leaving
recent-change presentation unowned or rebuilding helper-local labels inside AI,
Patrol, performance, or infrastructure surfaces.
The shared correlation and policy-posture presentation boundaries are also
owned here now. `frontend-modern/src/components/Infrastructure/ResourceCorrelationSummary.tsx`
is the canonical shared card for dependency, dependent, and learned-correlation
context, while `frontend-modern/src/utils/resourceCorrelationPresentation.ts`
owns endpoint labels, headline formatting, summary wording, and canonical
correlation ordering. `frontend-modern/src/components/Infrastructure/ResourcePolicySummary.tsx`
is the canonical shared card for governed policy-posture counts, while
`frontend-modern/src/utils/resourcePolicyPresentation.ts` owns the canonical
sensitivity, routing, and redaction labels and aggregate count summaries.
Future correlation or policy-posture wording changes should extend those
unified-resource owners instead of drifting into page-local loops in AI,
Patrol, or infrastructure surfaces.
The canonical frontend resource-type and badge presentation boundaries are
also owned here now. `frontend-modern/src/utils/canonicalResourceTypes.ts`
owns the canonical allowed resource-type set for shared resource-picking and
validation flows, `frontend-modern/src/utils/resourceTypeCompat.ts` owns
frontend canonicalization from legacy or alias type tokens, and
`frontend-modern/src/utils/resourceTypePresentation.ts` owns canonical
resource-type labels and badge styling. `frontend-modern/src/utils/resourceBadgePresentation.ts`
and `frontend-modern/src/components/Infrastructure/resourceBadges.ts` then
own the shared resource/source/platform badge composition used by unified
resource tables, drawers, and infrastructure summary cards. Future resource
type or badge wording changes should extend these unified-resource owners
instead of rebuilding type-label or badge mapping logic inside infrastructure,
settings, alerts, recovery, or dashboard-local helpers.
The shared resource-runtime adapter boundary is also owned here now.
`frontend-modern/src/utils/agentResources.ts` owns canonical actionable
resource identities, agent-facet detection, cluster-name fallbacks, and
resource-derived chart-key candidates. `frontend-modern/src/utils/resourcePlatformData.ts`
owns the typed extraction of platform-data fragments from unified resources,
and `frontend-modern/src/utils/resourceStateAdapters.ts` owns canonical
projection from unified resources into node/PBS/PMG runtime view models.
Future resource-to-runtime mapping changes should extend these unified-resource
owners instead of rebuilding ad hoc platform-data parsing or action-target
fallback logic inside alerts, settings, recovery, AI, or infrastructure-local
state owners.
`ResourceFacetSummary` now consumes the shared
`frontend-modern/src/utils/resourceChangePresentation.ts` label helper for
canonical change kinds, source types, and adapter provenance, so the chip
wording stays aligned across table, drawer, and intelligence surfaces.
Timeline cards in that drawer surface change metadata when it is present, so
the history view preserves the richer provenance already carried by the
unified-resource model instead of flattening those fields away.
The same Infrastructure resource-only links now also default through the
shared `frontend-modern/src/components/Infrastructure/ResourceChangeSummary.tsx`
and `frontend-modern/src/components/Infrastructure/ResourceCorrelationSummary.tsx`
cards from the Patrol page, resource drawer, and problem-resource dashboard
panels, so canonical resource-filter path construction stays owned by the
shared summary cards rather than being duplicated per surface.
The unified resource table now also supplies a canonical resource-label
resolver into the resource drawer, so related-resource timeline chips can use
the same governed display labels as the table without adding a new
detail-local lookup path.
The resource drawer now also passes that same resolver into the shared
correlation summary, so dependency and dependent chips stay on governed
labels in the investigation path while the AI summary page keeps its broader
raw-ID fallback.
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
The infrastructure summary surfaces now use the shared normalized identity
lookup helper for these matches, so dotted hostnames such as
`tower.example.local` collapse to the same canonical lookup variants as the
resource table and resource detail surfaces instead of each view inventing
its own comparison rule.
The same identity surfaces also share the trimmed-string helper from
`frontend-modern/src/utils/stringUtils.ts` so resource-id, hostname, and
linked-node normalization keep the same fail-closed whitespace trimming rules
instead of reimplementing ad hoc local string cleanups.
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
options: `frontend-modern/src/features/infrastructure/InfrastructurePageSurface.tsx`
and `frontend-modern/src/features/infrastructure/useInfrastructurePageState.ts`
may render friendly string keys, but membership checks against available
sources must normalize through the shared
`frontend-modern/src/utils/sourcePlatforms.ts` helper before consulting
`KnownSourcePlatform` sets.
The route file `frontend-modern/src/pages/Infrastructure.tsx` is now only the
navigation boundary for that surface; canonical infrastructure filter, search,
deep-link, and expansion state now live behind the dedicated infrastructure
feature owner instead of accumulating in the page shell itself.
That infrastructure feature now also follows an explicit shell/composition/route
split: `frontend-modern/src/features/infrastructure/InfrastructurePageSurface.tsx`
owns the render shell, `frontend-modern/src/features/infrastructure/useInfrastructurePageState.ts`
owns page controls, persistence, and route composition,
`frontend-modern/src/features/infrastructure/infrastructurePageModel.ts`
owns filter/search/catalog derivation, and
`frontend-modern/src/features/infrastructure/useInfrastructurePageRouteState.ts`
owns URL-sync, deep-link expansion, highlight continuity, and managed
infrastructure-route navigation.
Shared unified-resource consumers now also normalize org scope through
`frontend-modern/src/utils/orgScope.ts` before building cache keys or
multi-tenant resource fetch state, so the canonical resource hooks do not
keep their own `getOrgID() || 'default'` fallback logic in each runtime
surface.
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
Shared privacy helpers also own the export sensitivity floor and route
decision derived from those canonical policy counts, so AI export audits and
route decisions reuse the same governed boundary logic instead of rebuilding
it in consumer packages.
That same export path now records canonical human-readable redaction labels
through the shared policy presentation helper, so audit records and prompt
context stay aligned on the same redaction vocabulary instead of duplicating
hint-to-label conversion in AI-local code.
The AI runtime now also uses the canonical policy presentation helpers to
surface those routing and redaction labels in shared context output, so the
same policy model is reflected in prompt summaries instead of being
re-described independently per surface.
That same shared presentation layer now also owns governed cluster-name and
IP-summary rendering for AI resource context, so topology labels and address
lists stay aligned with the same redaction vocabulary instead of being
formatted in AI-local helpers.
The AI correlation root-cause engine also consumes the canonical unified-
resource relationship model directly, so relationship reasoning and scoring
stay on the same owned edge vocabulary instead of keeping a separate
AI-local relationship struct.
Those helpers now own the canonical redaction-hint order and count-to-label
projection, so the AI summary and any other backend policy posture surface do
not re-sort redaction labels locally.
They also own the canonical sensitivity and routing order used to format
policy-posture count summaries with human-readable labels, so the AI summary
and frontend policy card both read the same presentation sequence from the
shared resource model.
Canonical resources now carry first-class relationship, capability, and
timeline fields: `Capabilities` (bounded action definitions with approval
levels), `Relationships` (typed inter-resource links with direction and
confidence), and `RecentChanges` (typed change timeline entries with source,
confidence, and related-resource references). These fields are defined in
`capabilities.go`, `relationships.go`, `changes.go`, `privacy.go`, and
`actions.go`. The backend keeps those fields for AI and correlation use,
while the frontend consumer stays timeline-first and only preserves the
recent-change slice plus facet counts it actually renders. The store now also
owns a `resource_changes` persistence table with `RecordChange` and
`GetRecentChanges` methods so change history is queryable by canonical ID and
time window.
Action plans in `actions.go` now keep stale-plan protection to the canonical
`resourceVersion`, `policyVersion`, and `planHash` fields, so the durable
audit record stays minimal and does not need extra relationship-topology
versioning.
The shared change presentation helper also owns the canonical kind, source
type, and source adapter labels for those timeline entries, so summary cards
and drawer history surfaces both read the same badge vocabulary instead of
rebuilding resource-change labels locally.

That frontend consumer rule now applies on the canonical decode path too:
`frontend-modern/src/hooks/useUnifiedResources.ts` must preserve backend-owned
policy metadata, AI-safe summaries, recent changes, and facet counts as
first-class `Resource` fields, and it must treat the backend refresh path as
the source of truth instead of re-normalizing those values locally.
Shared infrastructure consumers such as the unified resource table and detail
drawer must present that owned metadata through shared helpers instead of
reconstructing privacy posture from display names, source types, or other
incidental runtime hints.
