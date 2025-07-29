import type { State, Performance, Stats } from '@/types/api';

export class MonitoringAPI {
  private static baseUrl = '/api';

  static async getState(): Promise<State> {
    const response = await fetch(`${this.baseUrl}/state`);
    if (!response.ok) {
      throw new Error('Failed to fetch monitoring state');
    }
    return response.json();
  }

  static async getPerformance(): Promise<Performance> {
    const response = await fetch(`${this.baseUrl}/performance`);
    if (!response.ok) {
      throw new Error('Failed to fetch performance metrics');
    }
    return response.json();
  }

  static async getStats(): Promise<Stats> {
    const response = await fetch(`${this.baseUrl}/stats`);
    if (!response.ok) {
      throw new Error('Failed to fetch system stats');
    }
    return response.json();
  }

  static async getChartData(params: {
    id: string;
    type: 'guest' | 'node' | 'storage';
    metric: 'cpu' | 'memory' | 'disk' | 'network';
    timeRange: '1h' | '6h' | '24h' | '7d' | '30d';
  }): Promise<{
    timestamps: string[];
    values: number[];
    unit: string;
  }> {
    const queryParams = new URLSearchParams(params);
    const response = await fetch(`${this.baseUrl}/charts?${queryParams}`);
    
    if (!response.ok) {
      throw new Error('Failed to fetch chart data');
    }
    
    return response.json();
  }

  static async exportDiagnostics(): Promise<Blob> {
    const response = await fetch(`${this.baseUrl}/diagnostics/export`);
    if (!response.ok) {
      throw new Error('Failed to export diagnostics');
    }
    return response.blob();
  }
}