import type { State, Performance, Stats, DockerHostCommand, HostLookupResponse } from '@/types/api';
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

  static async setDockerHostDisplayName(hostId: string, displayName: string): Promise<void> {
    const url = `${this.baseUrl}/agents/docker/hosts/${encodeURIComponent(hostId)}/display-name`;

    const response = await apiFetch(url, {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ displayName }),
    });

    if (!response.ok) {
      if (response.status === 404) {
        throw new Error('Docker host not found');
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

  static async allowDockerHostReenroll(hostId: string): Promise<void> {
    const url = `${this.baseUrl}/agents/docker/hosts/${encodeURIComponent(hostId)}/allow-reenroll`;

    const response = await apiFetch(url, {
      method: 'POST',
    });

    if (!response.ok) {
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
          } catch (_err) {
            // ignore parse error, use raw text
          }
        }
      } catch (_err) {
        // ignore read error
      }

      throw new Error(message);
    }
  }

  static async deleteKubernetesCluster(clusterId: string): Promise<DeleteKubernetesClusterResponse> {
    const url = `${this.baseUrl}/agents/kubernetes/clusters/${encodeURIComponent(clusterId)}`;

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
            // ignore parse errors
          }
        }
      } catch (_err) {
        // ignore read error
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
      return JSON.parse(text) as DeleteKubernetesClusterResponse;
    } catch (err) {
      throw new Error((err as Error).message || 'Failed to parse delete kubernetes cluster response');
    }
  }

  static async unhideKubernetesCluster(clusterId: string): Promise<void> {
    const url = `${this.baseUrl}/agents/kubernetes/clusters/${encodeURIComponent(clusterId)}/unhide`;

    const response = await apiFetch(url, {
      method: 'PUT',
    });

    if (!response.ok) {
      if (response.status === 404) {
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
            // ignore parse errors
          }
        }
      } catch (_err) {
        // ignore read error
      }

      throw new Error(message);
    }
  }

  static async markKubernetesClusterPendingUninstall(clusterId: string): Promise<void> {
    const url = `${this.baseUrl}/agents/kubernetes/clusters/${encodeURIComponent(clusterId)}/pending-uninstall`;

    const response = await apiFetch(url, {
      method: 'PUT',
    });

    if (!response.ok) {
      if (response.status === 404) {
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
            // ignore parse errors
          }
        }
      } catch (_err) {
        // ignore read error
      }

      throw new Error(message);
    }
  }

  static async setKubernetesClusterDisplayName(clusterId: string, displayName: string): Promise<void> {
    const url = `${this.baseUrl}/agents/kubernetes/clusters/${encodeURIComponent(clusterId)}/display-name`;

    const response = await apiFetch(url, {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ displayName }),
    });

    if (!response.ok) {
      if (response.status === 404) {
        throw new Error('Kubernetes cluster not found');
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
            // ignore parse errors
          }
        }
      } catch (_err) {
        // ignore read error
      }

      throw new Error(message);
    }
  }

  static async allowKubernetesClusterReenroll(clusterId: string): Promise<void> {
    const url = `${this.baseUrl}/agents/kubernetes/clusters/${encodeURIComponent(clusterId)}/allow-reenroll`;

    const response = await apiFetch(url, {
      method: 'POST',
    });

    if (!response.ok) {
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
          } catch (_err) {
            // ignore parse error
          }
        }
      } catch (_err) {
        // ignore read error
      }

      throw new Error(message);
    }
  }

  static async deleteHostAgent(hostId: string): Promise<void> {
    if (!hostId) {
      throw new Error('Host ID is required to remove a host agent.');
    }

    const url = `${this.baseUrl}/agents/host/${encodeURIComponent(hostId)}`;
    const response = await apiFetch(url, { method: 'DELETE' });

    if (!response.ok) {
      let message = `Failed with status ${response.status}`;
      try {
        const text = await response.text();
        if (text?.trim()) {
          message = text.trim();
          try {
            const parsed = JSON.parse(text);
            if (typeof parsed?.error === 'string' && parsed.error.trim()) {
              message = parsed.error.trim();
            } else if (typeof parsed?.message === 'string' && parsed.message.trim()) {
              message = parsed.message.trim();
            }
          } catch (_err) {
            // Ignore JSON parse errors, fallback to raw text.
          }
        }
      } catch (_err) {
        // Ignore body read errors, keep default message.
      }

      throw new Error(message);
    }

    // Consume and ignore the body so the fetch can be reused by the connection pool.
    try {
      await response.text();
    } catch (_err) {
      // Swallow body read errors; the deletion already succeeded.
    }
  }

  static async updateHostAgentConfig(hostId: string, config: { commandsEnabled?: boolean }): Promise<void> {
    if (!hostId) {
      throw new Error('Host ID is required to update agent config.');
    }

    const url = `${this.baseUrl}/agents/host/${encodeURIComponent(hostId)}/config`;
    const response = await apiFetch(url, {
      method: 'PATCH',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(config),
    });

    if (!response.ok) {
      let message = `Failed with status ${response.status}`;
      try {
        const text = await response.text();
        if (text?.trim()) {
          message = text.trim();
          try {
            const parsed = JSON.parse(text);
            if (typeof parsed?.error === 'string' && parsed.error.trim()) {
              message = parsed.error.trim();
            } else if (typeof parsed?.message === 'string' && parsed.message.trim()) {
              message = parsed.message.trim();
            }
          } catch (_err) {
            // Ignore JSON parse errors.
          }
        }
      } catch (_err) {
        // Ignore body read errors.
      }

      throw new Error(message);
    }
  }

  static async unlinkHostAgent(hostId: string): Promise<void> {
    if (!hostId) {
      throw new Error('Host ID is required to unlink an agent.');
    }

    const url = `${this.baseUrl}/agents/host/unlink`;
    const response = await apiFetch(url, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ hostId }),
    });

    if (!response.ok) {
      let message = `Failed with status ${response.status}`;
      try {
        const text = await response.text();
        if (text?.trim()) {
          message = text.trim();
          try {
            const parsed = JSON.parse(text);
            if (typeof parsed?.error === 'string' && parsed.error.trim()) {
              message = parsed.error.trim();
            } else if (typeof parsed?.message === 'string' && parsed.message.trim()) {
              message = parsed.message.trim();
            }
          } catch (_err) {
            // Ignore JSON parse errors.
          }
        }
      } catch (_err) {
        // Ignore body read errors.
      }

      throw new Error(message);
    }
  }

  static async lookupHost(params: { id?: string; hostname?: string }): Promise<HostLookupResponse | null> {
    const search = new URLSearchParams();
    if (params.id) search.set('id', params.id);
    if (params.hostname) search.set('hostname', params.hostname);

    if (!search.toString()) {
      throw new Error('Provide a host identifier or hostname to look up.');
    }

    const url = `${this.baseUrl}/agents/host/lookup?${search.toString()}`;
    const response = await apiFetch(url);

    if (response.status === 404) {
      return null;
    }

    if (!response.ok) {
      const text = await response.text();
      let message = text?.trim() || `Lookup failed with status ${response.status}`;
      try {
        const parsed = text ? JSON.parse(text) : null;
        if (parsed?.error) {
          message = parsed.error;
        }
      } catch (_err) {
        // ignore parse error
      }
      throw new Error(message);
    }

    const text = await response.text();
    if (!text?.trim()) {
      return null;
    }

    const data = JSON.parse(text) as HostLookupResponse;
    const lastSeen = data?.host?.lastSeen as unknown;
    if (typeof lastSeen === 'string') {
      const parsed = Date.parse(lastSeen);
      data.host.lastSeen = Number.isFinite(parsed) ? parsed : Date.now();
    } else if (typeof lastSeen === 'number') {
      // assume already a timestamp
      data.host.lastSeen = lastSeen;
    } else {
      data.host.lastSeen = Date.now();
    }

    return data;
  }

  /**
   * Triggers an update for a Docker container on a specific host.
   * The update will pull the latest image and recreate the container.
   */
  static async updateDockerContainer(
    hostId: string,
    containerId: string,
    containerName: string
  ): Promise<UpdateDockerContainerResponse> {
    const url = `${this.baseUrl}/agents/docker/containers/update`;

    const response = await apiFetch(url, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ hostId, containerId, containerName }),
    });

    if (!response.ok) {
      let message = `Failed with status ${response.status}`;
      try {
        const text = await response.text();
        if (text?.trim()) {
          message = text.trim();
          try {
            const parsed = JSON.parse(text);
            if (typeof parsed?.error === 'string' && parsed.error.trim()) {
              message = parsed.error.trim();
            } else if (typeof parsed?.message === 'string' && parsed.message.trim()) {
              message = parsed.message.trim();
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

    const text = await response.text();
    if (!text?.trim()) {
      return { success: true };
    }

    try {
      return JSON.parse(text) as UpdateDockerContainerResponse;
    } catch (err) {
      throw new Error((err as Error).message || 'Failed to parse update container response');
    }
  }

  /**
   * Triggers an immediate update check for all containers on a specific Docker host.
   */
  static async checkDockerUpdates(hostId: string): Promise<{ success: boolean; commandId?: string }> {
    const url = `${this.baseUrl}/agents/docker/hosts/${encodeURIComponent(hostId)}/check-updates`;

    const response = await apiFetch(url, {
      method: 'POST',
    });

    if (!response.ok) {
      let message = `Failed with status ${response.status}`;
      try {
        const text = await response.text();
        if (text?.trim()) {
          try {
            const parsed = JSON.parse(text);
            if (typeof parsed?.error === 'string' && parsed.error.trim()) {
              message = parsed.error.trim();
            }
          } catch (_jsonErr) {
            message = text.trim();
          }
        }
      } catch (_err) {
        // ignore read error
      }
      throw new Error(message);
    }

    const text = await response.text();
    if (!text?.trim()) {
      return { success: true };
    }

    try {
      return JSON.parse(text) as { success: boolean; commandId?: string };
    } catch (err) {
      throw new Error((err as Error).message || 'Failed to parse check updates response');
    }
  }
}


export interface DeleteDockerHostResponse {
  success?: boolean;
  hostId?: string;
  message?: string;
  command?: DockerHostCommand;
}

export interface DeleteKubernetesClusterResponse {
  success?: boolean;
  clusterId?: string;
  message?: string;
}

export interface UpdateDockerContainerResponse {
  success?: boolean;
  commandId?: string;
  hostId?: string;
  container?: {
    id: string;
    name: string;
  };
  message?: string;
  note?: string;
}

