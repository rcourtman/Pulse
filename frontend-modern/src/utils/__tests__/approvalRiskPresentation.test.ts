import { describe, expect, it } from 'vitest';
import { getApprovalRiskPresentation } from '@/utils/approvalRiskPresentation';

describe('approvalRiskPresentation', () => {
  it('maps high and critical risk to danger styling', () => {
    expect(getApprovalRiskPresentation('high').badgeClass).toContain('red-100');
    expect(getApprovalRiskPresentation('critical').badgeClass).toContain('red-100');
  });

  it('maps medium risk to warning styling', () => {
    expect(getApprovalRiskPresentation('medium').badgeClass).toContain('amber-100');
  });

  it('maps low risk to success styling', () => {
    expect(getApprovalRiskPresentation('low').badgeClass).toContain('green-100');
  });

  it('normalizes mixed-case values', () => {
    const presentation = getApprovalRiskPresentation(' High ');
    expect(presentation.label).toBe('high');
    expect(presentation.badgeClass).toContain('red-100');
  });

  it('falls back unknown values to a neutral badge', () => {
    const presentation = getApprovalRiskPresentation('unknown');
    expect(presentation.label).toBe('unknown');
    expect(presentation.badgeClass).toContain('bg-surface-alt');
  });

  it('handles missing values as unknown', () => {
    expect(getApprovalRiskPresentation().label).toBe('unknown');
  });
});
