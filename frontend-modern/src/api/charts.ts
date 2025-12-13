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
}

export interface ChartsResponse {
    data: Record<string, ChartData>;       // VM/Container data keyed by ID
    nodeData: Record<string, ChartData>;   // Node data keyed by ID
    storageData: Record<string, ChartData>; // Storage data keyed by ID
    dockerData?: Record<string, ChartData>; // Docker container data keyed by container ID
    dockerHostData?: Record<string, ChartData>; // Docker host data keyed by host ID
    guestTypes?: Record<string, 'vm' | 'container'>; // Maps guest ID to type
    timestamp: number;
    stats: ChartStats;
}

// Persistent metrics history types (SQLite-backed, longer retention)
export type HistoryTimeRange = '1h' | '6h' | '12h' | '24h' | '7d' | '30d' | '90d';
export type ResourceType = 'node' | 'guest' | 'storage' | 'docker' | 'dockerHost';

export interface MetricsHistoryParams {
    resourceType: ResourceType;
    resourceId: string;
    metric?: string;  // Optional: 'cpu', 'memory', 'disk', etc. Omit for all metrics
    range?: HistoryTimeRange;  // Default: '24h'
}

export interface SingleMetricHistoryResponse {
    resourceType: string;
    resourceId: string;
    metric: string;
    range: string;
    start: number;  // Unix timestamp in milliseconds
    end: number;    // Unix timestamp in milliseconds
    points: AggregatedMetricPoint[];
}

export interface AllMetricsHistoryResponse {
    resourceType: string;
    resourceId: string;
    range: string;
    start: number;  // Unix timestamp in milliseconds
    end: number;    // Unix timestamp in milliseconds
    metrics: Record<string, AggregatedMetricPoint[]>;
}

export interface MetricsStoreStats {
    enabled: boolean;
    dbPath?: string;
    dbSize?: number;
    rawCount?: number;
    minuteCount?: number;
    hourlyCount?: number;
    dailyCount?: number;
    totalWrites?: number;
    bufferSize?: number;
    lastFlush?: string;
    lastRollup?: string;
    lastRetention?: string;
    error?: string;
}

export type TimeRange = '5m' | '15m' | '30m' | '1h' | '4h' | '12h' | '24h' | '7d';

export class ChartsAPI {
    private static baseUrl = '/api';

    /**
     * Fetch historical chart data for all resources
     * @param range Time range to fetch (default: 1h)
     */
    static async getCharts(range: TimeRange = '1h'): Promise<ChartsResponse> {
        const url = `${this.baseUrl}/charts?range=${range}`;
        return apiFetchJSON(url);
    }

    /**
     * Fetch storage-specific chart data
     * @param rangeMinutes Range in minutes (default: 60)
     */
    static async getStorageCharts(rangeMinutes: number = 60): Promise<Record<string, {
        usage?: MetricPoint[];
        used?: MetricPoint[];
        total?: MetricPoint[];
        avail?: MetricPoint[];
    }>> {
        const url = `${this.baseUrl}/storage/charts?range=${rangeMinutes}`;
        return apiFetchJSON(url);
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
        const url = `${this.baseUrl}/metrics-store/history?${searchParams.toString()}`;
        return apiFetchJSON(url);
    }

    /**
     * Fetch statistics about the persistent metrics store
     */
    static async getMetricsStoreStats(): Promise<MetricsStoreStats> {
        const url = `${this.baseUrl}/metrics-store/stats`;
        return apiFetchJSON(url);
    }
}

