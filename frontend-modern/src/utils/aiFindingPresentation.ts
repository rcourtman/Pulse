import type { UnifiedFinding } from '@/stores/aiIntelligence';
import type { ApprovalRequest } from '@/api/ai';
import type { InvestigationOutcome, InvestigationStatus } from '@/api/patrol';
import type { MetadataBadgeTone } from '@/components/shared/MetadataBadge';
import { isLivePendingApproval } from '@/utils/approvalState';
import { getPatrolProviderSettingsAction } from '@/utils/patrolRuntimeActions';
import { formatIdentifierLabel } from '@/utils/textPresentation';

const DEFAULT_BADGE_CLASSES = 'border-border bg-surface-alt text-muted';
const DEFAULT_LOOP_STATE_CLASSES = 'border-border bg-surface-alt text-muted';
const DEFAULT_FINDING_STATUS_LABEL = 'Dismissed';
const DEFAULT_BADGE_TONE: MetadataBadgeTone = 'muted';

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

const FINDING_SOURCE_TONES: Record<string, MetadataBadgeTone> = {
  threshold: 'orange',
  'ai-patrol': 'info',
  anomaly: 'info',
  'ai-chat': 'teal',
  correlation: 'sky',
  forecast: 'success',
};

const FINDING_SEVERITY_CLASSES: Record<string, string> = {
  critical:
    'border-red-200 bg-red-50 text-red-700 dark:border-red-800 dark:bg-red-900 dark:text-red-300',
  warning:
    'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-800 dark:bg-amber-900 dark:text-amber-300',
  info: 'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-800 dark:bg-blue-900 dark:text-blue-300',
  watch: 'border-border bg-surface-alt text-base-content',
};

const FINDING_SEVERITY_TONES: Record<string, MetadataBadgeTone> = {
  critical: 'danger',
  warning: 'warning',
  info: 'info',
  watch: 'neutral',
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

const INVESTIGATION_STATUS_TONES: Record<InvestigationStatus, MetadataBadgeTone> = {
  pending: 'muted',
  running: 'info',
  completed: 'success',
  failed: 'danger',
  needs_attention: 'warning',
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

const FINDING_LOOP_STATE_TONES: Record<string, MetadataBadgeTone> = {
  detected: 'info',
  investigating: 'indigo',
  remediation_planned: 'warning',
  remediating: 'warning',
  remediation_failed: 'danger',
  needs_attention: 'warning',
  timed_out: 'neutral',
  resolved: 'success',
  dismissed: 'muted',
  snoozed: 'info',
  suppressed: 'muted',
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
  content_replaced: 'Re-detected with different details',
};

const FINDING_STATUS_BADGE_CLASSES: Record<string, string> = {
  resolved:
    'border-green-200 bg-green-50 text-green-700 dark:border-green-800 dark:bg-green-900 dark:text-green-300',
  snoozed:
    'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-800 dark:bg-blue-900 dark:text-blue-300',
  dismissed: DEFAULT_BADGE_CLASSES,
};

const FINDING_STATUS_BADGE_TONES: Record<string, MetadataBadgeTone> = {
  resolved: 'success',
  snoozed: 'info',
  dismissed: 'muted',
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
  fix_rejected: 'Fix rejected',
  needs_attention: 'Needs attention',
  cannot_fix: 'Cannot remediate',
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
  fix_rejected:
    'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-800 dark:bg-amber-900 dark:text-amber-300',
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

const INVESTIGATION_OUTCOME_TONES: Record<InvestigationOutcome, MetadataBadgeTone> = {
  resolved: 'success',
  fix_queued: 'info',
  fix_executed: 'success',
  fix_failed: 'danger',
  fix_rejected: 'warning',
  needs_attention: 'warning',
  cannot_fix: 'muted',
  timed_out: 'neutral',
  fix_verified: 'success',
  fix_verification_failed: 'danger',
  fix_verification_unknown: 'warning',
};

const FINDING_SEVERITY_SORT_ORDER: Record<string, number> = {
  critical: 0,
  warning: 1,
  watch: 2,
  info: 3,
};

const FINDING_RESOURCE_CRITICALITY_SORT_ORDER: Record<string, number> = {
  high: 0,
  medium: 1,
  '': 2,
  low: 3,
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
  fix_rejected: 1,
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

export interface FindingRecencyPresentation {
  label: string;
  timestamp: string;
}

export interface PatrolFindingClassification {
  kind: 'runtime' | 'infrastructure';
  label: string;
  badgeClasses: string;
}

export interface FindingSubjectPresentation {
  label: string;
}

export interface FindingTitlePresentation {
  label: string;
}

export interface FindingPrimaryActionPresentation {
  label: string;
  href: string;
}

export interface FindingManualControlsPresentation {
  acknowledge: boolean;
  snooze: boolean;
  dismiss: boolean;
}

export interface PatrolFindingsBadgePresentation {
  tone: 'danger' | 'warning' | 'info' | 'muted';
}

export interface PatrolFindingDisplayGroup<TFinding> {
  id: string;
  resourceKey: string;
  resourceLabel: string;
  primaryFinding: TFinding;
  relatedFindings: TFinding[];
  findings: TFinding[];
}

export interface FindingSeverityPresentation {
  label: string;
  badgeClasses: string;
  badgeTone: MetadataBadgeTone;
  uppercase: boolean;
}

export interface FindingCompactBadgePresentation {
  label: string;
  badgeClasses: string;
}

export type FindingPatrolWorkflowStage =
  | 'investigating'
  | 'approval'
  | 'verification'
  | 'attention'
  | 'recorded'
  | 'paused';

export interface FindingPatrolWorkflowPresentation {
  stage: FindingPatrolWorkflowStage;
  label: string;
  detail: string;
  tone: MetadataBadgeTone;
}

export const getFindingSourceLabel = (source: UnifiedFinding['source'] | string): string =>
  FINDING_SOURCE_LABELS[source] || source;

export const getFindingSourceBadgeClasses = (source: UnifiedFinding['source'] | string): string =>
  FINDING_SOURCE_CLASSES[source] || FINDING_SOURCE_CLASSES['ai-patrol'];

export const getFindingSourceBadgeTone = (
  source: UnifiedFinding['source'] | string,
): MetadataBadgeTone => FINDING_SOURCE_TONES[source] || FINDING_SOURCE_TONES['ai-patrol'];

export const getFindingSeverityBadgeClasses = (
  severity: UnifiedFinding['severity'] | string,
): string => FINDING_SEVERITY_CLASSES[severity] || DEFAULT_BADGE_CLASSES;

export const getFindingSeverityBadgeTone = (
  severity: UnifiedFinding['severity'] | string,
): MetadataBadgeTone => FINDING_SEVERITY_TONES[severity] || DEFAULT_BADGE_TONE;

export const getFindingSeverityPresentation = (
  finding: Pick<UnifiedFinding, 'severity' | 'resourceId' | 'resourceName' | 'title'>,
): FindingSeverityPresentation => {
  if (!isPatrolRuntimeFinding(finding)) {
    return {
      label: String(finding.severity),
      badgeClasses: getFindingSeverityBadgeClasses(finding.severity),
      badgeTone: getFindingSeverityBadgeTone(finding.severity),
      uppercase: true,
    };
  }

  if (finding.severity === 'critical') {
    return {
      label: 'Runtime critical',
      badgeClasses:
        'border-red-200 bg-red-50 text-red-700 dark:border-red-800 dark:bg-red-900 dark:text-red-300',
      badgeTone: 'danger',
      uppercase: false,
    };
  }

  return {
    label: 'Runtime issue',
    badgeClasses:
      'border-sky-200 bg-sky-50 text-sky-700 dark:border-sky-800 dark:bg-sky-900 dark:text-sky-300',
    badgeTone: 'sky',
    uppercase: false,
  };
};

export const getFindingStatusBadgeClasses = (status: string): string =>
  FINDING_STATUS_BADGE_CLASSES[status] || DEFAULT_BADGE_CLASSES;

export const getFindingStatusBadgeTone = (status: string): MetadataBadgeTone =>
  FINDING_STATUS_BADGE_TONES[status] || DEFAULT_BADGE_TONE;

export const getFindingStatusLabel = (status: string): string =>
  FINDING_STATUS_LABELS[status] || DEFAULT_FINDING_STATUS_LABEL;

export const getFindingSeverityToneClasses = (
  severity: UnifiedFinding['severity'] | string,
): string => FINDING_SEVERITY_TONE_CLASSES[severity] || 'bg-surface-alt text-muted';

export const getFindingSeveritySortOrder = (
  severity: UnifiedFinding['severity'] | string,
): number => FINDING_SEVERITY_SORT_ORDER[severity] ?? 4;

export const getFindingResourceCriticalitySortOrder = (
  criticality: UnifiedFinding['resourceCriticality'] | string | undefined,
): number => {
  const normalized = String(criticality || '')
    .trim()
    .toLowerCase();
  return FINDING_RESOURCE_CRITICALITY_SORT_ORDER[normalized] ?? 2;
};

export const getFindingActiveRuntimeSortOrder = (
  finding: Pick<UnifiedFinding, 'status' | 'resourceId' | 'resourceName' | 'title'>,
): number => (finding.status === 'active' && isPatrolRuntimeFinding(finding) ? 0 : 1);

export const getPatrolFindingsBadgePresentation = (
  findings: Pick<UnifiedFinding, 'status' | 'severity' | 'resourceId' | 'resourceName' | 'title'>[],
): PatrolFindingsBadgePresentation => {
  const activeFindings = findings.filter((finding) => finding.status === 'active');
  if (
    activeFindings.some(
      (finding) => finding.severity === 'critical' && !isPatrolRuntimeFinding(finding),
    )
  ) {
    return { tone: 'danger' };
  }
  if (
    activeFindings.some(
      (finding) => finding.severity === 'critical' && isPatrolRuntimeFinding(finding),
    )
  ) {
    return { tone: 'info' };
  }
  if (
    activeFindings.some(
      (finding) => finding.severity === 'warning' && !isPatrolRuntimeFinding(finding),
    )
  ) {
    return { tone: 'warning' };
  }
  if (
    activeFindings.some(
      (finding) => finding.severity === 'warning' && isPatrolRuntimeFinding(finding),
    )
  ) {
    return { tone: 'info' };
  }

  return { tone: 'muted' };
};

export const getFindingSeverityCompactLabel = (
  severity: UnifiedFinding['severity'] | string,
): string => FINDING_SEVERITY_COMPACT_LABELS[severity] || String(severity).toUpperCase();

export const isPatrolRuntimeFinding = (
  finding: Pick<UnifiedFinding, 'resourceId' | 'resourceName' | 'title'>,
): boolean => {
  const resourceId = String(finding.resourceId || '')
    .trim()
    .toLowerCase();
  const resourceName = String(finding.resourceName || '')
    .trim()
    .toLowerCase();
  const title = String(finding.title || '')
    .trim()
    .toLowerCase();

  return (
    resourceId === 'ai-service' ||
    resourceName === 'pulse patrol service' ||
    title.startsWith('pulse patrol:')
  );
};

export const normalizePatrolRuntimeFindingLabel = (title: string | undefined): string => {
  const rawTitle = String(title || '').trim();
  const normalizedTitle = rawTitle.replace(/^Pulse Patrol:\s*/i, '').trim();

  if (/^insufficient api credits$/i.test(normalizedTitle)) {
    return 'Provider billing or quota issue';
  }

  return normalizedTitle || rawTitle;
};

export const getPatrolFindingClassification = (
  finding: Pick<UnifiedFinding, 'resourceId' | 'resourceName' | 'title'>,
): PatrolFindingClassification =>
  isPatrolRuntimeFinding(finding)
    ? {
        kind: 'runtime',
        label: 'Patrol runtime',
        badgeClasses:
          'border-sky-200 bg-sky-50 text-sky-700 dark:border-sky-800 dark:bg-sky-900 dark:text-sky-300',
      }
    : {
        kind: 'infrastructure',
        label: 'Infrastructure',
        badgeClasses: DEFAULT_BADGE_CLASSES,
      };

export const getFindingSubjectPresentation = (
  finding: Pick<UnifiedFinding, 'resourceId' | 'resourceName' | 'resourceType' | 'title'>,
): FindingSubjectPresentation => {
  if (isPatrolRuntimeFinding(finding)) {
    return { label: 'Patrol runtime' };
  }

  const resourceName =
    String(finding.resourceName || '').trim() || String(finding.resourceId || '').trim();
  const resourceType = String(finding.resourceType || '').trim();

  if (!resourceType) {
    return { label: resourceName };
  }

  return {
    label: `${resourceName} (${formatIdentifierLabel(resourceType)})`,
  };
};

export const getPatrolFindingResourceGroupKey = (
  finding: Pick<UnifiedFinding, 'id' | 'resourceId' | 'resourceName' | 'resourceType' | 'title'>,
): string => {
  const resourceName = String(finding.resourceName || '').trim();
  const resourceType = String(finding.resourceType || '').trim();
  if (resourceName || resourceType) {
    return `subject:${getFindingSubjectPresentation(finding).label.toLowerCase()}`;
  }

  const resourceId = String(finding.resourceId || '').trim();
  if (resourceId) {
    return `id:${resourceId}`;
  }

  return `finding:${finding.id}`;
};

export function buildPatrolFindingDisplayGroups<
  TFinding extends Pick<
    UnifiedFinding,
    'id' | 'resourceId' | 'resourceName' | 'resourceType' | 'title'
  >,
>(findings: readonly TFinding[]): PatrolFindingDisplayGroup<TFinding>[] {
  const groups: PatrolFindingDisplayGroup<TFinding>[] = [];
  const groupsByResourceKey = new Map<string, PatrolFindingDisplayGroup<TFinding>>();

  for (const finding of findings) {
    const resourceKey = getPatrolFindingResourceGroupKey(finding);
    const existing = groupsByResourceKey.get(resourceKey);
    if (existing) {
      existing.findings.push(finding);
      existing.relatedFindings.push(finding);
      continue;
    }

    const group: PatrolFindingDisplayGroup<TFinding> = {
      id: resourceKey,
      resourceKey,
      resourceLabel: getFindingSubjectPresentation(finding).label,
      primaryFinding: finding,
      relatedFindings: [],
      findings: [finding],
    };
    groupsByResourceKey.set(resourceKey, group);
    groups.push(group);
  }

  return groups;
}

export type PatrolFindingWorkType = 'approval' | 'failed' | 'in_progress' | 'recurring' | 'new';

const isFailedFixOutcome = (outcome: string | undefined): boolean =>
  outcome === 'fix_failed' ||
  outcome === 'fix_verification_failed' ||
  outcome === 'cannot_fix' ||
  outcome === 'timed_out';

export function classifyPatrolFindingWorkType(
  finding: Pick<
    UnifiedFinding,
    'status' | 'investigationStatus' | 'investigationOutcome' | 'regressionCount' | 'timesRaised'
  >,
): PatrolFindingWorkType {
  if (finding.status !== 'active') return 'new';

  if (finding.investigationOutcome === 'fix_queued') return 'approval';

  if (isFailedFixOutcome(finding.investigationOutcome)) return 'failed';

  if (
    finding.investigationStatus === 'running' ||
    finding.investigationOutcome === 'fix_executed'
  ) {
    return 'in_progress';
  }

  if ((finding.regressionCount ?? 0) > 0 || (finding.timesRaised ?? 0) > 1) {
    return 'recurring';
  }

  return 'new';
}

export interface PatrolWorkTypeComposition {
  total: number;
  approval: number;
  failed: number;
  inProgress: number;
  recurring: number;
  newIssues: number;
}

export function getPatrolWorkTypeComposition<
  TFinding extends Pick<
    UnifiedFinding,
    'status' | 'investigationStatus' | 'investigationOutcome' | 'regressionCount' | 'timesRaised'
  >,
>(findings: readonly TFinding[]): PatrolWorkTypeComposition {
  const composition: PatrolWorkTypeComposition = {
    total: findings.length,
    approval: 0,
    failed: 0,
    inProgress: 0,
    recurring: 0,
    newIssues: 0,
  };
  for (const finding of findings) {
    switch (classifyPatrolFindingWorkType(finding)) {
      case 'approval':
        composition.approval++;
        break;
      case 'failed':
        composition.failed++;
        break;
      case 'in_progress':
        composition.inProgress++;
        break;
      case 'recurring':
        composition.recurring++;
        break;
      case 'new':
        composition.newIssues++;
        break;
    }
  }
  return composition;
}

export function getPatrolWorkTypeCompositionClause(composition: PatrolWorkTypeComposition): string {
  const parts: string[] = [];
  if (composition.approval > 0) {
    parts.push(`${composition.approval} need${composition.approval === 1 ? 's' : ''} approval`);
  }
  if (composition.failed > 0) {
    parts.push(`${composition.failed} failed fix${composition.failed === 1 ? '' : 'es'}`);
  }
  if (composition.inProgress > 0) {
    parts.push(`${composition.inProgress} in progress`);
  }
  if (composition.recurring > 0) {
    parts.push(`${composition.recurring} recurring`);
  }
  if (parts.length === 0) return '';
  return ` — ${parts.join(', ')}`;
}

export interface PatrolActionableStatePresentation {
  label: string;
  tone: MetadataBadgeTone;
}

export type PatrolFindingRowScaffoldItemId =
  | 'affected'
  | 'checked'
  | 'next-step'
  | 'problem'
  | 'verification'
  | 'workflow'
  | 'why';

export interface PatrolFindingRowScaffoldItem {
  id: PatrolFindingRowScaffoldItemId;
  label: string;
  value: string;
}

export interface PatrolFindingRowScaffold {
  items: PatrolFindingRowScaffoldItem[];
}

export function getPatrolFindingActionableState(
  finding: Pick<UnifiedFinding, 'status' | 'investigationStatus' | 'investigationOutcome'>,
): PatrolActionableStatePresentation | undefined {
  if (finding.status !== 'active') return undefined;

  if (finding.investigationOutcome === 'fix_queued') {
    return { label: 'Approval required', tone: 'warning' };
  }

  if (isFailedFixOutcome(finding.investigationOutcome)) {
    return { label: 'Fix failed', tone: 'danger' };
  }

  if (finding.investigationStatus === 'running') {
    return { label: 'Investigating', tone: 'info' };
  }

  if (finding.investigationOutcome === 'fix_executed') {
    return { label: 'Verifying fix', tone: 'info' };
  }

  return undefined;
}

function getPatrolFindingVerificationSummary(
  finding: Pick<UnifiedFinding, 'investigationStatus' | 'investigationOutcome'>,
): string {
  switch (finding.investigationOutcome) {
    case 'fix_queued':
      return 'Waiting for approval before any fix runs.';
    case 'fix_executed':
      return 'Fix ran; verification is in progress.';
    case 'fix_verified':
    case 'resolved':
      return 'Verified outcome recorded.';
    case 'fix_verification_failed':
      return 'Verification failed and needs review.';
    case 'fix_verification_unknown':
      return 'Verification was inconclusive.';
    case 'fix_failed':
    case 'cannot_fix':
    case 'timed_out':
    case 'needs_attention':
      return 'No verified fix; action needs review.';
    case 'fix_rejected':
      return 'No change ran because the fix was rejected.';
    default:
      break;
  }

  if (finding.investigationStatus === 'running' || finding.investigationStatus === 'pending') {
    return 'Patrol is investigating; no fix has run yet.';
  }

  return 'No fix has run yet.';
}

function getPatrolFindingWorkflowSummary(
  workflow: FindingPatrolWorkflowPresentation | undefined,
): string {
  if (!workflow) {
    return 'Review evidence, decide the next action, and verify any outcome before closing.';
  }

  switch (workflow.stage) {
    case 'approval':
      return workflow.label === 'Approve or reject'
        ? 'Review evidence first; no change runs until the proposed fix is approved, then Patrol verifies the outcome.'
        : 'Recover the queued fix before any action can run, then verify the outcome after a decision.';
    case 'verification':
      return 'The governed action ran; review follow-up evidence before closing the issue.';
    case 'attention':
      return 'Review the blocked or failed step before approving another change or resolving manually.';
    case 'investigating':
      return 'Patrol is explaining the issue and preparing the next decision point.';
    case 'recorded':
      return 'Patrol recorded the outcome; use history if you need the completed trail.';
    case 'paused':
      return 'Patrol will return this issue to the workflow when the reminder expires.';
  }
}

const getNonEmptyPresentationText = (
  value: string | undefined,
  fallback: string,
): string => {
  const normalized = String(value || '').trim();
  return normalized || fallback;
};

export function getPatrolFindingRowScaffold(
  finding: Pick<
    UnifiedFinding,
    | 'description'
    | 'evidence'
    | 'id'
    | 'impact'
    | 'investigationOutcome'
    | 'investigationStatus'
    | 'loopState'
    | 'recommendation'
    | 'resourceId'
    | 'resourceName'
    | 'resourceType'
    | 'source'
    | 'status'
    | 'title'
  >,
  approvals: Pick<ApprovalRequest, 'status' | 'toolId' | 'targetId' | 'expiresAt'>[] = [],
  now = Date.now(),
): PatrolFindingRowScaffold | undefined {
  if (
    finding.source !== 'ai-patrol' ||
    finding.status !== 'active' ||
    isPatrolRuntimeFinding(finding)
  ) {
    return undefined;
  }

  const title = getNonEmptyPresentationText(
    getFindingTitlePresentation(finding).label,
    'Current Patrol issue',
  );
  const subject = getNonEmptyPresentationText(
    getFindingSubjectPresentation(finding).label,
    'Affected resource not specified',
  );
  const workflow = getFindingPatrolWorkflowPresentation(finding, approvals, now);

  return {
    items: [
      {
        id: 'problem',
        label: 'Problem',
        value: title,
      },
      {
        id: 'affected',
        label: 'Affected',
        value: subject,
      },
      {
        id: 'why',
        label: 'Why it matters',
        value: getNonEmptyPresentationText(
          finding.impact || finding.description,
          'Review this Patrol finding before making infrastructure changes.',
        ),
      },
      {
        id: 'checked',
        label: 'What Pulse checked',
        value: getNonEmptyPresentationText(
          finding.evidence,
          'Patrol recorded this from the current check.',
        ),
      },
      {
        id: 'workflow',
        label: 'Safe workflow',
        value: getPatrolFindingWorkflowSummary(workflow),
      },
      {
        id: 'next-step',
        label: 'Recommended next step',
        value: getNonEmptyPresentationText(
          workflow?.detail || finding.recommendation,
          'Open details to review evidence and decide the next action.',
        ),
      },
      {
        id: 'verification',
        label: 'Verification',
        value: getPatrolFindingVerificationSummary(finding),
      },
    ],
  };
}

export const getPatrolFindingIssueCountLabel = (count: number): string => {
  const normalized = Number.isFinite(count) ? Math.max(0, Math.trunc(count)) : 0;
  if (normalized === 1) {
    return '1 issue';
  }
  return `${normalized} issues`;
};

export const getFindingTitlePresentation = (
  finding: Pick<UnifiedFinding, 'resourceId' | 'resourceName' | 'title'>,
): FindingTitlePresentation => {
  const rawTitle = String(finding.title || '').trim();
  if (!isPatrolRuntimeFinding(finding)) {
    return { label: rawTitle };
  }

  const normalizedTitle = normalizePatrolRuntimeFindingLabel(rawTitle);
  return {
    label: normalizedTitle || rawTitle || 'Patrol runtime issue',
  };
};

export const getFindingPrimaryActionPresentation = (
  finding: Pick<UnifiedFinding, 'resourceId' | 'resourceName' | 'title'>,
): FindingPrimaryActionPresentation | undefined => {
  if (isPatrolRuntimeFinding(finding)) {
    return getPatrolProviderSettingsAction();
  }

  return undefined;
};

export const getFindingManualControlsPresentation = (
  finding: Pick<UnifiedFinding, 'resourceId' | 'resourceName' | 'title'>,
): FindingManualControlsPresentation =>
  isPatrolRuntimeFinding(finding)
    ? {
        acknowledge: false,
        snooze: false,
        dismiss: false,
      }
    : {
        acknowledge: true,
        snooze: true,
        dismiss: true,
      };

export const sortFindingsForAttentionQueue = (findings: UnifiedFinding[]): UnifiedFinding[] =>
  [...findings].sort((a, b) => {
    const aOutcome =
      a.status === 'active' && a.investigationOutcome
        ? getInvestigationOutcomeSortOrder(a.investigationOutcome)
        : 3;
    const bOutcome =
      b.status === 'active' && b.investigationOutcome
        ? getInvestigationOutcomeSortOrder(b.investigationOutcome)
        : 3;
    if (aOutcome !== bOutcome) return aOutcome - bOutcome;

    const aSeverity = getFindingSeveritySortOrder(a.severity);
    const bSeverity = getFindingSeveritySortOrder(b.severity);
    if (aSeverity !== bSeverity) return aSeverity - bSeverity;

    const aResourceCriticality = getFindingResourceCriticalitySortOrder(a.resourceCriticality);
    const bResourceCriticality = getFindingResourceCriticalitySortOrder(b.resourceCriticality);
    if (aResourceCriticality !== bResourceCriticality) {
      return aResourceCriticality - bResourceCriticality;
    }

    const aRuntime = getFindingActiveRuntimeSortOrder(a);
    const bRuntime = getFindingActiveRuntimeSortOrder(b);
    if (aRuntime !== bRuntime) return aRuntime - bRuntime;

    const aRecency = getFindingRecencyPresentation(a);
    const bRecency = getFindingRecencyPresentation(b);
    return new Date(bRecency.timestamp).getTime() - new Date(aRecency.timestamp).getTime();
  });

export const getFindingRecencyPresentation = (
  finding: Pick<UnifiedFinding, 'status' | 'detectedAt' | 'lastSeenAt'>,
): FindingRecencyPresentation => {
  if (finding.status === 'active' && finding.lastSeenAt) {
    return {
      label: 'last seen',
      timestamp: finding.lastSeenAt,
    };
  }

  return {
    label: 'detected',
    timestamp: finding.detectedAt,
  };
};

export const getInvestigationStatusBadgeClasses = (status: InvestigationStatus): string =>
  INVESTIGATION_STATUS_CLASSES[status] || DEFAULT_BADGE_CLASSES;

export const getInvestigationStatusBadgeTone = (
  status: InvestigationStatus | string,
): MetadataBadgeTone =>
  INVESTIGATION_STATUS_TONES[status as InvestigationStatus] || DEFAULT_BADGE_TONE;

export const getInvestigationStatusLabel = (status: InvestigationStatus | string): string =>
  INVESTIGATION_STATUS_LABELS[status as InvestigationStatus] || String(status);

export const getInvestigationOutcomeBadgeClasses = (
  outcome: InvestigationOutcome | string,
): string =>
  INVESTIGATION_OUTCOME_CLASSES[outcome as InvestigationOutcome] || DEFAULT_BADGE_CLASSES;

export const getInvestigationOutcomeBadgeTone = (
  outcome: InvestigationOutcome | string,
): MetadataBadgeTone =>
  INVESTIGATION_OUTCOME_TONES[outcome as InvestigationOutcome] || DEFAULT_BADGE_TONE;

export const getInvestigationOutcomeLabel = (outcome: InvestigationOutcome | string): string =>
  INVESTIGATION_OUTCOME_LABELS[outcome as InvestigationOutcome] || String(outcome);

// Confidence badge classes mirror severity intuition: high is reassuringly
// emphasized, medium is neutral, low is a soft amber so the operator notices
// when the trust signal is weak.
const INVESTIGATION_CONFIDENCE_CLASSES: Record<string, string> = {
  high: 'border-emerald-300 bg-emerald-50 text-emerald-800 dark:border-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-300',
  medium: 'border-border bg-surface-alt text-base-content',
  low: 'border-amber-300 bg-amber-50 text-amber-800 dark:border-amber-700 dark:bg-amber-950/40 dark:text-amber-300',
};

const INVESTIGATION_CONFIDENCE_TONES: Record<string, MetadataBadgeTone> = {
  high: 'success',
  medium: 'neutral',
  low: 'warning',
};

export const getInvestigationConfidenceBadgeClasses = (confidence: string): string =>
  INVESTIGATION_CONFIDENCE_CLASSES[confidence] || DEFAULT_BADGE_CLASSES;

export const getInvestigationConfidenceBadgeTone = (confidence: string): MetadataBadgeTone =>
  INVESTIGATION_CONFIDENCE_TONES[confidence] || DEFAULT_BADGE_TONE;

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
    | 'investigationSessionId'
    | 'investigationStatus'
    | 'investigationOutcome'
    | 'investigationAttempts'
  >,
): boolean =>
  Boolean(
    finding.investigationSessionId?.trim() ||
      finding.investigationStatus ||
      finding.investigationOutcome ||
      (finding.investigationAttempts ?? 0) > 0,
  );

// hasFindingInvestigationHandoffPointer is the narrower check used by the
// proposed-fix briefing path: it asks whether any finding-side reference
// exists that would let the assistant resume an investigation, regardless
// of investigationStatus. Callers OR this with their own approval-side
// pointer when one applies.
export const hasFindingInvestigationHandoffPointer = (
  finding: Pick<
    UnifiedFinding,
    'investigationOutcome' | 'investigationSessionId' | 'lastInvestigatedAt'
  >,
): boolean =>
  Boolean(
    finding.investigationOutcome || finding.investigationSessionId || finding.lastInvestigatedAt,
  );

const ATTENTION_OUTCOMES = new Set([
  'fix_verification_failed',
  'fix_verification_unknown',
  'fix_failed',
  'fix_rejected',
  'timed_out',
  'needs_attention',
  'cannot_fix',
]);

export const hasPendingInvestigationFixApproval = (
  findingId: string,
  approvals: Pick<ApprovalRequest, 'status' | 'toolId' | 'targetId' | 'expiresAt'>[],
  now = Date.now(),
): boolean =>
  approvals.some(
    (approval) =>
      isLivePendingApproval(approval, now) &&
      approval.toolId === 'investigation_fix' &&
      approval.targetId === findingId,
  );

export const isPatrolInvestigationFixApproval = (
  approval: Pick<ApprovalRequest, 'toolId'>,
): boolean => approval.toolId === 'investigation_fix';

export const doesFindingNeedAttention = (
  finding: Pick<UnifiedFinding, 'id' | 'status' | 'investigationOutcome'>,
  approvals: Pick<ApprovalRequest, 'status' | 'toolId' | 'targetId' | 'expiresAt'>[] = [],
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

export const getFindingPatrolWorkflowPresentation = (
  finding: Pick<
    UnifiedFinding,
    | 'id'
    | 'source'
    | 'status'
    | 'resourceId'
    | 'resourceName'
    | 'title'
    | 'investigationStatus'
    | 'investigationOutcome'
    | 'loopState'
  >,
  approvals: Pick<ApprovalRequest, 'status' | 'toolId' | 'targetId' | 'expiresAt'>[] = [],
  now = Date.now(),
): FindingPatrolWorkflowPresentation | undefined => {
  if (finding.source !== 'ai-patrol') {
    return undefined;
  }

  if (finding.status === 'resolved') {
    return {
      stage: 'recorded',
      label: 'Outcome recorded',
      detail: 'Patrol has a verified or cleared outcome for this finding.',
      tone: 'success',
    };
  }

  if (finding.status === 'snoozed') {
    return {
      stage: 'paused',
      label: 'Paused until reminder',
      detail: 'Patrol will surface this again when the snooze expires.',
      tone: 'info',
    };
  }

  if (finding.status !== 'active') {
    return undefined;
  }

  if (isPatrolRuntimeFinding(finding)) {
    return {
      stage: 'attention',
      label: 'Fix Patrol setup',
      detail: 'Patrol needs runtime or provider setup before it can check infrastructure reliably.',
      tone: 'info',
    };
  }

  const hasLiveApproval = hasPendingInvestigationFixApproval(finding.id, approvals, now);
  if (hasLiveApproval) {
    return {
      stage: 'approval',
      label: 'Approve or reject',
      detail: 'A governed fix is waiting for an approve or reject decision.',
      tone: 'warning',
    };
  }

  if (
    finding.investigationStatus === 'running' ||
    finding.investigationStatus === 'pending' ||
    finding.loopState === 'investigating'
  ) {
    return {
      stage: 'investigating',
      label: 'Patrol investigating',
      detail: 'Patrol is gathering context before asking for a decision.',
      tone: 'indigo',
    };
  }

  switch (finding.investigationOutcome) {
    case 'resolved':
    case 'fix_verified':
      return {
        stage: 'recorded',
        label: 'Outcome recorded',
        detail: 'Patrol verified or cleared this finding.',
        tone: 'success',
      };
    case 'fix_queued':
      return {
        stage: 'approval',
        label: 'Review fix',
        detail:
          'A fix was queued, but no live approval is available. Expand the finding to recover it.',
        tone: 'warning',
      };
    case 'fix_executed':
      return {
        stage: 'verification',
        label: 'Verify outcome',
        detail: 'Patrol executed a governed fix and is waiting for verification.',
        tone: 'info',
      };
    case 'fix_failed':
    case 'fix_verification_failed':
      return {
        stage: 'attention',
        label: 'Fix failed',
        detail:
          'The governed action did not resolve the finding. Reopen context before trying another change.',
        tone: 'danger',
      };
    case 'fix_rejected':
      return {
        stage: 'attention',
        label: 'Decide follow-up',
        detail:
          'The last governed fix was rejected. Review the finding for another option or mark it resolved after manual work.',
        tone: 'warning',
      };
    case 'fix_verification_unknown':
      return {
        stage: 'verification',
        label: 'Check outcome',
        detail: 'Patrol could not verify the fix. Inspect the resource or gather more evidence.',
        tone: 'warning',
      };
    case 'needs_attention':
    case 'cannot_fix':
    case 'timed_out':
      return {
        stage: 'attention',
        label: 'Needs input',
        detail: 'Patrol needs operator context before continuing.',
        tone: 'warning',
      };
    default:
      return undefined;
  }
};

export const getFindingLoopStateBadgeClasses = (loopState: string): string =>
  FINDING_LOOP_STATE_CLASSES[loopState] || DEFAULT_LOOP_STATE_CLASSES;

export const getFindingLoopStateBadgeTone = (loopState: string): MetadataBadgeTone =>
  FINDING_LOOP_STATE_TONES[loopState] || DEFAULT_BADGE_TONE;

export const formatFindingLoopState = (loopState: string): string =>
  formatIdentifierLabel(loopState);

export const formatFindingLifecycleType = (value: string): string =>
  FINDING_LIFECYCLE_LABELS[value] || formatIdentifierLabel(value);

export const getFindingResolutionReason = (
  finding: Pick<
    UnifiedFinding,
    'isThreshold' | 'source' | 'alertType' | 'investigationOutcome' | 'autoResolved'
  >,
  resolvedTime: string,
): string => {
  // Operator-driven manual resolution takes priority over the
  // category-specific copy: "Resolved by you" tells the operator (and any
  // teammate revisiting the timeline) that this closure was their action,
  // not Pulse's auto-detection. Patrol auto-resolution paths (fix_verified,
  // fix_executed) keep their existing copy because they describe Pulse's own
  // remediation outcome, which is more specific than "auto-resolved by Pulse".
  if (
    finding.autoResolved === false &&
    finding.investigationOutcome !== 'fix_verified' &&
    finding.investigationOutcome !== 'fix_executed' &&
    finding.investigationOutcome !== 'resolved'
  ) {
    return `Resolved by you ${resolvedTime}`;
  }
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
      case 'fix_rejected':
        return `Resolved after rejected fix ${resolvedTime}`;
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

/**
 * Format a finding as a Markdown summary the operator can paste into a chat
 * message, ticket, or incident channel. The output mirrors the seven-question
 * schema in render order — title + resource header, then description, impact,
 * recommendation, plus the trust signals (confidence, regression count) that
 * matter most to a teammate seeing the finding cold.
 *
 * Intentionally lean: only fields the operator can paste verbatim, no internal
 * IDs or timestamps in epoch form. Investigation evidence and rollback plans
 * are deferred to the Discuss with Assistant flow because those are
 * conversation context, not "share this finding" context.
 */
export const formatFindingForClipboard = (
  finding: Pick<
    UnifiedFinding,
    | 'severity'
    | 'title'
    | 'resourceName'
    | 'resourceType'
    | 'description'
    | 'impact'
    | 'recommendation'
    | 'investigationRecord'
    | 'regressionCount'
  >,
): string => {
  const lines: string[] = [];
  const severity = finding.severity ? finding.severity.toUpperCase() : 'FINDING';
  const resource = [finding.resourceName, finding.resourceType ? `(${finding.resourceType})` : '']
    .filter(Boolean)
    .join(' ')
    .trim();
  lines.push(`**[${severity}] ${finding.title || 'Untitled finding'}**`);
  if (resource) {
    lines.push(`Resource: ${resource}`);
  }
  if (finding.description) {
    lines.push('');
    lines.push(`Description: ${finding.description}`);
  }
  if (finding.impact) {
    lines.push('');
    lines.push(`Impact: ${finding.impact}`);
  }
  const trustParts: string[] = [];
  const confidence = finding.investigationRecord?.confidence;
  if (confidence) {
    trustParts.push(`Confidence: ${confidence}`);
  }
  if ((finding.regressionCount || 0) > 0) {
    trustParts.push(`Regressed: ${finding.regressionCount}×`);
  }
  if (trustParts.length > 0) {
    lines.push('');
    lines.push(trustParts.join(' · '));
  }
  return lines.join('\n');
};

/**
 * Returns the canonical `operator_state_cause` for a finding's most
 * recent dismissed lifecycle event, or "" when the finding has not been
 * auto-dismissed by operator-state suppression. Scans newest-first and
 * stops at the first `dismissed` event so a manual operator dismissal
 * that supersedes an earlier auto-dismiss is reported as manual (no
 * stale cause leaks through). Mirrors the Go-side
 * `findOperatorStateDismissCause` helper from
 * `internal/ai/findings.go`.
 *
 * Used by render code to badge auto-dismissed findings differently
 * from manual operator dismissals — both show DismissedReason=
 * 'expected_behavior' on the wire but tell different stories: Pulse
 * auto-handled vs operator decided.
 */
export const getOperatorStateDismissCause = (
  finding: Pick<UnifiedFinding, 'lifecycle'>,
): string => {
  const lifecycle = finding.lifecycle;
  if (!lifecycle || lifecycle.length === 0) return '';
  for (let i = lifecycle.length - 1; i >= 0; i -= 1) {
    const ev = lifecycle[i];
    if (ev.type !== 'dismissed') continue;
    const cause = ev.metadata?.operator_state_cause;
    if (cause) return cause;
    // First dismissed event without cause is a manual dismissal that
    // supersedes any earlier auto-dismiss — stop scanning so a stale
    // earlier cause does not falsely badge the finding as
    // auto-suppressed.
    return '';
  }
  return '';
};

/**
 * Human-readable label for an `operator_state_cause` value. Returns
 * empty string for unknown values so render code can gate cleanly.
 */
export const formatOperatorStateDismissCauseLabel = (cause: string): string => {
  switch (cause) {
    case 'maintenance_window':
      return 'maintenance';
    case 'intentionally_offline':
      return 'intentionally offline';
    default:
      return '';
  }
};
