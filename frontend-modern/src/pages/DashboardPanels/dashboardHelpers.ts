export type StatusTone = 'online' | 'offline' | 'warning' | 'critical' | 'unknown';

export function statusBadgeClass(tone: StatusTone): string {
  const base = 'inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs font-medium';
  switch (tone) {
    case 'online':
      return `${base} border-emerald-200 bg-emerald-100 text-emerald-700 dark:border-emerald-800 dark:bg-emerald-900 dark:text-emerald-300`;
    case 'warning':
      return `${base} border-amber-200 bg-amber-100 text-amber-700 dark:border-amber-800 dark:bg-amber-900 dark:text-amber-300`;
    case 'critical':
      return `${base} border-red-200 bg-red-100 text-red-700 dark:border-red-800 dark:bg-red-900 dark:text-red-300`;
    case 'offline': return `${base} border-border bg-surface-alt text-base-content`; default: return `${base} border-border bg-surface-alt text-muted`; }
} export function formatPercent(value: number): string { return `${Math.round(value)}%`;
} export function formatDelta(delta: number | null): string | null { if (delta === null) return null; const sign = delta >= 0 ?'+' : '';
  return `${sign}${delta.toFixed(1)}%`;
}

export function deltaColorClass(delta: number | null): string {
  if (delta === null) return 'text-muted';
  if (delta > 5) return 'text-red-500 dark:text-red-400';
  if (delta > 0) return 'text-amber-500 dark:text-amber-400';
  if (delta < -5) return 'text-emerald-500 dark:text-emerald-400';
  if (delta < 0) return 'text-blue-500 dark:text-blue-400';
  return 'text-muted';
}

export type ActionPriority = 'critical' | 'high' | 'medium' | 'low';

export interface ActionItem {
  id: string;
  priority: ActionPriority;
  label: string;
  link: string;
}

export const PRIORITY_ORDER: Record<ActionPriority, number> = {
  critical: 0,
  high: 1,
  medium: 2,
  low: 3,
};

export const MAX_ACTION_ITEMS = 5;

export function priorityBadgeClass(priority: ActionPriority): string {
  switch (priority) {
    case 'critical':
      return 'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300';
    case 'high':
      return 'bg-orange-100 text-orange-700 dark:bg-orange-900 dark:text-orange-300';
    case 'medium':
      return 'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300';
    case 'low':
      return 'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300';
  }
}

