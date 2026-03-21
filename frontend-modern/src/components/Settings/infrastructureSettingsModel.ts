import type { ClusterEndpoint, NodeConfig, NodeConfigWithStatus } from '@/types/nodes';
import type { Resource } from '@/types/resource';

export interface DiscoveredServer {
  ip: string;
  port: number;
  type: 'pve' | 'pbs' | 'pmg';
  version: string;
  hostname?: string;
  release?: string;
}

export interface DiscoveryScanStatus {
  scanning: boolean;
  subnet?: string;
  lastScanStartedAt?: number;
  lastResultAt?: number;
  errors?: string[];
}

export type NodeType = 'pve' | 'pbs' | 'pmg';

export const matchConfiguredNodeToResource = (
  configNode: NodeConfigWithStatus | NodeConfig,
  nodeResources: Resource[] | undefined,
) => {
  if (!nodeResources || nodeResources.length === 0) {
    return undefined;
  }

  return nodeResources.find((resource) => {
    if (resource.id === configNode.id) return true;
    if (resource.name === configNode.name) return true;
    const configNameBase = configNode.name.replace(/\.lan$/, '');
    const stateNameBase = resource.name.replace(/\.lan$/, '');
    if (configNameBase === stateNameBase) return true;
    if (resource.id.includes(configNode.name) || configNode.name.includes(resource.name)) {
      return true;
    }
    return false;
  });
};

export const collectConfiguredInfrastructureHosts = (nodes: NodeConfigWithStatus[]) => {
  const configuredHosts = new Set<string>();
  const clusterMemberIPs = new Set<string>();

  nodes.forEach((node) => {
    const cleanedHost = node.host.replace(/^https?:\/\//, '').replace(/:\d+$/, '');
    configuredHosts.add(cleanedHost.toLowerCase());

    if (!('isCluster' in node) || !node.isCluster || !('clusterEndpoints' in node)) {
      return;
    }

    node.clusterEndpoints?.forEach((endpoint: ClusterEndpoint) => {
      if (endpoint.ip) {
        clusterMemberIPs.add(endpoint.ip.toLowerCase());
      }
      if (endpoint.host) {
        clusterMemberIPs.add(endpoint.host.toLowerCase());
      }
    });
  });

  return {
    clusterMemberIPs,
    configuredHosts,
  };
};
