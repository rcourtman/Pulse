import type { UnifiedFinding } from '@/stores/aiIntelligence';
import type { ApprovalRequest } from '@/api/ai';
import type { InvestigationOutcome, InvestigationStatus } from '@/api/patrol';

const DEFAULT_BADGE_CLASSES = 'border-border bg-surface-alt text-muted';
const DEFAULT_LOOP_STATE_CLASSES = 'border-border bg-surface-alt text-muted';
const DEFAULT_FINDING_STATUS_LABEL = 'Dismissed';

const FINDING_SOURCE_LABELS: Record<string, string> = {
  threshold: 'Alert',
  'ai-patrol': 'Pulse Patrol',
  anomaly: 'Anomaly',
  'ai-chat': 'Pulse Assistant',
  correlation: 'Correlation',
  forecast: 'Forecast',
};

const FINDING_SOURCE_CLASSES: Record<string, string> = {
  threshold:
    'border-orange-200 bg-orange-50 text-orange-700 dark:border-orange-800 dark:bg-orange-900 dark:text-orange-300',
  'ai-patrol':
    'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-800 dark:bg-blue-900 dark:text-blue-300',
  anomaly:
    'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-800 dark:bg-blue-900 dark:text-blue-300',
  'ai-chat':
    'border-teal-200 bg-teal-50 text-teal-700 dark:border-teal-800 dark:bg-teal-900 dark:text-teal-300',
  correlation:
    'border-sky-200 bg-sky-50 text-sky-700 dark:border-sky-800 dark:bg-sky-900 dark:text-sky-300',
  forecast:
    'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-800 dark:bg-emerald-900 dark:text-emerald-300',
};

const FINDING_SEVERITY_CLASSES: Record<string, string> = {
  critical:
    'border-red-200 bg-red-50 text-red-700 dark:border-red-800 dark:bg-red-900 dark:text-red-300',
  warning:
    'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-800 dark:bg-amber-900 dark:text-amber-300',
  info: 'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-800 dark:bg-blue-900 dark:text-blue-300',
  watch: 'border-border bg-surface-alt text-base-content',
};

const FINDING_SEVERITY_TONE_CLASSES: Record<string, string> = {
  critical: 'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300',
  warning: 'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300',
  info: 'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300',
  watch: 'bg-surface-alt text-base-content',
};

const INVESTIGATION_STATUS_CLASSES: Record<InvestigationStatus, string> = {
  pending: 'border-border bg-surface-alt text-muted',
  running:
    'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-800 dark:bg-blue-900 dark:text-blue-300',
  completed:
    'border-green-200 bg-green-50 text-green-700 dark:border-green-800 dark:bg-green-900 dark:text-green-300',
  failed:
    'border-red-200 bg-red-50 text-red-700 dark:border-red-800 dark:bg-red-900 dark:text-red-300',
  needs_attention:
    'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-800 dark:bg-amber-900 dark:text-amber-300',
};

const INVESTIGATION_STATUS_LABELS: Record<InvestigationStatus, string> = {
  pending: 'Pending',
  running: 'Running',
  completed: 'Completed',
  failed: 'Failed',
  needs_attention: 'Needs Attention',
};

const FINDING_LOOP_STATE_CLASSES: Record<string, string> = {
  detected:
    'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-800 dark:bg-blue-900 dark:text-blue-300',
  investigating:
    'border-indigo-200 bg-indigo-50 text-indigo-700 dark:border-indigo-800 dark:bg-indigo-900 dark:text-indigo-300',
  remediation_planned:
    'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-800 dark:bg-amber-900 dark:text-amber-300',
  remediating:
    'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-800 dark:bg-amber-900 dark:text-amber-300',
  remediation_failed:
    'border-red-200 bg-red-50 text-red-700 dark:border-red-800 dark:bg-red-900 dark:text-red-300',
  needs_attention:
    'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-800 dark:bg-amber-900 dark:text-amber-300',
  timed_out: 'border-border bg-surface-alt text-base-content',
  resolved:
    'border-green-200 bg-green-50 text-green-700 dark:border-green-800 dark:bg-green-900 dark:text-green-300',
  dismissed: 'border-border bg-surface-alt text-muted',
  snoozed:
    'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-800 dark:bg-blue-900 dark:text-blue-300',
  suppressed: 'border-border bg-surface-alt text-muted',
};

const FINDING_LIFECYCLE_LABELS: Record<string, string> = {
  detected: 'Detected',
  regressed: 'Regressed',
  acknowledged: 'Acknowledged',
  snoozed: 'Snoozed',
  unsnoozed: 'Unsnoozed',
  dismissed: 'Dismissed',
  undismissed: 'Undismissed',
  suppressed: 'Suppressed',
  resolved: 'Resolved',
  auto_resolved: 'Auto-resolved',
  verification_passed: 'Fix verified',
  investigation_updated: 'Investigation updated',
  investigation_outcome: 'Investigation outcome',
  user_note_updated: 'Note updated',
  loop_state: 'Loop state changed',
  seen_while_suppressed: 'Seen while suppressed',
  loop_transition_violation: 'Invalid transition blocked',
};

const FINDING_STATUS_BADGE_CLASSES: Record<string, string> = {
  resolved:
    'border-green-200 bg-green-50 text-green-700 dark:border-green-800 dark:bg-green-900 dark:text-green-300',
  snoozed:
    'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-800 dark:bg-blue-900 dark:text-blue-300',
  dismissed: DEFAULT_BADGE_CLASSES,
};

const FINDING_STATUS_LABELS: Record<string, string> = {
  resolved: 'Resolved',
  snoozed: 'Snoozed',
  dismissed: 'Dismissed',
};

const INVESTIGATION_OUTCOME_LABELS: Record<InvestigationOutcome, string> = {
  resolved: 'Resolved',
  fix_queued: 'Fix queued',
  fix_executed: 'Fix applied',
  fix_failed: 'Fix failed',
  needs_attention: 'Needs attention',
  cannot_fix: 'Cannot auto-fix',
  timed_out: 'Timed out',
  fix_verified: 'Fix verified',
  fix_verification_failed: 'Verification failed',
  fix_verification_unknown: 'Verification unknown',
};

const INVESTIGATION_OUTCOME_CLASSES: Record<InvestigationOutcome, string> = {
  resolved:
    'border-green-200 bg-green-50 text-green-700 dark:border-green-800 dark:bg-green-900 dark:text-green-300',
  fix_queued:
    'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-800 dark:bg-blue-900 dark:text-blue-300',
  fix_executed:
    'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-800 dark:bg-emerald-900 dark:text-emerald-300',
  fix_failed:
    'border-red-200 bg-red-50 text-red-700 dark:border-red-800 dark:bg-red-900 dark:text-red-300',
  needs_attention:
    'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-800 dark:bg-amber-900 dark:text-amber-300',
  cannot_fix: 'border-border bg-surface-alt text-muted',
  timed_out: 'border-border bg-surface-alt text-base-content',
  fix_verified:
    'border-green-200 bg-green-50 text-green-700 dark:border-green-800 dark:bg-green-900 dark:text-green-300',
  fix_verification_failed:
    'border-red-200 bg-red-50 text-red-700 dark:border-red-800 dark:bg-red-900 dark:text-red-300',
  fix_verification_unknown:
    'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-800 dark:bg-amber-900 dark:text-amber-300',
};

const FINDING_SEVERITY_SORT_ORDER: Record<string, number> = {
  critical: 0,
  warning: 1,
  watch: 2,
  info: 3,
};

const FINDING_SEVERITY_COMPACT_LABELS: Record<string, string> = {
  critical: 'CRIT',
  warning: 'WARN',
  watch: 'WATCH',
  info: 'INFO',
};

const INVESTIGATION_OUTCOME_SORT_ORDER: Record<string, number> = {
  fix_verification_failed: 0,
  fix_failed: 0,
  fix_verification_unknown: 1,
  timed_out: 1,
  needs_attention: 1,
  cannot_fix: 1,
  fix_queued: 2,
};

export type FindingsFilter = 'all' | 'active' | 'resolved' | 'approvals' | 'attention';

export interface FindingFilterOption {
  value: FindingsFilter;
  label: string;
  tone?: 'default' | 'warning';
  count?: number;
}

export interface FindingEmptyStateCopy {
  title: string;
  body?: string;
}

export const getFindingSourceLabel = (source: UnifiedFinding['source'] | string): string =>
  FINDING_SOURCE_LABELS[source] || source;

export const getFindingSourceBadgeClasses = (source: UnifiedFinding['source'] | string): string =>
  FINDING_SOURCE_CLASSES[source] || FINDING_SOURCE_CLASSES['ai-patrol'];

export const getFindingSeverityBadgeClasses = (
  severity: UnifiedFinding['severity'] | string,
): string => FINDING_SEVERITY_CLASSES[severity] || DEFAULT_BADGE_CLASSES;

export const getFindingStatusBadgeClasses = (status: string): string =>
  FINDING_STATUS_BADGE_CLASSES[status] || DEFAULT_BADGE_CLASSES;

export const getFindingStatusLabel = (status: string): string =>
  FINDING_STATUS_LABELS[status] || DEFAULT_FINDING_STATUS_LABEL;

export const getFindingSeverityToneClasses = (
  severity: UnifiedFinding['severity'] | string,
): string => FINDING_SEVERITY_TONE_CLASSES[severity] || 'bg-surface-alt text-muted';

export const getFindingSeveritySortOrder = (
  severity: UnifiedFinding['severity'] | string,
): number => FINDING_SEVERITY_SORT_ORDER[severity] ?? 4;

export const getFindingSeverityCompactLabel = (
  severity: UnifiedFinding['severity'] | string,
): string => FINDING_SEVERITY_COMPACT_LABELS[severity] || String(severity).toUpperCase();

export const getInvestigationStatusBadgeClasses = (status: InvestigationStatus): string =>
  INVESTIGATION_STATUS_CLASSES[status] || DEFAULT_BADGE_CLASSES;

export const getInvestigationStatusLabel = (status: InvestigationStatus | string): string =>
  INVESTIGATION_STATUS_LABELS[status as InvestigationStatus] || String(status);

export const getInvestigationOutcomeBadgeClasses = (
  outcome: InvestigationOutcome | string,
): string => INVESTIGATION_OUTCOME_CLASSES[outcome as InvestigationOutcome] || DEFAULT_BADGE_CLASSES;

export const getInvestigationOutcomeLabel = (
  outcome: InvestigationOutcome | string,
): string => INVESTIGATION_OUTCOME_LABELS[outcome as InvestigationOutcome] || String(outcome);

export const getInvestigationOutcomeSortOrder = (
  outcome: InvestigationOutcome | string | undefined,
): number => {
  if (!outcome) {
    return 3;
  }
  return INVESTIGATION_OUTCOME_SORT_ORDER[outcome] ?? 3;
};

export const hasFindingInvestigationDetails = (
  finding: Pick<
    UnifiedFinding,
    'investigationSessionId' | 'investigationStatus' | 'investigationOutcome' | 'investigationAttempts'
  >,
): boolean =>
  Boolean(
    finding.investigationSessionId?.trim() ||
      finding.investigationStatus ||
      finding.investigationOutcome ||
      (finding.investigationAttempts ?? 0) > 0,
  );

const ATTENTION_OUTCOMES = new Set([
  'fix_verification_failed',
  'fix_verification_unknown',
  'fix_failed',
  'timed_out',
  'needs_attention',
  'cannot_fix',
]);

export const hasPendingInvestigationFixApproval = (
  findingId: string,
  approvals: Pick<ApprovalRequest, 'status' | 'toolId' | 'targetId'>[],
): boolean =>
  approvals.some(
    (approval) =>
      approval.status === 'pending' &&
      approval.toolId === 'investigation_fix' &&
      approval.targetId === findingId,
  );

export const isPatrolInvestigationFixApproval = (
  approval: Pick<ApprovalRequest, 'toolId'>,
): boolean => approval.toolId === 'investigation_fix';

export const doesFindingNeedAttention = (
  finding: Pick<UnifiedFinding, 'id' | 'status' | 'investigationOutcome'>,
  approvals: Pick<ApprovalRequest, 'status' | 'toolId' | 'targetId'>[] = [],
): boolean => {
  if (finding.status !== 'active' || !finding.investigationOutcome) {
    return false;
  }

  if (ATTENTION_OUTCOMES.has(finding.investigationOutcome)) {
    return true;
  }

  return (
    finding.investigationOutcome === 'fix_queued' &&
    !hasPendingInvestigationFixApproval(finding.id, approvals)
  );
};

export const getFindingLoopStateBadgeClasses = (loopState: string): string =>
  FINDING_LOOP_STATE_CLASSES[loopState] || DEFAULT_LOOP_STATE_CLASSES;

export const formatFindingLoopState = (loopState: string): string => loopState.replace(/_/g, ' ');

export const formatFindingLifecycleType = (value: string): string =>
  FINDING_LIFECYCLE_LABELS[value] || value.replace(/_/g, ' ');

export const getFindingResolutionReason = (
  finding: Pick<
    UnifiedFinding,
    'isThreshold' | 'source' | 'alertType' | 'investigationOutcome'
  >,
  resolvedTime: string,
): string => {
  if (finding.isThreshold || finding.source === 'threshold') {
    switch (finding.alertType || '') {
      case 'powered-off':
        return `Guest came online ${resolvedTime}`;
      case 'host-offline':
        return `Agent came online ${resolvedTime}`;
      case 'cpu':
        return `CPU returned to normal ${resolvedTime}`;
      case 'memory':
        return `Memory returned to normal ${resolvedTime}`;
      case 'disk':
        return `Disk usage returned to normal ${resolvedTime}`;
      case 'network':
        return `Network recovered ${resolvedTime}`;
      default:
        return `Condition cleared ${resolvedTime}`;
    }
  }

  if (finding.source === 'ai-patrol') {
    switch (finding.investigationOutcome) {
      case 'fix_verified':
        return `Fixed by Patrol ${resolvedTime}`;
      case 'fix_executed':
        return `Fix applied by Patrol ${resolvedTime}`;
      case 'resolved':
        return `Resolved by Patrol ${resolvedTime}`;
      case 'fix_failed':
        return `Resolved after fix failed ${resolvedTime}`;
      case 'fix_queued':
        return `Resolved while fix was pending ${resolvedTime}`;
      case 'fix_verification_failed':
        return `Resolved after failed verification ${resolvedTime}`;
      case 'fix_verification_unknown':
        return `Resolved after inconclusive verification ${resolvedTime}`;
      case 'timed_out':
        return `Resolved after investigation timeout ${resolvedTime}`;
      case 'cannot_fix':
        return `Resolved manually ${resolvedTime}`;
      case 'needs_attention':
        return `Resolved after manual review ${resolvedTime}`;
      default:
        return `Issue no longer detected ${resolvedTime}`;
    }
  }

  return `Resolved ${resolvedTime}`;
};

export const buildFindingFilterOptions = (counts: {
  needsAttentionCount: number;
  pendingApprovalCount: number;
}): FindingFilterOption[] => {
  const options: FindingFilterOption[] = [
    { value: 'active', label: 'Active' },
    { value: 'all', label: 'All' },
    { value: 'resolved', label: 'Resolved' },
  ];

  if (counts.needsAttentionCount > 0) {
    options.push({
      value: 'attention',
      label: 'Needs Attention',
      tone: 'warning',
      count: counts.needsAttentionCount,
    });
  }

  if (counts.pendingApprovalCount > 0) {
    options.push({
      value: 'approvals',
      label: 'Approvals',
      tone: 'warning',
      count: counts.pendingApprovalCount,
    });
  }

  return options;
};

export const getFindingEmptyStateCopy = (filter: FindingsFilter): FindingEmptyStateCopy => {
  switch (filter) {
    case 'active':
      return {
        title: 'No active findings',
        body: 'Your infrastructure looks healthy!',
      };
    case 'attention':
      return {
        title: 'No findings need attention right now.',
      };
    case 'approvals':
      return {
        title: 'No pending approvals.',
      };
    default:
      return {
        title: 'No Patrol findings to display',
      };
  }
};
