import { describe, expect, it } from 'vitest';
import {
  getApprovalRiskPresentation,
  sortPendingApprovalsByUrgency,
} from '@/utils/approvalRiskPresentation';

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

  it('sorts approvals by soonest expiry before risk', () => {
    const approvals = sortPendingApprovalsByUrgency([
      approval({ id: 'critical-later', expiresAt: '2026-05-07T10:10:00Z', riskLevel: 'critical' }),
      approval({ id: 'low-sooner', expiresAt: '2026-05-07T10:05:00Z', riskLevel: 'low' }),
    ]);

    expect(approvals.map(({ id }) => id)).toEqual(['low-sooner', 'critical-later']);
  });

  it('sorts same-expiry approvals by descending risk', () => {
    const approvals = sortPendingApprovalsByUrgency([
      approval({ id: 'low', riskLevel: 'low' }),
      approval({ id: 'high', riskLevel: 'high' }),
      approval({ id: 'critical', riskLevel: 'critical' }),
      approval({ id: 'medium', riskLevel: 'medium' }),
    ]);

    expect(approvals.map(({ id }) => id)).toEqual(['critical', 'high', 'medium', 'low']);
  });

  it('sorts same-expiry and same-risk approvals by older request time', () => {
    const approvals = sortPendingApprovalsByUrgency([
      approval({ id: 'newer', requestedAt: '2026-05-07T10:02:00Z' }),
      approval({ id: 'older', requestedAt: '2026-05-07T10:01:00Z' }),
    ]);

    expect(approvals.map(({ id }) => id)).toEqual(['older', 'newer']);
  });

  it('sorts malformed or missing expiry values after valid expiries', () => {
    const approvals = sortPendingApprovalsByUrgency([
      approval({ id: 'missing-expiry', expiresAt: undefined }),
      approval({ id: 'malformed-expiry', expiresAt: 'not-a-date' }),
      approval({ id: 'valid-expiry', expiresAt: '2026-05-07T10:05:00Z' }),
    ]);

    expect(approvals.map(({ id }) => id)).toEqual([
      'valid-expiry',
      'missing-expiry',
      'malformed-expiry',
    ]);
  });

  it('sorts malformed or missing request times after valid request times', () => {
    const approvals = sortPendingApprovalsByUrgency([
      approval({ id: 'missing-request', requestedAt: undefined }),
      approval({ id: 'malformed-request', requestedAt: 'not-a-date' }),
      approval({ id: 'valid-request', requestedAt: '2026-05-07T10:01:00Z' }),
    ]);

    expect(approvals.map(({ id }) => id)).toEqual([
      'valid-request',
      'missing-request',
      'malformed-request',
    ]);
  });
});

function approval(overrides: {
  id: string;
  expiresAt?: string | undefined;
  requestedAt?: string | undefined;
  riskLevel?: string;
}) {
  const hasOverride = (key: 'expiresAt' | 'requestedAt') =>
    Object.prototype.hasOwnProperty.call(overrides, key);

  return {
    id: overrides.id,
    expiresAt: hasOverride('expiresAt') ? overrides.expiresAt : '2026-05-07T10:05:00Z',
    requestedAt: hasOverride('requestedAt') ? overrides.requestedAt : '2026-05-07T10:00:00Z',
    riskLevel: overrides.riskLevel ?? 'medium',
  };
}
