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
For Docker and Podman container resources, the canonical CPU metric represents
host-capacity-normalized utilization. Runtime-native per-core CPU percent is
kept on Docker metadata as raw evidence and must not replace the canonical
metric or threshold surface. After that capacity normalization, app-container
CPU and memory metrics are already 0..100 percentages and must be clamped
directly rather than treated as 0..1 ratios.
Host-agent and Docker-host CPU, memory, and disk usage fields are already
reported as 0..100 percentages. Unified-resource host metric payloads and
host-derived storage adapters must clamp those reported percentages directly
rather than passing them through ratio-to-percent normalization, which is
reserved for providers that report 0..1 usage ratios.
Physical-disk resources own cross-source disk identity. When Proxmox inventory
and host-agent SMART telemetry describe the same device, the merged resource
must retain Proxmox node/instance source payloads while carrying SMART
temperature, capacity, health, and identity enrichment.

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
20. `internal/unifiedresources/monitored_system_projection.go`
21. `internal/unifiedresources/hostname_equivalence.go`
22. `internal/unifiedresources/capabilities.go`
23. `internal/unifiedresources/changes.go`
24. `internal/unifiedresources/relationships.go`
25. `internal/unifiedresources/privacy.go`
26. `internal/unifiedresources/actions.go`
26a. `internal/unifiedresources/action_dispatch.go`
26b. `internal/unifiedresources/action_dispatch_store.go`
27. `internal/unifiedresources/audit_redaction.go`
28. `frontend-modern/src/components/Infrastructure/ResourceDetailDrawer.tsx`
29. `frontend-modern/src/components/Infrastructure/ResourceDetailDrawerOverviewTab.tsx`
30. `frontend-modern/src/components/Infrastructure/ResourceDetailDrawerDebugTab.tsx`
31. `frontend-modern/src/components/Infrastructure/ResourceDetailDrawerSupportDisclosure.tsx`
32. `frontend-modern/src/components/Docker/SwarmServicesDrawer.tsx`
33. `frontend-modern/src/features/docker/DockerConfigsTable.tsx`
34. `frontend-modern/src/features/docker/DockerContainersTable.tsx`
34a. `frontend-modern/src/features/docker/DockerContainerLifecycleControls.tsx`
34b. `frontend-modern/src/features/docker/dockerContainerLifecycleActions.ts`
34c. `frontend-modern/src/features/docker/dockerContainerTableModel.ts`
35. `frontend-modern/src/features/docker/DockerImagesTable.tsx`
36. `frontend-modern/src/features/docker/DockerNativeTableShared.tsx`
37. `frontend-modern/src/features/docker/DockerNetworksTable.tsx`
38. `frontend-modern/src/features/docker/DockerPageSurface.tsx`
39. `frontend-modern/src/features/docker/DockerSecretsTable.tsx`
40. `frontend-modern/src/features/docker/DockerSwarmNodesTable.tsx`
41. `frontend-modern/src/features/docker/DockerTasksTable.tsx`
42. `frontend-modern/src/features/docker/DockerVolumesTable.tsx`
43. `frontend-modern/src/features/docker/dockerPageModel.ts`
44. `frontend-modern/src/components/Kubernetes/K8sDeploymentsDrawer.tsx`
45. `frontend-modern/src/components/Kubernetes/K8sNamespacesDrawer.tsx`
46. `frontend-modern/src/components/Infrastructure/ResourceActionHistory.tsx`
47. `frontend-modern/src/components/Infrastructure/ResourceFacetSummary.tsx`
48. `frontend-modern/src/components/Infrastructure/ResourceChangeSummary.tsx`
49. `frontend-modern/src/components/Infrastructure/ResourceCorrelationSummary.tsx`
50. `frontend-modern/src/components/Infrastructure/ResourceOperatorStateSection.tsx`
50a. `frontend-modern/src/components/Infrastructure/ResourcePolicySummary.tsx`
51. `frontend-modern/src/components/Infrastructure/UnifiedResourceHostTableCard.tsx`
52. `frontend-modern/src/components/Infrastructure/UnifiedResourcePBSTableSection.tsx`
53. `frontend-modern/src/components/Infrastructure/UnifiedResourcePMGTableSection.tsx`
54. `frontend-modern/src/components/Infrastructure/UnifiedResourceServiceInfrastructureCard.tsx`
55. `frontend-modern/src/components/Infrastructure/unifiedResourceTableModel.ts`
56. `frontend-modern/src/components/Infrastructure/unifiedResourceTableStateModel.ts`
57. `frontend-modern/src/components/Infrastructure/useResourceDetailDrawerDerivedState.ts`
58. `frontend-modern/src/components/Infrastructure/resourceDetailDrawerServiceModel.ts`
59. `frontend-modern/src/components/Infrastructure/resourceDetailDrawerVmwareModel.ts`
60. `frontend-modern/src/components/Infrastructure/resourceDetailDiscoveryModel.ts`
61. `frontend-modern/src/components/Infrastructure/resourceDetailDrawerOperationalModel.ts`
62. `frontend-modern/src/components/Infrastructure/useResourceDetailDrawerHistoryState.ts`
63. `frontend-modern/src/components/Infrastructure/useResourceDetailDrawerDockerActionsState.ts`
64. `frontend-modern/src/components/Infrastructure/useResourceDetailDrawerState.ts`
65. `frontend-modern/src/components/Infrastructure/useUnifiedResourceTableState.ts`
66. `frontend-modern/src/components/Infrastructure/useUnifiedResourceTableViewportSync.ts`
67. `frontend-modern/src/components/Discovery/discoveryReadiness.ts`
68. `frontend-modern/src/components/Discovery/DiscoveryTab.tsx`
69. `frontend-modern/src/components/Discovery/useDiscoveryTabState.ts`
70. `frontend-modern/src/utils/agentResources.ts`
71. `frontend-modern/src/utils/canonicalResourceTypes.ts`
72. `frontend-modern/src/utils/resourceBadgePresentation.ts`
73. `frontend-modern/src/utils/resourceChangePresentation.ts`
74. `frontend-modern/src/utils/actionAuditPresentation.ts`
75. `frontend-modern/src/utils/resourceCorrelationPresentation.ts`
76. `frontend-modern/src/utils/resourcePlatformData.ts`
76. `frontend-modern/src/utils/resourcePolicyPresentation.ts`
77. `frontend-modern/src/utils/resourceStateAdapters.ts`
78. `frontend-modern/src/utils/resourceTypeCompat.ts`
79. `frontend-modern/src/utils/resourceTypePresentation.ts`
80. `frontend-modern/src/utils/serviceHealthPresentation.ts`
81. `frontend-modern/src/utils/sourceTypePresentation.ts`
82. `frontend-modern/src/utils/workloadTypePresentation.ts`
83. `frontend-modern/src/utils/resourceIdentity.ts`
84. `frontend-modern/src/components/Infrastructure/resourceDetailDrawerIdentityModel.ts`
85. `frontend-modern/src/hooks/useUnifiedResources.ts`
86. `frontend-modern/src/types/resource.ts`
87. `frontend-modern/src/utils/sourcePlatforms.ts`
88. `frontend-modern/src/utils/platformSupportManifest.generated.ts`
89. `internal/unifiedresources/kubernetes_metric_ids.go`
90. `internal/unifiedresources/policy_posture.go`
91. `frontend-modern/src/features/platformNavigation/platformNavigationModel.ts`
91. `internal/unifiedresources/clone.go`
92. `frontend-modern/src/components/Infrastructure/resourceDetailDrawerPresentation.ts`
93. `internal/unifiedresources/storage_consumers.go`
94. `frontend-modern/src/features/standalone/standalonePageModel.ts`
95. `frontend-modern/src/features/standalone/StandalonePageSurface.tsx`
96. `frontend-modern/src/features/standalone/AgentsMachinesTable.tsx`
97. `frontend-modern/src/features/standalone/AvailabilityChecksTable.tsx`
98. `internal/platformsupport/manifest_generated.go`
99. `frontend-modern/src/features/kubernetes/KubernetesControllersTable.tsx`
100. `frontend-modern/src/features/kubernetes/KubernetesPageSurface.tsx`
101. `frontend-modern/src/features/kubernetes/kubernetesPageModel.ts`
102. `frontend-modern/src/features/kubernetes/KubernetesClustersTable.tsx`
103. `frontend-modern/src/features/kubernetes/KubernetesDeploymentsTable.tsx`
104. `frontend-modern/src/features/kubernetes/KubernetesNodesTable.tsx`
105. `frontend-modern/src/features/kubernetes/KubernetesPodsTable.tsx`
106. `frontend-modern/src/features/kubernetes/KubernetesStorageTable.tsx`
107. `frontend-modern/src/features/kubernetes/KubernetesNetworkingTable.tsx`
108. `frontend-modern/src/features/kubernetes/KubernetesServicesTable.tsx`
109. `frontend-modern/src/features/kubernetes/KubernetesConfigTable.tsx`
110. `frontend-modern/src/features/kubernetes/KubernetesPolicyTable.tsx`
111. `frontend-modern/src/features/kubernetes/KubernetesAutoscalingTable.tsx`
112. `frontend-modern/src/features/kubernetes/KubernetesEventsTable.tsx`
113. `frontend-modern/src/features/docker/DockerAlertsTable.tsx`
114. `frontend-modern/src/features/docker/DockerServicesTable.tsx`
115. `frontend-modern/src/features/docker/DockerStorageUsageTable.tsx`
116. `frontend-modern/src/features/actions/ActionDecisionPacket.tsx`
117. `frontend-modern/src/features/actions/ActionReviewDialog.tsx`
118. `frontend-modern/src/features/actions/actionPresentation.ts`
118a. `frontend-modern/src/features/actions/actionRouting.ts`
119. `frontend-modern/src/pages/Actions.tsx`
120. `frontend-modern/src/routing/navigation.ts`
121. `frontend-modern/src/routing/routePreload.ts`
116. `frontend-modern/src/features/kubernetes/KubernetesAlertsTable.tsx`
117. `frontend-modern/src/features/proxmox/ProxmoxBackupServersTable.tsx`
118. `frontend-modern/src/features/proxmox/ProxmoxCephTable.tsx`
119. `frontend-modern/src/features/proxmox/ProxmoxCoverageTable.tsx`
120. `frontend-modern/src/features/proxmox/ProxmoxMailGatewayTable.tsx`
121. `frontend-modern/src/features/proxmox/ProxmoxRecoverableTable.tsx`
122. `frontend-modern/src/features/proxmox/ProxmoxReplicationTable.tsx`
122a. `frontend-modern/src/features/proxmox/proxmoxHostTableModel.ts`
123. `frontend-modern/src/features/truenas/TrueNASAlertsTable.tsx`
124. `frontend-modern/src/features/truenas/TrueNASAppsTable.tsx`
125. `frontend-modern/src/features/truenas/TrueNASNetworkSharesTable.tsx`
126. `frontend-modern/src/features/truenas/TrueNASProtectionTable.tsx`
127. `frontend-modern/src/features/truenas/TrueNASServicesTable.tsx`
128. `frontend-modern/src/features/truenas/TrueNASStorageTopologyTable.tsx`
129. `frontend-modern/src/features/truenas/TrueNASSystemsTable.tsx`
130. `frontend-modern/src/features/truenas/TrueNASVirtualMachinesTable.tsx`
131. `frontend-modern/src/features/vmware/VsphereActivityTable.tsx`
132. `frontend-modern/src/features/vmware/VsphereAlertsTable.tsx`
133. `frontend-modern/src/features/vmware/VsphereDatastoresTable.tsx`
134. `frontend-modern/src/features/vmware/VsphereNetworksTable.tsx`

## Shared Boundaries

Kubernetes workload presentation is API-native, not generic inventory. Pods,
Deployments, controllers, and autoscalers render under the `/kubernetes/workloads`
workflow tab; legacy object routes such as `/kubernetes/pods`,
`/kubernetes/deployments`, `/kubernetes/controllers`, and
`/kubernetes/autoscaling` are compatibility aliases for that workflow. Pods
render through `frontend-modern/src/features/kubernetes/KubernetesPodsTable.tsx`
with Pod phase, container readiness, restart, owner, node, image, and age fields
from the Kubernetes API projection. StatefulSet, DaemonSet, Job, and CronJob rows
render through
`frontend-modern/src/features/kubernetes/KubernetesControllersTable.tsx` and must
preserve the Kubernetes API status semantics for desired replica/node/job
targets, current or active counts, ready/succeeded counts, availability,
misscheduled/unavailable/failed/suspended exceptions, service names, schedules,
and controller timing fields instead of falling back to infrastructure columns.
Deployment rows render through
`frontend-modern/src/features/kubernetes/KubernetesDeploymentsTable.tsx` with
Deployment status replica counts, `status.observedGeneration`, and metadata
creation age carried by the canonical Kubernetes resource facet rather than
page-local derivation or generic host columns. HorizontalPodAutoscaler rows in
the same workflow preserve scale target, min/max replica bounds, current and
desired replicas, and metric source types instead of routing HPA rows through the
generic controller/event inventory table.

Container runtime navigation is a unified-resource consumer boundary: the
`/docker` route is the Docker / Podman runtime lens, not an exclusive owning
platform. Overview is the primary Docker / Podman landing surface and carries
runtime host rows plus primary `app-container` workload rows so small and
medium estates preserve the proven Pulse host-then-workloads interaction. The
legacy `/docker/containers` route is a compatibility alias for that Overview
surface, not a separate visible tab. Supporting subtabs such as Images,
Storage, Networks, and Swarm may be shown only from canonical resource
evidence, and inactive standalone Swarm metadata must not be interpreted as
host-role or service-surface proof. The unified-resource adapter is the backend
fail-closed layer for that rule, so persisted or older-agent inactive Swarm
payloads cannot reintroduce false Swarm capability surfaces.
The empty state for this route must preserve that runtime-lens contract:
standalone Docker / Podman hosts use a local runtime agent, while Docker inside
Proxmox LXCs is represented as the explicitly opted-in Proxmox host-side guest
Docker inventory path rather than a requirement to install an agent in every
guest.

Platform Overview tabs are rollup boundaries, not duplicate inventory dumps.
Docker / Podman Overview owns runtime host rows and primary container workload
rows; image, storage, network, and Swarm object rows belong in their workflow
tabs. Kubernetes Overview owns cluster/control-plane rollup rows only; node,
workload, service, storage, configuration, policy, and event object rows belong
in their workflow tabs. If a future Overview repeats a detailed table, the
owning workflow must be retired or the Overview content must be reduced to
aggregate signal.
Cross-platform handoffs into the Docker / Podman Overview must preserve runtime
scope when the source surface knows it. A Proxmox LXC drawer that links to the
Docker runtime lens must carry the unambiguous Docker host facet as route state,
and Docker Overview must apply that host scope to both the runtime host summary
and the primary container table so the handoff lands on the same runtime the
operator just inspected.
Platform native tables are unified-resource consumers even when their visual
table frame is frontend-primitives-owned. Docker / Podman, Kubernetes, Proxmox,
Standalone, TrueNAS, and vSphere platform tables own source-specific row fields,
filter semantics, drawer handoffs, and resource projections, while
`PlatformTableShell` owns the shared table card, header row, and body frame.
Future platform tables must keep that split: row data and platform semantics
stay in the unified-resource consumer, and the repeated table shell stays in the
shared frontend primitive.
Alert decoration on those platform rows consumes the canonical active-alert
read model and the detector-enabled accessor. External notification activation
is not a resource-health field and must never suppress row alerts, change
resource filtering, or create a parallel platform-local alert truth.
The standalone Pulse Agent and Availability monitor may add one compact status
summary immediately above its canonical table. That summary must be derived
from the same already-loaded unified-resource slice, use canonical resource
status and `lastSeen` fields, keep the machine row indicator on that same
freshness-aware presentation so an old agent cannot stay visually green while
the summary warns, keep failed or degraded checks ahead of healthy
checks, and route management back to the canonical infrastructure or
availability settings paths. It must not introduce a page-local fetch, generic
Home dashboard, decorative chart, shadow health model, or proof strip detached
from the table it summarizes.
Availability timestamps must preserve absence as absence. Monitoring and mock
adapters must project zero `LastChecked` or `LastSuccess` values as nil
canonical facet pointers so REST and WebSocket consumers render `Not checked`
or `Never`, never a year-one timestamp expressed as hundreds of thousands of
days ago.
Shared text-cell fallback behavior follows the same boundary:
unified-resource consumers own which source field is displayed, but platform
tables must use the frontend-primitives-owned `formatPlatformTableTextValue`
helper for trimmed-string empty-cell rendering instead of recreating local
`textValue` helpers or inline `asTrimmedString(...) || '—'` fallbacks.
Simple title-case state/status label fallback follows that boundary too:
unified-resource consumers own which TrueNAS state or status value is shown,
but TrueNAS platform tables must use `formatPlatformTableTitleCaseValue`
instead of recreating local `titleCase` helpers.
Compact list summaries follow the same boundary: unified-resource consumers
own which source values belong in host, VM, access, client, security, label,
port, selector, policy type, or quota cells, while platform tables must use
`summarizePlatformTableValues` for trimming, empty-marker labels, `+N` overflow
labels, full title text, and normalized value lists instead of recreating local
`compactList` or `summarizeValues` helpers.
Platform table uptime labels follow the same boundary: unified-resource
consumers own which uptime field is displayed and whether the dense table
needs compact or full precision, but platform tables and platform-owned
drawers, including Standalone machines, Proxmox nodes and backup servers,
Proxmox Mail Gateway node rows, and vSphere ESXi hosts, must use
`formatPlatformTableUptimeValue` instead of recreating local days/hours/minutes
`formatUptime` helpers or importing the generic formatter in table files.
Platform byte-size table cells follow that boundary too: unified-resource
consumers own whether a capacity, memory, disk, requested-size, datastore,
pool, or backup-artifact size field is the right source-specific value, while
dense Docker / Podman, Kubernetes, Proxmox, and TrueNAS platform table cells
must use `formatPlatformTableBytesValue` for positive byte formatting and
unknown / non-positive empty-cell markers instead of carrying table-local
`formatBytes` wrappers or direct generic byte-formatter imports.
Compact timestamp table cells follow the same boundary: unified-resource
consumers own whether `completedAt`, `startedAt`, `occurredAt`, or `observedAt`
is the right source-specific value for the row, while dense platform table
rendering must use `PlatformTableDateTimeValue` /
`formatPlatformTableDateTimeValue` for parsing, empty markers, compact Intl
formatting, minimum-year filtering when platform adapters emit placeholder
dates, tabular styling, and canonical `numeric-value` alignment.
Relative timestamp-age cells keep the same split: unified-resource consumers
own whether created, observed, sync, agent `lastSeen`, or availability
`lastChecked` time is the meaningful source-specific freshness field, while
dense platform table rendering must use `PlatformTableRelativeTimeValue` /
`formatPlatformTableRelativeTimeValue` for relative labels, compact defaults,
invalid/empty markers, and tabular styling instead of importing
`formatRelativeTime` or declaring local timestamp-age helpers in table files.
Duration and interval cells keep the same split: unified-resource or
source-specific consumers own which elapsed duration, human fallback, or poll
interval field is meaningful, while dense platform table rendering must use
`PlatformTableDurationValue` / `formatPlatformTableDurationValue` for
seconds/minutes/hours labels, fallback text, invalid/empty markers, and
tabular styling instead of declaring local seconds/minutes helpers in table
files.
That boundary covers Proxmox replication last-sync duration cells and
Standalone availability-check poll interval cells, and future duration-like
platform table values must join the same shared primitive registry rule before
adding table-local formatting.
Responsive table layout keeps the same split: unified-resource and
source-specific table models own which columns are visible at each breakpoint
and the relative weights for those columns, while dense platform table
rendering must use `getPlatformTableWeightedColumnWidthStyle` for
visible-column percentage normalization, zero-width fallback, and stable
percentage formatting instead of declaring local width-percent helpers.
Optional numeric table cells follow the same split: unified-resource consumers
own which count or replica field is meaningful, whether the domain should
zero-default an absent scheduler/service/inventory count, whether a
domain-specific formatter is needed, and which current/total fields belong in a
grouped health ratio, while dense platform table cells must use
`PlatformTableNumberValue` for scalar count validation, tabular styling, and
unknown empty-cell markers, and `PlatformTableCountRatioValue` for
healthy/total or ready/total count-ratio skeletons. String-only table summaries
and titles use `formatPlatformTableCountRatioValue` for the same zero-default
and suffix behavior. Tables must not carry local
`numberValue`, `numericValue`, `replicaCount`, `countCell`, `childCountCell`,
`diskCountLabel`, or cell-level `tabular-nums` variants for grouped
child/share/service/storage counts, or local ready/total template strings for
endpoint summaries.
Locale-formatted integer count labels keep that same split:
unified-resource consumers own which domain count is meaningful and whether an
absent count should read as zero, while dense platform table and drawer
rendering must use `formatPlatformTableIntegerValue`, normally through
`PlatformTableNumberValue`, for rounded integer formatting, locale grouping,
and unknown empty-cell markers instead of local `formatInteger`,
`formatLocaleCount`, `formatNumber`, or direct `toLocaleString()` helpers.
Percent and temperature table cells follow the same split: unified-resource
consumers own which usage percentage or temperature source is meaningful for
the row, while dense platform table rendering must use
`PlatformTablePercentValue`, `formatPlatformTablePercentValue`, and
`PlatformTableTemperatureValue` for one-decimal percent formatting, ratio
normalization/clamping when the source metric requires it, positive Celsius
validation, tabular styling, and empty markers instead of local
`formatPercent`, `formatPercentLabel`, `toFixed(1)%`, `formatTemperature`, or
temperature label helpers.
Thermal pressure state is a separate host sensor facet, not a Celsius
temperature source. Unified-resource adapters must carry
`agent.sensors.thermalState` alongside `temperatureCelsius`; table consumers
may render that pressure as compact status text only when no numeric
temperature exists, and must not convert pressure into a Celsius metric or
history target.
Typed GPU host telemetry follows the same canonical sensor route.
`HostSensorMeta.gpu` may carry GPU id, name, temperature, utilization, and VRAM
readings through clone, merge, transport, and frontend decode paths, but those
readings remain descriptive host context. Unified-resource consumers must not
promote GPU sensors into separate hardware resources, lifecycle state, workload
identity, or history targets beyond direct numeric temperature compatibility.
Host-agent wattage follows that same canonical sensor route.
`HostSensorMeta.powerWatts` may carry named power readings through clone,
merge, transport, and frontend decode paths, but those readings remain
descriptive host context. Unified-resource consumers must not promote power
sensors into separate hardware resources, lifecycle state, workload identity,
storage health, alert metrics, or history targets without a separate governed
metric contract.
Metric bar fallbacks follow that split as well: unified-resource consumers own
which CPU or memory value is selected, plus any source-specific fallback reason
such as outdated standalone agent telemetry, while platform tables must use
`PlatformTableMetricFallback` and `getPlatformTableFiniteMetric` for CPU,
memory, disk, and capacity metric cells instead of recreating local fallback
markup or `Number.isFinite` wrappers.
Platform filter option semantics follow that split too: unified-resource
consumers own the source-specific status buckets and labels, while the repeated
FilterBar chip leading-dot presentation must use the frontend-primitives-owned
`filterChipStatusDot` helper instead of page-local span factories.
Platform resource status ranking follows the same split. Docker / Podman and
Kubernetes page models own their source-specific attention ordering, but any
rank table over shared `StatusIndicatorVariant` values must include the full
shared variant vocabulary, including informational states, instead of assuming
only success/warning/danger/muted status classes.
Platform table empty states use that same split. Unified-resource consumers own
the source-specific empty-state vocabulary, action choice, and evidence rule
that decides why a table is empty, but the table-card empty-state shell itself
must compose the frontend-primitives-owned `PlatformTableEmptyState` primitive
instead of importing `EmptyState` directly or wrapping it in a page-local card.
Platform loading and load-failure states follow the same boundary.
Unified-resource consumers such as `ProxmoxReplicationTable` own the API query,
resource-specific copy, retry callback, and evidence rule that decides which
state applies, but repeated loading rows and retry shells must compose the
frontend-primitives-owned `PlatformTableLoadingState` and `PlatformErrorState`
primitives instead of recreating page-local table-card status rows or local
Refresh-button error empty states.
The same split applies to inline detail row shells on platform, native-resource,
workload, and infrastructure tables. Unified-resource consumers own row
identity, source-specific drawer content, expansion state, and resource
semantics, but the repeated surface-alt table row, single-cell colspan wrapper,
detail padding, and row-click containment must compose the
frontend-primitives-owned `InlineDetailTableRow` primitive instead of
rebuilding a local `TableRow` / `TableCell` detail shell.
That same split applies to platform row-detail disclosure controls. Platform
tables own which row opens, which resource label is exposed, and which
source-specific detail payload or drawer is rendered, but the disclosure
affordance itself must compose
`PlatformResourceDetailToggleButton` from the frontend-primitives-owned
`PlatformResourceDetailTableRow.tsx` contract. Future platform tables must not
add page-local chevron buttons, bespoke `aria-expanded` handling, or local
event-propagation variants for row detail expansion.
Inline detail section content follows the same ownership split.
Unified-resource consumers own the source-specific section rows, labels,
resource identities, and severity/status semantics, but repeated section-row
compaction, table rendering, value-tone classes, and inline close-action chrome
must compose the frontend-primitives-owned `DetailSectionTable`,
`InlineDetailPanel`, and `detailSectionModel.ts` primitives instead of
recreating local `DetailField` grids or provider-named neutral detail tables.
Provider detail drawer models also own which byte, count, and integer fields
are meaningful, but the formatting and pluralization of those repeated numeric
detail values must use the shared `detailSectionModel.ts` helpers rather than
provider-local byte scaling, integer, or count helpers.
The same split applies to VMware vSphere detail sections: unified-resource
VMware metadata owns the vCenter-specific row selection, but the section/row
shape and renderer must be the shared frontend-primitives detail-section
contract instead of a VMware-local row model or custom vSphere detail shell.
The split also applies to web-interface launch affordances. Unified-resource
tables own whether a row has a saved, inferred, or source-native web-interface
URL and how that URL is derived, but the visible launch affordance belongs on
the resource name through the shared `WebInterfaceNameLink` primitive. Proxmox
host table columns are governed by `proxmoxHostTableModel.ts`; that model must
not reintroduce a separate `Web` column or move the web-interface launch back
into a page-local icon cell.
Product-originated resource references may arrive as registered unified
resource IDs, source-specific IDs, or canonical identity aliases. The
unified-resource registry owns resolving those references through
`GetByReference`, returning the registered resource ID for downstream store,
action, finding, and context-pack lookups. Resource drawer and workload drawer
Assistant handoffs may pass stable source IDs such as Proxmox
`instance:node:vmid`, but they must not expose generated registry IDs as the
browser-side contract or rebuild alias matching in frontend code.
Proxmox VM and system-container action/discovery ownership is derived from the
canonical parent node resource, not from guest-local heuristics. When the
registry or presentation coalescer merges a Proxmox-only node row with an
agent-backed node row, Proxmox children must be reparented to the displayed
agent-backed resource and their `ProxmoxData.LinkedAgentID` must inherit the
parent Pulse agent ID. That linked agent ID is the only source for Proxmox
workload `discoveryTarget.agentId`; node names and VMIDs alone are not enough
to authorize browser action targets.
Proxmox VM guest-agent outage projection is also a unified-resource boundary.
When monitoring marks a running VM's expected QEMU guest agent as unreachable,
the resource adapter must surface a Proxmox-authored `availability_unreachable`
`resource-incident` with source `qemu-guest-agent`; it must leave VM power
status online/warning rather than rewriting the VM to stopped/offline.
Source freshness is a separate unified-resource status input, not a fixed
global health timer. Snapshot rebuilds, resource seeding, supplemental record
overlays, and cloned read-state overlays must preserve the monitoring-provided
stale thresholds for Proxmox, PBS, and PMG sources so resources do not flap to
warning/degraded between successful configured poll cycles.

Service-discovery readiness is a unified-resource payload contract, not a
drawer-local decoration. Resource list/detail payloads that expose a
`discoveryTarget` must also carry the backend-authored `discoveryReadiness`
projection when the discovery owner is available. That projection is
metadata-only (`fresh`, `stale`, `missing`, `running`, `failed`, `unavailable`,
or `unsupported`, with provenance, generated/observed freshness, target
coordinates, optional service/category, and bounded fact count) and must not
include raw command output, provider commands, environment variables, config
paths, or secret-bearing metadata. Workload and resource drawers, Assistant
handoffs, and optional table columns consume this field instead of inferring
freshness from independent discovery reads.
Across platform/runtime pages, workflow tabs are evidence-gated from the
canonical model that owns their rows. `Overview` is the stable landing tab;
supporting tabs appear only when their native inventory or signal exists, and
legacy/direct object routes fall back to `Overview` when the requested workflow
has no rows for the current setup. Signals outside unified-resource inventory,
such as TrueNAS recovery protection points or vSphere activity timeline rows,
must be treated as explicit tab evidence rather than permanent navigation.
Primary platform navigation is also resource-evidence gated: runtime lenses such
as Docker / Podman must be admitted from explicit Docker source scopes, Docker
host or service resource types, or concrete runtime identity/inventory evidence
such as a runtime/version or a positive object count. Generic host metadata,
zero-count compatibility projections, and empty facets such as `docker: {}` on a
plain machine agent are not Docker inventory and must not create a transient or
empty Docker tab during hydration. Navigation admission and the corresponding
platform page model must consume the same evidence contract. A direct route to
an absent platform may show its setup action, but the primary navigation must
stay limited to platform pages that can render admitted inventory.

Kubernetes configuration and policy inventory are unified-resource consumer
boundaries: the `/kubernetes/configuration` workflow tab must render Namespace,
ConfigMap, Secret, ServiceAccount, Role, ClusterRole, RoleBinding,
ClusterRoleBinding, NetworkPolicy, PodDisruptionBudget, ResourceQuota, and
LimitRange rows through API-native config and policy tables. Legacy
`/kubernetes/config` and `/kubernetes/policy` routes resolve to that workflow
tab. Config rows preserve cluster/namespace scope, Namespace lifecycle phase,
ConfigMap/Secret immutable and type metadata, ServiceAccount
token/image-pull/secret references, RBAC summary counts (rule counts, role
kind / role name, subject count, subject Kinds, ClusterRole aggregation
labels), and metadata-only collection wording without rendering ConfigMap or
Secret payload values, individual RBAC subject names, or full PolicyRule
contents. Policy rows preserve API-specific policy types, ingress/egress rule
counts, disruption-budget health and allowed disruptions, quota hard/used
maps, and LimitRange item types instead of collapsing those resources into
generic detail text.

Kubernetes events inventory is a unified-resource consumer boundary: the
`/kubernetes/events` route must render Event rows through an event-native table
that preserves event type, reason, involved object, occurrence count, observed
time, and message instead of routing event rows through the generic controller
inventory table.

Docker / Podman runtime inventory is a unified-resource consumer boundary: the
container runtime page must render Engine container, image, volume, network,
Swarm node, task, secret, and config resources through API-object-native tables
that preserve the fields documented by the Docker Engine API instead of routing
those rows through a variant-switched generic inventory table. Swarm service
rows must preserve the service update status emitted by the Docker adapter, and
engine storage rows must stay host-scoped with table proof hooks so browser
proof can distinguish a populated disk-usage tab from an empty fixture.
Runtime container detail payloads must preserve the agent-reported lifecycle
timestamps, Podman pod/compose/auto-update/user-namespace metadata, and
cumulative block I/O totals on `DockerData`. They must also preserve nullable,
runtime-authored `OOMKilled` evidence without converting absent state or exit
code 137 into a positive classification; typed Docker views must return an
independent copy of that value. Frontend detail summaries and
Docker page search consume those backend-authored fields before falling back to
legacy labels.
Docker host duplicate-identity evidence is monitoring-authored and rides
`DockerData.IdentityConflict` unchanged: the adapter, resource clone, and
typed `DockerHostView` accessor must each hand out an independent copy so
concurrent snapshot replacement cannot alias the conflict's hostname and
machine-ID lists, and read-state reconstruction back to `models.DockerHost`
must preserve the field rather than dropping it. Consumers render the
conflict as a warning about cloned machines sharing `/etc/machine-id`; they
must not reinterpret it as staleness, or synthesize a second host row from
the flapping values.
Docker network rows must consume canonical runtime attachment relationships,
not page-local topology inference, when the unified-resource snapshot provides
them. Runtime container-to-network membership is represented as active
`attached_to` relationships from the `app-container` resource to the
`docker-network` resource, scoped to the reporting Docker host and discoverer,
with the same edge visible on both resources so network workflow tables and
resource drawers can explain the relationship from either endpoint. The
Docker Networks table may keep a host-scoped legacy `docker.networks[]`
fallback only for older snapshots that have not yet published the relationship
edge; that fallback must not match containers across Docker hosts. When a
single network has many attached containers, relationship consumers must keep
the attached container fields searchable and attention-filterable from the
network detail disclosure instead of forcing operators to inspect a full
container inventory table.

1. `frontend-modern/src/components/Infrastructure/infrastructureSelectors.ts` shared with `performance-and-scalability`: the infrastructure selector pipeline is both a canonical unified-resource consumer surface and a fleet-scale performance hot-path boundary.
2. `frontend-modern/src/components/Infrastructure/resourceDetailMappers.ts` shared with `performance-and-scalability`: resource detail mappers are both a canonical unified-resource consumer surface and a fleet-scale performance hot-path boundary.
3. `frontend-modern/src/components/Infrastructure/UnifiedResourceHostTableCard.tsx` shared with `performance-and-scalability`: the unified resource host table card is both a canonical unified-resource consumer surface and a fleet-scale performance hot-path boundary.
4. `frontend-modern/src/components/Infrastructure/UnifiedResourcePBSTableSection.tsx` shared with `performance-and-scalability`: the unified resource PBS section is both a canonical unified-resource consumer surface and a fleet-scale performance hot-path boundary.
5. `frontend-modern/src/components/Infrastructure/UnifiedResourcePMGTableSection.tsx` shared with `performance-and-scalability`: the unified resource PMG section is both a canonical unified-resource consumer surface and a fleet-scale performance hot-path boundary.
6. `frontend-modern/src/components/Infrastructure/UnifiedResourceServiceInfrastructureCard.tsx` shared with `performance-and-scalability`: the unified resource service infrastructure card is both a canonical unified-resource consumer surface and a fleet-scale performance hot-path boundary.
7. `frontend-modern/src/components/Infrastructure/UnifiedResourceTable.tsx` shared with `performance-and-scalability`: the unified resource table is both a canonical unified-resource consumer surface and a fleet-scale performance hot-path boundary.
8. `frontend-modern/src/components/Infrastructure/unifiedResourceTableModel.ts` shared with `performance-and-scalability`: unified resource service row shaping and I/O emphasis are both a canonical unified-resource consumer surface and a fleet-scale performance hot-path boundary.
9. `frontend-modern/src/components/Infrastructure/unifiedResourceTableStateModel.ts` shared with `performance-and-scalability`: unified resource table state derivation, sort-cycle policy, service sorting, and responsive column layout are both a canonical unified-resource consumer surface and a fleet-scale performance hot-path boundary.
10. `frontend-modern/src/components/Infrastructure/useUnifiedResourceTableState.ts` shared with `performance-and-scalability`: unified resource table state, grouping, and windowing are both a canonical unified-resource consumer surface and a fleet-scale performance hot-path boundary.
11. `frontend-modern/src/components/Infrastructure/useUnifiedResourceTableViewportSync.ts` shared with `performance-and-scalability`: unified resource table viewport sync and selected-row reveal are both a canonical unified-resource consumer surface and a fleet-scale performance hot-path boundary.
12. `frontend-modern/src/features/proxmox/ProxmoxBackupServersTable.tsx` shared with `storage-recovery`: Proxmox backup server table rows are both a storage/recovery backup-health surface and a unified-resource platform-table consumer boundary.
13. `frontend-modern/src/features/proxmox/ProxmoxCoverageTable.tsx` shared with `storage-recovery`: Proxmox workload coverage rows are both a storage/recovery protection-posture surface and a unified-resource identity consumer boundary.
14. `frontend-modern/src/features/proxmox/ProxmoxRecoverableTable.tsx` shared with `storage-recovery`: Proxmox recoverable workload table rows are both a storage/recovery coverage surface and a unified-resource platform-table consumer boundary.
15. `frontend-modern/src/routing/routePreload.ts` shared with `frontend-primitives`, `performance-and-scalability`: the app-shell route preload registry is a canonical frontend shell boundary, an authenticated hot-path performance boundary, and the entry point for the unified-resource Actions workspace.
16. `frontend-modern/src/utils/platformSupportManifest.generated.ts` shared with `frontend-primitives`: the generated platform support projection is both a canonical unified-resource platform union boundary and a shared frontend source/platform vocabulary boundary.
    It must carry the manifest `surface_kind` distinction so `docker` remains
    machine-readable as a `runtime-lens` while owning infrastructure sources
    remain `platform` entries.
17. `frontend-modern/src/utils/sourcePlatforms.ts` shared with `frontend-primitives`: the source platform normalizer is both a canonical unified-resource source adapter boundary and a shared frontend source/platform vocabulary boundary.
    That shared vocabulary boundary owns the generic `docker` platform label:
    selectors, badges, and filter options render it as "Docker / Podman" so
    v5 Docker users can still find the runtime surface while Podman-backed
    rows are not mislabeled as Docker-only.
    That same boundary also owns `resolveResourcePlatformType(resource)`,
    the canonical reader for "what platform family does a unified resource
    belong to". Consumers that bucket unified resources by family (platform
    pages, filter resolvers, presentation pickers) must call it instead of
    inspecting `resource.platformType` directly: the legacy backend resource
    projection leaves `platformType` empty on several canonical resource
    types, and the helper falls back through the existing source-platform
    normalization so client-side grouping behaves identically against mock
    fixtures and live backends.
    `Resource.PlatformScopes` is the backend-owned platform page membership
    contract for unified resources. It is derived by unified resources through
    `RefreshPlatformScopes`, transported through `useUnifiedResources`,
    `useWorkloads`, and `sourcePlatforms.ts`, and consumed by workload filters
    without page-local reinterpretation. `platformType` remains the primary
    display/source family; `platformScopes` is the overlap set used when a
    runtime workload belongs to both Docker and an owning infrastructure
    platform.
18. `internal/api/resources.go` shared with `api-contracts`: the unified resource endpoint is both a backend payload contract surface and a unified-resource runtime boundary.
    `/api/resources` type filters must accept URL-encoded comma-separated lists
    from browser query builders exactly like literal comma separators, so Docker
    / Podman runtime pages do not lose `docker-host` inventory while requesting
    canonical resource types.
    App-container Discovery targets are part of that shared payload contract.
    Backend resources must expose frontend-facing `resourceType=app-container`
    with the reporting Docker/Podman agent id and the stable container name as
    the target resource id; browser Discovery clients are responsible for
    translating that frontend resource type to the Discovery API's `docker`
    route type. Docker host resources must likewise prefer the reported agent
    id over host labels when building host-level Discovery targets, so detail
    drawers, websocket hydration, and API lookups use the same identity.
    The global resource timeline is also owned at this boundary. `GET
    /api/resources/timeline` may expose provider-wide `ResourceChange` records
    for platform pages before a single resource drawer is selected, but those
    records must still come from the canonical resource-change store and use
    the same filter parser as per-resource timelines. Relationship-aware
    expansion remains a per-resource timeline behavior; unscoped provider
    activity must not infer related resources in the frontend.
## Extension Points

The desktop Product Trust projection is owned at
`frontend-modern/src/features/actions/` with the route shell in
`frontend-modern/src/pages/Actions.tsx`. It must consume durable
`ActionAuditRecord.plan.policyDecision` and `result.actionResultV2` without
deriving policy authority or collapsing execution, verification, and recovery
into one outcome. Docker lifecycle controls may create a canonical plan and
open this shared review, but may not auto-approve, auto-execute, or use a
second-click local confirmation. Proof is owned by the colocated action tests,
`DockerNativeTables.test.tsx`, and desktop journeys 81 and 83.
Patrol findings may retain bounded action status and safety context, but exact
typed action ids hand off to the route-backed Actions review through
`frontend-modern/src/features/actions/actionRouting.ts`. The `action` query
parameter is the canonical shareable review identity: opening it fetches the
durable action directly, selects Open or History from the returned lifecycle
state, and clearing the dialog removes the query without discarding the inbox.
Patrol must not create a parallel decision or execution client around that
handoff.
The shared review may offer decision or execution controls only when the
current record carries a non-empty `planHash`; every mutation passes that exact
displayed identity through the canonical resource-action client. Legacy or
mixed-version records without it remain reviewable but require replanning and
cannot create a browser mutation.

1. Add new resource types and identity fields in `internal/unifiedresources/types.go`
   Agentless availability endpoints enter the model as
   `ResourceTypeNetworkEndpoint` with `SourceAvailability` and
   `AvailabilityData`. Canonical identity must prefer the saved target id as
   `availability:<target-id>`, keep the probe address as hostname/platform
   identity when no stronger identity exists, and preserve availability payloads
   through clone, merge, API transport, and frontend decode paths.
   Governed action execution freshness is part of the same resource-extension
   contract: API execution consumers must re-plan against current resource
   capabilities and policy before executor dispatch, treat action id,
   `resourceVersion`, `policyVersion`, or `planHash` drift as
   `action_plan_drift`, and record/publish a failed audit instead of leaving
   executor adapters to make stale-approval decisions.
   Docker/Podman `app-container` lifecycle actions are part of that same
   governed resource contract: `resourceFromDockerContainer` may advertise
   `start`, `stop`, and `restart` only for fresh, supported, agent-backed
   Docker/Podman reports whose daemon posture does not block mutating
   commands. An `update` capability rides the same gate with two extra
   preconditions: the report must carry a detected image update
   (`updateStatus.updateAvailable`) and a current image digest, because the
   plan binds the typed update dispatch to that digest and a container whose
   digest cannot be stated cannot be safely updated. The API action executor
   must consume those capabilities through
   `/api/actions/*` and the agent command server; platform rows or drawers
   must not wire direct shell, SSH, provider, or runtime calls around this
   capability contract.
   API resource responses must project only currently executable lifecycle
   capabilities: when the command agent that owns the Docker / Podman runtime is
   not connected, `resources.go` filters those capabilities from list and
   detail payloads before frontend controls render. Planning still rechecks the
   same readiness boundary and fails closed with `action_execution_unavailable`
   before audit creation if the live command path disappears after the resource
   response was read.
   When filtering an otherwise known lifecycle capability, the same resource
   response must preserve a typed `actionReadiness` entry with the action name,
   `available=false`, a stable reason code such as
   `command_agent_disconnected`, and operator-safe copy. Frontend consumers may
   use that field to explain disabled controls, while `capabilities` remains
   the executable action set.
   Proxmox VM and LXC lifecycle actions are also part of that governed
   resource contract. `resourceFromVM` and `resourceFromContainer` may advertise
   `start` only for stopped guests and `shutdown`, `reboot`, and `stop` only
   for running guests, and must fail closed for templates, locked guests, and
   unknown transition states. The API action executor consumes those
   capabilities through `/api/actions/*`, resolves the Proxmox node command
   agent from the unified resource's linked node context, and records
   `actionReadiness` when that command path is not currently connected.
   Unified-resource consumers must not infer Proxmox lifecycle affordances from
   table row status alone or issue direct `qm` / `pct`, SSH, provider API, or
   guest-agent calls outside the governed action contract.
   The lifecycle executor must project terminal truth through `ActionResultV2`
   without coupling execution success to verification. Same-agent status reads
   remain `agent_attested`; only a fresh, identity-matched read through the
   tenant's server-side Proxmox API client may be `independent`. Reboot
   confirmation additionally requires an uptime reset relative to the direct
   pre-action observation, not merely a post-action `running` status.
   Agent-managed Linux hosts may advertise `install_os_updates` only when the
   report carries supported APT package posture and typed command operations
   are enabled. The capability is admin-floor, `elevated` auto-authorization
   eligible, parameter-free, and routed through `host.package_updates`.
   Package names and versions are observed evidence, never caller-selected
   mutation input. The agent-authored inventory fingerprint binds execution to
   the observed set without turning that set into caller authority. API
   readiness must filter the capability when inventory is
   stale, erroneous, empty, unsupported, or its owning agent is disconnected;
   reboot-required state remains descriptive and does not imply a reboot
   capability.
   Agent-managed hosts may advertise `clean_package_cache` only when typed
   commands are enabled, the agent reports a fresh error-free
   `apt-package-cache` fingerprint with at least 64 MiB reclaimable, and the
   actual longest-prefix filesystem containing the fixed cache target is at
   least 90% used. A healthy separate `/var` mount must override pressure on
   `/`; sibling-prefix mounts must never match. The capability is admin-floor,
   `low_risk` auto-authorization eligible, parameter-free, and routed through
   `host.storage_cleanup`. API consumers must reuse the canonical target-disk
   resolver and may not infer cleanup authority from generic disk usage.
   TrueNAS app inventory enters the model as native `TrueNASData.App`
   metadata on canonical `app-container` resources. The facet is sourced from
   the TrueNAS API app inventory (`app.query` plus active workload/stat
   details where available), preserves app id/name/state/version/update
   signals, containers, ports, images, volumes, networks, and app stat
   collection metadata, and is cloned and merged by unified resources before
   transport. Docker metadata may still ride beside the app as compatibility
   runtime evidence, but the TrueNAS platform overview and other
   TrueNAS-native consumers must read the native facet first instead of
   treating TrueNAS apps as generic Docker rows.
   TrueNAS VM and network-share inventory use the same native-facet rule.
   TrueNAS API VMs publish `TrueNASData.VM` on canonical `vm` resources, and
   SMB/NFS shares publish `TrueNASData.Share` on canonical `network-share`
   resources. Share records must preserve protocol, path, dataset,
   enabled/read-only/access metadata, clone and merge through unified
   resources, redact filesystem paths through policy metadata, and derive
   storage-parent relationships without being flattened into generic Docker or
   storage rows.
   TrueNAS service inventory remains a system-owned native facet rather than a
   new generic resource type. `service.query` output must project into
   `TrueNASData.Services` on the canonical top-level `agent` resource,
   preserving service name, boot enablement, runtime state, and process IDs
   through clone, merge, transport, and frontend decode paths.
   Docker / Podman inventory extends that same canonical type contract beyond
   generic workload rows and Swarm services. Runtime containers must preserve
   Docker Engine container identity, owning host/runtime context, image,
   state/health/restart/update status, ports, networks, and mounts through
   `DockerData` so Docker Overview and its `/docker/containers` compatibility
   route can use the native `DockerContainersTable` rather than
   `WorkloadsSurface`. Runtime image,
   volume, network, task, Swarm node, Swarm secret, Swarm config, and
   storage-usage evidence must enter through `DockerData` and the typed Docker
   resource records (`docker-image`, `docker-volume`, `docker-network`,
   `docker-task`, `docker-swarm-node`, `docker-secret`, `docker-config`) rather
   than being inferred inside the container page. Swarm node records must
   preserve node id, hostname, role, availability, state, manager reachability,
   manager address, leader state, engine version, platform, resource capacity,
   labels, and engine labels under the owning Docker host or Swarm cluster
   identity.
   Docker container network membership also extends the relationship contract:
   `internal/unifiedresources/registry.go` must emit host-scoped
   `attached_to` edges from runtime containers to reported Docker networks,
   preserving network name and reported IPv4/IPv6 attachment metadata on the
   relationship instead of requiring frontend consumers to reconstruct topology
   from raw container facets.
   Swarm secret and config records are metadata projections only. They may
   preserve id, name, labels, driver/template metadata, and timestamps, but
   Docker secret/config payload bytes are outside the unified-resource contract.
   Docker secrets are restricted/local-only resource metadata; Docker configs
   are sensitive/local-first resource metadata.
   Host `/system/df` buckets remain Docker host facet data, not generic storage
   resources. Podman libpod pod records must not be projected until a
   libpod-native source can populate a native contract instead of deriving pods
   from Docker-compatible container labels.
   Kubernetes inventory likewise projects native API objects as first-class
   unified resources: Nodes, namespaces, Services, ReplicaSets, StatefulSets,
   DaemonSets, Jobs, CronJobs, Ingresses, EndpointSlices, NetworkPolicies,
   PersistentVolumes, PersistentVolumeClaims, StorageClasses, ConfigMaps,
   Secrets, ServiceAccounts, Roles, ClusterRoles, RoleBindings,
   ClusterRoleBindings, ResourceQuotas, LimitRanges, PodDisruptionBudgets,
   HorizontalPodAutoscalers, and Events must preserve their API identity and
   source metadata through clone, merge, REST, websocket, and frontend decode
   paths. RBAC resources carry summary counts only — rule count, role kind /
   role name (for bindings), subject count, subject Kinds, and ClusterRole
   aggregation labels — and never expose full PolicyRule contents or
   individual subject names (User / Group / ServiceAccount). `k8s-configmap` and `k8s-secret`
   resources must preserve the `metadataOnly` flag when the agent used the
   Kubernetes metadata-only API path; current agents must not fetch ConfigMap
   or Secret payload values merely to build inventory rows. Older reports may
   carry key names, but Secret values are outside the unified resource contract.
   Because Secret names and key names can disclose intent, `k8s-secret` policy
   metadata must classify them as `restricted` with `local-only` routing.
   Resource sensitivity classification (`classifyResourceSensitivity` in
   `internal/unifiedresources/policy_metadata.go`) is calibrated for Pulse's
   homelab/SMB audience: an ordinary compute workload's name and private LAN IP
   are not secrets, so VMs, system/app containers, pods, Kubernetes workload
   objects (Deployments, ReplicaSets, StatefulSets, DaemonSets, Services, Jobs,
   CronJobs, Ingresses, Namespaces, ...), Docker services, and Swarm nodes
   classify as `internal` (cloud-summary, no redaction) and are visible to cloud
   models. Redacting them by default reduced the cloud Assistant to "redacted by
   policy" noise. Escalation to `sensitive`/`restricted` is by tag (`database`,
   `backup`, `customer-data`, `secret`, `pii`, ...) or by genuinely sensitive
   TYPE: data-at-rest and storage (`storage`, `pbs`, `ceph`, `physical-disk`,
   `network-share`, `network`, k8s `pv`/`pvc`/`storage-class`), configuration
   (`docker-config`, `k8s-configmap`), and security (k8s RBAC roles/bindings and
   service accounts, k8s/docker secrets, PMG). The rule of thumb: a workload's
   identity is not sensitive by default; data, configuration, and security
   resources are.
   Kubernetes Node inventory is an API-native resource surface, not merely
   overview chrome: `/kubernetes/nodes` must expose the bespoke node table while
   the overview may still include the same rows for cluster orientation.
   Kubernetes Service inventory is likewise API-native: `/kubernetes/services`
   must render Service rows through a service-specific table that preserves
   cluster/namespace scope, Service type, ClusterIP, external IP metadata,
   ServicePort/targetPort/nodePort publication, and selectors instead of
   routing those API objects through the generic Kubernetes inventory table.
   Kubernetes storage inventory is likewise API-native: `/kubernetes/storage`
   must render StorageClass, PersistentVolume, and PersistentVolumeClaim rows
   through a storage-specific table that preserves cluster scope, binding mode
   or phase, class, size/request, access/reclaim policy, provisioner, and
   PV/PVC binding targets instead of routing those API objects through the
   generic Kubernetes inventory table.
   Kubernetes services inventory follows the same native-table rule:
   `/kubernetes/services` must render Service, Ingress, and EndpointSlice rows
   through service and networking-specific tables that preserve
   cluster/namespace scope, Service type, ClusterIP/external IPs, service ports,
   selectors, Ingress class, hosts and addresses, EndpointSlice address type,
   ready endpoint counts, endpoint ports, and service targets instead of routing
   those API objects through the generic Kubernetes inventory table. Legacy
   `/kubernetes/networking` resolves to that Services workflow tab.
   Kubernetes configuration inventory is likewise API-native and trust-sensitive:
   `/kubernetes/configuration` must render Namespace, ConfigMap, Secret,
   ServiceAccount, Role, ClusterRole, RoleBinding, ClusterRoleBinding, and
   policy rows through config and policy-specific tables that preserve
   cluster/namespace scope, Namespace lifecycle phase, ConfigMap/Secret
   immutable and type metadata, ServiceAccount token/image-pull/secret
   references, RBAC summary fields (rule counts, role kind / role name,
   subject count, subject Kinds, aggregation labels), metadata-only collection
   wording, and policy-specific health/count fields. Legacy `/kubernetes/config` and
   `/kubernetes/policy` resolve to that Configuration workflow tab. Config
   tables must not render ConfigMap or Secret payload values, and metadata-only
   rows must not expose key names as if payload fields had been read.
2. Add typed accessors and views in `internal/unifiedresources/views.go`
Resource detail mappers now reuse the shared
`frontend-modern/src/utils/textPresentation.ts` title-case helper for sensor
labels so the canonical unified-resource presentation layer owns the wording.

The canonical AI-safe summary builder now owns the sensitivity-specific suffix
phrases for `sensitive` and `restricted` resources, so the backend policy
contract controls those strings instead of duplicating them inside the summary
assembly branch.
Canonical policy presentation and exact-value redaction helpers are owned in
`internal/unifiedresources/policy_presentation.go`. AI, Patrol, alert, export,
and prompt consumers must use those helpers for governed resource names,
hostnames, IP addresses, platform IDs, aliases, and paths instead of inventing
consumer-local scrubbers. When a consumer has product-originated resource
references that are not guaranteed to be present on the canonical resource
record, it must pass them through the shared redaction-reference helper rather
than recomputing local policy checks.
Canonical policy posture aggregation is owned here as well. Resource API
payloads may expose a camelCase transport projection, but the counts must be
derived from `internal/unifiedresources/policy_posture.go` after canonical
policy metadata has been refreshed, not recomputed from frontend labels,
AI-only summary payloads, or page-local heuristics.
4. Add metrics-target normalization, surface-friendly projections of
   nested source payloads, or synthetic metrics support through
   `internal/unifiedresources/metrics_targets.go`,
   `internal/unifiedresources/metrics.go`, and the relevant adapter in
   `internal/unifiedresources/adapters.go`. The unified `Resource` shape
   carries top-level `Uptime` and `Temperature` projections so frontend
   tables that render those columns do not have to dig into per-source
   payloads (`agent.uptimeSeconds`, `proxmox.uptime`,
   `agent.temperature`, `proxmox.temperature`); adapters that wrap an
   `AgentData` or `ProxmoxData` must populate those top-level fields
   from the nested source values, and adapters for resource types that
   have no native uptime/temperature concept (e.g. `k8s-deployment`,
   `k8s-replicaset`, `k8s-configmap`, `k8s-secret`,
   `docker-service`, `k8s-cluster` aggregates) must leave them unset so
   bespoke platform-page tables can hide the column entirely.
   Kubernetes deployment metrics live on the canonical adapter through
   `metricsFromKubernetesDeployment(cluster, deployment)`. Upstream
   Deployments do not expose CPU / memory natively because they are
   scheduling abstractions over their controlled pods, so the helper
   returns nil for non-mock runtimes (until the adapter aggregates pod
   metrics into the owning deployment) and synthesizes deployment-stable
   CPU / memory / disk / network values for mock mode so the
   platform-page Deployments table renders meaningful operator values
   instead of dashes. The synthetic branch is gated by
   `mockmode.IsEnabled()` and scales with the deployment's
   ready/desired/available replica state so degraded deployments read as
   elevated pressure on the surviving replicas.
   Namespaced Kubernetes adapters share one Resource scaffold: a new
   namespaced kind populates its kind-specific `K8sData` fields (after
   `baseKubernetesData`) and delegates Resource assembly and identity to
   `namespacedKubernetesResource(cluster, clusterName, namespace, name,
   resourceType, status, data, labels)` in
   `internal/unifiedresources/adapters.go` instead of hand-rolling the
   `Resource{...}` literal plus `namespacedKubernetesIdentity` return.
   The scaffold owns `Technology: "kubernetes"`, `LastSeen` from the
   cluster, `UpdatedAt`, the `Kubernetes` facet pointer, and label-derived
   tags; only cluster-scoped or non-namespaced kinds (cluster, node, PV,
   StorageClass, namespace itself) keep bespoke identity construction.
5. Add platform registry, resolution, host-dedup, or monitored-system
   projection behavior through `internal/unifiedresources/registry.go`,
   `internal/unifiedresources/resolve.go`,
   `internal/unifiedresources/resolved_host_set.go`,
   `internal/unifiedresources/snapshot_source_filter.go`,
   `internal/unifiedresources/store.go`,
   `internal/unifiedresources/kubernetes_capabilities.go`,
   `internal/unifiedresources/pbs_rollups.go`,
   `internal/unifiedresources/monitored_systems.go`,
   `internal/unifiedresources/monitored_system_projection.go`, and
   the shared list-order helpers consumed by `internal/api/resources.go`;
   canonical unified-resource lists must preserve one deterministic
   `name -> type -> id` order across registry reads, REST pagination, and
   websocket-backed refreshes so equal-name resources do not silently reshuffle
   between cold hydrate and later runtime updates
   That same unified-resource owner also defines the canonical transport
   projection for operator-facing resources: `/api/resources` and websocket
   `state.resources` must share `ContractResourceType`, canonical display
   names, and canonical cluster labels instead of publishing separate REST and
   broadcast aliases for the same machine.
   Fleet command posture that reaches resource-facing rows must remain a
   projection of `/api/connections` `fleet.commandPolicy`: desired server
   policy, applied agent truth, enforcement, and reason stay separate. Unified
   resource consumers may show compact remote-control status, but they must not
   treat top-level `remoteControl` as applied agent runtime truth, and they
   must preserve desired/applied drift or no-report attention when enriching
   resource rows.
   Platform-page stale-agent notices may consume canonical agent identity from
   merged resources only to scope the Infrastructure settings update-command
   route to the affected agents. That scoped lifecycle handoff must not become a
   new resource-action authority, a page-local command runner, or a substitute
   for the `/api/connections` fleet command-policy truth described above.
   Resource consumers must also use the API-owned agent update target when
   comparing resource-carried agent versions; the running app build version is
   not a resource freshness contract.
   Kubernetes node rows are cluster-agent-backed for this purpose: even when a
   canonical `k8s-node` row is a pure Kubernetes API projection with no merged
   `agent` facet, `internal/unifiedresources/adapters.go` must carry the
   cluster `AgentID` and cluster-scoped `AgentVersion` on the row's
   Kubernetes facet so platform consumers can scope stale-agent notices and
   update-command links from typed resource evidence instead of rebuilding
   ownership from the parent cluster row.
   `internal/unifiedresources/top_level_systems.go`
   Explicit linked-host correlation is canonical here: when Kubernetes node
   ingest has a resolved backing host agent, the registry must merge that node
   into the agent resource instead of publishing duplicate top-level
   infrastructure rows for the same machine under both `agent` and `k8s-node`
   identities.
   Canonical read-state overlays belong here as well: when monitoring or a
   preview path needs to project extra source-native records onto an existing
   settled read state, it must do so through
   `internal/unifiedresources/monitor_adapter.go` and
   `internal/unifiedresources/registry.go` so matcher seeding, manual links,
   and merge semantics stay unified-resource-owned instead of being rebuilt in
   consumers.
   Storage consumer projection is unified-resource-owned through
   `internal/unifiedresources/storage_consumers.go`. When a provider publishes
   source-native storage consumer metadata that cannot be derived from shared
   Proxmox/PBS relationship indexes, refresh must preserve that source-owned
   consumer count, consumer type list, and top-consumer summary on the
   canonical storage resource unless a stronger shared consumer projection has
   already populated those fields in the same refresh.
   Operator-facing storage posture wording is part of that same ownership:
   when multiple storage-risk reasons exist, shared posture helpers must prefer
   the most decision-useful protection loss summary such as lost parity over a
   generic disk-count aggregate, so resource drawers and incidents do not hide
   the actual protection boundary behind a broader count phrase.
6. Add canonical governed name-resolution or policy-aware resource lookup behavior through `internal/unifiedresources/resolve.go` and `internal/unifiedresources/resolve_context.go`
8. Add or change discovery-support runtime under the resource drawer through `frontend-modern/src/components/Discovery/DiscoveryTab.tsx` for shell/presentation ownership, `frontend-modern/src/components/Discovery/useDiscoveryTabState.ts` for fetch, websocket-progress, manual-run triggering, and notes-mutation ownership, and `frontend-modern/src/components/Discovery/discoveryReadiness.ts` for the shared readiness verdict used by resource-drawer Discovery surfaces. Embedded drawers may expose the top-level run action through this shared Discovery tab, but they must still call the canonical discovery trigger state path instead of introducing drawer-local API mutations.
   Resource drawer secondary sections, action history, discovery run summaries,
   and other compact resource-detail cards may own their resource-specific
   labels, rows, filters, and actions, but the repeated bordered compact frame
   is a frontend-primitives boundary. `ResourceDetailDrawerOverviewTab.tsx`,
   `ResourceActionHistory.tsx`, and `DiscoveryTab.tsx` must compose
   `InfoCardFrame` for that shell instead of restoring local card-frame
   classes.
9. Keep dashboard and infrastructure freshness on the canonical unified-resource
   ownership path. `frontend-modern/src/stores/websocket.ts`,
   `frontend-modern/src/utils/resourceStateAdapters.ts`, and
   `frontend-modern/src/hooks/useUnifiedResources.ts` together own the frontend
   canonicalization boundary: REST may hydrate the initial snapshot and
   unsupported filtered queries, but supported snapshot freshness must come
   from websocket `state.resources` instead of layering confirmatory
   route-local REST refetch loops over already-owned resource
   updates.
   Browser WebSocket liveness tracking is part of that same store boundary:
   valid inbound server messages, including heartbeat `ping`/`pong` traffic,
   must refresh the browser-side activity timestamp so quiet periods between
   resource snapshots do not cause avoidable reconnect churn.
   That shared store/adapter/hook path must also preserve canonical row shape
   across transport boundaries: thinner realtime `state.resources` payloads
   must merge into the existing canonical resource snapshot instead of
   downgrading richer REST-only infrastructure details such as disk I/O, source
   metadata, or platform summary fields after first hydrate. For default
   Source lists and their source-specific facets are the exception: a current
   snapshot with canonical source evidence replaces stale source lists and
   removes provider facets that no longer have matching source evidence, so
   rows do not keep displaying a previous platform identity after websocket
   refreshes. For default
   `initialHydration: 'immediate'` consumers, that same path must not paint the
   thinner websocket transport before the first canonical REST snapshot exists;
   only explicit websocket-first consumers may render directly from the realtime
   transport before canonical hydrate completes. Operator surfaces that must
   preserve already-known infrastructure continuity after login, such as the
   Infrastructure page, must use websocket-first hydration with stale-cache
   REST revalidation after the first-paint settle window so the page can paint
   from live state immediately without forcing a second resource-shape
   transition while summary and table surfaces are still mounting.
   Org-scope and enabled-state transitions in
   `frontend-modern/src/hooks/useUnifiedResources.ts` must invalidate older
   in-flight REST refreshes before publishing the new scoped cache entry, so a
   stale request cannot set active-scope errors, clear the active request guard,
   or replace the currently mounted Infrastructure/Workloads resource snapshot.
   Canonical cluster membership in that shared path must come only from
   explicit cluster identity such as Kubernetes context or platform cluster
   labels; standalone resource names must never be repurposed as synthetic
   `clusterId` values.
13. Keep operator-facing resource analysis vocabulary task-first on unified-resource
    surfaces. `frontend-modern/src/components/Infrastructure/ResourceDetailDrawerOverviewTab.tsx`,
    `frontend-modern/src/components/Infrastructure/useResourceDetailDrawerDerivedState.ts`,
    and `frontend-modern/src/components/Discovery/DiscoveryTab.tsx` may expose
    provider identity or governed safe-summary posture when that context helps
    an operator, but the rendered labels must stay product-neutral and use
    `Analysis`, `Analysis Reasoning`, and `Safe Summary` rather than reviving
    generic `AI` or `AI-Safe` branding inside the resource drawer or discovery
    shell.
14. Keep the operator-facing unified resource table width-aware at the table
    surface, not just at the browser viewport. `frontend-modern/src/components/Infrastructure/UnifiedResourceTable.tsx`
    must route its root ref through `frontend-modern/src/components/Infrastructure/useUnifiedResourceTableState.ts`,
    and `frontend-modern/src/components/Infrastructure/unifiedResourceTableStateModel.ts`
    owns the column-priority breakpoints for host and service infrastructure
    rows. When the app shell leaves tablet-sized space during live resize, the
    table hides lower-priority metadata first, then preserves a readable
    640-pixel floor for the remaining identity and health columns inside the
    shared table scroll shell. The document must not overflow, but the table
    must not compress the prioritized mobile columns until resource names and
    metric headings become ambiguous.
15. Keep shared policy-posture framing on the unified-resource card owner.
    `frontend-modern/src/components/Infrastructure/ResourcePolicySummary.tsx`
    may accept caller-owned subtitle or resource-count wording when Patrol or
    another shared surface needs to explain how the same governed policy counts
    should be read, but those framing lines must extend the shared card API
    rather than spawning page-local policy summary shells.
16. Keep platform/runtime top-level route paths on the canonical resource-link
    helper. `frontend-modern/src/routing/resourceLinks.ts` owns the
    `STANDALONE_PATH`, `DOCKER_PATH`, `KUBERNETES_PATH`, `TRUENAS_PATH`,
    `VMWARE_PATH`, `PATROL_PATH`, `PATROL_CONTROL_ANCHOR`,
    `PATROL_CONTROL_PATH`, and `PATROL_CONTROL_STARTER_QUERY_PARAM` constants,
    the route-backed Patrol control starter helpers, and the `buildStandalonePath`,
    `buildDockerPath`, `buildKubernetesPath`, `buildTrueNASPath`,
    `buildVmwarePath` builders.
    Per-platform surfaces and tab specs must
    derive every internal link from those builders so the canonical resource
    URL vocabulary stays single-sourced; ad hoc string concatenation of
    platform routes inside feature directories is not permitted. The canonical
    Pulse Intelligence external-agent hash
    `/settings/pulse-intelligence/assistant#external-agent-setup` and legacy
    `/settings/security/api#external-agent-setup` /
    `/settings/security/api#pulse-mcp-setup` compatibility hashes may live in
    the shared route helper, but they are adjacent settings route state, not
    unified-resource identity, platform scope, or drawer focus state.
    The Patrol `patrolControlStarter=patrol_control` query is an adjacent
    first-party Patrol control handoff flag, with legacy
    `operationsLoopStarter` values accepted only as compatibility aliases, not
    unified-resource filters or focus keys. Unified-resource consumers must not reuse those values for resource
    identity, list filtering, contextual focus, storage state, recovery state,
    or platform scoping.
    The user-facing Machines surface's default resource route is the machines projection
    (`/standalone/machines`); agentless endpoint rows use the
    `/standalone/availability` projection and must not be collapsed into a
    generic overview URL.
    The frontend-primitives-owned Machines IA contract consumes the
    unified-resource projection for Pulse-managed standalone agent rows and
    agentless availability endpoint rows; this subsystem owns only the
    membership rules for those projected rows. Agent membership must require
    `resource.type === "agent"`, canonical Pulse-agent source evidence from
    resource sources or source status, and no stronger provider-owner evidence
    from Proxmox, VMware, TrueNAS, or Kubernetes. Source-less legacy snapshots
    may fall back to a normalized `platformType === "agent"`, but
    provider-owned nodes must not become machine-page members through
    hostname, `agent` platform scope, or agent telemetry alone; those facts
    surface as facets on the owning provider page.
    `AgentsMachinesTable.tsx` may own row membership, resource-derived menu
    eligibility, and remove-agent semantics for these projected rows, but the
    compact row action trigger chrome stays under the frontend-primitives
    `ActionIconButton` boundary rather than becoming a unified-resource-local
    button shell.
    The default tab for each platform path must point at a sub-tab whose
    canonical unified-resource projection actually populates, and visible
    workflow subtabs must stay evidence-gated by the same canonical row or
    signal source instead of advertising empty object browsers. The
    canonical TrueNAS adapter (`internal/truenas/provider.go::
    truenasRecordsFromSnapshot`) already emits the top-level TrueNAS
    appliance as a unified `agent` row tagged with the `truenas`
    platform, so TrueNAS defaults to `/truenas/overview` (the Systems
    sub-tab); the embedded `StorageSurface` lives at `/truenas/storage`.
    Any future platform that wants to default to a Systems / Hosts
    overview must first have its canonical resource adapter project the
    platform's top-level system as a unified resource so the builder
    default still resolves to a populated table.

17. Platform table ordering is a two-layer contract. Each platform table's
    default order is owned by its page model's status-first compare
    (`compareDockerContainers`, `compareKubernetesPods`, and peers), which
    keeps unhealthy rows surfaced without user configuration. User-selected
    column sorting layered on top must come from the shared platform-table
    sort fabric in
    `frontend-modern/src/features/platformPage/sharedPlatformPage.tsx`
    (`createPlatformTableSortState` plus `PlatformSortableTableHead`) as
    governed by the frontend-primitives contract, must never replace the
    built-in compare as the default order, and must keep missing metric
    values at the bottom in both directions so sparse agent data cannot
    float to the top of an ascending sort. This restores the v5
    sortable-table capability (v5.1.36 Docker and Kubernetes tables) that
    the v6 platform rebuild had dropped.

## Forbidden Paths

1. New ad hoc resource-type aliases outside unified resource normalization
2. New duplicate ID normalization logic outside unified resources
3. Reintroducing legacy runtime resource contracts as live truth

## Completion Obligations

1. Update this contract when canonical resource identity or type rules change
2. Update contract and guardrail tests when a new resource type is added
3. Route runtime changes through the explicit unified-resource proof policies in `registry.json`; default fallback proof routing is not allowed
4. Tighten banned-path tests when a compatibility bridge is removed
5. Keep the infrastructure landing empty state on canonical first-run routing:
   when inventory is empty, `frontend-modern/src/components/Settings/InfrastructureWorkspace.tsx`
   and `frontend-modern/src/components/Settings/InfrastructureSourceManager.tsx`
   must send operators directly to `/settings/infrastructure?add=pick`, name
   source strategy selection as the default next step, and present platform
   API inventory and Pulse Agent telemetry as peer source options instead of
   regressing to generic settings-root CTAs, an agent-only install jump, the
   retired `Platform connections` split, provider-specific one-off routes, or
   the removed top-level `/infrastructure` page. Agentless availability
   endpoints are the separate monitoring source home at
   `/settings/monitoring/availability`, and the add flow for MQTT, ping, TCP,
   and HTTP checks is `/settings/monitoring/availability?add=target`. Machine
   add handoffs must include `targetKind=machine`; service and device check
   handoffs must include `targetKind=service`. Saved availability targets carry
   a `targetKind` so agentless servers, laptops, desktops, and comparable
   computers can use machine-specific reachability copy in Availability checks
   without reclassifying service or device checks as machines. Machines
   inventory membership still requires Pulse Agent resource evidence.
6. Keep infrastructure source visibility on canonical unified-resource truth.
   Settings infrastructure source filters and summaries must preserve known
   sources such as `truenas` and `availability` through the shared source
   normalization boundary even when the current unified-resource snapshot has
   no matching rows, so cross-surface context does not silently fall back to
   generic host-only wording during hydration or empty-filter states.
7. Keep first-class platform classification on
   `docs/release-control/v6/internal/PLATFORM_SUPPORT_MODEL.md`. New platform
   work must declare its primary ingestion mode and canonical resource
   projections there before adding platform-local branches here, and runtime
   variants such as `podman` must stay inside the owning platform contract
   instead of becoming new top-level platforms or resource types.
8. Keep provider-backed signal metadata on shared canonical resource fields.
   VMware status, alarm, task, and snapshot signals must flow through shared
   `vmware` metadata plus shared `resource-incident` timeline entries on
   canonical `agent`, `vm`, `storage`, and `network` resources instead of creating
   provider-only resource kinds, identities, or history schemas.
9. Keep summary-surface emphasis on canonical resource IDs. Infrastructure
   summary row-hover, chart-hover, and route-focus behavior must keep using the
   same unified-resource IDs that power the table rows, chart series, and
   detail-route handoffs instead of introducing page-local summary IDs or
   provider-local hover aliases when the selected series is highlighted.
10. Keep infrastructure contextual focus route-backed and page-scoped. When an
    infrastructure row opens its detail drawer, the selection must stay on the
    same route through canonical resource query state, and the shared
    summary-table focus/runtime contract plus the root app-shell restore path
    must let direct row toggles open that drawer in place instead of looking
    like a page refresh or remount. The
    summary must keep `frontend-modern/src/components/Infrastructure/InfrastructureSummary.tsx`
    rendering the full page-level series set while only the focused label and
    highlight state change.
11. Keep infrastructure summary hover scope on canonical unified-resource ids
    even when one metric is empty. Shared chart hover may synchronize one
    timestamp across all four infrastructure cards, but the active emphasis
    must still resolve through the same unified-resource id that powers the
    table row, line charts, density maps, and drawer route state instead of
    dropping the highlight or inventing a metric-local summary identity when
    disk or network data is missing in range. When a sibling infrastructure
    card can resolve that same resource id locally, it should surface the
    synchronized value through the shared summary-card readout instead of
    spawning a second chart tooltip.
    Infrastructure summary numeric readouts that reflect canonical resource
    counts or capacity must use the shared `AnimatedNumber` primitive rather
    than page-local counter state, so readout motion stays presentation-only and
    canonical unified-resource identity and scope stay unchanged.
12. Keep infrastructure chart hover non-destructive to the unified-resource
    table. If the hovered resource row is already visible in
    `frontend-modern/src/components/Infrastructure/UnifiedResourceTable.tsx`,
    the row may highlight in place through the shared active-resource id; if it
    is off-screen, the page must offer an explicit `Jump to row` affordance
    rather than auto-scrolling or collapsing the table on hover.
12a. Keep infrastructure summary visibility as display preference, not a
    unified-resource filter. Platform/runtime pages and shared infrastructure
    summary consumers may hide or restore chart sections through shared
    presentation controls, but those controls must not mutate resource
    identity, table membership, source scope, or summary-hover state. The
    retired top-level `/infrastructure` page and its saved-view/route-state
    machinery must not be reintroduced for this purpose.
13. Keep infrastructure cluster headers as canonical summary scope. Grouped
    headers in `frontend-modern/src/components/Infrastructure/UnifiedResourceHostTableCard.tsx`
    must publish cluster scope from the same `ResourceGroup` / unified-resource
    ids that power the table rows, and
    `frontend-modern/src/components/Infrastructure/useInfrastructureSummaryState.ts`
    must consume that scope through the shared page/group/entity interaction
    contract rather than inventing infrastructure-local summary filters or
    route-backed cluster hover state. Host and service infrastructure table
    card frames must consume the frontend-primitives-owned `TableCard` wrapper;
    unified-resource ownership remains on resource identity, grouping, and row
    semantics rather than forking a table border/background shell. Deliberate
    cluster focus must also stay
    on the canonical infrastructure route through the shared `summaryGroup`
    query state, so pinned scope is shareable, reversible, and owned by the
    same route-backed summary contract as row focus. Infrastructure must stay
    row-first here: the pinned cluster header remains the visible scoped
    state, and explicit clearing belongs to the shared infrastructure table
    card header action plus the shared `Escape` reset path rather than a
    search-row fallback widget, page-level scope strip, or a second
    scope/pinned pill inside the cluster row chrome. Background whitespace
    clearing may remain a convenience, but infrastructure must not rely on it
    as the only reversible control.
14. Keep infrastructure row emphasis on the shared frontend presentation
    contract. Host, PBS, and PMG table sections may decide whether a resource
    is contextually active, but they must expose that state through
    `data-summary-row-active` and rely on the shared row presentation owned by
    `frontend-modern/src/index.css` instead of provider-specific background
    classes that drift across resource tables or hide inline metric bars.
    Cluster-member rows must also expose shared preview-versus-pinned group
    emphasis through `data-summary-group-member-active`, so the whole cluster
    block reads as the active scope without inventing a second infrastructure-
    local outline or banner treatment.
    Static grouped cluster-header emphasis must route through
    `frontend-modern/src/components/shared/groupedTableRowPresentation.ts` and
    the shared `.grouped-table-row` CSS contract in `frontend-modern/src/index.css`,
    rather than infrastructure-local background or hover-fill classes.
    Summary-linked infrastructure rows and cluster headers must also route
    pointer preview and focus preview through
    `frontend-modern/src/components/shared/summaryInteractionA11y.ts`, while
    deliberate expand/scope ownership must route through
    `frontend-modern/src/components/shared/SummaryRowActionButton.tsx`, so the
    unified-resource table does not fork mouse-only hover logic, focusable-row
    button shims, touch-hostile synthetic hover, or provider-specific control
    handling across host, PBS, and PMG sections.
15. Keep infrastructure search aligned with the governed display label. Shared
    infrastructure filtering through
    `frontend-modern/src/components/Infrastructure/infrastructureSelectors.ts`
    must match the user-visible safe label for governed resources instead of
    reintroducing redacted hostnames through search-only fallback candidates.
16. Preserve provider-backed storage backing-pool identity on canonical
    storage resources. `internal/unifiedresources/types.go`,
    `internal/unifiedresources/adapters.go`, `internal/unifiedresources/views.go`,
    `frontend-modern/src/types/resource.ts`, and
    `frontend-modern/src/hooks/useUnifiedResources.ts` must carry the
    provider-reported storage `pool` metadata alongside path and ZFS health so
    storage consumers do not have to recover backing-pool identity from names
    or path heuristics. ZFS health means the full pool report:
    `StorageMeta.ZFSPool` carries the provider's pool object (scan activity
    plus per-device states, error counters, and messages) to consumers
    alongside the flattened `zfsPoolState`/`zfs*Errors` scalars, so storage
    surfaces can show which device failed and whether a scrub/resilver is
    running instead of reducing pool health to a one-word state
    (`TestCanonicalStorageMetadataCarriesFullZFSPoolReport` pins the path).
17. Keep compatible unified-resource consumers on one shared snapshot truth.
    `frontend-modern/src/hooks/useUnifiedResources.ts` may seed narrower
    type-filtered consumers from a fresh `all-resources` snapshot when the
    broader cache already reflects canonical resource truth, and fresh empty
    snapshots must remain cacheable instead of regressing route handoffs back
    to transient full-page loading shells.
    That same `toResource` mapping owns canonical-field fallback for the
    per-resource scalars the frontend reads. `Resource.uptime` must fall
    back to the canonical `v2.uptime` field after the platform-specific
    carve-outs (`agent.uptimeSeconds`, `proxmox.uptime`, `pbs.uptimeSeconds`,
    `pmg.uptimeSeconds`, `kubernetes.uptimeSeconds`). The vSphere adapter
    populates only the canonical field on the REST contract, so without
    landing on `v2.uptime` ESXi hosts and VMware-backed VMs lose uptime on
    the unified-resources side even though the API payload carries it.
    That same shared cache boundary must normalize route/query type filters
    through the canonical frontend-to-`ResourceType` resolver before slicing
    the snapshot, so compatibility values such as `disk` / `physical_disk`
    stay on one cache truth instead of falling back to ad hoc filter aliases.
18. Keep storage summary target selection on canonical unified-resource truth.
    Storage-summary consumers may detect storage presence from canonical
    `isStorage(...)` resources and their shared metrics-target IDs, but once
    storage exists they must reuse the owned compact
    `/api/charts/storage-summary` contract instead of rebuilding page-local
    per-resource storage history fetches, storage-type aliases, or full
    storage-page `/api/storage-charts` fetches.
19. Keep infrastructure framing presentation-only on active product surfaces.
    Platform/runtime pages, shared infrastructure tables, and Settings
    infrastructure panels may render page or section headers, but canonical
    source/status/search state, summary scope, and row selection must remain on
    their owning shared selectors and settings state. The removed top-level
    `/infrastructure` page must not return as a second state owner, scope
    banner, or provider-local filter surface.
20. Keep audit-log persistence credential-safe.
    `internal/unifiedresources/audit_redaction.go` provides
    `RedactAuditText` and `RedactAuditRecord`, and every
    `ActionAuditRecord` persistence boundary in
    `internal/unifiedresources/store.go` (`RecordActionAudit`,
    `RecordActionExecutionStart`, `RecordActionExecutionResult` for both
    the SQLite and in-memory store implementations) must call
    `RedactAuditRecord` before writing. Operators sometimes paste
    secrets into a natural-language `reason`, and command output
    sometimes echoes tokens; both must be scrubbed to redaction
    markers (`[redacted]`, `[redacted-secret]`,
    `[redacted-credentials]`) before plaintext SQL persistence.
    `Plan`, `Approvals`, and identity fields are produced by Pulse
    rather than operators or external command output and must be left
    untouched by redaction (mutating them would break the
    `PlanHash`-based drift detection contract). The redactor set is
    intentionally narrower than the patrol-failure redactor: arbitrary
    URLs are preserved so operators can reference runbooks, ticket
    links, and GitHub issues in audit reasons.
    Verification command output is part of the same persistence and
    readback boundary: `ActionVerificationResult.Command`, `Output`,
    and `Note` must be replaced with stable redaction markers both on
    top-level `ActionAuditRecord.Verification` and nested
    `ExecutionResult.Verification`. Migrated legacy rows follow the same
    contract when rehydrated, so API and frontend readers never surface
    raw historical verification command, output, or note details. Stable
    status and error-token prefixes such as `plan_drift:` remain
    verbatim where agents depend on them. `Ran=false` normalization
    remains responsible for clearing verification details rather than
    persisting redaction markers for unrun checks.
21. Keep post-execution verification outcome on the canonical execution
    result. `ExecutionResult.Verification` carries
    `ActionVerificationResult` with `Ran`, `Command`, `Output`,
    `Success`, `RanAt`, and `Note`. The broker runs the
    class-derived read-after-write check after a successful dispatch
    and persists the outcome through the existing `result_json`
    column; no schema migration is required. Verification is
    best-effort: classes without a derivable check leave
    `Verification` nil rather than fabricating a verified=true entry,
    matching the no-fabrication rule used elsewhere in the trust
    arc. Frontend audit surfaces must surface verification when
    present (so operators see "Pulse confirmed the action took") and
    omit it cleanly when nil rather than inventing placeholder copy.
22. Keep governed-action drift refusal canonical in
    `internal/unifiedresources/actions.go`. `ErrActionPlanDrift` is the
    error any broker must return when the payload presented at execute
    time hashes to anything different than the approval-recorded
    `planHash`. Brokers must refuse rather than silently downgrade drift
    into a generic execution error or "plan expired" outcome — those
    are distinct refusal kinds (operator never approved vs operator
    approved a different action vs approval window passed) and audit
    review and operator UI surfaces should treat them differently. The
    drift contract is "the operator approved exactly this (command,
    target, reason) combination"; the freshly-recomputed
    approval-equivalent hash and the recorded `planHash` are how we
    enforce that. New broker call sites that introduce additional
    governed-action paths must either reuse the existing approval-
    equivalent hash or extend the canonical hash set in
    `internal/ai/tools/action_audit.go` rather than adding ad-hoc
    comparison logic.
23. Keep the `ActionVerificationResult` frontend mirror canonical and
    pin the operator-facing render location.
    `frontend-modern/src/types/actionAudit.ts` defines the TS
    `ActionVerificationResult` interface (`ran`, `command`, `output`,
    `success`, `ranAt`, `note`) that mirrors the Go type field-for-
    field, and `frontend-modern/src/components/Infrastructure/ResourceActionHistory.tsx`
    is the canonical operator-facing render location: each audit row
    surfaces verification as a distinct outcome row alongside the
    dispatch result, with emerald tone for verified and amber for
    failed (matching the trust palette already used on the findings
    panel). The render must show the stable redaction markers returned
    by the action-audit readback API for verification command, output,
    and note fields, not raw command or historical output details; the
    operator still sees whether Pulse ran the read-after-write probe and
    whether it confirmed the intended state. When `verification.ran` is
    false (no derivable check, or feature disabled for the action class)
    nothing is rendered —
    operators must not see fabricated "verified" claims for actions
    where Pulse cannot read back.
24. Keep resource action-history refusal and verification-outcome
    presentation shared. Refused action results may preserve stable
    backend prefixes for agent branching (`plan_drift:`,
    `action_plan_expired:`, `action_dry_run_only:`, and
    `resource_remediation_locked:`), but the drawer must render
    operator-safe labels and details through
    `frontend-modern/src/utils/actionAuditPresentation.ts` rather than
    prefix-first result copy or route-local parsing. The same helper owns
    `verificationOutcome.status` presentation for `verified`,
    `unverified`, `failed`, and `unknown` so resource history, future
    action audit consumers, and tests share one bounded vocabulary.
25. Keep canonical host IDs stable across restarts through durable
    identity pins. Canonical ID derivation hashes the strongest identity
    key available at mint time, and merged-source hosts (Proxmox node +
    pulse-agent) expose different identity subsets depending on which
    records are present when a registry rebuilds: agent records carry the
    machine ID, node records only cluster+hostname. The store-backed
    registry therefore persists identity pins
    (`canonical_id` ↔ machine ID / DMI UUID / cluster / hostname) in the
    `resource_identities` table (`ResourceStore.UpsertResourceIdentityPins`
    / `ListResourceIdentityPins`, written diff-aware after monitor-adapter
    rebuilds) and completes weak incoming identities from those pins
    before matching and ID derivation
    (`internal/unifiedresources/canonical_id_pins.go`). Persisted primary
    hostnames preserve their normalized dotted form. The pin index resolves
    that exact primary hostname first and exposes the historical short-name
    form only as an ambiguity-safe compatibility alias; two dotted hostnames
    that share a first label must never resolve through that shared alias.
    Change-journal
    reads (`GetRecentChanges*` / `CountRecentChanges*`) must expand a
    canonical ID to the era set recomputable from its pinned identity keys
    (`ResourceIdentityPin.EraIDs`) so journal rows recorded under an
    earlier era's ID stay part of the resource's timeline; reads keyed by
    a stale era ID resolve to the same merged timeline. New ingest paths
    must not mint host canonical IDs from snapshot-content-dependent
    identity subsets without consulting the pins, and pin writes stay on
    the durable store-backed registry (ephemeral per-request registries
    consult, never write). Pinned hostnames preserve the full dotted name
    (`NormalizeFullHostname`): distinct machines that share a short
    hostname (`cloud.rnd-lax1` vs `cloud.gce-or1`) must keep distinct
    pins, distinct pin-index buckets, and distinct presentation host rows.
    Short-hostname normalization (`NormalizeHostname`) is a matching-only
    convenience: pin lookups and the presentation host coalescer may pair
    a short name with its own FQDN (`web01` vs `web01.lan`), but only
    through an unambiguous bucket whose pinned hostname is
    short/FQDN-equivalent (`HostnamesEquivalent`) to the incoming one;
    they must never rewrite a persisted hostname down to its short form
    or cross-match two different FQDNs. Pin rows persisted before this
    rule hold the collapsed short name and heal in place on the host's
    next pin persist, and `ResourceIdentityPin.EraIDs` derives both full-
    and short-hostname eras so journal rows recorded under short-hostname
    IDs stay readable. Canonical ID derivation itself follows the same
    rule: the hostname-derived arms of `canonicalIDFromIdentity`
    (`cluster:<cluster>:<hostname>` and `hostname:<hostname>`) hash the
    full dotted hostname, never the short form, so machine-keyless FQDN
    Swarm members that share a short name (`cloud.a` vs `cloud.b` in one
    swarm) mint distinct canonical IDs instead of collapsing into one
    registry entry. Derivation stays a pure function of identity (pins
    complete identities but never substitute a stored ID) because
    store-less registries (AI tool executors, projections, mock) must
    derive the same IDs as the durable registry from the same identity.
    Hosts minted under the historical short-hostname derivation change
    canonical ID once: journal continuity rides the era expansion above,
    and operator-owned rows ride canonical-ID succession: when
    `PersistIdentityPins` collects a pin that supersedes an earlier era's
    pin for the same physical host (matching strong key, or a
    machine-keyless same-cluster pin whose hostname is the new pin's
    hostname or its collapsed short form), the store re-keys
    `resource_operator_state` and `action_audits` rows to the successor
    ID and drops the superseded pin row (`ApplyCanonicalIDSuccessions`
    in `internal/unifiedresources/canonical_id_succession.go`).
    Successions never fire while the old ID still belongs to a live
    resource, never follow a contradicting machine key (a reinstalled
    machine mints fresh, it does not absorb the old host's operator
    state), and never rewrite change-journal rows. Known limitation:
    machine-keyless hosts whose history already merged under a collapsed
    ID cannot be retroactively split; the merged era's journal rows
    appear in every member's timeline and the merged operator rows
    succeed to whichever member persists first. Regression coverage:
    `internal/unifiedresources/canonical_id_pins_test.go`,
    `internal/unifiedresources/canonical_id_succession_test.go`, and
    `TestStandaloneDottedHostnameAgentsStayDistinct` /
    `TestSwarmFQDNHostsWithoutMachineIDsStayDistinct` /
    `TestCanonicalIDSuccessionMovesOperatorState` in
    `internal/unifiedresources/registry_test.go`.
26. Keep action-audit origin metadata broker-owned. `ActionAuditRecord`
    carries an optional `Origin *ActionOrigin`
    (`surface`/`findingId`/`investigationId`/`proposalId`), persisted in
    the `action_audits.origin_json` column (added by
    `migrateActionAuditsSchema`) and round-tripped through
    `scanActionAuditRecord`. Origin identifies which internal surface
    proposed the action (e.g. Patrol) so decisions and terminal outcomes
    can be reconciled back onto that surface's records. It is set only by
    in-process planning callers through the action lifecycle service's
    plan options; the public `POST /api/actions/plan` body must never be
    able to claim a first-party origin. `NormalizeActionOrigin` trims
    fields and collapses an all-empty origin to nil so absent metadata
    never persists as an empty object. Origin fields are Pulse-produced
    identifiers, not operator text, and stay outside the redaction set.
    Downstream reconciliation of origin-tagged records rides the shared
    lifecycle service's org-scoped `OnActionTransition` hook (wired via
    `ResourceHandlers.SetActionTransitionPublisher`): transitions publish
    only after the corresponding store write succeeds, so a subscriber
    keyed by org ID never observes a state this store could still lose
    or apply it to the wrong tenant.
    Regression coverage: `TestSQLiteStoreActionAuditOriginRoundTrip` in
    `internal/unifiedresources/store_test.go`.
27. Keep API-added TrueNAS systems keyed by the configured connection,
    never by snapshot-reported identity. `systemSourceID` in
    `internal/truenas/provider.go` scopes the system source ID (and every
    child pool/dataset/app/VM/share/disk scoped under it) to the
    connection ID the poller passes through
    `NewLiveProviderForConnection`; the hostname arm exists only for
    fixture snapshots, which carry no connection. The system's ingest
    identity deliberately carries no machine key: the TrueNAS DMI serial
    is shared by DR clones and can be vendor placeholder garbage, and the
    retired client fallback minted machine keys from the reported
    hostname, fully merging same-hostname systems (#1573, #1575). For the
    same reason `ingest` skips `completeIdentityFromPins` for
    `SourceTrueNAS`: a hostname-bucket pin (stale pre-fix TrueNAS pin or
    a pulse-agent's pin on a same-named host) must not lend the system a
    machine key and re-merge what connection scoping keeps apart.
    `trueNASSystemMetricResourceID` must stay `systemSourceID` minus its
    `system:` prefix so `Agent.AgentID`, native history keys, and
    `canonicalAgentMetricID` in `BuildMetricsTarget` resolve one series.
    Rows minted under the retired hostname-keyed derivation re-key once
    through record-declared succession: records name their old canonical
    IDs in `IngestRecord.SupersededCanonicalIDs` and
    `ResourceRegistry.IngestRecords` applies the same
    `ApplyCanonicalIDSuccessions` semantics as pin-driven successions
    (operator state and action audits re-key, the superseded pin row
    drops, never while the old ID belongs to a live resource, journal
    rows never rewritten). Regression coverage:
    `TestRegistryIngestRecordsKeepsSameHostnameSystemsDistinct` /
    `TestIngestRecordsSucceedLegacyHostnameScopedCanonicalIDs` /
    `TestIngestRecordsDoNotCompleteTrueNASIdentityFromPins` in
    `internal/truenas/contract_test.go`, and
    `TestIngestRecordsRecordDeclaredSuccessions` /
    `TestIngestRecordsSkipRecordDeclaredSuccessionForLiveOldID` in
    `internal/unifiedresources/canonical_id_succession_test.go`.

## Current State

Resource detail drawer discovery-tab Suspense fallbacks now compose the
frontend-primitives `DiscoveryLoadingFallback` template. `ResourceDetailDrawer`
and `ResourceDetailDrawerOverviewTab` own when the Discovery tab is available,
but they must not recreate the centered loading row, local border spinner, or
discovery loading copy outside that shared primitive.

The resource detail drawer's "Open in Workloads / Storage / Recovery"
related-links injection
(`frontend-modern/src/components/Infrastructure/resourceDetailDrawerOperationalModel.ts`'s
`buildRelatedLinks`) was retired on 2026-05-16 alongside the platform-first
migration: the function now returns only service-detail links (PMG
thresholds). The supporting
`buildResourceSurfaceLinksForResource` /
`buildWorkloadsHrefForResource` / `buildStorageHrefForResource` /
`buildRecoveryHrefForResource` helpers were deleted from
`frontend-modern/src/routing/resourceLinks.ts`. New drawer affordances must
compose against the embedded platform-page sub-tabs rather than rebuilding
cross-jump URLs.

Cross-resource drilldown affordances were retired on 2026-05-16 alongside the
platform-first primary nav. The default `buildResourceHref` in
`frontend-modern/src/components/Infrastructure/ResourceCorrelationSummary.tsx`
and `frontend-modern/src/components/Infrastructure/ResourceChangeSummary.tsx`
now returns `null` instead of building a `/infrastructure?resource=...` link,
so correlation, change, dependency, dependent, and related-resource labels
render as plain text inside resource drawers
(`frontend-modern/src/components/Infrastructure/ResourceDetailDrawerOverviewTab.tsx`)
unless the caller supplies a platform-aware override. The host-row workloads
icon
(`frontend-modern/src/components/Infrastructure/UnifiedResourceHostTableCard.tsx`)
and the PBS "Open Recovery Events" link
(`frontend-modern/src/components/Infrastructure/serviceDetailLinks.ts`) were
removed because they were drawer-local cross-jump chips into broad aggregate
workspaces rather than platform-aware resource actions; the workloads-href helper module
(`frontend-modern/src/components/Infrastructure/workloadsLink.ts`) was deleted
in the same pass. The K8s Namespaces and Deployments drawer "Open Pods" /
"View Pods" buttons
(`frontend-modern/src/components/Kubernetes/K8sNamespacesDrawer.tsx`,
`frontend-modern/src/components/Kubernetes/K8sDeploymentsDrawer.tsx`) now
route to `/kubernetes/workloads` rather than building
retired aggregate workload URLs; the legacy context/namespace query filter
does not carry forward because the new Kubernetes Workloads tab does not
consume those parameters. Workloads, storage, and recovery route-state helpers
in `frontend-modern/src/routing/resourceLinks.ts` now serialize query state
only; the owning platform/runtime route supplies the pathname. New drawer or
correlation surfaces must anchor on platform routes (or stay-in-place drawer
expansion) instead of resurrecting the retired top-level paths.
Unified-resource drawers and Kubernetes drill-down controls may own resource
timeline filter semantics, namespace choices, and destination routes, but their
native select chrome must compose the frontend-primitives-owned `FormSelect`.
`ResourceDetailDrawerOverviewTab.tsx` and `K8sDeploymentsDrawer.tsx` must not
recreate raw labelled `<select>` controls for those filters locally.

The Proxmox platform page is a route-level consumer of canonical unified
resources, not a new resource source. It filters the existing resource snapshot
to Proxmox VE, Backup Server, Mail Gateway, storage, disk, and workload rows,
then composes the existing Workloads, Storage, Recovery, and infrastructure
table owners. Proxmox host row version, uptime, temperature, CPU, memory, disk,
network I/O, and disk I/O presentation must derive from canonical resource
facets and the shared `nodeFromResource` adapter; platform pages must not
rebuild resource identity, merge policy, or metric-target inference locally.
The registry and presentation coalescer also own metric-source freshness. When
two source facets contribute the same metric, source priority decides only if
both sources have equivalent freshness. A stale source must not hold CPU,
memory, disk, network, or disk I/O metrics against a live source, and a stale
incoming source must not clobber a live metric already attached to the
canonical resource. This freshness gate belongs in
`internal/unifiedresources/registry.go` and
`internal/unifiedresources/presentation_coalesce.go`, not in platform-page
rendering or frontend fallback code.

`resource_operator_state.go` owns the operator-set per-resource intent
schema. `ResourceOperatorState` carries five narrow operator-intent
fields (`IntentionallyOffline`, `NeverAutoRemediate`, maintenance
window via `MaintenanceStartAt` / `MaintenanceEndAt` / `MaintenanceReason`,
canonical `Criticality` hint of `high|medium|low|""`, and an explicit
`AutoRemediationPolicy`) plus
operator-attribution metadata (`Note`, `SetAt`, `SetBy`). The shape is
intentionally fixed-field rather than a freeform metadata bag so
downstream finding-suppression and severity-weighting logic has a stable
contract to honor. `ValidateResourceOperatorState` rejects malformed
records with a stable `ErrResourceOperatorStateInvalid` sentinel
(empty canonical id, partial maintenance window, end ≤ start, unknown
criticality, enabled automatic-action policy without a capability allowlist,
unknown timezone, or invalid recurring-window minute). The automatic-action
policy is opt-in and names exact capability IDs; its optional daily IANA-timezone
window may cross midnight. `NeverAutoRemediate` always wins. Capability-owned
`ResourceCapability.AutoAuthorization` independently declares whether a
capability is never eligible, low-risk eligible, or elevated eligible; it does
not lower `MinimumApprovalLevel`. The first eligible vertical is Docker/Podman
container `restart` at `low_risk`; unspecified capabilities normalize to never.
`NormalizeResourceOperatorState` trims whitespace, de-duplicates capability
names, and lower-cases the criticality value before persistence. The
`ResourceStore` interface now exposes
`GetResourceOperatorState`, `SetResourceOperatorState`, and
`ClearResourceOperatorState`; both the SQLite (table
`resource_operator_state` keyed on `canonical_id`) and Memory stores
implement the same upsert + idempotent-clear contract. The same
store also owns `RecordActionPolicyExecutionStart`, the automatic-admission
CAS that moves a planned or pending action directly to `executing` while
persisting the server-owned policy approval and typed
`ActionPolicyAuthorizationLease` in the same transaction. Memory and SQLite
implement identical semantics. The lease binds org/action/resource/capability,
plan and capability-policy hashes, safety and approval floors, tenant
mode/license/unlock version, resource allowlist/window/Never version, expiry,
and digest. A policy approval without that lease is historical-only and cannot
admit a nonterminal action after restart.
The same
store powers the agent-consumable bundled context endpoint at
`/api/agent/resource-context/{id}` (handler in
`internal/api/agent_resource_context.go`) — that endpoint reads
operator state and recent action audits through the canonical
`ResourceStore` accessors plus active findings via an
`AgentFindingsProvider` adapter wired from the patrol service, and
projects each shape into agent-stable `AgentResource*` types so the
wire contract stays decoupled from the storage type's evolution.

The operator-facing surface for this contract is
`ResourceOperatorStateSection.tsx` on the resource detail drawer
overview tab, which routes through the canonical TS client at
`frontend-modern/src/api/resourceOperatorState.ts` to
`/api/resources/{id}/operator-state`. Alongside the existing operator-state
controls, the drawer offers automatic actions only for backend-advertised
eligible capabilities. Enabling it requires selecting exact capabilities and
may add a recurring daily time/timezone window; copy must state that this is a
resource allowlist and that the tenant Patrol mode remains the upper bound.
The UI must never infer eligibility from capability names, severity, or the
human approval floor.

The `/api/resources/{id}/operator-state` API surface (GET / PUT /
DELETE) in `internal/api/resources_operator_state.go` is the
operator-facing consumer of this contract; the URL canonical_id always wins over the
body, server-side `setAt` / `setBy` populate from request time and
authenticated identity, and validation rejections surface a stable
`operator_state_invalid` error code. The action broker
(`executeCommandWithAudit`) consults the same store on every dispatch
and refuses with `ErrResourceRemediationLocked` when the operator has
set `NeverAutoRemediate=true`; the refusal is persisted as a Failed
audit record with `resource_remediation_locked:` prefix on the
`ErrorMessage` so audit-UI filters and alert rules can branch on the
stable token. Finding-suppression integrations consume the same
`ResourceOperatorState` shape via the
`ResourceOperatorStateProvider` interface in `internal/ai`.

`actions.go` now owns the canonical action preflight and audit-normalization
contract. Action plans must carry dry-run availability, safety checks, and
verification steps through `preflight`, and `RecordActionAudit` plus
`RecordActionLifecycleEvent` must normalize action id, request id, resource,
capability, approval policy, timestamps, lifecycle state, and requester before
records reach durable audit history.
Action approval decisions belong to the same resource-owned state machine:
`ApplyActionDecision` may transition only a persisted `pending_approval`
record to `approved` or `rejected`, while `RecordActionDecision` must update
the current audit record and append the lifecycle event atomically without
creating execution results or accepting stale second decisions.
Action execution now follows that same resource-owned state machine:
`BeginActionExecution` may transition only an approved action, or an
approval-free allowed plan that is not `ApprovalDryRun`, into `executing`;
dry-run-only plans may remain audited planning evidence, but must fail closed
before any `executing` lifecycle mutation. `CompleteActionExecution` may
transition only an executing record into `completed` or `failed` with an
explicit `ExecutionResult`. `RecordActionExecutionStart` and
`RecordActionExecutionResult` must perform the audit update and lifecycle
append atomically against the current persisted state, reject stale duplicate
starts or terminal writes, and preserve approval as separate from execution.
`ResourceActionHistory.tsx` now surfaces that same normalized action audit
trail inside the resource-detail workflow through the canonical
`frontend-modern/src/api/actionAudit.ts` client and shared
`frontend-modern/src/utils/actionAuditPresentation.ts` labels. The resource
drawer must treat gated action-audit reads as unavailable rather than turning
ordinary infrastructure inspection into an upgrade-prompt path.
Docker / Podman container lifecycle controls in
`frontend-modern/src/features/docker/DockerContainerLifecycleControls.tsx` and
`dockerContainerLifecycleActions.ts` are unified-resource capability consumers:
they may enable start/stop/restart only from backend-advertised resource
capabilities and must use `sourceStatus`, `docker.agentId`, `docker.runtime`,
`docker.security`, and backend-owned `actionReadiness` only for disabled-state
explanation, never as a feature-local execution bypass. The same controls may render in
`DockerContainersTable.tsx` and the canonical `ResourceDetailDrawer.tsx` header
only for Docker-source app containers backed by Docker or Podman runtime
metadata; other app-container sources such as TrueNAS must not inherit Docker
lifecycle controls from runtime-shaped metadata alone. After a governed request
completes, the owning surface may ask its existing `useUnifiedResources` query
to refetch the resource snapshot, but refresh is not an alternate execution,
verification, or provider-control path.
Proxmox VM and LXC lifecycle affordances follow the same resource-owned rule:
the backend advertises only status-appropriate `start`, `shutdown`, `reboot`,
and `stop` capabilities for non-template, unlocked Proxmox guests, filters them
when the owning node command agent is disconnected, and records disabled-state
context in `actionReadiness`. Proxmox platform pages and detail drawers may
consume those fields, but they must not treat row status, VMID, node name, or
guest metadata as separate permission to run `qm` / `pct` or provider calls.
Typed VM and system-container read views expose detached capability slices and
per-source delivery status for internal consumers. A Patrol detector must bind
to the exact resource-owned lifecycle capability and consume the canonical
Proxmox source status/timestamp; it must not reconstruct action availability or
freshness from guest power state or a locally hard-coded polling window.

`InfrastructureSummary.tsx` and `infrastructureSummaryModel.ts` now surface
`degraded` and `alerting` resource counts alongside the existing `online` and
`offline` totals. `buildInfrastructureResourceCounts` is the canonical owner of
this health-state projection: it must derive counts only from the resources
accessor already available to the summary state, not from a separate API call or
a re-projection of websocket state outside the shared summary pipeline.

`resourceBadgePresentation.ts` now owns the Infrastructure table system
identity resolver. That resolver must prefer provider/API platform identity,
then reported host OS or appliance identity, before falling back to Docker or
other runtime capability labels.

This subsystem now sits under the dedicated core monitoring runtime lane so
canonical resource identity, discovery normalization, and platform-runtime
coverage stay governed as a first-class Pulse product surface, including the
shared VMware signal-metadata and `resource-incident` timeline vocabulary that
canonical resources expose to alerts, AI, and frontend consumers.
Agentless availability checks are now canonical resources rather than
connection-only status rows. `SourceAvailability` emits `network-endpoint`
records with the saved target id, probe address, protocol, cadence, last check,
failure count, and threshold in `AvailabilityData`. Registry merge policy must
preserve that payload and incident state. An availability record attaches as a
facet onto a known resource when (a) the target carries an explicit
`LinkedResourceID` that resolves to a resource already in the registry by exact
resource id, unique source id, or unique canonical identity alias, or (b) the
probe address unambiguously matches exactly one known resource by IP through
`FindCandidates` with reason `ip` or `hostname+ip` (confidence at or above the
merge threshold). Fuzzy hostname-only correlation (reason `hostname`, confidence
below threshold), ambiguous source/canonical references, and references to
availability-owned resources must not attach. When attached, the known resource
inherits the `AvailabilityData` facet and `SourceAvailability` in its source
list, and no standalone `network-endpoint` is minted. When no link resolves, the
record falls back to a standalone `network-endpoint` as before. A second probe
must not overwrite a facet already attached by a different target; the second
probe stays standalone in that case.
Frontend resource adapters must preserve that same availability identity on
both REST and realtime paths: a thin `network-endpoint` update with
availability data is still `platformType=availability`, `sourceType=api`, and
must not regress to a generic platform badge in infrastructure rows or drawers.
Infrastructure row presentation must also consume that availability payload as
operator evidence, not only as badge identity. Any resource row carrying an
`AvailabilityData` facet—whether a standalone `network-endpoint` or a known
guest that inherited the facet through explicit link or IP correlation—must
surface one visible probe readout from either the top-level availability field
or the live-state `platformData.availability` mirror: the System column uses
the probe protocol as the compact identity badge (`ICMP`, `TCP`, or `HTTP`),
while the metric cell shows only the target detail and latest latency or
failure result, such as `6053: 11 ms`, `/status: 503`, `3 ms`, or `timed out`.
Frontend primitives owns Machines as the operational presentation for those
same agentless checks; unified resources owns the projection contract consumed
there. `StandalonePageSurface.tsx` must fetch both `agent` and
`network-endpoint` resources, keep standalone machines and availability checks
as separate buckets in `standalonePageModel.ts`, and let
`AvailabilityChecksTable.tsx` render saved probe method, target, latest result,
check age, failure count, and cadence from the canonical availability payload.
Recent check timing and fuller failure context may stay in tooltip or drawer
detail, but the table row must not duplicate the same probe protocol and
result text across both identity and metric cells.
That same frontend-owned compatibility boundary must remain intentionally
narrow. Shared resource adapters may admit explicit aliases such as `host`,
`truenas`, and `ceph`, and VMware detail mappers may project typed metadata
through the canonical resource model, but unified-resource consumers must not
reintroduce removed workload aliases or feature-local resource-type shims just
to satisfy one table, drawer, or badge surface.

Patrol attention resource navigation carries the canonical subject resource ID
through shared route builders as an opaque query value. Attention deep links
carry the canonical operational-record ID separately. Neither link may derive
a replacement resource identity from display text, provider labels, or alert
metadata.

### Protection posture identity consumer

`ProxmoxCoverageTable` remains a unified-resource identity consumer while
storage/recovery owns protection truth. Live VM/LXC rows carry the exact
canonical `Resource.id` into one bounded posture batch; the table must not parse
its presentation key, VMID, name, node, or instance to mint a replacement
resource identity. Orphaned backup artifacts have no canonical live resource
ID and therefore render unknown posture while retaining their forensic backup
detail. Unified resources own row identity only; they do not derive backup
freshness, provider completeness, verification, or protection state.

### APT Product Trust browser projection

The durable Actions inbox and resource action history consume the canonical APT
workflows without adding a frontend action-truth dialect. The shared
`aptActionPresentation.ts` translator may recognize only the complete bounded
host-update and package-cache summaries emitted by the accepted backend
contract. It validates the closed phase set, safe integer counts, cleanup byte
arithmetic, and exact empty parameter envelope before presenting facts. Any
partial, malformed, unknown-phase, or arithmetically impossible summary yields
no fact card and directs the operator to refresh and review the canonical
record. Execution, verification plus adjacent evidence source, and compensation
remain three independent `ActionResultV2` cards.

The Actions inbox presents that durable record as a compact operator queue,
not a stack of equally weighted audit cards. Open work orders approval-required
decisions before runnable and executing actions; each collapsed row exposes
only state, action, bounded resource identity, recency, and reason before the
operator opens the governed review. Action list and detail reads enrich each
audit with the current canonical resource name and contract type when that
resource can be resolved. This metadata is a read-time presentation projection,
not part of `ActionRequest`, durable plan identity, or `planHash`; a resource
rename therefore cannot change action authority. Opaque canonical resource IDs
remain in the row's accessible name and title, while the visual row prefers the
API-supplied name and type and falls back to the bounded ID-derived type plus
short suffix only when the resource projection is unavailable. Read-only demo
posture is quiet supporting context rather than a page-level callout. The
empty Open queue must explain producer state rather than promising content:
when the effective Patrol mode is Watch only (read via the canonical
`/api/ai/patrol/autonomy` settings read, failing closed to the generic calm
copy when that read is unavailable), the calm state names that Watch only
never queues proposed fixes and routes unlocked installs to the Patrol mode
switch, while plan-locked installs get the presentation-policy-aware Pro
capability line from `actionPresentation.getActionsWatchOnlyEmptyState` with
the upgrade action suppressed whenever upgrade prompts are hidden. That
guidance is presentation-only: Actions must not gain a second Patrol mode
mutation surface. The
governed review dialog continues to own the full exact resource ID, plan,
policy evidence, lifecycle, authority, and outcome truth. Its first layer keeps
intent plus safety/authority facts visible while the immutable planning-time
policy sources, revisions, and reason codes sit behind an explicit disclosure;
missing provenance remains an immediate fail-closed warning rather than hidden
detail. Actions is the canonical browser hub for action review, execution
progress, and recorded outcomes; contextual sources such as Patrol link into an
exact action review instead of duplicating those mutations locally.

APT review presents server-recorded policy provenance and distinguishes the
elevated update posture from low-risk-eligible cache cleanup. Both typed actions
declare that no command, path, package selection, removal choice, or reboot
authority is accepted from the browser. Reboot-required is a reported fact only;
there is no reboot control. Durable attempt and receipt detail appears once and
explains that refresh or reconnect rehydrates the existing action rather than
creating a duplicate. Package cleanup is explicitly irreversible; an
inconclusive or failed cleanup requires a fresh scan and never presents fake
rollback or automatic retry.

Task 09 owns shared APT telemetry freshness in
`internal/unifiedresources/host_apt_telemetry.go`. Capability construction,
finding admission, and execution readiness consume the same dual-timestamp and
SHA-256 validation rules. A retransmitted old agent observation stays stale
when freshly received, future/skewed clocks fail closed, and capability removal
cannot be bypassed by duplicated telemetry predicates.

### Patrol Autopilot authority evidence

`internal/unifiedresources/patrol_autopilot.go` owns the versioned immutable
acknowledgement, revocation, activation, accepted-scope/limits, digest, status,
and evaluation contracts. A static server-owned registry binds every supported
acknowledgement version to its exact immutable scope, limits, and optional
lifetime; the current version provider is separate from that historical
registry. One pure stored-evidence validator checks each record against its own
registered version, canonical IDs, human actor and organization binding,
coherent times, digests, activation binding, and unique same-acknowledgement
revocation before any config mutation. Runtime evaluation uses the current
registry entry and fails closed; a malformed foreign revocation cannot become
a valid revocation attributed to the victim organization.

This authority evidence does not replace the governed action lifecycle,
approval strength, dispatch continuity, or `ActionResultV2`. It only controls
whether requested Patrol full mode is effective.

### Canonical action-result truth

`internal/unifiedresources/action_result_v2.go` is the sole schema authority
for terminal action truth. `ActionResultV2` versions execution, verification,
evidence trust, and nested compensation as independent facts. Independent
verification requires durable bounded evidence from a trust domain distinct
from the executor; evidence identifies observer kind as well as observer and
trust domain, and an agent readback remains agent-attested. Nil executor
results, legacy completed rows without results, timeouts with unknown effect,
and malformed evidence remain explicitly inconclusive. Legacy `Success` and
`VerificationOutcome` fields are derived compatibility projections only.
Canonical evidence is bounded before redaction and digested as SHA-256 over
the canonical redacted envelope. Redaction deep-copies nested evidence and
fails closed to `redaction_contract_violation`; it never preserves invalid or
unredacted input. Result normalization is copy-on-normalize across evidence,
verification, and nested compensation, so callers may safely normalize shared
immutable snapshots without mutating their stored input or racing other
readers. A present but malformed stored V2 also fails closed to
`result_v2_invalid`; legacy booleans can never override it. Compensation truth
never rewrites the primary result and carries declared trigger, durable
attempt/step identity, timing, nested execution and verification, and restored
state digest identity for downstream recovery without implementing recovery.
Workflow, API, AI, agent, Docker, host-agent, and relay packages must not
declare competing truth enums. Generated wire mirrors remain a bounded later
presentation concern, not a second source of semantics.

The first production distinct-trust-domain consumer is the Proxmox VM/LXC
lifecycle executor. Its node-agent dispatch and Proxmox API observation remain
separate evidence domains, and the API-owned projection binds the exact action,
resource, before/after snapshots, observation times, and canonical digest.
This does not authorize provider-local mutation or relax the false-independence
guard for Docker or agent-attested APT readback.

The canonical action resource contract now owns immutable `ActionActor` and
versioned `ApprovalRequirement` bindings. Actor subject/kind/credential/org and
requirement floor/quorum/separation are part of deterministic action identity
and plan hashing. Human approvals are append-only under a durable monotonic
`decisionRevision`: SQLite and MemoryStore compare pending state, revision, and
the complete prior approval prefix before accepting the next decision. Each
accepted revision persists a typed decision event with the bound approval and
evidence; approved/rejected state changes persist a separate unique transition
atomically, while pending quorum decisions do not fabricate a state change.
Legacy nonterminal approvals without these bindings are readable but fail
closed as replan-required.

`internal/unifiedresources/action_policy_provenance.go` is the sole schema
authority for plan-time action policy provenance. Every new canonical plan
captures the capability-registry authority and may add only the ordered,
bounded tenant Patrol and resource-operator authorities actually consulted by
a trusted broker. Version, stable source identity/revision, organization,
resource, capability, resulting approval requirement, and closed reason codes
are digested and included in `planHash`; malformed, duplicate, cross-scope,
contradictory, unsupported, or unbounded facts fail before audit persistence.
`planningAllowed` means only that a typed plan could be created. It is not an
authorization result and cannot replace the fresh Task 04 dispatch lease or
current-policy recheck. Legacy rows without the field remain readable only as
`legacy_unknown`; normalization never invents historical policy authorities.
Memory and SQLite preserve the same object through replay and reopen.

Action audits are the durable source of truth for Patrol action continuity.
The store exposes optional `ActionAuditOriginReader` and
`PendingActionAuditReader` capabilities; origin lookup is scoped by org and
investigation identity, while pending reads are oldest-first. SQLite persists
an absent origin as NULL, guards JSON-expression queries and indexes with
`json_valid(origin_json)`, and keeps dedicated origin/state indexes so an old
empty or malformed value cannot reject otherwise valid audit rows. Memory and
SQLite implementations preserve the same ordering and clone semantics.
Terminal audit persistence derives `VerificationOutcome` from the canonical
execution result: no verifier is unknown, a configured verifier that did not
run is unverified, a successful read-back is verified, and a failed read-back
is failed. Consumers must use that durable result rather than inferring
success from executor completion alone.

Unified-resource row actions must remain operable at phone widths without
changing capability ownership. Docker and Podman lifecycle controls retain
their backend-authored availability and approval semantics while using the
shared 40-pixel mobile touch floor; compact desktop sizing may resume at the
small breakpoint. Responsive styling must not synthesize capabilities, widen
the action set, or create provider-local lifecycle state.
That same runtime now also owns prospective monitored-system projection. Add
and update consumers must ask the unified-resource layer whether candidate or
preview records change the deduped top-level monitored-system count, including
replacement-aware projection and VMware host-UUID identity, instead of
rebuilding handler-local counting heuristics.
That same current-state contract now also treats candidate activity as
canonical unified-resource ownership. Inactive candidates must not materialize
projected monitored-system resources at all, new-candidate previews must fall
back to zero-delta/no-change when only an inactive candidate is proposed, and
replacement previews may remove only the replaced source while preserving any
remaining grouped monitored-system evidence.
Candidate matching must fail closed when the prospective monitored-system
candidate cannot resolve to a canonical top-level resource; malformed or
whitespace-only inputs are not allowed to masquerade as an already-counted
system simply because their no-op projection has zero additional count.
VMware replacement selectors must also keep source-specific identity signals
unambiguous: `ResourceID` scopes saved vCenter connection replacement to
`ConnectionID`, while host UUID / DMI identity matching belongs to the
machine-identity selector path, so editing one connection cannot strip an
unrelated VMware host whose UUID happens to equal that connection ID.
That shared consumer ownership now includes same-tab summary hydration too.
`frontend-modern/src/components/Infrastructure/useInfrastructureSummaryState.ts`
may keep an in-memory remount cache for canonical resource charts, but the key
must carry an explicit summary contract version so a long-lived browser tab
cannot resurrect older unified-resource summary shapes after the chart model or
identity mapping changes.
That same consumer ownership now also includes infrastructure drawer history
fetches. `frontend-modern/src/components/Infrastructure/useResourceDetailDrawerHistoryState.ts`
may hydrate canonical facet and intelligence data for the selected resource,
but it must do so through the shared retained-value query helper so opening a
resource drawer never drops the whole infrastructure surface into the app-level
`Loading view...` fallback.
VMware vSphere now follows the same admission rule. In the admitted phase-1
direction, vCenter is the connection authority but not a
top-level unified resource: ESXi hosts must project as canonical `agent`
resources, virtual machines as canonical `vm`, and datastores as canonical
`storage`. Datacenters, clusters, folders, and resource pools stay topology or
relationship metadata under those shared resources rather than becoming new
top-level types. Phase-1 vSphere work must not invent `esxi-host`,
`vsphere-vm`, `vsphere-cluster`, `system-container`, `app-container`, or
`physical-disk` projections just because the upstream APIs expose additional
object classes.
That same VMware contract now also includes the shared source boundary. When
runtime work starts, VMware-backed records must flow through one canonical
VMware source key plus `platformType: vmware-vsphere`, not through separate
`vcenter` and `esxi` source forks or provider-local raw type aliases. One
host, VM, datastore, or network from VMware should therefore still look like
one shared Pulse `agent`, `vm`, `storage`, or `network` resource to downstream
selectors, drawers, alerts, AI, and route filters.
That shared source boundary now also has a concrete frontend/runtime adapter
floor. `internal/unifiedresources/types.go`, `internal/unifiedresources/registry.go`,
`internal/unifiedresources/views.go`, `frontend-modern/src/hooks/useUnifiedResources.ts`,
`frontend-modern/src/types/resource.ts`, and
`frontend-modern/src/utils/sourcePlatforms.ts` must keep raw VMware ingest on
`SourceVMware` while projecting the operator-facing platform as
`vmware-vsphere`. Shared filters, route state, and badges may normalize that
single source onto the canonical platform vocabulary, but they must not invent
separate `vcenter` or `esxi` filter keys or a VMware-only top-level resource
family to make the phase-1 slice render.
That same frontend/runtime adapter floor now also owns the typed platform
projection. `frontend-modern/src/utils/platformSupportManifest.generated.ts`
must stay generated from
`docs/release-control/v6/internal/PLATFORM_SUPPORT_MANIFEST.json`, and
`frontend-modern/src/types/resource.ts` must derive `PlatformType` from that
generated supported-plus-admitted projection rather than hand-maintaining a
second platform union that can drift from the governed manifest or re-admit
presentation-only labels by mistake. The supported/admitted split in that
projection is also contractual: VMware may stay a valid `PlatformType` and
source-platform alias while admitted, but unified-resource consumers must not
infer current platform support from that union without checking governance
state/readiness. Agent host profiles are generated beside
that platform projection for shared identity/presentation use only; a profile
such as Unraid may label a Pulse Agent host, but it must not enter
`PlatformType`, `PLATFORM_TYPE_KEYS`, unified-resource source filters, or
canonical top-level platform identity.
Host-profile runtime fallback values are generated beside those labels so
agent-backed appliance reports can normalize to canonical runtime platforms
such as `linux` without making the appliance profile a unified-resource
platform. Unified-resource `AgentData` must carry that distinction explicitly:
`agent.platform` is the normalized runtime platform, while
`agent.hostProfile` carries a governed profile id such as `unraid` for
presentation and host/appliance support-floor copy. Raw appliance identity
aliases such as `unraid-os` may be accepted only through the generated
host-profile token projection and must resolve to a governed profile id before
they reach platform filters, source IDs, or top-level resource identity.
Unified-resource `AgentData` also carries the host memory composition:
`AgentMemoryMeta` exposes the reclaimable page-cache split (`cache`) beside
used/free/swap so machine surfaces can render used | cache | free without a
parallel payload, mirroring the `proxmox.memoryCache` transport nodes and
guests use. The field is additive and omitted when an agent does not report
it; consumers must treat missing cache as zero rather than inferring it from
free space.
Frontend resource identity presenters may append a runtime version to a
displayed system badge only when that version is sourced from the same canonical
platform or host-profile identity, such as PVE `ResourceProxmoxMeta.pveVersion`
or `platformData.proxmox.pveVersion`, PBS `version`, or an agent OS report that
resolves to Unraid or Proxmox VE. They must not attach a collector OS version to
a different API-backed platform identity.
Container runtime badges follow that same shared identity boundary:
`frontend-modern/src/utils/resourceBadgePresentation.ts` owns the runtime badge
label and tone mapping for Docker, Podman, and unknown runtimes, and consumers
must render those badges from the shared helper instead of rebuilding local
runtime chips or page-specific colour classes.
Agent-backed storage resources follow the same distinction: `StorageMeta.platform`
may carry appliance presentation context such as `unraid` so the operator can see
what system owns the array, but realtime `platformType` and source filters must
remain agent-backed unless the storage resource is actually owned by a governed
API platform such as Proxmox, PBS, TrueNAS, or VMware. Storage resources must not
fall back to Proxmox solely because their canonical resource type is `storage`.
That same shared source boundary also applies when unified seeds and
supplemental providers coexist. If a canonical unified-resource seed omits an
owned supplemental source such as TrueNAS or VMware, the shared resource API
must still ingest that provider-owned source instead of letting the seed
silence the platform entirely. When a supplemental provider identity-matches an
existing resource, adding the source tag alone is not enough; the matching
provider facet must be merged onto the shared resource so source identity,
health, routing, and detail payloads cannot disagree. Operator-facing source
filters may accept the `vmware-vsphere` alias for the VMware platform, but the
emitted shared source family remains canonical `vmware`.
That same canonical identity boundary now also applies to infrastructure
summary emphasis. `frontend-modern/src/components/Infrastructure/InfrastructureSummary.tsx`
may vary card presentation, but the active sparkline or density-map series must
still resolve through the shared resource ID already emitted by unified
resources so table hover, focused detail state, and summary highlight all point
to the same resource identity.
That same unified-resource boundary now also applies to infrastructure
contextual focus plumbing. `frontend-modern/src/components/Infrastructure/useInfrastructureSummaryState.ts`
must reuse `frontend-modern/src/components/shared/contextualFocus.ts` for
interactive-series filtering, focused-resource naming, and active-series
resolution so the infrastructure summary stays page-scoped while still
highlighting the same canonical resource IDs used by the unified-resource
table and route state.
TrueNAS application resources now carry the same shared source boundary.
`internal/truenas/provider.go` projects native app API records into
`TrueNASData.App`, while `internal/unifiedresources/types.go`,
`internal/unifiedresources/clone.go`, `internal/unifiedresources/registry.go`,
`frontend-modern/src/types/resource.ts`, and
`frontend-modern/src/hooks/useUnifiedResources.ts` must preserve that facet
across backend clone/merge and frontend transport. The owning resource remains
the canonical `app-container`; the TrueNAS facet explains the platform-native
app shape, and Docker metadata remains secondary runtime compatibility rather
than the source of truth for the TrueNAS overview.
That same infrastructure focus boundary now also owns deliberate drawer reveal.
When an infrastructure row opens inline detail on the same route, the page must
tag that detail with the canonical active resource ID. Direct row toggles that
already have the row in view must capture the current `.app-scroll-shell`
position through `frontend-modern/src/utils/appShellScrollRestoration.ts`, so
the remounted root shell in `frontend-modern/src/App.tsx` can stay anchored,
and then still hand off to the shared summary-table/contextual-focus helpers
when the opened drawer would otherwise fall below the fold. That reveal must
scroll only enough of the infrastructure table to keep the row header plus the
start of the detail visible, not leave the drawer clipped and not hard-center
the selected row.
`useUnifiedResourceTableViewportSync.ts` must stay viewport-only; it may not
grow a second selected-row reveal path or a resource-local centering rule.
That same unified-resource boundary now also owns stored metrics-target
continuity for provider-backed resources. When registry rebuild cannot derive a
fresh metrics target from raw source facets, `internal/unifiedresources/registry.go`
must preserve the canonical `MetricsTarget` already attached to provider-backed
agents or app-containers, and typed views must expose that stored target
unchanged so shared chart routes keep using canonical IDs across live and demo
projections.
Seeded unified-resource snapshots are part of that same registry ownership.
`internal/unifiedresources/registry.go` must rebuild the canonical `bySource`
index from source-owned facets already present on seeded unified resources
before manual links, overlays, or metrics-target resolution run, so
`/api/resources`, infrastructure summary joins, storage summary joins, and
other chart consumers keep resolving history through canonical source
coordinates instead of silently falling back to hashed unified resource IDs.
That same registry/view boundary now also applies to provider-backed storage.
`internal/unifiedresources/registry.go` must attach the resolved
`MetricsTarget` onto cached view clones before `ReadState` exposes
`StoragePoolView` or `PhysicalDiskView`, so `/api/resources`, storage summary
selection, `/api/storage-charts`, and `/api/charts/storage-summary` all see
the same canonical history
identity instead of splitting between view-cache resource IDs and API
serialization-time metric IDs.
Proxmox Ceph pools are canonical storage resources when present in
`CephCluster.Pools`. `internal/models.CephPoolStorageID` owns the stable
source and metrics identity, and `internal/unifiedresources/registry.go` must
project each pool through normal storage ingest so API resources expose a
storage metrics target whose resource id is the storage runtime metric id
rather than the hashed unified-resource id.
That same VMware contract now also includes the identity rule. VMware managed
object identifiers are phase-1 provider identities, but they must be scoped by
the owning `vCenter` connection or discovered vCenter identity so bare object
IDs do not masquerade as workspace-global keys. Secondary identities such as
VM `instance_uuid` / `bios_uuid` and host UUID when available belong under the
shared canonical identity model for future merge or assistant reasoning, not
inside a VMware-only dedupe lane.
That same VMware contract now also includes the topology rule. `vCenter`,
datacenter, cluster, folder, resource pool, and datastore cluster objects may
enrich canonical `agent`, `vm`, `storage`, and `network` resources as
placement metadata or relationships, but they must not appear as synthetic
top-level VMware resource types just to mirror the upstream inventory tree.
Snapshot trees and VMware alarm/event/task context are also governed by that
same rule: they may enrich canonical `vm`, `agent`, `storage`, or `network`
resources and their timelines, but they do not become shared recovery
artifacts, new provider-local resource kinds, or a parallel VMware incident
model.
That same topology contract now also has a concrete projection seam.
`internal/vmware/provider.go` must preserve VMware placement and identity
detail on the shared `vmware` facet only: hosts may carry datacenter,
compute-resource, cluster, cluster HA/DRS service state, folder, and
attached-datastore metadata; VMs may carry runtime-host, cluster HA/DRS
service state, folder, resource-pool, datastore, guest-identity, VM
virtual-hardware configuration, VMware Tools runtime status, and VM hardware
Ethernet adapter plus VM hardware disk metadata plus canonical parentage to
the owning ESXi `agent`; datastores may
carry datacenter/folder placement plus shared storage-node and workload
consumer metadata through `storage.nodes`, `storage.consumerCount`, and
`storage.topConsumers`; networks may carry network type, datacenter/folder
placement, host attachments, VM attachments, and VMware health/task/event
signal summaries under the shared `vmware` facet on canonical `network`
resources. VMs may also carry VI JSON snapshot-tree context under
`vmware.currentSnapshotId` and `vmware.snapshotTree`, including snapshot
managed-object reference, display name, description, creation time, power
state, quiesce flag, current marker, replay support, and child snapshots.
VM hardware Ethernet adapter context belongs under
`vmware.networkAdapters`, including vCenter adapter id, label, emulation
type, MAC address/type, PCI slot, backing type, backing network id/name,
distributed switch/port or opaque-network references, connection state,
start-connected, guest-control, Wake-on-LAN, and UPT compatibility flags.
VM hardware disk context belongs under `vmware.virtualDisks`, including the
vCenter disk id, label, host-bus adapter type, IDE/SCSI/SATA/NVMe placement,
backing type, VMDK file path, bracketed datastore name when the VMDK path
provides it, and capacity in bytes.
VMware Tools context belongs under `vmware.tools`, including Tools run state,
version status, version number/string, install type, upgrade policy,
auto-update support, install-attempt count, last install error, guest reboot
request flag, requesting components, and request time. Those fields are
operator-facing monitoring facts from vCenter's VM Tools API; they must not be
promoted into lifecycle control, recovery status, workload identity, or a
separate guest-runtime resource.
VM virtual-hardware configuration belongs under `vmware.hardware`, including
vCenter VM guest OS, instant-clone frozen flag, virtual hardware version,
hardware upgrade policy/version/status/error, boot type, EFI legacy boot,
network boot protocol, boot delay/retry/setup-mode flags, boot-device order,
CPU cores-per-socket and CPU hot-add/remove flags, and memory hot-add
increment/limit settings. Those fields are API-native read-only VM
configuration facts; they must not become Pulse VM-control authority, workload
identity aliases, recovery protection posture, or a separate hardware resource.
Cluster HA and DRS state belongs under `vmware.clusterHaEnabled` and
`vmware.clusterDrsEnabled` for hosts and VMs whose placement resolves to that
cluster. It is API-native monitoring context from the vCenter cluster summary,
not a synthetic cluster resource, lifecycle command surface, scheduling policy
model, or recovery/protection signal.
Those enrichments must remain subordinate to shared `agent`, `vm`, `storage`,
and `network` resources rather than becoming a VMware-only topology graph,
recovery artifact, canonical identity alias, or separate provider detail drawer
contract.
TrueNAS disk telemetry now follows the same rule. API-backed TrueNAS disks must
populate canonical `physicalDisk.temperature` and reuse the shared
physical-disk risk semantics, so infrastructure, storage, charts, and AI read
the same disk-health contract instead of inventing a provider-local temperature
surface.
That same canonical physical-disk risk contract must also absorb disk-health
incidents during cross-source merges. A provider alert such as TrueNAS
`truenas_smart` is not presentation-only context; it must become a canonical
`physicalDisk.risk.reasons` entry so hybrid agent/API disk resources keep one
shared disk-health truth after deduplication.
That same canonical disk contract now also owns recent aggregate temperature
history. When a provider such as TrueNAS can supply `disk.temperature_agg`
min/avg/max readings, it must project those onto
`physicalDisk.temperatureAggregate` instead of introducing a provider-local
history blob or a parallel disk-temperature presentation model.
That same canonical source and seed contract now also applies to runtime mock
mode. When mock-backed TrueNAS or VMware supplemental records are present, they
must enter the shared resource graph through the same canonical source/type
rules as live providers instead of introducing a mock-only source family or
resource kind.
That same mock seed contract now also includes legacy snapshot-backed sources.
`internal/mock/fixture_graph.go` must own the runtime mock snapshot and the
provider-backed TrueNAS/VMware fixtures together, and
`internal/mock/platform_fixtures.go` must project unified resources from that
one graph instead of combining a legacy snapshot read with standalone provider
defaults. The shared resource graph must therefore see one coherent mock
platform set regardless of whether a platform is snapshot-backed or
supplemental-provider-backed.
The graph also owns governed action fixtures in
`internal/mock/action_fixtures.go`. Those records must reference canonical
resource IDs from the graph snapshot, carry current policy provenance and typed
result truth, and reach the Actions workspace through the backend mock read
projection rather than a page-local fallback or durable-store contamination.
Agentless availability fixtures join that same graph-owned seed contract:
mock UPS, MQTT, ESPHome, HTTP, and controller endpoints must project as
`SourceAvailability` `network-endpoint` resources with real availability
payloads, incidents, and source status rather than as generic host rows or
settings-only sample data. Mock service availability fixtures must use
`LinkedResourceID` source references when the target service row is produced by
the same graph, so service ping evidence lands as an `AvailabilityData` facet on
the Docker or Kubernetes service instead of creating a disconnected duplicate
endpoint.
Callers should therefore consume `CurrentFixtureGraph()` and graph-owned
projections rather than reintroducing platform-only or state-only mock helper
exports.
TrueNAS-managed applications now follow the same canonical workload rule. One
TrueNAS app instance from `app.query` must project as one canonical
`app-container` resource under `SourceTrueNAS`, reusing the shared workload and
Docker payload contracts instead of inventing a `truenas-app` resource type or
lane-local workload UI. API-backed `app.stats` telemetry must now project onto
that same canonical `app-container` contract as live metrics and metrics
targets, using the shared `docker:<id>` in-memory key and `dockerContainer`
history store path instead of adding a TrueNAS-local workload history lane.
TrueNAS-managed virtual machines follow the same canonical workload rule. One
TrueNAS VM from `vm.query` must project as one canonical `vm` resource under
`SourceTrueNAS`, carrying native `TrueNASData.VM` metadata instead of inventing
a `truenas-vm` resource type, borrowing Proxmox guest fields, or rendering a
provider-local VM contract. Native VM lifecycle/control is read-only at this
floor until a governed action path owns it.
TrueNAS top-level systems now follow the same canonical host rule. One
TrueNAS appliance must project as one canonical `agent` resource under
`SourceTrueNAS`, carrying host-facing `AgentData` plus provider-specific
`TrueNASData` instead of inventing a `truenas-system` resource type or a
provider-local host surface. API-backed `reporting.realtime` telemetry must
project onto that same canonical `agent` contract as live metrics and metrics
targets, using the shared `agent:<id>` in-memory key and `agent` history store
path instead of adding a TrueNAS-only host history lane.
That same rule applies at the frontend boundary. Shared compatibility adapters
such as `frontend-modern/src/hooks/useUnifiedResources.ts` and
`frontend-modern/src/utils/resourceTypeCompat.ts` must collapse any legacy raw
`truenas` top-level type payloads back onto canonical `agent` before route
helpers, dashboard summaries, alert context, or selector models consume them.
Downstream shared consumers such as metrics-history target helpers and drawer
discovery mappers must reuse that same canonicalization step instead of adding
their own `case 'truenas'` branches later in the render path.
API-backed TrueNAS CPU temperature now follows that same canonical host rule.
TrueNAS system records must project `cputemp` readings into the shared
`agent.temperature` and `agent.sensors.temperatureCelsius` fields instead of
inventing a provider-local temperature payload or leaving TrueNAS host
temperatures unavailable without the unified agent.
Pressure-only host telemetry follows the same canonical host-sensor route:
agent resources may include `agent.sensors.thermalState` even when
`agent.temperature` and `agent.sensors.temperatureCelsius` are absent, so
platform consumers can show macOS pressure without creating provider-local
thermal payloads or fake Celsius values.
AI discovery and query surfaces now follow the same rule. Assistant runtime
paths such as `pulse_query` and unified AI context must expose TrueNAS-backed
canonical `agent`, `vm`, `app-container`, `storage`, and `physical-disk`
resources through those shared contracts, filtering dataset-topology storage
children out of any `storage-pool` presentation instead of inventing
TrueNAS-local assistant types or mislabeling datasets as pools.
That same canonical app-container rule now also governs diagnostics. Assistant
runtime paths such as `pulse_read` must resolve API-backed TrueNAS apps
through the shared canonical `app-container` identity and `resource_id`
contract instead of reintroducing host-local container routing assumptions for
platforms that do not use the unified agent as their primary runtime path.
That same canonical app-container rule now also governs configuration reads.
Assistant runtime paths such as `pulse_query action="config"` must resolve
API-backed TrueNAS apps through the same canonical `app-container` identity
and `resource_id` contract, then project native runtime/config shape through
the shared app-container payload rather than forcing those resources through
guest-config routing or inventing a TrueNAS-local config type.
That projection contract is the product-definition floor for TrueNAS support:
Pulse supports TrueNAS only when it lands on the shared `agent`, `vm`,
`app-container`, `storage`, `physical-disk`, and recovery-linked resource
shapes, with native system-owned service inventory carried as
`TrueNASData.Services` on the canonical `agent`. The unified agent may augment
a TrueNAS system later, but baseline support does not depend on it, and product
surfaces must not reopen a parallel `truenas-*` resource model just to add
another capability.
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
underlying host's monitored-system identity. Production grouping and monitored
system candidate previews must both flow through
`monitoredSystemCandidateAllowsHostAttachment(candidate)` so Kubernetes cluster
exclusions and non-unique IP filtering cannot drift between those paths.
Unspecified addresses such as `0.0.0.0` and `::` are not identity evidence for
host attachment. The canonical resolver coverage is pinned by an explicit
top-level source matrix and mixed-environment characterization tests so new
top-level sources cannot quietly bypass the counting contract.
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
and the drawer header action chrome is a frontend-primitives dependency:
ResourceDetailDrawer may own which actions are available and how they call the
resource state, but Assistant, copy-context, close, and future drawer-header
actions must compose the shared Button drawer-header action primitives instead
of restoring resource-drawer-local button classes,
and platform alert tables use the same dependency rule for severity
presentation: Docker, Kubernetes, TrueNAS, and vSphere incident rows may own
which incidents exist and which native detail rows are shown, but visible
severity dots, badges, and severity label formatting must compose
`AlertSeverityDot`, `AlertSeverityBadge`, and
`formatAlertSeverityLabel` from the frontend-primitives-owned shared path
instead of restoring table-local `severityVariant`, `severityTextClass`, or
severity-label helpers, detail severity row tones must use
`getAlertSeverityDetailTone` instead of restoring local `alertTone` helpers,
and their severity toolbar filters must use
`getPlatformAlertSeverityFilterOptions` instead of restoring local
All/Critical/Warning/Info option arrays,
and platform alert detail fields must use
`formatPlatformAlertCode`, `formatPlatformAlertResourceType`,
`formatPlatformAlertEntityType`, `formatPlatformAlertStartedAt`, and
`formatPlatformAlertDetailDateTime` from
`frontend-modern/src/utils/alertDetailPresentation.ts` instead of restoring
table-local provider code, resource-type, entity-type, or timestamp formatter
helpers,
and local operator identity labels now split from governed detail summaries:
infrastructure tables, selectors, links, and drawer headings must preserve the
canonical local instance identity (`displayName`, canonical display name,
hostname, then primary ID fallback) so service and host surfaces stay uniquely
addressable for operators, while the drawer governance summary and other
explanation surfaces still show the full canonical `aiSafeSummary`,
`resourceDetailDrawerServiceModel.ts` owns the pure Docker/PBS/PMG service
summary and breakdown derivations that feed the overview service-details
surface,
and PBS running-task visibility is part of that same canonical service model:
raw backup, sync, verify, prune, and garbage job arrays travel through the
unified-resource metadata contract in `frontend-modern/src/types/resource.ts`
and `frontend-modern/src/components/Infrastructure/resourceDetailMappers.ts`,
`resourceDetailDrawerServiceModel.ts` owns active-task status classification and
shared activity wording, and both
`frontend-modern/src/components/Infrastructure/UnifiedResourcePBSTableSection.tsx`
and `frontend-modern/src/components/Infrastructure/ResourceDetailDrawerOverviewTab.tsx`
must render from that shared projection instead of rescanning raw job arrays or
inventing local PBS status heuristics,
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
The retired dashboard overview trend hook must not return as a second
infrastructure sparkline consumer. Infrastructure summary surfaces must consume
the infrastructure summary chart cache and shared unified-resource series
matching logic instead of issuing bespoke per-resource
`/api/metrics-store/history` fetches for overview cards. That keeps summary
sparklines aligned with canonical resource identity matching, agent-facet
fallback behavior, and first-sample empty-state semantics already owned by the
infrastructure summary surface.
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
to always-on row chrome. Inline row badges are reserved for blocking policy
states such as `local-only` routing or `restricted` sensitivity, while
`sensitive` + `local-first` and redaction-only posture remains visible in Data
Handling, detail, and AI/governance surfaces. The table must not imply that
ordinary sensitive resources carry an operator-actionable infrastructure
exception.
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
Assistant finding handoffs are part of that same relationship-context contract:
when the runtime needs topology context for product-originated handoff
resources, it should resolve the canonical unified resource, synthesize the
canonical parent edge through `ResourceRelationshipsWithCanonicalParent(...)`,
and call `FormatResourceRelationshipContext(...)` rather than rebuilding
relationship markdown in chat or Patrol-local helpers. The resulting topology
block remains model-only explanation context, not action authority.
Assistant finding handoffs are also part of the canonical resource-state
contract: when the AI runtime needs current state for a product-originated
handoff resource, it should resolve the canonical unified resource and summarize
the owned status, freshness, source-health, metrics, incidents, and governed
capabilities from `unifiedresources.Resource` instead of rebuilding a
Patrol-local or chat-local resource model. That state block remains model-only
read-only infrastructure context, and capability names or approval policies in
that snapshot do not grant execution authority.
Assistant finding handoffs are also part of the canonical resource-timeline
contract: when recent-change context is hydrated for a handoff resource, chat
execution should resolve the resource through the current unified-resource
provider before calling `GetRecentChangesFiltered(...)`, with raw handoff IDs
used only as a compatibility fallback. That keeps Patrol-originated discussion
anchored to canonical resource identity instead of lane-local IDs or display
names.
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
the first-class relationship-map surface for canonical `resource.relationships`,
the correlation ordering, and the truncation rule, so callers pass raw
relationships and correlation lists instead of encoding their own sort or
top-N behavior.
Canonical parent edges now also originate in this subsystem: `ParentID` is
folded into the facet relationship set through
`ResourceRelationshipsWithCanonicalParent` before any drawer or Patrol
consumer renders a relationship map, so pages do not rederive parent topology
from raw resource fields or invent relationship-map fallbacks locally.
The same surfaces now also render recent changes through the shared
`frontend-modern/src/components/Infrastructure/ResourceChangeSummary.tsx`
card, so canonical timeline wording and ordering stay governed by one
frontend feed instead of separate page-local loops. Callers may suppress
resource-change metadata badges only for compact operator-context surfaces such
as Patrol's supporting context; the shared card still owns headline/reason
dedupe so prefixed backend reasons do not render as duplicated visible copy.
Assistant finding handoffs are part of that same timeline contract: when the AI
runtime needs recent changes for product-originated handoff resources, it should
read the canonical unified-resource timeline and `FormatResourceRecentChangesContext`
rather than rebuilding recent-change wording or querying Patrol-local change
detectors first. Those handoff timeline entries remain read-only explanation
context, not action authority.
The unified-resource detail drawer now also routes resource-tag presentation
through the shared `frontend-modern/src/components/shared/TagBadges.tsx`
primitive instead of importing a workload-local badge helper into the
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
The mobile shell class from that shared state model now keeps a 640-pixel
minimum table width below the small-screen breakpoint while restoring
`min-w-full` above it. This retains the prioritized mobile column set without
forcing those columns into unreadable tracks, and the existing shared `Table`
overflow owner contains the horizontal scroll locally.
That same unified-resource consumer contract now also owns CSP-safe table
presentation for infrastructure rows. Host, PBS, and PMG table sections must
consume the shared column presentation owner and render canonical table sizing
through classes plus DOM width/height attributes rather than lane-local inline
style objects, so the same unified-resource dataset can reach the public demo
without transport-specific DOM drift.
That same consumer contract now also owns full-width desktop balance for the
infrastructure tables. The shared column presentation owner must publish an
explicit desktop `Resource` width for host, PBS, and PMG sections so wide
shells redistribute surplus width across the remaining columns instead of
turning the first column into blank filler that hides metric density.
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
the pre-v6 `timestamp` column by adding canonical columns without rewriting
large historical tables during startup. Timeline reads must resolve legacy
`timestamp` and nullable `observed_at` values through read-time fallback
expressions, while writes preserve the legacy timestamp on target databases
that still require it.
`internal/api/resources.go` now exposes that same history through dedicated
`/api/resources/{id}/timeline` reads, while the bundled `/api/resources/{id}/facets`
surface keeps the facet summary and recent-change history available without
forcing consumers to parse the full resource payload.
The facet bundle's relationship slice must be produced with
`ResourceRelationshipsWithCanonicalParent`, preserving explicit resource
relationships and adding a canonical parent edge from `ParentID` only when an
equivalent typed edge is not already present.
Those resource-owned timeline and facet reads are relationship-aware at the
API boundary: when the drawer requests a resource timeline, the store must
return direct changes for that canonical ID plus changes whose
`relatedResources` contains the same ID. The default store
`GetRecentChanges` path remains direct-resource-only for incident and AI
callers unless they explicitly opt into `ResourceChangeFilters.IncludeRelated`.
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
workload-to-infrastructure href builder used by Workloads row and drawer
consumers. Future workload-to-resource navigation changes must extend through
that shared routing contract instead of reintroducing workload-local path
builders.
That same shared routing boundary now also owns the canonical Patrol
destination used by cross-surface findings and drill-down links. Shared
dashboard, alert, and settings referrals may target `/patrol` through
`frontend-modern/src/routing/resourceLinks.ts`, but retired `/ai` browser route
shapes must stay unregistered rather than compatibility redirects or forked
primary destinations in local link builders.
That same shared routing boundary now also owns storage route-state vocabulary
for unified resources without restoring the retired `/storage` top-level route.
Platform pages and other embedded owners may carry owned `source`, `node`, and
`resource` queries, but cross-surface consumers must land inside an owning
platform/runtime route instead of rebuilding drawer-local storage paths,
provider-local highlight rules, or standalone aggregate workspace URLs. When a
top-level system is a merged hybrid surface, the route-state helper must
resolve the deep-link source from the canonical merged source set before
falling back to raw `platformType`, so TrueNAS-backed hybrid systems do not
lose their storage context just because agent telemetry is also present.
That same routing contract now also owns workload platform scoping without
restoring the retired `/workloads` top-level route. Shared workloads links,
dashboard URL-sync state, and infrastructure drill-down helpers must preserve
`platform=<owned-source-key>` for API-backed workloads such as TrueNAS
app-containers instead of collapsing those routes back to generic agent or
Docker-only semantics when host telemetry is also present. Runtime-local
filters like `agent` or cluster context may still be added as secondary scope,
but the canonical platform query must remain the first-class route-state
boundary for shared workload navigation.
That same shared workload-route contract also owns exact guest selection for
node-scoped workloads. Proxmox VM and system-container links must emit the
canonical workload identity (`<instance>:<node>:<vmid>`) in the shared
`resource` query rather than an opaque unified-resource id, so direct route
loads and cross-surface drill-downs reopen the correct workload drawer instead
of landing on an unselected table state.
That same routing contract now also owns recovery route-state vocabulary for
unified resources without restoring the retired `/recovery` top-level route.
Infrastructure drawers and other cross-surface consumers must carry canonical
`platform` and `node` query state into an owning platform/runtime route instead
of rebuilding drawer-local recovery links, assuming only PBS services can
expose recovery handoffs from infrastructure, or sending operators to a
standalone aggregate workspace URL.
When shared recovery links include protected-inventory posture, they must use
the recovery-owned `state` query instead of overloading event `status`. Event
outcome `status` remains recovery-history state, and compatibility input such
as legacy `stale=1` must be normalized by the recovery route owner before
cross-surface links are rebuilt.
That same shared routing boundary now also owns alert-investigation handoffs.
Resource-incident panels and other alert-side resource drill-down consumers
must route operators back through canonical infrastructure resource detail and
then into platform-owned workload, storage, and recovery surfaces via
`frontend-modern/src/routing/resourceLinks.ts` route-state vocabulary, instead
of treating alert investigation as a provider-local dead end, freezing
per-surface route strings outside the unified-resource contract, or reviving
retired aggregate workspace URLs.
That same routing contract also owns Patrol finding handoffs. Expanded
Patrol-finding rows and scoped-run finding snapshots must resolve the backing
unified resource and surface the same platform-owned workload/storage/recovery
route-state vocabulary there, including exact workload and physical-disk route
state when the selected resource is itself the workload or disk, instead of
stopping at finding text, rebuilding patrol-local route strings, or reviving
retired aggregate workspace URLs.
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
The shared summary chart contract now also requires the backend feed to
normalize tiered infrastructure history into equal-time summary buckets, so
long-range unified-resource cards do not expose storage-tier density changes as
right-edge chart compression.
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
Discovery validity in that runtime state must defer to the shared
`discoveryPresentation.ts` meaningful-context gate. CLI access, confidence, and
diagnostics may support an audit trail, but they must not turn placeholder
records such as unknown service/category/version with no ports, paths, facts,
or suggested URL into a valid identified result. Generic workload types such
as `service`, `container`, or `lxc` remain placeholders until Discovery also
provides a more specific service signal.
Discovery endpoint candidates are support context, not authoritative resource
links. Resource and workload drawers may pass observed Discovery URLs into the
shared web-interface field with visible provenance, copy/open affordances, and
an explicit adopt/save path, but row-name links and persisted metadata must
continue to use the operator-saved web-interface URL. Version, config-path,
port, and endpoint facts surfaced outside the Discovery sub-tab must be
labelled as Discovery-observed so API-owned resource facts and command-derived
facts remain distinguishable.
That label must be visible through the shared Discovery provenance marker on
compact cards, suggested URL panels, and other out-of-tab Discovery values
rather than buried in helper text or inferred from the tab where the operator
found the value.
Command-availability guidance inside that analysis surface must consume the
shared `frontend-modern/src/utils/discoveryPresentation.ts` settings targets
and scan-error copy instead of hard-coding legacy settings labels such as
Unified Agents, API Tokens, or `/settings/integrations/api`; the canonical API
access route is `/settings/security/api`.
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
Timeline filter default options such as all kinds, all sources, and all
adapters must use the shared frontend all-option presentation helper through
`ResourceDetailDrawerOverviewTab.tsx`, not drawer-local hard-coded strings, so
future wording changes stay aligned with workload, storage, recovery, and alert
filters.
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
Docker and Kubernetes platform subtabs now follow canonical resource evidence
inside workflow-level navigation instead of exposing one visible tab per API
kind. Docker / Podman keeps top-level `Overview`, `Containers`, `Images`,
`Storage`, `Networks`, and `Swarm` tabs: `Storage` is backed by host-level
Docker/Podman `/system/df` usage buckets plus canonical `docker-volume` rows,
and `Swarm` is backed by canonical `docker-service`, `docker-task`,
`docker-swarm-node`, `docker-secret`, and `docker-config` rows from manager
inventory. Kubernetes keeps top-level `Overview`, `Nodes`, `Workloads`,
`Services`, `Storage`, `Configuration`, and `Events` tabs: `Workloads` groups
Pods, Deployments, controller resources, and HorizontalPodAutoscalers;
`Services` groups Service rows with ingress and endpoint-slice rows without
duplicating Services in the networking table; and `Configuration` groups
metadata-only config rows with policy, quota, limit, and disruption-budget
rows. Legacy object-specific routes may resolve to the owning workflow tab, but
top-level tab visibility is governed by the workflow model. Inactive standalone
Docker Swarm metadata is not a tab, host role, or service-surface signal.
The frontend Docker facet contract covers host runtime telemetry, runtime
container projection, and Swarm service/task/node projection. Host resources
may expose runtime/version, OS, temperature, uptime,
container/image/volume/network/node counts, update state, command metadata,
engine storage usage buckets, and Swarm local-state/control evidence through
`ResourceDockerMeta`; app-container resources may expose container id, host
name, runtime/version, image/digest, container state, health, restart/exit
counts, published ports, attached networks, mounts, and image-update status.
Docker platform pages must consume those canonical fields rather than keeping
page-local host-runtime or container-runtime shape aliases.
Canonical resources now also carry `platformScopes`, the normalized
platform-page membership list consumed by workload filters. `platformType`
remains the primary display/source family, while `platformScopes` captures
overlap: Docker/Podman resources reported from a Proxmox LXC carry both
`proxmox-pve` and `docker`, but TrueNAS app containers that reuse
`DockerData` for runtime metadata remain scoped to `truenas` and must not be
promoted to Docker-managed action targets.
That overlap is not default peer-row membership on every platform overview.
The Docker / Podman runtime lens is the canonical detailed table for Docker
containers, while the Proxmox Overview workload table stays focused on VMs and
LXCs. Proxmox may surface Docker-inside-LXC evidence as LXC drawer detail, but
it must not promote those `app-container` rows into the default Proxmox peer
workload table or add always-visible child rows that compete with VM/LXC scan
flow. The peer row may expose only a compact nested-runtime cue derived from the
same context so operators can discover that the LXC has nested containers
without reading inline container details. That nested context must be read-only
and keyed by the same canonical
Proxmox guest identity and Docker host source identity used by the resource
model; ambiguous runtime-host matches must be omitted rather than guessed. When
that drawer links into the Docker / Podman runtime lens, it must carry the
unambiguous Docker host facet in the route (`host=<runtime hostname>`) and the
Docker overview must apply that scope to the host and container tables; if the
runtime host label is missing or ambiguous, the handoff falls back to the
unfiltered Docker overview.
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
leakage or over-redaction in chat context. That same unified-resource contract
now also owns the canonical mention identity path for shared Assistant reads:
chat surfaces must carry the backend-issued unified resource ID for canonical
`agent`, `vm`, `storage`, and `app-container` records, and backend mention
resolution must be able to recover those resources by canonical ID and
canonical name through the shared read-state instead of relying on
provider-local route families or frontend reconstruction of VMware- or
TrueNAS-specific coordinates.
Assistant finding handoffs now resolve product-originated resource references
through the same canonical unified-resource helper before building policy
context, so handoff handling guidance uses `CanonicalGovernanceMetadata(...)`,
`ResourcePolicyLabel(...)`, and `ResourcePolicySummaryLines(...)` instead of
chat-local sensitivity inference. The resulting policy block is model-only
read guidance and must not become saved user text, disclosure authority, or
action authority.
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
That same shared workloads-link path now also owns top-level TrueNAS system
handoffs to the canonical app-container workloads route, so infrastructure
drawers and related-link surfaces do not strand canonical `truenas` resources
on infrastructure-only navigation while API-backed apps already exist in the
shared workloads model. That same helper must also honor merged
`agent`+`truenas` source sets for top-level systems, so hybrid TrueNAS
surfaces keep the same workload handoff even when the resource shape presents
through the canonical host path instead of a raw `type='truenas'` row.
The resource drawer's Kubernetes namespace and deployment tabs use the
canonical cluster-name helper for backend fetch keys, keeping lookup identity
separate from the governed display-label contract.
The shared workloads projection in `useWorkloads` also uses that helper for
pod context labels, so dashboard Kubernetes grouping follows the same
canonical cluster-name contract instead of re-encoding the fallback locally.
That same shared workloads projection now also owns canonical app-container
identity on workload surfaces. Frontend workload records must keep the
unified resource `id` intact for drawer selection, deep links, anomalies, and
discovery, while Docker-native action fields such as `containerId` stay
separate and are only consumed by Docker-specific controls. API-backed
platforms such as TrueNAS must not regress that path back to runtime-native
container ids just because they expose Docker-compatible runtime metadata.
That same workload-state boundary also owns summary-chart identity for
provider-backed workloads. Unified-resource VM and system-container views may
surface `MetricsTarget` only as the history lookup target, while workload
chart transport and hover/focus selection must keep using the canonical
workload row ID so provider-backed workloads such as VMware VMs stay aligned
with workload rows and summary cards.
Kubernetes pods follow that same split. Pod rows may surface
`MetricsTarget.ResourceID` only as the history lookup target, but that target
must stay on the canonical prefixed runtime key
`k8s:<cluster>:pod:<uid>` even when the underlying source-owned pod ID arrives
as bare `<cluster>:pod:<uid>`. `/api/resources`, workload charts, and
`/api/metrics-store/history` must therefore collapse onto that prefixed pod
target instead of letting pod rows and pod charts drift onto separate metric
series.
Kubernetes clusters, nodes, and deployments are governed by that same shared
identity family. The registry must derive those metric targets through one
canonical helper boundary in `internal/unifiedresources/kubernetes_metric_ids.go`
and publish them unchanged through `/api/resources`, chart clients, and
mock-history paths, so cluster, node, pod, and deployment charts all bind to
one runtime key family instead of each layer rebuilding Kubernetes IDs from
cluster name, namespace, or surface-local aliases.
The drawer's discovery mapper also reuses that helper for pod fallback agent
IDs, so the resource-detail path and the Workloads path stay aligned on the
same cluster-name source of truth.
The workload projection and workloads-link route helpers also share
the same Kubernetes context prefix helper in the shared agent-resource
layer, so pod grouping and cluster navigation keep the same cluster-context
prefix before any surface-specific display fallback is applied.
The unified-resource projection also reuses that same prefix helper for
projected Kubernetes `clusterId`, so the shared resource store stays aligned
with the Workloads and detail surfaces on the same canonical cluster-context
source of truth.
That same contract also owns the canonical resource display-name fallback, so
name-or-ID presentation stays consistent between the unified AI adapter, the
AI resource context, and the shared resource selectors instead of being
recomputed locally.

That same shared store now also persists append-only action lifecycle, action
audit, and export audit records, giving the control-plane verbs a durable home
next to the resource timeline instead of leaving those records isolated in
memory-only models.
The in-memory store mirrors the durable audit contract through atomic
create-or-return-current action identity and monotonic typed transitions; it
must never replace an existing action record merely because an ID collides.
SQLite uses insert-on-conflict-do-nothing for creation and conditional state
updates for transitions, with the lifecycle event committed in the same
transaction. Both stores treat rejected, explicitly expired, completed, and failed states as
absorbing and admit exactly one successful transition to executing. Lifecycle
events are unique per action and state, with migration deduplication for rows
written before this invariant became database-enforced.
Execution admission commits the audit transition and lifecycle event with one
deterministic `ActionDispatchAttempt` and its outbox row in the same
transaction. Transport states are deliberately limited to `queued`, `claimed`,
`receipt_pending`, and `receipt_recorded`; they do not encode Task 10 execution,
verification, evidence, or compensation truth. An expired claim before
`MarkActionDispatchStarted` requeues the same attempt. That one-shot CAS is the
sole pre-send linearization point, increments `dispatchCount`, and removes the
outbox row. Once reached, restart recovery may accept only a correlated receipt
or query a transport reconciler by attempt ID; it cannot claim or send again.
When a correlated response includes the existing `ExecutionResult`, SQLite and
MemoryStore commit the receipt, `receipt_recorded` attempt, terminal audit, and
lifecycle event atomically. Standalone callbacks without a result remain
transport-only receipts and do not invent terminal truth.
Current typed agent attempts also persist the transport-derived operation kind,
operation version, request digest, and agent identity with that admission.
Pre-B2 rows migrate with empty binding columns and remain readable but inert;
they are never rebound from current resource telemetry or resent. Agent
operation reconciliation requires exact equality with this persisted binding.
Queued, claimed, forged, conflicting, duplicate, late, and out-of-order receipt
handling is monotonic and tenant-scoped, and the in-memory store mirrors the
SQLite crash-boundary behavior.
It also mirrors the durable decision contract: approval/rejection writes must
target an existing pending action and must fail rather than creating a
decision-only record or overwriting an already decided action.
The enterprise audit API now reads those same unified-resource action and
export records back out, so the durable store is not just a write sink but the
canonical history surface for the control-plane verbs.
Assistant handoff action context must also read current action-plan and
action-audit state from this same store when an approval reference resolves to
a governed action, so chat follow-ups describe the canonical lifecycle record
rather than a stale approval snapshot. That read remains model-only review
context and must not expose raw command text or raw execution output.
Patrol queued-fix approvals are producers of the same unified action-audit
records: the approval adapter may seed planned and pending lifecycle events for
the governed action, and those initial request/lifecycle records must identify
`pulse_patrol` as the producer. The durable action identity, state transitions,
and history remain owned by this store rather than by a Patrol-specific audit
ledger or by Assistant-local requester inference.
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
guest URL, connection health, PVE/kernel version identity, temperature details,
and pending-update metadata so monitoring can derive `models.Node` from unified
views without depending on legacy snapshot ownership.

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
status reason, QEMU guest-agent runtime status/expectation, and container
OCI/Docker-detection metadata so monitoring can derive `models.VM` and
`models.Container` from unified views without depending on legacy snapshot
ownership.

Canonical PBS metadata now carries full instance boundary payload such as host
and guest URLs, full datastore details, and PBS job arrays so monitoring can
derive `models.PBSInstance` from unified views without depending on legacy
snapshot ownership.

Canonical physical-disk views now expose the full disk identity and SMART
metadata needed by monitoring refresh paths, so physical-disk temperature and
SMART merges can run from unified `ReadState` instead of from snapshot-owned
disk arrays.
When host-agent SMART and Proxmox physical-disk rows merge, the unified
resource must preserve both the enriched `PhysicalDiskMeta` and the Proxmox
source payload (`ProxmoxData.NodeName`, `Instance`, and source id). A
SMART-enriched disk must not lose Proxmox node membership, or later read-state
refreshes and Proxmox-scoped views can drop the disk even though inventory
still exists. `HostSMARTMeta` must carry `sizeBytes` through adapter, clone,
read-state, and API transport so disk capacity remains available after
registry rebuilds.
That same canonical physical-disk view must also expose source-independent host
context. When a disk is API-backed rather than node-backed, typed views should
fall back to canonical host identity such as `identity.hostnames` instead of
returning an empty node label purely because no Proxmox metadata is present.

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
inventing a separate ad hoc summary path. Dense table rows must bound those
chips with an explicit visible limit and overflow label, while row-level
policy chips are limited to blocking `local-only`/`restricted` posture so
mock-rich canonical resources cannot stack or overstate governance badges
inside the resource column. Those row summaries now prefer canonical
`facetCounts` on the
resource object when available, so the backend list/read shapes remain the
source of truth instead of forcing the frontend to infer totals only from
loaded slices. The drawer now fetches those facets
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
Patrol, performance, or infrastructure surfaces. The shared card must not repeat
a reason line when that reason already equals the canonical headline.
The shared correlation and policy-posture presentation boundaries are also
owned here now. `frontend-modern/src/components/Infrastructure/ResourceCorrelationSummary.tsx`
is the canonical shared card for canonical resource relationships, dependency,
dependent, and learned-correlation context, while
`frontend-modern/src/utils/resourceCorrelationPresentation.ts` owns endpoint
labels, relationship labels, headline formatting, summary wording, and canonical
relationship/correlation ordering. Correlation and relationship badge text must
preserve the formatter's human-readable title case instead of forcing visual
uppercase over enum-like payloads such as `ALERT → ALERT`.
`frontend-modern/src/components/Infrastructure/ResourcePolicySummary.tsx`
is the canonical shared card for governed policy-posture counts, while
`frontend-modern/src/utils/resourcePolicyPresentation.ts` owns the canonical
sensitivity, routing, and redaction labels and aggregate count summaries.
That shared policy card also owns caller-supplied framing lines such as
subtitle and resource-count wording, so Patrol or other shared surfaces may
clarify whether the same governed counts read as policy-covered-resource
context without rebuilding their own policy-posture card shell. New
shared-surface framing needs such as Patrol's `policy-covered resources`
count label or explanatory subtitle should extend that card API instead of
forking a second page-local policy summary.
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
That same unified-resource presentation boundary also owns workload, source,
and service-health labeling now. `frontend-modern/src/utils/workloadTypePresentation.ts`
owns canonical workload-type alias handling plus badge labels/titles,
`frontend-modern/src/utils/sourceTypePresentation.ts` owns canonical source
labels and source badge styling, and
`frontend-modern/src/utils/serviceHealthPresentation.ts` owns canonical
service-health tone, dot, and compact-summary presentation. The PMG-facing
consumer `frontend-modern/src/components/PMG/ServiceHealthBadge.tsx` is part
of that same boundary and must stay a thin render shell over the canonical
service-health helper. Future workload/source/service-health wording changes
should extend these unified-resource owners instead of rebuilding status or
badge logic inside PMG, recovery, dashboard, or infrastructure-local views.
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
Those runtime adapters must preserve the same operator-facing local identity
boundary as the infrastructure tables and selectors: node, PBS, and PMG view
models keep canonical local instance labels for summary rows, drawers, and
settings selectors, while governed summaries remain available for policy/detail
surfaces rather than replacing per-instance operator identity.
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
That same filter contract now includes provider `activity` entries and the
`vmware_adapter` value, so VMware task/event breadcrumbs are narrowed through
the same shared timeline API used for alert, relationship, and action history
instead of adding a provider-specific query surface.
The Connected infrastructure settings surface now also depends on a backend
owned `connectedInfrastructure` projection derived from unified resources plus
reporting-ignore state. That projection is now also the only v6 client
contract for reporting/ignored infrastructure state; future settings-row
grouping or reporting-surface scope changes must be routed through that backend
projection instead of teaching the frontend to reinterpret raw resource facets
or removed-runtime arrays locally.
Canonical route-state helpers must also preserve recovery-specific drill-down
state when they serialize governed resource views. Recovery timeline day
selection is part of the durable route-state contract, so recovery query state
must round-trip the selected day inside an owning platform/runtime route
instead of dropping it as transient local UI state or restoring `/recovery`.
The same recovery route-state contract also applies to the selected timeline
range: canonical recovery query state must preserve explicit non-default chart
windows such as `7d`, `90d`, and `1y` so recovery drill-down transport does
not widen back to the default `30d` window on reload or shared navigation.
That same shared recovery route-state contract also owns the primary recovery
workspace selection. When an operator explicitly switches between protected
items and recovery events, canonical recovery query state must round-trip that
`view` selection unless the active `rollupId` or selected day already implies
the default recovery-events workspace.
That same shared recovery route helper contract now also owns canonical
protected-inventory posture encoding. Visible recovery inventory filters must
round-trip through the owned `state=<value>` query form instead of leaking ad
hoc booleans, overloading event `status`, or disappearing from shared links on
reload. Legacy `stale=1` may be parsed only as compatibility input that
rewrites to canonical inventory state.
That same route-state contract now also owns the canonical recovery `itemType`
query. Recovery query state must round-trip a provider-neutral item category
such as `vm`, `dataset`, or `pvc`, and
`frontend-modern/src/routing/resourceLinks.ts` may canonicalize provider-native
aliases like `proxmox-vm` into that shared vocabulary during parse/build, but
recovery route state must not drift back to raw platform-specific
`subjectType` values in shared navigation.
That same route-state contract also owns the canonical recovery `platform`
query. Recovery query state must emit `platform=<owned-source-key>` as the
shared operator-facing shape, while accepted legacy `provider` aliases may be
parsed only as compatibility input that rewrites back to canonical platform
route state. `frontend-modern/src/routing/resourceLinks.ts` must not keep
legacy `provider` as a first-class build option once parse-time compatibility
has converted it back to canonical recovery route state.
Recovery-linked consumers that decode `/api/recovery/*` payloads must likewise
prefer canonical `platform` / `platforms` response fields over legacy
`provider` aliases, so unified resource drill-downs and shared recovery links
carry the same platform vocabulary on both route and payload boundaries.
That same shared drill-down contract also owns which primary recovery workspace
an upstream surface is targeting. Infrastructure service links that open
platform-level recovery activity must emit canonical recovery route state with
`view=events` inside an owning platform/runtime destination instead of
inheriting the inventory default, and those entry links should describe the
destination as recovery events rather than platform-specific backup wording.
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
That same registry-owned contract now also covers prospective admission and
source replacement. Add/update handlers must project candidates or preview
records through shared monitored-system projection helpers, including the
delta from replacing one existing source-owned surface, instead of guessing
from handler-local priority rules or transport-specific counters.
That same monitored-system contract now also owns the safe hostname
equivalence rule for top-level host attachment and replacement selectors.
Shared grouping may attach `qnap` to `qnap.local` when one surface reports the
short hostname and another reports the FQDN, but it must not collapse two
distinct fully-qualified hosts that merely share the same short prefix across
different domains.
That same projection contract now also owns structured replacement selectors
and detailed previews. Shared callers may serialize source-native selector
fields such as hostname, host URL, machine or agent identity, and source-owned
resource identifiers, but they must not invent preview-only matcher logic or
hide source replacement behind handler-local closures once the request crosses
into unified-resource ownership.
Candidate activity is part of that same canonical projection contract.
`MonitoredSystemCandidate.State` decides whether a candidate contributes a
counted top-level monitored system at all: inactive candidates must not
materialize a projected resource, new-candidate previews must resolve to a
zero-delta/no-change result when only an inactive candidate is proposed, and
replacement previews must remove only the replaced source while preserving any
remaining grouped monitored-system evidence.
Source-native record-set preview is required for provider-backed onboarding
that can discover multiple top-level systems from one saved connection, so
TrueNAS, VMware, and future multi-record providers stay on the same canonical
projection boundary for explanation.
VMware host previews are part of that same contract: canonical host identity
must honor VMware host UUID plus normalized hostnames so a vCenter add or
update explains host-backed systems consistently before and after persistence.

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
Kubernetes pod metrics targets must also canonicalize onto the shared
`k8s:` runtime namespace before they become query coordinates. Bare pod source
IDs such as `cluster-1:pod:pod-1` are discovery keys, not public history IDs;
unified resources and history consumers must expose and query them as
`k8s:cluster-1:pod:pod-1` so pod charts, drawer history, and workload summary
cards stay on one owned series.

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
That same resolved-context boundary now also owns cross-platform app-container
routing. `internal/unifiedresources/resolve.go`,
`internal/unifiedresources/resolve_context.go`, and
`internal/ai/tools/tools_query.go` must register TrueNAS-backed canonical
`app-container` resources into the same resolved-context model as Docker app
containers while preserving adapter-specific routing. Reusing shared
`DockerData` for workload metadata must not misclassify a TrueNAS app as a
Docker-routed control target.
That same contract applies to the frontend workload state boundary:
`frontend-modern/src/hooks/useWorkloads.ts`,
`frontend-modern/src/utils/workloads.ts`,
`frontend-modern/src/components/Workloads/workloadTopology.ts`, and
`frontend-modern/src/components/Workloads/useGuestRowState.ts` may reuse
shared `DockerData` for runtime metadata, but they must keep Docker-only
action identifiers and update affordances scoped to Docker-managed
app-containers. TrueNAS-backed `app-container` workloads must still navigate
through the canonical app-container host path without populating Docker-only
action IDs, but discovery support must come from canonical
`discoveryTarget` ownership rather than synthesized `node` or `instance`
fallbacks. API-backed workloads such as TrueNAS apps may not expose agent-only
Discovery affordances unless the unified resource contract explicitly carries a
discovery target.
That same resolved-context ownership also governs read diagnostics. When
`internal/ai/tools/tools_read.go` executes `pulse_read action="logs"` against
a canonical `app-container` resource, TrueNAS-backed app containers must
route through adapter-aware native read providers keyed by canonical
`resource_id`, not through Docker host/container routing or agent-host
fallbacks.

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
That same topology contract now also includes Proxmox guest pool membership.
`ProxmoxData`, `VMView`, and `ContainerView` must preserve trimmed `pool`
coordinates from the canonical monitoring runtime so inventory, reporting, and
future fleet grouping surfaces do not re-query or infer pool ownership locally.

Frontend unified-resource consumers must now also normalize legacy discovery
resource type aliases before storing `discoveryTarget`. Backend `k8s`
discovery coordinates collapse to the canonical frontend `pod` target, and
typed PBS/storage facets must be preserved as the explicit frontend resource
meta interfaces instead of floating as untyped platform-data consumers.
That same consumer boundary now also owns legacy top-level TrueNAS alias
collapse for discovery and chart consumers: shared frontend hooks may accept
raw `truenas` payloads at ingest, but once normalized the rest of the frontend
must consume the canonical `agent` host type rather than repeating the alias.

Resolved host deduplication must also fail safe on unfamiliar connector types.
Unknown source types may contribute identity and source-label evidence, but
they must not outrank the known canonical primary-type order when a merged host
contains a governed connector such as `proxmox-pve` or `agent`.

Infrastructure selector consumers must also preserve the canonical
`KnownSourcePlatform` normalization boundary when collecting source filters and
status facets. The selector layer may accept arbitrary user-visible filter
strings, but it must not widen the canonical unified-resource source/status
contracts that feed the infrastructure table and workload links.

The same source-filter boundary now also applies to infrastructure source UI
options in Settings and platform/runtime pages. Those surfaces may render
friendly string keys, but membership checks against available sources must
normalize through the shared `frontend-modern/src/utils/sourcePlatforms.ts`
helper before consulting `KnownSourcePlatform` sets.
That same shared source-platform boundary now also owns TrueNAS-backed hybrid
resources. When a canonical resource carries both `agent` and `truenas`
sources, `frontend-modern/src/utils/sourcePlatforms.ts` must still resolve the
platform as `truenas` and the source mode as `hybrid`, so workload and
infrastructure consumers do not collapse API-backed TrueNAS systems or apps
back onto the generic agent path just because host telemetry is also present.
The shared `resolveResourcePlatformType(resource)` helper in that same module
is the canonical "what platform does this unified resource belong to" reader.
Platform-first top-level pages and any consumer that buckets unified resources
by family must call it instead of inspecting `resource.platformType` directly,
because the legacy backend resource projection leaves `platformType` empty on
several canonical resource types; the helper falls back to the resource's
`sources` array via the existing source-platform normalization so mock
fixtures and live backends produce the same client-side platform grouping.
That same boundary also owns the infrastructure table's operator-facing system
identity vocabulary. `frontend-modern/src/utils/resourceBadgePresentation.ts`,
`frontend-modern/src/components/Infrastructure/resourceBadges.ts`, and the
unified resource table sections may preserve full merged-source detail in
tooltips and accessibility metadata, but visible table headers, sort keys, and
row badges must answer what system the operator is looking at. Provider/API
platforms such as Proxmox, TrueNAS, VMware, and Kubernetes outrank collection
methods; reported agent OS or appliance identity such as Unraid or Ubuntu
outranks a generic container-runtime capability; and the shared generic source
label for the `docker` platform is "Docker / Podman" when the runtime capability
is the best available identity. Docker-only wording is reserved for
implementation or action surfaces that are explicitly scoped to Docker-native
behavior. Agent telemetry is collection-method detail when a stronger platform
or host identity is present, not a peer platform label that should crowd the
table or drive the primary system sort.
The former top-level infrastructure feature route never shipped as a stable v6
surface and is intentionally removed rather than preserved as a compatibility
redirect. Canonical infrastructure management lives under
`/settings/infrastructure`, while agentless endpoint availability management
lives under `/settings/monitoring/availability`; day-to-day resource
inspection lives on the platform/runtime pages named by the
frontend-primitives-owned IA contract, with unified resources projecting
standalone agent machines for the Machines consumer and agentless endpoint
checks for the Availability checks consumer. Future infrastructure work must
extend those owners rather than
recreating
`frontend-modern/src/features/infrastructure/`, a `/infrastructure` route, or
a separate top-level availability route.
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
When one counted group is being updated in place, the prospective projection
must remove only the replaced source from that grouped root and preserve any
remaining canonical source ownership that still keeps the monitored system
counted, so replacement-aware preview stays aligned with final runtime
counting.
When support or onboarding needs to explain that same change, unified
resources must be able to return both the current grouped monitored system and
the projected grouped monitored system for the candidate being evaluated, not
just the numeric count delta, so explanation stays on one
canonical projection boundary.

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
That same shared timeline vocabulary now includes the `activity` change kind
for provider-read breadcrumbs such as VMware tasks and events, plus the
`vmware_adapter` source-adapter token for canonical provenance drill-down.
`BuildPlatformActivityChange` must keep those provider reads inside the shared
change model instead of introducing a second event shape, and `RecordChange`
must stay idempotent by canonical change ID so poller refreshes and replayed
supplemental snapshots do not duplicate resource history.
Change emission over registry rebuilds must diff relationships by edge
identity only: canonical source, canonical target, type, and active state,
order-insensitive (`relationshipsEquivalent` in `change_emission.go`).
Volatile provenance fields (`ObservedAt`, `LastSeenAt`, `Confidence`,
`Metadata`, `Discoverer`) refresh on every rebuild cycle and must never
count as a relationship change; comparing rebuilt slices structurally
emitted a no-op `relationship_change` row per relationship-bearing resource
per cycle and grew `resource_changes` without bound (issue #1496, demo
outage 2026-07-08). Retention pruning must also enforce a hard row cap on
`resource_changes` (`maxResourceChangesRows` in `store.go`) so a
pathological writer cannot grow the table unbounded inside the time-based
retention window.
Action plans in `actions.go` still keep stale-plan protection to the canonical
`resourceVersion`, `policyVersion`, and `planHash` fields, so stale execution
checks stay in the shared resource action model rather than provider-local
helpers. The API execution endpoint must consume that model before dispatch:
it rebuilds the current resource/capability plan, rejects mismatched action id,
resource version, policy version, or plan hash as `action_plan_drift`, and
records the refusal as a failed action audit with a `plan_drift:` result.

`actions.go` also owns the canonical drift error vocabulary for the
governed-action broker. `ErrActionPlanDrift` is returned by brokers when
the payload presented at execute time hashes to anything different than
the approval-recorded `planHash`. Brokers must refuse execution rather
than silently downgrade drift into a generic execution error or a
"plan expired" outcome: the contract is "the operator approved exactly
this (command, target, reason) combination" and a different combination
cannot run under the stale approval. The error sits alongside
`ErrActionNotApproved`, `ErrActionPlanExpired`, and the other governed
refusal kinds so callers can distinguish "operator never approved" from
"operator approved a different action" from "approval window passed" —
each is a distinct safety failure that audit review and operator UI
surfaces should treat differently.
The shared change presentation helper also owns the canonical kind, source
type, and source adapter labels for those timeline entries, so summary cards
and drawer history surfaces both read the same badge vocabulary instead of
rebuilding resource-change labels locally.

That frontend consumer rule now applies on the canonical decode path too:
`frontend-modern/src/hooks/useUnifiedResources.ts` must preserve backend-owned
policy metadata, AI-safe summaries, recent changes, and facet counts as
first-class `Resource` fields. It may use the backend refresh path for initial
hydration and unsupported filtered queries, but canonical live freshness for
supported resource snapshots must flow from websocket `state.resources`
instead of layering confirmatory REST refresh loops on top of already-owned
resource updates.
Shared infrastructure consumers such as the unified resource table and detail
drawer must present that owned metadata through shared helpers instead of
reconstructing privacy posture from display names, source types, or other
incidental runtime hints.
That same shared-consumer boundary now also owns VMware phase-1 detail
presentation. `frontend-modern/src/components/Infrastructure/`
`resourceDetailDrawerVmwareModel.ts`,
`ResourceDetailDrawerOverviewTab.tsx`, `ResourceDetailDrawerDebugTab.tsx`,
`resourceDetailDrawerIdentityModel.ts`, and
`useResourceDetailDrawerDerivedState.ts` must surface VMware read-only
placement, signal, and snapshot context through the canonical resource drawer
and debug/source sections rather than introducing a VMware-only detail route,
drawer tab, or provider-local investigation shell.
That same infrastructure consumer boundary also owns source selection
continuity. Settings infrastructure panels and platform/runtime pages must
keep canonical sources such as `truenas` and `availability` present in their
source option sets when the source is known from configuration or route
context, even when the currently loaded unified-resource snapshot does not
contain matching rows yet, so cross-surface handoffs from settings, alerts, or
findings do not collapse back to generic host-only language during hydration
or empty-filter states.
That same frontend-owned compatibility boundary must remain intentionally
narrow. Shared resource adapters may admit explicit aliases such as `host`,
`truenas`, and `ceph`, and VMware detail mappers may project typed metadata
through the canonical resource model, but unified-resource consumers must not
reintroduce removed workload aliases or feature-local resource-type shims just
to satisfy one table, drawer, or badge surface.
