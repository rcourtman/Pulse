// Type-safe formatting utilities
import type { Disk } from '@/types/api';

/**
 * Format bytes to human-readable string with dynamic precision.
 * @param bytes - Number of bytes to format
 * @param decimals - Number of decimal places, or 'auto' for dynamic precision:
 *   - Values < 10: 2 decimals (e.g., "5.94 GB")
 *   - Values 10-100: 1 decimal (e.g., "45.2 GB")
 *   - Values >= 100: 0 decimals (e.g., "256 GB")
 */
export function formatBytes(bytes: number, decimals: number | 'auto' = 'auto'): string {
  if (!bytes || bytes < 0) return '0 B';

  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  const value = bytes / Math.pow(k, i);

  // Determine precision
  let precision: number;
  if (decimals === 'auto') {
    if (value < 10) precision = 2;
    else if (value < 100) precision = 1;
    else precision = 0;
  } else {
    precision = decimals;
  }

  return `${value.toFixed(precision)} ${sizes[i]}`;
}

export function formatSpeed(bytesPerSecond: number, decimals: number | 'auto' = 'auto'): string {
  if (!bytesPerSecond || bytesPerSecond < 0) return '0 B/s';
  return `${formatBytes(bytesPerSecond, decimals)}/s`;
}

export function formatPercent(value: number): string {
  if (!Number.isFinite(value)) return '0%';
  const abs = Math.abs(value);
  if (abs === 0) return '0%';
  if (abs < 0.5) {
    return '0%';
  }
  return `${Math.round(value)}%`;
}

export function formatNumber(value: number): string {
  if (!Number.isFinite(value)) return '0';
  return value.toLocaleString();
}

export function formatUptime(seconds: number, condensed = false): string {
  if (!seconds || seconds < 0) return '0s';

  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);

  if (days > 0) {
    return condensed ? `${days}d` : `${days}d ${hours}h`;
  } else if (hours > 0) {
    return condensed ? `${hours}h` : `${hours}h ${minutes}m`;
  } else {
    return `${minutes}m`;
  }
}

export function formatAbsoluteTime(timestamp: number | undefined): string {
  if (!timestamp) return '';
  const date = new Date(timestamp);

  const months = [
    'Jan',
    'Feb',
    'Mar',
    'Apr',
    'May',
    'Jun',
    'Jul',
    'Aug',
    'Sep',
    'Oct',
    'Nov',
    'Dec',
  ];

  const month = months[date.getMonth()];
  const day = date.getDate();
  const hours = date.getHours().toString().padStart(2, '0');
  const minutes = date.getMinutes().toString().padStart(2, '0');

  return `${day} ${month} ${hours}:${minutes}`;
}

/**
 * Format a timestamp as a human-readable relative time string.
 * ALL relative time formatting MUST use this function.
 *
 * @param timestamp - Unix ms number, ISO date string, Date object, or undefined
 * @param options.compact - Use short format: "5m ago" instead of "5 mins ago"
 * @param options.emptyText - Text for falsy input (default: '')
 */
export function formatRelativeTime(
  timestamp: number | string | Date | undefined,
  options?: { compact?: boolean; emptyText?: string },
): string {
  if (!timestamp) return options?.emptyText ?? '';

  let ms: number;
  if (typeof timestamp === 'number') {
    ms = timestamp;
  } else if (typeof timestamp === 'string') {
    ms = new Date(timestamp).getTime();
  } else {
    ms = timestamp.getTime();
  }

  const diffMs = Date.now() - ms;
  return formatTimeDiff(diffMs, options?.compact);
}

/**
 * Formats a time difference in milliseconds to a human-readable string.
 * Internal helper used by formatRelativeTime and formatBackupAge.
 */
function formatTimeDiff(diffMs: number, compact?: boolean): string {
  // Handle invalid or future timestamps
  if (isNaN(diffMs) || !isFinite(diffMs)) return '';
  if (diffMs < 0) return compact ? 'just now' : '0s ago';

  const diffSeconds = Math.floor(diffMs / 1000);
  const diffMinutes = Math.floor(diffSeconds / 60);
  const diffHours = Math.floor(diffMinutes / 60);
  const diffDays = Math.floor(diffHours / 24);
  const diffMonths = Math.floor(diffDays / 30);
  const diffYears = Math.floor(diffDays / 365);

  if (compact) {
    if (diffSeconds < 60) return 'just now';
    if (diffMinutes < 60) return `${diffMinutes}m ago`;
    if (diffHours < 24) return `${diffHours}h ago`;
    return `${diffDays}d ago`;
  }

  if (diffSeconds < 60) {
    return `${diffSeconds}s ago`;
  } else if (diffMinutes < 60) {
    return diffMinutes === 1 ? '1 min ago' : `${diffMinutes} mins ago`;
  } else if (diffHours < 24) {
    return diffHours === 1 ? '1 hour ago' : `${diffHours} hours ago`;
  } else if (diffDays < 30) {
    return diffDays === 1 ? '1 day ago' : `${diffDays} days ago`;
  } else if (diffMonths < 12) {
    return diffMonths === 1 ? '1 month ago' : `${diffMonths} months ago`;
  } else {
    return diffYears === 1 ? '1 year ago' : `${diffYears} years ago`;
  }
}

export type BackupStatus = 'fresh' | 'stale' | 'critical' | 'never';

export interface BackupInfo {
  status: BackupStatus;
  ageMs: number | null;
  ageFormatted: string;
}

// Default thresholds (used when no config is provided)
const DEFAULT_FRESH_HOURS = 24;
const DEFAULT_STALE_HOURS = 72;

export interface BackupThresholds {
  freshHours?: number;
  staleHours?: number;
}

/**
 * Analyzes backup freshness for a guest.
 * @param lastBackup - ISO timestamp string or Unix timestamp (ms)
 * @param thresholds - Optional thresholds for fresh/stale determination (in hours)
 * @returns BackupInfo with status and formatted age
 */
export function getBackupInfo(
  lastBackup: string | number | null | undefined,
  thresholds?: BackupThresholds
): BackupInfo {
  if (!lastBackup) {
    return { status: 'never', ageMs: null, ageFormatted: 'Never' };
  }

  let timestamp: number;
  if (typeof lastBackup === 'string') {
    timestamp = new Date(lastBackup).getTime();
  } else {
    timestamp = lastBackup;
  }

  if (isNaN(timestamp) || timestamp <= 0) {
    return { status: 'never', ageMs: null, ageFormatted: 'Never' };
  }

  const now = Date.now();
  const ageMs = now - timestamp;

  // Use provided thresholds or fall back to defaults
  const freshHours = thresholds?.freshHours ?? DEFAULT_FRESH_HOURS;
  const staleHours = thresholds?.staleHours ?? DEFAULT_STALE_HOURS;
  const freshThresholdMs = freshHours * 60 * 60 * 1000;
  const staleThresholdMs = staleHours * 60 * 60 * 1000;

  let status: BackupStatus;
  if (ageMs <= freshThresholdMs) {
    status = 'fresh';
  } else if (ageMs <= staleThresholdMs) {
    status = 'stale';
  } else {
    status = 'critical';
  }

  return {
    status,
    ageMs,
    ageFormatted: formatTimeDiff(ageMs),
  };
}

/**
 * Format disk power-on hours into a human-readable duration.
 * ALL power-on-hours formatting MUST use this function.
 */
export function formatPowerOnHours(hours: number, condensed = false): string {
  if (hours >= 8760) {
    const years = (hours / 8760).toFixed(1);
    return condensed ? `${years}y` : `${years} years`;
  }
  if (hours >= 24) {
    const days = Math.round(hours / 24);
    return condensed ? `${days}d` : `${days} days`;
  }
  return condensed ? `${hours}h` : `${hours} hours`;
}

/**
 * Estimate rendered text width based on character count.
 * Used for determining if labels fit inside metric bars.
 * ALL text-width estimation MUST use this function.
 */
export function estimateTextWidth(text: string): number {
  return text.length * 5.5 + 8;
}

/**
 * Format anomaly ratio for display in metric bars.
 * Returns null if no meaningful anomaly, or a short indicator string.
 * ALL anomaly ratio formatting MUST use this function.
 */
/** CSS class for anomaly severity badge text color. */
export const ANOMALY_SEVERITY_CLASS: Record<string, string> = {
  critical: 'text-red-400',
  high: 'text-orange-400',
  medium: 'text-yellow-400',
  low: 'text-blue-400',
};

export function formatAnomalyRatio(anomaly: { baseline_mean: number; current_value: number } | null | undefined): string | null {
  if (!anomaly || anomaly.baseline_mean === 0) return null;
  const ratio = anomaly.current_value / anomaly.baseline_mean;
  if (ratio >= 2) return `${ratio.toFixed(1)}x`;
  if (ratio >= 1.5) return '↑↑';
  return '↑';
}

/**
 * Shorten image registry URLs to show only the last two name components (repo/name).
 * e.g., "ghcr.io/rcourtman/pulse:latest" -> "rcourtman/pulse:latest"
 */
export function getShortImageName(fullImage: string | undefined): string {
  if (!fullImage) return '—';
  // Handle case with @sha256: digests
  const cleanImage = fullImage.split('@')[0];
  const parts = cleanImage.split('/');
  if (parts.length >= 2) {
    return parts.slice(-2).join('/');
  }
  return cleanImage;
}

/**
 * Normalize raw disk objects (from API/agent) into proper Disk[].
 * Calculates `usage` from used/total and defaults missing fields.
 */
export function normalizeDiskArray(
  disks?: Array<{ device?: string; mountpoint?: string; filesystem?: string; type?: string; total?: number; used?: number; free?: number }>,
): Disk[] | undefined {
  if (!disks || disks.length === 0) return undefined;
  return disks.map((d) => {
    const total = d.total ?? 0;
    const used = d.used ?? 0;
    const free = d.free ?? (total > 0 ? Math.max(0, total - used) : 0);
    const usage = total > 0 ? (used / total) * 100 : 0;
    return {
      total,
      used,
      free,
      usage,
      mountpoint: d.mountpoint,
      type: d.filesystem ?? d.type,
      device: d.device,
    };
  });
}
