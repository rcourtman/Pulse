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
    timestamp: number;
    stats: ChartStats;
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
}
