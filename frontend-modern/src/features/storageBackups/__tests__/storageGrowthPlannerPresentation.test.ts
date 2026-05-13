import { describe, expect, it } from 'vitest';
import type { StorageRecord } from '@/features/storageBackups/models';
import { buildStorageGrowthPlannerPresentation } from '@/features/storageBackups/storageGrowthPlannerPresentation';

const GB = 1024 * 1024 * 1024;

const buildRecord = (
  id: string,
  name: string,
  capacity: StorageRecord['capacity'],
): StorageRecord => ({
  id,
  name,
  category: 'pool',
  health: 'healthy',
  hostLabel: 'pve1',
  location: { label: 'pve1', scope: 'node' },
  capacity,
  capabilities: ['capacity'],
  source: {
    platform: 'proxmox-pve',
    family: 'virtualization',
    origin: 'resource',
    adapterId: 'test',
  },
  observedAt: 0,
  metricsTarget: {
    resourceType: 'storage',
    resourceId: `pool:${id}`,
  },
  refs: {},
  details: {},
});

describe('storageGrowthPlannerPresentation', () => {
  it('ranks pools by runway using selected-range used history and current capacity', () => {
    const planner = buildStorageGrowthPlannerPresentation({
      rangeLabel: '24h',
      records: [
        buildRecord('alpha', 'Alpha Pool', {
          totalBytes: 250 * GB,
          usedBytes: 140 * GB,
          freeBytes: 110 * GB,
          usagePercent: 56,
        }),
        buildRecord('beta', 'Beta Pool', {
          totalBytes: 300 * GB,
          usedBytes: 276 * GB,
          freeBytes: 24 * GB,
          usagePercent: 92,
        }),
        buildRecord('gamma', 'Gamma Pool', {
          totalBytes: 200 * GB,
          usedBytes: 80 * GB,
          freeBytes: 120 * GB,
          usagePercent: 40,
        }),
      ],
      pools: {
        'pool:alpha': {
          name: 'Alpha Pool',
          usage: [],
          used: [
            { timestamp: 1, value: 100 * GB },
            { timestamp: 86_400_001, value: 140 * GB },
          ],
          avail: [
            { timestamp: 1, value: 150 * GB },
            { timestamp: 86_400_001, value: 110 * GB },
          ],
        },
        'pool:beta': {
          name: 'Beta Pool',
          usage: [],
          used: [
            { timestamp: 1, value: 276 * GB },
            { timestamp: 86_400_001, value: 276 * GB },
          ],
          avail: [
            { timestamp: 1, value: 24 * GB },
            { timestamp: 86_400_001, value: 24 * GB },
          ],
        },
        'pool:gamma': {
          name: 'Gamma Pool',
          usage: [],
          used: [
            { timestamp: 1, value: 90 * GB },
            { timestamp: 86_400_001, value: 80 * GB },
          ],
          avail: [
            { timestamp: 1, value: 110 * GB },
            { timestamp: 86_400_001, value: 120 * GB },
          ],
        },
      },
    });

    expect(planner.rangeLabel).toBe('24h');
    expect(planner.trackedPoolCount).toBe(3);
    expect(planner.growingPoolCount).toBe(1);
    expect(planner.topPools.map((pool) => pool.name)).toEqual(['Alpha Pool', 'Beta Pool']);
    expect(planner.topPools[0]).toMatchObject({
      growthLabel: '+40.0 GB',
      runwayLabel: '3 days',
      priorityLabel: 'Plan now',
    });
    expect(planner.topPools[1]).toMatchObject({
      growthLabel: '0 B',
      runwayLabel: 'Unknown',
      priorityLabel: 'Plan now',
    });
  });

  it('returns an empty state when scoped pools are stable', () => {
    const planner = buildStorageGrowthPlannerPresentation({
      rangeLabel: '7d',
      records: [
        buildRecord('stable', 'Stable Pool', {
          totalBytes: 100 * GB,
          usedBytes: 20 * GB,
          freeBytes: 80 * GB,
          usagePercent: 20,
        }),
      ],
      pools: {
        'pool:stable': {
          name: 'Stable Pool',
          usage: [],
          used: [
            { timestamp: 1, value: 30 * GB },
            { timestamp: 604_800_001, value: 20 * GB },
          ],
          avail: [
            { timestamp: 1, value: 70 * GB },
            { timestamp: 604_800_001, value: 80 * GB },
          ],
        },
      },
    });

    expect(planner.topPools).toEqual([]);
    expect(planner.emptyTitle).toBe('No capacity planning pressure over 7d');
  });
});
