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
  warning?: string;
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
  isSourceBuild: boolean;
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
}
