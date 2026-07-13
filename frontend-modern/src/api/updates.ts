import { apiFetchJSON } from '@/utils/apiClient';
import type { UpdateChannel } from '@/types/config';

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
  dockerUpdate?: DockerUpdateCommands;
}

// Digest-pinned Docker update commands from the license server download
// broker. Only present for Docker deployments of the Pro runtime, which must
// never be pointed at the community rcourtman/pulse image.
export interface DockerUpdateCommands {
  version: string;
  image: string;
  imageDigest: string;
  loginCommand?: string;
  composePullCommand: string;
  composeUpCommand: string;
}

export interface ReleaseNotesInfo {
  version: string;
  releaseNotes: string;
  releaseDate: string;
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
  channel?: UpdateChannel;
  isDocker: boolean;
  isSourceBuild: boolean;
  isDevelopment: boolean;
  deploymentType?: string;
  agentUpdateTargetVersion?: string;
}

export type UpdateReadinessStatus = 'ready' | 'attention' | 'blocked';
export type UpdateReadinessCheckStatus = 'pass' | 'warning' | 'blocked';

export interface UpdateReadinessCheck {
  id: string;
  status: UpdateReadinessCheckStatus;
  title: string;
  summary: string;
  details?: string[];
}

export interface UpdateReadiness {
  status: UpdateReadinessStatus;
  summary: string;
  checks: UpdateReadinessCheck[];
}

export interface UpdatePlan {
  canAutoUpdate: boolean;
  instructions?: string[];
  prerequisites?: string[];
  estimatedTime?: string;
  requiresRoot: boolean;
  rollbackSupport: boolean;
  downloadUrl?: string;
  readiness?: UpdateReadiness;
}

export interface UpdateHistoryEntryError {
  message: string;
  code?: string;
  details?: string;
}

export interface UpdateHistoryEntry {
  event_id: string;
  timestamp: string;
  action: string;
  channel: string;
  version_from: string;
  version_to: string;
  deployment_type: string;
  initiated_by: string;
  initiated_via: string;
  status: string;
  duration_ms: number;
  backup_path?: string;
  log_path?: string;
  error?: UpdateHistoryEntryError;
  download_bytes?: number;
  related_event_id?: string;
  notes?: string;
}

const requireNonEmpty = (value: string, fieldName: string): string => {
  const trimmed = value.trim();
  if (!trimmed) {
    throw new Error(`${fieldName} is required`);
  }
  return trimmed;
};

export class UpdatesAPI {
  static async checkForUpdates(channel?: UpdateChannel): Promise<UpdateInfo> {
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

  static async rollbackUpdate(eventId: string): Promise<{ status: string; message: string }> {
    const normalizedEventId = requireNonEmpty(eventId, 'Event ID');
    return apiFetchJSON('/api/updates/rollback', {
      method: 'POST',
      body: JSON.stringify({ eventId: normalizedEventId }),
    });
  }

  static async listUpdateHistory(limit = 20): Promise<UpdateHistoryEntry[]> {
    const search = new URLSearchParams({ limit: String(limit) });
    return apiFetchJSON(`/api/updates/history?${search.toString()}`);
  }

  static async getUpdateStatus(): Promise<UpdateStatus> {
    return apiFetchJSON('/api/updates/status');
  }

  static async getVersion(): Promise<VersionInfo> {
    return apiFetchJSON('/api/version');
  }

  static async getReleaseNotes(): Promise<ReleaseNotesInfo> {
    return apiFetchJSON('/api/updates/release-notes');
  }

  static async getUpdatePlan(version: string, channel?: UpdateChannel): Promise<UpdatePlan> {
    const validatedVersion = requireNonEmpty(version, 'Version');
    const search = new URLSearchParams({ version: validatedVersion });
    if (channel) {
      search.set('channel', channel);
    }
    const url = `/api/updates/plan?${search.toString()}`;
    return apiFetchJSON(url);
  }
}
