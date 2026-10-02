import { describe, expect, it } from 'vitest';
import type { State } from '@/types/api';
import type { Resource } from '@/types/resource';
import { buildStorageRecords } from '../storageAdapters';
import { getStoragePoolLinkedDisks } from '../storagePoolDetailPresentation';

const storage = (overrides: Partial<Resource> = {}): Resource =>
  ({
    id: 'pool-a',
    type: 'storage',
    name: 'tank',
    displayName: 'tank',
    platformId: 'provider-a',
    platformType: 'proxmox-pve',
    sourceType: 'api',
    status: 'online',
    lastSeen: 1,
    parentId: 'host-a',
    parentName: 'Same display name',
    storage: {
      type: 'zfspool',
      isZfs: true,
      zfsPool: {
        name: 'tank',
        state: 'ONLINE',
        readErrors: 0,
        writeErrors: 0,
        checksumErrors: 0,
        devices: [{ name: 'sda', type: 'disk', state: 'ONLINE' }],
      },
    },
    proxmox: { nodeName: 'pve1', instance: 'cluster-a' },
    ...overrides,
  }) as Resource;

const disk = (id: string, overrides: Partial<Resource> = {}): Resource =>
  ({
    id,
    type: 'physical_disk',
    name: id,
    displayName: id,
    platformId: 'provider-a',
    platformType: 'proxmox-pve',
    sourceType: 'api',
    status: 'online',
    lastSeen: 1,
    parentId: 'host-a',
    parentName: 'Same display name',
    proxmox: { nodeName: 'pve1', instance: 'cluster-a' },
    physicalDisk: { devPath: '/dev/sda', model: id, temperature: 35 },
    ...overrides,
  }) as Resource;

const record = (resource = storage()) =>
  buildStorageRecords({ state: {} as State, resources: [resource] })[0];
const linked = (resource: Resource, disks: Resource[]) =>
  getStoragePoolLinkedDisks(record(resource), disks).map((item) => item.id);

describe('pool physical disk ownership', () => {
  it('excludes other hosts and unknown owners despite identical paths and labels', () => {
    expect(
      linked(storage(), [
        disk('local'),
        disk('remote', { parentId: 'host-b' }),
        disk('unknown', { parentId: undefined, proxmox: undefined }),
      ]),
    ).toEqual(['local']);
  });

  it('never lets matching provider names override mismatched canonical parents', () => {
    expect(linked(storage(), [disk('remote', { parentId: 'host-b' })])).toEqual([]);
  });

  it('preserves direct canonical pool and legacy resource-reference children', () => {
    const target = record();
    target.refs = { resourceId: 'legacy-pool' };
    expect(
      getStoragePoolLinkedDisks(target, [
        disk('direct', { parentId: 'pool-a', physicalDisk: { devPath: '/dev/other' } }),
        disk('legacy', { parentId: 'legacy-pool', physicalDisk: { devPath: '/dev/legacy' } }),
      ]).map((item) => item.id),
    ).toEqual(['legacy', 'direct']);
  });

  it.each(['top-level', 'legacy'] as const)(
    'retains exact native instance/node ownership: %s',
    (shape) => {
      const pool = storage({ parentId: undefined, proxmox: undefined, platformData: undefined });
      const local = disk('local', { parentId: undefined, proxmox: undefined });
      const native = { nodeName: 'pve1', instance: 'cluster-a' };
      if (shape === 'top-level') {
        pool.proxmox = native;
        local.proxmox = native;
      } else {
        pool.platformData = { proxmox: native };
        local.platformData = { proxmox: native };
      }
      expect(record(pool).details?.instance).toBe('cluster-a');
      expect(
        linked(pool, [
          local,
          disk('other-cluster', {
            parentId: undefined,
            proxmox: { ...native, instance: 'cluster-b' },
          }),
          disk('other-node', { parentId: undefined, proxmox: { ...native, nodeName: 'pve2' } }),
          disk('unscoped', { parentId: undefined, proxmox: { nodeName: 'pve1' } }),
        ]),
      ).toEqual(['local']);
    },
  );

  it('requires both native instance and node when canonical ownership is missing', () => {
    expect(
      linked(storage({ parentId: undefined, proxmox: { nodeName: 'pve1' } }), [
        disk('unscoped', { parentId: undefined, proxmox: { nodeName: 'pve1' } }),
      ]),
    ).toEqual([]);
  });

  it.each(['sda', '/dev/sda'])('matches whole device identifiers, not a suffix: %s', (name) => {
    const pool = storage();
    pool.storage!.zfsPool!.devices = [
      { name, type: 'disk', state: 'ONLINE', readErrors: 0, writeErrors: 0, checksumErrors: 0 },
    ];
    expect(
      linked(pool, [
        disk('local'),
        disk('suffix-collision', { physicalDisk: { devPath: '/dev/not-sda' } }),
        disk('partition', { physicalDisk: { devPath: '/dev/sda1' } }),
      ]),
    ).toEqual(['local']);
  });

  it('does not confuse nvme device numbers with a shorter vdev name', () => {
    const pool = storage();
    pool.storage!.zfsPool!.devices = [
      {
        name: 'n1',
        type: 'disk',
        state: 'ONLINE',
        readErrors: 0,
        writeErrors: 0,
        checksumErrors: 0,
      },
    ];
    expect(linked(pool, [disk('nvme', { physicalDisk: { devPath: '/dev/nvme0n1' } })])).toEqual([]);
  });

  it.each([
    ['unraid-array', 'Array', 'unraid-array'],
    ['unraid-cache-pool', 'cache', 'cache'],
  ])('keeps repeated UnRAID groups on their own host: %s', (type, name, group) => {
    const pool = storage({
      name,
      platformType: 'unraid',
      proxmox: undefined,
      storage: { type, platform: 'unraid' },
    });
    const member = { devPath: '/dev/sda', storageGroup: group, storageRole: 'data' };
    expect(
      linked(pool, [
        disk('local', { physicalDisk: member }),
        disk('remote', { parentId: 'host-b', physicalDisk: member }),
        disk('unknown', { parentId: undefined, proxmox: undefined, physicalDisk: member }),
        disk('different-group', { physicalDisk: { ...member, storageGroup: 'other' } }),
      ]),
    ).toEqual(['local']);
  });
});
