import { apiFetchJSON } from '@/utils/apiClient';

export type AgentFleetDiagnosticStatus = 'healthy' | 'warning' | 'critical' | 'removed';

export interface AgentFleetDiagnosticSummary {
  total: number;
  healthy: number;
  warning: number;
  critical: number;
  removed: number;
}

export interface AgentFleetDiagnosticReason {
  code: string;
  severity: AgentFleetDiagnosticStatus | string;
  message: string;
  evidence?: string[];
}

export interface AgentFleetDiagnosticRepair {
  code: string;
  label: string;
  description: string;
  supported: boolean;
  mode?: 'handoff' | string;
  platform?: string;
  scope?: string;
}

export interface AgentFleetDiagnosticUpdate {
  state: string;
  autoUpdate: boolean;
  updatedFrom?: string;
  availableVersion?: string;
  lastCheckedAt?: string;
  lastAttemptAt?: string;
  lastSuccessAt?: string;
  lastError?: string;
}

export interface AgentFleetDiagnosticModule {
  name: string;
  enabled: boolean;
  state: string;
  lastError?: string;
  updatedAt?: string;
}

export interface AgentFleetAgentDiagnostic {
  /** Canonical `/api/connections` identifier. */
  connectionId?: string;
  rowKey: string;
  id: string;
  agentId?: string;
  name: string;
  hostname?: string;
  platform?: string;
  osName?: string;
  osVersion?: string;
  kernelVersion?: string;
  architecture?: string;
  machineIdFingerprint?: string;
  reportIp?: string;
  interfaceAddresses?: string[];
  types: string[];
  status: AgentFleetDiagnosticStatus;
  rawStatus?: string;
  lastSeen?: number;
  intervalSeconds?: number;
  version?: string;
  profileId?: string;
  profileName?: string;
  profileVersion?: number;
  deployedProfileVersion?: number;
  agentUpdate?: AgentFleetDiagnosticUpdate;
  agentModules?: AgentFleetDiagnosticModule[];
  reasons: AgentFleetDiagnosticReason[];
  repairActions?: AgentFleetDiagnosticRepair[];
}

export interface AgentFleetDiagnosticsResponse {
  schemaVersion: number;
  generatedAt: number;
  serverVersion?: string;
  agentUpdateTargetVersion?: string;
  summary: AgentFleetDiagnosticSummary;
  agents: AgentFleetAgentDiagnostic[];
}

const EMPTY_SUMMARY: AgentFleetDiagnosticSummary = {
  total: 0,
  healthy: 0,
  warning: 0,
  critical: 0,
  removed: 0,
};

interface AgentFleetDiagnosticsWireResponse {
  schemaVersion?: number;
  generatedAt?: number;
  serverVersion?: string;
  agentUpdateTargetVersion?: string;
  summary?: Partial<AgentFleetDiagnosticSummary>;
  agents?: AgentFleetAgentDiagnostic[];
}

export class AgentDiagnosticsAPI {
  static async getFleetDiagnostics(): Promise<AgentFleetDiagnosticsResponse> {
    const response =
      await apiFetchJSON<AgentFleetDiagnosticsWireResponse>('/api/agents/diagnostics');
    return {
      schemaVersion: response.schemaVersion ?? 0,
      generatedAt: response.generatedAt ?? 0,
      serverVersion: response.serverVersion,
      agentUpdateTargetVersion: response.agentUpdateTargetVersion,
      summary: { ...EMPTY_SUMMARY, ...response.summary },
      agents: response.agents ?? [],
    };
  }
}
