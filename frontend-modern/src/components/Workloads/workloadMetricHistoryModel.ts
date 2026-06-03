import type { ChartData, MetricPoint, TimeRange } from '@/api/charts';
import type { Node } from '@/types/api';
import type { WorkloadGuest } from '@/types/workloads';
import { getCanonicalWorkloadId } from '@/utils/workloads';

export type WorkloadTableMetric = 'cpu' | 'memory' | 'disk' | 'netIo' | 'diskIo';
export type WorkloadTableMetricHistoryRange = Extract<TimeRange, '1h' | '12h' | '24h' | '7d'>;

export interface WorkloadMetricSparklineSeries {
  id: string;
  label: string;
  color: string;
  points: MetricPoint[];
}

export interface WorkloadMetricHistoryReader {
  getGuestMetricSeries: (
    guest: WorkloadGuest,
    metric: WorkloadTableMetric,
  ) => WorkloadMetricSparklineSeries[];
  getNodeMetricSeries: (node: Node, metric: WorkloadTableMetric) => WorkloadMetricSparklineSeries[];
}

export interface MetricMiniSparklineScale {
  minValue: number;
  maxValue: number;
}

export interface MetricMiniSparklineTimeRange {
  minTimestamp: number;
  maxTimestamp: number;
}

export interface MetricMiniSparklineHoverEntry {
  id: string;
  label: string;
  color: string;
  value: number;
  timestamp: number;
}

export interface MetricMiniSparklineHoverState {
  cursorX: number;
  cursorRatio: number;
  timestamp: number;
  entries: MetricMiniSparklineHoverEntry[];
}

const CPU_COLOR = '#8b5cf6';
const MEMORY_COLOR = '#f59e0b';
const DISK_COLOR = '#10b981';
const NET_IN_COLOR = '#10b981';
const NET_OUT_COLOR = '#fb923c';
const DISK_READ_COLOR = '#3b82f6';
const DISK_WRITE_COLOR = '#f59e0b';

export const WORKLOAD_TABLE_HISTORY_RANGES: WorkloadTableMetricHistoryRange[] = [
  '1h',
  '12h',
  '24h',
  '7d',
];
export const WORKLOAD_TABLE_HISTORY_RANGE_LABELS: Record<WorkloadTableMetricHistoryRange, string> =
  {
    '1h': '1h',
    '12h': '12h',
    '24h': '24h',
    '7d': '7d',
  };
export const WORKLOAD_TABLE_HISTORY_DEFAULT_RANGE: WorkloadTableMetricHistoryRange = '1h';
export const WORKLOAD_TABLE_HISTORY_MAX_POINTS = 72;
export const WORKLOAD_TABLE_HISTORY_POLL_MS = 30_000;

export const WORKLOAD_TABLE_HISTORY_INFRA_METRICS = [
  'cpu',
  'memory',
  'disk',
  'diskread',
  'diskwrite',
  'netin',
  'netout',
] as const;

export const isWorkloadTableMetricHistoryRange = (
  value: string,
): value is WorkloadTableMetricHistoryRange =>
  (WORKLOAD_TABLE_HISTORY_RANGES as readonly string[]).includes(value);

const clampPercent = (value: number): number => {
  if (!Number.isFinite(value)) return 0;
  if (value < 0) return 0;
  if (value > 100) return 100;
  return value;
};

const clampNonNegative = (value: number): number => {
  if (!Number.isFinite(value) || value < 0) return 0;
  return value;
};

export const normalizeWorkloadChartKey = (id: string): string => {
  const trimmed = id.trim();
  if (!trimmed) return '';
  if (trimmed.includes(':')) return trimmed;

  const parts = trimmed.split('-');
  const vmid = parts[parts.length - 1];

  if (parts.length === 2 && /^\d+$/.test(vmid)) {
    const node = parts[0];
    return node ? `${node}:${node}:${vmid}` : trimmed;
  }

  if (parts.length < 3) return trimmed;
  const node = parts[parts.length - 2];
  const instance = parts.slice(0, -2).join('-');
  if (!instance || !node || !/^\d+$/.test(vmid)) return trimmed;
  return `${instance}:${node}:${vmid}`;
};

const pushCandidate = (candidates: string[], value?: string | number | null) => {
  const candidate = String(value ?? '').trim();
  if (!candidate || candidates.includes(candidate)) return;
  candidates.push(candidate);
};

export const getWorkloadChartKeyCandidates = (guest: WorkloadGuest): string[] => {
  const candidates: string[] = [];
  const canonicalId = getCanonicalWorkloadId(guest);
  pushCandidate(candidates, canonicalId);
  pushCandidate(candidates, guest.id);
  pushCandidate(candidates, normalizeWorkloadChartKey(guest.id || ''));
  pushCandidate(candidates, guest.containerId);

  const node = (guest.node || '').trim();
  const instance = (guest.instance || '').trim();
  if (node && Number.isFinite(guest.vmid) && guest.vmid > 0) {
    pushCandidate(candidates, `${node}-${guest.vmid}`);
    pushCandidate(candidates, `${node}:${node}:${guest.vmid}`);
    if (instance) {
      pushCandidate(candidates, `${instance}-${node}-${guest.vmid}`);
      pushCandidate(candidates, `${instance}:${node}:${guest.vmid}`);
    }
  }

  return candidates;
};

export const getNodeChartKeyCandidates = (node: Node): string[] => {
  const candidates: string[] = [];
  pushCandidate(candidates, node.id);
  pushCandidate(candidates, node.linkedAgentId);
  pushCandidate(candidates, node.name);
  pushCandidate(candidates, `${node.instance}-${node.name}`);
  pushCandidate(candidates, `${node.instance}:${node.name}`);
  if (node.id.startsWith('agent:')) {
    pushCandidate(candidates, node.id.slice('agent:'.length));
  }
  return candidates;
};

type ChartDataMap = Record<string, ChartData | undefined> | Map<string, ChartData> | undefined;

const readChartData = (source: ChartDataMap, key: string): ChartData | undefined => {
  if (!source || !key) return undefined;
  if (source instanceof Map) return source.get(key);
  return source[key];
};

export const findChartDataForCandidates = (
  candidates: readonly string[],
  sources: readonly ChartDataMap[],
): ChartData | undefined => {
  for (const candidate of candidates) {
    const normalizedCandidate = normalizeWorkloadChartKey(candidate);
    for (const source of sources) {
      const direct = readChartData(source, candidate);
      if (direct) return direct;
      if (normalizedCandidate !== candidate) {
        const normalized = readChartData(source, normalizedCandidate);
        if (normalized) return normalized;
      }
    }
  }
  return undefined;
};

const sanitizeMetricPoints = (
  points: MetricPoint[] | undefined,
  clamp: (value: number) => number,
): MetricPoint[] => {
  if (!points || points.length === 0) return [];
  return points
    .filter((point) => Number.isFinite(point.timestamp) && Number.isFinite(point.value))
    .map((point) => ({ timestamp: point.timestamp, value: clamp(point.value) }))
    .sort((a, b) => a.timestamp - b.timestamp);
};

export const getMetricSparklineSeriesFromChartData = (
  chartData: ChartData | undefined,
  metric: WorkloadTableMetric,
): WorkloadMetricSparklineSeries[] => {
  if (!chartData) return [];

  switch (metric) {
    case 'cpu':
      return [
        {
          id: 'cpu',
          label: 'CPU',
          color: CPU_COLOR,
          points: sanitizeMetricPoints(chartData.cpu, clampPercent),
        },
      ];
    case 'memory':
      return [
        {
          id: 'memory',
          label: 'Memory',
          color: MEMORY_COLOR,
          points: sanitizeMetricPoints(chartData.memory, clampPercent),
        },
      ];
    case 'disk':
      return [
        {
          id: 'disk',
          label: 'Disk',
          color: DISK_COLOR,
          points: sanitizeMetricPoints(chartData.disk, clampPercent),
        },
      ];
    case 'netIo':
      return [
        {
          id: 'netin',
          label: 'In',
          color: NET_IN_COLOR,
          points: sanitizeMetricPoints(chartData.netin, clampNonNegative),
        },
        {
          id: 'netout',
          label: 'Out',
          color: NET_OUT_COLOR,
          points: sanitizeMetricPoints(chartData.netout, clampNonNegative),
        },
      ];
    case 'diskIo':
      return [
        {
          id: 'diskread',
          label: 'Read',
          color: DISK_READ_COLOR,
          points: sanitizeMetricPoints(chartData.diskread, clampNonNegative),
        },
        {
          id: 'diskwrite',
          label: 'Write',
          color: DISK_WRITE_COLOR,
          points: sanitizeMetricPoints(chartData.diskwrite, clampNonNegative),
        },
      ];
    default:
      return [];
  }
};

export const hasRenderableMetricSeries = (
  series: readonly WorkloadMetricSparklineSeries[],
): boolean => series.some((item) => item.points.length >= 2);

export const getMetricMiniSparklineScale = (
  series: readonly WorkloadMetricSparklineSeries[],
  unit?: string,
): MetricMiniSparklineScale => {
  if (unit === '%') {
    return { minValue: 0, maxValue: 100 };
  }

  let maxValue = 0;
  for (const item of series) {
    for (const point of item.points) {
      if (Number.isFinite(point.value) && point.value > maxValue) {
        maxValue = point.value;
      }
    }
  }

  return {
    minValue: 0,
    maxValue: Math.max(1, maxValue * 1.15),
  };
};

export const getMetricMiniSparklineTimeRange = (
  series: readonly WorkloadMetricSparklineSeries[],
): MetricMiniSparklineTimeRange | null => {
  let minTimestamp = Number.POSITIVE_INFINITY;
  let maxTimestamp = Number.NEGATIVE_INFINITY;

  for (const item of series) {
    for (const point of item.points) {
      if (!Number.isFinite(point.timestamp) || !Number.isFinite(point.value)) continue;
      minTimestamp = Math.min(minTimestamp, point.timestamp);
      maxTimestamp = Math.max(maxTimestamp, point.timestamp);
    }
  }

  if (!Number.isFinite(minTimestamp) || !Number.isFinite(maxTimestamp)) return null;
  if (maxTimestamp <= minTimestamp) return null;
  return { minTimestamp, maxTimestamp };
};

export const buildMetricMiniSparklinePath = (
  points: readonly MetricPoint[],
  scale: MetricMiniSparklineScale,
  width = 96,
  height = 18,
  timeRange?: MetricMiniSparklineTimeRange | null,
): string => {
  const renderable = points
    .filter((point) => Number.isFinite(point.timestamp) && Number.isFinite(point.value))
    .sort((a, b) => a.timestamp - b.timestamp);
  if (renderable.length < 2) return '';

  const fallbackRange = {
    minTimestamp: renderable[0].timestamp,
    maxTimestamp: renderable[renderable.length - 1].timestamp,
  };
  const activeRange =
    timeRange && timeRange.maxTimestamp > timeRange.minTimestamp ? timeRange : fallbackRange;
  const minTimestamp = activeRange.minTimestamp;
  const maxTimestamp = activeRange.maxTimestamp;
  const timeSpan = Math.max(1, maxTimestamp - minTimestamp);
  const valueSpan = Math.max(1, scale.maxValue - scale.minValue);
  const xPadding = 1;
  const yPadding = 2;
  const plotWidth = width - xPadding * 2;
  const plotHeight = height - yPadding * 2;

  return renderable
    .map((point, index) => {
      const x = xPadding + ((point.timestamp - minTimestamp) / timeSpan) * plotWidth;
      const value = Math.min(Math.max(point.value, scale.minValue), scale.maxValue);
      const y = yPadding + (1 - (value - scale.minValue) / valueSpan) * plotHeight;
      return `${index === 0 ? 'M' : 'L'}${x.toFixed(2)},${y.toFixed(2)}`;
    })
    .join(' ');
};

const findNearestMetricMiniSparklinePoint = (
  points: readonly MetricPoint[],
  targetTimestamp: number,
): MetricPoint | null => {
  const renderable = points
    .filter((point) => Number.isFinite(point.timestamp) && Number.isFinite(point.value))
    .sort((a, b) => a.timestamp - b.timestamp);
  if (renderable.length === 0) return null;

  let low = 0;
  let high = renderable.length - 1;
  while (low < high) {
    const mid = Math.floor((low + high) / 2);
    if (renderable[mid].timestamp < targetTimestamp) low = mid + 1;
    else high = mid;
  }

  const candidate = renderable[low];
  const previous = low > 0 ? renderable[low - 1] : candidate;
  return Math.abs(previous.timestamp - targetTimestamp) <=
    Math.abs(candidate.timestamp - targetTimestamp)
    ? previous
    : candidate;
};

export const computeMetricMiniSparklineHoverState = (
  series: readonly WorkloadMetricSparklineSeries[],
  cursorX: number,
  width: number,
): MetricMiniSparklineHoverState | null => {
  const timeRange = getMetricMiniSparklineTimeRange(series);
  if (!timeRange || !Number.isFinite(width) || width <= 0) return null;

  const clampedCursorX = Math.max(0, Math.min(cursorX, width));
  const cursorRatio = clampedCursorX / width;
  const targetTimestamp =
    timeRange.minTimestamp + cursorRatio * (timeRange.maxTimestamp - timeRange.minTimestamp);
  const entries: MetricMiniSparklineHoverEntry[] = [];
  let nearestTimestamp = targetTimestamp;
  let nearestDistance = Number.POSITIVE_INFINITY;

  for (const item of series) {
    const point = findNearestMetricMiniSparklinePoint(item.points, targetTimestamp);
    if (!point) continue;

    const distance = Math.abs(point.timestamp - targetTimestamp);
    if (distance < nearestDistance) {
      nearestDistance = distance;
      nearestTimestamp = point.timestamp;
    }

    entries.push({
      id: item.id,
      label: item.label,
      color: item.color,
      value: point.value,
      timestamp: point.timestamp,
    });
  }

  if (entries.length === 0) return null;
  return {
    cursorX: clampedCursorX,
    cursorRatio,
    entries,
    timestamp: nearestTimestamp,
  };
};

export const formatMetricMiniSparklineHoverTime = (timestamp: number): string =>
  new Date(timestamp).toLocaleString([], {
    month: 'short',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  });
