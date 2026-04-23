import { apiFetchJSON } from '@/utils/apiClient';
import { MonitoringAPI } from './monitoring';

export type ConnectionType =
  | 'pve'
  | 'pbs'
  | 'pmg'
  | 'vmware'
  | 'truenas'
  | 'agent'
  | 'docker'
  | 'kubernetes';

export type ConnectionState =
  | 'active'
  | 'paused'
  | 'unauthorized'
  | 'unreachable'
  | 'stale'
  | 'pending';

export type ConnectionSource = 'manual' | 'agent' | 'script';

export interface ConnectionCapabilities {
  supportsPause: boolean;
  supportsScope: boolean;
  supportsTest: boolean;
}

export interface ConnectionError {
  message: string;
  at: string;
}

export interface ConnectionAgentIdentity {
  hostname?: string;
  platform?: string;
  osName?: string;
  osVersion?: string;
  kernelVersion?: string;
  architecture?: string;
  reportIp?: string;
  commandsEnabled?: boolean;
}

export interface Connection {
  id: string;
  type: ConnectionType;
  name: string;
  address: string;
  hostAliases?: string[];
  state: ConnectionState;
  stateReason: string;
  enabled: boolean;
  surfaces: string[];
  scope: Record<string, boolean>;
  lastSeen: string | null;
  lastError: ConnectionError | null;
  source: ConnectionSource;
  agentIdentity?: ConnectionAgentIdentity;
  agentVersion?: string;
  expectedAgentVersion?: string;
  agentUpdateAvailable?: boolean;
  capabilities: ConnectionCapabilities;
}

export type ConnectionSystemComponentRole = 'primary' | 'attachment';

export interface ConnectionSystemComponent {
  connectionId: string;
  type: ConnectionType;
  role: ConnectionSystemComponentRole;
}

export interface ConnectionSystemMember {
  id: string;
  name: string;
  endpoint?: string;
  hostAliases?: string[];
  state: ConnectionState;
  lastSeen?: string | null;
  primary?: boolean;
  agentConnectionId?: string;
}

export interface ConnectionSystem {
  id: string;
  type: ConnectionType;
  clusterName?: string;
  components: ConnectionSystemComponent[];
  members?: ConnectionSystemMember[];
}

export interface ConnectionsListResponse {
  connections: Connection[];
  systems?: ConnectionSystem[];
}

export interface ProbeRequest {
  address: string;
}

export interface ProbeCandidate {
  type: ConnectionType;
  host: string;
  port: number;
  hints?: Record<string, string>;
}

export interface ProbeResponse {
  candidates: ProbeCandidate[];
  probedMs: number;
}

export class ConnectionsAPI {
  private static readonly baseUrl = '/api/connections';

  static async list(): Promise<ConnectionsListResponse> {
    const response: ConnectionsListResponse = await apiFetchJSON(this.baseUrl);
    return {
      connections: response.connections ?? [],
      systems: response.systems ?? [],
    };
  }

  static async probe(address: string): Promise<ProbeResponse> {
    return apiFetchJSON(`${this.baseUrl}/probe`, {
      method: 'POST',
      body: JSON.stringify({ address } satisfies ProbeRequest),
    });
  }

  static async setEnabled(connectionId: string, enabled: boolean): Promise<void> {
    const { type, suffix } = splitConnectionId(connectionId);
    switch (type) {
      case 'pve':
      case 'pbs':
      case 'pmg':
        await apiFetchJSON(`/api/config/nodes/${encodeURIComponent(connectionId)}`, {
          method: 'PUT',
          body: JSON.stringify({ enabled }),
        });
        return;
      case 'vmware':
        await apiFetchJSON(`/api/vmware/connections/${encodeURIComponent(suffix)}`, {
          method: 'PUT',
          body: JSON.stringify({ enabled }),
        });
        return;
      case 'truenas':
        await apiFetchJSON(`/api/truenas/connections/${encodeURIComponent(suffix)}`, {
          method: 'PUT',
          body: JSON.stringify({ enabled }),
        });
        return;
      case 'agent':
      case 'docker':
      case 'kubernetes':
        throw new Error(`Pause is not supported for ${type} connections`);
      default:
        throw new Error(`Unknown connection type: ${type}`);
    }
  }

  static async remove(connectionId: string): Promise<void> {
    const { type, suffix } = splitConnectionId(connectionId);
    switch (type) {
      case 'pve':
      case 'pbs':
      case 'pmg':
        await apiFetchJSON(`/api/config/nodes/${encodeURIComponent(connectionId)}`, {
          method: 'DELETE',
        });
        return;
      case 'vmware':
        await apiFetchJSON(`/api/vmware/connections/${encodeURIComponent(suffix)}`, {
          method: 'DELETE',
        });
        return;
      case 'truenas':
        await apiFetchJSON(`/api/truenas/connections/${encodeURIComponent(suffix)}`, {
          method: 'DELETE',
        });
        return;
      case 'agent':
        await MonitoringAPI.deleteAgent(suffix);
        return;
      case 'docker':
      case 'kubernetes':
        throw new Error(`Remove is not yet supported for ${type} connections`);
      default:
        throw new Error(`Unknown connection type: ${type}`);
    }
  }
}

const splitConnectionId = (id: string): { type: ConnectionType; suffix: string } => {
  const colon = id.indexOf(':');
  if (colon <= 0) {
    throw new Error(`Invalid connection id: ${id}`);
  }
  const type = id.slice(0, colon) as ConnectionType;
  const suffix = id.slice(colon + 1);
  if (!suffix) {
    throw new Error(`Invalid connection id (empty suffix): ${id}`);
  }
  return { type, suffix };
};
