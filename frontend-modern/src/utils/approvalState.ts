import type { ApprovalRequest } from '@/api/ai';

export const getApprovalExpiryTime = (
  approval: { expiresAt?: ApprovalRequest['expiresAt'] | null },
): number | null => {
  const expiry = Date.parse(approval.expiresAt ?? '');
  return Number.isFinite(expiry) ? expiry : null;
};

export const isLivePendingApproval = (
  approval: Pick<ApprovalRequest, 'status'> & { expiresAt?: ApprovalRequest['expiresAt'] | null },
  now = Date.now(),
): boolean => {
  if (approval.status !== 'pending') {
    return false;
  }

  const expiry = getApprovalExpiryTime(approval);
  if (expiry === null) {
    return false;
  }

  return expiry > now;
};
