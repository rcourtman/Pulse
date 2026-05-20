import {
  getRecoveryNiceAxisMax,
  recoveryDateKeyFromTimestamp,
} from '@/utils/recoveryDatePresentation';
import { getRecoveryTimelineLabelEvery } from '@/utils/recoveryTimelineChartPresentation';
import { getRecoveryTimelineDayFilterStateLabel } from '@/utils/recoveryTimelinePresentation';

export type BackupActivityRangeDays = 7 | 30 | 90 | 365;

export const BACKUP_ACTIVITY_RANGE_DAYS = [
  7, 30, 90, 365,
] as const satisfies readonly BackupActivityRangeDays[];

export type BackupActivitySegmentKind = 'archive' | 'ok' | 'failed' | 'running';

interface BackupActivitySegmentPresentation {
  label: string;
  segmentClassName: string;
  swatchClassName: string;
}

const SEGMENT_PRESENTATION: Record<BackupActivitySegmentKind, BackupActivitySegmentPresentation> = {
  archive: {
    label: 'Archives',
    segmentClassName: 'bg-blue-500',
    swatchClassName: 'bg-blue-500',
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
}

export interface BackupActivityTimeline {
  points: BackupActivityPoint[];
  axisMax: number;
  labelEvery: number;
}

function emptyCounts(): Record<BackupActivitySegmentKind, number> {
  return { archive: 0, ok: 0, failed: 0, running: 0 };
}

function startOfLocalDayMs(date: Date): number {
  const copy = new Date(date);
  copy.setHours(0, 0, 0, 0);
  return copy.getTime();
}

export function buildBackupActivityTimeline<T>(
  days: BackupActivityRangeDays,
  items: readonly T[],
  getTimestampMs: (item: T) => number | undefined,
  classify: (item: T) => BackupActivitySegmentKind | null,
  now: Date = new Date(),
): BackupActivityTimeline {
  const todayStart = startOfLocalDayMs(now);
  const windowStart = todayStart - (days - 1) * 24 * 60 * 60 * 1000;

  const buckets = new Map<string, BackupActivityPoint>();
  const orderedKeys: string[] = [];

  for (let i = 0; i < days; i += 1) {
    const dayStart = windowStart + i * 24 * 60 * 60 * 1000;
    const key = recoveryDateKeyFromTimestamp(dayStart);
    orderedKeys.push(key);
    buckets.set(key, { key, total: 0, counts: emptyCounts() });
  }

  for (const item of items) {
    const ts = getTimestampMs(item);
    if (ts === undefined || !Number.isFinite(ts)) continue;
    if (ts < windowStart) continue;
    if (ts >= todayStart + 24 * 60 * 60 * 1000) continue;
    const kind = classify(item);
    if (!kind) continue;
    const key = recoveryDateKeyFromTimestamp(ts);
    const bucket = buckets.get(key);
    if (!bucket) continue;
    bucket.counts[kind] += 1;
    bucket.total += 1;
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

export function getBackupActivityTooltipRows(
  point: BackupActivityPoint,
  kinds: readonly BackupActivitySegmentKind[],
): BackupActivityTooltipRow[] {
  const total = Math.max(0, point.total);
  return kinds.map((kind) => {
    const presentation = SEGMENT_PRESENTATION[kind];
    const count = Math.max(0, point.counts[kind] ?? 0);
    const percentage = total > 0 && count > 0 ? Math.round((count / total) * 100) : 0;
    return {
      kind,
      label: presentation.label,
      count,
      value: percentage > 0 ? `${count} (${percentage}%)` : String(count),
      segmentClassName: presentation.segmentClassName,
      muted: count === 0,
    };
  });
}

export function getBackupActivityPointTotalLabel(total: number, noun: 'archive' | 'task'): string {
  const normalized = Math.max(0, total);
  if (normalized === 1) return `1 ${noun}`;
  return `${normalized} ${noun}s`;
}

export function getBackupActivityColumnAriaLabel(
  dateLabel: string,
  total: number,
  selected: boolean,
  noun: 'archive' | 'task',
): string {
  const countLabel = getBackupActivityPointTotalLabel(total, noun);
  return selected ? `${dateLabel}: ${countLabel}, selected` : `${dateLabel}: ${countLabel}`;
}

export { getRecoveryTimelineDayFilterStateLabel as getBackupActivityDayFilterStateLabel };
