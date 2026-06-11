import type { Resource, ResourceMetric, ResourceType } from '@/types/resource';
import { formatProxmoxVersion } from '@/utils/proxmoxVersion';
import { resourceMatchesSearch } from '@/utils/resourceSearchMatch';

export type ProxmoxPageTabId = 'overview' | 'storage' | 'replication' | 'backups' | 'ceph' | 'mail';

export type ProxmoxPlatformScope = 'proxmox-pve' | 'proxmox-pbs' | 'proxmox-pmg';

export type ProxmoxTabSpec = {
  id: ProxmoxPageTabId;
  label: string;
  path: string;
};

export type ProxmoxClusterGroup = {
  id: string;
  label: string;
  nodes: Resource[];
  guests: Resource[];
  storage: Resource[];
};

export type ProxmoxPageSummary = {
  clusterCount: number;
  nodeCount: number;
  guestCount: number;
  runningGuestCount: number;
  stoppedGuestCount: number;
  storageCount: number;
  pbsCount: number;
  pmgCount: number;
  cephCount: number;
  alertCount: number;
};

export type ProxmoxPageModel = {
  resources: Resource[];
  pveNodes: Resource[];
  guests: Resource[];
  storage: Resource[];
  pbs: Resource[];
  pmg: Resource[];
  ceph: Resource[];
  physicalDisks: Resource[];
  clusterGroups: ProxmoxClusterGroup[];
  summary: ProxmoxPageSummary;
};

const PROXMOX_RESOURCE_TYPES = new Set<ResourceType>([
  'agent',
  'vm',
  'system-container',
  'oci-container',
  'storage',
  'datastore',
  'pool',
  'dataset',
  'physical_disk',
  'ceph',
  'pbs',
  'pmg',
]);

const PROXMOX_WORKLOAD_TYPES = new Set<ResourceType>(['vm', 'system-container', 'oci-container']);

const PROXMOX_STORAGE_TYPES = new Set<ResourceType>([
  'storage',
  'datastore',
  'pool',
  'dataset',
  'physical_disk',
]);

export const PROXMOX_TAB_SPECS: ProxmoxTabSpec[] = [
  { id: 'overview', label: 'Overview', path: '/proxmox/overview' },
  { id: 'storage', label: 'Storage', path: '/proxmox/storage' },
  { id: 'replication', label: 'Replication', path: '/proxmox/replication' },
  { id: 'backups', label: 'Backups', path: '/proxmox/backups' },
  { id: 'ceph', label: 'Ceph', path: '/proxmox/ceph' },
  { id: 'mail', label: 'Mail Gateway', path: '/proxmox/mail' },
];

const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === 'object' && value !== null;

const getPlatformData = (resource: Resource): Record<string, unknown> => {
  if (!isRecord(resource.platformData)) return {};
  return resource.platformData;
};

const getPlatformSources = (resource: Resource): string[] => {
  const sources = getPlatformData(resource).sources;
  if (!Array.isArray(sources)) return resource.sources ?? [];
  return sources.filter((source): source is string => typeof source === 'string');
};

export function resolveProxmoxPlatformScope(resource: Resource): ProxmoxPlatformScope | null {
  if (resource.type === 'pbs') return 'proxmox-pbs';
  if (resource.type === 'pmg') return 'proxmox-pmg';
  if (resource.platformType === 'proxmox-pve') return 'proxmox-pve';
  if (resource.platformType === 'proxmox-pbs') return 'proxmox-pbs';
  if (resource.platformType === 'proxmox-pmg') return 'proxmox-pmg';
  if (resource.proxmox || isRecord(getPlatformData(resource).proxmox)) return 'proxmox-pve';

  const sources = getPlatformSources(resource).map((source) => source.toLowerCase());
  if (sources.some((source) => source === 'pbs' || source === 'proxmox-pbs')) {
    return 'proxmox-pbs';
  }
  if (sources.some((source) => source === 'pmg' || source === 'proxmox-pmg')) {
    return 'proxmox-pmg';
  }
  if (
    sources.some((source) => source === 'pve' || source === 'proxmox' || source === 'proxmox-pve')
  ) {
    return 'proxmox-pve';
  }

  return null;
}

export function isProxmoxResource(resource: Resource): boolean {
  return (
    PROXMOX_RESOURCE_TYPES.has(resource.type) && resolveProxmoxPlatformScope(resource) !== null
  );
}

export function isProxmoxWorkload(resource: Resource): boolean {
  return (
    PROXMOX_WORKLOAD_TYPES.has(resource.type) &&
    resolveProxmoxPlatformScope(resource) === 'proxmox-pve'
  );
}

export function isProxmoxStorageResource(resource: Resource): boolean {
  const scope = resolveProxmoxPlatformScope(resource);
  if (resource.type === 'ceph' || resource.storage?.isCeph === true) return false;
  return (
    PROXMOX_STORAGE_TYPES.has(resource.type) && (scope === 'proxmox-pve' || scope === 'proxmox-pbs')
  );
}

export function getMetricPercent(metric?: ResourceMetric): number {
  if (!metric) return 0;
  if (typeof metric.current === 'number' && Number.isFinite(metric.current)) {
    return Math.max(0, Math.min(100, metric.current));
  }
  if (metric.total && metric.used) {
    return Math.max(0, Math.min(100, (metric.used / metric.total) * 100));
  }
  return 0;
}

export function getResourceClusterLabel(resource: Resource): string {
  return (
    resource.proxmox?.clusterName ||
    resource.identity?.clusterName ||
    resource.clusterId ||
    'Standalone'
  );
}

export function getResourceNodeName(resource: Resource): string {
  return (
    resource.proxmox?.nodeName ||
    resource.proxmox?.node ||
    resource.parentName ||
    resource.identity?.hostname ||
    resource.name
  );
}

// filterProxmoxNodesForSearch narrows the nodes table to match the shared
// workload search box. Because that box is a VM/LXC filter, filtering nodes by
// the raw term alone collapses the table to its empty state whenever the term
// matches a guest but not a node name — which misreads as "no Proxmox nodes"
// while the matching guest is listed right below. Keep a node when it matches
// the term directly OR when it hosts a guest that matches, so a guest search
// still shows that guest's host node for context.
export function filterProxmoxNodesForSearch(
  nodes: Resource[],
  guests: Resource[],
  term: string,
): Resource[] {
  if (!term.trim()) return nodes;
  const matchingGuestNodeNames = new Set(
    guests
      .filter((guest) => resourceMatchesSearch(guest, term))
      .map((guest) => getResourceNodeName(guest))
      .filter(Boolean),
  );
  return nodes.filter(
    (node) =>
      resourceMatchesSearch(node, term) || matchingGuestNodeNames.has(getResourceNodeName(node)),
  );
}

export function getResourceVmid(resource: Resource): string {
  const vmid = resource.proxmox?.vmid;
  if (typeof vmid === 'number' && Number.isFinite(vmid)) {
    return String(vmid);
  }
  const platformProxmox = getPlatformData(resource).proxmox;
  if (isRecord(platformProxmox) && typeof platformProxmox.vmid === 'number') {
    return String(platformProxmox.vmid);
  }
  return '';
}

export function getResourceLastBackup(resource: Resource): string | number | null {
  const platformProxmox = getPlatformData(resource).proxmox;
  if (!isRecord(platformProxmox)) return null;
  const value = platformProxmox.lastBackup;
  return typeof value === 'string' || typeof value === 'number' ? value : null;
}

const hasBackupSignal = (resource: Resource): boolean => {
  if (getResourceLastBackup(resource) !== null) return true;
  const haystack = [
    resource.id,
    resource.name,
    resource.displayName,
    ...(resource.tags ?? []),
    ...(resource.recentChanges ?? []).flatMap((change) => [
      change.id,
      change.kind,
      change.sourceAdapter,
      change.reason,
      ...(change.relatedResources ?? []),
    ]),
  ]
    .filter((value): value is string => typeof value === 'string')
    .join(' ')
    .toLowerCase();

  return (
    haystack.includes('backup') || haystack.includes('snapshot') || haystack.includes('vzdump')
  );
};

export function getResourceVersion(resource: Resource): string {
  const pveVersion = formatProxmoxVersion(resource.proxmox?.pveVersion);
  if (pveVersion) return pveVersion;
  const platformProxmox = getPlatformData(resource).proxmox;
  if (isRecord(platformProxmox) && typeof platformProxmox.pveVersion === 'string') {
    const version = formatProxmoxVersion(platformProxmox.pveVersion);
    if (version) return version;
  }
  if (resource.pbs?.version) return resource.pbs.version;
  const platformPbs = getPlatformData(resource).pbs;
  if (isRecord(platformPbs) && typeof platformPbs.version === 'string') return platformPbs.version;
  const platformPmg = getPlatformData(resource).pmg;
  if (isRecord(platformPmg) && typeof platformPmg.version === 'string') return platformPmg.version;
  if (resource.agent?.osName?.toLowerCase().includes('proxmox') && resource.agent.osVersion) {
    return formatProxmoxVersion(resource.agent.osVersion) || resource.agent.osVersion;
  }
  return '';
}

export function buildProxmoxPageModel(resources: Resource[]): ProxmoxPageModel {
  const proxmoxResources = resources.filter(isProxmoxResource);
  const pveNodes = proxmoxResources.filter(
    (resource) =>
      resource.type === 'agent' && resolveProxmoxPlatformScope(resource) === 'proxmox-pve',
  );
  const guests = proxmoxResources.filter(isProxmoxWorkload);
  const storage = proxmoxResources.filter(isProxmoxStorageResource);
  const pbs = proxmoxResources.filter(
    (resource) => resolveProxmoxPlatformScope(resource) === 'proxmox-pbs',
  );
  const pmg = proxmoxResources.filter(
    (resource) => resolveProxmoxPlatformScope(resource) === 'proxmox-pmg',
  );
  const ceph = proxmoxResources.filter(
    (resource) =>
      resource.type === 'ceph' ||
      resource.storage?.isCeph === true ||
      isRecord(getPlatformData(resource).ceph),
  );
  const physicalDisks = proxmoxResources.filter((resource) => resource.type === 'physical_disk');

  const groupsById = new Map<string, ProxmoxClusterGroup>();
  const ensureGroup = (label: string): ProxmoxClusterGroup => {
    const id = label.toLowerCase() === 'standalone' ? '__standalone__' : label;
    const existing = groupsById.get(id);
    if (existing) return existing;
    const created: ProxmoxClusterGroup = {
      id,
      label,
      nodes: [],
      guests: [],
      storage: [],
    };
    groupsById.set(id, created);
    return created;
  };

  pveNodes.forEach((resource) =>
    ensureGroup(getResourceClusterLabel(resource)).nodes.push(resource),
  );

  guests.forEach((resource) => {
    const nodeName = getResourceNodeName(resource);
    const owningNode = pveNodes.find((node) => getResourceNodeName(node) === nodeName);
    ensureGroup(
      owningNode ? getResourceClusterLabel(owningNode) : getResourceClusterLabel(resource),
    ).guests.push(resource);
  });

  storage.forEach((resource) => {
    const nodeName = getResourceNodeName(resource);
    const owningNode = pveNodes.find((node) => getResourceNodeName(node) === nodeName);
    ensureGroup(
      owningNode ? getResourceClusterLabel(owningNode) : getResourceClusterLabel(resource),
    ).storage.push(resource);
  });

  const clusterGroups = Array.from(groupsById.values()).sort((a, b) => {
    if (a.id === '__standalone__') return 1;
    if (b.id === '__standalone__') return -1;
    return a.label.localeCompare(b.label);
  });

  const alertCount = proxmoxResources.reduce(
    (total, resource) => total + (resource.alerts?.length ?? 0) + (resource.incidentCount ?? 0),
    0,
  );

  return {
    resources: proxmoxResources,
    pveNodes,
    guests,
    storage,
    pbs,
    pmg,
    ceph,
    physicalDisks,
    clusterGroups,
    summary: {
      clusterCount: clusterGroups.filter((group) => group.id !== '__standalone__').length,
      nodeCount: pveNodes.length,
      guestCount: guests.length,
      runningGuestCount: guests.filter(
        (resource) => resource.status === 'online' || resource.status === 'running',
      ).length,
      stoppedGuestCount: guests.filter(
        (resource) => resource.status === 'offline' || resource.status === 'stopped',
      ).length,
      storageCount: storage.length,
      pbsCount: pbs.length,
      pmgCount: pmg.length,
      cephCount: ceph.length,
      alertCount,
    },
  };
}

// Replication deliberately bypasses the unified-resource pipeline (it is
// projected straight from Monitor.ReplicationJobsSnapshot via
// /api/replication/jobs), so the tab is gated on the fetched job count
// rather than on anything derivable from `model`.
export function buildVisibleProxmoxTabSpecs(
  model: ProxmoxPageModel,
  replicationJobCount: number,
): ProxmoxTabSpec[] {
  const visible = new Set<ProxmoxPageTabId>(['overview']);

  if (model.storage.length > 0 || model.physicalDisks.length > 0) {
    visible.add('storage');
  }
  if (replicationJobCount > 0) {
    visible.add('replication');
  }
  if (model.resources.some(hasBackupSignal) || model.pbs.length > 0) {
    visible.add('backups');
  }
  if (model.ceph.length > 0) {
    visible.add('ceph');
  }
  if (model.pmg.length > 0) {
    visible.add('mail');
  }

  return PROXMOX_TAB_SPECS.filter((tab) => visible.has(tab.id));
}
