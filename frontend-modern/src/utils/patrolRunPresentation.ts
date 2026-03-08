import type { PatrolRunStatus } from '@/api/patrol';

export interface PatrolRunStatusPresentation {
  badgeClass: string;
  label: string;
}

export function getPatrolRunStatusPresentation(
  status: PatrolRunStatus | string,
): PatrolRunStatusPresentation {
  const normalized = String(status || '')
    .trim()
    .toLowerCase()
    .replace(/\s+/g, '_');

  switch (normalized) {
    case 'critical':
    case 'error':
      return {
        badgeClass: 'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300',
        label: normalized.replace(/_/g, ' '),
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
        label: normalized ? normalized.replace(/_/g, ' ') : 'unknown',
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
