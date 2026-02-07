/**
 * Charts API
 *
 * Fetches historical metrics data from the backend for sparkline visualizations.
 * The backend maintains proper historical data with 30s sample intervals.
 */

import { apiFetchJSON } from '@/utils/apiClient';

// Types matching backend response format
export interface MetricPoint {
    timestamp: number;  // Unix timestamp in milliseconds
    value: number;
}

// Extended metric point with min/max for aggregated data
export interface AggregatedMetricPoint {
    timestamp: number;  // Unix timestamp in milliseconds
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
        hosts?: number;
    };
}

export interface ChartsResponse {
    data: Record<string, ChartData>;       // VM/Container data keyed by ID
    nodeData: Record<string, ChartData>;   // Node data keyed by ID
    storageData: Record<string, ChartData>; // Storage data keyed by ID
    dockerData?: Record<string, ChartData>; // Docker container data keyed by container ID
    dockerHostData?: Record<string, ChartData>; // Docker host data keyed by host ID
    hostData?: Record<string, ChartData>; // Unified host agent data keyed by host ID
    guestTypes?: Record<string, 'vm' | 'container' | 'k8s'>; // Maps guest ID to type
    timestamp: number;
    stats: ChartStats;
}

export interface InfrastructureChartsResponse {
    nodeData: Record<string, ChartData>;
    dockerHostData?: Record<string, ChartData>;
    hostData?: Record<string, ChartData>;
    timestamp: number;
    stats: ChartStats;
}

export interface WorkloadChartsResponse {
    data: Record<string, ChartData>;
    dockerData?: Record<string, ChartData>;
    guestTypes?: Record<string, 'vm' | 'container' | 'k8s'>;
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
export type ResourceType = 'node' | 'guest' | 'vm' | 'container' | 'storage' | 'docker' | 'dockerHost' | 'k8s' | 'host' | 'disk';

export interface MetricsHistoryParams {
    resourceType: ResourceType;
    resourceId: string;
    metric?: string;  // Optional: 'cpu', 'memory', 'disk', etc. Omit for all metrics
    range?: HistoryTimeRange;  // Default: '24h'
    maxPoints?: number; // Optional cap on returned points (backend may downsample)
}

export interface SingleMetricHistoryResponse {
    resourceType: string;
    resourceId: string;
    metric: string;
    range: string;
    start: number;  // Unix timestamp in milliseconds
    end: number;    // Unix timestamp in milliseconds
    points: AggregatedMetricPoint[];
    source?: 'store' | 'memory' | 'live';
}

export interface AllMetricsHistoryResponse {
    resourceType: string;
    resourceId: string;
    range: string;
    start: number;  // Unix timestamp in milliseconds
    end: number;    // Unix timestamp in milliseconds
    metrics: Record<string, AggregatedMetricPoint[]>;
    source?: 'store' | 'memory' | 'live';
}

export type TimeRange = '5m' | '15m' | '30m' | '1h' | '4h' | '12h' | '24h' | '7d' | '30d';

export class ChartsAPI {
    private static baseUrl = '/api';

    private static buildChartsUrl(path: string, params: { range: TimeRange; nodeId?: string | null }): string {
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
        const url = this.buildChartsUrl('/charts/infrastructure-summary', {
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
        if (typeof options?.maxPoints === 'number' && Number.isFinite(options.maxPoints) && options.maxPoints > 0) {
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
    static async getMetricsHistory(params: MetricsHistoryParams): Promise<SingleMetricHistoryResponse | AllMetricsHistoryResponse> {
        const searchParams = new URLSearchParams({
            resourceType: params.resourceType,
            resourceId: params.resourceId,
        });
        if (params.metric) {
            searchParams.set('metric', params.metric);
        }
        if (params.range) {
            searchParams.set('range', params.range);
        }
        if (typeof params.maxPoints === 'number' && Number.isFinite(params.maxPoints) && params.maxPoints > 0) {
            searchParams.set('maxPoints', Math.round(params.maxPoints).toString());
        }
        const url = `${this.baseUrl}/metrics-store/history?${searchParams.toString()}`;
        return apiFetchJSON(url);
    }

}
