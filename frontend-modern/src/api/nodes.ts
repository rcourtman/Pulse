import type { ClusterEndpoint, NodeConfig } from '../types/nodes';
import { apiFetchJSON } from '@/utils/apiClient';
import {
  arrayOrUndefined,
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
