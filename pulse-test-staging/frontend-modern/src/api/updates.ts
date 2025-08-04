// Remove apiRequest import - use fetch directly

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
}

export class UpdatesAPI {
  static async checkForUpdates(): Promise<UpdateInfo> {
    const response = await fetch('/api/updates/check');
    if (!response.ok) {
      throw new Error('Failed to check for updates');
    }
    return response.json();
  }

  static async applyUpdate(downloadUrl: string): Promise<{ status: string; message: string }> {
    const response = await fetch('/api/updates/apply', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ downloadUrl }),
    });
    if (!response.ok) {
      throw new Error('Failed to apply update');
    }
    return response.json();
  }

  static async getUpdateStatus(): Promise<UpdateStatus> {
    const response = await fetch('/api/updates/status');
    if (!response.ok) {
      throw new Error('Failed to get update status');
    }
    return response.json();
  }

  static async getVersion(): Promise<VersionInfo> {
    const response = await fetch('/api/version');
    if (!response.ok) {
      throw new Error('Failed to get version');
    }
    return response.json();
  }
}