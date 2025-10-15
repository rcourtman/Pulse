import type { State, Performance, Stats, DockerHostCommand } from '@/types/api';
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

  static async deleteDockerHost(
    hostId: string,
    options: { hide?: boolean; force?: boolean } = {}
  ): Promise<DeleteDockerHostResponse> {
    const params = new URLSearchParams();
    if (options.hide) params.set('hide', 'true');
    if (options.force) params.set('force', 'true');
    const query = params.toString();
    const url = `${this.baseUrl}/agents/docker/hosts/${encodeURIComponent(hostId)}${query ? `?${query}` : ''}`;

    const response = await apiFetch(url, {
      method: 'DELETE',
    });

    if (!response.ok) {
      if (response.status === 404) {
        return {};
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

    if (response.status === 204) {
      return {};
    }

    const text = await response.text();
    if (!text?.trim()) {
      return {};
    }

    try {
      const parsed = JSON.parse(text) as DeleteDockerHostResponse;
      return parsed;
    } catch (err) {
      throw new Error((err as Error).message || 'Failed to parse delete docker host response');
    }
  }

  static async unhideDockerHost(hostId: string): Promise<void> {
    const url = `${this.baseUrl}/agents/docker/hosts/${encodeURIComponent(hostId)}/unhide`;

    const response = await apiFetch(url, {
      method: 'PUT',
    });

    if (!response.ok) {
      if (response.status === 404) {
        // Host already gone; treat as success
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

  static async markDockerHostPendingUninstall(hostId: string): Promise<void> {
    const url = `${this.baseUrl}/agents/docker/hosts/${encodeURIComponent(hostId)}/pending-uninstall`;

    const response = await apiFetch(url, {
      method: 'PUT',
    });

    if (!response.ok) {
      if (response.status === 404) {
        // Host already gone; treat as success
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

export interface DeleteDockerHostResponse {
  success?: boolean;
  hostId?: string;
  message?: string;
  command?: DockerHostCommand;
}
