import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  buildProxmoxEstateTopology,
  deserializePlatformEstateCountsVisibility,
} from '../platformEstateOverviewModel';

const makeNode = (id: string, clusterName?: string): Resource =>
  ({
    id,
    type: 'agent',
    name: id,
    displayName: id,
    platformId: 'proxmox',
    platformType: 'proxmox-pve',
    sourceType: 'api',
    status: 'online',
    lastSeen: 1_700_000_000_000,
    proxmox: clusterName ? { clusterName } : {},
  }) as Resource;

describe('platformEstateOverviewModel', () => {
  it('derives node, cluster, and standalone totals from the canonical resource set', () => {
    const resources = [
      makeNode('pve-a', 'production'),
      makeNode('pve-b', 'production'),
      makeNode('pve-c', 'lab'),
      makeNode('pve-standalone'),
      { ...makeNode('vm-101'), type: 'vm' as const },
    ];

    expect(buildProxmoxEstateTopology(resources)).toEqual({
      clusters: 2,
      nodes: 4,
      standalone: 1,
    });
  });

  it('keeps a very large topology projection linear and bounded', () => {
    const resources = Array.from({ length: 10_000 }, (_, index) =>
      makeNode(`node-${index}`, index % 25 === 0 ? undefined : `cluster-${index % 40}`),
    );

    expect(buildProxmoxEstateTopology(resources)).toEqual({
      clusters: 40,
      nodes: 10_000,
      standalone: 400,
    });
  });

  it('preserves the tolerant global visibility preference', () => {
    expect(deserializePlatformEstateCountsVisibility('false')).toBe(false);
    expect(deserializePlatformEstateCountsVisibility('true')).toBe(true);
    expect(deserializePlatformEstateCountsVisibility('legacy-value')).toBe(true);
  });
});
