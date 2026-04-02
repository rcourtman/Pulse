import { describe, expect, it } from 'vitest';
import { getAIQuickstartCreditsPresentation } from '@/utils/aiQuickstartPresentation';

describe('getAIQuickstartCreditsPresentation', () => {
  it('returns active quickstart credit presentation', () => {
    expect(getAIQuickstartCreditsPresentation(3, 10)).toEqual({
      className:
        'bg-blue-50 dark:bg-blue-950 border-blue-200 dark:border-blue-800 text-blue-700 dark:text-blue-300',
      summary: 'Patrol quickstart: 3/10 runs left',
      title: '3 of 10 free Patrol quickstart runs remaining. No API key needed for Patrol.',
    });
  });

  it('returns exhausted quickstart presentation', () => {
    expect(getAIQuickstartCreditsPresentation(0, 10)).toEqual({
      className:
        'bg-amber-50 dark:bg-amber-950 border-amber-200 dark:border-amber-800 text-amber-700 dark:text-amber-300',
      summary: 'Patrol quickstart exhausted',
      title: 'Patrol quickstart is exhausted. Connect your API key to continue using AI Patrol.',
    });
  });
});
