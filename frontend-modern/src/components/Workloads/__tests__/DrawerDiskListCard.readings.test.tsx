import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';
import type { Disk } from '@/types/api';
import { normalizeDiskArray } from '@/utils/format';
import { getMetricTextColorClass } from '@/utils/metricThresholds';
import { buildDrawerDiskListItems, DrawerDiskListCard } from '../DrawerDiskListCard';

const GB = 1024 ** 3;
afterEach(cleanup);

describe('host drawer filesystem measurement admission', () => {
  it.each([
    { total: 100 * GB, usage: -1 },
    { total: 100 * GB, used: 70 * GB, usage: -1 },
    { total: 100 * GB, used: -1 },
    { total: 100 * GB, used: NaN },
    { total: 100 * GB, used: Infinity },
    { used: 30 * GB },
    { total: 0, used: 0 },
    { total: -1, used: 0 },
    { total: Infinity, used: 0 },
    { total: NaN, used: 0 },
    { mountpoint: '/capacity-unavailable', usage: 0.5 },
  ] satisfies Disk[])('does not turn unknown/invalid bytes into healthy usage: %j', (disk) => {
    // These are the same defaulting/filtering helpers used by both callers.
    const [item] = buildDrawerDiskListItems(normalizeDiskArray([disk])!);
    expect(item.percent).toBeNull();
    expect(item.usageText).toMatch(/^Usage unavailable/);
    expect(item.usageText).not.toMatch(/%|0 B/);
    expect(item.color).toBe('');
    expect(item.textClass).toBe('text-muted');
  });

  it('retains valid capacity without using it as evidence of usage', () => {
    const [item] = buildDrawerDiskListItems([
      { total: 100 * GB, usage: -1, device: '/dev/fixture' },
    ]);
    expect(item.usageText).toBe('Usage unavailable · 100 GB total');
    expect(item.device).toBe('/dev/fixture');
  });

  it.each([{ used: 0 }, {}])(
    'preserves true empty filesystems including DiskInfo omitted zero: %j',
    (bytes) => {
      const [item] = buildDrawerDiskListItems(
        normalizeDiskArray([{ total: 100 * GB, free: 100 * GB, ...bytes }])!,
      );
      expect(item.percent).toBe(0);
      expect(item.usageText).toBe('0% · 0 B / 100 GB');
      expect(item.textClass).toBe(getMetricTextColorClass(0, 'disk'));
    },
  );

  it.each([0.5, 70, 95, 110])(
    'uses the byte ratio, not a guessed fraction, at %s percent',
    (percent) => {
      const disk = { total: 100 * GB, used: percent * GB, usage: percent };
      const [item] = buildDrawerDiskListItems([disk]);
      expect(item.percent).toBeCloseTo(percent);
      expect(item.usageText).toMatch(new RegExp(`^${Math.round(percent)}%`));
    },
  );

  it('does not draw a utilization bar for an unavailable filesystem', () => {
    const { container } = render(() => (
      <DrawerDiskListCard
        disks={buildDrawerDiskListItems([
          { mountpoint: '/unknown', total: 100 * GB, usage: -1 },
          { mountpoint: '/empty', total: 100 * GB, used: 0 },
        ])}
      />
    ));
    expect(screen.getByText('Usage unavailable · 100 GB total')).toHaveClass('text-muted');
    expect(container.querySelectorAll('[style]')).toHaveLength(1);
    expect(container.querySelector('[style]')).toHaveStyle({ width: '0%' });
  });
});
