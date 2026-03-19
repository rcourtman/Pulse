import type { ResourceApprovalLevel } from '@/types/resource';
import { humanizeToken } from '@/utils/textPresentation';

const APPROVAL_LEVEL_LABELS: Record<ResourceApprovalLevel, string> = {
  none: 'None',
  dry_run_only: 'Dry Run Only',
  admin: 'Admin',
  mfa: 'MFA',
};

export function getResourceApprovalLevelLabel(level?: ResourceApprovalLevel | string): string {
  if (!level) {
    return '—';
  }

  const normalized = level.trim() as ResourceApprovalLevel;
  return APPROVAL_LEVEL_LABELS[normalized] ?? humanizeToken(normalized, { fallback: '—' });
}
