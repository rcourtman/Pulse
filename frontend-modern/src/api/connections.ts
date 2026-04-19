import { apiFetchJSON } from '@/utils/apiClient';

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

export interface Connection {
  id: string;
  type: ConnectionType;
  name: string;
  address: string;
  state: ConnectionState;
  stateReason: string;
  enabled: boolean;
  surfaces: string[];
  scope: Record<string, boolean>;
  lastSeen: string | null;
  lastError: ConnectionError | null;
  source: ConnectionSource;
  capabilities: ConnectionCapabilities;
}

export interface ConnectionsListResponse {
  connections: Connection[];
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

  static async list(): Promise<Connection[]> {
    const response: ConnectionsListResponse = await apiFetchJSON(this.baseUrl);
    return response.connections ?? [];
  }

  static async probe(address: string): Promise<ProbeResponse> {
    return apiFetchJSON(`${this.baseUrl}/probe`, {
      method: 'POST',
      body: JSON.stringify({ address } satisfies ProbeRequest),
    });
  }
}
