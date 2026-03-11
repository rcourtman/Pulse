import { describe, expect, it } from 'vitest';
import { getAISessionDiffStatusPresentation } from '@/utils/aiSessionDiffPresentation';

describe('aiSessionDiffPresentation', () => {
  it('returns added presentation', () => {
    expect(getAISessionDiffStatusPresentation('added')).toEqual({
      label: 'Added',
      badgeClasses: 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-200',
    });
  });

  it('returns modified presentation', () => {
    expect(getAISessionDiffStatusPresentation('modified')).toEqual({
      label: 'Modified',
      badgeClasses: 'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-200',
    });
  });

  it('returns deleted presentation', () => {
    expect(getAISessionDiffStatusPresentation('deleted')).toEqual({
      label: 'Deleted',
      badgeClasses: 'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-200',
    });
  });

  it('falls back to changed presentation', () => {
    expect(getAISessionDiffStatusPresentation('renamed' as never)).toEqual({
      label: 'Changed',
      badgeClasses: 'bg-surface-alt text-base-content',
    });
  });
});
