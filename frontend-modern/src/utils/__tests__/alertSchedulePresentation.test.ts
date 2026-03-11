import { describe, expect, it } from 'vitest';
import { getAlertQuietDayButtonClass } from '@/utils/alertSchedulePresentation';

describe('alertSchedulePresentation', () => {
  it('returns the selected quiet-day button presentation', () => {
    expect(getAlertQuietDayButtonClass(true)).toBe(
      'rounded-md px-2 py-2 text-xs font-medium transition-all duration-200 bg-blue-500 text-white shadow-sm',
    );
  });

  it('returns the unselected quiet-day button presentation', () => {
    expect(getAlertQuietDayButtonClass(false)).toBe(
      'rounded-md px-2 py-2 text-xs font-medium transition-all duration-200 text-muted hover:bg-surface-hover',
    );
  });
});
