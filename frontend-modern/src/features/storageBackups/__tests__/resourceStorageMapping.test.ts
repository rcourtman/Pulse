import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  getStorageCapabilitiesForResource,
  getStorageCategoryFromType,
  readResourceStorageMeta,
  resolveResourceStorageContent,
} from '@/features/storageBackups/resourceStorageMapping';

const makeResource = (overrides: Partial<Resource> = {}): Resource =>
  ({
    id: 'storage-1',
    type: 'storage',
    name: 'tank',
    platformType: 'truenas',
    sourceType: 'api',
    ...overrides,
  }) as Resource;

describe('resourceStorageMapping', () => {
  it('prefers direct resource storage meta over nested platform data', () => {
    const resource = makeResource({
      storage: {
        type: 'rbd',
        platform: 'proxmox-pve',
        topology: 'pool',
        contentTypes: ['images', 'rootdir'],
        shared: false,
        isCeph: true,
      },
      platformData: {
        storage: {
          type: 'dir',
          platform: 'proxmox-pbs',
          topology: 'backup-target',
          shared: true,
        },
      },
    });

    expect(readResourceStorageMeta(resource, resource.platformData as Record<string, unknown>)).toEqual({
      type: 'rbd',
      platform: 'proxmox-pve',
      topology: 'pool',
      content: undefined,
      contentTypes: ['images', 'rootdir'],
      shared: false,
      isCeph: true,
      isZfs: undefined,
    });
  });

  it('resolves canonical storage content from direct, contentTypes, or platform fallback', () => {
    expect(
      resolveResourceStorageContent(
        { content: 'backup' },
        { content: 'ignored' },
        '',
      ),
    ).toBe('backup');
    expect(
      resolveResourceStorageContent(
        { contentTypes: ['images', 'rootdir'] },
        { content: 'ignored' },
        '',
      ),
    ).toBe('images,rootdir');
    expect(resolveResourceStorageContent(undefined, { content: 'iso' }, 'backup')).toBe('iso');
    expect(resolveResourceStorageContent(undefined, {}, 'backup')).toBe('backup');
  });

  it('derives canonical storage capabilities and categories', () => {
    expect(getStorageCapabilitiesForResource('pbs')).toEqual([
      'capacity',
      'health',
      'backup-repository',
      'deduplication',
      'namespaces',
    ]);
    expect(
      getStorageCapabilitiesForResource('rbd', {
        isCeph: true,
      }),
    ).toEqual(['capacity', 'health', 'replication', 'multi-node']);
    expect(
      getStorageCapabilitiesForResource('vmfs', {
        platform: 'vmware-vsphere',
        topology: 'datastore',
        shared: true,
      }),
    ).toEqual(['capacity', 'health', 'multi-node']);
    expect(getStorageCategoryFromType('zfs-pool')).toBe('pool');
    expect(getStorageCategoryFromType('pbs')).toBe('backup-repository');
    expect(
      getStorageCategoryFromType('vmfs', {
        platform: 'vmware-vsphere',
        topology: 'datastore',
        entityType: 'datastore',
      }),
    ).toBe('datastore');
    expect(getStorageCategoryFromType('filesystem')).toBe('filesystem');
  });
});
