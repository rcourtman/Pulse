import { describe, expect, it } from 'vitest';
import { getRecoveryTimelineColumnButtonClass } from '@/utils/recoveryTimelinePresentation';

describe('getRecoveryTimelineColumnButtonClass', () => {
  it('returns the canonical selected class', () => {
    expect(getRecoveryTimelineColumnButtonClass(true)).toBe('bg-blue-100 dark:bg-blue-900');
  });

  it('returns the canonical hover class when unselected', () => {
    expect(getRecoveryTimelineColumnButtonClass(false)).toBe('hover:bg-surface-hover');
  });
});
