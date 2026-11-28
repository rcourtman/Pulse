import { describe, expect, it } from 'vitest';
import {
  getAlertQuietDayButtonClass,
  getAlertQuietSuppressCardClass,
  getAlertQuietSuppressCheckboxClass,
} from '@/utils/alertSchedulePresentation';

describe('alertSchedulePresentation', () => {
  it('returns the selected quiet-day button presentation', () => {
    expect(getAlertQuietDayButtonClass(true)).toBe(
      'min-h-11 rounded-md px-2 py-2 text-xs font-medium transition-all duration-200 focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-1 sm:min-h-0 dark:focus-visible:ring-offset-surface bg-blue-500 text-white shadow-xs',
    );
  });

  it('returns the unselected quiet-day button presentation', () => {
    expect(getAlertQuietDayButtonClass(false)).toBe(
      'min-h-11 rounded-md px-2 py-2 text-xs font-medium transition-all duration-200 focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-1 sm:min-h-0 dark:focus-visible:ring-offset-surface text-muted hover:bg-surface-hover',
    );
  });

  it('returns the selected quiet suppress card presentation', () => {
    expect(getAlertQuietSuppressCardClass(true)).toBe(
      'flex cursor-pointer items-start gap-3 rounded-md border px-3 py-2 transition-colors focus-within:ring-2 focus-within:ring-blue-500 focus-within:ring-offset-1 dark:focus-within:ring-offset-surface border-blue-500 bg-blue-50 dark:border-blue-400 dark:bg-blue-900/25',
    );
  });

  it('returns the unselected quiet suppress card presentation', () => {
    expect(getAlertQuietSuppressCardClass(false)).toBe(
      'flex cursor-pointer items-start gap-3 rounded-md border px-3 py-2 transition-colors focus-within:ring-2 focus-within:ring-blue-500 focus-within:ring-offset-1 dark:focus-within:ring-offset-surface border-border hover:bg-surface-hover',
    );
  });

  it('returns the selected quiet suppress checkbox presentation', () => {
    expect(getAlertQuietSuppressCheckboxClass(true)).toBe(
      'mt-1 flex h-4 w-4 shrink-0 items-center justify-center rounded-sm border-2 border-blue-500 bg-blue-500',
    );
  });

  it('returns the unselected quiet suppress checkbox presentation', () => {
    expect(getAlertQuietSuppressCheckboxClass(false)).toBe(
      'mt-1 flex h-4 w-4 shrink-0 items-center justify-center rounded-sm border-2 border-border',
    );
  });
});
