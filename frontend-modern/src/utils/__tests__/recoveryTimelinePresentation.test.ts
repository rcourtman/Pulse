import { describe, expect, it } from 'vitest';
import {
  getRecoveryTimelineBarMarkerClass,
  getRecoveryTimelineColumnButtonClass,
  getRecoveryTimelineDayFilterStateLabel,
  getRecoveryTimelineEmptyMarkerClass,
} from '@/utils/recoveryTimelinePresentation';

describe('getRecoveryTimelineColumnButtonClass', () => {
  it('keeps the full-height column as an accessible hit target only', () => {
    expect(getRecoveryTimelineColumnButtonClass(true)).toContain('group');
    expect(getRecoveryTimelineColumnButtonClass(true)).toContain('focus-visible:outline-solid');
    expect(getRecoveryTimelineColumnButtonClass(true)).not.toContain('ring-blue-500');
    expect(getRecoveryTimelineColumnButtonClass(true)).not.toContain('bg-blue-100');
  });

  it('does not dim the full-height click column when another day is focused', () => {
    expect(getRecoveryTimelineColumnButtonClass(false)).toContain('focus-visible:outline-solid');
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

  it('formats selected-day filter state labels', () => {
    expect(getRecoveryTimelineDayFilterStateLabel(true, true)).toBe('Day filter');
    expect(getRecoveryTimelineDayFilterStateLabel(false, true)).toBe('Outside day filter');
  });
});
