import type { UnifiedFinding } from '@/stores/aiIntelligence';
import type { InvestigationStatus } from '@/api/patrol';

const DEFAULT_BADGE_CLASSES = 'border-border bg-surface-alt text-muted';
const DEFAULT_LOOP_STATE_CLASSES = 'border-border bg-surface-alt text-muted';

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

export const getFindingSourceLabel = (source: UnifiedFinding['source'] | string): string =>
  FINDING_SOURCE_LABELS[source] || source;

export const getFindingSourceBadgeClasses = (source: UnifiedFinding['source'] | string): string =>
  FINDING_SOURCE_CLASSES[source] || FINDING_SOURCE_CLASSES['ai-patrol'];

export const getFindingSeverityBadgeClasses = (
  severity: UnifiedFinding['severity'] | string,
): string => FINDING_SEVERITY_CLASSES[severity] || DEFAULT_BADGE_CLASSES;

export const getInvestigationStatusBadgeClasses = (status: InvestigationStatus): string =>
  INVESTIGATION_STATUS_CLASSES[status] || DEFAULT_BADGE_CLASSES;

export const getFindingLoopStateBadgeClasses = (loopState: string): string =>
  FINDING_LOOP_STATE_CLASSES[loopState] || DEFAULT_LOOP_STATE_CLASSES;

export const formatFindingLoopState = (loopState: string): string => loopState.replace(/_/g, ' ');

export const formatFindingLifecycleType = (value: string): string =>
  FINDING_LIFECYCLE_LABELS[value] || value.replace(/_/g, ' ');
