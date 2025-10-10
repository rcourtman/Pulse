// Remove apiRequest import - use fetch directly
import { apiFetchJSON } from '@/utils/apiClient';

export interface UpdateInfo {
  available: boolean;
  currentVersion: string;
  latestVersion: string;
  releaseNotes: string;
  releaseDate: string;
  downloadUrl: string;
  isPrerelease: boolean;
}

export interface UpdateStatus {
  status: string;
  progress: number;
  message: string;
  error?: string;
  updatedAt: string;
}

export interface VersionInfo {
  version: string;
  build: string;
  runtime: string;
  channel?: string;
  isDocker: boolean;
  isDevelopment: boolean;
  deploymentType?: string;
}

export interface UpdatePlan {
  canAutoUpdate: boolean;
  instructions?: string[];
  prerequisites?: string[];
  estimatedTime?: string;
  requiresRoot: boolean;
  rollbackSupport: boolean;
  downloadUrl?: string;
}

export interface UpdateHistoryEntry {
  event_id: string;
  timestamp: string;
  action: 'update' | 'rollback';
  channel: string;
  version_from: string;
  version_to: string;
  deployment_type: string;
  initiated_by: 'user' | 'auto' | 'api';
  initiated_via: 'ui' | 'cli' | 'script' | 'webhook';
  status: 'in_progress' | 'success' | 'failed' | 'rolled_back' | 'cancelled';
  duration_ms: number;
  backup_path?: string;
  log_path?: string;
  error?: {
    message: string;
    code?: string;
    details?: string;
  };
  download_bytes?: number;
  related_event_id?: string;
  notes?: string;
}

export class UpdatesAPI {
  static async checkForUpdates(channel?: string): Promise<UpdateInfo> {
    const url = channel ? `/api/updates/check?channel=${channel}` : '/api/updates/check';
    return apiFetchJSON(url);
  }

  static async applyUpdate(downloadUrl: string): Promise<{ status: string; message: string }> {
    return apiFetchJSON('/api/updates/apply', {
      method: 'POST',
      body: JSON.stringify({ downloadUrl }),
    });
  }

  static async getUpdateStatus(): Promise<UpdateStatus> {
    return apiFetchJSON('/api/updates/status');
  }

  static async getVersion(): Promise<VersionInfo> {
    return apiFetchJSON('/api/version');
  }

  static async getUpdatePlan(version: string, channel?: string): Promise<UpdatePlan> {
    const url = channel
      ? `/api/updates/plan?version=${version}&channel=${channel}`
      : `/api/updates/plan?version=${version}`;
    return apiFetchJSON(url);
  }

  static async getUpdateHistory(
    limit?: number,
    status?: string
  ): Promise<UpdateHistoryEntry[]> {
    const params = new URLSearchParams();
    if (limit) params.append('limit', limit.toString());
    if (status) params.append('status', status);
    const url = `/api/updates/history${params.toString() ? `?${params.toString()}` : ''}`;
    return apiFetchJSON(url);
  }

  static async getUpdateHistoryEntry(eventId: string): Promise<UpdateHistoryEntry> {
    return apiFetchJSON(`/api/updates/history/entry?id=${eventId}`);
  }
}
