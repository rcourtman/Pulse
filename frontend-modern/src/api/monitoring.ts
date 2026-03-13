import type {
  State,
  Performance,
  Stats,
  DockerRuntimeCommand,
  AgentLookupResponse,
} from '@/types/api';
import { apiFetch, apiFetchJSON } from '@/utils/apiClient';
import {
  assertAPIResponseOK,
  assertAPIResponseOKOrAllowedStatus,
  assertAPIResponseOKOrThrowStatus,
  coerceTimestampMillis,
  parseOptionalAPIResponseOrAllowedStatus,
  parseOptionalSuccessAPIResponse,
  parseOptionalAPIResponseOrNull,
} from './responseUtils';

async function deleteManagedResource<T extends object>(
  url: string,
  parseErrorMessage: string,
): Promise<T> {
  const response = await apiFetch(url, {
    method: 'DELETE',
  });

  return parseOptionalAPIResponseOrAllowedStatus(
    response,
    [204, 404],
    {} as T,
    `Failed with status ${response.status}`,
    parseErrorMessage,
  );
}

async function runIdempotentManagedMutation(url: string): Promise<void> {
  const response = await apiFetch(url, {
    method: 'PUT',
  });

  await assertAPIResponseOKOrAllowedStatus(response, 404, `Failed with status ${response.status}`);
}

async function setManagedResourceDisplayName(
  url: string,
  displayName: string,
  missingMessage: string,
): Promise<void> {
  const response = await apiFetch(url, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ displayName }),
  });

  await assertAPIResponseOKOrThrowStatus(
    response,
    404,
    missingMessage,
    `Failed with status ${response.status}`,
  );
}

async function runManagedResourceAction(url: string): Promise<void> {
  const response = await apiFetch(url, {
    method: 'POST',
  });

  await assertAPIResponseOK(response, `Failed with status ${response.status}`);
}

type ManagedResourceCommandInit = Omit<RequestInit, 'headers'> & {
  headers?: Record<string, string>;
};

async function triggerManagedResourceCommand<T extends { success?: boolean }>(
  url: string,
  parseErrorMessage: string,
  init?: ManagedResourceCommandInit,
): Promise<T> {
  const response = await apiFetch(url, {
    method: 'POST',
    ...init,
  });

  return parseOptionalSuccessAPIResponse<T>(
    response,
    `Failed with status ${response.status}`,
    parseErrorMessage,
  );
}

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

  static async deleteDockerRuntime(
    agentId: string,
    options: { hide?: boolean; force?: boolean } = {},
  ): Promise<DeleteDockerRuntimeResponse> {
    const params = new URLSearchParams();
    if (options.hide) params.set('hide', 'true');
    if (options.force) params.set('force', 'true');
    const query = params.toString();
    const url = `${this.baseUrl}/agents/docker/runtimes/${encodeURIComponent(agentId)}${query ? `?${query}` : ''}`;

    return deleteManagedResource<DeleteDockerRuntimeResponse>(
      url,
      'Failed to parse delete container runtime response',
    );
  }

  static async unhideDockerRuntime(agentId: string): Promise<void> {
    const url = `${this.baseUrl}/agents/docker/runtimes/${encodeURIComponent(agentId)}/unhide`;
    await runIdempotentManagedMutation(url);
  }

  static async markDockerRuntimePendingUninstall(agentId: string): Promise<void> {
    const url = `${this.baseUrl}/agents/docker/runtimes/${encodeURIComponent(agentId)}/pending-uninstall`;
    await runIdempotentManagedMutation(url);
  }

  static async setDockerRuntimeDisplayName(agentId: string, displayName: string): Promise<void> {
    const url = `${this.baseUrl}/agents/docker/runtimes/${encodeURIComponent(agentId)}/display-name`;
    await setManagedResourceDisplayName(
      url,
      displayName,
      'Container runtime not found',
    );
  }

  static async allowDockerRuntimeReenroll(agentId: string): Promise<void> {
    const url = `${this.baseUrl}/agents/docker/runtimes/${encodeURIComponent(agentId)}/allow-reenroll`;
    await runManagedResourceAction(url);
  }

  static async deleteKubernetesCluster(
    clusterId: string,
  ): Promise<DeleteKubernetesClusterResponse> {
    const url = `${this.baseUrl}/agents/kubernetes/clusters/${encodeURIComponent(clusterId)}`;
    return deleteManagedResource<DeleteKubernetesClusterResponse>(
      url,
      'Failed to parse delete kubernetes cluster response',
    );
  }

  static async unhideKubernetesCluster(clusterId: string): Promise<void> {
    const url = `${this.baseUrl}/agents/kubernetes/clusters/${encodeURIComponent(clusterId)}/unhide`;
    await runIdempotentManagedMutation(url);
  }

  static async markKubernetesClusterPendingUninstall(clusterId: string): Promise<void> {
    const url = `${this.baseUrl}/agents/kubernetes/clusters/${encodeURIComponent(clusterId)}/pending-uninstall`;
    await runIdempotentManagedMutation(url);
  }

  static async setKubernetesClusterDisplayName(
    clusterId: string,
    displayName: string,
  ): Promise<void> {
    const url = `${this.baseUrl}/agents/kubernetes/clusters/${encodeURIComponent(clusterId)}/display-name`;
    await setManagedResourceDisplayName(
      url,
      displayName,
      'Kubernetes cluster not found',
    );
  }

  static async allowKubernetesClusterReenroll(clusterId: string): Promise<void> {
    const url = `${this.baseUrl}/agents/kubernetes/clusters/${encodeURIComponent(clusterId)}/allow-reenroll`;
    await runManagedResourceAction(url);
  }

  static async deleteAgent(agentId: string): Promise<void> {
    if (!agentId) {
      throw new Error('Agent ID is required to remove an agent.');
    }

    const url = `${this.baseUrl}/agents/agent/${encodeURIComponent(agentId)}`;
    const response = await apiFetch(url, { method: 'DELETE' });

    await assertAPIResponseOK(response, `Failed with status ${response.status}`);

    // Consume and ignore the body so the fetch can be reused by the connection pool.
    try {
      await response.text();
    } catch (_err) {
      // Swallow body read errors; the deletion already succeeded.
    }
  }

  static async updateAgentConfig(
    agentId: string,
    config: { commandsEnabled?: boolean },
  ): Promise<void> {
    if (!agentId) {
      throw new Error('Agent ID is required to update agent config.');
    }

    const url = `${this.baseUrl}/agents/agent/${encodeURIComponent(agentId)}/config`;
    const response = await apiFetch(url, {
      method: 'PATCH',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(config),
    });

    await assertAPIResponseOK(response, `Failed with status ${response.status}`);
  }

  static async unlinkAgent(agentId: string): Promise<void> {
    if (!agentId) {
      throw new Error('Agent ID is required to unlink an agent.');
    }

    const url = `${this.baseUrl}/agents/agent/unlink`;
    const response = await apiFetch(url, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        agentId,
      }),
    });

    await assertAPIResponseOK(response, `Failed with status ${response.status}`);
  }

  static async lookupAgent(params: {
    id?: string;
    hostname?: string;
  }): Promise<AgentLookupResponse | null> {
    const search = new URLSearchParams();
    if (params.id) search.set('id', params.id);
    if (params.hostname) search.set('hostname', params.hostname);

    if (!search.toString()) {
      throw new Error('Provide an agent identifier or hostname to look up.');
    }

    const url = `${this.baseUrl}/agents/agent/lookup?${search.toString()}`;
    const response = await apiFetch(url);

    const data = await parseOptionalAPIResponseOrNull<AgentLookupResponse>(
      response,
      404,
      `Lookup failed with status ${response.status}`,
      'Failed to parse agent lookup response',
    );
    if (!data) {
      return null;
    }

    const identity = data?.agent as AgentLookupResponse['agent'];
    if (!identity) {
      return null;
    }

    identity.lastSeen = coerceTimestampMillis(identity.lastSeen, Date.now());

    data.agent = identity;
    return data;
  }

  /**
   * Triggers an update for a Docker container on a specific agent.
   * The update will pull the latest image and recreate the container.
   */
  static async updateDockerContainer(
    agentId: string,
    containerId: string,
    containerName: string,
  ): Promise<UpdateDockerContainerResponse> {
    const url = `${this.baseUrl}/agents/docker/containers/update`;
    return triggerManagedResourceCommand<UpdateDockerContainerResponse>(
      url,
      'Failed to parse update container response',
      {
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ agentId, containerId, containerName }),
      },
    );
  }

  /**
   * Triggers an immediate update check for all containers on a specific container runtime.
   */
  static async checkDockerUpdates(
    agentId: string,
  ): Promise<{ success: boolean; commandId?: string }> {
    const url = `${this.baseUrl}/agents/docker/runtimes/${encodeURIComponent(agentId)}/check-updates`;
    return triggerManagedResourceCommand<{ success: boolean; commandId?: string }>(
      url,
      'Failed to parse check updates response',
    );
  }

  /**
   * Triggers a batch update for all containers with updates available on a specific container runtime.
   */
  static async updateAllDockerContainers(
    agentId: string,
  ): Promise<{ success: boolean; commandId?: string }> {
    const url = `${this.baseUrl}/agents/docker/runtimes/${encodeURIComponent(agentId)}/update-all`;
    return triggerManagedResourceCommand<{ success: boolean; commandId?: string }>(
      url,
      'Failed to parse update all response',
    );
  }
}

export interface DeleteDockerRuntimeResponse {
  success?: boolean;
  agentId?: string;
  message?: string;
  command?: DockerRuntimeCommand;
}

export interface DeleteKubernetesClusterResponse {
  success?: boolean;
  clusterId?: string;
  message?: string;
}

export interface UpdateDockerContainerResponse {
  success?: boolean;
  commandId?: string;
  agentId?: string;
  container?: {
    id: string;
    name: string;
  };
  message?: string;
  note?: string;
}
