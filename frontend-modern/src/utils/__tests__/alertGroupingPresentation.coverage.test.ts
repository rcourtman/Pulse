import { describe, expect, it } from 'vitest';
import {
  getAlertGroupingCardClass,
  getAlertGroupingCheckboxClass,
} from '@/utils/alertGroupingPresentation';

describe('getAlertGroupingCardClass', () => {
  it('emits the active presentation when selected', () => {
    expect(getAlertGroupingCardClass(true)).toBe(
      'relative flex items-center gap-2 rounded-md border-2 p-3 transition-all border-blue-500 bg-blue-50 shadow-sm dark:bg-blue-900',
    );
  });

  it('emits the idle presentation when not selected', () => {
    expect(getAlertGroupingCardClass(false)).toBe(
      'relative flex items-center gap-2 rounded-md border-2 p-3 transition-all border-border hover:bg-surface-hover',
    );
  });

  it('selects distinct styling for the selected vs unselected card', () => {
    const selected = getAlertGroupingCardClass(true);
    const unselected = getAlertGroupingCardClass(false);

    expect(selected).not.toBe(unselected);
    // The selected card signals the active border/fill; the idle card defers to the theme border.
    expect(selected).toContain('border-blue-500');
    expect(selected).toContain('bg-blue-50');
    expect(unselected).toContain('border-border');
    expect(unselected).not.toContain('border-blue-500');
  });
});

describe('getAlertGroupingCheckboxClass', () => {
  it('emits the checked presentation when selected', () => {
    expect(getAlertGroupingCheckboxClass(true)).toBe(
      'flex h-4 w-4 items-center justify-center rounded border-2 border-blue-500 bg-blue-500',
    );
  });

  it('emits the unchecked presentation when not selected', () => {
    expect(getAlertGroupingCheckboxClass(false)).toBe(
      'flex h-4 w-4 items-center justify-center rounded border-2 border-border',
    );
  });

  it('selects distinct styling for the selected vs unselected checkbox', () => {
    const selected = getAlertGroupingCheckboxClass(true);
    const unselected = getAlertGroupingCheckboxClass(false);

    expect(selected).not.toBe(unselected);
    // The checked checkbox applies the blue fill; the unchecked one only shows the theme border.
    expect(selected).toContain('bg-blue-500');
    expect(unselected).not.toContain('bg-blue-500');
  });
});
