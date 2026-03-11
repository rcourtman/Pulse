/**
 * Charts API
 *
 * Fetches historical metrics data from the backend for sparkline visualizations.
 * The backend maintains proper historical data with 30s sample intervals.
 */

import { apiFetchJSON } from '@/utils/apiClient';

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

export interface WorkloadChartsResponse {
  data: Record<string, ChartData>;
  dockerData?: Record<string, ChartData>;
  guestTypes?: Record<string, 'vm' | 'system-container' | 'k8s'>;
  timestamp: number;
  stats: ChartStats;
}

export interface WorkloadsSummaryMetricData {
  p50: MetricPoint[];
  p95: MetricPoint[];
}

export interface WorkloadsGuestCounts {
  total: number;
  running: number;
  stopped: number;
}

export interface WorkloadsSummaryContributor {
  id: string;
  name: string;
  value: number;
}

export interface WorkloadsSummaryContributors {
  cpu: WorkloadsSummaryContributor[];
  memory: WorkloadsSummaryContributor[];
  disk: WorkloadsSummaryContributor[];
  network: WorkloadsSummaryContributor[];
}

export interface WorkloadsSummaryBlastRadius {
  scope: string;
  top3Share: number;
  activeWorkloads: number;
}

export interface WorkloadsSummaryBlastRadiusGroup {
  cpu: WorkloadsSummaryBlastRadius;
  memory: WorkloadsSummaryBlastRadius;
  disk: WorkloadsSummaryBlastRadius;
  network: WorkloadsSummaryBlastRadius;
}

export interface WorkloadsSummaryChartsResponse {
  cpu: WorkloadsSummaryMetricData;
  memory: WorkloadsSummaryMetricData;
  disk: WorkloadsSummaryMetricData;
  network: WorkloadsSummaryMetricData;
  guestCounts: WorkloadsGuestCounts;
  topContributors: WorkloadsSummaryContributors;
  blastRadius: WorkloadsSummaryBlastRadiusGroup;
  timestamp: number;
  stats: ChartStats;
}

// Persistent metrics history types (SQLite-backed, longer retention)
export type HistoryTimeRange = '1h' | '6h' | '12h' | '24h' | '7d' | '30d' | '90d';
type MetricsHistoryAPIResourceType =
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
  | 'agent'
  | 'vm'
  | 'system-container'
  | 'oci-container'
  | 'app-container'
  | 'storage'
  | 'docker-host'
  | 'k8s-cluster'
  | 'k8s-node'
  | 'pod'
  | 'agent'
  | 'disk';

export interface MetricsHistoryParams {
  resourceType: ResourceType;
  resourceId: string;
  metric?: string; // Optional: 'cpu', 'memory', 'disk', etc. Omit for all metrics
  range?: HistoryTimeRange; // Default: '24h'
  maxPoints?: number; // Optional cap on returned points (backend may downsample)
}

export function toMetricsHistoryAPIResourceType(
  resourceType: ResourceType,
): MetricsHistoryAPIResourceType {
  switch (resourceType) {
    case 'k8s-cluster':
    case 'k8s-node':
    case 'pod':
      return 'k8s';
    default:
      return resourceType;
  }
}

export function asMetricsHistoryResourceType(type: string): ResourceType | null {
  const normalizedType = type.trim().toLowerCase();
  const historyTypes: ResourceType[] = [
    'agent',
    'vm',
    'system-container',
    'oci-container',
    'app-container',
    'storage',
    'docker-host',
    'k8s-cluster',
    'k8s-node',
    'pod',
    'disk',
  ];
  return historyTypes.includes(normalizedType as ResourceType)
    ? (normalizedType as ResourceType)
    : null;
}

export function mapUnifiedTypeToHistoryResourceType(type: string): ResourceType | null {
  switch (type) {
    case 'agent':
      return 'agent';
    case 'docker-host':
      return 'docker-host';
    case 'k8s-node':
      return 'k8s-node';
    case 'k8s-cluster':
      return 'k8s-cluster';
    case 'truenas':
      return 'agent';
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
  if (normalized === 'node') {
    return 'agent';
  }
  if (normalized === 'k8s') {
    switch (unifiedType) {
      case 'k8s-cluster':
        return 'k8s-cluster';
      case 'k8s-node':
        return 'k8s-node';
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
  source?: 'store' | 'memory' | 'live';
}

export interface AllMetricsHistoryResponse {
  resourceType: string;
  resourceId: string;
  range: string;
  start: number; // Unix timestamp in milliseconds
  end: number; // Unix timestamp in milliseconds
  metrics: Record<string, AggregatedMetricPoint[]>;
  source?: 'store' | 'memory' | 'live';
}

export type TimeRange = '5m' | '15m' | '30m' | '1h' | '4h' | '12h' | '24h' | '7d' | '30d';

export class ChartsAPI {
  private static baseUrl = '/api';

  private static buildChartsUrl(
    path: string,
    params: { range: TimeRange; nodeId?: string | null },
  ): string {
    const searchParams = new URLSearchParams({ range: params.range });
    if (params.nodeId) {
      searchParams.set('node', params.nodeId);
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
    options?: { nodeId?: string | null },
  ): Promise<InfrastructureChartsResponse> {
    const url = this.buildChartsUrl('/charts/infrastructure', {
      range,
      nodeId: options?.nodeId,
    });
    return apiFetchJSON(url, { signal });
  }

  /**
   * Fetch workloads aggregate chart data for Workloads top-card sparklines.
   * Returns compact p50/p95 series, not per-workload lines.
   */
  static async getWorkloadsSummaryCharts(
    range: TimeRange = '1h',
    signal?: AbortSignal,
    options?: { nodeId?: string | null },
  ): Promise<WorkloadsSummaryChartsResponse> {
    const url = this.buildChartsUrl('/charts/workloads-summary', {
      range,
      nodeId: options?.nodeId,
    });
    return apiFetchJSON(url, { signal });
  }

  /**
   * Fetch workload-only chart data used by WorkloadsSummary sparklines.
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
   * @deprecated Use getInfrastructureSummaryCharts.
   */
  static async getInfrastructureCharts(
    range: TimeRange = '1h',
    signal?: AbortSignal,
    options?: { nodeId?: string | null },
  ): Promise<InfrastructureChartsResponse> {
    return this.getInfrastructureSummaryCharts(range, signal, options);
  }

  /**
   * Fetch persistent metrics history for a specific resource
   * This uses the SQLite-backed store with longer retention (up to 90 days)
   * @param params Query parameters
   */
  static async getMetricsHistory(
    params: MetricsHistoryParams,
  ): Promise<SingleMetricHistoryResponse | AllMetricsHistoryResponse> {
    const searchParams = new URLSearchParams({
      resourceType: toMetricsHistoryAPIResourceType(params.resourceType),
      resourceId: params.resourceId,
    });
    if (params.metric) {
      searchParams.set('metric', params.metric);
    }
    if (params.range) {
      searchParams.set('range', params.range);
    }
    if (
      typeof params.maxPoints === 'number' &&
      Number.isFinite(params.maxPoints) &&
      params.maxPoints > 0
    ) {
      searchParams.set('maxPoints', Math.round(params.maxPoints).toString());
    }
    const url = `${this.baseUrl}/metrics-store/history?${searchParams.toString()}`;
    return apiFetchJSON(url);
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
