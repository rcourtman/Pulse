import type {
  ActionAuditRefusalPrefix,
  ActionAuditRecord,
  ActionAuditState,
  ActionVerificationResult,
} from '@/types/actionAudit';

export interface ActionAuditStatePresentation {
  label: string;
  className: string;
}

export interface ActionAuditRefusalPresentation {
  prefix: ActionAuditRefusalPrefix;
  label: string;
  detail: string;
  recordedDetail?: string;
  className: string;
}

export interface ActionAuditResultPresentation {
  kind: 'success' | 'failure' | 'refusal';
  label: string;
  reasonLabel?: string;
  detail?: string;
  recordedDetail?: string;
  className: string;
}

export interface ActionAuditVerificationOutcomePresentation {
  label: string;
  detail: string;
  evidenceSummary?: string;
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
  expired: {
    label: 'Expired',
    className:
      'bg-amber-100 text-amber-800 border-amber-200 dark:bg-amber-900 dark:text-amber-200 dark:border-amber-700',
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

const REFUSED_ACTION_STATE_PRESENTATION: ActionAuditStatePresentation = {
  label: 'Refused',
  className:
    'bg-rose-100 text-rose-800 border-rose-200 dark:bg-rose-900 dark:text-rose-200 dark:border-rose-700',
};

const ACTION_REFUSAL_PRESENTATION: Record<
  ActionAuditRefusalPrefix,
  Omit<ActionAuditRefusalPresentation, 'prefix' | 'recordedDetail'>
> = {
  'plan_drift:': {
    label: 'Plan changed',
    detail:
      'Pulse refused the action before dispatch because the approved plan no longer matched the current resource or policy state.',
    className:
      'border-rose-200 bg-rose-50 text-rose-800 dark:border-rose-800 dark:bg-rose-950/40 dark:text-rose-300',
  },
  'action_plan_expired:': {
    label: 'Approval expired',
    detail:
      'Pulse refused the action before dispatch because the approved execution window had expired.',
    className:
      'border-amber-200 bg-amber-50 text-amber-800 dark:border-amber-800 dark:bg-amber-950/40 dark:text-amber-300',
  },
  'action_dry_run_only:': {
    label: 'Dry-run only',
    detail:
      'Pulse refused to dispatch this action because the plan is limited to dry-run evidence.',
    className:
      'border-amber-200 bg-amber-50 text-amber-800 dark:border-amber-800 dark:bg-amber-950/40 dark:text-amber-300',
  },
  'resource_remediation_locked:': {
    label: 'Resource remediation locked',
    detail:
      'Pulse refused the action before dispatch because this resource is locked against automatic remediation.',
    className:
      'border-rose-200 bg-rose-50 text-rose-800 dark:border-rose-800 dark:bg-rose-950/40 dark:text-rose-300',
  },
  'policy_authorization_expired:': {
    label: 'Policy authority expired',
    detail: 'Pulse refused dispatch because the server authorization lease expired.',
    className: 'border-amber-200 bg-amber-50 text-amber-800 dark:border-amber-800 dark:bg-amber-950/40 dark:text-amber-300',
  },
  'policy_authorization_invalid:': {
    label: 'Policy authority invalid',
    detail: 'Pulse refused dispatch because current server policy authority could not be validated.',
    className: 'border-rose-200 bg-rose-50 text-rose-800 dark:border-rose-800 dark:bg-rose-950/40 dark:text-rose-300',
  },
  'policy_authorization_revoked:': {
    label: 'Policy authority revoked',
    detail: 'Pulse refused dispatch because current server policy authority was revoked.',
    className: 'border-rose-200 bg-rose-50 text-rose-800 dark:border-rose-800 dark:bg-rose-950/40 dark:text-rose-300',
  },
  'action_emergency_stop:': {
    label: 'Emergency stop active',
    detail: 'Pulse refused dispatch because the action emergency stop is active.',
    className: 'border-rose-200 bg-rose-50 text-rose-800 dark:border-rose-800 dark:bg-rose-950/40 dark:text-rose-300',
  },
  'action_replan_required:': {
    label: 'New review required',
    detail: 'Pulse refused dispatch because current policy requires a new plan and review.',
    className: 'border-amber-200 bg-amber-50 text-amber-800 dark:border-amber-800 dark:bg-amber-950/40 dark:text-amber-300',
  },
};

const ACTION_REFUSAL_PREFIXES = Object.keys(
  ACTION_REFUSAL_PRESENTATION,
) as ActionAuditRefusalPrefix[];

const VERIFICATION_OUTCOME_PRESENTATION: Record<
  'unknown' | 'verified' | 'unverified' | 'failed',
  Omit<ActionAuditVerificationOutcomePresentation, 'evidenceSummary'>
> = {
  unknown: {
    label: 'Verification unknown',
    detail: 'Pulse does not have conclusive verification evidence for this action.',
    className: 'border-border bg-surface text-base-content',
  },
  verified: {
    label: 'Legacy check passed (source unclassified)',
    detail: 'This older record does not identify an evidence source. Do not treat it as independent confirmation.',
    className:
      'border-emerald-200 bg-emerald-50 text-emerald-800 dark:border-emerald-800 dark:bg-emerald-950/40 dark:text-emerald-300',
  },
  unverified: {
    label: 'Verification not confirmed',
    detail:
      'Pulse did not receive verification evidence that confirmed the intended state.',
    className:
      'border-amber-200 bg-amber-50 text-amber-800 dark:border-amber-800 dark:bg-amber-950/40 dark:text-amber-300',
  },
  failed: {
    label: 'Verification failed',
    detail: 'Pulse could not complete the verification check after execution.',
    className:
      'border-rose-200 bg-rose-50 text-rose-800 dark:border-rose-800 dark:bg-rose-950/40 dark:text-rose-300',
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

const getActionAuditResultMessage = (
  result: Pick<NonNullable<ActionAuditRecord['result']>, 'errorMessage' | 'output'> | undefined,
): string => (result?.errorMessage || result?.output || '').trim();

export const getActionAuditRefusalPresentation = (
  audit: Pick<ActionAuditRecord, 'result'>,
): ActionAuditRefusalPresentation | undefined => {
  if (!audit.result || audit.result.success) return undefined;

  const message = getActionAuditResultMessage(audit.result);
  const normalizedMessage = message.toLowerCase();
  const prefix = ACTION_REFUSAL_PREFIXES.find((candidate) =>
    normalizedMessage.startsWith(candidate),
  );
  if (!prefix) return undefined;

  const recordedDetail = message.slice(prefix.length).trim();
  const presentation = ACTION_REFUSAL_PRESENTATION[prefix];
  return {
    prefix,
    ...presentation,
    recordedDetail: recordedDetail || undefined,
  };
};

export const getActionAuditRecordStatePresentation = (
  audit: Pick<ActionAuditRecord, 'state' | 'result'>,
): ActionAuditStatePresentation => {
  if (audit.state === 'failed' && getActionAuditRefusalPresentation(audit)) {
    return REFUSED_ACTION_STATE_PRESENTATION;
  }
  return getActionAuditStatePresentation(audit.state);
};

export const getActionAuditResultPresentation = (
  audit: Pick<ActionAuditRecord, 'result'>,
): ActionAuditResultPresentation | undefined => {
  const result = audit.result;
  if (!result) return undefined;

  const refusal = getActionAuditRefusalPresentation(audit);
  if (refusal) {
    return {
      kind: 'refusal',
      label: 'Execution refused',
      reasonLabel: refusal.label,
      detail: refusal.detail,
      recordedDetail: refusal.recordedDetail,
      className: refusal.className,
    };
  }

  const truth = result.actionResultV2?.execution;
  if (truth) {
    const presentation = {
      succeeded: { kind: 'success' as const, label: 'Execution succeeded', className: 'border-blue-200 bg-blue-50 text-blue-800 dark:border-blue-800 dark:bg-blue-950/40 dark:text-blue-200' },
      failed: { kind: 'failure' as const, label: 'Execution failed', className: 'border-red-200 bg-red-50 text-red-800 dark:border-red-800 dark:bg-red-950/40 dark:text-red-300' },
      not_run: { kind: 'failure' as const, label: 'Execution did not run', className: 'border-border bg-surface text-base-content' },
      inconclusive: { kind: 'failure' as const, label: 'Execution inconclusive', className: 'border-amber-200 bg-amber-50 text-amber-800 dark:border-amber-800 dark:bg-amber-950/40 dark:text-amber-300' },
    }[truth.status];
    return { ...presentation, detail: truth.summary?.trim() || undefined, reasonLabel: truth.reasonCode ? formatActionCapabilityLabel(truth.reasonCode) : undefined };
  }

  if (result.success) {
    return {
      kind: 'success',
      label: 'Result',
      detail: result.output?.trim() || undefined,
      className: 'border-border bg-surface text-base-content',
    };
  }

  return {
    kind: 'failure',
    label: 'Execution failed',
    detail: getActionAuditResultMessage(result) || undefined,
    className:
      'border-red-200 bg-red-50 text-red-800 dark:border-red-800 dark:bg-red-950/40 dark:text-red-300',
  };
};

export const getActionAuditVerificationOutcomePresentation = (
  audit: {
    verificationOutcome?: { status?: string; evidenceSummary?: string };
    result?: ActionAuditRecord['result'];
  },
): ActionAuditVerificationOutcomePresentation | undefined => {
  const truth = audit.result?.actionResultV2?.verification;
  if (truth) {
    const source = truth.evidenceClass === 'independent' ? 'Independent observer' : truth.evidenceClass === 'agent_attested' ? 'Executing agent (agent-attested)' : 'No evidence source';
    const statusCopy = {
      confirmed: truth.evidenceClass === 'independent'
        ? ['Confirmed by independent observer', 'An observer in a different trust domain confirmed the intended state.']
        : truth.evidenceClass === 'agent_attested'
          ? ['Confirmed by executing agent', 'The same agent trust domain that executed the action reported the intended state.']
          : ['Confirmation lacks an evidence source', 'The record says confirmed but provides no evidence source; do not treat it as independently verified.'],
      contradicted: ['Outcome contradicted', 'Observed evidence contradicted the intended state.'],
      inconclusive: ['Outcome inconclusive', 'Available evidence could not establish the intended state.'],
      not_attempted: ['Outcome not verified', 'No outcome verification was attempted.'],
    } as const;
    const [label, detail] = statusCopy[truth.status];
    return {
      label,
      detail,
      evidenceSummary: `${truth.summary?.trim() || 'No additional verification summary.'} Source: ${source}.`,
      className: truth.status === 'confirmed'
        ? 'border-emerald-200 bg-emerald-50 text-emerald-800 dark:border-emerald-800 dark:bg-emerald-950/40 dark:text-emerald-300'
        : truth.status === 'contradicted'
          ? 'border-rose-200 bg-rose-50 text-rose-800 dark:border-rose-800 dark:bg-rose-950/40 dark:text-rose-300'
          : 'border-amber-200 bg-amber-50 text-amber-800 dark:border-amber-800 dark:bg-amber-950/40 dark:text-amber-300',
    };
  }
  const outcome = audit.verificationOutcome;
  const status = (outcome?.status || '').trim().toLowerCase();
  if (!status) return undefined;

  const presentation =
    VERIFICATION_OUTCOME_PRESENTATION[
      status as keyof typeof VERIFICATION_OUTCOME_PRESENTATION
    ] ?? {
      label: 'Verification outcome recorded',
      detail: 'Pulse recorded a bounded verification status that this client does not classify.',
      className: 'border-border bg-surface text-base-content',
    };

  const evidenceSummary = outcome?.evidenceSummary?.trim();
  return {
    ...presentation,
    evidenceSummary: evidenceSummary || undefined,
  };
};

export const getActionAuditVerification = (
  audit: Pick<ActionAuditRecord, 'verification' | 'result'>,
): ActionVerificationResult | undefined => audit.verification ?? audit.result?.verification;

export const shouldRenderActionAuditVerification = (
  audit: Pick<ActionAuditRecord, 'verification' | 'result'>,
): boolean =>
  audit.result?.actionResultV2 === undefined && getActionAuditVerification(audit)?.ran === true;
