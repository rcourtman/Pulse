import { describe, expect, it } from 'vitest';
import { getPMGThreatPresentation } from '@/utils/pmgThreatPresentation';

describe('pmgThreatPresentation', () => {
  it('returns spam threat classes', () => {
    expect(getPMGThreatPresentation('spam')).toEqual({
      barClass: 'bg-orange-500',
      textClass: 'text-orange-600 dark:text-orange-400',
    });
  });

  it('returns virus threat classes', () => {
    expect(getPMGThreatPresentation('virus')).toEqual({
      barClass: 'bg-red-500',
      textClass: 'text-red-600 dark:text-red-400',
    });
  });

  it('returns quarantine threat classes', () => {
    expect(getPMGThreatPresentation('quarantine')).toEqual({
      barClass: 'bg-yellow-500',
      textClass: 'text-yellow-600 dark:text-yellow-400',
    });
  });
});
