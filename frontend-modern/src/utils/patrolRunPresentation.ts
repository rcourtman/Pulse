import type { PatrolRunRecord, PatrolRunStatus, PatrolTriggerStatus } from '@/api/patrol';
import type { MetadataBadgeTone } from '@/components/shared/MetadataBadge';
import { formatIdentifierLabel } from '@/utils/textPresentation';
import {
  formatDurationMs,
  formatPatrolRuntimeFailureSummary,
  getCanonicalScopeResourceIds,
} from '@/utils/patrolFormat';
import {
  getPatrolProviderSettingsAction,
  type PatrolRuntimeActionPresentation,
} from '@/utils/patrolRuntimeActions';
import type { StatusIndicatorVariant } from '@/utils/status';

export interface PatrolRunStatusPresentation {
  badgeClass: string;
  variant: StatusIndicatorVariant;
  label: string;
}

export interface PatrolLatestRunPresentation {
  coverageSummary: string;
  findingsSnapshotAvailable: boolean;
  kindLabel: string;
  status: PatrolRunStatusPresentation;
  timestamp?: string;
}

export interface PatrolRunRecordSummaryPresentation {
  action?: PatrolRunPrimaryActionPresentation;
  outcome: string;
  summary: string;
}

export interface PatrolRunOperatorRecordPresentation {
  detail: string;
  headline: string;
}

export interface PatrolActivityBreakdown {
  totalRuns: number;
  fullPatrols: number;
  verificationChecks: number;
  alertTriggeredRuns: number;
  anomalyTriggeredRuns: number;
  alertClearedRuns: number;
  otherScopedRuns: number;
  newFindings: number;
}

export type PatrolRunPrimaryActionPresentation = PatrolRuntimeActionPresentation;

export const PATROL_FINDING_RECORD_UNAVAILABLE_LABEL = 'Finding record unavailable';

export interface PatrolTriggerStatusSummaryOptions {
  manualRunAvailable?: boolean;
  manualRunBlockedReason?: string;
}

function formatResourceCount(count: number, qualifier?: string): string {
  const normalized = Math.max(0, count || 0);
  const qualifierPrefix = qualifier ? `${qualifier} ` : '';
  return `${normalized} ${qualifierPrefix}resource${normalized === 1 ? '' : 's'}`;
}

function normalizePatrolRunType(type: string | undefined): string {
  return String(type || '')
    .trim()
    .toLowerCase()
    .replace(/\s+/g, '_');
}

function isFullPatrolRunType(type: string | undefined): boolean {
  switch (normalizePatrolRunType(type)) {
    case '':
    case 'full':
    case 'patrol':
      return true;
    default:
      return false;
  }
}

function normalizePatrolRunStatus(status: PatrolRunStatus | string | undefined): string {
  return String(status || '')
    .trim()
    .toLowerCase()
    .replace(/\s+/g, '_');
}

function getEffectivePatrolRunStatus(
  status: PatrolRunStatus | string | undefined,
  errorCount: number = 0,
): string {
  const normalized = normalizePatrolRunStatus(status);
  if (errorCount > 0) {
    return 'error';
  }
  return normalized;
}

export function isPatrolRunHealthy(
  status: PatrolRunStatus | string | undefined,
  errorCount: number = 0,
): boolean {
  return getEffectivePatrolRunStatus(status, errorCount) === 'healthy';
}

function punctuateSentence(value: string): string {
  const normalized = value.trim();
  if (!normalized) return '';
  return /[.!?]$/.test(normalized) ? normalized : `${normalized}.`;
}

function formatRunActionContext(
  run: Pick<
    PatrolRunRecord,
    'duration_ms' | 'effective_scope_resource_ids' | 'resources_checked' | 'scope_resource_ids'
  >,
): string {
  const coverageSummary = getPatrolRunCoverageSummary(run);
  const duration = formatDurationMs(run.duration_ms);
  if (coverageSummary && duration) {
    return `${coverageSummary} in ${duration}`;
  }
  if (coverageSummary) {
    return coverageSummary;
  }
  if (duration) {
    return `Patrol ran in ${duration}`;
  }
  return 'Patrol ran';
}

function formatIssueCount(count: number, singular: string = 'issue'): string {
  const normalized = Math.max(0, count || 0);
  return `${normalized} ${singular}${normalized === 1 ? '' : 's'}`;
}

export function getPatrolRunOperatorRecordPresentation(
  run: Pick<
    PatrolRunRecord,
    | 'auto_fix_count'
    | 'duration_ms'
    | 'effective_scope_resource_ids'
    | 'error_count'
    | 'error_detail'
    | 'error_summary'
    | 'existing_findings'
    | 'finding_ids'
    | 'new_findings'
    | 'resources_checked'
    | 'resolved_findings'
    | 'scope_resource_ids'
    | 'status'
  >,
): PatrolRunOperatorRecordPresentation {
  const context = formatRunActionContext(run);
  const runtimeFailure = formatPatrolRuntimeFailureSummary({
    errorSummary: run.error_summary,
    errorDetail: run.error_detail,
    errorCount: run.error_count,
  });

  const runHadRuntimeIssue =
    Math.max(0, run.error_count || 0) > 0 || normalizePatrolRunStatus(run.status) === 'error';

  if (runHadRuntimeIssue) {
    return {
      headline: 'Patrol needs attention',
      detail: runtimeFailure
        ? punctuateSentence(`${context}. Runtime issue: ${runtimeFailure}`)
        : `${context}. It ended with issues that need review.`,
    };
  }

  if (run.finding_ids === undefined) {
    return {
      headline: 'Completed check',
      detail: `${context}. This older run has no issue list.`,
    };
  }

  const fixedCount = Math.max(0, run.auto_fix_count || 0);
  const newFindings = Math.max(0, run.new_findings || 0);
  const resolvedFindings = Math.max(0, run.resolved_findings || 0);
  const existingFindings = Math.max(0, run.existing_findings || 0);

  if (fixedCount > 0) {
    return {
      headline: `Fixed ${formatIssueCount(fixedCount)}`,
      detail:
        newFindings > 0
          ? `${context}. Found ${formatIssueCount(newFindings, 'new issue')} and fixed ${formatIssueCount(fixedCount)}.`
          : `${context}. Fixed ${formatIssueCount(fixedCount)}.`,
    };
  }

  if (resolvedFindings > 0) {
    return {
      headline: `Confirmed ${formatIssueCount(resolvedFindings)} resolved`,
      detail: `${context}. Confirmed ${formatIssueCount(resolvedFindings)} resolved.`,
    };
  }

  if (newFindings > 0) {
    return {
      headline: `Found ${formatIssueCount(newFindings, 'new issue')}`,
      detail: `${context}. Ready for review.`,
    };
  }

  if (existingFindings > 0) {
    return {
      headline: `${formatIssueCount(existingFindings)} still open`,
      detail: `${context}. No new issues.`,
    };
  }

  if (!isPatrolRunHealthy(run.status, run.error_count)) {
    return {
      headline: 'Patrol needs attention',
      detail: `${context}. It ended with issues that need review.`,
    };
  }

  return {
    headline: 'All clear',
    detail: `${context}. No issues recorded.`,
  };
}

export function getPatrolRunPrimaryActionPresentation(
  run: Pick<PatrolRunRecord, 'error_count' | 'error_summary' | 'error_detail'>,
): PatrolRunPrimaryActionPresentation | undefined {
  const hasRuntimeFailureDetail = Boolean(
    String(run.error_summary || '').trim() || String(run.error_detail || '').trim(),
  );
  if (Math.max(0, run.error_count || 0) <= 0) {
    return undefined;
  }
  if (!hasRuntimeFailureDetail) {
    return undefined;
  }

  return getPatrolProviderSettingsAction();
}

export function getPatrolRunRecordSummaryPresentation(
  run: Pick<
    PatrolRunRecord,
    | 'auto_fix_count'
    | 'duration_ms'
    | 'effective_scope_resource_ids'
    | 'error_count'
    | 'error_detail'
    | 'error_summary'
    | 'existing_findings'
    | 'finding_ids'
    | 'new_findings'
    | 'resources_checked'
    | 'scope_resource_ids'
    | 'status'
  >,
): PatrolRunRecordSummaryPresentation {
  const coverageSummary = getPatrolRunCoverageSummary(run);
  const duration = formatDurationMs(run.duration_ms);
  const summarySubject = coverageSummary || 'Patrol ran';
  const summary = duration ? `${summarySubject} in ${duration}.` : `${summarySubject}.`;
  const runtimeFailure = formatPatrolRuntimeFailureSummary({
    errorSummary: run.error_summary,
    errorDetail: run.error_detail,
    errorCount: run.error_count,
  });
  const action = getPatrolRunPrimaryActionPresentation(run);

  if (!isPatrolRunHealthy(run.status, run.error_count)) {
    return {
      action,
      summary,
      outcome: runtimeFailure
        ? punctuateSentence(`Patrol ended with a runtime issue: ${runtimeFailure}`)
        : 'Patrol ended with issues requiring review.',
    };
  }

  if (run.finding_ids === undefined) {
    return {
      summary,
      outcome: 'This older run has no finding record, so Patrol cannot show its issue list.',
    };
  }

  const newFindings = Math.max(0, run.new_findings || 0);
  const existingFindings = Math.max(0, run.existing_findings || 0);
  const fixedCount = Math.max(0, run.auto_fix_count || 0);

  if (newFindings > 0) {
    const fixedPart =
      fixedCount > 0 ? ` Patrol fixed ${fixedCount} issue${fixedCount === 1 ? '' : 's'}.` : '';
    return {
      summary,
      outcome: `Patrol found ${newFindings} new issue${newFindings === 1 ? '' : 's'}.${fixedPart}`,
    };
  }

  if (existingFindings > 0) {
    return {
      summary,
      outcome: `No new issues. ${existingFindings} existing issue${existingFindings === 1 ? ' remains' : 's remain'}.`,
    };
  }

  return {
    summary,
    outcome: 'No issues recorded for this run.',
  };
}

export function getPatrolRunStatusPresentation(
  status: PatrolRunStatus | string,
  errorCount: number = 0,
  findingsSnapshotAvailable: boolean = true,
): PatrolRunStatusPresentation {
  const normalized = getEffectivePatrolRunStatus(status, errorCount);

  switch (normalized) {
    case 'critical':
    case 'error':
      return {
        badgeClass: 'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300',
        variant: 'danger',
        label: formatIdentifierLabel(normalized),
      };
    case 'issues_found':
      return {
        badgeClass: 'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300',
        variant: 'warning',
        label: 'issues found',
      };
    case 'healthy':
      if (!findingsSnapshotAvailable) {
        return {
          badgeClass: 'bg-surface-alt text-base-content',
          variant: 'muted',
          label: 'completed',
        };
      }
      return {
        badgeClass: 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300',
        variant: 'success',
        label: 'healthy',
      };
    default:
      return {
        badgeClass: 'bg-surface-alt text-base-content',
        variant: 'muted',
        label: formatIdentifierLabel(normalized, { fallback: 'unknown' }),
      };
  }
}

export function getToolCallResultBadgeTone(success: boolean): MetadataBadgeTone {
  return success ? 'success' : 'danger';
}

export function getToolCallResultTextClass(success: boolean): string {
  return success ? 'text-emerald-600 dark:text-emerald-400' : 'text-red-600 dark:text-red-400';
}

export function getPatrolRunKindLabel(type: string | undefined): string {
  switch (normalizePatrolRunType(type)) {
    case 'scoped':
      return 'Targeted check';
    case 'verification':
      return 'Follow-up check';
    default:
      return isFullPatrolRunType(type) ? 'Patrol check' : 'Patrol run';
  }
}

export function getPatrolLatestRunPresentation(
  runs: PatrolRunRecord[],
): PatrolLatestRunPresentation | undefined {
  const latestRun = runs.find((run) => Boolean(run.completed_at?.trim() || run.started_at?.trim()));
  if (!latestRun) {
    return undefined;
  }

  const findingsSnapshotAvailable = latestRun.finding_ids !== undefined;

  return {
    coverageSummary: getPatrolRunCoverageSummary(latestRun),
    findingsSnapshotAvailable,
    kindLabel: getPatrolRunKindLabel(latestRun.type),
    status: getPatrolRunStatusPresentation(
      latestRun.status ?? 'unknown',
      latestRun.error_count ?? 0,
      findingsSnapshotAvailable,
    ),
    timestamp: latestRun.completed_at || latestRun.started_at,
  };
}

function isAlreadyRunningBlockedReason(reason?: string): boolean {
  return /\balready running\b|\brun\b.*\bin progress\b/i.test(reason?.trim() || '');
}

function getManualRunBlockedTriggerSummary(reason?: string): string | undefined {
  if (isAlreadyRunningBlockedReason(reason)) {
    return 'A Patrol run is already in progress. New automatic and manual runs are paused until it finishes.';
  }
  return undefined;
}

export function getPatrolTriggerStatusSummary(
  status: PatrolTriggerStatus | undefined,
  options: PatrolTriggerStatusSummaryOptions = {},
): string | undefined {
  if (!status) {
    return undefined;
  }

  if (status.event_triggers_blocked) {
    if (options.manualRunAvailable === false) {
      return getManualRunBlockedTriggerSummary(options.manualRunBlockedReason);
    }
    return undefined;
  }

  const notes: string[] = [];
  if (status.pending_triggers > 0) notes.push(`${status.pending_triggers} queued`);
  if (status.is_busy_mode) notes.push('busy mode');
  if (!status.alert_triggers_enabled) notes.push('alerts off');
  if (!status.anomaly_triggers_enabled) notes.push('anomalies off');

  return notes.length > 0 ? notes.join(' · ') : undefined;
}

function isSameLocalDay(timestamp: string | undefined, referenceDate: Date): boolean {
  if (!timestamp) return false;
  const value = new Date(timestamp);
  if (Number.isNaN(value.getTime())) return false;
  return (
    value.getFullYear() === referenceDate.getFullYear() &&
    value.getMonth() === referenceDate.getMonth() &&
    value.getDate() === referenceDate.getDate()
  );
}

export function getPatrolActivityBreakdown(
  runs: PatrolRunRecord[],
  referenceDate: Date = new Date(),
): PatrolActivityBreakdown {
  return runs.reduce<PatrolActivityBreakdown>(
    (summary, run) => {
      if (!isSameLocalDay(run.started_at, referenceDate)) {
        return summary;
      }

      summary.totalRuns += 1;
      summary.newFindings += Math.max(0, run.new_findings || 0);

      if (isFullPatrolRunType(run.type)) {
        summary.fullPatrols += 1;
        return summary;
      }

      switch (normalizePatrolRunType(run.type)) {
        case 'verification':
          summary.verificationChecks += 1;
          return summary;
      }

      switch (normalizePatrolRunType(run.trigger_reason)) {
        case 'alert_fired':
          summary.alertTriggeredRuns += 1;
          break;
        case 'anomaly':
          summary.anomalyTriggeredRuns += 1;
          break;
        case 'alert_cleared':
          summary.alertClearedRuns += 1;
          break;
        default:
          summary.otherScopedRuns += 1;
          break;
      }

      return summary;
    },
    {
      totalRuns: 0,
      fullPatrols: 0,
      verificationChecks: 0,
      alertTriggeredRuns: 0,
      anomalyTriggeredRuns: 0,
      alertClearedRuns: 0,
      otherScopedRuns: 0,
      newFindings: 0,
    },
  );
}

function formatCountLabel(count: number, singular: string, plural?: string): string {
  return `${count} ${count === 1 ? singular : (plural ?? `${singular}s`)}`;
}

export function formatPatrolActivityBreakdown(summary: PatrolActivityBreakdown): string {
  const segments: string[] = [];
  if (summary.fullPatrols > 0) {
    segments.push(formatCountLabel(summary.fullPatrols, 'full check', 'full checks'));
  }
  if (summary.alertTriggeredRuns > 0) {
    segments.push(
      formatCountLabel(
        summary.alertTriggeredRuns,
        'alert-triggered check',
        'alert-triggered checks',
      ),
    );
  }
  if (summary.anomalyTriggeredRuns > 0) {
    segments.push(
      formatCountLabel(
        summary.anomalyTriggeredRuns,
        'anomaly-triggered check',
        'anomaly-triggered checks',
      ),
    );
  }
  if (summary.alertClearedRuns > 0) {
    segments.push(
      formatCountLabel(summary.alertClearedRuns, 'alert-cleared check', 'alert-cleared checks'),
    );
  }
  if (summary.verificationChecks > 0) {
    segments.push(
      formatCountLabel(summary.verificationChecks, 'follow-up check', 'follow-up checks'),
    );
  }
  if (summary.otherScopedRuns > 0) {
    segments.push(formatCountLabel(summary.otherScopedRuns, 'targeted check', 'targeted checks'));
  }
  return segments.join(', ');
}

export function getPatrolRunCoverageSummary(
  run: Pick<
    PatrolRunRecord,
    'resources_checked' | 'scope_resource_ids' | 'effective_scope_resource_ids'
  >,
): string {
  const resourcesChecked = Math.max(0, run.resources_checked || 0);
  const scopedResourceCount = getCanonicalScopeResourceIds(run)?.length ?? 0;

  if (scopedResourceCount > 0) {
    if (resourcesChecked < scopedResourceCount) {
      return `Checked ${resourcesChecked} of ${scopedResourceCount} scoped resources`;
    }
    if (resourcesChecked > 0) {
      return `Checked ${formatResourceCount(resourcesChecked, 'scoped')}`;
    }
  }

  if (resourcesChecked > 0) {
    return `Checked ${formatResourceCount(resourcesChecked)}`;
  }

  return '';
}

export function getPatrolRunResourcesHeading(
  run: Pick<
    PatrolRunRecord,
    'resources_checked' | 'scope_resource_ids' | 'effective_scope_resource_ids'
  >,
): string {
  const resourcesChecked = Math.max(0, run.resources_checked || 0);
  const scopedResourceCount = getCanonicalScopeResourceIds(run)?.length ?? 0;

  if (scopedResourceCount > 0 && resourcesChecked > 0 && resourcesChecked < scopedResourceCount) {
    return `Resources checked (${resourcesChecked} of ${scopedResourceCount} scoped)`;
  }

  return `Resources checked (${resourcesChecked})`;
}

export function getRunHistoryLoadingState(): string {
  return 'Loading history...';
}

export function getRunHistorySelectionHint(
  runs: Array<Pick<PatrolRunRecord, 'finding_ids'>>,
  selectedRun?: Pick<PatrolRunRecord, 'finding_ids'> | null,
): string {
  if (selectedRun && selectedRun.finding_ids === undefined) {
    return 'This older check has no issue list.';
  }

  if (runs.some((run) => run.finding_ids === undefined)) {
    return 'Open a check to review what Patrol found. Older checks may not have issue lists.';
  }

  return 'Open a check to review what Patrol found.';
}

export function getToolCallsLoadingState(): string {
  return 'Loading tool calls...';
}

export function getToolCallsUnavailableState(): string {
  return 'Tool call details not available for this run.';
}
