export interface ApprovalRiskPresentation {
  badgeClass: string;
  label: string;
}

const APPROVAL_RISK_SORT_ORDER: Record<string, number> = {
  critical: 0,
  high: 0,
  medium: 1,
  low: 2,
  unknown: 3,
};

function normalizeApprovalRiskLevel(level?: string): string {
  const normalized = level?.trim().toLowerCase();
  if (!normalized) return 'unknown';
  return normalized;
}

export function getApprovalRiskPresentation(level?: string): ApprovalRiskPresentation {
  const normalized = normalizeApprovalRiskLevel(level);

  switch (normalized) {
    case 'critical':
    case 'high':
      return {
        badgeClass: 'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300',
        label: normalized,
      };
    case 'medium':
      return {
        badgeClass: 'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300',
        label: normalized,
      };
    case 'low':
      return {
        badgeClass: 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300',
        label: normalized,
      };
    default:
      return {
        badgeClass: 'bg-surface-alt text-muted',
        label: normalized,
      };
  }
}

export function getApprovalRiskSortOrder(level?: string): number {
  const normalized = normalizeApprovalRiskLevel(level);
  return APPROVAL_RISK_SORT_ORDER[normalized] ?? APPROVAL_RISK_SORT_ORDER.unknown;
}

export function sortPendingApprovalsByUrgency<
  T extends { expiresAt: string; requestedAt: string; riskLevel?: string },
>(approvals: T[]): T[] {
  return [...approvals].sort((a, b) => {
    const expiryDiff = new Date(a.expiresAt).getTime() - new Date(b.expiresAt).getTime();
    if (expiryDiff !== 0) return expiryDiff;

    const riskDiff = getApprovalRiskSortOrder(a.riskLevel) - getApprovalRiskSortOrder(b.riskLevel);
    if (riskDiff !== 0) return riskDiff;

    return new Date(a.requestedAt).getTime() - new Date(b.requestedAt).getTime();
  });
}
