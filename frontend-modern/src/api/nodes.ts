import type { ClusterEndpoint, NodeConfig } from '../types/nodes';
import { apiFetch, apiFetchJSON } from '@/utils/apiClient';
import {
  arrayOrUndefined,
  finiteNumberOrUndefined,
  optionalTrimmedString,
  strictBoolean,
  trimmedString,
} from './responseUtils';

type RawClusterEndpoint = Partial<ClusterEndpoint>;

const normalizeClusterEndpoint = (endpoint: RawClusterEndpoint): ClusterEndpoint => ({
  nodeId: trimmedString(endpoint.nodeId),
  nodeName: trimmedString(endpoint.nodeName),
  host: trimmedString(endpoint.host),
  guestURL: optionalTrimmedString(endpoint.guestURL),
  ip: trimmedString(endpoint.ip),
  ipOverride: optionalTrimmedString(endpoint.ipOverride),
  fingerprint: optionalTrimmedString(endpoint.fingerprint),
  online: strictBoolean(endpoint.online),
  lastSeen: trimmedString(endpoint.lastSeen),
  pulseReachable: endpoint.pulseReachable ?? undefined,
  lastPulseCheck: optionalTrimmedString(endpoint.lastPulseCheck),
  pulseError: optionalTrimmedString(endpoint.pulseError),
});

const nodeHasClusterEndpoints = (
  node: NodeConfig,
): node is NodeConfig & { type: 'pve'; clusterEndpoints?: RawClusterEndpoint[] } => node.type === 'pve';

const normalizeNodeConfig = (node: NodeConfig): NodeConfig => {
  if (!nodeHasClusterEndpoints(node)) return node;
  const clusterEndpoints = arrayOrUndefined<RawClusterEndpoint>(node.clusterEndpoints);
  if (!clusterEndpoints) return node;
  return {
    ...node,
    clusterEndpoints: clusterEndpoints.map((endpoint) =>
      normalizeClusterEndpoint(endpoint as RawClusterEndpoint),
    ),
  };
};

type ProxmoxSetupType = 'pve' | 'pbs';

type RawAgentInstallCommandResponse = {
  command?: unknown;
  token?: unknown;
};

export type AgentInstallCommandResponse = {
  command: string;
};

type RawProxmoxSetupCommandResponse = {
  type?: unknown;
  host?: unknown;
  url?: unknown;
  downloadURL?: unknown;
  scriptFileName?: unknown;
  command?: unknown;
  commandWithEnv?: unknown;
  commandWithoutEnv?: unknown;
  setupToken?: unknown;
  expires?: unknown;
  tokenHint?: unknown;
};

export type ProxmoxSetupCommandResponse = {
  type: ProxmoxSetupType;
  host: string;
  url: string;
  downloadURL: string;
  scriptFileName: string;
  command: string;
  commandWithEnv: string;
  commandWithoutEnv?: string;
  expires: number;
  tokenHint: string;
};

export type DownloadedProxmoxSetupScript = {
  content: string;
  contentType: string;
  fileName: string;
};

const normalizeAgentInstallCommandResponse = (
  response: RawAgentInstallCommandResponse,
): AgentInstallCommandResponse => {
  const command = trimmedString(response.command);
  if (!command) {
    throw new Error('Invalid agent install command response');
  }

  return { command };
};

const normalizeProxmoxSetupCommandResponse = (
  response: RawProxmoxSetupCommandResponse,
  expectedType: ProxmoxSetupType,
): ProxmoxSetupCommandResponse => {
  const type = trimmedString(response.type) as ProxmoxSetupType;
  const host = trimmedString(response.host);
  const url = trimmedString(response.url);
  const downloadURL = trimmedString(response.downloadURL);
  const scriptFileName = trimmedString(response.scriptFileName);
  const command = trimmedString(response.command);
  const commandWithEnv = trimmedString(response.commandWithEnv) || command;
  const commandWithoutEnv = optionalTrimmedString(response.commandWithoutEnv);
  const setupToken = trimmedString(response.setupToken);
  const expires = finiteNumberOrUndefined(response.expires);
  const tokenHint = trimmedString(response.tokenHint);
  const nowUnix = Date.now() / 1000;

  if (type !== expectedType) {
    throw new Error('Invalid Proxmox setup response type');
  }
  if (!host) {
    throw new Error('Invalid Proxmox setup response host');
  }
  if (!url) {
    throw new Error('Invalid Proxmox setup response URL');
  }
  if (!downloadURL) {
    throw new Error('Invalid Proxmox setup response downloadURL');
  }
  if (!scriptFileName) {
    throw new Error('Invalid Proxmox setup response scriptFileName');
  }
  if (!command) {
    throw new Error('Invalid Proxmox setup response command');
  }
  if (!commandWithEnv) {
    throw new Error('Invalid Proxmox setup response commandWithEnv');
  }
  if (!setupToken) {
    throw new Error('Invalid Proxmox setup response setup token');
  }
  if (!tokenHint) {
    throw new Error('Invalid Proxmox setup response token hint');
  }
  if (expires === undefined || expires <= nowUnix) {
    throw new Error('Invalid Proxmox setup response expiry');
  }

  return {
    type,
    host,
    url,
    downloadURL,
    scriptFileName,
    command,
    commandWithEnv,
    commandWithoutEnv,
    expires,
    tokenHint,
  };
};

export class NodesAPI {
  private static readonly baseUrl = '/api/config/nodes';

  static async getNodes(): Promise<NodeConfig[]> {
    // The API returns an array of nodes directly
    const nodes: NodeConfig[] = await apiFetchJSON(this.baseUrl);
    return nodes.map(normalizeNodeConfig);
  }

  static async addNode(node: NodeConfig): Promise<{ success: boolean; message?: string }> {
    return apiFetchJSON(this.baseUrl, {
      method: 'POST',
      body: JSON.stringify(node),
    });
  }

  static async updateNode(
    nodeId: string,
    node: NodeConfig,
  ): Promise<{ success: boolean; message?: string }> {
    return apiFetchJSON(`${this.baseUrl}/${encodeURIComponent(nodeId)}`, {
      method: 'PUT',
      body: JSON.stringify(node),
    });
  }

  static async deleteNode(nodeId: string): Promise<{ success: boolean; message?: string }> {
    return apiFetchJSON(`${this.baseUrl}/${encodeURIComponent(nodeId)}`, {
      method: 'DELETE',
    });
  }

  static async testConnection(node: NodeConfig): Promise<{
    status: string;
    message?: string;
    isCluster?: boolean;
    nodeCount?: number;
    clusterNodeCount?: number;
    datastoreCount?: number;
    warnings?: string[];
  }> {
    return apiFetchJSON(`${this.baseUrl}/test-connection`, {
      method: 'POST',
      body: JSON.stringify(node),
    });
  }

  static async testExistingNode(nodeId: string): Promise<{
    status: string;
    message?: string;
    latency?: number;
    warnings?: string[];
  }> {
    return apiFetchJSON(`${this.baseUrl}/${encodeURIComponent(nodeId)}/test`, {
      method: 'POST',
    });
  }

  static async refreshClusterNodes(nodeId: string): Promise<{
    status: string;
    clusterName?: string;
    oldNodeCount?: number;
    newNodeCount?: number;
    nodesAdded?: number;
    clusterNodes?: Array<{
      nodeId: string;
      nodeName: string;
      host: string;
      online: boolean;
    }>;
  }> {
    return apiFetchJSON(`${this.baseUrl}/${encodeURIComponent(nodeId)}/refresh-cluster`, {
      method: 'POST',
    });
  }

  static async getAgentInstallCommand(params: {
    type: 'pve' | 'pbs';
    enableProxmox: boolean;
  }): Promise<AgentInstallCommandResponse> {
    const response = await apiFetchJSON<RawAgentInstallCommandResponse>('/api/agent-install-command', {
      method: 'POST',
      body: JSON.stringify(params),
    });
    return normalizeAgentInstallCommandResponse(response);
  }

  static async getProxmoxSetupCommand(params: {
    type: 'pve' | 'pbs';
    host: string;
    backupPerms: boolean;
  }): Promise<ProxmoxSetupCommandResponse> {
    const response = await apiFetchJSON<RawProxmoxSetupCommandResponse>('/api/setup-script-url', {
      method: 'POST',
      body: JSON.stringify(params),
    });
    return normalizeProxmoxSetupCommandResponse(response, params.type);
  }

  static async downloadProxmoxSetupScript(
    bootstrap: ProxmoxSetupCommandResponse,
  ): Promise<DownloadedProxmoxSetupScript> {
    const response = await apiFetch(bootstrap.downloadURL);
    if (!response.ok) {
      throw new Error('Failed to fetch setup script');
    }
    const contentType = trimmedString(response.headers.get('Content-Type'));
    if (!contentType.startsWith('text/x-shellscript')) {
      throw new Error('Invalid Proxmox setup script content type');
    }

    const contentDisposition = trimmedString(response.headers.get('Content-Disposition'));
    const fileNameMatch =
      contentDisposition.match(/filename="([^"]+)"/i) ??
      contentDisposition.match(/filename=([^;]+)/i);
    const fileName = trimmedString(fileNameMatch?.[1] ?? '');
    if (!fileName || fileName !== bootstrap.scriptFileName) {
      throw new Error('Invalid Proxmox setup script filename');
    }

    const content = await response.text();
    if (!content.trim()) {
      throw new Error('Empty Proxmox setup script response');
    }

    return {
      content,
      contentType,
      fileName,
    };
  }
}
