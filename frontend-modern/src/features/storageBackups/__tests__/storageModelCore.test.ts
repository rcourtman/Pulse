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
      makeRecord({
        id: 'storage-3',
        name: 'tank',
        source: {
          platform: 'truenas',
          family: 'onprem',
          origin: 'resource',
          adapterId: 'resource-storage',
        },
      }),
    ];
    expect(buildStorageSourceOptions(records)).toEqual([
      'all',
      'proxmox-pve',
      'proxmox-pbs',
      'truenas',
    ]);
    expect(matchesStorageRecordSearch(makeRecord(), 'tank')).toBe(true);
    expect(matchesStorageRecordSearch(makeRecord(), 'node:pve1')).toBe(true);
    expect(matchesStorageRecordSearch(makeRecord(), 'node:pve1 tank')).toBe(true);
    expect(matchesStorageRecordSearch(makeRecord(), 'node:pve2')).toBe(false);
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
      healthFilter: 'attention',
      selectedNode: node,
    });

    expect(sortStorageRecords(records, 'priority', 'desc').map((record) => record.id)).toEqual([
      'storage-2',
    ]);
    expect(groupStorageRecords(records, 'status').map((group) => group.key)).toEqual(['degraded']);
    expect(summarizeStorageRecords(records)).toMatchObject({
      count: 1,
      totalBytes: 100,
      usedBytes: 40,
      usagePercent: 40,
    });
  });

  it('sorts storage records by the visible pool-table columns', () => {
    const alpha = makeRecord({
      id: 'storage-alpha',
      name: 'Alpha',
      platformLabel: 'Zeta source',
      topologyLabel: 'Dataset',
      hostLabel: 'node-b',
      protectionLabel: 'Mirror',
      details: { status: 'online', type: 'dir' },
      capacity: { totalBytes: 100, usedBytes: 20, freeBytes: 80, usagePercent: 20 },
    });
    const beta = makeRecord({
      id: 'storage-beta',
      name: 'Beta',
      platformLabel: 'Alpha source',
      topologyLabel: 'Block',
      hostLabel: 'node-a',
      protectionLabel: 'Raidz',
      details: { status: 'degraded', type: 'zfspool' },
      capacity: { totalBytes: 100, usedBytes: 80, freeBytes: 20, usagePercent: 80 },
    });
    const gamma = makeRecord({
      id: 'storage-gamma',
      name: 'Gamma',
      platformLabel: 'Beta source',
      topologyLabel: 'File',
      hostLabel: 'node-c',
      protectionLabel: 'Archive',
      details: { status: 'unknown', type: 'nfs' },
      capacity: { totalBytes: 0, usedBytes: 0, freeBytes: 0, usagePercent: 0 },
    });
    const records = [alpha, beta, gamma];
    const growthBySeriesId = new Map([
      [
        'storage-alpha',
        { deltaBytes: 200, label: '+200 B', title: 'Growth', toneClass: 'text-muted' },
      ],
      [
        'storage-beta',
        { deltaBytes: 800, label: '+800 B', title: 'Growth', toneClass: 'text-muted' },
      ],
    ]);

    expect(sortStorageRecords(records, 'source', 'asc').map((record) => record.id)).toEqual([
      'storage-beta',
      'storage-gamma',
      'storage-alpha',
    ]);
    expect(sortStorageRecords(records, 'host', 'asc').map((record) => record.id)).toEqual([
      'storage-beta',
      'storage-alpha',
      'storage-gamma',
    ]);
    expect(sortStorageRecords(records, 'state', 'asc').map((record) => record.id)).toEqual([
      'storage-beta',
      'storage-alpha',
      'storage-gamma',
    ]);
    expect(sortStorageRecords(records, 'protection', 'desc').map((record) => record.id)).toEqual([
      'storage-beta',
      'storage-alpha',
      'storage-gamma',
    ]);
    expect(
      sortStorageRecords(records, 'growth', 'desc', { growthBySeriesId }).map(
        (record) => record.id,
      ),
    ).toEqual(['storage-beta', 'storage-alpha', 'storage-gamma']);
  });

  it('does not synthesize an empty group when filters remove every storage record', () => {
    expect(groupStorageRecords([], 'none')).toEqual([]);
    expect(groupStorageRecords([], 'status')).toEqual([]);
  });
});
