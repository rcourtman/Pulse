import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  getPhysicalDiskNodeIdentity,
  matchesPhysicalDiskNode,
} from '@/components/Storage/diskResourceUtils';

const buildDiskResource = (overrides: Partial<Resource> = {}): Resource =>
  ({
    id: 'disk-1',
    type: 'physical_disk',
    name: 'Disk',
    displayName: 'Disk',
    platformId: 'disk-platform',
    platformType: 'proxmox-pve',
    sourceType: 'api',
    status: 'online',
    lastSeen: Date.now(),
    ...overrides,
  }) as Resource;

describe('diskResourceUtils', () => {
  it('derives node identity from hostname fallback when proxmox metadata is absent', () => {
    const disk = buildDiskResource({
      identity: { hostname: 'minipc' },
      canonicalIdentity: { hostname: 'minipc' },
      platformData: {
        physicalDisk: { devPath: '/dev/sda' },
      },
    });

    expect(getPhysicalDiskNodeIdentity(disk)).toEqual({
      node: 'minipc',
      instance: '',
    });
  });

  it('matches disks to nodes by hostname when parentId is absent', () => {
    const disk = buildDiskResource({
      identity: { hostname: 'delly' },
      canonicalIdentity: { hostname: 'delly' },
      platformData: {
        physicalDisk: { devPath: '/dev/nvme0n1' },
      },
    });

    expect(
      matchesPhysicalDiskNode(disk, {
        id: 'node-1',
        name: 'delly',
        instance: 'cluster-a',
      }),
    ).toBe(true);
  });

  it('still prefers direct parent matches when parentId is present', () => {
    const disk = buildDiskResource({
      parentId: 'node-2',
      identity: { hostname: 'wrong-host' },
      platformData: {
        physicalDisk: { devPath: '/dev/sda' },
      },
    });

    expect(
      matchesPhysicalDiskNode(disk, {
        id: 'node-2',
        name: 'minipc',
        instance: 'cluster-a',
      }),
    ).toBe(true);
  });
});
