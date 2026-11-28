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

  it('orders the default priority view as a risk list: alerts, incidents, fullest, then A to Z', () => {
    const usage = (usagePercent: number) => ({
      totalBytes: 100,
      usedBytes: usagePercent,
      freeBytes: 100 - usagePercent,
      usagePercent,
    });
    const records = [
      makeRecord({ id: 'quiet-b', name: 'bravo', capacity: usage(40) }),
      makeRecord({ id: 'quiet-a', name: 'alpha', capacity: usage(40) }),
      makeRecord({ id: 'full', name: 'zulu', capacity: usage(80) }),
      makeRecord({ id: 'warning', name: 'yankee', capacity: usage(20) }),
      makeRecord({ id: 'critical', name: 'xray', capacity: usage(10) }),
      makeRecord({ id: 'incident', name: 'whiskey', capacity: usage(5), incidentPriority: 4 }),
    ];
    const alertWeights: Record<string, number> = { critical: 3, warning: 2 };
    expect(
      sortStorageRecords(records, 'priority', 'desc', {
        alertWeightByRecordId: (id) => alertWeights[id] ?? 0,
      }).map((record) => record.id),
    ).toEqual(['critical', 'warning', 'incident', 'full', 'quiet-a', 'quiet-b']);
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

  it('keeps both PVE pools and PBS datastores in the Proxmox workspace scope', () => {
    const pbs = makeRecord({
      id: 'pbs-datastore',
      source: {
        platform: 'proxmox-pbs',
        family: 'virtualization',
        origin: 'resource',
        adapterId: 'resource-storage',
      },
    });

    expect(
      filterStorageRecords([makeRecord(), pbs], {
        search: '',
        sourceFilter: 'proxmox-all',
        healthFilter: 'all',
        selectedNode: null,
      }).map((record) => record.id),
    ).toEqual(['storage-1', 'pbs-datastore']);
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

  it('filters, searches, sorts, groups, and summarizes stale storage pools truthfully', () => {
    const freshHighUsage = makeRecord({
      id: 'fresh-high',
      name: 'fresh-high',
      health: 'healthy',
      freshness: 'fresh',
      capacity: { totalBytes: 100, usedBytes: 85, freeBytes: 15, usagePercent: 85 },
      details: { status: 'online', node: 'pve1', nodeHints: ['cluster-main'] },
    });
    const stalePool = makeRecord({
      id: 'stale-low',
      name: 'stale-low',
      health: 'healthy',
      freshness: 'stale',
      freshnessError: 'Datastore.Audit permission denied',
      capacity: { totalBytes: 100, usedBytes: 25, freeBytes: 75, usagePercent: 25 },
      details: { status: 'online', node: 'pve1', nodeHints: ['cluster-main'] },
    });
    const staleFaulted = makeRecord({
      id: 'stale-faulted',
      name: 'stale-faulted',
      health: 'critical',
      freshness: 'stale',
      freshnessError: 'PVE poll stalled',
      details: { status: 'faulted', node: 'pve1', nodeHints: ['cluster-main'] },
    });

    const all = [freshHighUsage, stalePool, staleFaulted];

    expect(
      filterStorageRecords(all, {
        search: '',
        sourceFilter: 'all',
        healthFilter: 'healthy',
        selectedNode: null,
      }).map((r) => r.id),
    ).toEqual(['fresh-high']);

    expect(
      filterStorageRecords(all, {
        search: '',
        sourceFilter: 'all',
        healthFilter: 'attention',
        selectedNode: null,
      }).map((r) => r.id),
    ).toEqual(['stale-low', 'stale-faulted']);

    expect(
      filterStorageRecords(all, {
        search: '',
        sourceFilter: 'all',
        healthFilter: 'warning',
        selectedNode: null,
      }).map((r) => r.id),
    ).toEqual(['stale-low']);

    expect(matchesStorageRecordSearch(stalePool, 'stale')).toBe(true);
    expect(matchesStorageRecordSearch(stalePool, 'stale data')).toBe(true);
    expect(matchesStorageRecordSearch(stalePool, 'datastore.audit')).toBe(true);
    expect(matchesStorageRecordSearch(freshHighUsage, 'stale')).toBe(false);

    expect(
      sortStorageRecords([freshHighUsage, stalePool], 'priority', 'desc').map((r) => r.id),
    ).toEqual(['stale-low', 'fresh-high']);

    expect(
      groupStorageRecords([freshHighUsage, stalePool, staleFaulted], 'status').map((g) => g.key),
    ).toEqual(['faulted', 'online', 'stale']);

    expect(summarizeStorageRecords([freshHighUsage, stalePool]).byHealth).toMatchObject({
      healthy: 1,
      warning: 1,
    });
  });
});
