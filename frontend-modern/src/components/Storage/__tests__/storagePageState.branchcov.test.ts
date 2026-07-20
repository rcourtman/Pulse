import { describe, expect, it, vi } from 'vitest';
import type { Resource } from '@/types/resource';
import type { StorageRecord } from '@/features/storageBackups/models';
import type { ParsedStorageSearch } from '@/components/Storage/useStorageRouteState';
import {
  buildStorageNodeOnlineByLabel,
  buildStorageRouteFields,
  countActiveStorageFilters,
  countVisiblePhysicalDisksForNode,
  DEFAULT_STORAGE_DISK_GROUP_FILTER,
  DEFAULT_STORAGE_DISK_ROLE_FILTER,
  DEFAULT_STORAGE_GROUP_KEY,
  DEFAULT_STORAGE_SORT_DIRECTION,
  DEFAULT_STORAGE_SORT_KEY,
  DEFAULT_STORAGE_SOURCE_FILTER,
  DEFAULT_STORAGE_STATUS_FILTER,
  getStorageMetaBoolean,
  hasActiveStorageFilters,
  isStorageRecordCeph,
  toStorageHealthFilterValue,
  type StorageFilterActivityState,
  type StoragePageNodeOption,
  type StorageStatusFilterValue,
} from '@/components/Storage/storagePageState';

const makeResource = (overrides: Partial<Resource> = {}): Resource =>
  ({
    id: 'node-1',
    type: 'agent',
    name: 'pve1',
    displayName: 'pve1',
    platformId: 'cluster-main',
    platformType: 'proxmox-pve',
    sourceType: 'api',
    status: 'online',
    uptime: 1,
    lastSeen: 0,
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
    lastSeen: 0,
    ...overrides,
  }) as Resource;

const makeStorageRecord = (overrides: Partial<StorageRecord> = {}): StorageRecord => ({
  id: 'storage-1',
  name: 'pool-1',
  category: 'pool',
  health: 'healthy',
  location: { label: 'pve1', scope: 'node' },
  capacity: { totalBytes: 100, usedBytes: 40, freeBytes: 60, usagePercent: 40 },
  capabilities: [],
  source: {
    platform: 'truenas',
    family: 'generic',
    origin: 'resource',
    adapterId: 'adapter-1',
  },
  observedAt: 0,
  ...overrides,
});

const makeParsed = (overrides: Partial<ParsedStorageSearch> = {}): ParsedStorageSearch => ({
  tab: '',
  group: '',
  source: '',
  status: '',
  diskRole: '',
  diskGroup: '',
  node: '',
  query: '',
  resource: '',
  sort: '',
  order: '',
  summaryGroup: '',
  ...overrides,
});

const buildFields = () =>
  buildStorageRouteFields({
    view: () => 'pools',
    setView: () => {},
    sourceFilter: () => 'all',
    setSourceFilter: () => {},
    healthFilter: () => 'all',
    setHealthFilter: () => {},
    diskRoleFilter: () => 'all',
    setDiskRoleFilter: () => {},
    diskGroupFilter: () => 'all',
    setDiskGroupFilter: () => {},
    selectedNodeId: () => 'all',
    setSelectedNodeId: () => {},
    groupBy: () => 'none',
    setGroupBy: () => {},
    sortKey: () => 'priority',
    setSortKey: () => {},
    sortDirection: () => 'desc',
    setSortDirection: () => {},
    search: () => '',
    setSearch: () => {},
  });

describe('storagePageState branch coverage', () => {
  describe('toStorageHealthFilterValue', () => {
    it.each<[StorageStatusFilterValue, ReturnType<typeof toStorageHealthFilterValue>]>([
      ['warning', 'warning'],
      ['critical', 'critical'],
      ['offline', 'offline'],
      ['unknown', 'unknown'],
    ])('passes %s through unchanged (fallthrough arm)', (input, expected) => {
      expect(toStorageHealthFilterValue(input)).toBe(expected);
    });
  });

  describe('countActiveStorageFilters', () => {
    // Only the required fields are populated; every optional filter is left
    // undefined so the `|| <default>` fallback arm fires for each dimension.
    const baseRequiredOnly: StorageFilterActivityState = {
      search: '',
      sortKey: DEFAULT_STORAGE_SORT_KEY,
      sortDirection: DEFAULT_STORAGE_SORT_DIRECTION,
    };

    it('returns 0 when every optional field is undefined (fallback arms all fire)', () => {
      expect(countActiveStorageFilters({ ...baseRequiredOnly })).toBe(0);
    });

    it('counts a non-whitespace search even when optional fields are undefined', () => {
      expect(countActiveStorageFilters({ ...baseRequiredOnly, search: 'tank' })).toBe(1);
    });

    it('ignores whitespace-only search', () => {
      expect(countActiveStorageFilters({ ...baseRequiredOnly, search: '\t \n' })).toBe(0);
    });

    it.each<[string, Partial<StorageFilterActivityState>]>([
      ['groupBy', { groupBy: 'node' }],
      ['statusFilter', { statusFilter: 'critical' }],
      ['sourceFilter', { sourceFilter: 'agent' }],
      ['diskRoleFilter', { diskRoleFilter: 'nvme-disk' }],
      ['diskGroupFilter', { diskGroupFilter: 'data-pool' }],
    ])('counts an active %s while the remaining optionals stay undefined', (_field, patch) => {
      expect(countActiveStorageFilters({ ...baseRequiredOnly, ...patch })).toBe(1);
    });

    it('accumulates across an intermediate mix of active filters (3 of 6)', () => {
      expect(
        countActiveStorageFilters({
          ...baseRequiredOnly,
          search: 'tank',
          groupBy: 'node',
          statusFilter: 'warning',
        }),
      ).toBe(3);
    });

    it('accumulates across four active filters including both disk facets', () => {
      expect(
        countActiveStorageFilters({
          ...baseRequiredOnly,
          search: 'tank',
          groupBy: 'type',
          diskRoleFilter: 'nvme-disk',
          diskGroupFilter: 'data-pool',
        }),
      ).toBe(4);
    });

    it('trims leading/trailing whitespace before deciding search is active', () => {
      expect(countActiveStorageFilters({ ...baseRequiredOnly, search: '   tank   ' })).toBe(1);
    });
  });

  describe('hasActiveStorageFilters', () => {
    const baseRequiredOnly: StorageFilterActivityState = {
      search: '',
      sortKey: DEFAULT_STORAGE_SORT_KEY,
      sortDirection: DEFAULT_STORAGE_SORT_DIRECTION,
    };

    it('returns false when every optional field is undefined and required fields are default', () => {
      expect(hasActiveStorageFilters({ ...baseRequiredOnly })).toBe(false);
    });

    it.each<[string, Partial<StorageFilterActivityState>]>([
      ['search', { search: 'tank' }],
      ['sortKey', { sortKey: 'name' }],
      ['sortDirection', { sortDirection: 'asc' }],
      ['groupBy', { groupBy: 'node' }],
      ['statusFilter', { statusFilter: 'warning' }],
      ['sourceFilter', { sourceFilter: 'agent' }],
      ['diskRoleFilter', { diskRoleFilter: 'nvme-disk' }],
      ['diskGroupFilter', { diskGroupFilter: 'data-pool' }],
    ])('returns true when only %s deviates from default', (_field, patch) => {
      expect(hasActiveStorageFilters({ ...baseRequiredOnly, ...patch })).toBe(true);
    });

    it('returns false when every optional field is explicitly set to its default', () => {
      expect(
        hasActiveStorageFilters({
          ...baseRequiredOnly,
          groupBy: DEFAULT_STORAGE_GROUP_KEY,
          statusFilter: DEFAULT_STORAGE_STATUS_FILTER,
          sourceFilter: DEFAULT_STORAGE_SOURCE_FILTER,
          diskRoleFilter: DEFAULT_STORAGE_DISK_ROLE_FILTER,
          diskGroupFilter: DEFAULT_STORAGE_DISK_GROUP_FILTER,
        }),
      ).toBe(false);
    });
  });

  describe('buildStorageNodeOnlineByLabel', () => {
    it('returns an empty map for an empty node list', () => {
      expect(buildStorageNodeOnlineByLabel([])).toEqual(new Map<string, boolean>());
    });

    it('skips nodes whose name is empty or whitespace-only', () => {
      expect(buildStorageNodeOnlineByLabel([makeResource({ name: '' })])).toEqual(
        new Map<string, boolean>(),
      );
      expect(buildStorageNodeOnlineByLabel([makeResource({ name: '   ' })])).toEqual(
        new Map<string, boolean>(),
      );
    });

    it('skips nodes that report neither a status string nor an uptime number', () => {
      const node = makeResource({ name: 'host-a', status: undefined, uptime: undefined });
      expect(buildStorageNodeOnlineByLabel([node])).toEqual(new Map<string, boolean>());
    });

    it('marks a node online only when status is "online" AND uptime is a positive number', () => {
      expect(
        buildStorageNodeOnlineByLabel([
          makeResource({ name: 'host-a', status: 'online', uptime: 1 }),
        ]),
      ).toEqual(new Map([['host-a', true]]));
    });

    it('marks a node NOT online when status is "online" but uptime is missing', () => {
      // status alone is insufficient: (uptime || 0) > 0 must also hold.
      expect(
        buildStorageNodeOnlineByLabel([
          makeResource({ name: 'host-a', status: 'online', uptime: undefined }),
        ]),
      ).toEqual(new Map([['host-a', false]]));
    });

    it('marks a node NOT online when status is "online" but uptime is 0 (boundary)', () => {
      expect(
        buildStorageNodeOnlineByLabel([
          makeResource({ name: 'host-a', status: 'online', uptime: 0 }),
        ]),
      ).toEqual(new Map([['host-a', false]]));
    });

    it('marks a node NOT online when uptime is present but status is not "online"', () => {
      expect(
        buildStorageNodeOnlineByLabel([
          makeResource({ name: 'host-a', status: 'offline', uptime: 500 }),
        ]),
      ).toEqual(new Map([['host-a', false]]));
    });

    it('enters the map (as false) when uptime is 0 but status is missing', () => {
      // uptime 0 is still typeof 'number', so the node is not skipped; the
      // online predicate evaluates to false because status !== 'online'.
      expect(
        buildStorageNodeOnlineByLabel([
          makeResource({ name: 'host-a', status: undefined, uptime: 0 }),
        ]),
      ).toEqual(new Map([['host-a', false]]));
    });

    it('treats a whitespace-only status string as no status', () => {
      expect(
        buildStorageNodeOnlineByLabel([
          makeResource({
            name: 'host-a',
            status: '   ',
            uptime: 100,
          } as unknown as Partial<Resource>),
        ]),
      ).toEqual(new Map([['host-a', false]]));
    });

    it('normalizes the map key to trimmed lowercase', () => {
      expect(
        buildStorageNodeOnlineByLabel([
          makeResource({ name: '  PVE-1  ', status: 'online', uptime: 10 }),
        ]),
      ).toEqual(new Map([['pve-1', true]]));
    });

    it('lets a later node with the same normalized label overwrite an earlier entry', () => {
      const nodes = [
        makeResource({ id: 'a', name: 'dup', status: 'online', uptime: 5 }),
        makeResource({ id: 'b', name: 'DUP', status: 'offline', uptime: 0 }),
      ];
      expect(buildStorageNodeOnlineByLabel(nodes)).toEqual(new Map([['dup', false]]));
    });
  });

  describe('getStorageMetaBoolean', () => {
    it('returns null when details is undefined', () => {
      expect(getStorageMetaBoolean(makeStorageRecord({ details: undefined }), 'isCeph')).toBeNull();
    });

    it('returns null when details is null (falsy-coalesces to {})', () => {
      const record = makeStorageRecord({
        details: null as unknown as Record<string, unknown>,
      });
      expect(getStorageMetaBoolean(record, 'isCeph')).toBeNull();
    });

    it('returns null when details is an empty object', () => {
      expect(getStorageMetaBoolean(makeStorageRecord({ details: {} }), 'isZfs')).toBeNull();
    });

    it('returns null when the requested key is absent', () => {
      expect(
        getStorageMetaBoolean(makeStorageRecord({ details: { other: 1 } }), 'isCeph'),
      ).toBeNull();
    });

    it('returns the stored boolean for isCeph (true and false)', () => {
      expect(
        getStorageMetaBoolean(makeStorageRecord({ details: { isCeph: true } }), 'isCeph'),
      ).toBe(true);
      expect(
        getStorageMetaBoolean(makeStorageRecord({ details: { isCeph: false } }), 'isCeph'),
      ).toBe(false);
    });

    it('returns the stored boolean for isZfs (true and false)', () => {
      expect(getStorageMetaBoolean(makeStorageRecord({ details: { isZfs: true } }), 'isZfs')).toBe(
        true,
      );
      expect(getStorageMetaBoolean(makeStorageRecord({ details: { isZfs: false } }), 'isZfs')).toBe(
        false,
      );
    });

    it('returns null for non-boolean values (string, number, null)', () => {
      expect(
        getStorageMetaBoolean(makeStorageRecord({ details: { isCeph: 'true' } }), 'isCeph'),
      ).toBeNull();
      expect(
        getStorageMetaBoolean(makeStorageRecord({ details: { isCeph: 1 } }), 'isCeph'),
      ).toBeNull();
      expect(
        getStorageMetaBoolean(makeStorageRecord({ details: { isCeph: null } }), 'isCeph'),
      ).toBeNull();
    });
  });

  describe('isStorageRecordCeph', () => {
    it('short-circuits to true when details.isCeph is the boolean true', () => {
      // The underlying type/category would otherwise read as non-ceph; the
      // meta flag must win.
      const record = makeStorageRecord({
        details: { isCeph: true, type: 'zfs' },
        capabilities: [],
        source: { platform: 'truenas', family: 'generic', origin: 'resource', adapterId: 'a' },
      });
      expect(isStorageRecordCeph(record)).toBe(true);
    });

    it('short-circuits to false when details.isCeph is the boolean false even if the type is ceph', () => {
      const record = makeStorageRecord({
        details: { isCeph: false, type: 'rbd' },
        capabilities: ['replication'],
        source: {
          platform: 'proxmox-pve',
          family: 'virtualization',
          origin: 'resource',
          adapterId: 'a',
        },
      });
      expect(isStorageRecordCeph(record)).toBe(false);
    });

    it('falls back to isCephStorageRecord and returns true for a ceph type', () => {
      const record = makeStorageRecord({
        details: { type: 'rbd' },
        capabilities: [],
        source: {
          platform: 'proxmox-pve',
          family: 'virtualization',
          origin: 'resource',
          adapterId: 'a',
        },
      });
      expect(isStorageRecordCeph(record)).toBe(true);
    });

    it('falls back to isCephStorageRecord and returns true for replication + proxmox platform', () => {
      const record = makeStorageRecord({
        details: { type: 'lvm' },
        capabilities: ['capacity', 'replication'],
        source: {
          platform: 'proxmox-pve',
          family: 'virtualization',
          origin: 'resource',
          adapterId: 'a',
        },
      });
      expect(isStorageRecordCeph(record)).toBe(true);
    });

    it('returns false when type is non-ceph and replication is absent', () => {
      const record = makeStorageRecord({
        details: { type: 'zfs' },
        capabilities: ['capacity'],
        source: {
          platform: 'truenas-scale',
          family: 'generic',
          origin: 'resource',
          adapterId: 'a',
        },
      });
      expect(isStorageRecordCeph(record)).toBe(false);
    });

    it('returns false when replication is present but the platform is not proxmox', () => {
      const record = makeStorageRecord({
        details: { type: 'lvm' },
        capabilities: ['capacity', 'replication'],
        source: { platform: 'truenas', family: 'generic', origin: 'resource', adapterId: 'a' },
      });
      expect(isStorageRecordCeph(record)).toBe(false);
    });

    it('returns false when the platform is proxmox but replication is absent', () => {
      const record = makeStorageRecord({
        details: { type: 'lvm' },
        capabilities: ['capacity'],
        source: {
          platform: 'proxmox-pve',
          family: 'virtualization',
          origin: 'resource',
          adapterId: 'a',
        },
      });
      expect(isStorageRecordCeph(record)).toBe(false);
    });
  });

  describe('countVisiblePhysicalDisksForNode', () => {
    const nodeOptions: StoragePageNodeOption[] = [
      { id: 'node-1', label: 'pve1', instance: 'cluster-main' },
      { id: 'node-2', label: 'pve2', instance: 'cluster-main' },
    ];

    it('returns the full disk count when the selected node is "all"', () => {
      const disks = [
        makeDisk({ id: 'd1', parentId: 'node-1' }),
        makeDisk({ id: 'd2', parentId: 'node-2' }),
        makeDisk({ id: 'd3', parentId: 'node-1' }),
      ];
      expect(countVisiblePhysicalDisksForNode('all', nodeOptions, disks)).toBe(3);
    });

    it('returns the full disk count when the selected node is not in nodeOptions', () => {
      const disks = [makeDisk({ id: 'd1', parentId: 'node-1' })];
      expect(countVisiblePhysicalDisksForNode('missing', nodeOptions, disks)).toBe(1);
    });

    it('counts only disks matching the selected node by parentId', () => {
      const disks = [
        makeDisk({ id: 'd1', parentId: 'node-1' }),
        makeDisk({ id: 'd2', parentId: 'node-2' }),
        makeDisk({ id: 'd3', parentId: 'node-1' }),
      ];
      expect(countVisiblePhysicalDisksForNode('node-1', nodeOptions, disks)).toBe(2);
    });

    it('returns 0 when the selected node matches no disks', () => {
      const disks = [makeDisk({ id: 'd1', parentId: 'node-2' })];
      expect(countVisiblePhysicalDisksForNode('node-1', nodeOptions, disks)).toBe(0);
    });

    it('returns 0 for a known node when the disk list is empty', () => {
      expect(countVisiblePhysicalDisksForNode('node-1', nodeOptions, [])).toBe(0);
    });

    it('returns 0 for "all" when the disk list is empty', () => {
      expect(countVisiblePhysicalDisksForNode('all', nodeOptions, [])).toBe(0);
    });

    it('matches disks by node label/instance when parentId is absent', () => {
      const diskByName = makeDisk({
        id: 'd-name',
        parentId: undefined,
        identity: { hostname: 'pve1' },
        canonicalIdentity: { hostname: 'pve1' },
        platformData: { proxmox: { nodeName: 'pve1', instance: 'cluster-main' } },
      });
      expect(countVisiblePhysicalDisksForNode('node-1', nodeOptions, [diskByName])).toBe(1);
    });
  });

  describe('buildStorageRouteFields', () => {
    describe('tab field', () => {
      it('write returns the value when it is not the default view', () => {
        expect(buildFields().tab!.write?.('disks')).toBe('disks');
      });

      it('get returns the accessor value and set forwards to the setter', () => {
        const setView = vi.fn();
        const fields = buildStorageRouteFields({
          view: () => 'disks',
          setView,
          sourceFilter: () => 'all',
          setSourceFilter: () => {},
          healthFilter: () => 'all',
          setHealthFilter: () => {},
          diskRoleFilter: () => 'all',
          setDiskRoleFilter: () => {},
          diskGroupFilter: () => 'all',
          setDiskGroupFilter: () => {},
          selectedNodeId: () => 'all',
          setSelectedNodeId: () => {},
          groupBy: () => 'none',
          setGroupBy: () => {},
          sortKey: () => 'priority',
          setSortKey: () => {},
          sortDirection: () => 'desc',
          setSortDirection: () => {},
          search: () => '',
          setSearch: () => {},
        });
        expect(fields.tab!.get()).toBe('disks');
        fields.tab!.set('pools');
        expect(setView).toHaveBeenCalledWith('pools');
      });
    });

    describe('source field', () => {
      it('write collapses whitespace-only/empty input to null via the default', () => {
        expect(buildFields().source!.write?.('   ')).toBeNull();
        expect(buildFields().source!.write?.('')).toBeNull();
      });
    });

    describe('status field', () => {
      it.each([
        ['critical', 'critical'],
        ['offline', 'offline'],
        ['unknown', 'unknown'],
      ] as const)(
        'write passes %s through getStorageStatusFilterValue unchanged',
        (input, expected) => {
          expect(buildFields().status!.write?.(input)).toBe(expected);
        },
      );
    });

    describe('diskRole field', () => {
      it('write slugifies spaced/cased input to the canonical facet value', () => {
        expect(buildFields().diskRole!.write?.(' NVME Disk ')).toBe('nvme-disk');
      });

      it('write returns null when the slugified value collapses to the default', () => {
        expect(buildFields().diskRole!.write?.(' All ')).toBeNull();
      });
    });

    describe('diskGroup field', () => {
      it('write slugifies spaced/cased input to the canonical facet value', () => {
        expect(buildFields().diskGroup!.write?.(' Data Pool ')).toBe('data-pool');
      });

      it('write returns null when the slugified value collapses to the default', () => {
        expect(buildFields().diskGroup!.write?.(' All ')).toBeNull();
      });
    });

    describe('group field', () => {
      it.each([
        ['none', 'none'],
        ['node', 'node'],
        ['type', 'type'],
      ])('read normalizes %s to its canonical group key', (input, expected) => {
        expect(buildFields().group!.read(makeParsed({ group: input }))).toBe(expected);
      });

      it('write returns the value when it is not the default', () => {
        expect(buildFields().group!.write?.('node')).toBe('node');
      });
    });

    describe('sort field', () => {
      it.each(['priority', 'name', 'type', 'host'])('read accepts canonical sort key %s', (key) => {
        expect(buildFields().sort!.read(makeParsed({ sort: key }))).toBe(key);
      });

      it('write returns the value when it is not the default', () => {
        expect(buildFields().sort!.write?.('name')).toBe('name');
      });
    });

    describe('order field', () => {
      it('read returns "desc" for an explicit "desc" token', () => {
        expect(buildFields().order!.read(makeParsed({ order: 'desc' }))).toBe('desc');
      });

      it('write returns "asc" when it is not the default', () => {
        expect(buildFields().order!.write?.('asc')).toBe('asc');
      });
    });

    describe('query field', () => {
      it('write returns null for an empty string (distinct from whitespace)', () => {
        expect(buildFields().query!.write?.('')).toBeNull();
      });
    });
  });
});
