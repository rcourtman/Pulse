export interface ApprovalRiskPresentation {
  badgeClass: string;
  label: string;
}

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
