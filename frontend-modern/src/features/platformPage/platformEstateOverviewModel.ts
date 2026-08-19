import type { Resource, ResourceType } from '@/types/resource';

export const PLATFORM_ESTATE_OVERVIEW_STORAGE_KEY = 'platformEstateOverviewVisible';

export type PlatformEstateOverviewPlatform =
  'proxmox' | 'docker' | 'kubernetes' | 'truenas' | 'vmware' | 'standalone';

export type PlatformEstateMetric = {
  id: string;
  label: string;
  value: number | string;
};

export type PlatformEstateSpotlightTone = 'danger' | 'warning';

export type PlatformEstateSpotlight = {
  id: string;
  href: string;
  label: string;
  meta: string;
  resourceId: string;
  tone: PlatformEstateSpotlightTone;
};

export type PlatformEstateOverviewModel = {
  metrics: readonly PlatformEstateMetric[];
  spotlights: readonly PlatformEstateSpotlight[];
};

const ATTENTION_STATUSES = new Set(['offline', 'degraded', 'warning']);
const DANGER_TOKENS = new Set(['critical', 'danger', 'fatal', 'offline', 'failed', 'error']);
const WARNING_TOKENS = new Set(['warning', 'warn', 'degraded', 'attention']);

const normalize = (value: unknown): string =>
  typeof value === 'string' ? value.trim().toLowerCase() : '';

const displayName = (resource: Resource): string =>
  resource.displayName?.trim() || resource.name?.trim() || resource.id;

const countTypes = (resources: readonly Resource[], types: readonly ResourceType[]): number => {
  const accepted = new Set<ResourceType>(types);
  return resources.filter((resource) => accepted.has(resource.type)).length;
};

const resourceHasDangerEvidence = (resource: Resource): boolean => {
  if (normalize(resource.status) === 'offline') return true;
  if (DANGER_TOKENS.has(normalize(resource.incidentSeverity))) return true;
  if (resource.incidents?.some((incident) => DANGER_TOKENS.has(normalize(incident.severity)))) {
    return true;
  }
  return Boolean(resource.alerts?.some((alert) => DANGER_TOKENS.has(normalize(alert.level))));
};

const resourceHasWarningEvidence = (resource: Resource): boolean => {
  if (ATTENTION_STATUSES.has(normalize(resource.status))) return true;
  if (WARNING_TOKENS.has(normalize(resource.incidentSeverity))) return true;
  if (resource.incidents?.some((incident) => WARNING_TOKENS.has(normalize(incident.severity)))) {
    return true;
  }
  if (resource.alerts?.some((alert) => WARNING_TOKENS.has(normalize(alert.level)))) return true;
  return resource.type === 'storage' && Number(resource.disk?.current) >= 85;
};

export const isPlatformEstateAttentionResource = (resource: Resource): boolean =>
  resourceHasDangerEvidence(resource) || resourceHasWarningEvidence(resource);

const attentionCount = (resources: readonly Resource[]): number =>
  resources.filter(isPlatformEstateAttentionResource).length;

const metric = (id: string, label: string, value: number | string): PlatformEstateMetric => ({
  id,
  label,
  value,
});

export const formatPlatformEstateMetricValue = (value: number | string): string =>
  typeof value === 'number' ? value.toLocaleString() : value;

const buildProxmoxMetrics = (resources: readonly Resource[]): PlatformEstateMetric[] => {
  const nodes = resources.filter((resource) => resource.type === 'agent');
  const clusters = new Set(
    nodes
      .map((resource) => resource.proxmox?.clusterName?.trim() || resource.clusterId?.trim() || '')
      .filter(Boolean),
  );
  const standalone = nodes.filter(
    (resource) => !resource.proxmox?.clusterName?.trim() && !resource.clusterId?.trim(),
  ).length;

  return [
    metric('nodes', 'Proxmox nodes', nodes.length),
    metric(
      'workloads',
      'VMs and containers',
      countTypes(resources, ['vm', 'system-container', 'oci-container']),
    ),
    metric('topology', 'Clusters + standalone', `${clusters.size} + ${standalone}`),
    metric('attention', 'Need attention', attentionCount(resources)),
  ];
};

const buildDockerMetrics = (resources: readonly Resource[]): PlatformEstateMetric[] => [
  metric(
    'hosts',
    'Runtime hosts',
    resources.filter(
      (resource) =>
        resource.type === 'docker-host' || (resource.type === 'agent' && resource.docker),
    ).length,
  ),
  metric('containers', 'Containers', countTypes(resources, ['app-container'])),
  metric('images', 'Images', countTypes(resources, ['docker-image'])),
  metric('attention', 'Need attention', attentionCount(resources)),
];

const buildKubernetesMetrics = (resources: readonly Resource[]): PlatformEstateMetric[] => [
  metric('clusters', 'Clusters', countTypes(resources, ['k8s-cluster'])),
  metric(
    'nodes',
    'Nodes',
    resources.filter(
      (resource) =>
        resource.type === 'k8s-node' || (resource.type === 'agent' && resource.kubernetes),
    ).length,
  ),
  metric('pods', 'Pods', countTypes(resources, ['pod'])),
  metric('attention', 'Need attention', attentionCount(resources)),
];

const buildTrueNASMetrics = (resources: readonly Resource[]): PlatformEstateMetric[] => [
  metric('systems', 'TrueNAS systems', countTypes(resources, ['agent'])),
  metric(
    'pools',
    'Storage pools',
    resources.filter(
      (resource) => resource.type === 'pool' || resource.storage?.topology === 'pool',
    ).length,
  ),
  metric('workloads', 'Apps and VMs', countTypes(resources, ['app-container', 'vm'])),
  metric('attention', 'Need attention', attentionCount(resources)),
];

const buildVmwareMetrics = (resources: readonly Resource[]): PlatformEstateMetric[] => {
  const connections = new Set(
    resources.map((resource) => resource.vmware?.connectionId?.trim()).filter(Boolean),
  );
  return [
    metric('connections', 'vCenter connections', connections.size),
    metric('hosts', 'ESXi hosts', countTypes(resources, ['agent'])),
    metric('workloads', 'Virtual machines', countTypes(resources, ['vm'])),
    metric('attention', 'Need attention', attentionCount(resources)),
  ];
};

const buildStandaloneMetrics = (resources: readonly Resource[]): PlatformEstateMetric[] => {
  const machines = resources.filter((resource) => resource.type === 'agent');
  return [
    metric('machines', 'Machines', machines.length),
    metric('checks', 'Availability checks', countTypes(resources, ['network-endpoint'])),
    metric(
      'healthy',
      'Reporting normally',
      machines.filter((resource) => ['online', 'running'].includes(normalize(resource.status)))
        .length,
    ),
    metric('attention', 'Need attention', attentionCount(resources)),
  ];
};

export function buildPlatformEstateMetrics(
  platform: PlatformEstateOverviewPlatform,
  resources: readonly Resource[],
): PlatformEstateMetric[] {
  switch (platform) {
    case 'proxmox':
      return buildProxmoxMetrics(resources);
    case 'docker':
      return buildDockerMetrics(resources);
    case 'kubernetes':
      return buildKubernetesMetrics(resources);
    case 'truenas':
      return buildTrueNASMetrics(resources);
    case 'vmware':
      return buildVmwareMetrics(resources);
    case 'standalone':
      return buildStandaloneMetrics(resources);
  }
}

const resourceTypeLabel = (type: ResourceType): string => {
  const labels: Partial<Record<ResourceType, string>> = {
    agent: 'System',
    'docker-host': 'Runtime host',
    'app-container': 'Container',
    'docker-image': 'Image',
    'k8s-cluster': 'Cluster',
    'k8s-node': 'Node',
    pod: 'Pod',
    vm: 'Virtual machine',
    'system-container': 'Container',
    'oci-container': 'OCI container',
    storage: 'Storage',
    physical_disk: 'Physical disk',
    'network-endpoint': 'Availability check',
  };
  return labels[type] ?? type.replace(/-/g, ' ');
};

const spotlightRoute = (platform: PlatformEstateOverviewPlatform, resource: Resource): string => {
  switch (platform) {
    case 'proxmox':
      if (resource.type === 'pbs') return '/proxmox/backups';
      if (resource.type === 'ceph') return '/proxmox/ceph';
      if (resource.type === 'pmg') return '/proxmox/mail';
      if (['storage', 'physical_disk'].includes(resource.type)) return '/proxmox/storage';
      return '/proxmox/overview';
    case 'docker':
      if (resource.type === 'docker-image') return '/docker/images';
      if (resource.type === 'docker-volume') return '/docker/storage';
      if (resource.type === 'docker-network') return '/docker/networks';
      if (
        [
          'docker-service',
          'docker-task',
          'docker-swarm-node',
          'docker-secret',
          'docker-config',
        ].includes(resource.type)
      ) {
        return '/docker/swarm';
      }
      return '/docker/overview';
    case 'kubernetes':
      if (resource.type === 'k8s-node' || (resource.type === 'agent' && resource.kubernetes)) {
        return '/kubernetes/nodes';
      }
      if (
        ['pod', 'k8s-deployment', 'k8s-replicaset', 'k8s-statefulset', 'k8s-daemonset'].includes(
          resource.type,
        )
      ) {
        return '/kubernetes/workloads';
      }
      if (
        ['k8s-persistent-volume', 'k8s-persistent-volume-claim', 'k8s-storage-class'].includes(
          resource.type,
        )
      ) {
        return '/kubernetes/storage';
      }
      if (resource.type === 'k8s-event') return '/kubernetes/events';
      return '/kubernetes/overview';
    case 'truenas':
      if (['storage', 'pool', 'dataset', 'physical_disk'].includes(resource.type)) {
        return '/truenas/storage';
      }
      if (resource.type === 'app-container') return '/truenas/apps';
      if (resource.type === 'vm') return '/truenas/vms';
      if (resource.type === 'network-share') return '/truenas/shares';
      return '/truenas/overview';
    case 'vmware':
      if (resource.type === 'storage') return '/vmware/storage';
      if (resource.type === 'network') return '/vmware/networks';
      if (resource.incidentCount || resource.incidents?.length) return '/vmware/health';
      return '/vmware/overview';
    case 'standalone':
      return resource.type === 'network-endpoint' ? '/machines/availability' : '/machines';
  }
};

const spotlightEvidence = (
  resource: Resource,
): { label: string; meta: string; tone: PlatformEstateSpotlightTone } => {
  const name = displayName(resource);
  const incident = resource.incidents?.find((item) => item.summary?.trim());
  const alert = resource.alerts?.find((item) => item.message?.trim());
  const tone = resourceHasDangerEvidence(resource) ? 'danger' : 'warning';
  if (incident) {
    return {
      label: incident.summary.trim(),
      meta: `${name} · ${resourceTypeLabel(resource.type)}`,
      tone,
    };
  }
  if (alert) {
    return {
      label: alert.message.trim(),
      meta: `${name} · ${resourceTypeLabel(resource.type)}`,
      tone,
    };
  }
  if (resource.type === 'storage' && Number(resource.disk?.current) >= 85) {
    return {
      label: `${name} storage pressure`,
      meta: `${Math.round(Number(resource.disk?.current))}% used`,
      tone,
    };
  }
  const status = normalize(resource.status);
  return {
    label: status === 'offline' ? `${name} offline` : `${name} needs attention`,
    meta: `${resourceTypeLabel(resource.type)} · ${status || 'attention'}`,
    tone,
  };
};

export function buildPlatformOperationalSpotlights(
  platform: PlatformEstateOverviewPlatform,
  resources: readonly Resource[],
  limit = 3,
): PlatformEstateSpotlight[] {
  const boundedLimit = Math.max(0, Math.trunc(limit));
  if (boundedLimit === 0) return [];

  const compare = (left: PlatformEstateSpotlight, right: PlatformEstateSpotlight): number => {
    if (left.tone !== right.tone) return left.tone === 'danger' ? -1 : 1;
    return left.label.localeCompare(right.label, undefined, { numeric: true });
  };
  const selected: PlatformEstateSpotlight[] = [];

  for (const resource of resources) {
    if (!isPlatformEstateAttentionResource(resource)) continue;
    const evidence = spotlightEvidence(resource);
    const spotlight = {
      id: `${platform}:${resource.id}`,
      href: spotlightRoute(platform, resource),
      label: evidence.label,
      meta: evidence.meta,
      resourceId: resource.id,
      tone: evidence.tone,
    } satisfies PlatformEstateSpotlight;
    const insertionIndex = selected.findIndex((candidate) => compare(spotlight, candidate) < 0);
    selected.splice(insertionIndex < 0 ? selected.length : insertionIndex, 0, spotlight);
    if (selected.length > boundedLimit) selected.pop();
  }

  return selected;
}

export function buildPlatformEstateOverviewModel(
  platform: PlatformEstateOverviewPlatform,
  resources: readonly Resource[],
): PlatformEstateOverviewModel {
  return {
    metrics: buildPlatformEstateMetrics(platform, resources),
    spotlights: buildPlatformOperationalSpotlights(platform, resources),
  };
}

export const deserializePlatformEstateOverviewVisibility = (raw: string): boolean =>
  raw !== 'false';
