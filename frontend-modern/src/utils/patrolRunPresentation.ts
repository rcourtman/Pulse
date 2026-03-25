import type { PatrolRunRecord, PatrolRunStatus } from '@/api/patrol';
import { formatIdentifierLabel } from '@/utils/textPresentation';
import { getCanonicalScopeResourceIds } from '@/utils/patrolFormat';

export interface PatrolRunStatusPresentation {
  badgeClass: string;
  label: string;
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
  return normalizePatrolRunType(type) === 'scoped' ? 'Scoped run' : 'Full patrol';
}

export function getPatrolRunCoverageSummary(run: Pick<PatrolRunRecord, 'resources_checked' | 'scope_resource_ids' | 'effective_scope_resource_ids'>): string {
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

export function getPatrolRunResourcesHeading(run: Pick<PatrolRunRecord, 'resources_checked' | 'scope_resource_ids' | 'effective_scope_resource_ids'>): string {
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

export function getToolCallsLoadingState(): string {
  return 'Loading tool calls...';
}

export function getToolCallsUnavailableState(): string {
  return 'Tool call details not available for this run.';
}
