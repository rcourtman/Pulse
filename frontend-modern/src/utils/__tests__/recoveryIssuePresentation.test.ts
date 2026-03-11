import { describe, expect, it } from 'vitest';
import { getRecoveryIssueRailClass } from '@/utils/recoveryIssuePresentation';

describe('recoveryIssuePresentation', () => {
  it('returns canonical issue rail classes', () => {
    expect(getRecoveryIssueRailClass('amber')).toBe('bg-amber-400');
    expect(getRecoveryIssueRailClass('rose')).toBe('bg-rose-500');
    expect(getRecoveryIssueRailClass('blue')).toBe('bg-blue-500');
  });
});
