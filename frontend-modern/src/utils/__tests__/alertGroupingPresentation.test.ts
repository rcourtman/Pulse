import { describe, expect, it } from 'vitest';
import {
  getAlertGroupingCardClass,
  getAlertGroupingCheckboxClass,
} from '@/utils/alertGroupingPresentation';

describe('alertGroupingPresentation', () => {
  it('returns the selected grouping card presentation', () => {
    expect(getAlertGroupingCardClass(true)).toBe(
      'relative flex cursor-pointer items-center gap-2 rounded-md border-2 p-3 transition-all focus-within:ring-2 focus-within:ring-blue-500 focus-within:ring-offset-1 dark:focus-within:ring-offset-surface border-blue-500 bg-blue-50 shadow-xs dark:bg-blue-900/25',
    );
  });

  it('returns the unselected grouping checkbox presentation', () => {
    expect(getAlertGroupingCheckboxClass(false)).toBe(
      'flex h-4 w-4 shrink-0 items-center justify-center rounded-sm border-2 border-border',
    );
  });
});
