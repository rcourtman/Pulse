import type { PBSDatastore, Storage } from '@/types/api';
import type { Resource } from '@/types/resource';
import { STORAGE_DATA_ORIGIN_PRECEDENCE } from './models';
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
  (value || '')
    .trim()
    .toLowerCase();

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
  const location = normalizeIdentityPart(record.location?.label) || normalizeIdentityPart(record.refs?.platformEntityId);
  const name = normalizeIdentityPart(record.name) || normalizeIdentityPart(record.id);
  const category = normalizeIdentityPart(record.category || 'other');

  return [platform, location || 'unknown-location', name || 'unknown-name', category].join('|');
};

const resolvePlatformFamily = (platform: StorageBackupPlatform): PlatformFamily => {
  const value = String(platform).toLowerCase();
  if (value.includes('kubernetes') || value.includes('docker')) return 'container';
  if (value.includes('cloud') || value === 'aws' || value === 'azure' || value === 'gcp') return 'cloud';
  if (value.includes('proxmox') || value.includes('vmware') || value.includes('hyperv')) {
    return 'virtualization';
  }
  if (value.includes('generic')) return 'generic';
  return 'onprem';
};

const fromSource = (
  platform: StorageBackupPlatform,
  adapterId: string,
  origin: SourceDescriptor['origin'],
): SourceDescriptor => ({
  platform,
  family: resolvePlatformFamily(platform),
  adapterId,
  origin,
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

  if (normalized === 'warning' || normalized === 'warn' || normalized === 'degraded' || normalized === 'health_warn') {
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

  if (normalized === 'offline' || normalized === 'stopped' || normalized === 'down' || normalized === 'unavailable') {
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

  if (normalized.includes('offline') || normalized.includes('stopped') || normalized.includes('down')) {
    return 'offline';
  }

  if (normalized.includes('healthy') || normalized.includes('online') || normalized.includes('available')) {
    return 'healthy';
  }

  if (normalized === 'unknown') return 'unknown';
  return undefined;
};

const normalizeResourceHealth = (status: string | undefined, tags: string[] | undefined): NormalizedHealth =>
  normalizeHealthValue(extractHealthTag(tags)) || normalizeHealthValue(status) || 'unknown';

const normalizeLegacyStorageHealth = (status: string | undefined): NormalizedHealth => {
  if ((status || '').toLowerCase() === 'available') return 'healthy';
  if ((status || '').toLowerCase() === 'degraded') return 'warning';
  if ((status || '').toLowerCase() === 'offline') return 'offline';
  return 'unknown';
};

const categoryFromStorageType = (type: string | undefined): StorageCategory => {
  const value = (type || '').toLowerCase();
  if (!value) return 'other';
  if (value.includes('pbs')) return 'backup-repository';
  if (value.includes('zfs') || value.includes('lvm') || value.includes('ceph') || value.includes('pool')) {
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
    ? candidate.contentTypes.filter((item): item is string => typeof item === 'string' && item.trim().length > 0)
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
  const legacyContent = (platformData.content as string | undefined)?.trim();
  return legacyContent || fallback;
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
  const storageType = storageMeta?.type || (platformData.type as string | undefined) || (isDatastore ? 'pbs' : resourceType);
  const content = resolveStorageContent(storageMeta, platformData, isDatastore ? 'backup' : '');
  const shared =
    typeof storageMeta?.shared === 'boolean'
      ? storageMeta.shared
      : typeof platformData.shared === 'boolean'
        ? platformData.shared
        : undefined;
  const locationLabel = isDatastore
    ? ((platformData.pbsInstanceName as string | undefined) || resource.parentId || resource.platformId || 'Unknown')
    : ((platformData.node as string | undefined) || resource.parentId || resource.platformId || 'Unknown');
  const usagePercent = asNumberOrNull(resource.disk?.current);
  const totalBytes = asNumberOrNull(resource.disk?.total);
  const usedBytes = asNumberOrNull(resource.disk?.used);
  const freeBytes = asNumberOrNull(resource.disk?.free);

  return {
    id: resource.id,
    name: resource.name,
    category: isDatastore
      ? 'backup-repository'
      : storageMeta?.isCeph || storageMeta?.isZfs
        ? 'pool'
        : categoryFromStorageType(storageType),
    health: normalizeResourceHealth(resource.status, resource.tags),
    location: {
      label: locationLabel,
      scope: isDatastore ? 'cluster' : 'node',
    },
    capacity: capacity(totalBytes, usedBytes, freeBytes, usagePercent),
    capabilities: isDatastore
      ? dedupe(['capacity', 'health', 'backup-repository', 'deduplication', 'namespaces'])
      : capabilitiesForStorage(storageType, storageMeta),
    source: fromSource(platform, adapterId, 'resource'),
    observedAt:
      typeof resource.lastSeen === 'number' && Number.isFinite(resource.lastSeen)
        ? resource.lastSeen
        : Date.now(),
    refs: {
      resourceId: resource.id,
      platformEntityId: resource.platformId,
    },
    details: {
      type: storageType,
      status: resource.status,
      parentId: resource.parentId,
      node: platformData.node,
      content,
      contentTypes: storageMeta?.contentTypes,
      shared,
      isCeph: storageMeta?.isCeph,
      isZfs: storageMeta?.isZfs,
      zfsPool: platformData.zfsPool,
    },
  };
};

const mapLegacyStorageRecord = (storage: Storage, adapterId: string): StorageRecord => {
  const usagePercent = Number.isFinite(storage.usage) ? storage.usage : null;
  const totalBytes = Number.isFinite(storage.total) ? storage.total : null;
  const usedBytes = Number.isFinite(storage.used) ? storage.used : null;
  const freeBytes = Number.isFinite(storage.free) ? storage.free : null;
  const type = storage.type || '';

  return {
    id: storage.id || `${storage.instance}-${storage.node}-${storage.name}`,
    name: storage.name || 'Storage',
    category: categoryFromStorageType(type),
    health: normalizeLegacyStorageHealth(storage.status),
    location: {
      label: storage.node || storage.instance || 'Unknown',
      scope: storage.shared ? 'cluster' : 'node',
    },
    capacity: capacity(totalBytes, usedBytes, freeBytes, usagePercent),
    capabilities: capabilitiesForStorage(type),
    source: fromSource('proxmox-pve', adapterId, 'legacy'),
    observedAt: Date.now(),
    refs: {
      legacyStorageId: storage.id,
      platformEntityId: storage.instance,
    },
    details: {
      type,
      content: storage.content,
      shared: storage.shared,
      nodes: storage.nodes || [],
      node: storage.node,
      status: storage.status,
      zfsPool: storage.zfsPool,
    },
  };
};

const mapLegacyPBSDatastore = (
  instance: { id: string; name: string; datastores: PBSDatastore[] },
  datastore: PBSDatastore,
  adapterId: string,
): StorageRecord => {
  const usagePercent = Number.isFinite(datastore.usage) ? datastore.usage : null;
  const totalBytes = Number.isFinite(datastore.total) ? datastore.total : null;
  const usedBytes = Number.isFinite(datastore.used) ? datastore.used : null;
  const freeBytes = Number.isFinite(datastore.free) ? datastore.free : null;
  const datastoreID = `pbs-${instance.id}-${datastore.name || 'datastore'}`;

  return {
    id: datastoreID,
    name: datastore.name || 'PBS Datastore',
    category: 'backup-repository',
    health: normalizeLegacyStorageHealth(datastore.status),
    location: {
      label: instance.name || instance.id || 'PBS',
      scope: 'cluster',
    },
    capacity: capacity(totalBytes, usedBytes, freeBytes, usagePercent),
    capabilities: ['capacity', 'health', 'backup-repository', 'deduplication', 'namespaces'],
    source: fromSource('proxmox-pbs', adapterId, 'legacy'),
    observedAt: Date.now(),
    refs: {
      legacyStorageId: datastoreID,
      platformEntityId: instance.id,
    },
    details: {
      deduplicationFactor: datastore.deduplicationFactor,
      namespaceCount: datastore.namespaces?.length || 0,
      error: datastore.error,
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

const legacyStorageAdapter: StorageAdapter = {
  id: 'legacy-storage',
  supports: (ctx) => (ctx.state.storage || []).length > 0,
  build: (ctx) => (ctx.state.storage || []).map((storage) => mapLegacyStorageRecord(storage, 'legacy-storage')),
};

const legacyPbsDatastoreAdapter: StorageAdapter = {
  id: 'legacy-pbs-datastore',
  supports: (ctx) => (ctx.state.pbs || []).some((instance) => (instance.datastores || []).length > 0),
  build: (ctx) =>
    (ctx.state.pbs || []).flatMap((instance) =>
      (instance.datastores || []).map((datastore) =>
        mapLegacyPBSDatastore(
          { id: instance.id, name: instance.name, datastores: instance.datastores || [] },
          datastore,
          'legacy-pbs-datastore',
        ),
      ),
    ),
};

export const DEFAULT_STORAGE_ADAPTERS: StorageAdapter[] = [
  resourceStorageAdapter,
  legacyStorageAdapter,
  legacyPbsDatastoreAdapter,
];

const mergeStorageRecords = (current: StorageRecord, incoming: StorageRecord): StorageRecord => {
  const currentRank = STORAGE_DATA_ORIGIN_PRECEDENCE[current.source.origin];
  const incomingRank = STORAGE_DATA_ORIGIN_PRECEDENCE[incoming.source.origin];
  const preferred = incomingRank > currentRank ? incoming : current;
  const secondary = preferred === current ? incoming : current;

  return {
    ...secondary,
    ...preferred,
    capabilities: dedupe([...(current.capabilities || []), ...(incoming.capabilities || [])]),
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
