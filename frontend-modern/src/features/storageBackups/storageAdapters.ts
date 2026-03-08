import type { Resource } from '@/types/resource';
import type {
  CapacitySnapshot,
  NormalizedHealth,
  PlatformFamily,
  SourceDescriptor,
  StorageBackupPlatform,
  StorageCapability,
  StorageCategory,
  StorageRecord,
  StorageAdapter,
  StorageAdapterContext,
} from './models';

const asNumberOrNull = (value: unknown): number | null => {
  if (typeof value === 'number' && Number.isFinite(value)) return value;
  return null;
};

const dedupe = <T>(values: T[]): T[] => Array.from(new Set(values));
const normalizeIdentityPart = (value: string | undefined | null): string =>
  (value || '').trim().toLowerCase();

const getStringArray = (value: unknown): string[] =>
  Array.isArray(value)
    ? value.filter((item): item is string => typeof item === 'string' && item.trim().length > 0)
    : [];

const titleize = (value: string | undefined | null): string =>
  (value || '')
    .split(/[\s_-]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ');

type ResourceStorageMeta = {
  type?: string;
  content?: string;
  contentTypes?: string[];
  shared?: boolean;
  isCeph?: boolean;
  isZfs?: boolean;
};

type ResourceWithStorageMeta = Resource & {
  storage?: unknown;
};

const canonicalStorageIdentityKey = (record: StorageRecord): string => {
  const platform = normalizeIdentityPart(String(record.source.platform || 'generic'));
  const location =
    normalizeIdentityPart(record.location?.label) ||
    normalizeIdentityPart(record.refs?.platformEntityId);
  const name = normalizeIdentityPart(record.name) || normalizeIdentityPart(record.id);
  const category = normalizeIdentityPart(record.category || 'other');

  return [platform, location || 'unknown-location', name || 'unknown-name', category].join('|');
};

const resolvePlatformFamily = (platform: StorageBackupPlatform): PlatformFamily => {
  const value = String(platform).toLowerCase();
  if (value.includes('kubernetes') || value.includes('docker')) return 'container';
  if (value.includes('cloud') || value === 'aws' || value === 'azure' || value === 'gcp')
    return 'cloud';
  if (value.includes('proxmox') || value.includes('vmware') || value.includes('hyperv')) {
    return 'virtualization';
  }
  if (value.includes('generic')) return 'generic';
  return 'onprem';
};

const fromSource = (platform: StorageBackupPlatform, adapterId: string): SourceDescriptor => ({
  platform,
  family: resolvePlatformFamily(platform),
  adapterId,
  origin: 'resource',
});

const capacity = (
  totalBytes: number | null,
  usedBytes: number | null,
  freeBytes: number | null,
  usagePercent: number | null,
): CapacitySnapshot => ({
  totalBytes,
  usedBytes,
  freeBytes,
  usagePercent,
});

const extractHealthTag = (tags: string[] | undefined): string | undefined => {
  if (!Array.isArray(tags)) return undefined;
  const healthTag = tags
    .map((tag) => tag.trim())
    .filter((tag) => tag.toLowerCase().startsWith('health:'))
    .at(-1);
  if (!healthTag) return undefined;
  return healthTag.slice('health:'.length).trim();
};

const normalizeHealthValue = (value: string | undefined): NormalizedHealth | undefined => {
  const normalized = (value || '').trim().toLowerCase();
  if (!normalized) return undefined;

  if (
    normalized === 'online' ||
    normalized === 'running' ||
    normalized === 'available' ||
    normalized === 'healthy' ||
    normalized === 'ok' ||
    normalized === 'optimal'
  ) {
    return 'healthy';
  }

  if (
    normalized === 'warning' ||
    normalized === 'warn' ||
    normalized === 'degraded' ||
    normalized === 'health_warn'
  ) {
    return 'warning';
  }

  if (
    normalized === 'critical' ||
    normalized === 'faulted' ||
    normalized === 'failed' ||
    normalized === 'error' ||
    normalized === 'unhealthy' ||
    normalized === 'health_crit' ||
    normalized === 'health_err'
  ) {
    return 'critical';
  }

  if (
    normalized === 'offline' ||
    normalized === 'stopped' ||
    normalized === 'down' ||
    normalized === 'unavailable'
  ) {
    return 'offline';
  }

  if (
    normalized.includes('fault') ||
    normalized.includes('fail') ||
    normalized.includes('critical') ||
    normalized.includes('error') ||
    normalized.includes('health_err') ||
    normalized.includes('health_crit') ||
    normalized.includes('unhealthy')
  ) {
    return 'critical';
  }

  if (normalized.includes('degraded') || normalized.includes('warn')) {
    return 'warning';
  }

  if (
    normalized.includes('offline') ||
    normalized.includes('stopped') ||
    normalized.includes('down')
  ) {
    return 'offline';
  }

  if (
    normalized.includes('healthy') ||
    normalized.includes('online') ||
    normalized.includes('available')
  ) {
    return 'healthy';
  }

  if (normalized === 'unknown') return 'unknown';
  return undefined;
};

const normalizeResourceHealth = (
  status: string | undefined,
  tags: string[] | undefined,
  incidentSeverity?: string,
): NormalizedHealth =>
  normalizeHealthValue(incidentSeverity) ||
  normalizeHealthValue(extractHealthTag(tags)) || normalizeHealthValue(status) || 'unknown';

const platformLabelForResource = (resource: Resource, storagePlatform?: string): string => {
  const normalizedPlatform = (storagePlatform || '').trim().toLowerCase();
  switch (normalizedPlatform || resource.platformType) {
    case 'proxmox':
    case 'proxmox-pve':
      return 'PVE';
    case 'pbs':
    case 'proxmox-pbs':
      return 'PBS';
    case 'pmg':
    case 'proxmox-pmg':
      return 'PMG';
    case 'truenas':
      return 'TrueNAS';
    case 'unraid':
      return 'Unraid';
    case 'agent':
      return 'Agent';
    case 'kubernetes':
      return 'Kubernetes';
    default:
      return titleize(storagePlatform) || titleize(resource.platformType) || 'Unknown';
  }
};

const topologyLabelForResource = (
  resource: Resource,
  storageType: string,
  topology?: string,
): string => {
  const normalized = (topology || '').trim().toLowerCase();
  if (normalized) {
    return titleize(normalized);
  }
  if (resource.type === 'datastore') return 'Backup Target';
  switch ((storageType || '').trim().toLowerCase()) {
    case 'zfspool':
    case 'zfs-pool':
    case 'pool':
      return 'Pool';
    case 'zfs-dataset':
    case 'dataset':
      return 'Dataset';
    case 'dir':
    case 'filesystem':
      return 'Filesystem';
    case 'pbs':
      return 'Backup Target';
    case 'rbd':
    case 'cephfs':
      return 'Cluster Storage';
    default:
      return titleize(storageType) || titleize(resource.type) || 'Storage';
  }
};

const issueLabelForResource = (resource: Resource): string => {
  if (resource.incidentLabel?.trim()) return resource.incidentLabel.trim();
  if (resource.storage?.postureSummary?.trim()) return resource.storage.postureSummary.trim();
  if (resource.storage?.riskSummary?.trim()) return resource.storage.riskSummary.trim();
  if (resource.pbs?.postureSummary?.trim()) return resource.pbs.postureSummary.trim();
  return 'Healthy';
};

const issueSummaryForResource = (resource: Resource): string => {
  if (resource.incidentSummary?.trim()) return resource.incidentSummary.trim();
  if (resource.storage?.riskSummary?.trim()) return resource.storage.riskSummary.trim();
  if (resource.storage?.postureSummary?.trim()) return resource.storage.postureSummary.trim();
  if (resource.pbs?.postureSummary?.trim()) return resource.pbs.postureSummary.trim();
  return '';
};

const impactSummaryForResource = (resource: Resource): string => {
  if (resource.incidentImpactSummary?.trim()) return resource.incidentImpactSummary.trim();
  if (resource.storage?.consumerImpactSummary?.trim()) return resource.storage.consumerImpactSummary.trim();
  if (resource.pbs?.protectedWorkloadSummary?.trim()) return resource.pbs.protectedWorkloadSummary.trim();
  if (resource.pbs?.affectedDatastoreSummary?.trim()) return resource.pbs.affectedDatastoreSummary.trim();
  return 'No dependent resources';
};

const actionSummaryForResource = (resource: Resource): string => {
  if (resource.incidentAction?.trim()) return resource.incidentAction.trim();
  if (resource.storage?.rebuildInProgress) return resource.storage.rebuildSummary || 'Monitor rebuild progress';
  if (resource.storage?.protectionReduced) {
    return resource.storage.protectionSummary || 'Restore redundancy';
  }
  return 'Monitor';
};

const protectionLabelForResource = (resource: Resource): string => {
  if (resource.storage?.rebuildInProgress) {
    return resource.storage.rebuildSummary || 'Rebuild In Progress';
  }
  if (resource.storage?.protectionReduced) {
    return resource.storage.protectionSummary || 'Protection Reduced';
  }
  if (resource.incidentCategory === 'recoverability') {
    return resource.incidentLabel || 'Backup Risk';
  }
  if (resource.storage?.protection?.trim()) {
    return titleize(resource.storage.protection.trim());
  }
  if (resource.type === 'datastore' || resource.type === 'pbs') {
    return 'Protected';
  }
  return 'Healthy';
};

const categoryFromStorageType = (type: string | undefined): StorageCategory => {
  const value = (type || '').toLowerCase();
  if (!value) return 'other';
  if (value.includes('pbs')) return 'backup-repository';
  if (
    value.includes('zfs') ||
    value.includes('lvm') ||
    value.includes('ceph') ||
    value.includes('pool')
  ) {
    return 'pool';
  }
  if (value.includes('dataset')) return 'dataset';
  if (value.includes('nfs') || value.includes('cifs') || value.includes('smb')) return 'share';
  if (value.includes('dir') || value.includes('filesystem')) return 'filesystem';
  return 'other';
};

const normalizeStorageMeta = (value: unknown): ResourceStorageMeta | null => {
  if (!value || typeof value !== 'object') return null;
  const candidate = value as Record<string, unknown>;
  const contentTypes = Array.isArray(candidate.contentTypes)
    ? candidate.contentTypes.filter(
        (item): item is string => typeof item === 'string' && item.trim().length > 0,
      )
    : undefined;

  return {
    type: typeof candidate.type === 'string' ? candidate.type : undefined,
    content: typeof candidate.content === 'string' ? candidate.content : undefined,
    contentTypes,
    shared: typeof candidate.shared === 'boolean' ? candidate.shared : undefined,
    isCeph: typeof candidate.isCeph === 'boolean' ? candidate.isCeph : undefined,
    isZfs: typeof candidate.isZfs === 'boolean' ? candidate.isZfs : undefined,
  };
};

const readResourceStorageMeta = (
  resource: Resource,
  platformData: Record<string, unknown>,
): ResourceStorageMeta | undefined => {
  const directMeta = normalizeStorageMeta((resource as ResourceWithStorageMeta).storage);
  if (directMeta) return directMeta;
  const nestedMeta = normalizeStorageMeta(platformData.storage);
  return nestedMeta || undefined;
};

const resolveStorageContent = (
  storageMeta: ResourceStorageMeta | undefined,
  platformData: Record<string, unknown>,
  fallback: string,
): string => {
  const directContent = (storageMeta?.content || '').trim();
  if (directContent) return directContent;
  if (storageMeta?.contentTypes?.length) return storageMeta.contentTypes.join(',');
  const platformContent = (platformData.content as string | undefined)?.trim();
  return platformContent || fallback;
};

const capabilitiesForStorage = (
  type: string | undefined,
  storageMeta?: ResourceStorageMeta,
): StorageCapability[] => {
  const value = (type || '').toLowerCase();
  const caps: StorageCapability[] = ['capacity', 'health'];
  if (value.includes('pbs')) {
    caps.push('backup-repository', 'deduplication', 'namespaces');
  }
  if (storageMeta?.isZfs || value.includes('zfs')) {
    caps.push('snapshots', 'compression');
  }
  if (storageMeta?.isCeph || value.includes('ceph')) {
    caps.push('replication', 'multi-node');
  }
  return dedupe(caps);
};

const mapResourceStorageRecord = (resource: Resource, adapterId: string): StorageRecord => {
  const platformData = (resource.platformData as Record<string, unknown> | undefined) || {};
  const storageMeta = readResourceStorageMeta(resource, platformData);
  const resourceType = (resource.type || '').toLowerCase();
  const platform = (resource.platformType || 'generic') as StorageBackupPlatform;
  const isDatastore = resourceType === 'datastore';
  const storageType =
    storageMeta?.type ||
    (platformData.type as string | undefined) ||
    (isDatastore ? 'pbs' : resourceType);
  const content = resolveStorageContent(storageMeta, platformData, isDatastore ? 'backup' : '');
  const shared =
    typeof storageMeta?.shared === 'boolean'
      ? storageMeta.shared
      : typeof platformData.shared === 'boolean'
        ? platformData.shared
        : undefined;
  const proxmoxNode =
    resource.proxmox?.node ||
    ((platformData.proxmox as Record<string, unknown> | undefined)?.nodeName as string | undefined);
  const storageNodes = getStringArray(
    (resource.storage as Record<string, unknown> | undefined)?.nodes,
  );
  const nodeHints = dedupe(
    [
      resource.parentName,
      proxmoxNode,
      platformData.node as string | undefined,
      ...storageNodes,
      resource.parentId,
      resource.platformId,
    ].filter((value): value is string => typeof value === 'string' && value.trim().length > 0),
  );
  const locationLabel = isDatastore
    ? resource.parentName ||
      (platformData.pbsInstanceName as string | undefined) ||
      resource.parentId ||
      resource.platformId ||
      'Unknown'
    : resource.parentName ||
      proxmoxNode ||
      (platformData.node as string | undefined) ||
      storageNodes[0] ||
      resource.parentId ||
      resource.platformId ||
      'Unknown';
  const usagePercent = asNumberOrNull(resource.disk?.current);
  const totalBytes = asNumberOrNull(resource.disk?.total);
  const usedBytes = asNumberOrNull(resource.disk?.used);
  const freeBytes = asNumberOrNull(resource.disk?.free);
  const hostLabel = locationLabel;
  const platformLabel = platformLabelForResource(resource, resource.storage?.platform);
  const issueLabel = issueLabelForResource(resource);
  const issueSummary = issueSummaryForResource(resource);
  const impactSummary = impactSummaryForResource(resource);
  const actionSummary = actionSummaryForResource(resource);
  const protectionLabel = protectionLabelForResource(resource);
  const topologyLabel = topologyLabelForResource(resource, storageType, resource.storage?.topology);

  return {
    id: resource.id,
    name: resource.name,
    category: isDatastore
      ? 'backup-repository'
      : storageMeta?.isCeph || storageMeta?.isZfs
        ? 'pool'
        : categoryFromStorageType(storageType),
    health: normalizeResourceHealth(resource.status, resource.tags, resource.incidentSeverity),
    statusLabel: resource.status,
    hostLabel,
    platformLabel,
    platformKey: resource.storage?.platform || resource.platformType,
    topologyLabel,
    protectionLabel,
    protectionReduced: resource.storage?.protectionReduced,
    rebuildInProgress: resource.storage?.rebuildInProgress,
    incidentCategory: resource.incidentCategory,
    incidentSeverity: resource.incidentSeverity,
    incidentPriority: resource.incidentPriority || 0,
    issueLabel,
    issueSummary,
    actionSummary,
    impactSummary,
    consumerCount: resource.storage?.consumerCount || 0,
    protectedWorkloadCount: resource.pbs?.protectedWorkloadCount || 0,
    affectedDatastoreCount: resource.pbs?.affectedDatastoreCount || 0,
    location: {
      label: locationLabel,
      scope: isDatastore ? 'cluster' : 'node',
    },
    capacity: capacity(totalBytes, usedBytes, freeBytes, usagePercent),
    capabilities: isDatastore
      ? dedupe(['capacity', 'health', 'backup-repository', 'deduplication', 'namespaces'])
      : capabilitiesForStorage(storageType, storageMeta),
    source: fromSource(platform, adapterId),
    observedAt:
      typeof resource.lastSeen === 'number' && Number.isFinite(resource.lastSeen)
        ? resource.lastSeen
        : Date.now(),
    refs: {
      resourceId: resource.metricsTarget?.resourceId || resource.id,
      platformEntityId: resource.platformId,
    },
    details: {
      type: storageType,
      status: resource.status,
      parentId: resource.parentId,
      parentName: resource.parentName,
      node: proxmoxNode || (platformData.node as string | undefined) || storageNodes[0],
      nodeHints,
      hostLabel,
      platformLabel,
      topologyLabel,
      issueLabel,
      issueSummary,
      impactSummary,
      actionSummary,
      incidentCategory: resource.incidentCategory,
      incidentSeverity: resource.incidentSeverity,
      incidentPriority: resource.incidentPriority,
      protectionLabel,
      protectionReduced: resource.storage?.protectionReduced,
      rebuildInProgress: resource.storage?.rebuildInProgress,
      content,
      contentTypes: storageMeta?.contentTypes,
      shared,
      isCeph: storageMeta?.isCeph,
      isZfs: storageMeta?.isZfs,
      zfsPool: platformData.zfsPool,
    },
  };
};

const resourceStorageAdapter: StorageAdapter = {
  id: 'resource-storage',
  supports: (ctx) => Array.isArray(ctx.resources) && ctx.resources.length > 0,
  build: (ctx) =>
    (ctx.resources || [])
      .filter((resource) => resource.type === 'storage' || resource.type === 'datastore')
      .map((resource) => mapResourceStorageRecord(resource, 'resource-storage')),
};

export const DEFAULT_STORAGE_ADAPTERS: StorageAdapter[] = [resourceStorageAdapter];

const mergeStorageRecords = (current: StorageRecord, incoming: StorageRecord): StorageRecord => {
  const preferred = (current.incidentPriority || 0) >= (incoming.incidentPriority || 0) ? current : incoming;
  const secondary = preferred === current ? incoming : current;

  return {
    ...secondary,
    ...preferred,
    capabilities: dedupe([...(current.capabilities || []), ...(incoming.capabilities || [])]),
    health:
      preferred.health === 'unknown' && secondary.health !== 'unknown'
        ? secondary.health
        : preferred.health,
    statusLabel: preferred.statusLabel || secondary.statusLabel,
    hostLabel: preferred.hostLabel || secondary.hostLabel,
    platformLabel: preferred.platformLabel || secondary.platformLabel,
    platformKey: preferred.platformKey || secondary.platformKey,
    topologyLabel: preferred.topologyLabel || secondary.topologyLabel,
    protectionLabel:
      preferred.protectionReduced || preferred.rebuildInProgress || preferred.incidentPriority
        ? preferred.protectionLabel || secondary.protectionLabel
        : secondary.protectionLabel || preferred.protectionLabel,
    protectionReduced: preferred.protectionReduced || secondary.protectionReduced,
    rebuildInProgress: preferred.rebuildInProgress || secondary.rebuildInProgress,
    issueLabel:
      (preferred.incidentPriority || 0) > 0
        ? preferred.issueLabel || secondary.issueLabel
        : secondary.issueLabel || preferred.issueLabel,
    issueSummary:
      (preferred.incidentPriority || 0) > 0
        ? preferred.issueSummary || secondary.issueSummary
        : secondary.issueSummary || preferred.issueSummary,
    actionSummary:
      (preferred.incidentPriority || 0) > 0
        ? preferred.actionSummary || secondary.actionSummary
        : secondary.actionSummary || preferred.actionSummary,
    impactSummary:
      secondary.impactSummary &&
      secondary.impactSummary !== 'No dependent resources' &&
      (preferred.impactSummary === 'No dependent resources' || !preferred.impactSummary)
        ? secondary.impactSummary
        : preferred.impactSummary || secondary.impactSummary,
    consumerCount: Math.max(current.consumerCount || 0, incoming.consumerCount || 0),
    protectedWorkloadCount: Math.max(
      current.protectedWorkloadCount || 0,
      incoming.protectedWorkloadCount || 0,
    ),
    affectedDatastoreCount: Math.max(
      current.affectedDatastoreCount || 0,
      incoming.affectedDatastoreCount || 0,
    ),
    details: {
      ...(secondary.details || {}),
      ...(preferred.details || {}),
    },
  };
};

export const buildStorageRecords = (
  ctx: StorageAdapterContext,
  adapters: StorageAdapter[] = DEFAULT_STORAGE_ADAPTERS,
): StorageRecord[] => {
  const map = new Map<string, StorageRecord>();

  for (const adapter of adapters) {
    if (!adapter.supports(ctx)) continue;
    const records = adapter.build(ctx);
    for (const record of records) {
      const key = canonicalStorageIdentityKey(record);
      const existing = map.get(key);
      if (!existing) {
        map.set(key, record);
        continue;
      }
      map.set(key, mergeStorageRecords(existing, record));
    }
  }

  return Array.from(map.values());
};
