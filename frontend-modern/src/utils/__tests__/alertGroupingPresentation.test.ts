import { describe, expect, it } from 'vitest';
import {
  getAlertGroupingCardClass,
  getAlertGroupingCheckboxClass,
} from '@/utils/alertGroupingPresentation';

describe('alertGroupingPresentation', () => {
  it('returns the selected grouping card presentation', () => {
    expect(getAlertGroupingCardClass(true)).toBe(
      'relative flex items-center gap-2 rounded-md border-2 p-3 transition-all border-blue-500 bg-blue-50 shadow-sm dark:bg-blue-900',
    );
  });

  it('returns the unselected grouping checkbox presentation', () => {
    expect(getAlertGroupingCheckboxClass(false)).toBe(
      'flex h-4 w-4 items-center justify-center rounded border-2 border-border',
    );
  });
});
