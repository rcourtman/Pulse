import type { MetricPoint } from '@/api/charts';
import { formatBytes } from '@/utils/format';

export interface StorageCapacityDeltaPresentation {
  deltaBytes: number | null;
  label: string;
  title: string;
  toneClass: string;
}

export interface StorageCapacityDeltaAnalysis {
  deltaBytes: number;
  durationMs: number;
  startTimestamp: number;
  endTimestamp: number;
}

function normalizeMetricPoints(points: MetricPoint[]): MetricPoint[] {
  return points
    .filter((point) => Number.isFinite(point.timestamp) && Number.isFinite(point.value))
    .sort((left, right) => left.timestamp - right.timestamp);
}

function averageMetricValues(points: MetricPoint[]): number {
  if (points.length === 0) {
    return 0;
  }
  return points.reduce((sum, point) => sum + point.value, 0) / points.length;
}

function averageMetricTimestamps(points: MetricPoint[]): number {
  if (points.length === 0) {
    return 0;
  }
  return points.reduce((sum, point) => sum + point.timestamp, 0) / points.length;
}

export function computeStorageCapacityDeltaAnalysis(
  points: MetricPoint[],
): StorageCapacityDeltaAnalysis | null {
  const normalized = normalizeMetricPoints(points);
  if (normalized.length < 2) {
    return null;
  }

  const sampleWindowSize =
    normalized.length >= 4 ? Math.max(2, Math.floor(normalized.length * 0.25)) : 1;
  const startWindow = normalized.slice(0, sampleWindowSize);
  const endWindow = normalized.slice(normalized.length - sampleWindowSize);
  const startAverage = averageMetricValues(startWindow);
  const endAverage = averageMetricValues(endWindow);
  const startTimestamp = averageMetricTimestamps(startWindow);
  const endTimestamp = averageMetricTimestamps(endWindow);
  const durationMs = endTimestamp - startTimestamp;
  const delta = endAverage - startAverage;
  if (!Number.isFinite(delta) || !Number.isFinite(durationMs) || durationMs <= 0) {
    return null;
  }
  return {
    deltaBytes: Math.abs(delta) < 1 ? 0 : delta,
    durationMs,
    startTimestamp,
    endTimestamp,
  };
}

export function computeStorageCapacityDelta(points: MetricPoint[]): number | null {
  return computeStorageCapacityDeltaAnalysis(points)?.deltaBytes ?? null;
}

export function formatStorageCapacityDelta(deltaBytes: number | null): string {
  if (deltaBytes === null) {
    return '—';
  }
  if (deltaBytes === 0) {
    return '0 B';
  }
  const sign = deltaBytes > 0 ? '+' : '-';
  return `${sign}${formatBytes(Math.abs(deltaBytes))}`;
}

export function getStorageCapacityDeltaToneClass(deltaBytes: number | null): string {
  if (deltaBytes === null || deltaBytes === 0) {
    return 'text-muted';
  }
  if (deltaBytes > 0) {
    return 'text-amber-600 dark:text-amber-300';
  }
  return 'text-sky-600 dark:text-sky-300';
}

export function buildStorageCapacityDeltaPresentation(
  points: MetricPoint[],
  rangeLabel: string,
): StorageCapacityDeltaPresentation {
  const deltaBytes = computeStorageCapacityDelta(points);
  const label = formatStorageCapacityDelta(deltaBytes);
  if (deltaBytes === null) {
    return {
      deltaBytes,
      label,
      title: `No storage change history available for ${rangeLabel}.`,
      toneClass: getStorageCapacityDeltaToneClass(deltaBytes),
    };
  }
  if (deltaBytes === 0) {
    return {
      deltaBytes,
      label,
      title: `No used-capacity change over ${rangeLabel}.`,
      toneClass: getStorageCapacityDeltaToneClass(deltaBytes),
    };
  }
  const direction = deltaBytes > 0 ? 'Used capacity grew' : 'Used capacity shrank';
  return {
    deltaBytes,
    label,
    title: `${direction} by ${formatBytes(Math.abs(deltaBytes))} over ${rangeLabel}.`,
    toneClass: getStorageCapacityDeltaToneClass(deltaBytes),
  };
}
