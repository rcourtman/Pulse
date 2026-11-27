import type {
  Resource,
  ResourceIncident,
  ResourceKubernetesPodContainerStatus,
  ResourceType,
} from '@/types/resource';
import type { Alert } from '@/types/api';
import {
  getSimpleStatusIndicator,
  type StatusIndicator,
  type StatusIndicatorVariant,
} from '@/utils/status';
import { getUnifiedResourceAlertStyles } from '@/utils/alerts';
import { getResourceSourceStatus, hasImpairedResourceSource } from '@/utils/resourceSourceHealth';
import { resolveResourcePlatformType } from '@/utils/sourcePlatforms';
import { matchesSearchTermSplit, splitSearchExclusions } from '@/utils/searchQuery';

export type KubernetesPageTabId =
  'overview' | 'nodes' | 'workloads' | 'services' | 'storage' | 'configuration' | 'events';

export type KubernetesTabSpec = {
  id: KubernetesPageTabId;
  label: string;
  path: string;
};

// Keep Kubernetes tabs at the operator-workflow level. The API exposes many
// object kinds, but the page should not become one tab per kind or repeat a
// detailed table in Overview when that table already has a dedicated workflow
// home.
export const KUBERNETES_TAB_SPECS: readonly KubernetesTabSpec[] = [
  { id: 'overview', label: 'Overview', path: '/kubernetes/overview' },
  { id: 'nodes', label: 'Nodes', path: '/kubernetes/nodes' },
  { id: 'workloads', label: 'Workloads', path: '/kubernetes/workloads' },
  { id: 'services', label: 'Services', path: '/kubernetes/services' },
  { id: 'storage', label: 'Storage', path: '/kubernetes/storage' },
  { id: 'configuration', label: 'Configuration', path: '/kubernetes/configuration' },
  { id: 'events', label: 'Events', path: '/kubernetes/events' },
] as const;

const asTrimmedString = (value: unknown): string => (typeof value === 'string' ? value.trim() : '');

const eventTypeLabel = (value: string, fallback: string): string => {
  const trimmed = asTrimmedString(value);
  if (!trimmed) return fallback;
  return trimmed.charAt(0).toUpperCase() + trimmed.slice(1);
};

export function mapKubernetesEventSeverity(eventType: string | undefined): StatusIndicator {
  const normalized = asTrimmedString(eventType).toLowerCase();
  if (normalized === 'warning') return { variant: 'warning', label: 'Warning' };
  if (normalized === 'normal') return { variant: 'muted', label: 'Normal' };
  return {
    variant: 'muted',
    label: eventTypeLabel(eventType ?? '', 'Unknown'),
  };
}

const parseKubernetesEventObservedTime = (resource: Resource): number => {
  const observed =
    asTrimmedString(resource.kubernetes?.eventTime) ||
    asTrimmedString(resource.kubernetes?.firstSeen) ||
    asTrimmedString(resource.kubernetes?.createdAt);
  if (!observed) return 0;
  const timestamp = Date.parse(observed);
  return Number.isFinite(timestamp) ? timestamp : 0;
};

export const compareKubernetesEvents = (left: Resource, right: Resource): number => {
  const timeDelta =
    parseKubernetesEventObservedTime(right) - parseKubernetesEventObservedTime(left);
  if (timeDelta !== 0) return timeDelta;
  return left.id.localeCompare(right.id);
};

// Container-state reasons that mean "the kubelet can't get this container
// to a running state". Distinct from a transient `Pending` phase: these
// are the reasons that should escalate the pod row to a danger dot regardless
// of the surrounding phase.
const POD_CONTAINER_FATAL_REASONS = new Set([
  'crashloopbackoff',
  'imagepullbackoff',
  'errimagepull',
  'createcontainerconfigerror',
  'createcontainererror',
  'invalidimagename',
  'runcontainererror',
  'oomkilled',
]);

const normalizeKubernetesToken = (value: unknown): string =>
  typeof value === 'string'
    ? value
        .trim()
        .toLowerCase()
        .replace(/[\s_-]/g, '')
    : '';

const displayName = (resource: Resource): string =>
  asTrimmedString(resource.displayName) ||
  asTrimmedString(resource.name) ||
  asTrimmedString(resource.kubernetes?.podName) ||
  resource.id;

// Scope labels prefer the human-facing cluster name; the internal cluster id
// is only a fallback for rows whose adapter carries no name. Every Kubernetes
// page table shares this so the same cluster never reads as "Production EU"
// in one column and "k8s-production-1" in another.
// Pod names end in the generated segment that tells replicas apart
// (checkout-api-6d8f9c7b5-x7k2p, web-0), which is exactly what end truncation
// cuts off in a narrow column. Splitting it out lets the table truncate the
// head and keep the tail, so a long name reads checkout-api-6d…-x7k2p.
const KUBERNETES_NAME_TAIL_MAX = 10;

export function splitKubernetesNameTail(name: string): { head: string; tail: string } {
  const cut = name.lastIndexOf('-');
  if (cut <= 0 || name.length - cut - 1 > KUBERNETES_NAME_TAIL_MAX || cut === name.length - 1) {
    return { head: name, tail: '' };
  }
  return { head: name.slice(0, cut), tail: name.slice(cut) };
}

export const kubernetesClusterLabel = (resource: Resource): string =>
  asTrimmedString(resource.kubernetes?.clusterName) ||
  asTrimmedString(resource.kubernetes?.clusterId);

export const kubernetesScopeLabel = (resource: Resource): string => {
  const cluster = kubernetesClusterLabel(resource);
  const namespace = asTrimmedString(resource.kubernetes?.namespace);
  if (namespace) return cluster ? `${cluster}/${namespace}` : namespace;
  return cluster || 'Cluster';
};

const containerHasFatalReason = (container: ResourceKubernetesPodContainerStatus): boolean => {
  const reason = normalizeKubernetesToken(container.reason);
  if (reason && POD_CONTAINER_FATAL_REASONS.has(reason)) return true;
  const state = normalizeKubernetesToken(container.state);
  return state === 'terminated' && !container.ready;
};

const podHasFatalContainer = (containers: ResourceKubernetesPodContainerStatus[]): boolean =>
  containers.some(containerHasFatalReason);

const podAllContainersReady = (containers: ResourceKubernetesPodContainerStatus[]): boolean =>
  containers.length > 0 && containers.every((container) => container.ready === true);

// With one cluster in view, every row's scope repeats the cluster name and
// crowds out the namespace, the part that differs between rows. Tables then
// show the namespace alone (Cluster-wide for cluster-scoped objects) and keep
// the full cluster/namespace scope on hover. Several clusters keep both.
// Clusters are told apart by ID (two can share a display name), and a row with
// no cluster at all counts as its own, so a mix keeps the full scope.
const kubernetesClusterKey = (resource: Resource): string =>
  asTrimmedString(resource.kubernetes?.clusterId) ||
  asTrimmedString(resource.kubernetes?.clusterName) ||
  '';

export const isSingleKubernetesCluster = (resources: readonly Resource[]): boolean =>
  new Set(resources.map(kubernetesClusterKey)).size <= 1;

export const kubernetesScopeDisplayLabel = (resource: Resource, singleCluster: boolean): string =>
  singleCluster
    ? asTrimmedString(resource.kubernetes?.namespace) || 'Cluster'
    : kubernetesScopeLabel(resource);

export const KUBERNETES_STALE_READINGS_LABEL = 'Stale data';

export const hasStaleKubernetesReadings = (resource: Resource): boolean =>
  normalizeKubernetesToken(getResourceSourceStatus(resource, 'kubernetes')?.status) === 'stale';

const hasImpairedKubernetesSource = (resource: Resource): boolean =>
  hasImpairedResourceSource(resource, 'kubernetes');

const getKubernetesSourceError = (resource: Resource): string =>
  asTrimmedString(getResourceSourceStatus(resource, 'kubernetes')?.error);

export const describeKubernetesStaleReadings = (resource: Resource): string => {
  if (!hasStaleKubernetesReadings(resource)) return '';
  const sourceError = getKubernetesSourceError(resource);
  const base = 'Showing the last Kubernetes report — no fresh update received recently.';
  return sourceError ? `${base} Last error: ${sourceError}` : base;
};

// Keep real failure/attention verdicts ahead of staleness so an operator still
// sees NotReady, CrashLoopBackOff, or an under-replicated rollout, while
// preventing retained healthy snapshots from reading green when Kubernetes
// stops updating.
const applyKubernetesSourceFreshness = (
  resource: Resource,
  base: StatusIndicator,
): StatusIndicator => {
  if (base.variant === 'danger' || base.variant === 'warning') return base;
  if (hasStaleKubernetesReadings(resource)) {
    return { variant: 'warning', label: KUBERNETES_STALE_READINGS_LABEL };
  }
  if (hasImpairedKubernetesSource(resource)) {
    return { variant: 'warning', label: 'Degraded' };
  }
  return base;
};

export function mapKubernetesClusterStatus(resource: Resource): StatusIndicator {
  const base = getSimpleStatusIndicator(resource.status);
  if (base.variant === 'danger') return base;
  return applyKubernetesSourceFreshness(resource, base);
}

export function mapKubernetesPodStatus(resource: Resource): StatusIndicator {
  const phase = normalizeKubernetesToken(
    resource.kubernetes?.podPhase || resource.kubernetes?.phase,
  );
  const containers = resource.kubernetes?.podContainers ?? [];

  if (phase === 'failed') return { variant: 'danger', label: 'Failed' };
  if (podHasFatalContainer(containers)) {
    const reason = containers.find(containerHasFatalReason)?.reason?.trim() || 'Container error';
    return { variant: 'danger', label: reason };
  }
  if (phase === 'pending') return { variant: 'warning', label: 'Pending' };
  if (phase === 'running') {
    if (containers.length === 0 || podAllContainersReady(containers)) {
      return applyKubernetesSourceFreshness(resource, { variant: 'success', label: 'Running' });
    }
    return { variant: 'warning', label: 'Not ready' };
  }
  if (phase === 'succeeded') {
    return applyKubernetesSourceFreshness(resource, { variant: 'success', label: 'Succeeded' });
  }
  if (phase === 'unknown' || !phase) {
    return applyKubernetesSourceFreshness(resource, { variant: 'muted', label: 'Unknown' });
  }
  return applyKubernetesSourceFreshness(resource, {
    variant: 'muted',
    label: eventTypeLabel(resource.kubernetes?.podPhase ?? '', 'Unknown'),
  });
}

export function mapKubernetesNodeStatus(resource: Resource): StatusIndicator {
  const ready = resource.kubernetes?.ready;
  if (ready === false) return { variant: 'danger', label: 'NotReady' };
  // A cordoned node still reports ready=true; check unschedulable before
  // declaring the row healthy or the cordon is invisible on the page.
  if (resource.kubernetes?.unschedulable === true) {
    return { variant: 'warning', label: 'Unschedulable' };
  }
  const status = normalizeKubernetesToken(resource.status);
  if (status === 'offline' || status === 'stopped' || status === 'failed') {
    return { variant: 'danger', label: ready === true ? 'Offline' : 'NotReady' };
  }
  if (hasStaleKubernetesReadings(resource)) {
    return { variant: 'warning', label: KUBERNETES_STALE_READINGS_LABEL };
  }
  if (hasImpairedKubernetesSource(resource)) {
    return { variant: 'warning', label: 'Degraded' };
  }
  if (ready === true) return { variant: 'success', label: 'Ready' };

  if (status === 'online' || status === 'running' || status === 'healthy') {
    return { variant: 'success', label: 'Ready' };
  }
  if (status === 'degraded' || status === 'warning' || status === 'pending') {
    return { variant: 'warning', label: 'Degraded' };
  }
  return { variant: 'muted', label: 'Unknown' };
}

const replicaIndicator = (
  desired: number | undefined,
  ready: number | undefined,
  readyLabel = 'Ready',
): StatusIndicator => {
  const desiredCount = typeof desired === 'number' ? desired : 0;
  const readyCount = typeof ready === 'number' ? ready : 0;
  if (desiredCount <= 0) return { variant: 'muted', label: 'Scaled to 0' };
  if (readyCount >= desiredCount) return { variant: 'success', label: readyLabel };
  if (readyCount <= 0) return { variant: 'danger', label: `0 / ${desiredCount} ready` };
  return { variant: 'warning', label: `${readyCount} / ${desiredCount} ready` };
};

export function mapKubernetesDeploymentStatus(resource: Resource): StatusIndicator {
  return applyKubernetesSourceFreshness(
    resource,
    replicaIndicator(resource.kubernetes?.desiredReplicas, resource.kubernetes?.readyReplicas),
  );
}

export const mapKubernetesReplicaSetStatus = mapKubernetesDeploymentStatus;
export const mapKubernetesStatefulSetStatus = mapKubernetesDeploymentStatus;

export function mapKubernetesDaemonSetStatus(resource: Resource): StatusIndicator {
  const desired = resource.kubernetes?.desiredNumberScheduled;
  const ready = resource.kubernetes?.numberReady;
  const misscheduled = resource.kubernetes?.numberMisscheduled ?? 0;
  const base = replicaIndicator(desired, ready, 'Scheduled');
  if (base.variant === 'success' && misscheduled > 0) {
    return { variant: 'warning', label: `${misscheduled} misscheduled` };
  }
  return applyKubernetesSourceFreshness(resource, base);
}

export function mapKubernetesJobStatus(resource: Resource): StatusIndicator {
  const failed = resource.kubernetes?.failed ?? 0;
  const succeeded = resource.kubernetes?.succeeded ?? 0;
  const active = resource.kubernetes?.active ?? 0;
  if (failed > 0) return { variant: 'danger', label: `${failed} failed` };
  if (active > 0) return { variant: 'warning', label: `${active} active` };
  if (succeeded > 0) {
    return applyKubernetesSourceFreshness(resource, { variant: 'success', label: 'Succeeded' });
  }
  return applyKubernetesSourceFreshness(resource, { variant: 'muted', label: 'Idle' });
}

export function mapKubernetesCronJobStatus(resource: Resource): StatusIndicator {
  if (resource.kubernetes?.suspend === true) {
    return applyKubernetesSourceFreshness(resource, { variant: 'muted', label: 'Suspended' });
  }
  return applyKubernetesSourceFreshness(resource, { variant: 'success', label: 'Scheduled' });
}

export function mapKubernetesControllerStatus(resource: Resource): StatusIndicator {
  switch (resource.type) {
    case 'k8s-replicaset':
      return mapKubernetesReplicaSetStatus(resource);
    case 'k8s-statefulset':
      return mapKubernetesStatefulSetStatus(resource);
    case 'k8s-daemonset':
      return mapKubernetesDaemonSetStatus(resource);
    case 'k8s-job':
      return mapKubernetesJobStatus(resource);
    case 'k8s-cronjob':
      return mapKubernetesCronJobStatus(resource);
    default:
      return { variant: 'muted', label: 'Unknown' };
  }
}

// Attention-first ordering: rows that need an operator's eye float to the
// top of the table. Tie-broken by display name for stable rendering.
const STATUS_VARIANT_RANK: Record<StatusIndicatorVariant, number> = {
  danger: 0,
  warning: 1,
  info: 2,
  muted: 3,
  success: 4,
};

const compareByStatus = (
  mapper: (resource: Resource) => StatusIndicator,
): ((left: Resource, right: Resource) => number) => {
  return (left, right) => {
    const rankDelta =
      STATUS_VARIANT_RANK[mapper(left).variant] - STATUS_VARIANT_RANK[mapper(right).variant];
    if (rankDelta !== 0) return rankDelta;
    return displayName(left).localeCompare(displayName(right));
  };
};

export const compareKubernetesPods = compareByStatus(mapKubernetesPodStatus);
export const compareKubernetesNodes = compareByStatus(mapKubernetesNodeStatus);
export const compareKubernetesDeployments = compareByStatus(mapKubernetesDeploymentStatus);
export const compareKubernetesControllers = compareByStatus(mapKubernetesControllerStatus);

export type KubernetesIncidentSeverityFilter = 'all' | 'critical' | 'warning' | 'info';

export type KubernetesIncidentRow = {
  id: string;
  resource: Resource;
  resourceId: string;
  resourceName: string;
  resourceType: ResourceType;
  severity: string;
  severityBucket: Exclude<KubernetesIncidentSeverityFilter, 'all'>;
  code: string;
  source: string;
  summary: string;
  label: string;
  category: string;
  startedAt?: string;
  action: string;
  priority: number;
};

export function mapKubernetesIncidentSeverity(
  severity: string | undefined,
): Exclude<KubernetesIncidentSeverityFilter, 'all'> {
  const normalized = asTrimmedString(severity).toLowerCase();
  if (['critical', 'crit', 'fatal', 'error', 'failed', 'failure'].includes(normalized)) {
    return 'critical';
  }
  if (['warning', 'warn', 'alert', 'degraded'].includes(normalized)) return 'warning';
  return 'info';
}

const kubernetesIncidentSeverityRank = (severity: string): number => {
  switch (mapKubernetesIncidentSeverity(severity)) {
    case 'critical':
      return 3;
    case 'warning':
      return 2;
    case 'info':
      return 1;
  }
};

const titleCaseIncidentCode = (value: string): string =>
  value
    .split(/[\s_-]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1).toLowerCase())
    .join(' ');

const kubernetesIncidentLabel = (resource: Resource, incident: ResourceIncident): string => {
  const label = asTrimmedString(resource.incidentLabel);
  if (label) return label;
  const code = asTrimmedString(incident.code);
  return code ? titleCaseIncidentCode(code.replace(/^k8s_/, '')) : 'Kubernetes Alert';
};

const hasKubernetesIncidentSignal = (incident: ResourceIncident): boolean =>
  Boolean(asTrimmedString(incident.code) || asTrimmedString(incident.summary));

const hasKubernetesIncidentRollup = (resource: Resource): boolean =>
  (resource.incidentCount ?? 0) > 0 ||
  Boolean(
    asTrimmedString(resource.incidentCode) ||
    asTrimmedString(resource.incidentSummary) ||
    asTrimmedString(resource.incidentLabel),
  );

const buildKubernetesIncidentRow = (
  resource: Resource,
  incident: ResourceIncident,
  index: number,
): KubernetesIncidentRow => {
  const severity =
    asTrimmedString(incident.severity) || asTrimmedString(resource.incidentSeverity) || 'info';
  const code =
    asTrimmedString(incident.code) || asTrimmedString(resource.incidentCode) || 'k8s_alert';
  const summary =
    asTrimmedString(incident.summary) ||
    asTrimmedString(resource.incidentSummary) ||
    kubernetesIncidentLabel(resource, incident);
  const nativeId = asTrimmedString(incident.nativeId);
  const rowKey = nativeId || code || String(index);
  return {
    id: `${resource.id}:incident:${rowKey}:${index}`,
    resource,
    resourceId: resource.id,
    resourceName: displayName(resource),
    resourceType: resource.type,
    severity,
    severityBucket: mapKubernetesIncidentSeverity(severity),
    code,
    source: asTrimmedString(incident.source) || asTrimmedString(incident.provider) || 'kubernetes',
    summary,
    label: kubernetesIncidentLabel(resource, incident),
    category: asTrimmedString(resource.incidentCategory) || 'kubernetes-health',
    startedAt: incident.startedAt,
    action: asTrimmedString(resource.incidentAction) || 'Investigate in Pulse alerts',
    priority: resource.incidentPriority ?? kubernetesIncidentSeverityRank(severity) * 1000,
  };
};

const buildKubernetesRollupIncidentRow = (resource: Resource): KubernetesIncidentRow => {
  const severity = asTrimmedString(resource.incidentSeverity) || 'info';
  const code = asTrimmedString(resource.incidentCode) || 'k8s_alert';
  const count = resource.incidentCount ?? 0;
  const summary =
    asTrimmedString(resource.incidentSummary) ||
    asTrimmedString(resource.incidentLabel) ||
    `${count || 1} active Kubernetes alert${count === 1 ? '' : 's'}`;
  const incident: ResourceIncident = { code, severity, summary, source: 'kubernetes' };
  return {
    ...buildKubernetesIncidentRow(resource, incident, 0),
    id: `${resource.id}:incident:rollup`,
  };
};

// Walks resource.incidents[] for each row; when a resource carries only
// rollup-level incident fields but no per-incident list, emits a single
// synthesized row. Mirrors buildVmwareIncidentRows / buildTrueNASIncidentRows.
export function buildKubernetesIncidentRows(resources: Resource[]): KubernetesIncidentRow[] {
  const rows: KubernetesIncidentRow[] = [];
  for (const resource of resources) {
    const incidents = (resource.incidents ?? []).filter(hasKubernetesIncidentSignal);
    if (incidents.length > 0) {
      incidents.forEach((incident, index) =>
        rows.push(buildKubernetesIncidentRow(resource, incident, index)),
      );
      continue;
    }
    if (hasKubernetesIncidentRollup(resource)) {
      rows.push(buildKubernetesRollupIncidentRow(resource));
    }
  }
  return rows.sort((a, b) => {
    const severityDelta =
      kubernetesIncidentSeverityRank(b.severity) - kubernetesIncidentSeverityRank(a.severity);
    if (severityDelta !== 0) return severityDelta;
    const priorityDelta = b.priority - a.priority;
    if (priorityDelta !== 0) return priorityDelta;
    return a.resourceName.localeCompare(b.resourceName);
  });
}

const kubernetesIncidentSearchHaystack = (row: KubernetesIncidentRow): string =>
  [
    row.resourceName,
    row.resourceId,
    row.resourceType,
    row.resource.parentName,
    row.resource.platformId,
    row.resource.kubernetes?.clusterName,
    row.resource.kubernetes?.namespace,
    row.resource.kubernetes?.nodeName,
    row.resource.kubernetes?.podName,
    row.resource.kubernetes?.ownerName,
    row.severity,
    row.code,
    row.source,
    row.summary,
    row.label,
    row.category,
    row.action,
    ...(row.resource.tags ?? []),
  ]
    .filter((value): value is string => typeof value === 'string' && value.trim().length > 0)
    .join(' ')
    .toLowerCase();

export function filterKubernetesIncidents(
  incidents: KubernetesIncidentRow[],
  search: string,
  severity: KubernetesIncidentSeverityFilter,
): KubernetesIncidentRow[] {
  const needle = search.trim().toLowerCase();
  return incidents.filter((incident) => {
    if (severity !== 'all' && incident.severityBucket !== severity) return false;
    if (!needle) return true;
    return kubernetesIncidentSearchHaystack(incident).includes(needle);
  });
}

export type KubernetesResourceStatusFilter = 'all' | 'online' | 'degraded' | 'offline';

const ONLINE_RESOURCE_STATUSES = new Set<string>(['online', 'running']);
// The canonical Kubernetes adapter reports every unhealthy-but-alive state
// (cordoned node, not-ready container, pending pod, partially available
// controller) as `warning`, so the degraded bucket must accept it or those
// rows match none of the Healthy / Degraded / Offline filter chips.
const DEGRADED_RESOURCE_STATUSES = new Set<string>(['degraded', 'paused', 'warning']);
const OFFLINE_RESOURCE_STATUSES = new Set<string>(['offline', 'stopped']);

const mapResourceStatusToTriad = (
  status: string | undefined,
): Exclude<KubernetesResourceStatusFilter, 'all'> | 'unknown' => {
  if (!status) return 'unknown';
  if (ONLINE_RESOURCE_STATUSES.has(status)) return 'online';
  if (DEGRADED_RESOURCE_STATUSES.has(status)) return 'degraded';
  if (OFFLINE_RESOURCE_STATUSES.has(status)) return 'offline';
  return 'unknown';
};

// Builds the lowercase search haystack a Kubernetes page table consults when
// filtering rows. The shared platformPage helper carries only generic Resource
// fields; kubernetes.* lookups live here so the cross-platform helper does not
// have to know which platforms exist.
export function kubernetesResourceSearchHaystack(resource: Resource): string {
  const k = resource.kubernetes;
  const ci = resource.canonicalIdentity;
  return [
    resource.id,
    resource.name,
    resource.displayName,
    resource.parentName,
    resource.platformId,
    resource.platformType,
    resource.agent?.hostname,
    resource.identity?.hostname,
    ...(ci ? [ci.displayName, ci.hostname, ci.primaryId, ...(ci.aliases ?? [])] : []),
    ...(k
      ? [
          k.clusterId,
          k.clusterName,
          k.context,
          k.namespace,
          k.podName,
          k.podPhase,
          k.podReason,
          k.podMessage,
          k.ownerKind,
          k.ownerName,
          k.image,
          k.nodeName,
          k.resourceKind,
          k.serviceName,
          k.serviceType,
          k.clusterIp,
          k.storageClass,
          k.phase,
          k.reason,
          k.message,
          k.involvedKind,
          k.involvedName,
          k.eventType,
          k.volumeName,
          k.version,
          k.kubeletVersion,
          k.containerRuntimeVersion,
          k.osImage,
          k.architecture,
          k.provisioner,
          k.addressType,
          k.secretType,
          k.targetKind,
          k.targetName,
          k.server,
          ...(k.externalIps ?? []),
          ...(k.hosts ?? []),
          ...(k.addresses ?? []),
          ...(k.accessModes ?? []),
          ...(k.roles ?? []),
          ...(k.policyTypes ?? []),
          ...(k.metricTypes ?? []),
          ...(k.podContainers?.flatMap((c) => [c.name, c.image, c.state, c.reason]) ?? []),
        ]
      : []),
    ...(resource.tags ?? []),
  ]
    .filter((value): value is string => typeof value === 'string' && value.trim().length > 0)
    .join(' ')
    .toLowerCase();
}

export type KubernetesEventSeverityFilter = 'all' | 'warning' | 'normal';
export type KubernetesServiceCategoryFilter = 'all' | 'service' | 'ingress' | 'endpoint';
export type KubernetesStorageKindFilter = 'all' | 'pvc' | 'pv' | 'storageclass';
export type KubernetesConfigCategoryFilter = 'all' | 'namespace' | 'config' | 'access' | 'policy';

const matchesKubernetesSearch = (
  resource: Resource,
  split: ReturnType<typeof splitSearchExclusions>,
): boolean => {
  if (split.needle.length === 0 && split.excludes.length === 0) return true;
  return matchesSearchTermSplit(kubernetesResourceSearchHaystack(resource), split);
};

export function filterKubernetesEvents(
  resources: Resource[],
  search: string,
  severity: KubernetesEventSeverityFilter,
): Resource[] {
  const split = splitSearchExclusions(search);
  return resources.filter((resource) => {
    if (severity !== 'all') {
      const eventSeverity = mapKubernetesEventSeverity(resource.kubernetes?.eventType);
      const isWarning = eventSeverity.variant === 'warning';
      if (severity === 'warning' && !isWarning) return false;
      if (severity === 'normal' && isWarning) return false;
    }
    return matchesKubernetesSearch(resource, split);
  });
}

export function matchesKubernetesServiceCategory(
  resource: Resource,
  category: KubernetesServiceCategoryFilter,
): boolean {
  if (category === 'all') return true;
  if (category === 'service') return resource.type === 'k8s-service';
  if (category === 'ingress') return resource.type === 'k8s-ingress';
  return resource.type === 'k8s-endpoint-slice';
}

export function filterKubernetesStorageResources(
  resources: Resource[],
  search: string,
  kind: KubernetesStorageKindFilter,
): Resource[] {
  const split = splitSearchExclusions(search);
  return resources.filter((resource) => {
    if (kind === 'pvc' && resource.type !== 'k8s-persistent-volume-claim') return false;
    if (kind === 'pv' && resource.type !== 'k8s-persistent-volume') return false;
    if (kind === 'storageclass' && resource.type !== 'k8s-storage-class') return false;
    return matchesKubernetesSearch(resource, split);
  });
}

const K8S_CONTROLLER_TYPES: readonly ResourceType[] = [
  'k8s-replicaset',
  'k8s-statefulset',
  'k8s-daemonset',
  'k8s-job',
  'k8s-cronjob',
];
const K8S_ACCESS_TYPES: readonly ResourceType[] = [
  'k8s-serviceaccount',
  'k8s-role',
  'k8s-cluster-role',
  'k8s-role-binding',
  'k8s-cluster-role-binding',
];
const K8S_POLICY_TYPES: readonly ResourceType[] = [
  'k8s-network-policy',
  'k8s-pod-disruption-budget',
  'k8s-resource-quota',
  'k8s-limit-range',
];
const K8S_STORAGE_TYPES: readonly ResourceType[] = [
  'k8s-storage-class',
  'k8s-persistent-volume',
  'k8s-persistent-volume-claim',
];
const K8S_NETWORKING_TYPES: readonly ResourceType[] = ['k8s-ingress', 'k8s-endpoint-slice'];
const K8S_CONFIG_OBJECT_TYPES: readonly ResourceType[] = ['k8s-configmap', 'k8s-secret'];

const KUBERNETES_CONTROLLER_TYPES = new Set<ResourceType>(K8S_CONTROLLER_TYPES);
const KUBERNETES_ACCESS_TYPES = new Set<ResourceType>(K8S_ACCESS_TYPES);

export function matchesKubernetesConfigCategory(
  resource: Resource,
  category: KubernetesConfigCategoryFilter,
): boolean {
  if (category === 'all') return true;
  if (category === 'namespace') return resource.type === 'k8s-namespace';
  if (category === 'config') {
    return resource.type === 'k8s-configmap' || resource.type === 'k8s-secret';
  }
  if (category === 'access') return KUBERNETES_ACCESS_TYPES.has(resource.type);
  return K8S_POLICY_TYPES.includes(resource.type);
}

const effectiveKubernetesFilterStatus = (
  resource: Resource,
): Exclude<KubernetesResourceStatusFilter, 'all'> | 'unknown' => {
  const triad = mapResourceStatusToTriad(resource.status);
  if (triad === 'offline') return 'offline';
  if (hasStaleKubernetesReadings(resource) || hasImpairedKubernetesSource(resource)) {
    return 'degraded';
  }
  return triad;
};

export function isKubernetesWorkloadAttention(resource: Resource): boolean {
  const triad = effectiveKubernetesFilterStatus(resource);
  if (triad === 'degraded' || triad === 'offline') return true;
  const indicator =
    resource.type === 'pod'
      ? mapKubernetesPodStatus(resource)
      : resource.type === 'k8s-deployment'
        ? mapKubernetesDeploymentStatus(resource)
        : KUBERNETES_CONTROLLER_TYPES.has(resource.type)
          ? mapKubernetesControllerStatus(resource)
          : null;
  return indicator?.variant === 'warning' || indicator?.variant === 'danger';
}

export function filterKubernetesResources(
  resources: Resource[],
  search: string,
  status: KubernetesResourceStatusFilter,
): Resource[] {
  const split = splitSearchExclusions(search);
  const hasSearch = split.needle.length > 0 || split.excludes.length > 0;
  const result: Resource[] = [];
  for (const resource of resources) {
    if (status !== 'all') {
      const triad = effectiveKubernetesFilterStatus(resource);
      if (triad !== status) continue;
    }
    if (!hasSearch) {
      result.push(resource);
      continue;
    }
    if (matchesSearchTermSplit(kubernetesResourceSearchHaystack(resource), split)) {
      result.push(resource);
    }
  }
  return result;
}

const KUBERNETES_ROUTE_TAB_ALIASES: Record<string, KubernetesPageTabId> = {
  autoscaling: 'workloads',
  config: 'configuration',
  controllers: 'workloads',
  deployments: 'workloads',
  networking: 'services',
  pods: 'workloads',
  policy: 'configuration',
};

export const resolveKubernetesPageTabId = (segment: string | undefined): KubernetesPageTabId => {
  const normalized = asTrimmedString(segment).toLowerCase();
  if (!normalized) return 'overview';
  const direct = KUBERNETES_TAB_SPECS.find((tab) => tab.id === normalized);
  if (direct) return direct.id;
  return KUBERNETES_ROUTE_TAB_ALIASES[normalized] ?? 'overview';
};

const hasKubernetesTabInventory = (
  model: KubernetesPageModel,
  tab: KubernetesPageTabId,
  typeCounts?: Partial<Record<ResourceType, number>>,
): boolean => {
  const hasType = (types: readonly ResourceType[]) =>
    types.some((type) => (typeCounts?.[type] ?? 0) > 0);
  switch (tab) {
    case 'overview':
      return true;
    case 'nodes':
      return model.nodes.length > 0 || hasType(['k8s-node', 'agent']);
    case 'workloads':
      return (
        model.workloads.length > 0 ||
        model.autoscaling.length > 0 ||
        hasType(['pod', 'k8s-deployment', ...K8S_CONTROLLER_TYPES, 'k8s-horizontal-pod-autoscaler'])
      );
    case 'services':
      return (
        model.services.length > 0 ||
        model.serviceNetworking.length > 0 ||
        hasType(['k8s-service', ...K8S_NETWORKING_TYPES])
      );
    case 'storage':
      return model.storage.length > 0 || hasType(K8S_STORAGE_TYPES);
    case 'configuration':
      return (
        model.config.length > 0 ||
        model.policy.length > 0 ||
        hasType([
          'k8s-namespace',
          ...K8S_CONFIG_OBJECT_TYPES,
          ...K8S_ACCESS_TYPES,
          ...K8S_POLICY_TYPES,
        ])
      );
    case 'events':
      return model.events.length > 0 || hasType(['k8s-event']);
  }
};

export const getKubernetesPageTabSpecs = (
  model: KubernetesPageModel,
  typeCounts?: Partial<Record<ResourceType, number>>,
): readonly KubernetesTabSpec[] =>
  KUBERNETES_TAB_SPECS.filter((tab) => hasKubernetesTabInventory(model, tab.id, typeCounts));

const KUBERNETES_RESOURCE_TYPES = new Set<ResourceType>([
  'k8s-cluster',
  'k8s-node',
  'pod',
  'k8s-deployment',
  'k8s-namespace',
  'k8s-service',
  'k8s-horizontal-pod-autoscaler',
  'k8s-event',
  ...K8S_CONTROLLER_TYPES,
  ...K8S_NETWORKING_TYPES,
  ...K8S_STORAGE_TYPES,
  ...K8S_CONFIG_OBJECT_TYPES,
  ...K8S_ACCESS_TYPES,
  ...K8S_POLICY_TYPES,
]);

const isKubernetesPlatform = (resource: Resource): boolean => {
  if (resolveResourcePlatformType(resource) === 'kubernetes') return true;
  return KUBERNETES_RESOURCE_TYPES.has(resource.type);
};

// Kubernetes nodes registered as Pulse Agents are merged onto the linked
// agent row by the backend registry, so the "nodes" bucket must include
// both the standalone k8s-node projections and the agent rows that report a
// kubernetes source.
const isKubernetesNodeRow = (resource: Resource): boolean => {
  if (resource.type === 'k8s-node') return true;
  if (resource.type !== 'agent') return false;
  return resolveResourcePlatformType(resource) === 'kubernetes';
};

export type KubernetesPageModel = {
  resources: Resource[];
  clusters: Resource[];
  nodes: Resource[];
  pods: Resource[];
  deployments: Resource[];
  replicaSets: Resource[];
  namespaces: Resource[];
  services: Resource[];
  statefulSets: Resource[];
  daemonSets: Resource[];
  jobs: Resource[];
  cronJobs: Resource[];
  ingresses: Resource[];
  endpointSlices: Resource[];
  networkPolicies: Resource[];
  persistentVolumes: Resource[];
  persistentVolumeClaims: Resource[];
  storageClasses: Resource[];
  configMaps: Resource[];
  secrets: Resource[];
  serviceAccounts: Resource[];
  roles: Resource[];
  clusterRoles: Resource[];
  roleBindings: Resource[];
  clusterRoleBindings: Resource[];
  resourceQuotas: Resource[];
  limitRanges: Resource[];
  podDisruptionBudgets: Resource[];
  horizontalPodAutoscalers: Resource[];
  events: Resource[];
  workloads: Resource[];
  storage: Resource[];
  serviceNetworking: Resource[];
  config: Resource[];
  policy: Resource[];
  autoscaling: Resource[];
  incidents: KubernetesIncidentRow[];
};

export type KubernetesOverviewPosture = {
  nodeAttention: number;
  podAttention: number;
  deploymentAttention: number;
  criticalResources: number;
  criticalIncidents: number;
  warningIncidents: number;
  attentionResources: number;
  attentionSignals: number;
};

const countKubernetesAttention = (
  resources: readonly Resource[],
  mapper: (resource: Resource) => StatusIndicator,
): number =>
  resources.filter((resource) => {
    const variant = mapper(resource).variant;
    return variant === 'danger' || variant === 'warning';
  }).length;

const countKubernetesDanger = (
  resources: readonly Resource[],
  mapper: (resource: Resource) => StatusIndicator,
): number => resources.filter((resource) => mapper(resource).variant === 'danger').length;

// Attention order shared by the overview's "Nodes needing attention" list and
// the nodes table's default sort: 0 for a NotReady node or one with an
// unacknowledged critical alert, 1 for a cordoned or degraded node or one with
// another unacknowledged alert, 2 otherwise. The alerts are the ones the
// node's row tint and drawer use, so a Ready node at 100% memory ranks with
// the NotReady ones instead of below every degraded node.
export function getKubernetesNodeAttentionRank(
  node: Resource,
  activeAlerts: Record<string, Alert>,
  alertsEnabled: boolean,
): 0 | 1 | 2 {
  const variant = mapKubernetesNodeStatus(node).variant;
  const statusRank = variant === 'danger' ? 0 : variant === 'warning' ? 1 : 2;
  const alertStyles = getUnifiedResourceAlertStyles(node, activeAlerts, alertsEnabled);
  const alertRank = !alertStyles.hasUnacknowledgedAlert
    ? 2
    : alertStyles.severity === 'critical'
      ? 0
      : 1;
  return Math.min(statusRank, alertRank) as 0 | 1 | 2;
}

// Nodes the overview should name before any inventory, in attention order.
// "2/3 nodes" on the cluster row says something is wrong but not which node;
// this list answers that.
export function getKubernetesNodesNeedingAttention(
  model: KubernetesPageModel,
  activeAlerts: Record<string, Alert> = {},
  alertsEnabled = false,
): Resource[] {
  return model.nodes
    .map((node) => ({
      node,
      rank: getKubernetesNodeAttentionRank(node, activeAlerts, alertsEnabled),
    }))
    .filter((entry) => entry.rank < 2)
    .sort((a, b) => a.rank - b.rank || compareKubernetesNodes(a.node, b.node))
    .map((entry) => entry.node);
}

export function buildKubernetesOverviewPosture(
  model: KubernetesPageModel,
): KubernetesOverviewPosture {
  const nodeAttention = countKubernetesAttention(model.nodes, mapKubernetesNodeStatus);
  const podAttention = countKubernetesAttention(model.pods, mapKubernetesPodStatus);
  const deploymentAttention = countKubernetesAttention(
    model.deployments,
    mapKubernetesDeploymentStatus,
  );
  const criticalResources =
    countKubernetesDanger(model.nodes, mapKubernetesNodeStatus) +
    countKubernetesDanger(model.pods, mapKubernetesPodStatus) +
    countKubernetesDanger(model.deployments, mapKubernetesDeploymentStatus);
  const criticalIncidents = model.incidents.filter(
    (incident) => incident.severityBucket === 'critical',
  ).length;
  const warningIncidents = model.incidents.filter(
    (incident) => incident.severityBucket === 'warning',
  ).length;

  return {
    nodeAttention,
    podAttention,
    deploymentAttention,
    criticalResources,
    criticalIncidents,
    warningIncidents,
    attentionResources: nodeAttention + podAttention + deploymentAttention,
    attentionSignals: criticalIncidents + warningIncidents,
  };
}

export type KubernetesClusterChildCount = {
  total: number;
  // Rows whose status indicator is danger or warning. Healthy-for-display is
  // total - attention, so muted states (Unknown, Scaled to 0) never read as
  // degraded.
  attention: number;
};

export type KubernetesClusterChildCounts = {
  nodes: KubernetesClusterChildCount;
  pods: KubernetesClusterChildCount;
  deployments: KubernetesClusterChildCount;
};

export const emptyKubernetesClusterChildCounts = (): KubernetesClusterChildCounts => ({
  nodes: { total: 0, attention: 0 },
  pods: { total: 0, attention: 0 },
  deployments: { total: 0, attention: 0 },
});

const tallyKubernetesChild = (
  count: KubernetesClusterChildCount,
  indicator: StatusIndicator,
): void => {
  count.total += 1;
  if (indicator.variant === 'danger' || indicator.variant === 'warning') {
    count.attention += 1;
  }
};

const matchClusterFor = (
  resource: Resource,
  clusters: readonly Resource[],
): Resource | undefined => {
  const clusterId =
    asTrimmedString(resource.kubernetes?.clusterId) ||
    asTrimmedString(resource.kubernetes?.clusterName);
  if (!clusterId) return undefined;
  for (const cluster of clusters) {
    const k = cluster.kubernetes;
    if (!k) continue;
    if (
      asTrimmedString(k.clusterId) === clusterId ||
      asTrimmedString(k.clusterName) === clusterId
    ) {
      return cluster;
    }
  }
  return undefined;
};

// Walks the resource snapshot once and rolls per-cluster Nodes / Pods /
// Deployments into a Map keyed by cluster row id, carrying both the total
// and how many rows need attention so the Overview can render the v5-style
// healthy/total fractions. The Overview clusters table calls this instead
// of computing counts inline, so the same rollup is testable and reusable
// by other consumers (incident rollups, future Overview "active issues"
// cards).
export function buildKubernetesClusterChildCounts(
  resources: readonly Resource[],
  clusters: readonly Resource[],
): Map<string, KubernetesClusterChildCounts> {
  const counts = new Map<string, KubernetesClusterChildCounts>();
  for (const cluster of clusters) {
    counts.set(cluster.id, emptyKubernetesClusterChildCounts());
  }
  if (clusters.length === 0) return counts;

  for (const resource of resources) {
    const cluster = matchClusterFor(resource, clusters);
    if (!cluster) continue;
    const bucket = counts.get(cluster.id);
    if (!bucket) continue;
    if (resource.type === 'k8s-node') {
      tallyKubernetesChild(bucket.nodes, mapKubernetesNodeStatus(resource));
    } else if (resource.type === 'agent' && resource.sources?.includes('kubernetes')) {
      tallyKubernetesChild(bucket.nodes, mapKubernetesNodeStatus(resource));
    } else if (resource.type === 'pod') {
      tallyKubernetesChild(bucket.pods, mapKubernetesPodStatus(resource));
    } else if (resource.type === 'k8s-deployment') {
      tallyKubernetesChild(bucket.deployments, mapKubernetesDeploymentStatus(resource));
    }
  }
  return counts;
}

export function buildKubernetesPageModel(resources: Resource[]): KubernetesPageModel {
  const k8sResources = resources.filter(isKubernetesPlatform);
  const byType = new Map<ResourceType, Resource[]>();
  const nodes: Resource[] = [];
  for (const resource of k8sResources) {
    const bucket = byType.get(resource.type);
    if (bucket) bucket.push(resource);
    else byType.set(resource.type, [resource]);
    if (isKubernetesNodeRow(resource)) nodes.push(resource);
  }
  const of = (type: ResourceType): Resource[] => byType.get(type) ?? [];
  const allOf = (types: readonly ResourceType[]): Resource[] => types.flatMap(of);
  nodes.sort(compareKubernetesNodes);
  const pods = [...of('pod')].sort(compareKubernetesPods);
  const deployments = [...of('k8s-deployment')].sort(compareKubernetesDeployments);
  const sortedControllers = allOf(K8S_CONTROLLER_TYPES).sort(compareKubernetesControllers);
  const horizontalPodAutoscalers = of('k8s-horizontal-pod-autoscaler');

  return {
    resources: k8sResources,
    clusters: of('k8s-cluster'),
    nodes,
    pods,
    deployments,
    replicaSets: of('k8s-replicaset'),
    namespaces: of('k8s-namespace'),
    services: of('k8s-service'),
    statefulSets: of('k8s-statefulset'),
    daemonSets: of('k8s-daemonset'),
    jobs: of('k8s-job'),
    cronJobs: of('k8s-cronjob'),
    ingresses: of('k8s-ingress'),
    endpointSlices: of('k8s-endpoint-slice'),
    networkPolicies: of('k8s-network-policy'),
    persistentVolumes: of('k8s-persistent-volume'),
    persistentVolumeClaims: of('k8s-persistent-volume-claim'),
    storageClasses: of('k8s-storage-class'),
    configMaps: of('k8s-configmap'),
    secrets: of('k8s-secret'),
    serviceAccounts: of('k8s-serviceaccount'),
    roles: of('k8s-role'),
    clusterRoles: of('k8s-cluster-role'),
    roleBindings: of('k8s-role-binding'),
    clusterRoleBindings: of('k8s-cluster-role-binding'),
    resourceQuotas: of('k8s-resource-quota'),
    limitRanges: of('k8s-limit-range'),
    podDisruptionBudgets: of('k8s-pod-disruption-budget'),
    horizontalPodAutoscalers,
    events: [...of('k8s-event')].sort(compareKubernetesEvents),
    workloads: [...deployments, ...sortedControllers, ...pods],
    storage: allOf(K8S_STORAGE_TYPES),
    serviceNetworking: allOf(K8S_NETWORKING_TYPES),
    config: [...of('k8s-namespace'), ...allOf(K8S_CONFIG_OBJECT_TYPES), ...allOf(K8S_ACCESS_TYPES)],
    policy: allOf(K8S_POLICY_TYPES),
    autoscaling: [...horizontalPodAutoscalers],
    incidents: buildKubernetesIncidentRows(k8sResources),
  };
}
