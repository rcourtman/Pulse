import { formatBytes } from '@/utils/format';
import {
  getRecoveryNiceAxisMax,
  recoveryDateKeyFromTimestamp,
} from '@/utils/recoveryDatePresentation';
import { getRecoveryTimelineLabelEvery } from '@/utils/recoveryTimelineChartPresentation';
import { getRecoveryTimelineDayFilterStateLabel } from '@/utils/recoveryTimelinePresentation';

import { getProxmoxBackupSourcePresentation } from './proxmoxBackupSourcePresentation';

export type BackupActivityRangeDays = 7 | 30 | 90 | 365;

export const BACKUP_ACTIVITY_RANGE_DAYS = [
  7, 30, 90, 365,
] as const satisfies readonly BackupActivityRangeDays[];

// count accumulates one item per day; volume accumulates bytes per day.
export type BackupActivityMetricMode = 'count' | 'volume';

export type BackupActivitySegmentKind =
  'archive' | 'pbs' | 'ok' | 'failed' | 'running' | 'snapshot';

interface BackupActivitySegmentPresentation {
  label: string;
  segmentClassName: string;
  swatchClassName: string;
}

const SEGMENT_PRESENTATION: Record<BackupActivitySegmentKind, BackupActivitySegmentPresentation> = {
  archive: {
    label: getProxmoxBackupSourcePresentation('archive').timelineLabel,
    segmentClassName: getProxmoxBackupSourcePresentation('archive').timelineSegmentClassName,
    swatchClassName: getProxmoxBackupSourcePresentation('archive').timelineSwatchClassName,
  },
  pbs: {
    label: getProxmoxBackupSourcePresentation('pbs').timelineLabel,
    segmentClassName: getProxmoxBackupSourcePresentation('pbs').timelineSegmentClassName,
    swatchClassName: getProxmoxBackupSourcePresentation('pbs').timelineSwatchClassName,
  },
  ok: {
    label: 'OK',
    segmentClassName: 'bg-emerald-500',
    swatchClassName: 'bg-emerald-500',
  },
  failed: {
    label: 'Failed',
    segmentClassName: 'bg-red-500',
    swatchClassName: 'bg-red-500',
  },
  running: {
    label: 'Running',
    segmentClassName: 'bg-amber-500',
    swatchClassName: 'bg-amber-500',
  },
  snapshot: {
    label: getProxmoxBackupSourcePresentation('snapshot').timelineLabel,
    segmentClassName: getProxmoxBackupSourcePresentation('snapshot').timelineSegmentClassName,
    swatchClassName: getProxmoxBackupSourcePresentation('snapshot').timelineSwatchClassName,
  },
};

export function getBackupActivitySegmentPresentation(
  kind: BackupActivitySegmentKind,
): BackupActivitySegmentPresentation {
  return SEGMENT_PRESENTATION[kind];
}

export interface BackupActivityPoint {
  key: string;
  total: number;
  counts: Record<BackupActivitySegmentKind, number>;
  // Value-mode observations without a usable positive reading still establish
  // activity. They cannot become zero-byte backups or disappear from the day.
  unmeasuredCount?: number;
}

export interface BackupActivityTimeline {
  points: BackupActivityPoint[];
  axisMax: number;
  labelEvery: number;
}

function emptyCounts(): Record<BackupActivitySegmentKind, number> {
  return { archive: 0, pbs: 0, ok: 0, failed: 0, running: 0, snapshot: 0 };
}

function startOfLocalDayMs(date: Date, dayOffset: number): number {
  return new Date(date.getFullYear(), date.getMonth(), date.getDate() + dayOffset).getTime();
}

export function buildBackupActivityTimeline<T>(
  days: BackupActivityRangeDays,
  items: readonly T[],
  getTimestampMs: (item: T) => number | undefined,
  classify: (item: T) => BackupActivitySegmentKind | null,
  options?: {
    now?: Date;
    // Per-item contribution to the bucket total. Defaults to count mode.
    getValue?: (item: T) => number;
  },
): BackupActivityTimeline {
  const now = options?.now ?? new Date();
  const getValue = options?.getValue ?? (() => 1);
  // The chart and its day filter use local calendar dates. A clock change
  // can make a day shorter or longer than 24 hours, including half-hour shifts.
  const windowStart = startOfLocalDayMs(now, 1 - days);

  const buckets = new Map<string, BackupActivityPoint>();
  const orderedKeys: string[] = [];

  for (let i = 0; i < days; i += 1) {
    const dayStart = startOfLocalDayMs(now, 1 - days + i);
    const key = recoveryDateKeyFromTimestamp(dayStart);
    orderedKeys.push(key);
    buckets.set(key, { key, total: 0, counts: emptyCounts() });
  }

  for (const item of items) {
    const ts = getTimestampMs(item);
    if (ts === undefined || !Number.isFinite(ts)) continue;
    // A future time on today's date is still not observed past activity.
    if (ts > now.getTime()) continue;
    if (ts < windowStart) continue;
    const kind = classify(item);
    if (!kind) continue;
    const key = recoveryDateKeyFromTimestamp(ts);
    const bucket = buckets.get(key);
    if (!bucket) continue;
    const value = getValue(item);
    if (!Number.isFinite(value) || value <= 0) {
      bucket.unmeasuredCount = (bucket.unmeasuredCount ?? 0) + 1;
      continue;
    }
    bucket.counts[kind] += value;
    bucket.total += value;
  }

  const points = orderedKeys.map((key) => buckets.get(key)!);
  const rawMax = points.reduce((max, p) => (p.total > max ? p.total : max), 0);
  const axisMax = Math.max(2, getRecoveryNiceAxisMax(rawMax));
  const labelEvery = getRecoveryTimelineLabelEvery(points.length);

  return { points, axisMax, labelEvery };
}

export interface BackupActivityTooltipRow {
  kind: BackupActivitySegmentKind;
  label: string;
  count: number;
  value: string;
  segmentClassName: string;
  muted: boolean;
}

export type BackupActivityNoun = 'archive' | 'artifact' | 'backup' | 'task' | 'snapshot';

function formatActivityValue(value: number, mode: BackupActivityMetricMode): string {
  if (mode === 'volume') return formatBytes(Math.max(0, value));
  return String(Math.max(0, Math.round(value)));
}

export function getBackupActivityTooltipRows(
  point: BackupActivityPoint,
  kinds: readonly BackupActivitySegmentKind[],
  mode: BackupActivityMetricMode = 'count',
): BackupActivityTooltipRow[] {
  const total = Math.max(0, point.total);
  return kinds.map((kind) => {
    const presentation = SEGMENT_PRESENTATION[kind];
    const count = Math.max(0, point.counts[kind] ?? 0);
    const percentage = total > 0 && count > 0 ? Math.round((count / total) * 100) : 0;
    const formatted = formatActivityValue(count, mode);
    return {
      kind,
      label: presentation.label,
      count,
      value: percentage > 0 && kinds.length > 1 ? `${formatted} (${percentage}%)` : formatted,
      segmentClassName: presentation.segmentClassName,
      muted: count === 0,
    };
  });
}

export function getBackupActivityPointTotalLabel(
  total: number,
  noun: BackupActivityNoun,
  mode: BackupActivityMetricMode = 'count',
  unmeasuredCount = 0,
): string {
  if (mode === 'volume') {
    if (unmeasuredCount > 0) {
      const missing = `${unmeasuredCount} ${noun}${unmeasuredCount === 1 ? '' : 's'} without a positive size`;
      return `${total > 0 ? `${formatBytes(total)} reported` : 'Size unavailable'}. ${missing}`;
    }
    return formatBytes(Math.max(0, total));
  }
  const normalized = Math.max(0, Math.round(total));
  if (normalized === 1) return `1 ${noun}`;
  return `${normalized} ${noun}s`;
}

export function getBackupActivityColumnAriaLabel(
  dateLabel: string,
  total: number,
  selected: boolean,
  noun: BackupActivityNoun,
  mode: BackupActivityMetricMode = 'count',
  unmeasuredCount = 0,
): string {
  const countLabel = getBackupActivityPointTotalLabel(total, noun, mode, unmeasuredCount);
  return selected ? `${dateLabel}: ${countLabel}, selected` : `${dateLabel}: ${countLabel}`;
}

export function getBackupActivityAxisLabel(value: number, mode: BackupActivityMetricMode): string {
  if (mode === 'volume') return formatBytes(Math.max(0, value));
  return String(Math.max(0, Math.round(value)));
}

export { getRecoveryTimelineDayFilterStateLabel as getBackupActivityDayFilterStateLabel };
