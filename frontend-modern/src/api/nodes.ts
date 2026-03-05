import type { ClusterEndpoint, NodeConfig } from '../types/nodes';
import { apiFetchJSON } from '@/utils/apiClient';

type RawClusterEndpoint = Partial<ClusterEndpoint>;

const asString = (value: unknown): string =>
  typeof value === 'string' ? value.trim() : value == null ? '' : String(value).trim();

const asOptionalString = (value: unknown): string | undefined => {
  const normalized = asString(value);
  return normalized.length > 0 ? normalized : undefined;
};

const asBoolean = (value: unknown): boolean => value === true;

const normalizeClusterEndpoint = (endpoint: RawClusterEndpoint): ClusterEndpoint => ({
  nodeId: asString(endpoint.nodeId),
  nodeName: asString(endpoint.nodeName),
  host: asString(endpoint.host),
  guestURL: asOptionalString(endpoint.guestURL),
  ip: asString(endpoint.ip),
  ipOverride: asOptionalString(endpoint.ipOverride),
  fingerprint: asOptionalString(endpoint.fingerprint),
  online: asBoolean(endpoint.online),
  lastSeen: asString(endpoint.lastSeen),
  pulseReachable: endpoint.pulseReachable ?? undefined,
  lastPulseCheck: asOptionalString(endpoint.lastPulseCheck),
  pulseError: asOptionalString(endpoint.pulseError),
});

const normalizeNodeConfig = (node: NodeConfig): NodeConfig => {
  if (!('clusterEndpoints' in node) || !Array.isArray(node.clusterEndpoints)) return node;
  return {
    ...node,
    clusterEndpoints: node.clusterEndpoints.map((endpoint) =>
      normalizeClusterEndpoint(endpoint as RawClusterEndpoint),
    ),
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
    type: 'pve';
    enableProxmox: boolean;
  }): Promise<{ command: string }> {
    return apiFetchJSON('/api/agent-install-command', {
      method: 'POST',
      body: JSON.stringify(params),
    });
  }
}
