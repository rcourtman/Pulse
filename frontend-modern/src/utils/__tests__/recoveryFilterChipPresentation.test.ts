import { describe, expect, it } from 'vitest';
import { getRecoveryFilterChipPresentation } from '@/utils/recoveryFilterChipPresentation';

describe('getRecoveryFilterChipPresentation', () => {
  it('returns the canonical day chip presentation', () => {
    expect(getRecoveryFilterChipPresentation('day')).toEqual({
      clearButtonClass: 'rounded px-1 py-0.5 text-[10px] hover:bg-blue-100 dark:hover:bg-blue-900',
      className:
        'inline-flex max-w-full items-center gap-1 rounded border px-2 py-0.5 text-[10px] border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-700 dark:bg-blue-900 dark:text-blue-200',
      label: 'Day',
    });
  });

  it('returns the canonical namespace chip presentation', () => {
    expect(getRecoveryFilterChipPresentation('namespace')).toMatchObject({
      clearButtonClass:
        'rounded px-1 py-0.5 text-[10px] hover:bg-violet-100 dark:hover:bg-violet-900',
      label: 'Namespace',
    });
    expect(getRecoveryFilterChipPresentation('namespace').className).toContain('border-violet-200');
  });
});
