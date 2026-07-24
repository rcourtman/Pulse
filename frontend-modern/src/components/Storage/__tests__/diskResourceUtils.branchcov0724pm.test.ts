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

describe('diskResourceUtils branch coverage', () => {
  describe('getPhysicalDiskNodeIdentity node fallback chain', () => {
    // L24: `resource.platformData || {}` right-hand fallback — platformData falsy.
    it('falls back to an empty platformData record and resolves the node from identity', () => {
      const disk = buildDisk({ platformData: undefined });
      // No proxmox block survives the fallback, so identity.hostname is the
      // only node evidence and the instance collapses to ''.
      expect(getPhysicalDiskNodeIdentity(disk)).toEqual({ node: 'tower', instance: '' });
    });

    // L28: proxmox.nodeName absent -> identity?.hostname arm.
    it('derives the node from resource identity hostname when proxmox omits it', () => {
      const disk = buildDisk({
        platformData: { physicalDisk: { devPath: '/dev/sda' } } as Resource['platformData'],
        identity: { hostname: 'host-from-identity' },
      });
      expect(getPhysicalDiskNodeIdentity(disk)).toEqual({
        node: 'host-from-identity',
        instance: '',
      });
    });

    // L28: proxmox.nodeName + identity absent -> canonicalIdentity?.hostname arm.
    it('falls back to canonicalIdentity hostname when proxmox and identity are absent', () => {
      const disk = buildDisk({
        platformData: { physicalDisk: { devPath: '/dev/sda' } } as Resource['platformData'],
        identity: undefined,
        canonicalIdentity: { hostname: 'canon-host' },
      });
      expect(getPhysicalDiskNodeIdentity(disk)).toEqual({ node: 'canon-host', instance: '' });
    });

    // L28: every hostname source absent -> final `|| ''` arm.
    it('returns an empty node when no hostname evidence exists anywhere', () => {
      const disk = buildDisk({
        platformData: { physicalDisk: { devPath: '/dev/sda' } } as Resource['platformData'],
        identity: undefined,
        canonicalIdentity: undefined,
      });
      expect(getPhysicalDiskNodeIdentity(disk)).toEqual({ node: '', instance: '' });
    });
  });

  describe('matchesPhysicalDiskNode', () => {
    // L40: `target.id && disk.parentId === target.id` -> early `return true`.
    it('short-circuits to a match when the disk parentId equals the target id', () => {
      // A name that would NOT match by node identity, proving the parentId
      // early-return path is the one taken.
      const disk = buildDisk({ parentId: 'node-1' });
      expect(matchesPhysicalDiskNode(disk, { id: 'node-1', name: 'unrelated-node-name' })).toBe(
        true,
      );
    });
  });

  describe('resolvePhysicalDiskMetricResourceId', () => {
    // L82 ternary false arm + L84 `!agentId` early return: no candidate node.
    it('returns null when the node collection is empty and no agent can be linked', () => {
      expect(resolvePhysicalDiskMetricResourceId(buildDisk(), [], '/dev/sda')).toBeNull();
    });
  });
});
