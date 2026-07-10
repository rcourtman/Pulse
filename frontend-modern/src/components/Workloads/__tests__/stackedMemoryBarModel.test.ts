import { describe, expect, it } from 'vitest';

import { buildStackedMemoryBarPresentation } from '../stackedMemoryBarModel';

const GiB = 1024 ** 3;

describe('buildStackedMemoryBarPresentation', () => {
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
    const presentation = buildStackedMemoryBarPresentation(
      { used: 4 * GiB, total: 16 * GiB },
      400,
    );

    expect(presentation.segments.map((segment) => segment.label)).toEqual(['Active']);
    const labels = presentation.tooltipRows.map((row) => row.label);
    expect(labels).toEqual(['Used', 'Free']);
    const rows = Object.fromEntries(presentation.tooltipRows.map((row) => [row.label, row.value]));
    expect(rows['Free']).toBe('12.0 GB');
  });
});
