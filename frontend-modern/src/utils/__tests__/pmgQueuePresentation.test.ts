import { describe, expect, it } from 'vitest';
import {
  getPMGOldestAgeTextClass,
  getPMGQueueSeverity,
  getPMGQueueTextClass,
} from '@/utils/pmgQueuePresentation';

describe('pmgQueuePresentation', () => {
  it('derives queue severity from total depth', () => {
    expect(getPMGQueueSeverity(5)).toBe('low');
    expect(getPMGQueueSeverity(40)).toBe('medium');
    expect(getPMGQueueSeverity(120)).toBe('high');
  });

  it('returns queue text classes', () => {
    expect(getPMGQueueTextClass(5, 'text-green-600')).toBe('text-green-600');
    expect(getPMGQueueTextClass(40, 'text-green-600')).toBe(
      'text-yellow-600 dark:text-yellow-400',
    );
    expect(getPMGQueueTextClass(120, 'text-green-600')).toBe(
      'text-red-600 dark:text-red-400',
    );
  });

  it('returns oldest-age warning class only when queue age is high', () => {
    expect(getPMGOldestAgeTextClass(1200)).toBe('');
    expect(getPMGOldestAgeTextClass(1900)).toBe('text-yellow-400');
  });
});
