import { describe, expect, it } from 'vitest';

import { buildStackedMemoryBarPresentation } from '../stackedMemoryBarModel';

const GiB = 1024 ** 3;

describe('buildStackedMemoryBarPresentation', () => {
  it('renders the v5 used | reclaimable cache split with the Proxmox reconciliation row', () => {
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
    // Proxmox counts cache as used: (4 + 6) / 16.
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
