import { describe, expect, it } from 'vitest';
import {
  getRecoveryTimelineColumnAriaLabel,
  getRecoveryTimelineColumnButtonClass,
} from '@/utils/recoveryTimelinePresentation';

describe('getRecoveryTimelineColumnButtonClass', () => {
  it('returns the canonical selected class', () => {
    expect(getRecoveryTimelineColumnButtonClass(true)).toContain('bg-blue-100');
    expect(getRecoveryTimelineColumnButtonClass(true)).toContain('ring-blue-500');
  });

  it('returns the canonical hover class when unselected', () => {
    expect(getRecoveryTimelineColumnButtonClass(false)).toContain('hover:bg-surface-hover');
    expect(getRecoveryTimelineColumnButtonClass(false)).toContain('focus-visible:outline');
  });

  it('builds accessible selected-day labels', () => {
    expect(getRecoveryTimelineColumnAriaLabel('Feb 13, 2026', 1, false)).toBe(
      'Feb 13, 2026: 1 recovery point',
    );
    expect(getRecoveryTimelineColumnAriaLabel('Feb 14, 2026', 2, true)).toBe(
      'Feb 14, 2026: 2 recovery points, selected',
    );
  });
});
