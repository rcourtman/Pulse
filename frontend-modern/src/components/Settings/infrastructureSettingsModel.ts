import type { ClusterEndpoint, NodeConfig, NodeConfigWithStatus } from '@/types/nodes';
import type { Resource } from '@/types/resource';
import type { InfrastructureSystemRow } from './connectionsTableModel';

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

export interface RepresentedDiscoveryHosts {
  pve: Set<string>;
  pbs: Set<string>;
  pmg: Set<string>;
}

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

const createRepresentedDiscoveryHosts = (): RepresentedDiscoveryHosts => ({
  pve: new Set<string>(),
  pbs: new Set<string>(),
  pmg: new Set<string>(),
});

const isNodeType = (value?: string | null): value is NodeType =>
  value === 'pve' || value === 'pbs' || value === 'pmg';

export const normalizeInfrastructureHost = (value?: string | null): string | null => {
  const trimmed = value?.trim();
  if (!trimmed) return null;

  try {
    const parsed = new URL(trimmed);
    if (parsed.hostname) {
      return parsed.hostname.toLowerCase();
    }
  } catch {
    // Fall through to string heuristics below.
  }

  const withoutScheme = trimmed.replace(/^[a-z]+:\/\//i, '');
  const withoutPath = withoutScheme.split('/')[0]?.trim() ?? '';
  if (!withoutPath) return null;

  const bracketedIPv6 = withoutPath.match(/^\[([^\]]+)\](?::\d+)?$/);
  if (bracketedIPv6?.[1]) {
    return bracketedIPv6[1].toLowerCase();
  }

  const hostPortMatch = withoutPath.match(/^(.+):\d+$/);
  if (hostPortMatch?.[1] && !hostPortMatch[1].includes(':')) {
    return hostPortMatch[1].toLowerCase();
  }

  return withoutPath.toLowerCase();
};

const addRepresentedDiscoveryHost = (
  representedHosts: RepresentedDiscoveryHosts,
  type: string | null | undefined,
  value?: string | null,
) => {
  if (!isNodeType(type)) return;
  const normalized = normalizeInfrastructureHost(value);
  if (!normalized) return;
  representedHosts[type].add(normalized);
};

const addRepresentedDiscoveryHosts = (
  representedHosts: RepresentedDiscoveryHosts,
  type: string | null | undefined,
  values: readonly (string | null | undefined)[],
) => {
  values.forEach((value) => addRepresentedDiscoveryHost(representedHosts, type, value));
};

const addClusterEndpointHosts = (
  representedHosts: RepresentedDiscoveryHosts,
  type: 'pve',
  endpoints: readonly ClusterEndpoint[] | undefined,
) => {
  endpoints?.forEach((endpoint) => {
    addRepresentedDiscoveryHosts(representedHosts, type, [
      endpoint.ip,
      endpoint.host,
      endpoint.guestURL,
      endpoint.nodeName,
      endpoint.nodeId,
    ]);
  });
};

const addRowDiscoveryHosts = (
  representedHosts: RepresentedDiscoveryHosts,
  row: InfrastructureSystemRow,
) => {
  if (row.ownerType !== 'pve' && row.ownerType !== 'pbs' && row.ownerType !== 'pmg') {
    return;
  }

  const type = row.ownerType;
  addRepresentedDiscoveryHosts(representedHosts, type, [
    row.name,
    row.host,
    row.connection.name,
    row.connection.address,
    ...(row.connection.hostAliases ?? []),
  ]);

  row.attachedConnections.forEach((connection) => {
    addRepresentedDiscoveryHosts(representedHosts, type, [
      connection.name,
      connection.address,
      ...(connection.hostAliases ?? []),
    ]);
  });

  row.members.forEach((member) => {
    addRepresentedDiscoveryHosts(representedHosts, type, [
      member.name,
      member.host,
      ...(member.hostAliases ?? []),
      member.agentConnection?.name,
      member.agentConnection?.address,
      ...(member.agentConnection?.hostAliases ?? []),
    ]);
  });
};

export const collectRepresentedDiscoveryHosts = (
  nodes: readonly NodeConfigWithStatus[],
  rows: readonly InfrastructureSystemRow[] = [],
): RepresentedDiscoveryHosts => {
  const representedHosts = createRepresentedDiscoveryHosts();

  nodes.forEach((node) => {
    addRepresentedDiscoveryHosts(representedHosts, node.type, [node.name, node.host, node.guestURL]);
    if (node.type === 'pve' && 'clusterEndpoints' in node) {
      addClusterEndpointHosts(representedHosts, node.type, node.clusterEndpoints);
    }
  });

  rows.forEach((row) => addRowDiscoveryHosts(representedHosts, row));

  return representedHosts;
};

export const filterRepresentedDiscoveredServers = (
  servers: readonly DiscoveredServer[],
  nodes: readonly NodeConfigWithStatus[],
  rows: readonly InfrastructureSystemRow[] = [],
): DiscoveredServer[] => {
  const representedHosts = collectRepresentedDiscoveryHosts(nodes, rows);
  return servers.filter((server) => {
    const represented = representedHosts[server.type];
    const normalizedIP = normalizeInfrastructureHost(server.ip);
    if (normalizedIP && represented.has(normalizedIP)) {
      return false;
    }

    const normalizedHostname = normalizeInfrastructureHost(server.hostname);
    if (normalizedHostname && represented.has(normalizedHostname)) {
      return false;
    }

    return true;
  });
};
