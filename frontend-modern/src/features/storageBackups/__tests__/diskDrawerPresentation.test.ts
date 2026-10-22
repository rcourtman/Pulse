import { describe, expect, it } from 'vitest';
import {
  buildDiskDrawerActivitySection,
  buildDiskDrawerHealthSection,
  getDiskDrawerHistoryGroups,
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
