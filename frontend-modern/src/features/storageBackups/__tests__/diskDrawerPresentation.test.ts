import { describe, expect, it } from 'vitest';
import {
  buildDiskDrawerActivitySection,
  buildDiskDrawerHealthSection,
  buildDiskDrawerSmartSection,
  getDiskDrawerCurrentMetrics,
  getDiskDrawerDeferredMetrics,
  getDiskDrawerHistoryGroups,
  getDiskDrawerNotices,
  isDiskDrawerIOAvailable,
} from '@/features/storageBackups/diskDrawerPresentation';
import {
  getPhysicalDiskHealthIndicator,
  getPhysicalDiskHealthStatus,
  type PhysicalDiskPresentationData,
} from '@/features/storageBackups/diskPresentation';

const disk = (
  overrides: Partial<PhysicalDiskPresentationData> = {},
): PhysicalDiskPresentationData => ({
  node: 'tower',
  instance: 'lab',
  devPath: '/dev/sda',
  model: 'Archive HDD',
  serial: 'S1',
  wwn: '',
  size: 4_000_000_000_000,
  health: 'PASSED',
  riskReasons: [],
  wearout: -1,
  type: 'hdd',
  temperature: 40,
  rpm: 7200,
  used: '',
  ...overrides,
});

const metrics = (groups: ReturnType<typeof getDiskDrawerHistoryGroups>) =>
  groups.flatMap((group) => group.series.map((series) => series.metric));

describe('disk drawer presentation', () => {
  it('gives the drawer dot the Health cell verdict, judged the same way', () => {
    for (const [data, variant] of [
      [disk(), 'success'],
      [disk({ health: 'FAILED' }), 'danger'],
      [disk({ smartAttributes: { reallocatedSectors: 4 } }), 'warning'],
      [disk({ health: 'UNKNOWN' }), 'muted'],
    ] as const) {
      expect(getPhysicalDiskHealthIndicator(data)).toEqual({
        variant,
        label: getPhysicalDiskHealthStatus(data).label,
      });
    }
  });

  it('charts temperature first, then I/O when collected, then the wear counter for the kind', () => {
    expect(metrics(getDiskDrawerHistoryGroups(disk(), true))).toEqual([
      'smart_temp',
      'diskread',
      'diskwrite',
      'disk',
      'smart_reallocated_sectors',
    ]);
    expect(metrics(getDiskDrawerHistoryGroups(disk({ type: 'nvme' }), false))).toEqual([
      'smart_temp',
      'smart_percentage_used',
      'smart_available_spare',
    ]);
    // Stable panels: an ordinary snapshot does not remount the charts.
    expect(getDiskDrawerHistoryGroups(disk(), true)).toBe(getDiskDrawerHistoryGroups(disk(), true));
    expect(isDiskDrawerIOAvailable(disk())).toBe(true);
    expect(isDiskDrawerIOAvailable(disk({ collection: { io: { state: 'unsupported' } } }))).toBe(
      false,
    );
  });

  it('picks the history catalog by disk family, not by the latest readings', () => {
    const full = disk({ smartAttributes: { powerOnHours: 100, reallocatedSectors: 0 } });
    const bare = disk({ temperature: 0, smartAttributes: undefined });
    const hot = disk({ temperature: 65, smartAttributes: { reallocatedSectors: 0 } });
    // The same panels, by reference, whatever the snapshot carries.
    expect(getDiskDrawerHistoryGroups(bare, true)).toBe(getDiskDrawerHistoryGroups(full, true));
    expect(getDiskDrawerHistoryGroups(hot, true)).toBe(getDiskDrawerHistoryGroups(full, true));
    expect(metrics(getDiskDrawerHistoryGroups(bare, false))).toEqual([
      'smart_temp',
      'smart_reallocated_sectors',
    ]);
    const nvme = disk({ type: 'nvme', temperature: 0, smartAttributes: undefined });
    expect(metrics(getDiskDrawerHistoryGroups(nvme, true))).toEqual([
      'smart_temp',
      'diskread',
      'diskwrite',
      'disk',
      'smart_percentage_used',
      'smart_available_spare',
    ]);
    expect(getDiskDrawerHistoryGroups(nvme, false)).toBe(
      getDiskDrawerHistoryGroups(disk({ type: 'NVMe' }), false),
    );
  });

  it('leaves I/O panels out only when the collector says per-disk I/O is not available', () => {
    for (const state of ['unsupported', 'unavailable', 'missing'] as const) {
      expect(isDiskDrawerIOAvailable(disk({ collection: { io: { state } } }))).toBe(false);
    }
    expect(isDiskDrawerIOAvailable(disk({ collection: { io: { state: 'available' } } }))).toBe(
      true,
    );
    // Silence about I/O is not a report that it is missing.
    expect(isDiskDrawerIOAvailable(disk({ collection: {} }))).toBe(true);
    expect(
      getDiskDrawerNotices(
        disk({
          smartAttributes: { reallocatedSectors: 0 },
          collection: {
            io: { state: 'unsupported', reason: 'per-member counters unavailable' },
          },
        }),
      ),
    ).toEqual(['Disk I/O is unsupported: per-member counters unavailable']);
  });

  it('gives the charts only readings the snapshot reports, and a retained one as last known', () => {
    expect(getDiskDrawerCurrentMetrics(disk({ smartAttributes: undefined }))).toEqual({
      smart_temp: 40,
      smart_reallocated_sectors: undefined,
      smart_percentage_used: undefined,
      smart_available_spare: undefined,
    });
    // A stored zero counter is a reading; a missing or zero temperature is not.
    expect(
      getDiskDrawerCurrentMetrics(
        disk({ temperature: 0, smartAttributes: { reallocatedSectors: 0 } }),
      ),
    ).toEqual({
      smart_temp: undefined,
      smart_reallocated_sectors: 0,
      smart_percentage_used: undefined,
      smart_available_spare: undefined,
    });
    expect(getDiskDrawerDeferredMetrics(disk())).toBeUndefined();
    const retained = disk({
      collection: { temperature: { state: 'unavailable', reason: 'collection deadline exceeded' } },
    });
    expect(getDiskDrawerCurrentMetrics(retained).smart_temp).toBeUndefined();
    expect(getDiskDrawerDeferredMetrics(retained)).toEqual({
      smart_temp: { lastKnownValue: 40, message: expect.stringMatching(/\.$/) },
    });
  });

  it('shows a positive temperature without the extended SMART object, and invents no counters', () => {
    const tempOnly = disk({ smartAttributes: undefined });
    expect(buildDiskDrawerHealthSection(tempOnly).rows).toContainEqual(
      expect.objectContaining({ label: 'Temperature', value: '40°C' }),
    );
    expect(buildDiskDrawerSmartSection(tempOnly).rows).toEqual([]);
    expect(getDiskDrawerNotices(tempOnly)).toEqual([]);
    const empty = disk({ smartAttributes: {} });
    expect(buildDiskDrawerHealthSection(empty).rows).toContainEqual(
      expect.objectContaining({ label: 'Temperature', value: '40°C' }),
    );
    expect(buildDiskDrawerSmartSection(empty).rows).toEqual([]);
    for (const temperature of [0, -5, Number.NaN]) {
      const unread = disk({ temperature, smartAttributes: undefined });
      expect(buildDiskDrawerHealthSection(unread).rows.map((row) => row.label)).not.toContain(
        'Temperature',
      );
      expect(getDiskDrawerNotices(unread)).toEqual([
        'Detailed SMART attributes are not available for this disk.',
      ]);
    }
  });

  it('tones SSD life and a disk error count so the drawer lifts them', () => {
    const health = buildDiskDrawerHealthSection(disk({ type: 'ssd', wearout: 30 }));
    expect(health.rows.find((row) => row.label === 'Life remaining')).toMatchObject({
      value: '30%',
      tone: 'warning',
    });
    const activity = buildDiskDrawerActivitySection(
      disk({ readCount: 1200, writeCount: 30, errorCount: 3 }),
    );
    expect(activity.rows.find((row) => row.label === 'Errors')).toMatchObject({
      value: '3',
      tone: 'warning',
    });
    expect(buildDiskDrawerActivitySection(disk()).rows).toEqual([]);
  });

  it('lets a wear verdict state worn flash once instead of lifting the life left again', () => {
    const worn = disk({ type: 'ssd', wearout: 7 });
    expect(['warning', 'danger']).toContain(getPhysicalDiskHealthIndicator(worn).variant);
    const health = buildDiskDrawerHealthSection(worn);
    expect(health.rows.find((row) => row.label === 'Health')?.tone).toMatch(/warning|danger/);
    expect(health.rows.find((row) => row.label === 'Life remaining')).toMatchObject({
      value: '7%',
      tone: 'default',
    });
  });
});
