import { describe, expect, it } from 'vitest';
import type { StorageRecord } from '@/features/storageBackups/models';
import {
  buildStorageSourceOptions,
  filterStorageRecords,
  findSelectedStorageNode,
  groupStorageRecords,
  matchesStorageRecordNode,
  matchesStorageRecordSearch,
  sortStorageRecords,
  summarizeStorageRecords,
  type StorageNodeOption,
} from '@/features/storageBackups/storageModelCore';

const makeNode = (overrides: Partial<StorageNodeOption> = {}): StorageNodeOption => ({
  id: 'node-1',
  label: 'pve1',
  aliases: ['cluster-main', 'pve1.local'],
  ...overrides,
});

const makeRecord = (overrides: Partial<StorageRecord> = {}): StorageRecord =>
  ({
    id: 'storage-1',
    name: 'tank',
    category: 'pool',
    health: 'healthy',
    location: { label: 'pve1', scope: 'node' },
    capacity: { totalBytes: 100, usedBytes: 40, freeBytes: 60, usagePercent: 40 },
    capabilities: ['capacity'],
    source: {
      platform: 'proxmox-pve',
      family: 'virtualization',
      origin: 'resource',
      adapterId: 'resource-storage',
    },
    observedAt: Date.now(),
    statusLabel: 'Healthy',
    details: { node: 'pve1', nodeHints: ['cluster-main'] },
    refs: { platformEntityId: '' },
    ...overrides,
  }) as StorageRecord;

describe('storageModelCore', () => {
  it('finds and matches selected storage nodes canonically', () => {
    const node = makeNode();
    expect(findSelectedStorageNode('all', [node])).toBeNull();
    expect(findSelectedStorageNode('node-1', [node])).toEqual(node);
    expect(matchesStorageRecordNode(makeRecord(), node)).toBe(true);
  });

  it('builds canonical source options and search matching', () => {
    const records = [
      makeRecord(),
      makeRecord({
        id: 'storage-2',
        name: 'main',
        source: {
          platform: 'proxmox-pbs',
          family: 'virtualization',
          origin: 'resource',
          adapterId: 'resource-storage',
        },
      }),
    ];
    expect(buildStorageSourceOptions(records)).toEqual(['all', 'proxmox-pbs', 'proxmox-pve']);
    expect(matchesStorageRecordSearch(makeRecord(), 'tank')).toBe(true);
    expect(matchesStorageRecordSearch(makeRecord(), 'missing')).toBe(false);
  });

  it('filters, sorts, groups, and summarizes storage records canonically', () => {
    const node = makeNode();
    const warning = makeRecord({
      id: 'storage-2',
      name: 'backup',
      health: 'warning',
      incidentPriority: 5,
      source: {
        platform: 'proxmox-pbs',
        family: 'virtualization',
        origin: 'resource',
        adapterId: 'resource-storage',
      },
      details: { node: 'pve1', nodeHints: ['cluster-main'] },
    });
    const records = filterStorageRecords([makeRecord(), warning], {
      search: '',
      sourceFilter: 'all',
      healthFilter: 'all',
      selectedNode: node,
    });

    expect(sortStorageRecords(records, 'priority', 'desc').map((record) => record.id)).toEqual([
      'storage-2',
      'storage-1',
    ]);
    expect(groupStorageRecords(records, 'status').map((group) => group.key)).toEqual([
      'available',
      'degraded',
    ]);
    expect(summarizeStorageRecords(records)).toMatchObject({
      count: 2,
      totalBytes: 200,
      usedBytes: 80,
      usagePercent: 40,
    });
  });
});
