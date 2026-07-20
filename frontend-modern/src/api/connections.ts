import { apiFetchJSON } from '@/utils/apiClient';
import { MonitoringAPI } from './monitoring';

export type ConnectionType =
  'pve' | 'pbs' | 'pmg' | 'vmware' | 'truenas' | 'availability' | 'agent' | 'docker' | 'kubernetes';

export type ConnectionState =
  'active' | 'paused' | 'unauthorized' | 'unreachable' | 'stale' | 'pending';

export type ConnectionSource = 'manual' | 'agent' | 'script';

export type ConnectionFleetEnrollmentState = 'configured' | 'enrolled' | 'paused' | 'pending';
export type ConnectionFleetLivenessState = ConnectionState;
export type ConnectionFleetVersionDrift = 'behind' | 'current' | 'unknown' | 'not-applicable';
export type ConnectionFleetAdapterHealth =
  'blocked' | 'degraded' | 'healthy' | 'paused' | 'unknown';
export type ConnectionFleetConfigRollout = 'configured' | 'paused' | 'reported' | 'unknown';
export type ConnectionFleetCredentialStatus = 'invalid' | 'paused' | 'unknown' | 'verified';
export type ConnectionFleetUpdateStatus =
  | 'checking'
  | 'current'
  | 'disabled'
  | 'failed'
  | 'not-applicable'
  | 'unknown'
  | 'update-available'
  | 'updating';
export type ConnectionFleetRemoteControl = 'disabled' | 'enabled' | 'not-applicable' | 'unknown';
export type ConnectionFleetConfigDriftStatus =
  'current' | 'drifted' | 'not-applicable' | 'paused' | 'pending' | 'unknown';
export type ConnectionFleetRolloutStatus =
  'blocked' | 'current' | 'not-applicable' | 'paused' | 'pending' | 'unknown';
export type ConnectionFleetCredentialHealthStatus =
  'expired' | 'expiring' | 'invalid' | 'not-applicable' | 'paused' | 'unknown' | 'verified';
export type ConnectionFleetCommandPolicyStatus =
  'blocked' | 'disabled' | 'enabled' | 'not-applicable' | 'unknown';
export type ConnectionFleetCommandPolicyState =
  'disabled' | 'enabled' | 'not-applicable' | 'unknown';
export type ConnectionFleetCommandPolicyEnforcement =
  'blocked' | 'drifted' | 'in-sync' | 'not-applicable' | 'pending' | 'unknown';

export interface ConnectionFleetConfigFingerprint {
  version: string;
  hash: string;
}

export interface ConnectionFleetConfigDrift {
  status: ConnectionFleetConfigDriftStatus;
  desired?: ConnectionFleetConfigFingerprint;
  applied?: ConnectionFleetConfigFingerprint;
  lastObservedAt?: string | null;
  reason?: string;
}

export interface ConnectionFleetRolloutState {
  status: ConnectionFleetRolloutStatus;
  stage?: string;
  reason?: string;
}

export interface ConnectionFleetCredentialHealth {
  status: ConnectionFleetCredentialHealthStatus;
  kind?: string;
  rotation?: string;
  lastVerifiedAt?: string | null;
  lastFailedAt?: string | null;
  lastUsedAt?: string | null;
  expiresAt?: string | null;
}

export interface ConnectionFleetCommandPolicy {
  status: ConnectionFleetCommandPolicyStatus;
  desired?: ConnectionFleetCommandPolicyState;
  applied?: ConnectionFleetCommandPolicyState;
  enforcement?: ConnectionFleetCommandPolicyEnforcement;
  reason?: string;
}

export interface ConnectionCapabilities {
  supportsPause: boolean;
  supportsScope: boolean;
  supportsTest: boolean;
}

export interface ConnectionError {
  message: string;
  at: string;
}

export interface ConnectionFleetGovernance {
  enrollmentState: ConnectionFleetEnrollmentState;
  livenessState: ConnectionFleetLivenessState;
  versionDrift: ConnectionFleetVersionDrift;
  adapterHealth: ConnectionFleetAdapterHealth;
  configRollout: ConnectionFleetConfigRollout;
  credentialStatus: ConnectionFleetCredentialStatus;
  updateStatus: ConnectionFleetUpdateStatus;
  remoteControl: ConnectionFleetRemoteControl;
  configDrift?: ConnectionFleetConfigDrift;
  rollout?: ConnectionFleetRolloutState;
  credentialHealth?: ConnectionFleetCredentialHealth;
  commandPolicy?: ConnectionFleetCommandPolicy;
}

export interface ConnectionAgentIdentity {
  hostname?: string;
  platform?: string;
  hostProfile?: string;
  osName?: string;
  osVersion?: string;
  kernelVersion?: string;
  architecture?: string;
  reportIp?: string;
  commandsEnabled?: boolean;
}

export interface ConnectionAgentUpdateStatus {
  state: 'idle' | 'checking' | 'update-available' | 'updating' | 'error' | 'disabled';
  autoUpdate: boolean;
  updatedFrom?: string;
  availableVersion?: string;
  lastCheckedAt?: string;
  lastAttemptAt?: string;
  lastSuccessAt?: string;
  lastError?: string;
}

export interface ConnectionAgentModuleStatus {
  name: string;
  enabled: boolean;
  state: 'disabled' | 'starting' | 'retrying' | 'running';
  lastError?: string;
  updatedAt: string;
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
  agentUpdate?: ConnectionAgentUpdateStatus;
  agentModules?: ConnectionAgentModuleStatus[];
  fleet?: ConnectionFleetGovernance;
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
  systems: ConnectionSystem[];
}

interface ConnectionsListWireResponse {
  connections?: Connection[];
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
    const response: ConnectionsListWireResponse = await apiFetchJSON(this.baseUrl);
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
      case 'availability':
        await apiFetchJSON(`/api/availability-targets/${encodeURIComponent(suffix)}`, {
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
      case 'availability':
        await apiFetchJSON(`/api/availability-targets/${encodeURIComponent(suffix)}`, {
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
