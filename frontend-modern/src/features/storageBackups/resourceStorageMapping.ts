import type { Resource } from '@/types/resource';
import type { ZFSPool } from '@/types/api';
import type { StorageCapability, StorageCategory } from './models';

export type ResourceStorageMeta = {
  type?: string;
  platform?: string;
  topology?: string;
  content?: string;
  contentTypes?: string[];
  shared?: boolean;
  path?: string;
  protection?: string;
  arrayState?: string;
  syncAction?: string;
  syncProgress?: number;
  numProtected?: number;
  numDisabled?: number;
  numInvalid?: number;
  numMissing?: number;
  isCeph?: boolean;
  isZfs?: boolean;
  zfsPool?: ZFSPool;
};

type ResourceWithStorageMeta = Resource & {
  storage?: unknown;
};

const dedupe = <T>(values: T[]): T[] => Array.from(new Set(values));

export type StorageClassificationContext = {
  resourceType?: string;
  platform?: string;
  topology?: string;
  entityType?: string;
  shared?: boolean;
};

const normalizeClassificationValue = (value: string | undefined | null): string =>
  (value || '').trim().toLowerCase();

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
    platform: typeof candidate.platform === 'string' ? candidate.platform : undefined,
    topology: typeof candidate.topology === 'string' ? candidate.topology : undefined,
    content: typeof candidate.content === 'string' ? candidate.content : undefined,
    contentTypes,
    shared: typeof candidate.shared === 'boolean' ? candidate.shared : undefined,
    path: typeof candidate.path === 'string' ? candidate.path : undefined,
    protection: typeof candidate.protection === 'string' ? candidate.protection : undefined,
    arrayState: typeof candidate.arrayState === 'string' ? candidate.arrayState : undefined,
    syncAction: typeof candidate.syncAction === 'string' ? candidate.syncAction : undefined,
    syncProgress: typeof candidate.syncProgress === 'number' ? candidate.syncProgress : undefined,
    numProtected: typeof candidate.numProtected === 'number' ? candidate.numProtected : undefined,
    numDisabled: typeof candidate.numDisabled === 'number' ? candidate.numDisabled : undefined,
    numInvalid: typeof candidate.numInvalid === 'number' ? candidate.numInvalid : undefined,
    numMissing: typeof candidate.numMissing === 'number' ? candidate.numMissing : undefined,
    isCeph: typeof candidate.isCeph === 'boolean' ? candidate.isCeph : undefined,
    isZfs: typeof candidate.isZfs === 'boolean' ? candidate.isZfs : undefined,
    zfsPool:
      candidate.zfsPool && typeof candidate.zfsPool === 'object'
        ? (candidate.zfsPool as ResourceStorageMeta['zfsPool'])
        : undefined,
  };
};

export const readResourceStorageMeta = (
  resource: Resource,
  platformData: Record<string, unknown>,
): ResourceStorageMeta | undefined => {
  const directMeta = normalizeStorageMeta((resource as ResourceWithStorageMeta).storage);
  if (directMeta) return directMeta;
  const nestedMeta = normalizeStorageMeta(platformData.storage);
  return nestedMeta || undefined;
};

export const resolveResourceStorageContent = (
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

export const isBackupRepositoryStorageResource = (
  type: string | undefined,
  storageMeta?: ResourceStorageMeta,
  context?: StorageClassificationContext,
): boolean => {
  const value = normalizeClassificationValue(type);
  const resourceType = normalizeClassificationValue(context?.resourceType);
  const platform = normalizeClassificationValue(context?.platform || storageMeta?.platform);
  const topology = normalizeClassificationValue(context?.topology || storageMeta?.topology);

  return (
    resourceType === 'pbs' ||
    value.includes('pbs') ||
    platform.includes('pbs') ||
    topology === 'backup-target'
  );
};

export const isCanonicalDatastoreStorageResource = (
  type: string | undefined,
  storageMeta?: ResourceStorageMeta,
  context?: StorageClassificationContext,
): boolean => {
  if (isBackupRepositoryStorageResource(type, storageMeta, context)) {
    return false;
  }

  const value = normalizeClassificationValue(type);
  const resourceType = normalizeClassificationValue(context?.resourceType);
  const platform = normalizeClassificationValue(context?.platform || storageMeta?.platform);
  const topology = normalizeClassificationValue(context?.topology || storageMeta?.topology);
  const entityType = normalizeClassificationValue(context?.entityType);

  return (
    resourceType === 'datastore' ||
    topology === 'datastore' ||
    entityType === 'datastore' ||
    (platform.includes('vmware') && value.length > 0)
  );
};

export const getStorageCapabilitiesForResource = (
  type: string | undefined,
  storageMeta?: ResourceStorageMeta,
  context?: StorageClassificationContext,
): StorageCapability[] => {
  const value = (type || '').toLowerCase();
  const caps: StorageCapability[] = ['capacity', 'health'];
  if (isBackupRepositoryStorageResource(type, storageMeta, context)) {
    caps.push('backup-repository', 'deduplication', 'namespaces');
  }
  if (storageMeta?.isZfs || value.includes('zfs')) {
    caps.push('snapshots', 'compression');
  }
  if (storageMeta?.isCeph || value.includes('ceph')) {
    caps.push('replication', 'multi-node');
  }
  if ((context?.shared ?? storageMeta?.shared) === true) {
    caps.push('multi-node');
  }
  return dedupe(caps);
};

export const getStorageCategoryFromType = (
  type: string | undefined,
  context?: StorageClassificationContext,
): StorageCategory => {
  const value = (type || '').toLowerCase();
  if (!value) return 'other';
  if (isBackupRepositoryStorageResource(type, undefined, context)) return 'backup-repository';
  if (isCanonicalDatastoreStorageResource(type, undefined, context)) return 'datastore';
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
