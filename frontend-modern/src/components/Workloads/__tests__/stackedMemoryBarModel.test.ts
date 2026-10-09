import { describe, expect, it } from 'vitest';

import { buildStackedMemoryBarPresentation } from '../stackedMemoryBarModel';
import type { AnomalyReport } from '@/types/aiIntelligence';

const GiB = 1024 ** 3;

const anomaly: AnomalyReport = {
  resource_id: 'vm-100',
  resource_name: 'test-vm',
  resource_type: 'vm',
  metric: 'memory',
  current_value: 90,
  baseline_mean: 30,
  baseline_std_dev: 5,
  z_score: 12,
  severity: 'critical',
  description: 'Memory usage 3x above baseline',
};

describe('source-owned memory decorations', () => {
  const props = {
    used: 90,
    total: 100,
    cache: 2,
    balloon: 98,
    swapUsed: 10,
    swapTotal: 20,
    anomaly,
  };

  it.each(['last-known', 'unknown'] as const)(
    'keeps %s composition inspectable without a live severity or anomaly',
    (state) => {
      const message = `${state}. Source: QEMU guest agent. Not a current measurement.`;
      const current = buildStackedMemoryBarPresentation(props, 400);
      const retained = buildStackedMemoryBarPresentation(
        { ...props, reading: { state, message } },
        400,
      );

      expect(retained.displayPercentValue).toBe(90);
      expect(retained.displaySublabel).toBe(current.displaySublabel);
      expect(
        retained.segments.map(({ label, leftPercent, widthPercent }) => ({
          label,
          leftPercent,
          widthPercent,
        })),
      ).toEqual(
        current.segments.map(({ label, leftPercent, widthPercent }) => ({
          label,
          leftPercent,
          widthPercent,
        })),
      );
      expect(retained.segments.every(({ color }) => color === 'rgba(148, 163, 184, 0.5)')).toBe(
        true,
      );
      expect(retained.tooltipRows.map(({ label, value }) => ({ label, value }))).toEqual(
        current.tooltipRows.map(({ label, value }) => ({ label, value })),
      );
      expect(retained.tooltipRows.every(({ labelClass }) => labelClass === 'text-muted')).toBe(
        true,
      );
      expect(retained.tooltipMessage).toBe(message);
      expect(retained.showSwapBar).toBe(true);
      expect(retained.swapBarPercent).toBe(50);
      expect(retained.swapBarColor).toBe('rgba(148, 163, 184, 0.5)');
      expect(retained.anomalyDescription).toBeUndefined();
      expect(retained.anomalyRatio).toBe('');
    },
  );

  it.each(['last-known', 'unknown'] as const)(
    'does not colour a %s percentage-only or host-share value as current pressure',
    (state) => {
      const reading = { state, message: 'Not a current measurement.' };
      const percentOnly = buildStackedMemoryBarPresentation(
        { used: 0, total: 0, percentOnly: 95, reading, anomaly },
        400,
      );
      const hostShare = buildStackedMemoryBarPresentation(
        {
          used: 10,
          total: 100,
          severityPercent: 95,
          comparisonTotalLabel: 'Host total',
          reading,
          anomaly,
        },
        400,
      );
      for (const presentation of [percentOnly, hostShare]) {
        expect(presentation.segments[0].color).toBe('rgba(148, 163, 184, 0.5)');
        expect(presentation.anomalyDescription).toBeUndefined();
        expect(presentation.tooltipMessage).toBe(reading.message);
      }
      expect(percentOnly.displayPercentValue).toBe(95);
      expect(hostShare.displayPercentValue).toBe(10);
      expect(hostShare.tooltipRows.map(({ label }) => label)).toEqual(['Used', 'Host total']);
    },
  );

  it.each([
    { unavailable: true },
    { reading: { state: 'unavailable' as const, message: 'Reading unavailable.' } },
    { unavailable: true, reading: { state: 'last-known' as const, message: 'Last known.' } },
  ])('withholds every live decoration for unavailable usage (%j)', (availability) => {
    const presentation = buildStackedMemoryBarPresentation({ ...props, ...availability }, 400);
    expect(presentation.unavailable).toBe(true);
    expect(presentation.segments).toEqual([]);
    expect(presentation.showSwapBar).toBe(false);
    expect(presentation.swapBarPercent).toBe(0);
    expect(presentation.showSublabel).toBe(false);
    expect(presentation.tooltipRows.map(({ label }) => label)).toEqual(['Usage', 'Total']);
    expect(presentation.anomalyDescription).toBeUndefined();
    expect(presentation.anomalyRatio).toBe('');
  });

  it('preserves current and unannotated-platform severity, swap and anomaly behaviour', () => {
    const legacy = buildStackedMemoryBarPresentation(props, 400);
    const current = buildStackedMemoryBarPresentation(
      { ...props, reading: { state: 'current', message: 'Current. Source: Pulse Agent.' } },
      400,
    );
    for (const presentation of [current, legacy]) {
      expect(presentation.segments[0].color).toBe('rgba(239, 68, 68, 0.6)');
      expect(presentation.tooltipRows[0].labelClass).toBe('text-red-400');
      expect(presentation.showSwapBar).toBe(true);
      expect(presentation.swapBarColor).toBe('rgb(168 85 247)');
      expect(presentation.anomalyDescription).toBe(anomaly.description);
      expect(presentation.anomalyRatio).toBe('3.0x');
    }
    expect(legacy.tooltipMessage).toBeUndefined();
  });

  it('keeps a retained measured zero as zero rather than unavailable', () => {
    const presentation = buildStackedMemoryBarPresentation(
      { used: 0, total: 100, reading: { state: 'last-known', message: 'Last known zero.' } },
      400,
    );
    expect(presentation.unavailable).toBe(false);
    expect(presentation.displayLabel).toBe('0%');
    expect(presentation.tooltipRows[0].value).toBe('0 B');
    expect(presentation.tooltipMessage).toBe('Last known zero.');
  });
});

describe('buildStackedMemoryBarPresentation', () => {
  it('renders unavailable usage without inventing a zero-percent segment', () => {
    const presentation = buildStackedMemoryBarPresentation(
      { used: 0, total: 8 * GiB, unavailable: true },
      400,
    );

    expect(presentation.unavailable).toBe(true);
    expect(presentation.segments).toEqual([]);
    expect(presentation.displaySublabel).toBe('');
    expect(presentation.showSublabel).toBe(false);
    expect(presentation.tooltipRows).toEqual([
      {
        borderTop: false,
        label: 'Usage',
        labelClass: 'text-muted',
        value: 'Unavailable',
      },
      {
        borderTop: true,
        label: 'Total',
        labelClass: 'text-muted',
        value: '8.00 GB',
      },
    ]);
  });

  it('renders the used | reclaimable cache split with a source-neutral reconciliation row', () => {
    const presentation = buildStackedMemoryBarPresentation(
      { used: 4 * GiB, total: 16 * GiB, cache: 6 * GiB },
      400,
    );

    expect(presentation.segments.map((segment) => segment.label)).toEqual([
      'Active',
      'Reclaimable',
    ]);
    expect(presentation.segments[1].leftPercent).toBeCloseTo(25);
    expect(presentation.segments[1].widthPercent).toBeCloseTo(37.5);

    const rows = Object.fromEntries(presentation.tooltipRows.map((row) => [row.label, row.value]));
    expect(rows['Used']).toBe('4.00 GB');
    expect(rows['Reclaimable cache']).toBe('6.00 GB');
    // Truly free excludes the reclaimable cache: 16 - 4 - 6.
    expect(rows['Free']).toBe('6.00 GB');
    expect(rows['Used with cache']).toBe('63%');
  });

  it('allows provider-owned surfaces to name the cache-inclusive comparison', () => {
    const presentation = buildStackedMemoryBarPresentation(
      {
        used: 4 * GiB,
        total: 16 * GiB,
        cache: 6 * GiB,
        cacheInclusiveLabel: 'Shown in Proxmox',
      },
      400,
    );

    const rows = Object.fromEntries(presentation.tooltipRows.map((row) => [row.label, row.value]));
    expect(rows['Shown in Proxmox']).toBe('63%');
  });

  it('renders an explicit host-total comparison without implying the remainder is free', () => {
    const presentation = buildStackedMemoryBarPresentation(
      {
        used: 4 * GiB,
        total: 64 * GiB,
        comparisonTotalLabel: 'Host total',
        tooltipTitle: 'pve-01 memory share',
      },
      400,
    );

    expect(presentation.displayPercentValue).toBeCloseTo(6.25);
    expect(presentation.tooltipTitle).toBe('pve-01 memory share');
    expect(presentation.tooltipRows.map((row) => row.label)).toEqual(['Used', 'Host total']);
    expect(presentation.tooltipRows.map((row) => row.value)).toEqual(['4.00 GB', '64.0 GB']);
  });

  it('keeps the cache segment between active and the balloon limit', () => {
    const presentation = buildStackedMemoryBarPresentation(
      { used: 4 * GiB, total: 16 * GiB, cache: 2 * GiB, balloon: 8 * GiB },
      400,
    );

    expect(presentation.segments.map((segment) => segment.label)).toEqual([
      'Active',
      'Reclaimable',
      'Balloon',
    ]);
    const balloonSegment = presentation.segments[2];
    expect(balloonSegment.leftPercent).toBeCloseTo(37.5);
    expect(balloonSegment.widthPercent).toBeCloseTo(12.5);

    const rows = Object.fromEntries(presentation.tooltipRows.map((row) => [row.label, row.value]));
    // Ballooning caps the usable ceiling: free = 8 - 4 - 2.
    expect(rows['Free']).toBe('2.00 GB');
  });

  it('colors the Used tooltip label with the same severity as the bar segment', () => {
    const normal = buildStackedMemoryBarPresentation(
      { used: 4 * GiB, total: 16 * GiB, cache: 6 * GiB },
      400,
    );
    expect(normal.tooltipRows[0].label).toBe('Used');
    expect(normal.tooltipRows[0].labelClass).toBe('text-green-400');

    // 80% used trips the default memory warning threshold (75), so the used
    // segment renders yellow and the legend must not claim green.
    const warning = buildStackedMemoryBarPresentation(
      { used: 12.8 * GiB, total: 16 * GiB, cache: 3.2 * GiB },
      400,
    );
    expect(warning.tooltipRows[0].labelClass).toBe('text-yellow-400');

    const critical = buildStackedMemoryBarPresentation(
      { used: 14 * GiB, total: 16 * GiB, cache: 2 * GiB },
      400,
    );
    expect(critical.tooltipRows[0].labelClass).toBe('text-red-400');
  });

  it('matches the pre-cache layout when no cache is reported', () => {
    const presentation = buildStackedMemoryBarPresentation({ used: 4 * GiB, total: 16 * GiB }, 400);

    expect(presentation.segments.map((segment) => segment.label)).toEqual(['Active']);
    const labels = presentation.tooltipRows.map((row) => row.label);
    expect(labels).toEqual(['Used', 'Free']);
    const rows = Object.fromEntries(presentation.tooltipRows.map((row) => [row.label, row.value]));
    expect(rows['Free']).toBe('12.0 GB');
  });
});
