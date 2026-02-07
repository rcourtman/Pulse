import { describe, expect, it } from 'vitest';
import type { Storage } from '@/types/api';
import {
  buildStorageSourceOptions,
  normalizeStorageSourceKey,
  resolveStorageSourceKey,
} from '@/components/Storage/storageSourceOptions';

const makeStorage = (partial: Partial<Storage>): Storage => ({
  id: partial.id || 'storage-1',
  name: partial.name || 'pool',
  node: partial.node || 'node-1',
  instance: partial.instance || 'cluster-a',
  type: partial.type || 'dir',
  status: partial.status || 'available',
  total: partial.total ?? 1,
  used: partial.used ?? 0,
  free: partial.free ?? 1,
  usage: partial.usage ?? 0,
  content: partial.content || 'images',
  shared: partial.shared ?? false,
  enabled: partial.enabled ?? true,
  active: partial.active ?? true,
  nodes: partial.nodes,
  nodeIds: partial.nodeIds,
  nodeCount: partial.nodeCount,
  pbsNames: partial.pbsNames,
  zfsPool: partial.zfsPool,
});

describe('storageSourceOptions', () => {
  it('normalizes known source aliases', () => {
    expect(normalizeStorageSourceKey('PVE')).toBe('proxmox');
    expect(normalizeStorageSourceKey('proxmox-pbs')).toBe('pbs');
    expect(normalizeStorageSourceKey('rbd')).toBe('ceph');
    expect(normalizeStorageSourceKey('k8s')).toBe('kubernetes');
  });

  it('resolves storage source from storage type', () => {
    expect(resolveStorageSourceKey(makeStorage({ type: 'pbs' }))).toBe('pbs');
    expect(resolveStorageSourceKey(makeStorage({ type: 'cephfs' }))).toBe('ceph');
    expect(resolveStorageSourceKey(makeStorage({ type: 'lvmthin' }))).toBe('proxmox');
  });

  it('builds data-driven source options with stable ordering and all option', () => {
    const options = buildStorageSourceOptions([
      makeStorage({ type: 'ceph' }),
      makeStorage({ id: '2', type: 'pbs' }),
      makeStorage({ id: '3', type: 'dir' }),
      makeStorage({ id: '4', type: 'custom-backend' }),
    ]);

    expect(options[0]).toMatchObject({ key: 'all', label: 'All Sources' });
    expect(options.map((option) => option.key)).toEqual([
      'all',
      'proxmox',
      'pbs',
      'ceph',
      'custom-backend',
    ]);
  });
});
