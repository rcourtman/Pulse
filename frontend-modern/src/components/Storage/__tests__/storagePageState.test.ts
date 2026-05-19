import { createSignal } from 'solid-js';
import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import type { StorageHealthFilter, StorageRecord } from '@/features/storageBackups/models';
import type { StorageGroupKey, StorageSortKey } from '@/components/Storage/useStorageModel';
import {
  buildStorageNodeFilterOptions,
  buildStorageNodeOnlineByLabel,
  buildStorageNodeOptions,
  buildStorageRouteFields,
  coerceSelectedStorageNodeId,
  countActiveStorageFilters,
  countVisiblePhysicalDisksForNode,
  DEFAULT_STORAGE_DISK_GROUP_FILTER,
  DEFAULT_STORAGE_DISK_ROLE_FILTER,
  DEFAULT_STORAGE_GROUP_KEY,
  DEFAULT_STORAGE_SORT_OPTIONS,
  DEFAULT_STORAGE_SORT_DIRECTION,
  DEFAULT_STORAGE_SORT_KEY,
  DEFAULT_STORAGE_SOURCE_FILTER,
  DEFAULT_STORAGE_STATUS_FILTER,
  DEFAULT_STORAGE_VIEW,
  filterStorageDiskNodeOptions,
  getStorageFilterGroupBy,
  getStorageNodeFilterLabel,
  getStorageStatusFilterValue,
  hasActiveStorageFilters,
  isStorageRecordCeph,
  normalizeStorageGroupKey,
  normalizeStorageHealthFilter,
  normalizeStorageSortDirection,
  normalizeStorageSortKey,
  STORAGE_STATUS_FILTER_OPTIONS,
  STORAGE_GROUP_BY_OPTIONS,
  normalizeStorageView,
  readStorageRouteValue,
  storageResourceMatchesSourceFilter,
  syncExpandedStorageGroups,
  toggleExpandedStorageGroup,
  toStorageHealthFilterValue,
  writeStorageRouteValue,
} from '@/components/Storage/storagePageState';

const makeNode = (overrides: Partial<Resource> = {}): Resource =>
  ({
    id: 'node-1',
    type: 'agent',
    name: 'pve1',
    displayName: 'pve1',
    platformId: 'cluster-main',
    platformType: 'proxmox-pve',
    sourceType: 'api',
    status: 'online',
    uptime: 1000,
    lastSeen: Date.now(),
    platformData: { proxmox: { instance: 'cluster-main' } },
    ...overrides,
  }) as Resource;

const makeDisk = (overrides: Partial<Resource> = {}): Resource =>
  ({
    id: 'disk-1',
    type: 'physical_disk',
    name: '/dev/sda',
    displayName: '/dev/sda',
    platformId: 'cluster-main',
    platformType: 'proxmox-pve',
    sourceType: 'api',
    parentId: 'node-1',
    status: 'online',
    lastSeen: Date.now(),
    identity: { hostname: 'pve1' },
    canonicalIdentity: { hostname: 'pve1' },
    platformData: {
      proxmox: { nodeName: 'pve1', instance: 'cluster-main' },
      physicalDisk: { devPath: '/dev/sda', model: 'Disk', health: 'PASSED' },
    },
    ...overrides,
  }) as Resource;

const makeStorageRecord = (overrides: Partial<StorageRecord> = {}): StorageRecord => ({
  id: 'storage-1',
  name: 'ceph-pool',
  category: 'pool',
  health: 'healthy',
  location: { label: 'pve1', scope: 'node' },
  capacity: { totalBytes: 100, usedBytes: 40, freeBytes: 60, usagePercent: 40 },
  capabilities: ['capacity', 'replication'],
  source: {
    platform: 'proxmox-pve',
    family: 'virtualization',
    origin: 'resource',
    adapterId: 'resource-storage',
  },
  observedAt: Date.now(),
  details: { type: 'rbd' },
  ...overrides,
});

describe('storagePageState', () => {
  it('normalizes storage query state canonically', () => {
    expect(normalizeStorageHealthFilter('available')).toBe('healthy');
    expect(normalizeStorageHealthFilter('needs-attention')).toBe('attention');
    expect(normalizeStorageSortKey(' usage ')).toBe('usage');
    expect(normalizeStorageSortKey('usage')).toBe('usage');
    expect(normalizeStorageSortKey('weird')).toBe('priority');
    expect(normalizeStorageGroupKey(' status ')).toBe('status');
    expect(normalizeStorageGroupKey('status')).toBe('status');
    expect(normalizeStorageGroupKey('weird')).toBe('none');
    expect(normalizeStorageView(' disks ')).toBe('disks');
    expect(normalizeStorageView('disks')).toBe('disks');
    expect(normalizeStorageView('other')).toBe('pools');
    expect(normalizeStorageSortDirection(' asc ')).toBe('asc');
    expect(normalizeStorageSortDirection('asc')).toBe('asc');
    expect(normalizeStorageSortDirection('bad')).toBe('desc');
    expect(getStorageFilterGroupBy('node')).toBe('node');
    expect(getStorageStatusFilterValue('healthy')).toBe('available');
    expect(getStorageStatusFilterValue('attention')).toBe('attention');
    expect(toStorageHealthFilterValue('available')).toBe('healthy');
    expect(toStorageHealthFilterValue('attention')).toBe('attention');
    expect(getStorageNodeFilterLabel('pools')).toBe('All nodes');
    expect(getStorageNodeFilterLabel('disks')).toBe('All disk hosts');
    expect(DEFAULT_STORAGE_VIEW).toBe('pools');
    expect(DEFAULT_STORAGE_SOURCE_FILTER).toBe('all');
    expect(DEFAULT_STORAGE_DISK_ROLE_FILTER).toBe('all');
    expect(DEFAULT_STORAGE_DISK_GROUP_FILTER).toBe('all');
    expect(DEFAULT_STORAGE_SORT_KEY).toBe('priority');
    expect(DEFAULT_STORAGE_SORT_DIRECTION).toBe('desc');
    expect(DEFAULT_STORAGE_GROUP_KEY).toBe('none');
    expect(DEFAULT_STORAGE_STATUS_FILTER).toBe('all');
    expect(DEFAULT_STORAGE_SORT_OPTIONS.map((option) => option.value)).toEqual([
      'priority',
      'name',
      'usage',
      'type',
    ]);
    expect(STORAGE_STATUS_FILTER_OPTIONS.map((option) => option.value)).toEqual([
      'all',
      'attention',
      'available',
      'warning',
      'critical',
      'offline',
      'unknown',
    ]);
    expect(STORAGE_GROUP_BY_OPTIONS.map((option) => option.value)).toEqual([
      'none',
      'node',
      'type',
      'status',
    ]);
  });

  it('derives canonical ceph and node state helpers', () => {
    const record = makeStorageRecord();
    const nodes = [
      makeNode(),
      makeNode({ id: 'node-2', name: 'pve2', status: 'offline', uptime: 0 }),
    ];
    const nodeOptions = buildStorageNodeOptions(nodes);
    const diskNodeOptions = filterStorageDiskNodeOptions(nodeOptions, [makeDisk()]);

    expect(isStorageRecordCeph(record)).toBe(true);
    expect(diskNodeOptions.map((node) => node.label)).toEqual(['pve1']);
    expect(buildStorageNodeOnlineByLabel(nodes)).toEqual(
      new Map([
        ['pve1', true],
        ['pve2', false],
      ]),
    );
    expect(countVisiblePhysicalDisksForNode('node-1', nodeOptions, [makeDisk()])).toBe(1);
    expect(coerceSelectedStorageNodeId('node-1', nodeOptions)).toBe('node-1');
    expect(coerceSelectedStorageNodeId('missing', nodeOptions)).toBe('all');
    expect(buildStorageNodeFilterOptions('disks', nodeOptions)).toEqual([
      { value: 'all', label: 'All disk hosts' },
      { value: 'node-1', label: 'pve1' },
      { value: 'node-2', label: 'pve2' },
    ]);
  });

  it('matches storage node resources to the active source scope', () => {
    const node = makeNode({
      platformType: undefined,
      sourceType: 'api',
      platformData: { sources: ['proxmox-pve', 'agent'] },
    });

    expect(storageResourceMatchesSourceFilter(node, 'all')).toBe(true);
    expect(storageResourceMatchesSourceFilter(node, 'proxmox-pve')).toBe(true);
    expect(storageResourceMatchesSourceFilter(node, 'truenas')).toBe(false);
  });

  it('keeps expanded storage groups canonical across data refreshes', () => {
    expect(syncExpandedStorageGroups(new Set(), ['A', 'B'])).toEqual(new Set(['A', 'B']));
    expect(syncExpandedStorageGroups(new Set(['A']), ['A', 'B'])).toEqual(new Set(['A', 'B']));
    expect(toggleExpandedStorageGroup(new Set(['A']), 'A')).toEqual(new Set());
    expect(toggleExpandedStorageGroup(new Set(['A']), 'B')).toEqual(new Set(['A', 'B']));
  });

  it('derives active storage filters canonically', () => {
    expect(
      countActiveStorageFilters({
        search: 'tank',
        sortKey: 'priority',
        sortDirection: 'desc',
        groupBy: 'none',
        statusFilter: 'all',
        sourceFilter: 'all',
        diskRoleFilter: 'all',
        diskGroupFilter: 'all',
      }),
    ).toBe(1);

    expect(
      hasActiveStorageFilters({
        search: '',
        sortKey: 'priority',
        sortDirection: 'desc',
        groupBy: 'none',
        statusFilter: 'all',
        sourceFilter: 'all',
        diskRoleFilter: 'all',
        diskGroupFilter: 'all',
      }),
    ).toBe(false);

    expect(
      hasActiveStorageFilters({
        search: '',
        sortKey: 'name',
        sortDirection: 'desc',
        groupBy: 'none',
        statusFilter: 'all',
        sourceFilter: 'all',
        diskRoleFilter: 'all',
        diskGroupFilter: 'all',
      }),
    ).toBe(true);

    expect(
      countActiveStorageFilters({
        search: '',
        sortKey: 'priority',
        sortDirection: 'desc',
        groupBy: 'none',
        statusFilter: 'all',
        sourceFilter: 'all',
        diskRoleFilter: 'nvme-disk',
        diskGroupFilter: 'all',
      }),
    ).toBe(1);

    expect(readStorageRouteValue(undefined, 'all')).toBe('all');
    expect(readStorageRouteValue('   ', 'all')).toBe('all');
    expect(readStorageRouteValue(' warning ', 'all')).toBe('warning');
    expect(readStorageRouteValue('warning', 'all')).toBe('warning');
    expect(writeStorageRouteValue('all', 'all')).toBeNull();
    expect(writeStorageRouteValue('warning', 'all')).toBe('warning');
  });

  it('builds storage route fields from shared defaults and normalizers', () => {
    const [view, setView] = createSignal<'pools' | 'disks'>('pools');
    const [sourceFilter, setSourceFilter] = createSignal('all');
    const [healthFilter, setHealthFilter] = createSignal<StorageHealthFilter>('all');
    const [diskRoleFilter, setDiskRoleFilter] = createSignal('all');
    const [diskGroupFilter, setDiskGroupFilter] = createSignal('all');
    const [selectedNodeId, setSelectedNodeId] = createSignal('all');
    const [groupBy, setGroupBy] = createSignal<StorageGroupKey>('none');
    const [sortKey, setSortKey] = createSignal<StorageSortKey>('priority');
    const [sortDirection, setSortDirection] = createSignal<'asc' | 'desc'>('desc');
    const [search, setSearch] = createSignal('');

    const fields = buildStorageRouteFields({
      view,
      setView,
      sourceFilter,
      setSourceFilter,
      healthFilter,
      setHealthFilter,
      diskRoleFilter,
      setDiskRoleFilter,
      diskGroupFilter,
      setDiskGroupFilter,
      selectedNodeId,
      setSelectedNodeId,
      groupBy,
      setGroupBy,
      sortKey,
      setSortKey,
      sortDirection,
      setSortDirection,
      search,
      setSearch,
    });

    expect(fields.tab?.read({ tab: ' disks ' } as any)).toBe('disks');
    expect(fields.tab?.write?.('pools')).toBeNull();
    expect(fields.source?.read({ source: ' agent ' } as any)).toBe('agent');
    expect(fields.source?.read({ source: ' PVE ' } as any)).toBe('proxmox-pve');
    expect(fields.source?.read({ source: ' ALL ' } as any)).toBe('all');
    expect(fields.source?.write?.('all')).toBeNull();
    expect(fields.source?.write?.(' PVE ' as any)).toBe('proxmox-pve');
    expect(fields.status?.read({ status: 'available' } as any)).toBe('healthy');
    expect(fields.status?.read({ status: 'attention' } as any)).toBe('attention');
    expect(fields.status?.write?.('healthy')).toBe('available');
    expect(fields.status?.write?.('attention')).toBe('attention');
    expect(fields.status?.write?.('warning')).toBe('warning');
    expect(fields.status?.write?.('all')).toBeNull();
    expect(fields.diskRole?.read({ diskRole: ' NVME Disk ' } as any)).toBe('nvme-disk');
    expect(fields.diskRole?.write?.('nvme-disk')).toBe('nvme-disk');
    expect(fields.diskRole?.write?.('all')).toBeNull();
    expect(fields.diskGroup?.read({ diskGroup: ' Data Pool ' } as any)).toBe('data-pool');
    expect(fields.diskGroup?.write?.('data-pool')).toBe('data-pool');
    expect(fields.diskGroup?.write?.('all')).toBeNull();
    expect(fields.node?.read({ node: ' node-1 ' } as any)).toBe('node-1');
    expect(fields.node?.read({ node: ' ALL ' } as any)).toBe('all');
    expect(fields.node?.write?.('all')).toBeNull();
    expect(fields.node?.write?.(' ALL ' as any)).toBeNull();
    expect(fields.group?.read({ group: ' status ' } as any)).toBe('status');
    expect(fields.sort?.read({ sort: ' usage ' } as any)).toBe('usage');
    expect(fields.order?.read({ order: ' asc ' } as any)).toBe('asc');
    expect(fields.query?.read({ query: '  tank  ' } as any)).toBe('tank');
    expect(fields.query?.write?.('  tank  ')).toBe('tank');
    expect(fields.query?.write?.('   ')).toBeNull();
  });
});
