import { describe, expect, it } from 'vitest';
import {
  getAlertFrequencyClearFilterButtonClass,
  getAlertFrequencySelectionPresentation,
} from '@/utils/alertFrequencyPresentation';

describe('alertFrequencyPresentation', () => {
  it('returns the canonical filtered-range chip presentation', () => {
    expect(getAlertFrequencySelectionPresentation()).toEqual({
      containerClass:
        'inline-flex items-center gap-2 rounded-full border border-blue-200 bg-blue-50 px-3 py-1 text-xs text-blue-700 dark:border-blue-700 dark:bg-blue-900 dark:text-blue-200',
      labelClass:
        'font-medium uppercase tracking-wide text-[10px] text-blue-600 dark:text-blue-300',
    });
  });

  it('returns the canonical clear-filter button presentation', () => {
    expect(getAlertFrequencyClearFilterButtonClass()).toBe(
      'rounded bg-blue-100 px-2 py-0.5 text-xs text-blue-700 transition-colors hover:bg-blue-200 dark:bg-blue-900 dark:text-blue-300 dark:hover:bg-blue-800',
    );
  });
});
