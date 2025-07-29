import type { ChartData, NodeChartData, StorageChartData, ChartStats } from '@/types/charts';

export interface ChartsResponse {
  data: ChartData;
  nodeData: NodeChartData;
  storageData: StorageChartData;
  timestamp: number;
  stats: ChartStats;
}

export class ChartsAPI {
  private static baseUrl = '/api/charts';

  static async getCharts(timeRange: string = '1h'): Promise<ChartsResponse> {
    const response = await fetch(`${this.baseUrl}?range=${timeRange}`);
    if (!response.ok) {
      throw new Error('Failed to fetch chart data');
    }
    return response.json();
  }

  static async getStorageCharts(rangeMinutes: number = 60): Promise<Record<string, {
    usage: Array<{timestamp: number; value: number}>;
    used: Array<{timestamp: number; value: number}>;
    total: Array<{timestamp: number; value: number}>;
    avail: Array<{timestamp: number; value: number}>;
  }>> {
    const response = await fetch(`/api/storage-charts?range=${rangeMinutes}`);
    if (!response.ok) {
      throw new Error('Failed to fetch storage chart data');
    }
    return response.json();
  }
}