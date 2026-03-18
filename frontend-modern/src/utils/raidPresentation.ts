import type { HostRAIDDevice } from '@/types/api';
import type { StatusIndicatorVariant } from '@/utils/status';

const normalize = (value?: string | null): string => (value || '').trim().toLowerCase();

export function getRaidStateVariant(state?: string | null): StatusIndicatorVariant {
  const normalized = normalize(state);
  if (normalized === 'active' || normalized === 'clean') return 'success';
  if (
    normalized.includes('fail') ||
    normalized.includes('inactive') ||
    normalized.includes('offline') ||
    normalized.includes('stopped')
  ) {
    return 'danger';
  }
  return 'warning';
}

export function getRaidStateTextClass(state?: string | null): string {
  const variant = getRaidStateVariant(state);
  if (variant === 'success') return 'text-emerald-600 dark:text-emerald-400';
  if (variant === 'danger') return 'text-red-600 dark:text-red-400';
  return 'text-amber-600 dark:text-amber-400';
}

export function getRaidDeviceBadgeClass(device: HostRAIDDevice): string {
  const normalized = normalize(device.state);
  if (normalized === 'active' || normalized === 'in_sync' || normalized === 'online') {
    return 'bg-emerald-50 text-emerald-700 border-emerald-200 dark:bg-emerald-900 dark:text-emerald-200 dark:border-emerald-800';
  }
  if (
    normalized.includes('fail') ||
    normalized.includes('fault') ||
    normalized.includes('offline') ||
    normalized.includes('removed')
  ) {
    return 'bg-red-50 text-red-700 border-red-200 dark:bg-red-900 dark:text-red-200 dark:border-red-800';
  }
  return 'bg-amber-50 text-amber-700 border-amber-200 dark:bg-amber-900 dark:text-amber-200 dark:border-amber-800';
}
