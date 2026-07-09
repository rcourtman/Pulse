import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import type { StorageRecord } from '@/features/storageBackups/models';
import {
  resolvePhysicalDiskMetricResourceId,
  resolveStorageRecordMetricResourceId,
} from '@/features/storageBackups/storageMetricsIdentity';

const makeRecord = (overrides: Partial<StorageRecord> = {}): StorageRecord => ({
  id: 'storage-1',
  name: 'tank',
  category: 'pool',
  health: 'healthy',
  location: { label: 'truenas01', scope: 'host' },
  capacity: { totalBytes: 100, usedBytes: 40, freeBytes: 60, usagePercent: 40 },
  capabilities: ['capacity'],
  source: {
    platform: 'truenas',
    family: 'onprem',
    origin: 'resource',
    adapterId: 'resource-storage',
  },
  observedAt: Date.now(),
  ...overrides,
});

const makeResource = (overrides: Partial<Resource> = {}): Resource =>
  ({
    id: 'disk-1',
    type: 'storage',
    name: 'sda',
    platformType: 'truenas',
    sourceType: 'api',
    ...overrides,
  }) as Resource;

describe('resolveStorageRecordMetricResourceId', () => {
  it('prefers metricsTarget.resourceId over refs and id', () => {
    expect(
      resolveStorageRecordMetricResourceId(
        makeRecord({
          metricsTarget: { resourceType: 'storage', resourceId: 'metrics-id' },
          refs: { resourceId: 'refs-id' },
          id: 'storage-1',
        }),
      ),
    ).toBe('metrics-id');
  });

  it('falls back to refs.resourceId when metricsTarget is absent', () => {
    expect(
      resolveStorageRecordMetricResourceId(
        makeRecord({ refs: { resourceId: 'refs-id' }, id: 'storage-1' }),
      ),
    ).toBe('refs-id');
  });

  it('falls back to refs.resourceId when metricsTarget.resourceId is empty or whitespace', () => {
    expect(
      resolveStorageRecordMetricResourceId(
        makeRecord({
          metricsTarget: { resourceType: 'storage', resourceId: '   ' },
          refs: { resourceId: 'refs-id' },
        }),
      ),
    ).toBe('refs-id');
    expect(
      resolveStorageRecordMetricResourceId(
        makeRecord({
          metricsTarget: { resourceType: 'storage', resourceId: '' },
          refs: { resourceId: 'refs-id' },
        }),
      ),
    ).toBe('refs-id');
  });

  it('falls back to id when neither metricsTarget nor refs carry a resourceId', () => {
    expect(
      resolveStorageRecordMetricResourceId(
        makeRecord({ id: 'storage-1', refs: { resourceId: '   ' } }),
      ),
    ).toBe('storage-1');
    expect(resolveStorageRecordMetricResourceId(makeRecord({ id: 'storage-1' }))).toBe(
      'storage-1',
    );
  });

  it('trims whitespace from the selected field at every precedence level', () => {
    expect(
      resolveStorageRecordMetricResourceId(
        makeRecord({
          metricsTarget: { resourceType: 'storage', resourceId: '  trimmed-metrics  ' },
        }),
      ),
    ).toBe('trimmed-metrics');
    expect(
      resolveStorageRecordMetricResourceId(
        makeRecord({ refs: { resourceId: '  trimmed-refs  ' } }),
      ),
    ).toBe('trimmed-refs');
    expect(
      resolveStorageRecordMetricResourceId(makeRecord({ id: '  trimmed-id  ' })),
    ).toBe('trimmed-id');
  });

  it('returns an empty string when id is empty and no higher-precedence field is set', () => {
    expect(resolveStorageRecordMetricResourceId(makeRecord({ id: '' }))).toBe('');
  });
});

describe('resolvePhysicalDiskMetricResourceId', () => {
  it('returns metricsTarget.resourceId when type is explicitly disk', () => {
    expect(
      resolvePhysicalDiskMetricResourceId(
        makeResource({
          metricsTarget: { resourceType: 'disk', resourceId: 'metrics-disk-id' },
        }),
      ),
    ).toBe('metrics-disk-id');
  });

  it('matches the disk type case-insensitively', () => {
    expect(
      resolvePhysicalDiskMetricResourceId(
        makeResource({
          metricsTarget: { resourceType: 'DISK', resourceId: 'metrics-disk-id' },
        } as unknown as Partial<Resource>),
      ),
    ).toBe('metrics-disk-id');
  });

  it('returns metricsTarget.resourceId when type is absent', () => {
    expect(
      resolvePhysicalDiskMetricResourceId(
        makeResource({
          metricsTarget: { resourceType: '', resourceId: 'metrics-disk-id' },
        } as unknown as Partial<Resource>),
      ),
    ).toBe('metrics-disk-id');
  });

  it('returns metricsTarget.resourceId when type is whitespace-only (normalized to empty)', () => {
    expect(
      resolvePhysicalDiskMetricResourceId(
        makeResource({
          metricsTarget: { resourceType: '   ', resourceId: 'metrics-disk-id' },
        } as unknown as Partial<Resource>),
      ),
    ).toBe('metrics-disk-id');
  });

  it('skips metricsTarget when type is a non-disk resource type, falling through to serial', () => {
    expect(
      resolvePhysicalDiskMetricResourceId(
        makeResource({
          metricsTarget: { resourceType: 'storage', resourceId: 'metrics-storage-id' },
          physicalDisk: { serial: 'SERIAL-1' },
        }),
      ),
    ).toBe('SERIAL-1');
  });

  it('skips metricsTarget when resourceId is empty even with a disk type, falling through', () => {
    expect(
      resolvePhysicalDiskMetricResourceId(
        makeResource({
          metricsTarget: { resourceType: 'disk', resourceId: '' },
          physicalDisk: { serial: 'SERIAL-1' },
        }),
      ),
    ).toBe('SERIAL-1');
  });

  it('trims the selected metricsTarget.resourceId', () => {
    expect(
      resolvePhysicalDiskMetricResourceId(
        makeResource({
          metricsTarget: { resourceType: 'disk', resourceId: '  trimmed-disk  ' },
        }),
      ),
    ).toBe('trimmed-disk');
  });

  it('returns serial from disk.physicalDisk.serial', () => {
    expect(
      resolvePhysicalDiskMetricResourceId(
        makeResource({ physicalDisk: { serial: 'SERIAL-1' } }),
      ),
    ).toBe('SERIAL-1');
  });

  it('returns serial from platformData.physicalDisk.serial when disk.physicalDisk is absent', () => {
    expect(
      resolvePhysicalDiskMetricResourceId(
        makeResource({
          platformData: { physicalDisk: { serial: 'PD-SERIAL-1' } },
        }),
      ),
    ).toBe('PD-SERIAL-1');
  });

  it('prefers disk.physicalDisk.serial over platformData.physicalDisk.serial', () => {
    expect(
      resolvePhysicalDiskMetricResourceId(
        makeResource({
          physicalDisk: { serial: 'DIRECT' },
          platformData: { physicalDisk: { serial: 'PLATFORM' } },
        }),
      ),
    ).toBe('DIRECT');
  });

  it('trims the selected serial', () => {
    expect(
      resolvePhysicalDiskMetricResourceId(
        makeResource({ physicalDisk: { serial: '  TRIMMED-SERIAL  ' } }),
      ),
    ).toBe('TRIMMED-SERIAL');
  });

  it('falls through to wwn when no serial is available', () => {
    expect(
      resolvePhysicalDiskMetricResourceId(
        makeResource({ physicalDisk: { wwn: 'WWN-1' } }),
      ),
    ).toBe('WWN-1');
  });

  it('returns wwn from platformData.physicalDisk.wwn when disk.physicalDisk.wwn is absent', () => {
    expect(
      resolvePhysicalDiskMetricResourceId(
        makeResource({
          platformData: { physicalDisk: { wwn: 'PD-WWN-1' } },
        }),
      ),
    ).toBe('PD-WWN-1');
  });

  it('prefers disk.physicalDisk.wwn over platformData.physicalDisk.wwn', () => {
    expect(
      resolvePhysicalDiskMetricResourceId(
        makeResource({
          physicalDisk: { wwn: 'DIRECT-WWN' },
          platformData: { physicalDisk: { wwn: 'PLATFORM-WWN' } },
        }),
      ),
    ).toBe('DIRECT-WWN');
  });

  it('trims the selected wwn', () => {
    expect(
      resolvePhysicalDiskMetricResourceId(
        makeResource({ physicalDisk: { wwn: '  TRIMMED-WWN  ' } }),
      ),
    ).toBe('TRIMMED-WWN');
  });

  it('falls back to id when serial, wwn, and metricsTarget are all absent', () => {
    expect(resolvePhysicalDiskMetricResourceId(makeResource({ id: 'disk-1' }))).toBe(
      'disk-1',
    );
  });

  it('falls back to id when serial and wwn are whitespace-only', () => {
    expect(
      resolvePhysicalDiskMetricResourceId(
        makeResource({ id: 'disk-1', physicalDisk: { serial: '   ', wwn: '   ' } }),
      ),
    ).toBe('disk-1');
  });

  it('lets a whitespace-only disk.physicalDisk.serial shadow platformData.physicalDisk.serial (|| binds before trim)', () => {
    // The `||` on the raw values runs before trim(): a whitespace-only direct
    // serial wins the `||`, then trims to empty, so the platformData serial is
    // never consulted and resolution falls all the way through to id. This pins
    // the current (suspected-buggy) behavior — see GLM_REPORT.md caveat.
    expect(
      resolvePhysicalDiskMetricResourceId(
        makeResource({
          id: 'disk-1',
          physicalDisk: { serial: '   ' },
          platformData: { physicalDisk: { serial: 'PD-SERIAL-1' } },
        }),
      ),
    ).toBe('disk-1');
  });
});
