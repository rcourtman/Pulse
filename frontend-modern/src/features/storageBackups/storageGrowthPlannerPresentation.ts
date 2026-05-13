import type { MetricPoint, StoragePoolChartData } from '@/api/charts';
import { formatBytes, formatPercent } from '@/utils/format';
import type { StorageRecord } from './models';
import {
  buildStorageCapacityDeltaPresentation,
  computeStorageCapacityDeltaAnalysis,
} from './storageCapacityDeltaPresentation';
import { getStorageRecordHostLabel } from './recordPresentation';
import { resolveStorageRecordMetricResourceId } from './storageMetricsIdentity';

const DAY_MS = 24 * 60 * 60 * 1000;
const TOP_POOL_LIMIT = 3;

export type StorageGrowthPlannerPriority = 'critical' | 'warning' | 'watch' | 'stable' | 'unknown';

export interface StorageGrowthPlannerPool {
  recordId: string;
  seriesId: string;
  name: string;
  hostLabel: string;
  usageLabel: string;
  freeLabel: string;
  growthDeltaBytes: number | null;
  growthLabel: string;
  growthTitle: string;
  growthToneClass: string;
  runwayDays: number | null;
  runwayLabel: string;
  runwayTitle: string;
  priority: StorageGrowthPlannerPriority;
  priorityLabel: string;
  priorityToneClass: string;
}

export interface StorageGrowthPlannerPresentation {
  rangeLabel: string;
  trackedPoolCount: number;
  growingPoolCount: number;
  planningPools: StorageGrowthPlannerPool[];
  topPools: StorageGrowthPlannerPool[];
  emptyTitle: string;
  emptyMessage: string;
}

function asFiniteNonNegative(value: number | null | undefined): number | null {
  if (typeof value !== 'number' || !Number.isFinite(value) || value < 0) {
    return null;
  }
  return value;
}

function getLatestMetricValue(points: MetricPoint[] | undefined): number | null {
  if (!points) {
    return null;
  }
  let latest: MetricPoint | null = null;
  for (const point of points) {
    if (!Number.isFinite(point.timestamp) || !Number.isFinite(point.value)) {
      continue;
    }
    if (!latest || point.timestamp > latest.timestamp) {
      latest = point;
    }
  }
  return asFiniteNonNegative(latest?.value);
}

function formatRunwayDays(runwayDays: number | null): string {
  if (runwayDays === null) {
    return 'Unknown';
  }
  if (runwayDays <= 0) {
    return 'Full now';
  }
  if (runwayDays < 1) {
    return '<1 day';
  }
  if (runwayDays < 90) {
    const days = Math.max(1, Math.round(runwayDays));
    return `${days} ${days === 1 ? 'day' : 'days'}`;
  }
  const months = Math.max(3, Math.round(runwayDays / 30));
  if (months < 24) {
    return `${months} mo`;
  }
  const years = Math.round(months / 12);
  return `${years} yr`;
}

function getPriority(
  usagePercent: number | null,
  growthDeltaBytes: number | null,
  runwayDays: number | null,
): StorageGrowthPlannerPriority {
  if (runwayDays !== null) {
    if (runwayDays <= 30) return 'critical';
    if (runwayDays <= 90) return 'warning';
    return 'watch';
  }
  if ((usagePercent ?? 0) >= 90) {
    return 'critical';
  }
  if ((usagePercent ?? 0) >= 80) {
    return 'warning';
  }
  if ((growthDeltaBytes ?? 0) > 0) {
    return 'unknown';
  }
  return 'stable';
}

function getPriorityLabel(priority: StorageGrowthPlannerPriority): string {
  switch (priority) {
    case 'critical':
      return 'Plan now';
    case 'warning':
      return 'Plan soon';
    case 'watch':
      return 'Watch';
    case 'unknown':
      return 'Needs trend';
    case 'stable':
      return 'Stable';
  }
}

function getPriorityToneClass(priority: StorageGrowthPlannerPriority): string {
  switch (priority) {
    case 'critical':
      return 'border-red-200 bg-red-50 text-red-700 dark:border-red-900/60 dark:bg-red-950/30 dark:text-red-300';
    case 'warning':
      return 'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-300';
    case 'watch':
      return 'border-sky-200 bg-sky-50 text-sky-700 dark:border-sky-900/60 dark:bg-sky-950/30 dark:text-sky-300';
    case 'unknown':
      return 'border-border bg-surface-alt text-muted';
    case 'stable':
      return 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-900/60 dark:bg-emerald-950/30 dark:text-emerald-300';
  }
}

function buildRunwayTitle(
  poolName: string,
  runwayDays: number | null,
  growthDeltaBytes: number | null,
  rangeLabel: string,
): string {
  if ((growthDeltaBytes ?? 0) <= 0) {
    return `${poolName} is not growing over ${rangeLabel}.`;
  }
  if (runwayDays === null) {
    return `${poolName} needs more capacity data before runway can be estimated.`;
  }
  return `${poolName} reaches full capacity in about ${formatRunwayDays(runwayDays)} at the ${rangeLabel} growth rate.`;
}

function buildPoolPresentation(
  record: StorageRecord,
  chartData: StoragePoolChartData | undefined,
  rangeLabel: string,
): StorageGrowthPlannerPool {
  const seriesId = resolveStorageRecordMetricResourceId(record);
  const totalBytes = asFiniteNonNegative(record.capacity.totalBytes);
  const usedBytes = asFiniteNonNegative(record.capacity.usedBytes);
  const latestAvailBytes = getLatestMetricValue(chartData?.avail);
  const freeBytes =
    asFiniteNonNegative(record.capacity.freeBytes) ??
    (totalBytes !== null && usedBytes !== null ? Math.max(totalBytes - usedBytes, 0) : null) ??
    latestAvailBytes;
  const effectiveTotalBytes =
    totalBytes ??
    (usedBytes !== null && freeBytes !== null ? Math.max(usedBytes + freeBytes, 0) : null);
  const usagePercent =
    typeof record.capacity.usagePercent === 'number' &&
    Number.isFinite(record.capacity.usagePercent)
      ? record.capacity.usagePercent
      : effectiveTotalBytes && usedBytes !== null
        ? (usedBytes / effectiveTotalBytes) * 100
        : null;
  const deltaPresentation = buildStorageCapacityDeltaPresentation(
    chartData?.used ?? [],
    rangeLabel,
  );
  const deltaAnalysis = computeStorageCapacityDeltaAnalysis(chartData?.used ?? []);
  const growthRatePerMs =
    deltaAnalysis && deltaAnalysis.deltaBytes > 0
      ? deltaAnalysis.deltaBytes / deltaAnalysis.durationMs
      : null;
  const runwayDays =
    growthRatePerMs && freeBytes !== null
      ? Math.max(0, freeBytes / growthRatePerMs / DAY_MS)
      : null;
  const priority = getPriority(usagePercent, deltaPresentation.deltaBytes, runwayDays);

  return {
    recordId: record.id,
    seriesId,
    name: record.name,
    hostLabel: getStorageRecordHostLabel(record),
    usageLabel: usagePercent === null ? 'Usage n/a' : `${formatPercent(usagePercent)} used`,
    freeLabel: freeBytes === null ? 'Free n/a' : `${formatBytes(freeBytes)} free`,
    growthDeltaBytes: deltaPresentation.deltaBytes,
    growthLabel: deltaPresentation.label,
    growthTitle: deltaPresentation.title,
    growthToneClass: deltaPresentation.toneClass,
    runwayDays,
    runwayLabel: formatRunwayDays(runwayDays),
    runwayTitle: buildRunwayTitle(
      record.name,
      runwayDays,
      deltaPresentation.deltaBytes,
      rangeLabel,
    ),
    priority,
    priorityLabel: getPriorityLabel(priority),
    priorityToneClass: getPriorityToneClass(priority),
  };
}

function comparePlanningPools(
  left: StorageGrowthPlannerPool,
  right: StorageGrowthPlannerPool,
): number {
  const priorityRank: Record<StorageGrowthPlannerPriority, number> = {
    critical: 0,
    warning: 1,
    watch: 2,
    unknown: 3,
    stable: 4,
  };
  const priorityDelta = priorityRank[left.priority] - priorityRank[right.priority];
  if (priorityDelta !== 0) {
    return priorityDelta;
  }
  const leftRunway = left.runwayDays ?? Number.POSITIVE_INFINITY;
  const rightRunway = right.runwayDays ?? Number.POSITIVE_INFINITY;
  if (leftRunway !== rightRunway) {
    return leftRunway - rightRunway;
  }
  return (right.growthDeltaBytes ?? 0) - (left.growthDeltaBytes ?? 0);
}

export function buildStorageGrowthPlannerPresentation(params: {
  records: StorageRecord[];
  pools: Record<string, StoragePoolChartData> | undefined;
  rangeLabel: string;
}): StorageGrowthPlannerPresentation {
  const planningPools = params.records
    .map((record) => {
      const seriesId = resolveStorageRecordMetricResourceId(record);
      return buildPoolPresentation(record, params.pools?.[seriesId], params.rangeLabel);
    })
    .sort(comparePlanningPools);
  const topPools = planningPools
    .filter((pool) => pool.priority !== 'stable')
    .slice(0, TOP_POOL_LIMIT);
  const growingPoolCount = planningPools.filter((pool) => (pool.growthDeltaBytes ?? 0) > 0).length;

  return {
    rangeLabel: params.rangeLabel,
    trackedPoolCount: planningPools.length,
    growingPoolCount,
    planningPools,
    topPools,
    emptyTitle:
      planningPools.length === 0
        ? 'No pools in scope'
        : `No capacity planning pressure over ${params.rangeLabel}`,
    emptyMessage:
      planningPools.length === 0
        ? 'Storage filters have removed every pool from the planner.'
        : 'Tracked pools are stable or shrinking for the selected range.',
  };
}
