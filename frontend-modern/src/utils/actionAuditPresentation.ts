import type {
  ActionAuditRecord,
  ActionAuditState,
  ActionVerificationResult,
} from '@/types/actionAudit';

export interface ActionAuditStatePresentation {
  label: string;
  className: string;
}

const ACTION_STATE_PRESENTATION: Record<ActionAuditState, ActionAuditStatePresentation> = {
  planned: {
    label: 'Planned',
    className: 'bg-surface-alt text-muted border-border',
  },
  pending_approval: {
    label: 'Pending approval',
    className:
      'bg-amber-100 text-amber-800 border-amber-200 dark:bg-amber-900 dark:text-amber-200 dark:border-amber-700',
  },
  approved: {
    label: 'Approved',
    className:
      'bg-blue-100 text-blue-800 border-blue-200 dark:bg-blue-900 dark:text-blue-200 dark:border-blue-700',
  },
  rejected: {
    label: 'Rejected',
    className:
      'bg-red-100 text-red-800 border-red-200 dark:bg-red-900 dark:text-red-200 dark:border-red-700',
  },
  executing: {
    label: 'Executing',
    className:
      'bg-sky-100 text-sky-800 border-sky-200 dark:bg-sky-900 dark:text-sky-200 dark:border-sky-700',
  },
  completed: {
    label: 'Completed',
    className:
      'bg-emerald-100 text-emerald-800 border-emerald-200 dark:bg-emerald-900 dark:text-emerald-200 dark:border-emerald-700',
  },
  failed: {
    label: 'Failed',
    className:
      'bg-red-100 text-red-800 border-red-200 dark:bg-red-900 dark:text-red-200 dark:border-red-700',
  },
};

export const getActionAuditStatePresentation = (
  state: ActionAuditState | string | undefined,
): ActionAuditStatePresentation =>
  ACTION_STATE_PRESENTATION[state as ActionAuditState] ?? {
    label: 'Unknown',
    className: 'bg-surface-alt text-muted border-border',
  };

export const formatActionCapabilityLabel = (capabilityName: string | undefined): string => {
  const normalized = (capabilityName || '').trim();
  if (!normalized) return 'Action';

  return normalized
    .replace(/[._-]+/g, ' ')
    .split(/\s+/)
    .filter(Boolean)
    .map((word) => `${word.slice(0, 1).toUpperCase()}${word.slice(1)}`)
    .join(' ');
};

export const formatActionApprovalPolicyLabel = (policy: string | undefined): string => {
  switch ((policy || '').trim()) {
    case 'none':
      return 'No approval';
    case 'dry_run_only':
      return 'Dry run only';
    case 'admin':
      return 'Admin approval';
    case 'mfa':
      return 'MFA approval';
    default:
      return formatActionCapabilityLabel(policy || 'Policy');
  }
};

export const getActionAuditVerification = (
  audit: Pick<ActionAuditRecord, 'verification' | 'result'>,
): ActionVerificationResult | undefined => audit.verification ?? audit.result?.verification;

export const shouldRenderActionAuditVerification = (
  audit: Pick<ActionAuditRecord, 'verification' | 'result'>,
): boolean => getActionAuditVerification(audit)?.ran === true;
