export type RecoveryRollupStatusPill = 'stale' | 'never-succeeded' | 'recent';

export function getRecoveryProtectedToggleClass(active: boolean): string {
  return active
    ? 'border-amber-300 bg-amber-50 text-amber-800 dark:border-amber-700 dark:bg-amber-900 dark:text-amber-100'
    : 'border-border bg-surface text-base-content hover:bg-surface-hover';
}

export function getRecoveryRollupStatusPillClass(kind: RecoveryRollupStatusPill): string {
  switch (kind) {
    case 'recent':
      return 'rounded-full bg-blue-100/80 px-1.5 py-px text-[9px] font-medium text-blue-700 dark:bg-blue-900/70 dark:text-blue-200';
    case 'stale':
      return 'whitespace-nowrap rounded px-1 py-0.5 text-[10px] font-medium text-amber-700 bg-amber-50 dark:text-amber-300 dark:bg-amber-900';
    case 'never-succeeded':
      return 'whitespace-nowrap rounded px-1 py-0.5 text-[10px] font-medium text-rose-700 bg-rose-100 dark:text-rose-200 dark:bg-rose-900';
  }
}

export function getRecoveryRollupStatusPillLabel(kind: RecoveryRollupStatusPill): string {
  switch (kind) {
    case 'recent':
      return 'recent';
    case 'stale':
      return 'stale';
    case 'never-succeeded':
      return 'never succeeded';
  }
}

export function getRecoverySpecialOutcomeTextClass(kind: 'never'): string {
  if (kind === 'never') return 'text-amber-600 dark:text-amber-400';
  return 'text-muted';
}
