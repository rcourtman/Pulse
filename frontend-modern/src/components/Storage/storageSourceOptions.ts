import type { Storage } from '@/types/api';

export type StorageSourceTone = 'slate' | 'blue' | 'emerald' | 'violet' | 'cyan';

export interface StorageSourceOption {
  key: string;
  label: string;
  tone: StorageSourceTone;
}

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

const STORAGE_SOURCE_PRESETS: Record<string, Omit<StorageSourceOption, 'key'>> = {
  proxmox: { label: 'PVE', tone: 'blue' },
  pbs: { label: 'PBS', tone: 'emerald' },
  ceph: { label: 'Ceph', tone: 'violet' },
  kubernetes: { label: 'K8s', tone: 'cyan' },
  pmg: { label: 'PMG', tone: 'blue' },
};

const STORAGE_SOURCE_ORDER = ['proxmox', 'pbs', 'ceph', 'kubernetes', 'pmg'];

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
  switch (normalized) {
    case 'all':
      return 'all';
    case 'pve':
    case 'proxmox':
    case 'proxmox-pve':
      return 'proxmox';
    case 'pbs':
    case 'proxmox-pbs':
      return 'pbs';
    case 'ceph':
    case 'cephfs':
    case 'rbd':
      return 'ceph';
    case 'k8s':
    case 'kubernetes':
      return 'kubernetes';
    default:
      return normalized;
  }
};

export const resolveStorageSourceKey = (storage: Storage): string => {
  const type = normalizeStorageSourceKey(storage.type);

  if (type === 'pbs' || type === 'ceph' || type === 'kubernetes' || type === 'pmg') {
    return type;
  }

  if (CEPH_TYPES.has(type)) {
    return 'ceph';
  }

  if (PROXMOX_STORAGE_TYPES.has(type) || type === '' || type === 'storage') {
    return 'proxmox';
  }

  if (type.includes('k8s') || type.includes('kubernetes') || type.includes('csi')) {
    return 'kubernetes';
  }

  return type || 'proxmox';
};

export const buildStorageSourceOptions = (storageList: Storage[]): StorageSourceOption[] => {
  const keys = new Set<string>();

  storageList.forEach((storage) => {
    keys.add(resolveStorageSourceKey(storage));
  });

  const orderedKeys = Array.from(keys).sort((a, b) => {
    const indexA = STORAGE_SOURCE_ORDER.indexOf(a);
    const indexB = STORAGE_SOURCE_ORDER.indexOf(b);
    if (indexA !== -1 || indexB !== -1) {
      if (indexA === -1) return 1;
      if (indexB === -1) return -1;
      return indexA - indexB;
    }
    return a.localeCompare(b);
  });

  const dynamicOptions = orderedKeys.map<StorageSourceOption>((key) => {
    const preset = STORAGE_SOURCE_PRESETS[key];
    if (preset) {
      return { key, ...preset };
    }
    return {
      key,
      label: titleCaseLabel(key) || 'Other',
      tone: 'slate',
    };
  });

  return [{ key: 'all', label: 'All Sources', tone: 'slate' }, ...dynamicOptions];
};
