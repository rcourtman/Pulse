import type { Resource } from '@/types/resource';
import type { StorageCapability, StorageCategory } from './models';

export type ResourceStorageMeta = {
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

const dedupe = <T>(values: T[]): T[] => Array.from(new Set(values));

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

export const getStorageCapabilitiesForResource = (
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

export const getStorageCategoryFromType = (type: string | undefined): StorageCategory => {
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
