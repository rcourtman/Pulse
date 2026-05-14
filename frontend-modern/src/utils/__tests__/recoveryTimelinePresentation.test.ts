import { describe, expect, it } from 'vitest';
import {
  getRecoveryTimelineBarMarkerClass,
  getRecoveryTimelineColumnAriaLabel,
  getRecoveryTimelineColumnButtonClass,
  getRecoveryTimelineDayFilterLabel,
  getRecoveryTimelineDayFilterStateLabel,
  getRecoveryTimelineEmptyMarkerClass,
  getRecoveryTimelinePointTotalLabel,
  getRecoveryTimelineTooltipRows,
} from '@/utils/recoveryTimelinePresentation';

describe('getRecoveryTimelineColumnButtonClass', () => {
  it('keeps the full-height column as an accessible hit target only', () => {
    expect(getRecoveryTimelineColumnButtonClass(true)).toContain('group');
    expect(getRecoveryTimelineColumnButtonClass(true)).toContain('focus-visible:outline');
    expect(getRecoveryTimelineColumnButtonClass(true)).not.toContain('ring-blue-500');
    expect(getRecoveryTimelineColumnButtonClass(true)).not.toContain('bg-blue-100');
  });

  it('does not dim the full-height click column when another day is focused', () => {
    expect(getRecoveryTimelineColumnButtonClass(false)).toContain('focus-visible:outline');
    expect(getRecoveryTimelineColumnButtonClass(false, true)).not.toContain('opacity-40');
  });

  it('applies selected and dimmed states to the actual bar marker', () => {
    expect(getRecoveryTimelineBarMarkerClass(true, true)).toContain('ring-blue-500');
    expect(getRecoveryTimelineBarMarkerClass(true, true)).toContain('ring-inset');
    expect(getRecoveryTimelineBarMarkerClass(false, true)).toContain('opacity-40');
    expect(getRecoveryTimelineBarMarkerClass(false, true)).toContain('group-hover:opacity-100');
    expect(getRecoveryTimelineBarMarkerClass(false, false)).toContain('opacity-100');
  });

  it('shows a small selected baseline marker for empty focused days', () => {
    expect(getRecoveryTimelineEmptyMarkerClass(true, true)).toContain('h-1');
    expect(getRecoveryTimelineEmptyMarkerClass(true, true)).toContain('ring-blue-500');
    expect(getRecoveryTimelineEmptyMarkerClass(false, true)).toContain('opacity-40');
  });

  it('builds accessible selected-day labels', () => {
    expect(getRecoveryTimelineColumnAriaLabel('Feb 13, 2026', 1, false)).toBe(
      'Feb 13, 2026: 1 recovery point',
    );
    expect(getRecoveryTimelineColumnAriaLabel('Feb 14, 2026', 2, true)).toBe(
      'Feb 14, 2026: 2 recovery points, selected',
    );
  });

  it('builds tooltip rows as a vertical mode breakdown', () => {
    expect(
      getRecoveryTimelineTooltipRows({ total: 10, snapshot: 2, local: 3, remote: 5 }).map((row) => [
        row.label,
        row.value,
        row.muted,
      ]),
    ).toEqual([
      ['Snapshots', '2 (20%)', false],
      ['Local Copies', '3 (30%)', false],
      ['Remote Copies', '5 (50%)', false],
    ]);

    expect(
      getRecoveryTimelineTooltipRows({ total: 2, snapshot: 0, local: 2, remote: 0 }).map((row) => [
        row.label,
        row.value,
        row.muted,
      ]),
    ).toEqual([
      ['Snapshots', '0', true],
      ['Local Copies', '2 (100%)', false],
      ['Remote Copies', '0', true],
    ]);
  });

  it('formats timeline tooltip and selected-day labels', () => {
    expect(getRecoveryTimelinePointTotalLabel(1)).toBe('1 recovery point');
    expect(getRecoveryTimelinePointTotalLabel(0)).toBe('0 recovery points');
    expect(getRecoveryTimelineDayFilterStateLabel(true, true)).toBe('Day filter');
    expect(getRecoveryTimelineDayFilterStateLabel(false, true)).toBe('Outside day filter');
    expect(getRecoveryTimelineDayFilterLabel('Feb 14, 2026', 2)).toBe(
      'Feb 14, 2026 - 2 recovery points',
    );
  });
});
