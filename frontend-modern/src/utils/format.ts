// Type-safe formatting utilities

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

export function formatRelativeTime(timestamp: number | undefined): string {
  if (!timestamp) return '';

  const now = Date.now();
  const diffMs = now - timestamp;

  return formatTimeDiff(diffMs);
}

/**
 * Formats a time difference in milliseconds to a human-readable string.
 * Internal helper used by formatRelativeTime and formatBackupAge.
 */
function formatTimeDiff(diffMs: number): string {
  // Handle invalid or future timestamps
  if (isNaN(diffMs) || !isFinite(diffMs)) return '';
  if (diffMs < 0) return '0s ago'; // Future timestamp, treat as current

  const diffSeconds = Math.floor(diffMs / 1000);
  const diffMinutes = Math.floor(diffSeconds / 60);
  const diffHours = Math.floor(diffMinutes / 60);
  const diffDays = Math.floor(diffHours / 24);
  const diffMonths = Math.floor(diffDays / 30);
  const diffYears = Math.floor(diffDays / 365);

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
