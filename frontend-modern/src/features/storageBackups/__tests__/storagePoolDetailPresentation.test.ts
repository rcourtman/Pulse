import { describe, expect, it } from 'vitest';
import type { StorageRecord } from '@/features/storageBackups/models';
import type { Resource } from '@/types/resource';
import {
  STORAGE_POOL_DETAIL_HISTORY_RANGE_OPTIONS,
  buildStoragePoolDetailConfigRows,
  buildStoragePoolDetailTopologyRows,
  buildStoragePoolDetailZfsSummary,
  getStoragePoolLinkedDisks,
  getZfsErrorSummary,
  getZfsErrorTextClass,
  getZfsScanTextClass,
  resolveStoragePoolDetailChartTarget,
} from '@/features/storageBackups/storagePoolDetailPresentation';

const buildRecord = (overrides: Partial<StorageRecord> = {}): StorageRecord => ({
  id: 'storage-1',
  name: 'tank',
  category: 'pool',
  health: 'healthy',
  location: { label: 'truenas01', scope: 'host' },
  capacity: { totalBytes: 1000, usedBytes: 400, freeBytes: 600, usagePercent: 40 },
  capabilities: ['capacity', 'health'],
  source: {
    platform: 'truenas',
    family: 'onprem',
    origin: 'resource',
    adapterId: 'resource-storage',
  },
  observedAt: Date.now(),
  details: {
    node: 'truenas01',
    parentId: 'host-truenas01',
    type: 'pool',
    zfsPool: {
      state: 'ONLINE',
      scan: 'scrub repaired 0B',
      readErrors: 1,
      writeErrors: 2,
      checksumErrors: 3,
      datasets: [
        {
          name: 'tank/apps',
          type: 'filesystem',
          mountpoint: '/tank/apps',
          usedBytes: 1024,
          availableBytes: 2048,
          referencedBytes: 512,
        },
      ],
      devices: [
        { name: 'sda', type: 'disk', state: 'ONLINE' },
        {
          name: 'sdb',
          type: 'disk',
          state: 'FAULTED',
          readErrors: 3,
          checksumErrors: 1,
          message: 'too many errors',
        },
      ],
    },
  },
  ...overrides,
});

const buildDisk = (): Resource =>
  ({
    id: 'disk-1',
    parentId: 'host-truenas01',
    type: 'physical_disk',
    name: 'disk-1',
    displayName: 'disk-1',
    platformId: 'truenas01',
    platformType: 'truenas',
    sourceType: 'agent',
    status: 'online',
    lastSeen: Date.now(),
    platformData: {
      physicalDisk: {
        devPath: '/dev/sda',
        model: 'Disk A',
        storageRole: 'data',
        storageGroup: 'tank',
        storageState: 'online',
        sizeBytes: 1_000,
        temperature: 44,
        smart: { reallocatedSectors: 1 },
      },
    },
  }) as Resource;

describe('storagePoolDetailPresentation', () => {
  it('returns canonical zfs scan and error text classes', () => {
    expect(getZfsScanTextClass()).toBe('text-yellow-600 dark:text-yellow-400 italic');
    expect(getZfsErrorTextClass()).toBe('font-medium text-red-600 dark:text-red-400');
  });

  it.each([
    [
      'missing used',
      { totalBytes: 1024, usedBytes: null, freeBytes: null, usagePercent: null },
      ['n/a', 'n/a', '1.00 KB', 'n/a'],
    ],
    [
      'all absent',
      { totalBytes: null, usedBytes: null, freeBytes: null, usagePercent: null },
      ['n/a', 'n/a', 'n/a', 'n/a'],
    ],
    [
      'measured empty',
      { totalBytes: 1024, usedBytes: 0, freeBytes: null, usagePercent: null },
      ['0 B', '1.00 KB', '1.00 KB', '0%'],
    ],
    [
      'measured full',
      { totalBytes: 1024, usedBytes: 1024, freeBytes: 0, usagePercent: 100 },
      ['1.00 KB', '0 B', '1.00 KB', '100%'],
    ],
    [
      'independent observations',
      { totalBytes: null, usedBytes: 512, freeBytes: 0, usagePercent: null },
      ['512 B', '0 B', 'n/a', 'n/a'],
    ],
    [
      'explicit ratio',
      { totalBytes: null, usedBytes: null, freeBytes: null, usagePercent: 25 },
      ['n/a', 'n/a', 'n/a', '25%'],
    ],
    [
      'explicit zero ratio',
      { totalBytes: null, usedBytes: null, freeBytes: null, usagePercent: 0 },
      ['n/a', 'n/a', 'n/a', '0%'],
    ],
    [
      'zero total',
      { totalBytes: 0, usedBytes: 0, freeBytes: 0, usagePercent: null },
      ['0 B', '0 B', '0 B', 'n/a'],
    ],
    [
      'invalid observations',
      { totalBytes: Infinity, usedBytes: NaN, freeBytes: -1, usagePercent: -1 },
      ['n/a', 'n/a', 'n/a', 'n/a'],
    ],
    [
      'derived ratio',
      { totalBytes: 1024, usedBytes: 512, freeBytes: null, usagePercent: null },
      ['512 B', '512 B', '1.00 KB', '50%'],
    ],
    [
      'provider free differs from subtraction',
      { totalBytes: 1024, usedBytes: 512, freeBytes: 256, usagePercent: 60 },
      ['512 B', '256 B', '1.00 KB', '60%'],
    ],
  ] as const)('preserves capacity evidence: %s', (_name, capacity, expected) => {
    const rows = buildStoragePoolDetailConfigRows(buildRecord({ capacity }));
    expect(
      ['Used', 'Free', 'Total', 'Usage'].map(
        (label) => rows.find((row) => row.label === label)?.value,
      ),
    ).toEqual(expected);
  });

  it('formats zfs error summaries canonically', () => {
    expect(getZfsErrorSummary(1, 2, 3)).toBe('Errors: R:1 W:2 C:3');
  });

  it('centralizes storage pool detail history ranges and chart target resolution', () => {
    expect(STORAGE_POOL_DETAIL_HISTORY_RANGE_OPTIONS.map((option) => option.value)).toEqual([
      '24h',
      '7d',
      '14d',
      '30d',
      '90d',
    ]);
    expect(
      resolveStoragePoolDetailChartTarget(
        buildRecord({
          metricsTarget: { resourceType: 'storage', resourceId: 'pool:tank' },
          refs: { resourceId: 'legacy:tank' },
        }),
      ),
    ).toEqual({
      resourceType: 'storage',
      resourceId: 'pool:tank',
    });
  });

  it('builds canonical pool config, zfs summary, and linked disk state', () => {
    const record = buildRecord();

    expect(buildStoragePoolDetailConfigRows(record)).toEqual(
      expect.arrayContaining([
        { label: 'Node', value: 'truenas01' },
        { label: 'Type', value: 'pool' },
        { label: 'Status', value: 'available' },
        { label: 'Usage', value: '40%' },
      ]),
    );
    expect(buildStoragePoolDetailZfsSummary(record)).toEqual({
      state: 'ONLINE',
      scan: 'scrub repaired 0B',
      errorSummary: 'Errors: R:1 W:2 C:3',
      devices: [
        { name: 'sda', type: 'disk', state: 'ONLINE', errorSummary: '', message: '' },
        {
          name: 'sdb',
          type: 'disk',
          state: 'FAULTED',
          errorSummary: '3R/0W/1C errors',
          message: 'too many errors',
        },
      ],
      datasets: [
        {
          name: 'tank/apps',
          type: 'filesystem',
          mountpoint: '/tank/apps',
          usedLabel: '1.00 KB',
          availableLabel: '2.00 KB',
          referencedLabel: '512 B',
        },
      ],
    });
    expect(getStoragePoolLinkedDisks(record, [buildDisk()])).toEqual([
      {
        id: 'disk-1',
        devPath: '/dev/sda',
        model: 'Disk A',
        role: 'data',
        state: 'online',
        sizeLabel: '1000 B',
        diskType: '',
        alertResourceIds: [],
        temperature: 44,
        hasIssue: true,
        spunDown: false,
        errorCount: 0,
        ioLabel: '',
      },
    ]);
  });

  it('links UnRAID array disks by storage group and summarizes native topology facts', () => {
    const record = buildRecord({
      name: 'Tower Array',
      source: {
        platform: 'unraid',
        family: 'onprem',
        origin: 'resource',
        adapterId: 'resource-storage',
      },
      details: {
        type: 'unraid-array',
        parentId: 'host-tower',
        platform: 'unraid',
        topology: 'array',
        arrayState: 'STARTED',
      },
    });
    const disks = [
      {
        id: 'parity',
        parentId: 'host-tower',
        type: 'physical_disk',
        name: 'Parity',
        displayName: 'Parity',
        platformId: 'tower',
        platformType: 'unraid',
        sourceType: 'agent',
        status: 'online',
        lastSeen: Date.now(),
        physicalDisk: {
          devPath: '/dev/sdb',
          model: 'Parity Disk',
          storageRole: 'parity',
          storageGroup: 'unraid-array',
          storageState: 'online',
          sizeBytes: 6_000,
          temperature: 31,
        },
      },
      {
        id: 'disk1',
        parentId: 'host-tower',
        type: 'physical_disk',
        name: 'Disk 1',
        displayName: 'Disk 1',
        platformId: 'tower',
        platformType: 'unraid',
        sourceType: 'agent',
        status: 'online',
        lastSeen: Date.now(),
        physicalDisk: {
          devPath: '/dev/sdc',
          model: 'Data Disk',
          storageRole: 'data',
          storageGroup: 'unraid-array',
          storageState: 'online',
          sizeBytes: 6_000,
          temperature: 32,
          spunDown: true,
          collection: {
            temperature: {
              state: 'unavailable',
              source: 'unraid',
              reason: 'disk is reported spun down',
            },
          },
          readCount: 10,
          writeCount: 20,
          errorCount: 16,
        },
      },
    ] as Resource[];

    const linkedDisks = getStoragePoolLinkedDisks(record, disks, (disk) => [`owner-of-${disk.id}`]);

    expect(linkedDisks.map((disk) => disk.role)).toEqual(['parity', 'data']);
    // Each linked disk carries the override keys its Temp colour is judged under.
    expect(linkedDisks.map((disk) => disk.alertResourceIds)).toEqual(
      linkedDisks.map((disk) => [`owner-of-${disk.id}`]),
    );
    expect(linkedDisks[0].temperatureLastKnownTitle).toBeUndefined();
    expect(linkedDisks[1]).toEqual(
      expect.objectContaining({
        devPath: '/dev/sdc',
        sizeLabel: '5.86 KB',
        temperature: 32,
        temperatureLastKnownTitle: 'Last known reading, not current: disk is reported spun down',
        spunDown: true,
        errorCount: 16,
        ioLabel: 'R 10 / W 20',
      }),
    );
    expect(buildStoragePoolDetailTopologyRows(record, linkedDisks)).toEqual(
      expect.arrayContaining([
        { label: 'Kind', value: 'Array' },
        { label: 'State', value: 'Started' },
        { label: 'Parity', value: '1 disk' },
        { label: 'Data disks', value: '1 disk' },
        { label: 'Spun down', value: '1 disk' },
        { label: 'Disk errors', value: '16' },
      ]),
    );
  });
});
