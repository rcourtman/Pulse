import type { Alert } from '@/types/api';

const ALERT_SEVERITY_COMPACT_LABELS: Record<string, string> = {
  critical: 'CRIT',
  warning: 'WARN',
  info: 'INFO',
};

export function getAlertSeverityBadgeClass(level: Alert['level'] | string): string {
  const base =
    'inline-flex shrink-0 items-center rounded px-1.5 py-0.5 text-[10px] font-semibold uppercase';

  switch (level) {
    case 'critical':
      return `${base} bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300`;
    case 'warning':
      return `${base} bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300`;
    default:
      return `${base} bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300`;
  }
}

export function getAlertSeverityTextClass(level: Alert['level'] | string): string {
  switch (level) {
    case 'critical':
      return 'text-red-600 dark:text-red-400';
    case 'warning':
      return 'text-amber-600 dark:text-amber-400';
    default:
      return 'text-blue-600 dark:text-blue-400';
  }
}

export function getAlertSeverityCompactLabel(level: Alert['level'] | string): string {
  return ALERT_SEVERITY_COMPACT_LABELS[level] || String(level).toUpperCase();
}

export function getAlertSeverityDotClass(level: Alert['level'] | string): string {
  switch (level) {
    case 'critical':
      return 'h-2 w-2 rounded-full bg-red-500';
    case 'warning':
      return 'h-2 w-2 rounded-full bg-yellow-500';
    default:
      return 'h-2 w-2 rounded-full bg-blue-500';
  }
}
