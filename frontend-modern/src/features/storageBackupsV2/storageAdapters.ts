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
  StorageRecordV2,
  StorageV2Adapter,
  StorageV2AdapterContext,
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

const canonicalStorageIdentityKey = (record: StorageRecordV2): string => {
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

const normalizeResourceHealth = (status: string | undefined): NormalizedHealth => {
  switch ((status || '').toLowerCase()) {
    case 'online':
    case 'running':
      return 'healthy';
    case 'degraded':
      return 'warning';
    case 'offline':
    case 'stopped':
      return 'offline';
    default:
      return 'unknown';
  }
};

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

const capabilitiesForStorageType = (type: string | undefined): StorageCapability[] => {
  const value = (type || '').toLowerCase();
  const caps: StorageCapability[] = ['capacity', 'health'];
  if (value.includes('pbs')) {
    caps.push('backup-repository', 'deduplication', 'namespaces');
  }
  if (value.includes('zfs')) {
    caps.push('snapshots', 'compression');
  }
  if (value.includes('ceph')) {
    caps.push('replication', 'multi-node');
  }
  return dedupe(caps);
};

const mapResourceStorageRecord = (resource: Resource, adapterId: string): StorageRecordV2 => {
  const platformData = (resource.platformData as Record<string, unknown> | undefined) || {};
  const resourceType = (resource.type || '').toLowerCase();
  const platform = (resource.platformType || 'generic') as StorageBackupPlatform;
  const isDatastore = resourceType === 'datastore';
  const storageType = (platformData.type as string | undefined) || (isDatastore ? 'pbs' : resourceType);
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
    category: isDatastore ? 'backup-repository' : categoryFromStorageType(storageType),
    health: normalizeResourceHealth(resource.status),
    location: {
      label: locationLabel,
      scope: isDatastore ? 'cluster' : 'node',
    },
    capacity: capacity(totalBytes, usedBytes, freeBytes, usagePercent),
    capabilities: isDatastore
      ? dedupe(['capacity', 'health', 'backup-repository', 'deduplication', 'namespaces'])
      : capabilitiesForStorageType(storageType),
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
      content: platformData.content,
      shared: platformData.shared,
      zfsPool: platformData.zfsPool,
    },
  };
};

const mapLegacyStorageRecord = (storage: Storage, adapterId: string): StorageRecordV2 => {
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
    capabilities: capabilitiesForStorageType(type),
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
): StorageRecordV2 => {
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

const resourceStorageAdapter: StorageV2Adapter = {
  id: 'resource-storage',
  supports: (ctx) => Array.isArray(ctx.resources) && ctx.resources.length > 0,
  build: (ctx) =>
    (ctx.resources || [])
      .filter((resource) => resource.type === 'storage' || resource.type === 'datastore')
      .map((resource) => mapResourceStorageRecord(resource, 'resource-storage')),
};

const legacyStorageAdapter: StorageV2Adapter = {
  id: 'legacy-storage',
  supports: (ctx) => (ctx.state.storage || []).length > 0,
  build: (ctx) => (ctx.state.storage || []).map((storage) => mapLegacyStorageRecord(storage, 'legacy-storage')),
};

const legacyPbsDatastoreAdapter: StorageV2Adapter = {
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

export const DEFAULT_STORAGE_V2_ADAPTERS: StorageV2Adapter[] = [
  resourceStorageAdapter,
  legacyStorageAdapter,
  legacyPbsDatastoreAdapter,
];

const mergeStorageRecords = (current: StorageRecordV2, incoming: StorageRecordV2): StorageRecordV2 => {
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

export const buildStorageRecordsV2 = (
  ctx: StorageV2AdapterContext,
  adapters: StorageV2Adapter[] = DEFAULT_STORAGE_V2_ADAPTERS,
): StorageRecordV2[] => {
  const map = new Map<string, StorageRecordV2>();

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
