import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  getPhysicalDiskNodeIdentity,
  matchesPhysicalDiskNode,
  resolvePhysicalDiskMetricResourceId,
} from '@/components/Storage/diskResourceUtils';

const buildDisk = (overrides: Partial<Resource> = {}): Resource =>
  ({
    id: 'disk-1',
    type: 'physical_disk',
    name: 'disk-1',
    displayName: 'disk-1',
    platformId: 'cluster-main',
    platformType: 'proxmox-pve',
    sourceType: 'api',
    status: 'online',
    lastSeen: Date.now(),
    platformData: {
      proxmox: { nodeName: 'tower', instance: 'cluster-main' },
      physicalDisk: { devPath: '/dev/sda' },
    },
    identity: { hostname: 'tower' },
    canonicalIdentity: { hostname: 'tower' },
    ...overrides,
  }) as Resource;

const buildNode = (overrides: Partial<Resource> = {}): Resource =>
  ({
    id: 'node-1',
    type: 'agent',
    name: 'tower',
    displayName: 'tower',
    platformId: 'cluster-main',
    platformType: 'proxmox-pve',
    sourceType: 'api',
    status: 'online',
    lastSeen: Date.now(),
    platformData: {
      proxmox: { instance: 'cluster-main' },
      agent: { agentId: 'agent-tower' },
    },
    ...overrides,
  }) as Resource;

describe('diskResourceUtils', () => {
  it('derives physical disk node identity canonically', () => {
    expect(getPhysicalDiskNodeIdentity(buildDisk())).toEqual({
      node: 'tower',
      instance: 'cluster-main',
    });
  });

  it('matches physical disks to nodes by canonical node identity', () => {
    expect(
      matchesPhysicalDiskNode(buildDisk(), {
        id: 'node-1',
        name: 'tower',
        instance: 'cluster-main',
      }),
    ).toBe(true);
  });

  it('resolves physical disk metric targets through linked agents when needed', () => {
    expect(resolvePhysicalDiskMetricResourceId(buildDisk(), [buildNode()], '/dev/sda')).toBe(
      'agent-tower:sda',
    );
    expect(
      resolvePhysicalDiskMetricResourceId(
        buildDisk({ metricsTarget: { resourceId: 'existing-target' } as any }),
        [buildNode()],
        '/dev/sda',
      ),
    ).toBe('existing-target');
  });
});
