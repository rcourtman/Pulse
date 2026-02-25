import { apiFetchJSON } from '@/utils/apiClient';

export interface UpdateInfo {
  available: boolean;
  currentVersion: string;
  latestVersion: string;
  releaseNotes: string;
  releaseDate: string;
  downloadUrl: string;
  isPrerelease: boolean;
  isMajorUpgrade: boolean;
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

const requireNonEmpty = (value: string, fieldName: string): string => {
  const trimmed = value.trim();
  if (!trimmed) {
    throw new Error(`${fieldName} is required`);
  }
  return trimmed;
};

export class UpdatesAPI {
  static async checkForUpdates(channel?: string): Promise<UpdateInfo> {
    const search = new URLSearchParams();
    const trimmedChannel = channel?.trim();
    if (trimmedChannel) {
      search.set('channel', trimmedChannel);
    }
    const query = search.toString();
    const url = query ? `/api/updates/check?${query}` : '/api/updates/check';
    return apiFetchJSON(url);
  }

  static async applyUpdate(downloadUrl: string): Promise<{ status: string; message: string }> {
    const normalizedDownloadUrl = requireNonEmpty(downloadUrl, 'Download URL');
    return apiFetchJSON('/api/updates/apply', {
      method: 'POST',
      body: JSON.stringify({ downloadUrl: normalizedDownloadUrl }),
    });
  }

  static async getUpdateStatus(): Promise<UpdateStatus> {
    return apiFetchJSON('/api/updates/status');
  }

  static async getVersion(): Promise<VersionInfo> {
    return apiFetchJSON('/api/version');
  }

  static async getUpdatePlan(version: string, channel?: string): Promise<UpdatePlan> {
    const validatedVersion = requireNonEmpty(version, 'Version');
    const search = new URLSearchParams({ version: validatedVersion });
    if (channel) {
      search.set('channel', channel);
    }
    const url = `/api/updates/plan?${search.toString()}`;
    return apiFetchJSON(url);
  }
}
