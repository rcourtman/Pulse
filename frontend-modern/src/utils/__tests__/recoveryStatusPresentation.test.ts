import { describe, expect, it } from 'vitest';
import {
  getRecoveryProtectedToggleClass,
  getRecoveryRollupStatusPillClass,
  getRecoveryRollupStatusPillLabel,
  getRecoverySpecialOutcomeTextClass,
} from '@/utils/recoveryStatusPresentation';

describe('recoveryStatusPresentation', () => {
  it('derives stale toggle classes', () => {
    expect(getRecoveryProtectedToggleClass(true)).toContain('border-amber-300');
    expect(getRecoveryProtectedToggleClass(false)).toContain('border-border');
  });

  it('derives rollup status pill labels and classes', () => {
    expect(getRecoveryRollupStatusPillLabel('recent')).toBe('recent');
    expect(getRecoveryRollupStatusPillClass('recent')).toContain('bg-blue-100/80');
    expect(getRecoveryRollupStatusPillLabel('stale')).toBe('stale');
    expect(getRecoveryRollupStatusPillClass('stale')).toContain('text-amber-700');
    expect(getRecoveryRollupStatusPillLabel('never-succeeded')).toBe('never succeeded');
    expect(getRecoveryRollupStatusPillClass('never-succeeded')).toContain('text-rose-700');
  });

  it('derives special outcome text classes', () => {
    expect(getRecoverySpecialOutcomeTextClass('never')).toContain('text-amber-600');
  });
});
