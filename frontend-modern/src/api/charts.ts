/**
 * Charts API
 *
 * Fetches historical metrics data from the backend for sparkline visualizations.
 * The backend maintains proper historical data with 30s sample intervals.
 */

import { apiFetchJSON } from '@/utils/apiClient';
import { canonicalizeFrontendResourceType } from '@/utils/resourceTypeCompat';

// Types matching backend response format
export interface MetricPoint {
  timestamp: number; // Unix timestamp in milliseconds
  value: number;
}

// Extended metric point with min/max for aggregated data
export interface AggregatedMetricPoint {
  timestamp: number; // Unix timestamp in milliseconds
  value: number;
  min: number;
  max: number;
}

export interface ChartData {
  cpu?: MetricPoint[];
  memory?: MetricPoint[];
  memoryused?: MetricPoint[];
  disk?: MetricPoint[];
  diskread?: MetricPoint[];
  diskwrite?: MetricPoint[];
  netin?: MetricPoint[];
  netout?: MetricPoint[];
}

export interface ChartStats {
  oldestDataTimestamp: number;
  range?: string;
  rangeSeconds?: number;
  metricsStoreEnabled?: boolean;
  primarySourceHint?: string;
  inMemoryThresholdSecs?: number;
  pointCounts?: {
    total?: number;
    guests?: number;
    nodes?: number;
    storage?: number;
    dockerContainers?: number;
    dockerHosts?: number;
    agents?: number;
  };
}

export interface ChartsResponse {
  data: Record<string, ChartData>; // VM/Container data keyed by ID
  nodeData: Record<string, ChartData>; // Node data keyed by ID
  storageData: Record<string, ChartData>; // Storage data keyed by ID
  dockerData?: Record<string, ChartData>; // Docker container data keyed by container ID
  dockerHostData?: Record<string, ChartData>; // Docker host data keyed by host ID
  agentData?: Record<string, ChartData>; // Unified agent data keyed by agent ID
  guestTypes?: Record<string, 'vm' | 'system-container' | 'k8s'>; // Maps guest ID to type
  timestamp: number;
  stats: ChartStats;
}

export interface InfrastructureChartsResponse {
  nodeData: Record<string, ChartData>;
  dockerHostData?: Record<string, ChartData>;
  agentData?: Record<string, ChartData>;
  timestamp: number;
  stats: ChartStats;
}

export type InfrastructureSummaryMetric =
  'cpu' | 'memory' | 'disk' | 'diskread' | 'diskwrite' | 'netin' | 'netout';

export interface WorkloadChartsResponse {
  data: Record<string, ChartData>;
  dockerData?: Record<string, ChartData>;
  guestTypes?: Record<string, 'vm' | 'system-container' | 'k8s'>;
  timestamp: number;
  stats: ChartStats;
}

// Persistent metrics history types (SQLite-backed, longer retention)
export type HistoryTimeRange = '30m' | '1h' | '6h' | '12h' | '24h' | '7d' | '14d' | '30d' | '90d';
type MetricsHistoryAPIResourceType =
  | 'node'
  | 'vm'
  | 'system-container'
  | 'oci-container'
  | 'app-container'
  | 'storage'
  | 'docker-host'
  | 'k8s'
  | 'agent'
  | 'disk';

export type ResourceType =
  | 'node'
  | 'agent'
  | 'vm'
  | 'system-container'
  | 'oci-container'
  | 'app-container'
  | 'storage'
  | 'docker-host'
  | 'k8s-cluster'
  | 'k8s-node'
  | 'k8s-deployment'
  | 'pod'
  | 'disk';

export interface MetricsHistoryParams {
  resourceType: ResourceType;
  resourceId: string;
  metric?: string; // Optional: 'cpu', 'memory', 'disk', etc. Omit for all metrics
  range?: HistoryTimeRange; // Default: '24h'
  maxPoints?: number; // Optional cap on returned points (backend may downsample)
  signal?: AbortSignal;
}

export function toMetricsHistoryAPIResourceType(
  resourceType: ResourceType,
): MetricsHistoryAPIResourceType {
  switch (resourceType) {
    case 'k8s-cluster':
    case 'k8s-node':
    case 'k8s-deployment':
    case 'pod':
      return 'k8s';
    default:
      return resourceType;
  }
}

export function asMetricsHistoryResourceType(type: string): ResourceType | null {
  const normalizedType = type.trim().toLowerCase();
  const historyTypes: ResourceType[] = [
    'node',
    'agent',
    'vm',
    'system-container',
    'oci-container',
    'app-container',
    'storage',
    'docker-host',
    'k8s-cluster',
    'k8s-node',
    'k8s-deployment',
    'pod',
    'disk',
  ];
  return historyTypes.includes(normalizedType as ResourceType)
    ? (normalizedType as ResourceType)
    : null;
}

export function mapUnifiedTypeToHistoryResourceType(type: string): ResourceType | null {
  const normalized = type.trim().toLowerCase();
  if (normalized === 'node') return 'node';
  const canonical = canonicalizeFrontendResourceType(type) || normalized;
  switch (canonical) {
    case 'agent':
      return 'agent';
    case 'docker-host':
      return 'docker-host';
    case 'k8s-node':
      return 'k8s-node';
    case 'k8s-cluster':
      return 'k8s-cluster';
    case 'k8s-deployment':
      return 'k8s-deployment';
    case 'vm':
      return 'vm';
    case 'system-container':
      return 'system-container';
    case 'oci-container':
      return 'oci-container';
    case 'app-container':
      return 'app-container';
    case 'pod':
      return 'pod';
    default:
      return null;
  }
}

export function canonicalizeMetricsHistoryTargetType(
  metricsType: string,
  unifiedType?: string,
): ResourceType | null {
  const normalized = metricsType.trim().toLowerCase();
  if (normalized === 'k8s') {
    switch (unifiedType) {
      case 'k8s-cluster':
        return 'k8s-cluster';
      case 'k8s-node':
        return 'k8s-node';
      case 'k8s-deployment':
        return 'k8s-deployment';
      case 'pod':
        return 'pod';
      default:
        return null;
    }
  }
  return asMetricsHistoryResourceType(normalized);
}

export interface SingleMetricHistoryResponse {
  resourceType: string;
  resourceId: string;
  metric: string;
  range: string;
  start: number; // Unix timestamp in milliseconds
  end: number; // Unix timestamp in milliseconds
  points: AggregatedMetricPoint[];
  source?: 'store' | 'memory' | 'live' | 'mock_synthetic';
}

export interface AllMetricsHistoryResponse {
  resourceType: string;
  resourceId: string;
  range: string;
  start: number; // Unix timestamp in milliseconds
  end: number; // Unix timestamp in milliseconds
  metrics: Record<string, AggregatedMetricPoint[]>;
  source?: 'store' | 'memory' | 'live' | 'mock_synthetic';
}

const isHistoryObject = (value: unknown): value is Record<string, unknown> =>
  typeof value === 'object' && value !== null && !Array.isArray(value);

const isFiniteHistoryNumber = (value: unknown): value is number =>
  typeof value === 'number' && Number.isFinite(value);

const isHistoryPointList = (value: unknown): value is AggregatedMetricPoint[] =>
  Array.isArray(value) &&
  value.every(
    (point: unknown) =>
      isHistoryObject(point) &&
      isFiniteHistoryNumber(point.timestamp) &&
      Number.isFinite(new Date(point.timestamp).getTime()) &&
      isFiniteHistoryNumber(point.value) &&
      isFiniteHistoryNumber(point.min) &&
      isFiniteHistoryNumber(point.max),
  );

// A successful transport is not evidence for the requested selection. Validate
// before any drawer, chart or hover cache can accept the response. Keep errors
// fixed: a mismatched body can contain another resource's private details.
function parseMetricsHistoryResponse(
  value: unknown,
  params: MetricsHistoryParams,
): SingleMetricHistoryResponse | AllMetricsHistoryResponse {
  const resourceType = toMetricsHistoryAPIResourceType(params.resourceType);
  const requestedId = params.resourceId.trim();
  // This is the server's one explicit ID compatibility rule, not an alias
  // search: legacy Kubernetes pod IDs receive the canonical k8s: prefix.
  const resourceId =
    resourceType === 'k8s' && requestedId.includes(':pod:') && !requestedId.startsWith('k8s:')
      ? `k8s:${requestedId}`
      : requestedId;
  if (
    !isHistoryObject(value) ||
    value.resourceType !== resourceType ||
    value.resourceId !== resourceId ||
    (params.range ? value.range !== params.range : value.range !== '' && value.range !== '24h') ||
    !isFiniteHistoryNumber(value.start) ||
    !isFiniteHistoryNumber(value.end) ||
    (value.source !== undefined &&
      !['store', 'memory', 'live', 'mock_synthetic'].includes(value.source as string))
  ) {
    throw new Error('Invalid metrics history response.');
  }
  if (params.metric) {
    if (value.metric !== params.metric || 'metrics' in value || !isHistoryPointList(value.points)) {
      throw new Error('Invalid metrics history response.');
    }
    return value as unknown as SingleMetricHistoryResponse;
  }
  if (
    'metric' in value ||
    'points' in value ||
    !isHistoryObject(value.metrics) ||
    !Object.values(value.metrics).every(isHistoryPointList)
  ) {
    throw new Error('Invalid metrics history response.');
  }
  return value as unknown as AllMetricsHistoryResponse;
}

export type TimeRange = '5m' | '15m' | '30m' | '1h' | '4h' | '12h' | '24h' | '7d' | '30d';

export class ChartsAPI {
  private static baseUrl = '/api';

  private static buildChartsUrl(
    path: string,
    params: {
      range: TimeRange;
      nodeId?: string | null;
      metrics?: readonly InfrastructureSummaryMetric[] | null;
    },
  ): string {
    const searchParams = new URLSearchParams({ range: params.range });
    if (params.nodeId) {
      searchParams.set('node', params.nodeId);
    }
    if (Array.isArray(params.metrics) && params.metrics.length > 0) {
      searchParams.set('metrics', Array.from(new Set(params.metrics)).join(','));
    }
    return `${this.baseUrl}${path}?${searchParams.toString()}`;
  }

  /**
   * Fetch historical chart data for all resources
   * @param range Time range to fetch (default: 1h)
   */
  static async getCharts(
    range: TimeRange = '1h',
    signal?: AbortSignal,
    options?: { nodeId?: string | null },
  ): Promise<ChartsResponse> {
    const url = this.buildChartsUrl('/charts', { range, nodeId: options?.nodeId });
    return apiFetchJSON(url, { signal });
  }

  /**
   * Fetch infrastructure-only chart data for summary sparklines.
   * This avoids guest/container/storage chart payloads.
   */
  static async getInfrastructureSummaryCharts(
    range: TimeRange = '1h',
    signal?: AbortSignal,
    options?: { nodeId?: string | null; metrics?: readonly InfrastructureSummaryMetric[] | null },
  ): Promise<InfrastructureChartsResponse> {
    const url = this.buildChartsUrl('/charts/infrastructure', {
      range,
      nodeId: options?.nodeId,
      metrics: options?.metrics,
    });
    return apiFetchJSON(url, { signal });
  }

  /**
   * Fetch workload-only chart data for the workload table's Trends sparklines.
   * Excludes infrastructure/storage series to keep payloads bounded at scale.
   */
  static async getWorkloadCharts(
    range: TimeRange = '1h',
    signal?: AbortSignal,
    options?: { nodeId?: string | null; maxPoints?: number | null },
  ): Promise<WorkloadChartsResponse> {
    let url = this.buildChartsUrl('/charts/workloads', {
      range,
      nodeId: options?.nodeId,
    });
    if (
      typeof options?.maxPoints === 'number' &&
      Number.isFinite(options.maxPoints) &&
      options.maxPoints > 0
    ) {
      const separator = url.includes('?') ? '&' : '?';
      url = `${url}${separator}maxPoints=${encodeURIComponent(Math.round(options.maxPoints).toString())}`;
    }
    return apiFetchJSON(url, { signal });
  }

  /**
   * Fetch persistent metrics history for a specific resource
   * This uses the SQLite-backed store with longer retention (up to 90 days)
   * @param params Query parameters
   */
  static async getMetricsHistory(
    params: MetricsHistoryParams,
  ): Promise<SingleMetricHistoryResponse | AllMetricsHistoryResponse> {
    const selection = { ...params };
    const searchParams = new URLSearchParams({
      resourceType: toMetricsHistoryAPIResourceType(selection.resourceType),
      resourceId: selection.resourceId,
    });
    if (selection.metric) {
      searchParams.set('metric', selection.metric);
    }
    if (selection.range) {
      searchParams.set('range', selection.range);
    }
    if (
      typeof selection.maxPoints === 'number' &&
      Number.isFinite(selection.maxPoints) &&
      selection.maxPoints > 0
    ) {
      searchParams.set('maxPoints', Math.round(selection.maxPoints).toString());
    }
    const url = `${this.baseUrl}/metrics-store/history?${searchParams.toString()}`;
    const response: unknown = selection.signal
      ? await apiFetchJSON(url, { signal: selection.signal })
      : await apiFetchJSON(url);
    return parseMetricsHistoryResponse(response, selection);
  }

  /**
   * Fetch storage summary chart data (pool capacity + disk temperature).
   */
  static async getStorageSummaryCharts(
    range_: TimeRange = '1h',
    signal?: AbortSignal,
    options?: { nodeId?: string },
  ): Promise<StorageSummaryChartsResponse> {
    const rangeMinutes = timeRangeToMinutes(range_);
    const params = new URLSearchParams({ range: String(rangeMinutes) });
    if (options?.nodeId) {
      params.set('node', options.nodeId);
    }
    const url = `${this.baseUrl}/storage-charts?${params.toString()}`;
    return apiFetchJSON(url, { signal });
  }
}

// ---------------------------------------------------------------------------
// Storage summary chart types
// ---------------------------------------------------------------------------

export interface StoragePoolChartData {
  name: string;
  usage: MetricPoint[];
  used: MetricPoint[];
  avail: MetricPoint[];
}

export interface StorageDiskChartData {
  name: string;
  node: string;
  temperature: MetricPoint[];
}

export interface StorageSummaryChartsResponse {
  pools: Record<string, StoragePoolChartData>;
  disks: Record<string, StorageDiskChartData>;
  stats: ChartStats;
}

function timeRangeToMinutes(range_: TimeRange): number {
  switch (range_) {
    case '5m':
      return 5;
    case '15m':
      return 15;
    case '30m':
      return 30;
    case '1h':
      return 60;
    case '4h':
      return 240;
    case '12h':
      return 720;
    case '24h':
      return 1440;
    case '7d':
      return 10080;
    case '30d':
      return 43200;
    default:
      return 60;
  }
}
