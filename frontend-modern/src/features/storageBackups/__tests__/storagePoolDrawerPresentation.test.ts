import { describe, expect, it } from 'vitest';
import type { StorageRecord } from '@/features/storageBackups/models';
import {
  buildStoragePoolDetailZfsSummary,
  type StoragePoolDetailLinkedDisk,
} from '@/features/storageBackups/storagePoolDetailPresentation';
import {
  buildStoragePoolDrawerDetailSections,
  buildStoragePoolDrawerExceptionRows,
  buildStoragePoolDrawerZfsSections,
  getStoragePoolDrawerFacts,
} from '@/features/storageBackups/storagePoolDrawerPresentation';

const record = (overrides: Partial<StorageRecord> = {}): StorageRecord => ({
  id: 'storage-1',
  name: 'tank',
  category: 'pool',
  health: 'healthy',
  location: { label: 'truenas01', scope: 'host' },
  capacity: { totalBytes: 1_000, usedBytes: 400, freeBytes: 600, usagePercent: 40 },
  capabilities: ['capacity', 'health'],
  source: { platform: 'truenas', family: 'onprem', origin: 'resource', adapterId: 'resource' },
  observedAt: Date.now(),
  ...overrides,
});

const linkedDisk = (overrides: Partial<StoragePoolDetailLinkedDisk> = {}) =>
  ({
    id: 'disk-1',
    devPath: '/dev/sda',
    model: 'Disk A',
    role: '',
    state: '',
    sizeLabel: '',
    diskType: 'hdd',
    alertResourceIds: [],
    temperature: 0,
    hasIssue: false,
    spunDown: false,
    errorCount: 0,
    ioLabel: '',
    ...overrides,
  }) satisfies StoragePoolDetailLinkedDisk;

describe('storage pool drawer presentation', () => {
  it('lists nothing worth checking for a healthy pool', () => {
    expect(
      buildStoragePoolDrawerExceptionRows({
        record: record(),
        zfsSummary: null,
        linkedDisks: [linkedDisk({ temperature: 35 })],
        getDiskHeat: () => null,
      }),
    ).toEqual([]);
  });

  it('lifts reduced protection, disk errors and hot disks, but not a retained reading', () => {
    const rows = buildStoragePoolDrawerExceptionRows({
      record: record({ protectionReduced: true, protectionSummary: 'One mirror member missing' }),
      zfsSummary: null,
      linkedDisks: [
        linkedDisk({ errorCount: 2 }),
        linkedDisk({ devPath: '/dev/sdb', hasIssue: true }),
        linkedDisk({ devPath: '/dev/sdc', temperature: 66 }),
        linkedDisk({
          devPath: '/dev/sdd',
          temperature: 70,
          temperatureLastKnownTitle: 'Last known reading, not current',
        }),
      ],
      getDiskHeat: (disk) => (disk.temperature >= 65 ? 'critical' : null),
    });
    expect(rows.map((row) => [row.label, row.value, row.tone])).toEqual([
      ['Protection', 'One mirror member missing', 'danger'],
      ['Disk', '/dev/sda: 2 errors', 'warning'],
      ['Disk', '/dev/sdb: reallocated sectors', 'warning'],
      ['Disk temperature', '/dev/sdc: 66°C', 'danger'],
    ]);
  });

  it('keeps the whole device tree in front and moves pool counters into Worth checking', () => {
    const zfs = buildStoragePoolDetailZfsSummary(
      record({
        details: {
          zfsPool: {
            name: 'tank',
            state: 'ONLINE',
            scan: 'scrub in progress',
            readErrors: 1,
            writeErrors: 0,
            checksumErrors: 0,
            devices: [
              { name: 'mirror-0', type: 'mirror', state: 'ONLINE' },
              { name: 'sdb', type: 'disk', state: 'DEGRADED' },
            ],
          },
        },
      }),
    );
    const sections = buildStoragePoolDrawerZfsSections(zfs);
    expect(sections.map((section) => section.label)).toEqual(['ZFS pool', 'ZFS devices (2)']);
    expect(sections[0].rows.map((row) => row.label)).toEqual(['State', 'Scan']);
    expect(sections[1].rows.map((row) => [row.label, row.tone, row.layout, row.wrap])).toEqual([
      ['mirror-0', 'default', 'stacked', true],
      ['sdb', 'warning', 'stacked', true],
    ]);
  });

  it('keeps configuration, topology and datasets for Details', () => {
    const sections = buildStoragePoolDrawerDetailSections({
      configRows: [{ label: 'Node', value: 'truenas01' }],
      topologyRows: [],
      zfsSummary: null,
    });
    expect(sections.map((section) => section.label)).toEqual(['Configuration']);
    expect(sections[0].rows[0]).toMatchObject({ label: 'Node', value: 'truenas01', wrap: true });
  });

  it('states facts plainly and never a used amount for a stale pool', () => {
    expect(getStoragePoolDrawerFacts(record())).toEqual([
      'pool',
      '400 B of 1000 B used',
      'On truenas01',
    ]);
    expect(getStoragePoolDrawerFacts(record({ freshness: 'stale' }))).toContain('1000 B total');
    // Proxmox lists content types comma-separated without spaces.
    expect(getStoragePoolDrawerFacts(record({ details: { content: 'images,rootdir' } }))).toContain(
      'Holds images, rootdir',
    );
  });
});
