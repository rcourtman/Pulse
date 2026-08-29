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

// Agent-reported privilege profile. Descriptive only: a least-privilege
// install is an intentional hardening choice, never a health defect.
export interface AgentFleetDiagnosticPrivilege {
  runningAsRoot: boolean;
  serviceUser?: string;
  commandAuthority?: 'monitoring-only' | 'command-capable' | 'legacy' | string;
  credentialKnown?: boolean;
  credentialExec?: boolean;
  typedHelper?: boolean;
  smartctlHelper?: boolean;
  pctHelper?: boolean;
  actionRunnerCredentialIssued?: boolean;
  actionRunnerCredentialActive?: boolean;
  actionRunnerRuntimeRole?: string;
  actionRunnerCapability?: string;
  actionRunnerBindingVersion?: string;
  actionRunnerConnected?: boolean;
  actionRunnerVersion?: string;
  actionRunnerConnectedAt?: number;
  actionRunnerReceiptProtocol?: number;
  actionRunnerPreflightProtocol?: number;
  actionRunnerDockerObservationProtocol?: number;
}

export interface ActionRunnerCredentialRequest {
  agentId: string;
  hostname: string;
  name?: string;
}

export interface ActionRunnerCredentialResponse {
  token: string;
  tokenId: string;
  organizationId: string;
  agentId: string;
  hostname: string;
  runtimeRole: 'action-runner';
  actionCapability: 'typed_actions.v1';
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
  privilege?: AgentFleetDiagnosticPrivilege;
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

  static issueActionRunnerCredential(
    request: ActionRunnerCredentialRequest,
  ): Promise<ActionRunnerCredentialResponse> {
    return apiFetchJSON('/api/agents/action-runner/credential', {
      method: 'POST',
      body: JSON.stringify(request),
    });
  }
}
