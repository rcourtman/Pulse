import type { PatrolRunRecord, PatrolRunStatus } from '@/api/patrol';
import { formatIdentifierLabel } from '@/utils/textPresentation';
import { getCanonicalScopeResourceIds } from '@/utils/patrolFormat';

export interface PatrolRunStatusPresentation {
  badgeClass: string;
  label: string;
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
        label: formatIdentifierLabel(normalized),
      };
    case 'issues_found':
      return {
        badgeClass: 'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300',
        label: 'issues found',
      };
    case 'healthy':
      if (!findingsSnapshotAvailable) {
        return {
          badgeClass: 'bg-surface-alt text-base-content',
          label: 'completed',
        };
      }
      return {
        badgeClass: 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300',
        label: 'healthy',
      };
    default:
      return {
        badgeClass: 'bg-surface-alt text-base-content',
        label: formatIdentifierLabel(normalized, { fallback: 'unknown' }),
      };
  }
}

export function getToolCallResultBadgeClass(success: boolean): string {
  return success
    ? 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300'
    : 'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300';
}

export function getToolCallResultTextClass(success: boolean): string {
  return success ? 'text-emerald-600 dark:text-emerald-400' : 'text-red-600 dark:text-red-400';
}

export function getPatrolRunKindLabel(type: string | undefined): string {
  switch (normalizePatrolRunType(type)) {
    case 'scoped':
      return 'Scoped run';
    case 'verification':
      return 'Verification check';
    default:
      return isFullPatrolRunType(type) ? 'Full patrol' : 'Patrol run';
  }
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

function formatCountLabel(count: number, label: string): string {
  return `${count} ${label}`;
}

export function formatPatrolActivityBreakdown(summary: PatrolActivityBreakdown): string {
  const segments: string[] = [];
  if (summary.fullPatrols > 0) segments.push(formatCountLabel(summary.fullPatrols, 'full'));
  if (summary.alertTriggeredRuns > 0) {
    segments.push(formatCountLabel(summary.alertTriggeredRuns, 'alert-triggered'));
  }
  if (summary.anomalyTriggeredRuns > 0) {
    segments.push(formatCountLabel(summary.anomalyTriggeredRuns, 'anomaly-triggered'));
  }
  if (summary.alertClearedRuns > 0) {
    segments.push(formatCountLabel(summary.alertClearedRuns, 'alert-cleared'));
  }
  if (summary.verificationChecks > 0) {
    segments.push(formatCountLabel(summary.verificationChecks, 'verification'));
  }
  if (summary.otherScopedRuns > 0) {
    segments.push(formatCountLabel(summary.otherScopedRuns, 'other scoped'));
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
  return 'Loading run history…';
}

export function getRunHistorySelectionHint(
  runs: Array<Pick<PatrolRunRecord, 'finding_ids'>>,
  selectedRun?: Pick<PatrolRunRecord, 'finding_ids'> | null,
): string {
  if (selectedRun && selectedRun.finding_ids === undefined) {
    return 'Selected run predates findings snapshots; run-scoped findings cannot be fully verified.';
  }

  if (runs.some((run) => run.finding_ids === undefined)) {
    return 'Select a run to filter findings when available. Some older runs do not include findings snapshots.';
  }

  return 'Select a run to filter findings to that snapshot';
}

export function getToolCallsLoadingState(): string {
  return 'Loading tool calls...';
}

export function getToolCallsUnavailableState(): string {
  return 'Tool call details not available for this run.';
}
