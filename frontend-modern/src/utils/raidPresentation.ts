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

export function getRaidDeviceStateVariant(device: HostRAIDDevice): StatusIndicatorVariant {
  const normalized = normalize(device.state);
  if (
    normalized.includes('fail') ||
    normalized.includes('fault') ||
    normalized.includes('offline') ||
    normalized.includes('removed')
  ) {
    return 'danger';
  }
  // mdadm --detail names a healthy member "active sync", with optional flags
  // such as writemostly after it.
  if (
    normalized === 'active' ||
    normalized === 'in_sync' ||
    normalized === 'online' ||
    normalized.startsWith('active sync')
  ) {
    return 'success';
  }
  return 'warning';
}

export function getRaidDeviceBadgeClass(device: HostRAIDDevice): string {
  const variant = getRaidDeviceStateVariant(device);
  if (variant === 'success') {
    return 'bg-emerald-50 text-emerald-700 border-emerald-200 dark:bg-emerald-900/25 dark:text-emerald-200 dark:border-emerald-800';
  }
  if (variant === 'danger') {
    return 'bg-red-50 text-red-700 border-red-200 dark:bg-red-900/25 dark:text-red-200 dark:border-red-800';
  }
  return 'bg-amber-50 text-amber-700 border-amber-200 dark:bg-amber-900/25 dark:text-amber-200 dark:border-amber-800';
}

// A retained array, such as a silent agent's last report, records what the
// array was, not what it is, so its members take no live status colour.
export const RAID_LAST_KNOWN_DEVICE_BADGE_CLASS = 'border-border bg-surface-alt text-muted';

/**
 * A retained member badge has no colour to carry its state, so a member that
 * was not healthy names its state in the badge.
 */
export function getRaidLastKnownDeviceLabel(device: HostRAIDDevice): string {
  const state = (device.state || '').trim();
  if (!state || getRaidDeviceStateVariant(device) === 'success') return device.device;
  return `${device.device} · ${state}`;
}
