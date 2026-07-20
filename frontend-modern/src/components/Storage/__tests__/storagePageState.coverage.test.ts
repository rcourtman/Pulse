import { describe, expect, it } from 'vitest';
import type { StorageHealthFilter } from '@/features/storageBackups/models';
import {
  buildStorageRouteFields,
  coerceSelectedStorageNodeId,
  countActiveStorageFilters,
  DEFAULT_STORAGE_DISK_GROUP_FILTER,
  DEFAULT_STORAGE_DISK_ROLE_FILTER,
  DEFAULT_STORAGE_GROUP_KEY,
  DEFAULT_STORAGE_SELECTED_NODE_ID,
  DEFAULT_STORAGE_SORT_DIRECTION,
  DEFAULT_STORAGE_SORT_KEY,
  DEFAULT_STORAGE_SOURCE_FILTER,
  DEFAULT_STORAGE_STATUS_FILTER,
  getDefaultStorageSortDirection,
  hasActiveStorageFilters,
  normalizeStorageHealthFilter,
  normalizeStorageSortKey,
  type StoragePageNodeOption,
  type StorageFilterActivityState,
} from '@/components/Storage/storagePageState';

const baseInactiveState: StorageFilterActivityState = {
  search: '',
  sortKey: DEFAULT_STORAGE_SORT_KEY,
  sortDirection: DEFAULT_STORAGE_SORT_DIRECTION,
  groupBy: DEFAULT_STORAGE_GROUP_KEY,
  statusFilter: DEFAULT_STORAGE_STATUS_FILTER,
  sourceFilter: DEFAULT_STORAGE_SOURCE_FILTER,
  diskRoleFilter: DEFAULT_STORAGE_DISK_ROLE_FILTER,
  diskGroupFilter: DEFAULT_STORAGE_DISK_GROUP_FILTER,
};

describe('storagePageState coverage', () => {
  describe('normalizeStorageHealthFilter', () => {
    it('returns "all" for empty, whitespace, and null-ish inputs', () => {
      expect(normalizeStorageHealthFilter('')).toBe('all');
      expect(normalizeStorageHealthFilter('   ')).toBe('all');
    });

    it('returns "all" for the literal "all" token (case-insensitive, trimmed)', () => {
      expect(normalizeStorageHealthFilter('all')).toBe('all');
      expect(normalizeStorageHealthFilter(' All ')).toBe('all');
      expect(normalizeStorageHealthFilter('ALL')).toBe('all');
    });

    it.each<[string, StorageHealthFilter]>([
      ['attention', 'attention'],
      ['needs-attention', 'attention'],
      ['issue', 'attention'],
      ['issues', 'attention'],
      ['unhealthy', 'attention'],
    ])('maps %s alias to "attention"', (input, expected) => {
      expect(normalizeStorageHealthFilter(input)).toBe(expected);
    });

    it.each<[string, StorageHealthFilter]>([
      ['available', 'healthy'],
      ['online', 'healthy'],
      ['healthy', 'healthy'],
    ])('maps %s alias to "healthy"', (input, expected) => {
      expect(normalizeStorageHealthFilter(input)).toBe(expected);
    });

    it.each<[string, StorageHealthFilter]>([
      ['degraded', 'warning'],
      ['warning', 'warning'],
      ['critical', 'critical'],
      ['offline', 'offline'],
      ['unknown', 'unknown'],
    ])('maps %s to its canonical bucket', (input, expected) => {
      expect(normalizeStorageHealthFilter(input)).toBe(expected);
    });

    it('is case-insensitive and trims whitespace', () => {
      expect(normalizeStorageHealthFilter('  CRITICAL  ')).toBe('critical');
      expect(normalizeStorageHealthFilter('Offline')).toBe('offline');
      expect(normalizeStorageHealthFilter('  Unhealthy  ')).toBe('attention');
    });

    it('falls back to "all" for unrecognized tokens', () => {
      expect(normalizeStorageHealthFilter('banana')).toBe('all');
      expect(normalizeStorageHealthFilter('error')).toBe('all');
    });
  });

  describe('normalizeStorageSortKey', () => {
    it.each(['priority', 'name', 'state', 'usage', 'type', 'host', 'protection', 'growth'])(
      'accepts canonical sort key %s (trimmed, lowercased)',
      (key) => {
        expect(normalizeStorageSortKey(key)).toBe(key);
        expect(normalizeStorageSortKey(` ${key.toUpperCase()} `)).toBe(key);
      },
    );

    it('falls back to "priority" for empty / whitespace input', () => {
      expect(normalizeStorageSortKey('')).toBe('priority');
      expect(normalizeStorageSortKey('   ')).toBe('priority');
    });

    it('falls back to "priority" for unknown keys', () => {
      expect(normalizeStorageSortKey('banana')).toBe('priority');
      expect(normalizeStorageSortKey('random')).toBe('priority');
    });

    it('folds the legacy "source" key back to the default "priority"', () => {
      expect(normalizeStorageSortKey('source')).toBe('priority');
    });
  });

  describe('getDefaultStorageSortDirection', () => {
    it('returns "desc" for priority, usage, and growth', () => {
      expect(getDefaultStorageSortDirection('priority')).toBe('desc');
      expect(getDefaultStorageSortDirection('usage')).toBe('desc');
      expect(getDefaultStorageSortDirection('growth')).toBe('desc');
    });

    it('returns "asc" for name, state, type, host, and protection', () => {
      expect(getDefaultStorageSortDirection('name')).toBe('asc');
      expect(getDefaultStorageSortDirection('state')).toBe('asc');
      expect(getDefaultStorageSortDirection('type')).toBe('asc');
      expect(getDefaultStorageSortDirection('host')).toBe('asc');
      expect(getDefaultStorageSortDirection('protection')).toBe('asc');
    });
  });

  describe('countActiveStorageFilters', () => {
    it('returns 0 when every field is at its default', () => {
      expect(countActiveStorageFilters({ ...baseInactiveState })).toBe(0);
    });

    it('counts only non-whitespace search', () => {
      expect(countActiveStorageFilters({ ...baseInactiveState, search: '   ' })).toBe(0);
      expect(countActiveStorageFilters({ ...baseInactiveState, search: 'tank' })).toBe(1);
    });

    it('counts groupBy when not "none"', () => {
      expect(countActiveStorageFilters({ ...baseInactiveState, groupBy: 'node' })).toBe(1);
      expect(countActiveStorageFilters({ ...baseInactiveState, groupBy: 'type' })).toBe(1);
    });

    it('counts statusFilter when not "all"', () => {
      expect(countActiveStorageFilters({ ...baseInactiveState, statusFilter: 'critical' })).toBe(1);
    });

    it('counts sourceFilter when not "all"', () => {
      expect(countActiveStorageFilters({ ...baseInactiveState, sourceFilter: 'agent' })).toBe(1);
    });

    it('counts diskRoleFilter when not at default', () => {
      expect(countActiveStorageFilters({ ...baseInactiveState, diskRoleFilter: 'nvme-disk' })).toBe(
        1,
      );
    });

    it('counts diskGroupFilter when not at default', () => {
      expect(
        countActiveStorageFilters({ ...baseInactiveState, diskGroupFilter: 'data-pool' }),
      ).toBe(1);
    });

    it('sums to 6 when all six filter dimensions are active', () => {
      expect(
        countActiveStorageFilters({
          search: 'tank',
          sortKey: 'priority',
          sortDirection: 'desc',
          groupBy: 'node',
          statusFilter: 'critical',
          sourceFilter: 'agent',
          diskRoleFilter: 'nvme-disk',
          diskGroupFilter: 'data-pool',
        }),
      ).toBe(6);
    });
  });

  describe('hasActiveStorageFilters', () => {
    it('returns false at full default state', () => {
      expect(hasActiveStorageFilters({ ...baseInactiveState })).toBe(false);
    });

    it('returns true for non-whitespace search', () => {
      expect(hasActiveStorageFilters({ ...baseInactiveState, search: 'tank' })).toBe(true);
    });

    it('returns false for whitespace-only search', () => {
      expect(hasActiveStorageFilters({ ...baseInactiveState, search: '   ' })).toBe(false);
    });

    it('returns true when sortKey differs from default', () => {
      expect(hasActiveStorageFilters({ ...baseInactiveState, sortKey: 'name' })).toBe(true);
    });

    it('returns true when sortDirection differs from default', () => {
      expect(hasActiveStorageFilters({ ...baseInactiveState, sortDirection: 'asc' })).toBe(true);
    });

    it('returns true when groupBy differs from default', () => {
      expect(hasActiveStorageFilters({ ...baseInactiveState, groupBy: 'type' })).toBe(true);
    });

    it('returns true when statusFilter differs from default', () => {
      expect(hasActiveStorageFilters({ ...baseInactiveState, statusFilter: 'warning' })).toBe(true);
    });

    it('returns true when sourceFilter differs from default', () => {
      expect(hasActiveStorageFilters({ ...baseInactiveState, sourceFilter: 'agent' })).toBe(true);
    });

    it('returns true when diskRoleFilter differs from default', () => {
      expect(hasActiveStorageFilters({ ...baseInactiveState, diskRoleFilter: 'nvme-disk' })).toBe(
        true,
      );
    });

    it('returns true when diskGroupFilter differs from default', () => {
      expect(hasActiveStorageFilters({ ...baseInactiveState, diskGroupFilter: 'data-pool' })).toBe(
        true,
      );
    });
  });

  describe('coerceSelectedStorageNodeId', () => {
    const nodeOptions: StoragePageNodeOption[] = [
      { id: 'node-1', label: 'pve1' },
      { id: 'node-2', label: 'pve2' },
    ];

    it('returns "all" unchanged', () => {
      expect(coerceSelectedStorageNodeId('all', nodeOptions)).toBe('all');
    });

    it('returns the id when it matches a known node option', () => {
      expect(coerceSelectedStorageNodeId('node-1', nodeOptions)).toBe('node-1');
      expect(coerceSelectedStorageNodeId('node-2', nodeOptions)).toBe('node-2');
    });

    it('falls back to "all" when the id is not in nodeOptions', () => {
      expect(coerceSelectedStorageNodeId('missing', nodeOptions)).toBe('all');
    });

    it('falls back to "all" when nodeOptions is empty', () => {
      expect(coerceSelectedStorageNodeId('node-1', [])).toBe('all');
    });
  });

  describe('route serde round-trip', () => {
    const noop = () => {};
    const makeFields = () =>
      buildStorageRouteFields({
        view: () => 'pools',
        setView: noop,
        sourceFilter: () => 'all',
        setSourceFilter: noop,
        healthFilter: () => 'all',
        setHealthFilter: noop,
        diskRoleFilter: () => 'all',
        setDiskRoleFilter: noop,
        diskGroupFilter: () => 'all',
        setDiskGroupFilter: noop,
        selectedNodeId: () => 'all',
        setSelectedNodeId: noop,
        groupBy: () => 'none',
        setGroupBy: noop,
        sortKey: () => 'priority',
        setSortKey: noop,
        sortDirection: () => 'desc',
        setSortDirection: noop,
        search: () => '',
        setSearch: noop,
      });

    // --- normalizeStorageRouteSelection (private, covered via node field) ---

    it('node.read returns "all" for "all" in any case, trimmed', () => {
      const fields = makeFields();
      expect(fields.node!.read({ node: 'all' } as any)).toBe(DEFAULT_STORAGE_SELECTED_NODE_ID);
      expect(fields.node!.read({ node: 'ALL' } as any)).toBe(DEFAULT_STORAGE_SELECTED_NODE_ID);
    });

    it('node.read returns the trimmed id (case preserved) for non-"all" values', () => {
      const fields = makeFields();
      expect(fields.node!.read({ node: 'node-9' } as any)).toBe('node-9');
      expect(fields.node!.read({ node: '  Node-ABC  ' } as any)).toBe('Node-ABC');
    });

    it('node.read falls back to "all" when normalizeStorageRouteSelection yields empty', () => {
      const fields = makeFields();
      expect(fields.node!.read({ node: '' } as any)).toBe(DEFAULT_STORAGE_SELECTED_NODE_ID);
      expect(fields.node!.read({ node: '   ' } as any)).toBe(DEFAULT_STORAGE_SELECTED_NODE_ID);
      expect(fields.node!.read({ node: undefined } as any)).toBe(DEFAULT_STORAGE_SELECTED_NODE_ID);
    });

    it('node.write returns null when the value resolves to "all"', () => {
      const fields = makeFields();
      expect(fields.node!.write?.('all')).toBeNull();
      expect(fields.node!.write?.('  ')).toBeNull();
    });

    it('node.write returns the resolved id when it is not "all"', () => {
      const fields = makeFields();
      expect(fields.node!.write?.('node-9')).toBe('node-9');
    });

    // --- unknown / invalid string fallbacks to defaults ---

    it('tab.read falls back to "pools" for unknown string', () => {
      const fields = makeFields();
      expect(fields.tab!.read({ tab: 'unknown' } as any)).toBe('pools');
      expect(fields.tab!.read({ tab: 'weird' } as any)).toBe('pools');
    });

    it('source.read falls back to "all" for empty / undefined / whitespace', () => {
      const fields = makeFields();
      expect(fields.source!.read({ source: '' } as any)).toBe('all');
      expect(fields.source!.read({ source: undefined } as any)).toBe('all');
      expect(fields.source!.read({ source: '   ' } as any)).toBe('all');
    });

    it('status.read falls back to "all" for unknown string and undefined', () => {
      const fields = makeFields();
      expect(fields.status!.read({ status: 'banana' } as any)).toBe('all');
      expect(fields.status!.read({ status: undefined } as any)).toBe('all');
      expect(fields.status!.read({} as any)).toBe('all');
    });

    it('diskRole.read falls back to default for undefined / empty', () => {
      const fields = makeFields();
      expect(fields.diskRole!.read({ diskRole: undefined } as any)).toBe(
        DEFAULT_STORAGE_DISK_ROLE_FILTER,
      );
      expect(fields.diskRole!.read({ diskRole: '   ' } as any)).toBe(
        DEFAULT_STORAGE_DISK_ROLE_FILTER,
      );
    });

    it('diskGroup.read falls back to default for undefined / empty', () => {
      const fields = makeFields();
      expect(fields.diskGroup!.read({ diskGroup: undefined } as any)).toBe(
        DEFAULT_STORAGE_DISK_GROUP_FILTER,
      );
      expect(fields.diskGroup!.read({ diskGroup: '   ' } as any)).toBe(
        DEFAULT_STORAGE_DISK_GROUP_FILTER,
      );
    });

    it('group.read falls back to "none" for unknown string', () => {
      const fields = makeFields();
      expect(fields.group!.read({ group: 'banana' } as any)).toBe('none');
    });

    it('sort.read falls back to "priority" for unknown string', () => {
      const fields = makeFields();
      expect(fields.sort!.read({ sort: 'banana' } as any)).toBe('priority');
    });

    it('order.read falls back to "desc" for unknown string', () => {
      const fields = makeFields();
      expect(fields.order!.read({ order: 'banana' } as any)).toBe('desc');
    });

    it('query.read falls back to empty string for undefined', () => {
      const fields = makeFields();
      expect(fields.query!.read({ query: undefined } as any)).toBe('');
      expect(fields.query!.read({} as any)).toBe('');
    });

    // --- write serde round-trip: default values serialize to null ---

    it('source.write returns null for the default "all"', () => {
      const fields = makeFields();
      expect(fields.source!.write?.('all')).toBeNull();
    });

    it('group.write returns null for "none"', () => {
      const fields = makeFields();
      expect(fields.group!.write?.('none')).toBeNull();
    });

    it('sort.write returns null for "priority"', () => {
      const fields = makeFields();
      expect(fields.sort!.write?.('priority')).toBeNull();
    });

    it('order.write returns null for "desc"', () => {
      const fields = makeFields();
      expect(fields.order!.write?.('desc')).toBeNull();
    });
  });
});
