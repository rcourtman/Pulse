export interface ApprovalRiskPresentation {
  badgeClass: string;
  label: string;
}

const APPROVAL_RISK_SORT_ORDER: Record<string, number> = {
  critical: 0,
  high: 1,
  medium: 2,
  low: 3,
  unknown: 4,
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

function parseApprovalTimestamp(value?: string | null): number | null {
  const parsed = Date.parse(value ?? '');
  return Number.isFinite(parsed) ? parsed : null;
}

function getApprovalTimestampSortValue(value?: string | null): number {
  return parseApprovalTimestamp(value) ?? Number.POSITIVE_INFINITY;
}

function compareApprovalTimestamps(a?: string | null, b?: string | null): number {
  const aValue = getApprovalTimestampSortValue(a);
  const bValue = getApprovalTimestampSortValue(b);

  if (aValue === bValue) return 0;
  return aValue < bValue ? -1 : 1;
}

export function sortPendingApprovalsByUrgency<
  T extends { expiresAt?: string | null; requestedAt?: string | null; riskLevel?: string },
>(approvals: T[]): T[] {
  return [...approvals].sort((a, b) => {
    const expiryDiff = compareApprovalTimestamps(a.expiresAt, b.expiresAt);
    if (expiryDiff !== 0) return expiryDiff;

    const riskDiff = getApprovalRiskSortOrder(a.riskLevel) - getApprovalRiskSortOrder(b.riskLevel);
    if (riskDiff !== 0) return riskDiff;

    return compareApprovalTimestamps(a.requestedAt, b.requestedAt);
  });
}

export function getApprovalExpiryStatusLabel(expiresAt?: string | null, now = Date.now()): string {
  const expiryTime = parseApprovalTimestamp(expiresAt);
  if (expiryTime === null) return 'expiry unavailable';

  const diff = expiryTime - now;
  if (diff <= 0) return 'expired';

  const mins = Math.floor(diff / 60000);
  const secs = Math.floor((diff % 60000) / 1000);
  if (mins > 0) return `expires ${mins}m ${secs}s`;
  return `expires ${secs}s`;
}
