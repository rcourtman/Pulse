import { describe, expect, it } from 'vitest';
import { getApprovalExpiryTime, isLivePendingApproval } from '@/utils/approvalState';

describe('approvalState', () => {
  const now = Date.parse('2026-05-07T10:00:00Z');

  it('parses valid approval expiry timestamps', () => {
    expect(getApprovalExpiryTime({ expiresAt: '2026-05-07T10:05:00Z' })).toBe(
      Date.parse('2026-05-07T10:05:00Z'),
    );
  });

  it('treats malformed or missing expiry timestamps as unavailable', () => {
    expect(getApprovalExpiryTime({ expiresAt: 'not-a-date' })).toBeNull();
    expect(getApprovalExpiryTime({ expiresAt: undefined })).toBeNull();
    expect(getApprovalExpiryTime({ expiresAt: null })).toBeNull();
  });

  it('keeps only pending approvals with a future parseable expiry live', () => {
    expect(isLivePendingApproval({ status: 'pending', expiresAt: '2026-05-07T10:05:00Z' }, now)).toBe(
      true,
    );
    expect(isLivePendingApproval({ status: 'pending', expiresAt: '2026-05-07T09:59:00Z' }, now)).toBe(
      false,
    );
    expect(isLivePendingApproval({ status: 'approved', expiresAt: '2026-05-07T10:05:00Z' }, now)).toBe(
      false,
    );
  });

  it('fails closed when a pending approval expiry is malformed or missing', () => {
    expect(isLivePendingApproval({ status: 'pending', expiresAt: 'not-a-date' }, now)).toBe(false);
    expect(isLivePendingApproval({ status: 'pending', expiresAt: undefined }, now)).toBe(false);
    expect(isLivePendingApproval({ status: 'pending', expiresAt: null }, now)).toBe(false);
  });
});
