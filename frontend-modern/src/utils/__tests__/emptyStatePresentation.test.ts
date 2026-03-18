import { describe, expect, it } from 'vitest';
import { getEmptyStatePresentation } from '@/utils/emptyStatePresentation';

describe('emptyStatePresentation', () => {
  it('returns canonical danger presentation', () => {
    expect(getEmptyStatePresentation('danger')).toEqual({
      iconClass: 'bg-red-50 dark:bg-red-900 text-red-500',
      titleClass: 'text-red-700 dark:text-red-300',
      descriptionClass: 'text-red-600 dark:text-red-300',
    });
  });

  it('returns canonical default presentation', () => {
    expect(getEmptyStatePresentation('default')).toEqual({
      iconClass: 'bg-surface-alt text-muted',
      titleClass: 'text-base-content',
      descriptionClass: 'text-muted',
    });
  });
});
