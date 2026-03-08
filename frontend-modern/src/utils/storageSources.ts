import type { Storage } from '@/types/api';
import { getSourcePlatformLabel, normalizeSourcePlatformKey } from '@/utils/sourcePlatforms';

export type StorageSourceTone = 'slate' | 'orange' | 'indigo' | 'rose' | 'violet' | 'cyan';

export interface StorageSourceOption {
  key: string;
  label: string;
  tone: StorageSourceTone;
}

const ALL_STORAGE_SOURCE_OPTION: StorageSourceOption = {
  key: 'all',
  label: 'All Sources',
  tone: 'slate',
};

const STORAGE_SOURCE_TONES: Record<string, StorageSourceTone> = {
  'proxmox-pve': 'orange',
  'proxmox-pbs': 'indigo',
  'proxmox-pmg': 'rose',
  ceph: 'violet',
  kubernetes: 'cyan',
};

const CEPH_TYPES = new Set(['ceph', 'cephfs', 'rbd']);

const PROXMOX_STORAGE_TYPES = new Set([
  'dir',
  'nfs',
  'cifs',
  'lvm',
  'lvmthin',
  'zfspool',
  'zfs',
  'iscsi',
  'glusterfs',
  'btrfs',
]);

const STORAGE_SOURCE_ORDER = ['proxmox-pve', 'proxmox-pbs', 'ceph', 'kubernetes', 'proxmox-pmg'];

const slugifySource = (value: string): string =>
  value
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');

const titleCaseLabel = (value: string): string =>
  value
    .split('-')
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ');

export const normalizeStorageSourceKey = (value: string | null | undefined): string => {
  const normalized = slugifySource(value || '');
  const canonicalPlatform = normalizeSourcePlatformKey(normalized);
  if (canonicalPlatform) return canonicalPlatform;

  switch (normalized) {
    case 'all':
      return 'all';
    case 'ceph':
    case 'cephfs':
    case 'rbd':
      return 'ceph';
    default:
      return normalized;
  }
};

export const resolveStorageSourceKey = (storage: Storage): string => {
  const type = normalizeStorageSourceKey(storage.type);

  if (
    type === 'proxmox-pbs' ||
    type === 'ceph' ||
    type === 'kubernetes' ||
    type === 'proxmox-pmg'
  ) {
    return type;
  }

  if (CEPH_TYPES.has(type)) {
    return 'ceph';
  }

  if (PROXMOX_STORAGE_TYPES.has(type) || type === '' || type === 'storage') {
    return 'proxmox-pve';
  }

  if (type.includes('k8s') || type.includes('kubernetes') || type.includes('csi')) {
    return 'kubernetes';
  }

  return type || 'proxmox-pve';
};

export const getStorageSourceOption = (value: string | null | undefined): StorageSourceOption => {
  const key = normalizeStorageSourceKey(value);

  if (key === 'all') {
    return ALL_STORAGE_SOURCE_OPTION;
  }

  return {
    key,
    label:
      (key === 'ceph' ? 'Ceph' : getSourcePlatformLabel(key)) || titleCaseLabel(key) || 'Other',
    tone: STORAGE_SOURCE_TONES[key] || 'slate',
  };
};

export const orderStorageSourceKeys = (keys: Iterable<string>): string[] =>
  Array.from(new Set(Array.from(keys).map((key) => normalizeStorageSourceKey(key)))).sort(
    (a, b) => {
      const indexA = STORAGE_SOURCE_ORDER.indexOf(a);
      const indexB = STORAGE_SOURCE_ORDER.indexOf(b);
      if (indexA !== -1 || indexB !== -1) {
        if (indexA === -1) return 1;
        if (indexB === -1) return -1;
        return indexA - indexB;
      }
      return a.localeCompare(b);
    },
  );

export const buildStorageSourceOptionsFromKeys = (
  keys: Iterable<string>,
): StorageSourceOption[] => {
  const orderedKeys = orderStorageSourceKeys(keys).filter((key) => key !== 'all');
  return [ALL_STORAGE_SOURCE_OPTION, ...orderedKeys.map((key) => getStorageSourceOption(key))];
};

export const DEFAULT_STORAGE_SOURCE_OPTIONS: StorageSourceOption[] =
  buildStorageSourceOptionsFromKeys(['proxmox-pve', 'proxmox-pbs', 'ceph']);

export const buildStorageSourceOptions = (storageList: Storage[]): StorageSourceOption[] =>
  buildStorageSourceOptionsFromKeys(storageList.map((storage) => resolveStorageSourceKey(storage)));
