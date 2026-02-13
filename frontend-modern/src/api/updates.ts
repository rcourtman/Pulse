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

const normalizeOptionalParam = (value?: string): string | undefined => {
  const trimmed = value?.trim();
  return trimmed ? trimmed : undefined;
};

const requireNonEmpty = (value: string, fieldName: string): string => {
  const trimmed = value.trim();
  if (!trimmed) {
    throw new Error(`${fieldName} is required`);
  }
  return trimmed;
};

const withQueryParams = (path: string, params: Record<string, string | undefined>): string => {
  const query = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined) {
      query.set(key, value);
    }
  }
  const queryString = query.toString();
  return queryString ? `${path}?${queryString}` : path;
};

export class UpdatesAPI {
  static async checkForUpdates(channel?: string): Promise<UpdateInfo> {
    const search = new URLSearchParams();
    if (channel) {
      search.set('channel', channel);
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
    const search = new URLSearchParams({ version });
    if (channel) {
      search.set('channel', channel);
    }
    const url = `/api/updates/plan?${search.toString()}`;
    return apiFetchJSON(url);
  }
}
