import type { RecoveryOutcome } from '@/types/recovery';

export const RECOVERY_OUTCOMES: RecoveryOutcome[] = [
  'success',
  'warning',
  'failed',
  'running',
  'unknown',
];

export function normalizeRecoveryOutcome(value: string | null | undefined): RecoveryOutcome {
  const normalized = (value || '').trim().toLowerCase();
  if (normalized === 'success' || normalized === 'ok') return 'success';
  if (normalized === 'warning' || normalized === 'warn') return 'warning';
  if (normalized === 'failed' || normalized === 'error' || normalized === 'failure')
    return 'failed';
  if (normalized === 'running') return 'running';
  if (normalized === 'unknown') {
    return 'unknown';
  }
  return 'unknown';
}

export function getRecoveryOutcomeLabel(outcome: RecoveryOutcome): string {
  switch (outcome) {
    case 'success':
      return 'Healthy';
    case 'warning':
      return 'Warning';
    case 'failed':
      return 'Failed';
    case 'running':
      return 'Running';
    default:
      return 'Unknown';
  }
}

export function getRecoveryOutcomeBadgeClass(outcome: RecoveryOutcome): string {
  const base = 'inline-flex items-center rounded-full px-2 py-1 text-xs font-medium';

  switch (outcome) {
    case 'success':
      return `${base} bg-emerald-100 text-emerald-700 dark:bg-emerald-900 dark:text-emerald-300`;
    case 'warning':
      return `${base} bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300`;
    case 'failed':
      return `${base} bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300`;
    case 'running':
      return `${base} bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300`;
    default:
      return `${base} bg-surface-alt text-muted`;
  }
}

export function getRecoveryOutcomeBarClass(outcome: RecoveryOutcome): string {
  switch (outcome) {
    case 'success':
      return 'bg-emerald-500';
    case 'warning':
      return 'bg-amber-400';
    case 'failed':
      return 'bg-red-500';
    case 'running':
      return 'bg-blue-500';
    default:
      return 'bg-gray-400';
  }
}

export function getRecoveryOutcomeTextClass(outcome: RecoveryOutcome): string {
  switch (outcome) {
    case 'success':
      return 'text-emerald-600 dark:text-emerald-400';
    case 'warning':
      return 'text-amber-600 dark:text-amber-400';
    case 'failed':
      return 'text-red-600 dark:text-red-400';
    case 'running':
      return 'text-blue-600 dark:text-blue-400';
    default:
      return 'text-muted';
  }
}
