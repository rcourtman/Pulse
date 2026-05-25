# Unified Resources Contract

## Contract Metadata

```json
{
  "subsystem_id": "unified-resources",
  "lane": "L13",
  "contract_file": "docs/release-control/v6/internal/subsystems/unified-resources.md",
  "status_file": "docs/release-control/v6/internal/status.json",
  "registry_file": "docs/release-control/v6/internal/subsystems/registry.json",
  "dependency_subsystem_ids": [
    "api-contracts"
  ]
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
20. `internal/unifiedresources/monitored_system_projection.go`
21. `internal/unifiedresources/hostname_equivalence.go`
22. `internal/unifiedresources/capabilities.go`
23. `internal/unifiedresources/changes.go`
24. `internal/unifiedresources/relationships.go`
25. `internal/unifiedresources/privacy.go`
26. `internal/unifiedresources/actions.go`
27. `internal/unifiedresources/audit_redaction.go`
28. `frontend-modern/src/components/Infrastructure/ResourceDetailDrawer.tsx`
29. `frontend-modern/src/components/Infrastructure/ResourceDetailDrawerOverviewTab.tsx`
30. `frontend-modern/src/components/Infrastructure/ResourceDetailDrawerDebugTab.tsx`
31. `frontend-modern/src/components/Infrastructure/ResourceDetailDrawerSupportDisclosure.tsx`
32. `frontend-modern/src/components/Docker/SwarmServicesDrawer.tsx`
33. `frontend-modern/src/features/docker/DockerConfigsTable.tsx`
34. `frontend-modern/src/features/docker/DockerContainersTable.tsx`
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
46. `frontend-modern/src/components/PMG/PMGInstanceDrawer.tsx`
47. `frontend-modern/src/components/PMG/MailGateway.tsx`
48. `frontend-modern/src/components/PMG/PMGInstancePanel.tsx`
49. `frontend-modern/src/components/Infrastructure/ResourceActionHistory.tsx`
50. `frontend-modern/src/components/Infrastructure/ResourceFacetSummary.tsx`
51. `frontend-modern/src/components/Infrastructure/ResourceChangeSummary.tsx`
52. `frontend-modern/src/components/Infrastructure/ResourceCorrelationSummary.tsx`
53. `frontend-modern/src/components/Infrastructure/ResourcePolicySummary.tsx`
54. `frontend-modern/src/components/Infrastructure/resourceBadges.ts`
55. `frontend-modern/src/components/Infrastructure/UnifiedResourceHostTableCard.tsx`
56. `frontend-modern/src/components/Infrastructure/UnifiedResourcePBSTableSection.tsx`
57. `frontend-modern/src/components/Infrastructure/UnifiedResourcePMGTableSection.tsx`
58. `frontend-modern/src/components/Infrastructure/UnifiedResourceServiceInfrastructureCard.tsx`
59. `frontend-modern/src/components/Infrastructure/unifiedResourceTableModel.ts`
60. `frontend-modern/src/components/Infrastructure/unifiedResourceTableStateModel.ts`
61. `frontend-modern/src/components/Infrastructure/useResourceDetailDrawerDerivedState.ts`
62. `frontend-modern/src/components/Infrastructure/resourceDetailDrawerServiceModel.ts`
63. `frontend-modern/src/components/Infrastructure/resourceDetailDrawerVmwareModel.ts`
64. `frontend-modern/src/components/Infrastructure/resourceDetailDiscoveryModel.ts`
65. `frontend-modern/src/components/Infrastructure/resourceDetailDrawerOperationalModel.ts`
66. `frontend-modern/src/components/Infrastructure/useResourceDetailDrawerHistoryState.ts`
67. `frontend-modern/src/components/Infrastructure/useResourceDetailDrawerDockerActionsState.ts`
68. `frontend-modern/src/components/Infrastructure/useResourceDetailDrawerState.ts`
69. `frontend-modern/src/components/Infrastructure/useUnifiedResourceTableState.ts`
70. `frontend-modern/src/components/Infrastructure/useUnifiedResourceTableViewportSync.ts`
71. `frontend-modern/src/components/Discovery/DiscoveryTab.tsx`
72. `frontend-modern/src/components/Discovery/useDiscoveryTabState.ts`
73. `frontend-modern/src/components/Infrastructure/InfrastructureSummary.tsx`
74. `frontend-modern/src/components/Infrastructure/useInfrastructureSummaryState.ts`
75. `frontend-modern/src/components/Infrastructure/infrastructureSummaryModel.ts`
76. `frontend-modern/src/utils/agentResources.ts`
77. `frontend-modern/src/utils/canonicalResourceTypes.ts`
78. `frontend-modern/src/utils/resourceBadgePresentation.ts`
79. `frontend-modern/src/utils/resourceChangePresentation.ts`
80. `frontend-modern/src/utils/actionAuditPresentation.ts`
81. `frontend-modern/src/utils/resourceCorrelationPresentation.ts`
82. `frontend-modern/src/utils/resourcePlatformData.ts`
83. `frontend-modern/src/utils/resourcePolicyPresentation.ts`
84. `frontend-modern/src/utils/resourceStateAdapters.ts`
85. `frontend-modern/src/utils/resourceTypeCompat.ts`
86. `frontend-modern/src/utils/resourceTypePresentation.ts`
87. `frontend-modern/src/utils/serviceHealthPresentation.ts`
88. `frontend-modern/src/utils/sourceTypePresentation.ts`
89. `frontend-modern/src/utils/workloadTypePresentation.ts`
90. `frontend-modern/src/components/PMG/ServiceHealthBadge.tsx`
91. `frontend-modern/src/utils/resourceIdentity.ts`
92. `frontend-modern/src/components/Infrastructure/resourceDetailDrawerIdentityModel.ts`
93. `frontend-modern/src/hooks/useUnifiedResources.ts`
94. `frontend-modern/src/types/resource.ts`
95. `frontend-modern/src/utils/sourcePlatforms.ts`
96. `frontend-modern/src/utils/platformSupportManifest.generated.ts`
97. `internal/unifiedresources/kubernetes_metric_ids.go`
98. `internal/unifiedresources/policy_posture.go`
99. `internal/unifiedresources/clone.go`
100. `frontend-modern/src/components/Infrastructure/resourceDetailDrawerPresentation.ts`
101. `internal/unifiedresources/storage_consumers.go`
102. `frontend-modern/src/features/standalone/standalonePageModel.ts`
103. `frontend-modern/src/features/standalone/StandalonePageSurface.tsx`
104. `frontend-modern/src/features/standalone/AgentsMachinesTable.tsx`
105. `frontend-modern/src/features/standalone/AvailabilityChecksTable.tsx`
106. `internal/platformsupport/manifest_generated.go`
107. `frontend-modern/src/features/kubernetes/KubernetesControllersTable.tsx`
108. `frontend-modern/src/features/kubernetes/KubernetesPageSurface.tsx`
109. `frontend-modern/src/features/kubernetes/kubernetesPageModel.ts`
110. `frontend-modern/src/features/kubernetes/KubernetesClustersTable.tsx`
111. `frontend-modern/src/features/kubernetes/KubernetesDeploymentsTable.tsx`
112. `frontend-modern/src/features/kubernetes/KubernetesNodesTable.tsx`
113. `frontend-modern/src/features/kubernetes/KubernetesPodsTable.tsx`
114. `frontend-modern/src/features/kubernetes/KubernetesStorageTable.tsx`
115. `frontend-modern/src/features/kubernetes/KubernetesNetworkingTable.tsx`
116. `frontend-modern/src/features/kubernetes/KubernetesServicesTable.tsx`
117. `frontend-modern/src/features/kubernetes/KubernetesConfigTable.tsx`
118. `frontend-modern/src/features/kubernetes/KubernetesPolicyTable.tsx`
119. `frontend-modern/src/features/kubernetes/KubernetesAutoscalingTable.tsx`
120. `frontend-modern/src/features/kubernetes/KubernetesEventsTable.tsx`

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
platform. Its subtabs may be shown only from canonical resource evidence
(`app-container` for containers, `docker-service` for Swarm services), and
inactive standalone Swarm metadata must not be interpreted as host-role or
service-surface proof. The unified-resource adapter is the backend fail-closed
layer for that rule, so persisted or older-agent inactive Swarm payloads cannot
reintroduce false Swarm capability surfaces.

Platform Overview tabs are rollup boundaries, not duplicate inventory dumps.
Docker / Podman Overview owns runtime host rows only; container, image, storage,
network, and Swarm object rows belong in their workflow tabs. Kubernetes
Overview owns cluster/control-plane rollup rows only; node, workload, service,
storage, configuration, policy, and event object rows belong in their workflow
tabs. If a future Overview repeats a detailed table, the owning workflow must be
retired or the Overview content must be reduced to aggregate signal.

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
15. `frontend-modern/src/utils/platformSupportManifest.generated.ts` shared with `frontend-primitives`: the generated platform support projection is both a canonical unified-resource platform union boundary and a shared frontend source/platform vocabulary boundary.
    It must carry the manifest `surface_kind` distinction so `docker` remains
    machine-readable as a `runtime-lens` while owning infrastructure sources
    remain `platform` entries.
16. `frontend-modern/src/utils/sourcePlatforms.ts` shared with `frontend-primitives`: the source platform normalizer is both a canonical unified-resource source adapter boundary and a shared frontend source/platform vocabulary boundary.
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
17. `internal/api/resources.go` shared with `api-contracts`: the unified resource endpoint is both a backend payload contract surface and a unified-resource runtime boundary.
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
   `DockerData` so `/docker/containers` can use the native
   `DockerContainersTable` rather than `WorkloadsSurface`. Runtime image,
   volume, network, task, Swarm node, Swarm secret, Swarm config, and
   storage-usage evidence must enter through `DockerData` and the typed Docker
   resource records (`docker-image`, `docker-volume`, `docker-network`,
   `docker-task`, `docker-swarm-node`, `docker-secret`, `docker-config`) rather
   than being inferred inside the container page. Swarm node records must
   preserve node id, hostname, role, availability, state, manager reachability,
   manager address, leader state, engine version, platform, resource capacity,
   labels, and engine labels under the owning Docker host or Swarm cluster
   identity.
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
3. Add source ingestion/adaptation in the adapter layer only
   Frontend resource platform contracts in
   `frontend-modern/src/types/resource.ts` must include platform ids that the
   canonical source/platform adapters can emit. Agent-native storage sources
   such as Unraid are first-class platform ids for typing, filtering, tests,
   and storage/recovery presentation; consumers must not hide them behind
   Proxmox-only or generic agent-only frontend unions when unified resources
   has already published explicit platform evidence.
   Infrastructure table platform presentation extends through
   `frontend-modern/src/utils/resourceBadgePresentation.ts`,
   `frontend-modern/src/components/Infrastructure/resourceBadges.ts`, and the
   unified resource table sections. Visible infrastructure table headers,
   filters, sort keys, and compact row badges must present the owning
   infrastructure platform first, while full merged-source detail remains
   available for tooltips, accessibility metadata, and routing. Agent telemetry
   is collection-method detail when a provider/API platform is also present.
   When the primary system identity includes a platform version, table metadata
   must suppress an otherwise duplicate unversioned platform source badge while
   preserving non-duplicate collection context such as Pulse Agent.
   Infrastructure table presentation controls must describe the table mode
   rather than a platform-specific resource concept: the grouped/flat toggle
   uses operator-facing `Grouped` and `List` wording with a `Group by`
   accessible group label, while Proxmox, Kubernetes, and other platform
   clusters stay reserved for actual resource identity, filters, and detail
   surfaces.
   Infrastructure row drawer expansion and summary group focus are local table
   interaction state. Inbound infrastructure deep links may hydrate `resource`
   and `summaryGroup` into the focused resource or group, but local expansion,
   closing, and clearing must not rewrite those params or navigate. Route sync
   for this surface is limited to filter-owned `source` and `q` state so
   unified-resource rows open inline without refreshing the page shell.
   Resource detail drawers must declare their presentation context explicitly.
   The default full presentation owns the canonical resource-change,
   operator-override, maintenance-verification, and action-audit surfaces. The
   `table-row` presentation is reserved for inline platform and resource table
   expansion: it presents current-state and identity details from the local
   resource payload, may show preloaded change data when present, and must not
   start remote history, intelligence, or action-audit reads only to render
   empty advanced governance panels inside a table row.
   Cluster group headers may use a compact `Cluster` type chip when the group
   name itself is only an estate label, but `unifiedResourceTableStateModel.ts`
   must suppress that chip when the visible group name already ends in
   `Cluster`, so rows do not read as duplicated labels such as `Production
   Cluster Cluster`.
   Infrastructure host and service table cards must consume the
   frontend-primitives-owned `TableCard` frame and `TableCardHeader` header
   band; unified resources owns the resource identity, rows, grouping, and
   detail presentation, not page-local table-shell chrome. The nested
   infrastructure tables must use the shared `Table` primitive's scroll frame
   directly instead of reintroducing page-local `overflow-x-auto` wrappers
   inside the canonical card shell.
   Unified resource host metric cells may consume alert-backed agent
   thresholds, but resource consumers must not resolve alert defaults or
   overrides locally. Host table state exposes the alerts-owned threshold
   resolver and row renderers pass the resolved values into metric bars while
   preserving unified-resource ownership over identity, rows, grouping, and
   detail presentation.
   Docker Swarm service, Kubernetes namespace/deployment, and PMG detail
   tables follow that same shell boundary: their platform-specific rows and
   actions are unified-resource-owned, while horizontal overflow is owned by
   the shared `Table` wrapper rather than drawer-local scroll divs. This
   includes both current resource drawers and legacy PMG resource panels that
   still consume unified-resource PMG state.
   Realtime resource payloads are part of the same consumer contract as REST:
   websocket broadcasts must carry canonical identity, discovery targets,
   metrics targets, incident rollups, facet counts, and source-specific raw
   facets such as `agent`, `storage`, `docker`, and provider API facets. Host
   rows and drawers may present compact issue labels such as Unraid
   no-parity posture, but those labels must derive from canonical incident or
   storage-risk summaries already present on the resource rather than local
   platform heuristics or generic status-only wording.
   Agent-backed Unraid topology belongs in the canonical adapter layer, not in
   storage-page inference. Assigned Unraid array members must produce physical
   disk resources even when SMART telemetry is absent, Unraid cache/pool members
   must produce agent-backed storage resources with native capacity metrics, and
   SMART rows may enrich or merge with those resources only through normalized
   device or serial identity. Generic SMART status must not overwrite stronger
   Unraid health/risk, and physical-disk metric IDs must prefer stable serials
   while normalizing device tokens such as `/dev/sdd` and `sdd [sat]` to the
   same fallback identity.
   Frontend canonicalization must treat top-level source facets as source
   evidence when `platformData.sources` is absent, but it must not invent
   provider facets from generic agent telemetry: flat agent disk payloads stay
   under the agent source and do not become Proxmox facets without explicit
   Proxmox source, node, or workload-shape evidence.
   Realtime resource adapters must also apply the canonical host source-bridge
   rule to incoming websocket snapshots: agent plus platform/API evidence for
   the same host coalesces into one hybrid infrastructure resource, while
   same-name agent-only records remain separate until stronger identity or
   platform evidence exists.
   Proxmox child resources must derive their canonical parent from the owning
   node identity across all supported source-key shapes. Guests, storage pools,
   and physical disks may arrive with instance-node, cluster-node, or bare-node
   parent evidence, and both snapshot ingest and already-unified registry seed
   paths must attach them to the same merged host resource before REST,
   websocket, Workloads, or Infrastructure consumers render the estate.
   This derivation must tolerate node source IDs that retain a previous cluster
   alias while the current Proxmox metadata exposes a renamed instance or
   cluster label; current node metadata is the fallback source of truth when
   source-key lookup misses.
   Platform pages that bucket Docker / Podman or Kubernetes resources must use
   these canonical resource types and source facets directly. Page-local
   scraping, generic type coercion, or "single table for every native object"
   fallbacks are not a substitute for adapter-owned image, volume, network,
   service, controller, storage, ingress, and event resource projections.

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
7. Add or change resource drawer timeline/facet/action-history presentation through `frontend-modern/src/components/Infrastructure/ResourceDetailDrawer.tsx`, `frontend-modern/src/components/Infrastructure/ResourceDetailDrawerOverviewTab.tsx`, `frontend-modern/src/components/Infrastructure/ResourceDetailDrawerDebugTab.tsx`, `frontend-modern/src/components/Docker/SwarmServicesDrawer.tsx`, `frontend-modern/src/components/Kubernetes/K8sDeploymentsDrawer.tsx`, `frontend-modern/src/components/Kubernetes/K8sNamespacesDrawer.tsx`, `frontend-modern/src/components/PMG/PMGInstanceDrawer.tsx`, `frontend-modern/src/components/PMG/MailGateway.tsx`, `frontend-modern/src/components/PMG/PMGInstancePanel.tsx`, `frontend-modern/src/components/Infrastructure/ResourceActionHistory.tsx`, `frontend-modern/src/components/Infrastructure/useResourceDetailDrawerState.ts`, `frontend-modern/src/components/Infrastructure/ResourceFacetSummary.tsx`, `frontend-modern/src/components/Infrastructure/resourceDetailMappers.ts`, and the governed `internal/api/resources.go` facet/timeline plus `internal/api/activity_audit_handlers.go` action-audit contracts together
8. Add or change discovery-support runtime under the resource drawer through `frontend-modern/src/components/Discovery/DiscoveryTab.tsx` for shell/presentation ownership and `frontend-modern/src/components/Discovery/useDiscoveryTabState.ts` for fetch, websocket-progress, manual-run triggering, and notes-mutation ownership. Embedded drawers may expose the top-level run action through this shared Discovery tab, but they must still call the canonical discovery trigger state path instead of introducing drawer-local API mutations.
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
10. Keep the retired dashboard overview shell absent from unified-resource
    consumers. `frontend-modern/src/pages/Dashboard.tsx`,
    `frontend-modern/src/hooks/useDashboardOverview.ts`, and
    `/api/resources/dashboard-summary` must not be restored as compatibility
    readers for KPI cards, problem-resource rows, or top-resource identity.
    Infrastructure and Workloads must consume their owning unified-resource
    projections directly.
11. Keep summary consumers on the payload they were already given.
    `frontend-modern/src/components/Infrastructure/useInfrastructureSummaryState.ts`
    may derive chart identity, storage presence, and infrastructure rollups
    from the resource snapshot it already owns, but it must not reopen
    `useResources()` or start a second unfiltered unified-resource fetch path
    as a replacement dashboard summary surface.
12. Keep shared selector hydration visibility-bound. Reusable shells such as
    `frontend-modern/src/components/shared/useInfrastructureSelectorState.ts`
    may consume `frontend-modern/src/hooks/useUnifiedResources.ts`, but hidden
    selectors must pass an explicit `enabled` gate instead of booting the
    `all-resources` transport behind a collapsed or non-rendered summary shell.
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
    table must hide lower-priority resource metadata columns rather than
    publishing a horizontally scrolling canonical resource list.
15. Keep shared policy-posture framing on the unified-resource card owner.
    `frontend-modern/src/components/Infrastructure/ResourcePolicySummary.tsx`
    may accept caller-owned subtitle or resource-count wording when Patrol or
    another shared surface needs to explain how the same governed policy counts
    should be read, but those framing lines must extend the shared card API
    rather than spawning page-local policy summary shells.
16. Keep platform/runtime top-level route paths on the canonical resource-link
    helper. `frontend-modern/src/routing/resourceLinks.ts` owns the
    `STANDALONE_PATH`, `DOCKER_PATH`, `KUBERNETES_PATH`, `TRUENAS_PATH`,
    `VMWARE_PATH` constants and the `buildStandalonePath`, `buildDockerPath`,
    `buildKubernetesPath`, `buildTrueNASPath`, `buildVmwarePath` builders.
    Per-platform surfaces and tab specs must
    derive every internal link from those builders so the canonical resource
    URL vocabulary stays single-sourced; ad hoc string concatenation of
    platform routes inside feature directories is not permitted.
    The `Standalone` route is the canonical browser projection of Pulse-managed
    standalone agent rows and agentless availability endpoint rows: agent
    membership must require
    `resource.type === "agent"`, canonical Pulse-agent source evidence from
    resource sources or source status, and no stronger provider-owner evidence
    from Proxmox, VMware, TrueNAS, or Kubernetes. Source-less legacy snapshots
    may fall back to a normalized `platformType === "agent"`, but
    provider-owned nodes must not become Standalone-page members through hostname,
    `agent` platform scope, or agent telemetry alone; those facts surface as
    facets on the owning provider page.
    The default tab for each platform path must point at a sub-tab whose
    canonical unified-resource projection actually populates. The
    canonical TrueNAS adapter (`internal/truenas/provider.go::
    truenasRecordsFromSnapshot`) already emits the top-level TrueNAS
    appliance as a unified `agent` row tagged with the `truenas`
    platform, so TrueNAS defaults to `/truenas/overview` (the Systems
    sub-tab); the embedded `StorageSurface` lives at `/truenas/storage`.
    Any future platform that wants to default to a Systems / Hosts
    overview must first have its canonical resource adapter project the
    platform's top-level system as a unified resource so the builder
    default still resolves to a populated table.

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
   and HTTP checks is `/settings/monitoring/availability?add=target`.
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
    or path heuristics.
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

## Current State

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

The Proxmox platform page is a route-level consumer of canonical unified
resources, not a new resource source. It filters the existing resource snapshot
to Proxmox VE, Backup Server, Mail Gateway, storage, disk, and workload rows,
then composes the existing Workloads, Storage, Recovery, and infrastructure
table owners. Proxmox host row version, uptime, temperature, CPU, memory, disk,
network I/O, and disk I/O presentation must derive from canonical resource
facets and the shared `nodeFromResource` adapter; platform pages must not
rebuild resource identity, merge policy, or metric-target inference locally.

`resource_operator_state.go` owns the operator-set per-resource intent
schema. `ResourceOperatorState` carries four narrow operator-intent
fields (`IntentionallyOffline`, `NeverAutoRemediate`, maintenance
window via `MaintenanceStartAt` / `MaintenanceEndAt` / `MaintenanceReason`,
and a canonical `Criticality` hint of `high|medium|low|""`) plus
operator-attribution metadata (`Note`, `SetAt`, `SetBy`). The shape is
intentionally fixed-field rather than a freeform metadata bag so
downstream finding-suppression and severity-weighting logic has a stable
contract to honor. `ValidateResourceOperatorState` rejects malformed
records with a stable `ErrResourceOperatorStateInvalid` sentinel
(empty canonical id, partial maintenance window, end ≤ start, unknown
criticality), and `NormalizeResourceOperatorState` trims whitespace
and lower-cases the criticality value before persistence. The
`ResourceStore` interface now exposes
`GetResourceOperatorState`, `SetResourceOperatorState`, and
`ClearResourceOperatorState`; both the SQLite (table
`resource_operator_state` keyed on `canonical_id`) and Memory stores
implement the same upsert + idempotent-clear contract. The same
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
`/api/resources/{id}/operator-state`. The drawer integration is
read-only on maintenance windows (the section badges an active
window when `now` falls within it but does not let the operator
schedule one — that lives in a follow-up slice). The two boolean
toggles (intentionally offline, never auto-remediate) are the
operator-facing primitives this slice surfaces.

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
preserve that payload and incident state without trying to fold endpoints into
hosts, VMs, or storage resources solely because an address matches.
Frontend resource adapters must preserve that same availability identity on
both REST and realtime paths: a thin `network-endpoint` update with
availability data is still `platformType=availability`, `sourceType=api`, and
must not regress to a generic platform badge in infrastructure rows or drawers.
Infrastructure row presentation must also consume that availability payload as
operator evidence, not only as badge identity. `network-endpoint` rows should
surface one visible probe readout from either the top-level availability field
or the live-state `platformData.availability` mirror: the System column uses
the probe protocol as the compact identity badge (`ICMP`, `TCP`, or `HTTP`),
while the metric cell shows only the target detail and latest latency or
failure result, such as `6053: 11 ms`, `/status: 503`, `3 ms`, or `timed out`.
The Standalone surface is the operational home for those same
agentless checks, not a new top-level platform page. `StandalonePageSurface.tsx`
must fetch both `agent` and `network-endpoint` resources, keep standalone
machines and availability checks as separate buckets in `standalonePageModel.ts`,
and let `AvailabilityChecksTable.tsx` render saved probe method, target,
latest result, check age, failure count, and cadence from the canonical
availability payload.
Recent check timing and fuller failure context may stay in tooltip or drawer
detail, but the table row must not duplicate the same probe protocol and
result text across both identity and metric cells.
That same frontend-owned compatibility boundary must remain intentionally
narrow. Shared resource adapters may admit explicit aliases such as `host`,
`truenas`, and `ceph`, and VMware detail mappers may project typed metadata
through the canonical resource model, but unified-resource consumers must not
reintroduce removed workload aliases or feature-local resource-type shims just
to satisfy one table, drawer, or badge surface.
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
Agentless availability fixtures join that same graph-owned seed contract:
mock UPS, MQTT, ESPHome, HTTP, and controller endpoints must project as
`SourceAvailability` `network-endpoint` resources with real availability
payloads, incidents, and source status rather than as generic host rows or
settings-only sample data.
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
frontend feed instead of separate page-local loops.
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
The in-memory store mirrors the durable audit contract by upserting action
audits on action ID, so tests and runtime callers observe the same current
record state that SQLite persists for the control-plane execution trail.
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
Patrol, performance, or infrastructure surfaces.
The shared correlation and policy-posture presentation boundaries are also
owned here now. `frontend-modern/src/components/Infrastructure/ResourceCorrelationSummary.tsx`
is the canonical shared card for canonical resource relationships, dependency,
dependent, and learned-correlation context, while
`frontend-modern/src/utils/resourceCorrelationPresentation.ts` owns endpoint
labels, relationship labels, headline formatting, summary wording, and canonical
relationship/correlation ordering.
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
inspection lives on the platform/runtime pages, with standalone agent machines
and agentless endpoint checks sharing the Standalone surface. Future
infrastructure work must extend those owners rather than recreating
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
