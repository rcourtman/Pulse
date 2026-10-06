// Type-safe formatting utilities
import type { Disk } from '@/types/api';
import type { Resource } from '@/types/resource';

type DiskInput = {
  device?: string;
  mountpoint?: string;
  filesystem?: string;
  type?: string;
  total?: number;
  used?: number;
  free?: number;
  usage?: number;
};

const NON_OPERATIONAL_DISK_MOUNT_PREFIXES = [
  '/System/Volumes/',
  '/Library/Developer/CoreSimulator/Volumes/',
];

const NON_OPERATIONAL_DISK_MOUNTPOINTS = new Set([
  '/boot/efi',
  '/boot/firmware',
  '/etc/pve',
  'System Reserved',
]);

function shouldDisplayDisk(disk: DiskInput): boolean {
  const mountpoint = disk.mountpoint?.trim() ?? '';
  if (!mountpoint) return true;
  if (NON_OPERATIONAL_DISK_MOUNTPOINTS.has(mountpoint)) return false;
  if (mountpoint.includes('System Reserved')) return false;
  return !NON_OPERATIONAL_DISK_MOUNT_PREFIXES.some((prefix) => mountpoint.startsWith(prefix));
}

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

export function formatObservedSpeed(bytesPerSecond: number | null | undefined): string {
  return typeof bytesPerSecond === 'number' &&
    Number.isFinite(bytesPerSecond) &&
    bytesPerSecond >= 0
    ? formatSpeed(bytesPerSecond)
    : '-';
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

/** Keep small observed CPU readings distinct from an actual zero. */
export function formatCpuPercent(value: number): string {
  if (!Number.isFinite(value)) return '—';
  if (value > 0 && value < 0.1) return '<0.1%';
  if (value > 0 && value < 10) return `${value.toFixed(1).replace(/\.0$/, '')}%`;
  return formatPercent(value);
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
 * @param options.now - The time to measure from (default: Date.now()). A cell
 *   that must keep its age current passes the shared relative-time clock.
 */
export function formatRelativeTime(
  timestamp: number | string | Date | undefined,
  options?: { compact?: boolean; emptyText?: string; now?: number },
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

  const diffMs = (options?.now ?? Date.now()) - ms;
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

export type BackupStatus = 'fresh' | 'stale' | 'overdue' | 'never' | 'unknown';

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
 * @param now - Optional current time override for deterministic consumers and tests
 * @returns BackupInfo with status and formatted age
 */
export function getBackupInfo(
  lastBackup: string | number | null | undefined,
  thresholds?: BackupThresholds,
  now: number | Date = Date.now(),
): BackupInfo {
  // Only the explicit absence sentinels mean no completed backup was found.
  // A present but unusable timestamp is uncertainty, not absence or freshness.
  if (lastBackup === null || lastBackup === undefined || lastBackup === '' || lastBackup === 0) {
    return { status: 'never', ageMs: null, ageFormatted: 'Never' };
  }

  let timestamp: number;
  if (typeof lastBackup === 'string') {
    timestamp = new Date(lastBackup).getTime();
  } else {
    timestamp = lastBackup;
  }

  if (
    !Number.isFinite(timestamp) ||
    timestamp <= 0 ||
    !Number.isFinite(new Date(timestamp).getTime())
  ) {
    return {
      status: 'unknown',
      ageMs: null,
      ageFormatted: 'Backup time unavailable: invalid timestamp.',
    };
  }

  const nowMs = typeof now === 'number' ? now : now.getTime();
  if (!Number.isFinite(nowMs) || !Number.isFinite(new Date(nowMs).getTime())) {
    return {
      status: 'unknown',
      ageMs: null,
      ageFormatted: 'Backup age unavailable: invalid current time.',
    };
  }
  if (timestamp > nowMs) {
    return {
      status: 'unknown',
      ageMs: null,
      ageFormatted:
        'Backup time unavailable: timestamp is in the future. Check the Proxmox and browser clocks.',
    };
  }
  const ageMs = nowMs - timestamp;

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
    status = 'overdue';
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

// Metric bar labels are set at text-[10px]: the value semibold (with tabular
// digits while it animates), any detail after it at normal weight. Glyph
// widths in that type vary too much for a per-character constant (a digit is
// more than twice as wide as a full stop), so label widths are summed from
// per-glyph advances of the font the browser actually renders. A hidden probe
// carrying the label's own type classes measures them once.
const LABEL_FONT_PX = 10;
const LABEL_PROBE_FIRST_CODE = 0x20;
const LABEL_PROBE_LAST_CODE = 0x7e;
const LABEL_PROBE_REPEAT = 16;

interface LabelGlyphAdvances {
  // Semibold; a digit takes the wider of its proportional and tabular advance.
  value: readonly number[];
  // Normal weight.
  detail: readonly number[];
}

// Advances in px for U+0020..U+007E as the probe reads them in Chrome with the
// macOS system font (2026-10-03). They stand in wherever layout is
// unavailable, such as unit tests.
const FALLBACK_LABEL_GLYPH_ADVANCES: LabelGlyphAdvances = {
  value: [
    2.78, 3.54, 5.51, 6.7, 6.7, 10.11, 7.46, 3.43, 4.25, 4.25, 4.92, 6.7, 3.43, 4.92, 3.43, 3.37,
    6.79, 6.7, 6.7, 6.7, 6.85, 6.7, 6.8, 6.7, 6.88, 6.8, 3.43, 3.43, 6.7, 6.7, 6.7, 5.55, 9.36,
    7.25, 6.93, 7.45, 7.52, 6.25, 6.01, 7.68, 7.82, 3.13, 5.92, 7.05, 5.98, 9.08, 7.71, 7.93, 6.73,
    7.93, 6.93, 6.75, 6.64, 7.65, 7.18, 10.08, 7.25, 7.03, 6.83, 4.25, 3.37, 4.25, 6.7, 5.88, 5.12,
    5.89, 6.51, 5.9, 6.51, 6.03, 4.08, 6.46, 6.3, 2.86, 2.86, 5.94, 2.93, 9.24, 6.25, 6.22, 6.47,
    6.47, 4.35, 5.55, 4.11, 6.25, 5.82, 8.35, 5.74, 5.91, 5.69, 4.25, 2.98, 4.25, 6.7,
  ],
  detail: [
    2.93, 3.23, 4.9, 6.42, 6.42, 9.38, 7.24, 3.09, 3.94, 3.94, 4.84, 6.42, 3.09, 4.84, 3.09, 3.17,
    6.42, 4.76, 6.16, 6.39, 6.56, 6.3, 6.49, 6, 6.51, 6.49, 3.09, 3.09, 6.42, 6.42, 6.42, 5.25, 9.3,
    6.86, 6.69, 7.28, 7.39, 6.08, 5.84, 7.59, 7.54, 2.8, 5.5, 6.71, 5.8, 8.86, 7.54, 7.84, 6.47,
    7.84, 6.66, 6.49, 6.46, 7.5, 6.86, 9.8, 6.91, 6.67, 6.74, 3.94, 3.17, 3.94, 6.42, 5.59, 5.12,
    5.64, 6.26, 5.72, 6.26, 5.84, 3.74, 6.22, 6.01, 2.59, 2.59, 5.55, 2.65, 8.82, 5.96, 6.03, 6.23,
    6.22, 4.02, 5.27, 3.75, 5.96, 5.54, 7.87, 5.37, 5.55, 5.51, 3.94, 2.71, 3.94, 6.42,
  ],
};

let labelGlyphAdvances: LabelGlyphAdvances | undefined;
let labelGlyphAdvancesPixelRatio = 0;

function measureLabelGlyphAdvances(): LabelGlyphAdvances | null {
  if (typeof document === 'undefined' || !document.body) return null;

  const probe = document.createElement('div');
  probe.setAttribute('aria-hidden', 'true');
  probe.className =
    'pointer-events-none invisible fixed left-0 top-0 whitespace-pre font-sans text-[10px] leading-none';
  const addGroup = (className: string, firstCode: number, lastCode: number) => {
    const group = document.createElement('div');
    group.className = className;
    for (let code = firstCode; code <= lastCode; code++) {
      const cell = document.createElement('div');
      cell.className = 'w-max';
      cell.textContent = String.fromCharCode(code).repeat(LABEL_PROBE_REPEAT);
      group.appendChild(cell);
    }
    probe.appendChild(group);
    return group;
  };
  const semiboldGroup = addGroup('font-semibold', LABEL_PROBE_FIRST_CODE, LABEL_PROBE_LAST_CODE);
  const tabularGroup = addGroup('font-semibold tabular-nums', 0x30, 0x39);
  const normalGroup = addGroup('font-normal', LABEL_PROBE_FIRST_CODE, LABEL_PROBE_LAST_CODE);

  document.body.appendChild(probe);
  const read = (group: HTMLElement) =>
    Array.from(group.children, (cell) => cell.getBoundingClientRect().width / LABEL_PROBE_REPEAT);
  const semibold = read(semiboldGroup);
  const tabular = read(tabularGroup);
  const normal = read(normalGroup);
  probe.remove();

  // No layout (jsdom), or a stubbed one that reports every box alike.
  const at = (advances: number[], glyph: string) =>
    advances[glyph.charCodeAt(0) - LABEL_PROBE_FIRST_CODE];
  if (!(at(normal, '.') > 0 && at(normal, 'M') > at(normal, '.'))) return null;

  const firstDigit = 0x30 - LABEL_PROBE_FIRST_CODE;
  tabular.forEach((advance, digit) => {
    semibold[firstDigit + digit] = Math.max(semibold[firstDigit + digit], advance);
  });
  return { value: semibold, detail: normal };
}

function getLabelGlyphAdvances(): LabelGlyphAdvances {
  // Zoom changes how glyph advances round, so the table is per pixel ratio.
  const pixelRatio = typeof window === 'undefined' ? 1 : window.devicePixelRatio || 1;
  if (!labelGlyphAdvances || pixelRatio !== labelGlyphAdvancesPixelRatio) {
    labelGlyphAdvances = measureLabelGlyphAdvances() ?? FALLBACK_LABEL_GLYPH_ADVANCES;
    labelGlyphAdvancesPixelRatio = pixelRatio;
  }
  return labelGlyphAdvances;
}

function sumLabelGlyphAdvances(text: string, advances: readonly number[]): number {
  let width = 0;
  for (let index = 0; index < text.length; index++) {
    // A glyph outside the measured range counts as a full em.
    width += advances[text.charCodeAt(index) - LABEL_PROBE_FIRST_CODE] ?? LABEL_FONT_PX;
  }
  return width;
}

export interface EstimateTextWidthOptions {
  /** Text that follows at normal weight, such as ` (265 GB/440 GB)`. */
  detail?: string;
  /** Label font size in px when it is not the bar label's 10px. */
  fontPx?: number;
}

/**
 * Estimate the rendered width in px of a metric bar label: `text` semibold,
 * `options.detail` after it at normal weight. The result is the text alone,
 * rounded up; callers add the padding their label sits in.
 * Used for determining if labels fit inside metric bars.
 * ALL text-width estimation MUST use this function.
 */
export function estimateTextWidth(text: string, options: EstimateTextWidthOptions = {}): number {
  const advances = getLabelGlyphAdvances();
  const width =
    sumLabelGlyphAdvances(text, advances.value) +
    sumLabelGlyphAdvances(options.detail ?? '', advances.detail);
  return Math.ceil((width * (options.fontPx ?? LABEL_FONT_PX)) / LABEL_FONT_PX);
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

export function formatAnomalyRatio(
  anomaly: { baseline_mean: number; current_value: number } | null | undefined,
): string | null {
  if (!anomaly || anomaly.baseline_mean === 0) return null;
  const ratio = anomaly.current_value / anomaly.baseline_mean;
  if (ratio >= 2) return `${ratio.toFixed(1)}x`;
  if (ratio >= 1.5) return '↑↑';
  return '↑';
}

/**
 * Shorten image registry URLs to the image leaf plus tag.
 * e.g., "ghcr.io/rcourtman/pulse:latest" -> "pulse:latest"
 */
export function getShortImageName(fullImage: string | undefined): string {
  if (!fullImage) return '—';
  // Handle case with @sha256: digests
  const cleanImage = fullImage.split('@')[0];
  const parts = cleanImage.split('/');
  return parts.at(-1) || cleanImage || '—';
}

/**
 * A Disk whose numeric fields are guaranteed present. The wire type leaves
 * them optional (the unified resources payload omits zero values); these
 * helpers compute and default them so consumers can render without guards.
 */
export type NormalizedDisk = Disk & {
  total: number;
  used: number;
  free: number;
  usage: number;
};

/**
 * Normalize raw disk objects (from API/agent) into proper Disk[].
 * Calculates `usage` from used/total and defaults missing fields.
 */
export function normalizeDiskArray(disks?: DiskInput[]): NormalizedDisk[] | undefined {
  if (!disks || disks.length === 0) return undefined;
  const normalized = disks.filter(shouldDisplayDisk).map((d) => {
    const total = d.total ?? 0;
    const used = d.used ?? 0;
    const free = d.free ?? (total > 0 ? Math.max(0, total - used) : 0);
    // usage < 0 is the poller's "usage unknown" sentinel (e.g. LXC mounts
    // known only from the guest config, #1477) — preserve it rather than
    // fabricating a computed percent.
    const usage =
      typeof d.usage === 'number' && d.usage < 0 ? -1 : total > 0 ? (used / total) * 100 : 0;
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
  return normalized.length > 0 ? normalized : undefined;
}

export function getResourceDiskSummary(
  resource: Pick<Resource, 'agent' | 'disk'>,
): NormalizedDisk | undefined {
  if (resource.disk) {
    const total = resource.disk.total ?? 0;
    const used = resource.disk.used ?? 0;
    return {
      total,
      used,
      free: resource.disk.free ?? Math.max(total - used, 0),
      usage: total > 0 ? (used / total) * 100 : resource.disk.current,
    };
  }

  // Disks with unknown usage (sentinel -1) have no measured used/free to sum.
  const disks = normalizeDiskArray(resource.agent?.disks)?.filter((disk) => disk.usage >= 0);
  if (!disks || disks.length === 0) return undefined;

  const total = disks.reduce((sum, disk) => sum + disk.total, 0);
  if (total <= 0) return undefined;
  const used = disks.reduce((sum, disk) => sum + disk.used, 0);
  const free = disks.reduce((sum, disk) => sum + disk.free, 0);

  return {
    total,
    used,
    free,
    usage: (used / total) * 100,
  };
}
