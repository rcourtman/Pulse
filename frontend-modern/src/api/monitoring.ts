import type { State, Performance, Stats } from '@/types/api';
import { apiFetch, apiFetchJSON } from '@/utils/apiClient';

export class MonitoringAPI {
  private static baseUrl = '/api';

  static async getState(): Promise<State> {
    return apiFetchJSON(`${this.baseUrl}/state`);
  }

  static async getPerformance(): Promise<Performance> {
    return apiFetchJSON(`${this.baseUrl}/performance`);
  }

  static async getStats(): Promise<Stats> {
    return apiFetchJSON(`${this.baseUrl}/stats`);
  }

  static async exportDiagnostics(): Promise<Blob> {
    const response = await apiFetch(`${this.baseUrl}/diagnostics/export`);
    return response.blob();
  }

  static async deleteDockerHost(hostId: string): Promise<void> {
    const response = await apiFetch(
      `${this.baseUrl}/agents/docker/hosts/${encodeURIComponent(hostId)}`,
      {
        method: 'DELETE',
      },
    );

    if (!response.ok) {
      if (response.status === 404) {
        // Host already gone; treat as success so UI state stays consistent
        return;
      }

      let message = `Failed with status ${response.status}`;
      try {
        const text = await response.text();
        if (text?.trim()) {
          message = text.trim();
          try {
            const parsed = JSON.parse(text);
            if (typeof parsed?.error === 'string' && parsed.error.trim()) {
              message = parsed.error.trim();
            }
          } catch (_jsonErr) {
            // ignore JSON parse errors, fallback to raw text
          }
        }
      } catch (_err) {
        // ignore read error, keep default message
      }

      throw new Error(message);
    }
  }
}
