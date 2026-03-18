import type { ApprovalRequest } from '@/api/ai';

export const getApprovalExpiryTime = (
  approval: Pick<ApprovalRequest, 'expiresAt'>,
): number | null => {
  const expiry = Date.parse(approval.expiresAt);
  return Number.isFinite(expiry) ? expiry : null;
};

export const isLivePendingApproval = (
  approval: Pick<ApprovalRequest, 'status' | 'expiresAt'>,
  now = Date.now(),
): boolean => {
  if (approval.status !== 'pending') {
    return false;
  }

  const expiry = getApprovalExpiryTime(approval);
  if (expiry === null) {
    return true;
  }

  return expiry > now;
};
