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
    type: 'pool',
    zfsPool: {
      state: 'ONLINE',
      scan: 'scrub repaired 0B',
      readErrors: 1,
      writeErrors: 2,
      checksumErrors: 3,
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
    });
    expect(getStoragePoolLinkedDisks(record, [buildDisk()])).toEqual([
      {
        id: 'disk-1',
        devPath: '/dev/sda',
        model: 'Disk A',
        role: 'data',
        state: 'online',
        sizeLabel: '1000 B',
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
        platform: 'unraid',
        topology: 'array',
        arrayState: 'STARTED',
      },
    });
    const disks = [
      {
        id: 'parity',
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
          readCount: 10,
          writeCount: 20,
          errorCount: 16,
        },
      },
    ] as Resource[];

    const linkedDisks = getStoragePoolLinkedDisks(record, disks);

    expect(linkedDisks.map((disk) => disk.role)).toEqual(['parity', 'data']);
    expect(linkedDisks[1]).toEqual(
      expect.objectContaining({
        devPath: '/dev/sdc',
        sizeLabel: '5.86 KB',
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
