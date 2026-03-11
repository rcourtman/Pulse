import { describe, expect, it } from 'vitest';
import type { StorageRecord } from '@/features/storageBackups/models';
import type { Resource } from '@/types/resource';
import {
  buildCephClusterLookup,
  buildExplicitCephClusters,
  deriveCephClustersFromStorageRecords,
  resolveCephClusterForStorageRecord,
  summarizeCephClusters,
} from '@/features/storageBackups/cephSummaryPresentation';

const makeRecord = (overrides: Partial<StorageRecord> = {}): StorageRecord =>
  ({
    id: 'storage-1',
    name: 'ceph-store',
    category: 'pool',
    health: 'healthy',
    location: { label: 'cluster-main', scope: 'cluster' },
    capacity: { totalBytes: 100, usedBytes: 40, freeBytes: 60, usagePercent: 40 },
    capabilities: ['capacity'],
    source: {
      platform: 'proxmox-pve',
      family: 'virtualization',
      origin: 'resource',
      adapterId: 'resource-storage',
    },
    observedAt: Date.now(),
    details: {
      type: 'rbd',
      isCeph: true,
      node: 'pve1',
      parentName: 'pve1',
      clusterKey: 'cluster-main',
    },
    refs: { platformEntityId: '' },
    ...overrides,
  }) as StorageRecord;

const makeCephResource = (): Resource =>
  ({
    id: 'ceph-1',
    type: 'ceph',
    name: 'ceph-main',
    displayName: 'ceph-main',
    platformId: 'cluster-main',
    platformType: 'proxmox-pve',
    sourceType: 'api',
    status: 'online',
    lastSeen: Date.now(),
    disk: { total: 300, used: 120, free: 180, current: 40 },
    platformData: {
      proxmox: { instance: 'cluster-main' },
      ceph: {
        healthStatus: 'HEALTH_OK',
        healthMessage: '',
        numMons: 3,
        numMgrs: 2,
        numOsds: 6,
        numOsdsUp: 6,
        numOsdsIn: 6,
        numPGs: 128,
      },
    },
  }) as Resource;

describe('cephSummaryPresentation', () => {
  it('derives ceph clusters from storage records canonically', () => {
    const clusters = deriveCephClustersFromStorageRecords([
      makeRecord(),
      makeRecord({
        id: 'storage-2',
        capacity: { totalBytes: 200, usedBytes: 80, freeBytes: 120, usagePercent: 40 },
        details: {
          type: 'rbd',
          isCeph: true,
          node: 'pve2',
          parentName: 'pve2',
          clusterKey: 'cluster-main',
        },
      }),
    ]);

    expect(clusters).toHaveLength(1);
    expect(clusters[0]).toMatchObject({
      instance: 'cluster-main',
      totalBytes: 300,
      usedBytes: 120,
      availableBytes: 180,
    });
  });

  it('summarizes ceph clusters canonically', () => {
    expect(
      summarizeCephClusters([
        {
          id: 'cluster-1',
          instance: 'cluster-main',
          name: 'cluster-main Ceph',
          health: 'HEALTH_OK',
          healthMessage: '',
          totalBytes: 300,
          usedBytes: 120,
          availableBytes: 180,
          usagePercent: 40,
          numMons: 3,
          numMgrs: 2,
          numOsds: 6,
          numOsdsUp: 6,
          numOsdsIn: 6,
          numPGs: 128,
          pools: undefined,
          services: undefined,
          lastUpdated: Date.now(),
        },
      ]),
    ).toMatchObject({
      totalBytes: 300,
      usedBytes: 120,
      availableBytes: 180,
      usagePercent: 40,
    });
  });

  it('builds explicit ceph clusters and lookup state canonically', () => {
    const explicit = buildExplicitCephClusters([makeCephResource()]);
    expect(explicit).toHaveLength(1);
    expect(explicit[0]).toMatchObject({
      instance: 'cluster-main',
      totalBytes: 300,
      usedBytes: 120,
      availableBytes: 180,
    });

    const lookup = buildCephClusterLookup(explicit);
    expect(resolveCephClusterForStorageRecord(makeRecord(), lookup)).toBe(explicit[0]);
  });
});
