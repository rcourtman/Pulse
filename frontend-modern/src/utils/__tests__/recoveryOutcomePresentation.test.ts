import { describe, expect, it } from 'vitest';
import {
  getRecoveryOutcomeBadgeClass,
  getRecoveryOutcomeLabel,
  normalizeRecoveryOutcome,
} from '@/utils/recoveryOutcomePresentation';

describe('recoveryOutcomePresentation', () => {
  it('normalizes known outcomes', () => {
    expect(normalizeRecoveryOutcome(' Failed ')).toBe('failed');
  });

  it('normalizes recovery aliases onto canonical outcomes', () => {
    expect(normalizeRecoveryOutcome('ok')).toBe('success');
    expect(normalizeRecoveryOutcome('warn')).toBe('warning');
    expect(normalizeRecoveryOutcome('failure')).toBe('failed');
  });

  it('falls back unknown outcomes', () => {
    expect(normalizeRecoveryOutcome('partial')).toBe('unknown');
  });

  it('maps failed outcomes to danger styling', () => {
    expect(getRecoveryOutcomeBadgeClass('failed')).toContain('red-100');
  });

  it('maps running outcomes to informational styling', () => {
    expect(getRecoveryOutcomeBadgeClass('running')).toContain('blue-100');
  });

  it('exposes canonical outcome labels', () => {
    expect(getRecoveryOutcomeLabel('success')).toBe('Healthy');
  });
});
