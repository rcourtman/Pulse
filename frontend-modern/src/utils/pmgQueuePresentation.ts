export type PMGQueueSeverity = 'high' | 'medium' | 'low';

export function getPMGQueueSeverity(total: number): PMGQueueSeverity {
  if (total > 100) return 'high';
  if (total > 20) return 'medium';
  return 'low';
}

export function getPMGQueueTextClass(total: number, lowClass = 'text-muted'): string {
  const severity = getPMGQueueSeverity(total);
  if (severity === 'high') return 'text-red-600 dark:text-red-400';
  if (severity === 'medium') return 'text-yellow-600 dark:text-yellow-400';
  return lowClass;
}

export function getPMGOldestAgeTextClass(seconds?: number | null): string {
  return (seconds || 0) > 1800 ? 'text-yellow-400' : '';
}
