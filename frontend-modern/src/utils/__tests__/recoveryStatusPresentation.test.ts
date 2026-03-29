import { describe, expect, it } from 'vitest';
import {
  getRecoveryProtectedToggleClass,
  getRecoveryRollupInventoryStatusLabel,
  getRecoveryRollupInventoryStatusTextClass,
  getRecoveryRollupInventoryStatusVariant,
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

  it('derives inventory status labels, tones, and dot variants', () => {
    expect(getRecoveryRollupInventoryStatusLabel('healthy')).toBe('Healthy');
    expect(getRecoveryRollupInventoryStatusLabel('stale')).toBe('Stale');
    expect(getRecoveryRollupInventoryStatusLabel('never-succeeded')).toBe('Never succeeded');
    expect(getRecoveryRollupInventoryStatusTextClass('stale')).toContain('text-amber-600');
    expect(getRecoveryRollupInventoryStatusTextClass('failed')).toContain('text-red-600');
    expect(getRecoveryRollupInventoryStatusTextClass('running')).toContain('text-blue-600');
    expect(getRecoveryRollupInventoryStatusVariant('healthy')).toBe('success');
    expect(getRecoveryRollupInventoryStatusVariant('stale')).toBe('warning');
    expect(getRecoveryRollupInventoryStatusVariant('never-succeeded')).toBe('danger');
  });

  it('derives special outcome text classes', () => {
    expect(getRecoverySpecialOutcomeTextClass('never')).toContain('text-amber-600');
  });
});
